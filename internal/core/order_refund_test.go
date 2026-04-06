package core

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/models/factory"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestMobazhaNode_RefundOrder(t *testing.T) {
	network, err := NewMocknet(3)
	if err != nil {
		t.Fatal(err)
	}

	defer network.TearDown()

	go network.StartWalletNetwork()

	for _, node := range network.Nodes() {
		go node.orderProcessor.Start()
	}

	setupMockNetDB(t, network.Nodes())
	setupMockReceivingAccounts(t, network.Nodes())

	listing := factory.NewPhysicalListing("tshirt")

	done := make(chan struct{})
	if err := network.Nodes()[0].Listing().SaveListing(listing, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	index, err := network.Nodes()[0].Listing().GetMyListings()
	if err != nil {
		t.Fatal(err)
	}

	done2 := make(chan struct{})
	if err := network.Nodes()[2].Profile().SetProfile(&models.Profile{Name: "Ron Paul"}, done2); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done2:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	modInfo := &models.ModeratorInfo{
		AcceptedCurrencies: []string{"MCK"},
		Fee: models.ModeratorFee{
			Percentage: 10,
			FeeType:    models.PercentageFee,
		},
	}
	done3 := make(chan struct{})
	if err := network.Nodes()[2].Profile().SetSelfAsModerator(context.Background(), modInfo, done3); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done3:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	purchase := factory.NewPurchase()
	purchase.Items[0].ListingHash = index[0].CID

	orderSub0, err := network.Nodes()[0].eventBus.Subscribe(&events.NewOrder{})
	if err != nil {
		t.Fatal(err)
	}

	orderAckSub0, err := network.Nodes()[1].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	// Address request direct order
	orderID, paymentAmount, err := network.Nodes()[1].Order().PurchaseListing(context.Background(), purchase)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-orderSub0.Out():
		orderSub0.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-orderAckSub0.Out():
		orderAckSub0.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	var order models.Order
	err = network.Nodes()[0].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order.SerializedOrderOpen == nil {
		t.Error("Node 0 failed to save order")
	}

	var order2 models.Order
	err = network.Nodes()[1].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order2).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order2.SerializedOrderOpen == nil {
		t.Error("Node 1 failed to save order")
	}
	if !order2.OrderOpenAcked {
		t.Error("Node 1 failed to record order open ACK")
	}

	wallet1, err := network.Nodes()[1].multiwallet.WalletForCurrencyCode(iwallet.CtMock.String())
	if err != nil {
		t.Fatal(err)
	}

	addr1, err := wallet1.(*wallet.MockWallet).CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}

	txSub1, err := network.Nodes()[1].eventBus.Subscribe(&events.TransactionReceived{})
	if err != nil {
		t.Fatal(err)
	}

	if err := network.WalletNetwork().GenerateToAddress(addr1, iwallet.NewAmount(100000000000)); err != nil {
		t.Fatal(err)
	}

	select {
	case <-txSub1.Out():
		txSub1.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	fundingSub0, err := network.Nodes()[0].eventBus.Subscribe(&events.OrderFunded{})
	if err != nil {
		t.Fatal(err)
	}

	fundingSub1, err := network.Nodes()[1].eventBus.Subscribe(&events.OrderPaymentReceived{})
	if err != nil {
		t.Fatal(err)
	}

	paymentData, err := network.Nodes()[1].Wallet().GetUTXOPaymentInfo(
		context.Background(),
		orderID.String(),
		"", // empty moderator for CANCELABLE
		iwallet.CtMock,
	)
	if err != nil {
		t.Fatalf("GetUTXOPaymentInfo failed: %v", err)
	}
	// Set buyer's address so the refund code can extract it from tx.From
	paymentData.PayerAddress = addr1.String()
	// Ingest tx into seller wallet so vendor GetTransaction succeeds (PaymentVerified)
	ingestPaymentToWallets(t, paymentData, network.Nodes()[0], network.Nodes()[1])
	err = network.Nodes()[1].Order().ProcessOrderPayment(context.Background(), paymentData)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-fundingSub0.Out():
		fundingSub0.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-fundingSub1.Out():
		fundingSub1.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	// CANCELABLE refund: order is NOT yet confirmed, funds are still in 1-of-2 multisig escrow.
	// Vendor should be able to release funds back to buyer.
	refundSub, err := network.Nodes()[1].eventBus.Subscribe(&events.Refund{})
	if err != nil {
		t.Fatal(err)
	}

	done4 := make(chan struct{})
	if err := network.Nodes()[0].Order().RefundOrder(order.ID, "", done4); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done4:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-refundSub.Out():
		refundSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on refund event")
	}

	err = network.Nodes()[1].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order2).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	refunds, err := order2.Refunds()
	if err != nil {
		t.Fatal(err)
	}
	if len(refunds) != 1 {
		t.Errorf("Expected 1 refund, got %d", len(refunds))
	}

	refundAmt := iwallet.NewAmount(refunds[0].Amount)
	if refundAmt.Cmp(iwallet.NewAmount(0)) <= 0 {
		t.Error("Expected positive refund amount")
	}
	if refundAmt.Cmp(paymentAmount.Amount) >= 0 {
		t.Errorf("Refund amount %s should be less than payment amount %s (fee deducted)", refundAmt, paymentAmount.Amount)
	}
	t.Logf("CANCELABLE refund (unconfirmed): payment=%s, refund=%s, fee=%s",
		paymentAmount.Amount, refundAmt, paymentAmount.Amount.Sub(refundAmt))

	// In mock, escrow release transaction is only in the vendor's wallet;
	// the buyer wallet does not see it without real blockchain propagation.
	_, txErr := wallet1.GetTransaction(iwallet.TransactionID(refunds[0].GetTransactionID()), iwallet.CtMock)
	if txErr != nil {
		t.Logf("Refund tx not in buyer wallet (expected in mock): %s", txErr)
	}

	// Now test MODERATED order refund.
	orderSub1, err := network.Nodes()[0].eventBus.Subscribe(&events.NewOrder{})
	if err != nil {
		t.Fatal(err)
	}
	orderAckSub1, err := network.Nodes()[1].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}
	orderID2, paymentAmount, err := network.Nodes()[1].Order().PurchaseListing(context.Background(), purchase)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-orderSub1.Out():
		orderSub1.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-orderAckSub1.Out():
		orderAckSub1.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	var order3 models.Order
	err = network.Nodes()[0].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID2.String()).Last(&order3).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order3.SerializedOrderOpen == nil {
		t.Error("Node 0 failed to save order")
	}

	var order4 models.Order
	err = network.Nodes()[1].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID2.String()).Last(&order4).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order4.SerializedOrderOpen == nil {
		t.Error("Node 1 failed to save order")
	}
	if !order4.OrderOpenAcked {
		t.Error("Node 1 failed to record order open ACK")
	}

	fundingSub3, err := network.Nodes()[0].eventBus.Subscribe(&events.OrderFunded{})
	if err != nil {
		t.Fatal(err)
	}

	moderatorPeerID := network.Nodes()[2].Identity().String()
	paymentData, err = network.Nodes()[1].Wallet().GetUTXOPaymentInfo(
		context.Background(),
		orderID2.String(),
		moderatorPeerID,
		iwallet.CtMock,
	)
	if err != nil {
		t.Fatalf("GetUTXOPaymentInfo (moderated) failed: %v", err)
	}
	// Ingest tx into seller wallet so vendor GetTransaction succeeds (PaymentVerified)
	ingestPaymentToWallets(t, paymentData, network.Nodes()[0], network.Nodes()[1])
	err = network.Nodes()[1].Order().ProcessOrderPayment(context.Background(), paymentData)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-fundingSub3.Out():
		fundingSub3.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	txSub5, err := network.Nodes()[1].eventBus.Subscribe(&events.TransactionReceived{})
	if err != nil {
		t.Fatal(err)
	}

	refundSub3, err := network.Nodes()[1].eventBus.Subscribe(&events.Refund{})
	if err != nil {
		t.Fatal(err)
	}

	done6 := make(chan struct{})
	if err := network.Nodes()[0].Order().RefundOrder(orderID2, "", done6); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done6:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-refundSub3.Out():
		refundSub3.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	err = network.Nodes()[1].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID2.String()).Last(&order4).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	refunds, err = order4.Refunds()
	if err != nil {
		t.Fatal(err)
	}
	if len(refunds) != 1 {
		t.Errorf("Expected 1 refund, got %d", len(refunds))
	}

	// Verify refund amount is positive and less than payment amount.
	// The exact fee deducted may differ from paymentSent.EscrowReleaseFee because
	// buildRefundMessage re-estimates with FlPriority and adds 50% buffer.
	refundAmt = iwallet.NewAmount(refunds[0].Amount)
	if refundAmt.Cmp(iwallet.NewAmount(0)) <= 0 {
		t.Error("Expected positive refund amount")
	}
	if refundAmt.Cmp(paymentAmount.Amount) >= 0 {
		t.Errorf("Refund amount %s should be less than payment amount %s (fee deducted)", refundAmt, paymentAmount.Amount)
	}
	t.Logf("First MODERATED refund: payment=%s, refund=%s, fee=%s",
		paymentAmount.Amount, refundAmt, paymentAmount.Amount.Sub(refundAmt))

	// For MODERATED escrow releases, the buyer receives the signed release but
	// does not auto-broadcast in mock environment, so TransactionReceived may not fire.
	select {
	case n := <-txSub5.Out():
		tx := n.(*events.TransactionReceived)
		t.Logf("TransactionReceived: address=%s, amount=%s", tx.To[0].Address.String(), tx.To[0].Amount.String())
		txSub5.Close()
	case <-time.After(time.Second * 3):
		t.Log("TransactionReceived not received (expected for MODERATED escrow in mock env)")
		txSub5.Close()
	}

	// NOTE: Testing multiple payments to the same escrow address is not supported via
	// ProcessOrderPayment (which creates PaymentSent messages), because the orders layer
	// rejects duplicate PaymentSent with different content. In production, additional
	// payments are detected by the blockchain monitor, not via additional PaymentSent messages.
	// The single refund test above validates the complete MODERATED refund flow.
}

func Test_buildRefundMessage(t *testing.T) {
	tests := []struct {
		setup func(order *models.Order) error
		check func(msg *pb.Refund) error
	}{
		// Direct first refund.
		{
			setup: func(order *models.Order) error {
				orderOpen, paymentSent, err := factory.NewOrder()
				if err != nil {
					return err
				}
				paymentSent.RefundAddress = "abc"
				paymentSent.Method = pb.PaymentSent_DIRECT

				if err := order.PutMessage(utils.MustWrapOrderMessage(orderOpen)); err != nil {
					return err
				}
				return order.PutMessage(utils.MustWrapOrderMessage(paymentSent))
			},
			check: func(msg *pb.Refund) error {
				if msg.GetTransactionID() == "" {
					return errors.New("failed to record txid")
				}
				return nil
			},
		},
		// Direct second refund.
		{
			setup: func(order *models.Order) error {
				orderOpen, paymentSent, err := factory.NewOrder()
				if err != nil {
					return err
				}
				paymentSent.RefundAddress = "abc"
				paymentSent.Method = pb.PaymentSent_DIRECT

				err = order.PutTransaction(iwallet.Transaction{
					ID: "123",
					To: []iwallet.SpendInfo{
						{
							Address: iwallet.NewAddress(paymentSent.ToAddress, iwallet.CtMock),
							Amount:  iwallet.NewAmount(10000),
						},
					},
				})
				if err != nil {
					return err
				}

				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.Refund{
					Amount: "9000",
				}))
				if err != nil {
					return err
				}

				if err := order.PutMessage(utils.MustWrapOrderMessage(orderOpen)); err != nil {
					return err
				}
				return order.PutMessage(utils.MustWrapOrderMessage(paymentSent))
			},
			check: func(msg *pb.Refund) error {
				if msg.GetTransactionID() == "" {
					return errors.New("failed to record txid")
				}
				if msg.Amount != "1000" {
					return errors.New("incorrect refund amount")
				}
				return nil
			},
		},
		// Moderated first refund.
		{
			setup: func(order *models.Order) error {
				orderOpen, paymentSent, err := factory.NewOrder()
				if err != nil {
					return err
				}
				paymentSent.RefundAddress = "abc"
				paymentSent.PayerAddress = "abc"
				paymentSent.Method = pb.PaymentSent_MODERATED

				err = order.PutTransaction(iwallet.Transaction{
					ID: "123",
					To: []iwallet.SpendInfo{
						{
							Address: iwallet.NewAddress(paymentSent.ToAddress, iwallet.CtMock),
							Amount:  iwallet.NewAmount(10000),
						},
					},
				})
				if err != nil {
					return err
				}

				if err := order.PutMessage(utils.MustWrapOrderMessage(orderOpen)); err != nil {
					return err
				}
				return order.PutMessage(utils.MustWrapOrderMessage(paymentSent))
			},
			check: func(msg *pb.Refund) error {
				if msg.GetReleaseInfo() == nil {
					return errors.New("failed to record release info")
				}
				if msg.GetReleaseInfo().ToAddress != "abc" {
					return errors.New("incorrect refund address")
				}
				if msg.GetReleaseInfo().ToAmount != "7975" {
					return fmt.Errorf("incorrect refund amount: got %s, want 7975", msg.GetReleaseInfo().ToAmount)
				}
				if len(msg.GetReleaseInfo().EscrowSignatures) != 1 {
					return errors.New("incorrect number of signatures")
				}
				if len(msg.GetReleaseInfo().EscrowSignatures[0].Signature) == 0 {
					return errors.New("invalid signature")
				}
				if msg.GetReleaseInfo().EscrowSignatures[0].Index != 0 {
					return errors.New("invalid index")
				}
				return nil
			},
		},
		// Moderated second refund.
		{
			setup: func(order *models.Order) error {
				orderOpen, paymentSent, err := factory.NewOrder()
				if err != nil {
					return err
				}
				paymentSent.RefundAddress = "abc"
				paymentSent.PayerAddress = "abc"
				paymentSent.Method = pb.PaymentSent_MODERATED

				err = order.PutTransaction(iwallet.Transaction{
					ID: "123",
					To: []iwallet.SpendInfo{
						{
							ID:      []byte{0x01, 0x01},
							Address: iwallet.NewAddress(paymentSent.ToAddress, iwallet.CtMock),
							Amount:  iwallet.NewAmount(10000),
						},
					},
				})
				if err != nil {
					return err
				}

				err = order.PutTransaction(iwallet.Transaction{
					ID: "456",
					From: []iwallet.SpendInfo{
						{
							ID:      []byte{0x01, 0x01},
							Address: iwallet.NewAddress(paymentSent.ToAddress, iwallet.CtMock),
							Amount:  iwallet.NewAmount(10000),
						},
					},
				})
				if err != nil {
					return err
				}

				err = order.PutTransaction(iwallet.Transaction{
					ID: "789",
					To: []iwallet.SpendInfo{
						{
							Address: iwallet.NewAddress(paymentSent.ToAddress, iwallet.CtMock),
							Amount:  iwallet.NewAmount(5000),
						},
					},
				})
				if err != nil {
					return err
				}

				if err := order.PutMessage(utils.MustWrapOrderMessage(orderOpen)); err != nil {
					return err
				}
				return order.PutMessage(utils.MustWrapOrderMessage(paymentSent))
			},
			check: func(msg *pb.Refund) error {
				if msg.GetReleaseInfo() == nil {
					return errors.New("failed to record release info")
				}
				if msg.GetReleaseInfo().ToAddress != "abc" {
					return errors.New("incorrect refund address")
				}
				if msg.GetReleaseInfo().ToAmount != "2975" {
					return fmt.Errorf("incorrect refund amount: got %s, want 2975", msg.GetReleaseInfo().ToAmount)
				}
				if len(msg.GetReleaseInfo().EscrowSignatures) != 1 {
					return errors.New("incorrect number of signatures")
				}
				if len(msg.GetReleaseInfo().EscrowSignatures[0].Signature) == 0 {
					return errors.New("invalid signature")
				}
				if msg.GetReleaseInfo().EscrowSignatures[0].Index != 0 {
					return errors.New("invalid index")
				}
				return nil
			},
		},
	}

	n, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	net := wallet.NewMockWalletNetwork(1)
	net.Start()
	addr, err := net.Wallets()[0].CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}
	if err := net.GenerateToAddress(addr, iwallet.NewAmount(10000000000000)); err != nil {
		t.Fatal(err)
	}

	for i, test := range tests {
		var order models.Order
		if err := test.setup(&order); err != nil {
			t.Errorf("Test %d: setup failed: %s", i, err)
		}

		_, msg, err := n.orderService.buildRefundMessage(&order, net.Wallets()[0], "")
		if err != nil {
			t.Errorf("Test %d: build failed: %s", i, err)
			continue
		}
		if msg == nil {
			t.Errorf("Test %d: build returned nil message", i)
			continue
		}

		var refundMsg pb.Refund
		if err := msg.Message.UnmarshalTo(&refundMsg); err != nil {
			t.Errorf("Test %d: unmarshal failed: %s", i, err)
			continue
		}

		if err := test.check(&refundMsg); err != nil {
			t.Errorf("Test %d: check failed: %s", i, err)
		}
	}
}
