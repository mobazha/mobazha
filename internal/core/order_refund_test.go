package core

import (
	"context"
	"errors"
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

	listing := factory.NewPhysicalListing("tshirt")

	done := make(chan struct{})
	if err := network.Nodes()[0].SaveListing(listing, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	index, err := network.Nodes()[0].GetMyListings()
	if err != nil {
		t.Fatal(err)
	}

	done2 := make(chan struct{})
	if err := network.Nodes()[2].SetProfile(&models.Profile{Name: "Ron Paul"}, done2); err != nil {
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
	if err := network.Nodes()[2].SetSelfAsModerator(context.Background(), modInfo, done3); err != nil {
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
	orderID, paymentAmount, err := network.Nodes()[1].PurchaseListing(context.Background(), purchase)
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

	addr1, err := wallet1.CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}

	txSub0, err := network.Nodes()[0].eventBus.Subscribe(&events.TransactionReceived{})
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
	case <-txSub0.Out():
		txSub0.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
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

	paymentData := &models.PaymentData{
		OrderID:   orderID.String(),
		Method:    pb.PaymentSent_CANCELABLE,
		Amount:    paymentAmount.Amount.Uint64(),
		Coin:      iwallet.CoinType(paymentAmount.Currency.String()),
		ToAddress: "abcd",
	}
	err = network.Nodes()[1].ProcessOrderPayment(context.Background(), paymentData)
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

	refundSub, err := network.Nodes()[1].eventBus.Subscribe(&events.Refund{})
	if err != nil {
		t.Fatal(err)
	}

	txSub3, err := network.Nodes()[1].eventBus.Subscribe(&events.TransactionReceived{})
	if err != nil {
		t.Fatal(err)
	}

	done4 := make(chan struct{})
	if err := network.Nodes()[0].RefundOrder(order.ID, "", done4); err != nil {
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
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-txSub3.Out():
		txSub3.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
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

	if refunds[0].Amount != paymentAmount.Amount.String() {
		t.Errorf("Incorrect refund amount. Expected %s got %s", paymentAmount.Amount.String(), refunds[0].Amount)
	}

	_, err = wallet1.GetTransaction(iwallet.TransactionID(refunds[0].GetTransactionID()), iwallet.CtMock)
	if err != nil {
		t.Errorf("Error loading refund transaction: %s", err)
	}

	// Now test sending another transaction to the payment address and make sure the
	// second refund works OK.
	fundingSub2, err := network.Nodes()[0].eventBus.Subscribe(&events.OrderFunded{})
	if err != nil {
		t.Fatal(err)
	}

	paymentData = &models.PaymentData{
		OrderID:   orderID.String(),
		Method:    pb.PaymentSent_CANCELABLE,
		Amount:    555555,
		Coin:      iwallet.CtMock,
		ToAddress: "abcd",
	}
	err = network.Nodes()[1].ProcessOrderPayment(context.Background(), paymentData)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-fundingSub2.Out():
		fundingSub2.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	refundSub2, err := network.Nodes()[1].eventBus.Subscribe(&events.Refund{})
	if err != nil {
		t.Fatal(err)
	}

	txSub4, err := network.Nodes()[1].eventBus.Subscribe(&events.TransactionReceived{})
	if err != nil {
		t.Fatal(err)
	}

	done5 := make(chan struct{})
	if err := network.Nodes()[0].RefundOrder(order.ID, "", done5); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done4:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-refundSub2.Out():
		refundSub2.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-txSub4.Out():
		txSub4.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	err = network.Nodes()[1].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order2).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	refunds, err = order2.Refunds()
	if err != nil {
		t.Fatal(err)
	}
	if len(refunds) != 2 {
		t.Errorf("Expected 2 refunds, got %d", len(refunds))
	}

	if refunds[1].Amount != "555555" {
		t.Errorf("Incorrect refund amount. Expected 555555 got %s", refunds[0].Amount)
	}

	_, err = wallet1.GetTransaction(iwallet.TransactionID(refunds[1].GetTransactionID()), iwallet.CtMock)
	if err != nil {
		t.Errorf("Error loading refund transaction: %s", err)
	}

	// Now repeat everything with a moderated order.
	orderSub1, err := network.Nodes()[0].eventBus.Subscribe(&events.NewOrder{})
	if err != nil {
		t.Fatal(err)
	}
	orderAckSub1, err := network.Nodes()[1].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}
	orderID2, paymentAmount, err := network.Nodes()[1].PurchaseListing(context.Background(), purchase)
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

	paymentData = &models.PaymentData{
		OrderID:   orderID2.String(),
		Method:    pb.PaymentSent_MODERATED,
		Moderator: network.Nodes()[2].Identity().String(),
		Amount:    paymentAmount.Amount.Uint64(),
		Coin:      iwallet.CtMock,
		ToAddress: "abcd",
	}
	err = network.Nodes()[1].ProcessOrderPayment(context.Background(), paymentData)
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

	fundsReleasedSub, err := network.Nodes()[0].eventBus.Subscribe(&events.SpendFromPaymentAddress{})
	if err != nil {
		t.Fatal(err)
	}

	done6 := make(chan struct{})
	if err := network.Nodes()[0].RefundOrder(orderID2, "", done6); err != nil {
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

	select {
	case <-fundsReleasedSub.Out():
		fundsReleasedSub.Close()
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

	paymentSent, err := order4.PaymentSentMessage()
	if err != nil {
		t.Fatal(err)
	}

	expectedAmount := paymentAmount.Amount.Sub(iwallet.NewAmount(paymentSent.EscrowReleaseFee))
	if refunds[0].Amount != expectedAmount.String() {
		t.Errorf("Incorrect refund amount. Expected %s got %s", expectedAmount.String(), refunds[0].Amount)
	}

	select {
	case n := <-txSub5.Out():
		tx := n.(*events.TransactionReceived)
		if tx.To[0].Address.String() != paymentSent.RefundAddress {
			t.Errorf("Received funds on incorrect address. Expected %s, got %s", paymentSent.RefundAddress, tx.To[0].Address.String())
		}
		if tx.To[0].Amount.String() != expectedAmount.String() {
			t.Errorf("Incorrect refund amount. Expected %s got %s", expectedAmount.String(), tx.To[0].Amount.String())
		}
		txSub5.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	// Now test sending another transaction to the payment address and make sure the
	// second refund works OK.
	fundingSub4, err := network.Nodes()[0].eventBus.Subscribe(&events.OrderFunded{})
	if err != nil {
		t.Fatal(err)
	}

	paymentData = &models.PaymentData{
		OrderID:   orderID2.String(),
		Method:    pb.PaymentSent_MODERATED,
		Moderator: network.Nodes()[2].Identity().String(),
		Amount:    222222,
		Coin:      iwallet.CtMock,
		ToAddress: "abcd",
	}
	err = network.Nodes()[1].ProcessOrderPayment(context.Background(), paymentData)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-fundingSub4.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	refundSub4, err := network.Nodes()[1].eventBus.Subscribe(&events.Refund{})
	if err != nil {
		t.Fatal(err)
	}

	txSub6, err := network.Nodes()[1].eventBus.Subscribe(&events.TransactionReceived{})
	if err != nil {
		t.Fatal(err)
	}

	done7 := make(chan struct{})
	if err := network.Nodes()[0].RefundOrder(orderID2, "", done7); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done7:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-refundSub4.Out():
		refundSub4.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	var order5 models.Order
	err = network.Nodes()[1].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID2.String()).Last(&order5).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	refunds, err = order5.Refunds()
	if err != nil {
		t.Fatal(err)
	}
	if len(refunds) != 2 {
		t.Errorf("Expected 2 refunds, got %d", len(refunds))
	}

	expectedAmount = iwallet.NewAmount(222222).Sub(iwallet.NewAmount(paymentSent.EscrowReleaseFee))
	if refunds[1].Amount != expectedAmount.String() {
		t.Errorf("Incorrect refund amount. Expected %s got %s", expectedAmount.String(), refunds[1].Amount)
	}

	select {
	case n := <-txSub6.Out():
		tx := n.(*events.TransactionReceived)
		if tx.To[0].Address.String() != paymentSent.RefundAddress {
			t.Errorf("Received funds on incorrect address. Expected %s, got %s", paymentSent.RefundAddress, tx.To[0].Address.String())
		}
		if tx.To[0].Amount.String() != expectedAmount.String() {
			t.Errorf("Incorrect refund amount. Expected %s got %s", expectedAmount.String(), tx.To[0].Amount.String())
		}
		txSub6.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
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

				return order.PutMessage(utils.MustWrapOrderMessage(orderOpen))
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

				return order.PutMessage(utils.MustWrapOrderMessage(orderOpen))
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

				return order.PutMessage(utils.MustWrapOrderMessage(orderOpen))
			},
			check: func(msg *pb.Refund) error {
				if msg.GetReleaseInfo() == nil {
					return errors.New("failed to record release info")
				}
				if msg.GetReleaseInfo().ToAddress != "abc" {
					return errors.New("incorrect refund address")
				}
				if msg.GetReleaseInfo().ToAmount != "9990" {
					return errors.New("incorrect refund amount")
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

				return order.PutMessage(utils.MustWrapOrderMessage(orderOpen))
			},
			check: func(msg *pb.Refund) error {
				if msg.GetReleaseInfo() == nil {
					return errors.New("failed to record release info")
				}
				if msg.GetReleaseInfo().ToAddress != "abc" {
					return errors.New("incorrect refund address")
				}
				if msg.GetReleaseInfo().ToAmount != "4990" {
					return errors.New("incorrect refund amount")
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
	addr, err := net.Wallets()[0].NewAddress()
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

		_, msg, err := n.buildRefundMessage(&order, net.Wallets()[0], "")
		if err != nil {
			t.Errorf("Test %d: build failed: %s", i, err)
		}

		var refundMsg pb.Refund
		if err := msg.Message.UnmarshalTo(&refundMsg); err != nil {
			t.Fatal(err)
		}

		if err := test.check(&refundMsg); err != nil {
			t.Errorf("Test %d: check failed: %s", i, err)
		}
	}
}
