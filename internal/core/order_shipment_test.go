package core

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/models/factory"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestMobazhaNode_ShipOrder(t *testing.T) {
	network, err := NewMocknet(3)
	if err != nil {
		t.Fatal(err)
	}

	defer network.TearDown()

	go network.StartWalletNetwork()

	for _, node := range network.Nodes() {
		go node.orderProcessor.Start()
	}

	setupMockReceivingAccounts(t, network.Nodes())

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

	orderID, _, err := network.Nodes()[1].Order().PurchaseListing(context.Background(), purchase)
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

	fundingSub0, err := network.Nodes()[0].eventBus.Subscribe(&events.OrderFunded{})
	if err != nil {
		t.Fatal(err)
	}

	fundingSub1, err := network.Nodes()[1].eventBus.Subscribe(&events.OrderPaymentReceived{})
	if err != nil {
		t.Fatal(err)
	}

	ratingSigAck, err := network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	paymentData, err := network.Nodes()[1].Wallet().GetUTXOPaymentInfo(
		context.Background(),
		orderID.String(),
		"",
		iwallet.CtMock,
	)
	if err != nil {
		t.Fatalf("GetUTXOPaymentInfo failed: %v", err)
	}
	processMockUTXOPayment(t, network.Nodes()[1], paymentData, network.Nodes()[0])

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

	select {
	case <-ratingSigAck.Out():
		ratingSigAck.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
	ensureMockUTXOFundingFacts(t, orderID, paymentData, network.Nodes()...)

	confirmSub, err := network.Nodes()[1].eventBus.Subscribe(&events.OrderConfirmation{})
	if err != nil {
		t.Fatal(err)
	}

	confirmAck, err := network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	mockPayoutAddr := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	done4 := make(chan struct{})
	if err := network.Nodes()[0].Order().ConfirmOrder(orderID, "", mockPayoutAddr, done4); err != nil {
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

	var order3 models.Order
	err = network.Nodes()[0].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order3).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order3.SerializedOrderConfirmation == nil {
		t.Error("Node 0 failed to save order confirmation")
	}
	if !order3.OrderConfirmationAcked {
		t.Error("Node 0 failed to save order confirmation ack")
	}

	var order4 models.Order
	err = network.Nodes()[1].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order4).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order4.SerializedOrderConfirmation == nil {
		t.Error("Node 1 failed to save order confirmation")
	}

	shipSub, err := network.Nodes()[1].eventBus.Subscribe(&events.OrderShipment{})
	if err != nil {
		t.Fatal(err)
	}

	shipAck, err := network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done5 := make(chan struct{})
	shipments := []models.Shipment{
		{
			ItemIndex: 0,
			PhysicalDelivery: &models.PhysicalDelivery{
				TrackingNumber: "1234",
				Shipper:        "UPS",
			},
		},
	}
	if err := network.Nodes()[0].Order().ShipOrder(orderID, shipments, done5); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done5:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-shipSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
	select {
	case <-shipAck.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	var order5 models.Order
	err = network.Nodes()[0].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order5).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order5.SerializedOrderShipments == nil {
		t.Error("Node 0 failed to save order shipment payload")
	}
	if !order5.OrderShipmentAcked {
		t.Error("Node 0 failed to save order shipment ack")
	}

	var order6 models.Order
	err = network.Nodes()[1].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order6).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order6.SerializedOrderShipments == nil {
		t.Error("Node 1 failed to save order shipment payload")
	}

	shipmentMessages, err := order6.OrderShipmentMessages()
	if err != nil {
		t.Fatal(err)
	}

	if len(shipmentMessages) != 1 {
		t.Errorf("Expected 1 saved shipment message, got %d", len(shipmentMessages))
	}

	if len(shipmentMessages[0].Shipments) != 1 {
		t.Errorf("Expected 1 saved shipment line, got %d", len(shipmentMessages[0].Shipments))
	}

	if shipmentMessages[0].Shipments[0].ItemIndex != 0 {
		t.Errorf("Expected item index of 0 got %d", shipmentMessages[0].Shipments[0].ItemIndex)
	}

	if shipmentMessages[0].Shipments[0].GetPhysicalDelivery().Shipper != "UPS" {
		t.Errorf("Expected shipper of UPS got %s", shipmentMessages[0].Shipments[0].GetPhysicalDelivery().Shipper)
	}

	if shipmentMessages[0].Shipments[0].GetPhysicalDelivery().TrackingNumber != "1234" {
		t.Errorf("Expected tracking number of 1234 got %s", shipmentMessages[0].Shipments[0].GetPhysicalDelivery().TrackingNumber)
	}

	if shipmentMessages[0].ReleaseInfo != nil {
		t.Error("Expected nil ReleaseInfo for direct (non-moderated) order")
	}
}
