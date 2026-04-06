package core

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/models/factory"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestMobazhaNode_DeclineOrder(t *testing.T) {
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
	orderID, _, err := network.Nodes()[1].Order().PurchaseListing(context.Background(), purchase)
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

	declineSub, err := network.Nodes()[1].eventBus.Subscribe(&events.OrderDeclined{})
	if err != nil {
		t.Fatal(err)
	}

	declineAck, err := network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done4 := make(chan struct{})
	if err := network.Nodes()[0].Order().DeclineOrder(orderID, iwallet.TransactionID(""), "sucks to be you", done4); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done4:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-declineSub.Out():
		declineSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
	select {
	case <-declineAck.Out():
		declineAck.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	err = network.Nodes()[0].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order.SerializedOrderDecline == nil {
		t.Error("Node 0 failed to save order decline")
	}
	if !order.OrderDeclineAcked {
		t.Error("Node 0 failed to save order decline ack")
	}

	err = network.Nodes()[1].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order2).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order2.SerializedOrderDecline == nil {
		t.Error("Node 1 failed to save order decline")
	}

	// Address request direct order that is funded.
	orderSub0, err = network.Nodes()[0].eventBus.Subscribe(&events.NewOrder{})
	if err != nil {
		t.Fatal(err)
	}

	orderAckSub0, err = network.Nodes()[1].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

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

	wallet0, err := network.Nodes()[0].multiwallet.WalletForCurrencyCode(iwallet.CtMock.String())
	if err != nil {
		t.Fatal(err)
	}

	addr0, err := wallet0.(*wallet.MockWallet).CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}

	wallet1, err := network.Nodes()[1].multiwallet.WalletForCurrencyCode(iwallet.CtMock.String())
	if err != nil {
		t.Fatal(err)
	}

	addr1, err := wallet1.(*wallet.MockWallet).CurrentAddress()
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

	if err := network.WalletNetwork().GenerateToAddress(addr0, iwallet.NewAmount(100000000000)); err != nil {
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

	txSub1, err = network.Nodes()[1].eventBus.Subscribe(&events.TransactionReceived{})
	if err != nil {
		t.Fatal(err)
	}

	declineSub, err = network.Nodes()[1].eventBus.Subscribe(&events.OrderDeclined{})
	if err != nil {
		t.Fatal(err)
	}

	declineAck, err = network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	refundSub, err := network.Nodes()[1].eventBus.Subscribe(&events.Refund{})
	if err != nil {
		t.Fatal(err)
	}

	done4 = make(chan struct{})
	if err := network.Nodes()[0].Order().DeclineOrder(orderID, iwallet.TransactionID(""), "sucks to be you", done4); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done4:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-declineSub.Out():
		declineSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
	select {
	case <-declineAck.Out():
		declineAck.Close()
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
	case <-txSub1.Out():
		txSub1.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	var order3 models.Order
	err = network.Nodes()[1].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order3).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	refunds, err := order3.Refunds()
	if err != nil {
		t.Fatal(err)
	}
	if len(refunds) != 1 {
		t.Errorf("Expected 1 refund, got %d", len(refunds))
	}

	_, err = wallet1.GetTransaction(iwallet.TransactionID(refunds[0].GetTransactionID()), iwallet.CtMock)
	if err != nil {
		t.Errorf("Error loading refund transaction: %s", err)
	}

	// Moderated order that is funded.
	orderSub0, err = network.Nodes()[0].eventBus.Subscribe(&events.NewOrder{})
	if err != nil {
		t.Fatal(err)
	}

	orderAckSub0, err = network.Nodes()[1].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	orderID, paymentAmount, err = network.Nodes()[1].Order().PurchaseListing(context.Background(), purchase)
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

	fundingSub2, err := network.Nodes()[0].eventBus.Subscribe(&events.OrderFunded{})
	if err != nil {
		t.Fatal(err)
	}
	fundingSub3, err := network.Nodes()[1].eventBus.Subscribe(&events.OrderPaymentReceived{})
	if err != nil {
		t.Fatal(err)
	}

	paymentData = &models.PaymentData{
		OrderID:   orderID.String(),
		Method:    pb.PaymentSent_MODERATED,
		Moderator: network.Nodes()[2].Identity().String(),
		Amount:    paymentAmount.Amount.Uint64(),
		Coin:      iwallet.CoinType(paymentAmount.Currency.String()),
		ToAddress: "abcd",
	}
	err = network.Nodes()[1].Order().ProcessOrderPayment(context.Background(), paymentData)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-fundingSub2.Out():
		fundingSub2.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
	select {
	case <-fundingSub3.Out():
		fundingSub3.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	txSub3, err := network.Nodes()[1].eventBus.Subscribe(&events.TransactionReceived{})
	if err != nil {
		t.Fatal(err)
	}

	declineSub, err = network.Nodes()[1].eventBus.Subscribe(&events.OrderDeclined{})
	if err != nil {
		t.Fatal(err)
	}

	declineAck, err = network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	refundSub, err = network.Nodes()[1].eventBus.Subscribe(&events.Refund{})
	if err != nil {
		t.Fatal(err)
	}

	done4 = make(chan struct{})
	if err := network.Nodes()[0].Order().DeclineOrder(orderID, iwallet.TransactionID(""), "sucks to be you", done4); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done4:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-declineSub.Out():
		declineSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
	select {
	case <-declineAck.Out():
		declineAck.Close()
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
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
}
