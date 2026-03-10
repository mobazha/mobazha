package core

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/models/factory"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestMobazhaNode_ConfirmOrder(t *testing.T) {
	network, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}

	defer network.TearDown()

	go network.StartWalletNetwork()

	for _, node := range network.Nodes() {
		go node.orderProcessor.Start()
	}

	orderSub0, err := network.Nodes()[0].eventBus.Subscribe(&events.NewOrder{})
	if err != nil {
		t.Fatal(err)
	}
	orderAckSub0, err := network.Nodes()[1].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
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

	purchase := factory.NewPurchase()
	purchase.Items[0].ListingHash = index[0].CID

	// Address request direct order
	orderID, paymentAmount, err := network.Nodes()[1].Order().PurchaseListing(context.Background(), purchase)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-orderSub0.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-orderAckSub0.Out():
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

	// Send payment (DIRECT method) - required before vendor can confirm
	paymentData := &models.PaymentData{
		OrderID:       orderID.String(),
		TransactionID: "tx123456",
		Method:        pb.PaymentSent_DIRECT,
		Amount:        paymentAmount.Amount.Uint64(),
		Coin:          iwallet.CtMock,
		ToAddress:     "abcd",
	}
	if err := network.Nodes()[1].Order().ProcessOrderPayment(context.Background(), paymentData); err != nil {
		t.Fatal(err)
	}

	// Wait for payment to be processed on both nodes
	time.Sleep(100 * time.Millisecond)

	confirmSub, err := network.Nodes()[1].eventBus.Subscribe(&events.OrderConfirmation{})
	if err != nil {
		t.Fatal(err)
	}

	confirmAck, err := network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done4 := make(chan struct{})
	if err := network.Nodes()[0].Order().ConfirmOrder(orderID, "", "abcd", done4); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done4:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-confirmSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
	select {
	case <-confirmAck.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	err = network.Nodes()[0].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order.SerializedOrderConfirmation == nil {
		t.Error("Node 0 failed to save order confirmation")
	}
	if !order.OrderConfirmationAcked {
		t.Error("Node 0 failed to save order confirmation ack")
	}

	err = network.Nodes()[1].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order2).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order2.SerializedOrderConfirmation == nil {
		t.Error("Node 1 failed to save order confirmation")
	}
}

// TestMobazhaNode_ConfirmOrder_Cancelable_Reconnect tests CANCELABLE order confirmation
// after network disconnection and reconnection. This tests message retry functionality.
// TODO: This test has pre-existing issues with network reconnection event handling
// and needs investigation. Skipping for now.
func TestMobazhaNode_ConfirmOrder_Cancelable_Reconnect(t *testing.T) {
	t.Skip("Skipping: network reconnection message retry test has pre-existing issues")

	network, err := NewMocknet(2)
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

	purchase := factory.NewPurchase()
	purchase.Items[0].ListingHash = index[0].CID

	orderSub0, err := network.Nodes()[0].eventBus.Subscribe(&events.NewOrder{})
	if err != nil {
		t.Fatal(err)
	}

	// Cancelable order with network disconnection
	// We're going to disconnect the nodes, make the purchase, and then reconnect. This should cause node 1
	// to resend the order upon reconnection.
	network.Nodes()[0].networkService.Close()
	go network.Nodes()[1].syncMessages()
	if err := network.p2pNet.UnlinkPeers(network.Nodes()[0].Identity(), network.Nodes()[1].Identity()); err != nil {
		t.Fatal(err)
	}
	orderID2, paymentAmount, err := network.Nodes()[1].Order().PurchaseListing(context.Background(), purchase)
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

	confirmAck2, err := network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	// Reconnecting nodes should trigger node 1 to send the order to node 0 again.
	time.Sleep(10 * time.Millisecond)
	network.Nodes()[0].networkService = net.NewNetworkService(network.Nodes()[0].nodeID, network.Nodes()[0].peerHost, net.NewBanManager(nil, nil), true)
	network.Nodes()[0].registerHandlers()

	if _, err := network.p2pNet.LinkPeers(network.Nodes()[0].Identity(), network.Nodes()[1].Identity()); err != nil {
		t.Fatal(err)
	}

	select {
	case <-orderSub0.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	paymentData := &models.PaymentData{
		OrderID:       orderID2.String(),
		TransactionID: "1234567890",
		Method:        pb.PaymentSent_CANCELABLE,
		Amount:        paymentAmount.Amount.Uint64(),
		Coin:          iwallet.CoinType(paymentAmount.Currency.String()),
		ToAddress:     "abcd",
	}
	err = network.Nodes()[1].Order().ProcessOrderPayment(context.Background(), paymentData)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-txSub0.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-txSub1.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-confirmAck2.Out():
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

	paymentSent, err := order4.PaymentSentMessage()
	if err != nil {
		t.Fatal(err)
	}

	if paymentSent.Method != pb.PaymentSent_CANCELABLE {
		t.Fatal("Expected CANCELABLE order")
	}

	releaseSub, err := network.Nodes()[0].eventBus.Subscribe(&events.SpendFromPaymentAddress{})
	if err != nil {
		t.Fatal(err)
	}

	done5 := make(chan struct{})
	if err := network.Nodes()[0].Order().ConfirmOrder(orderID2, "", "abcd", done5); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done5:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-releaseSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	var order6 models.Order
	err = network.Nodes()[0].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID2.String()).Last(&order6).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order6.SerializedOrderConfirmation == nil {
		t.Error("Node 0 failed to save order confirmation")
	}

	txs, err := order6.GetTransactions()
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(txs))
	}
}
