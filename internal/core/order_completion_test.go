package core

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/models/factory"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestMobazhaNode_CompleteOrder(t *testing.T) {
	network, err := NewMocknet(3)
	if err != nil {
		t.Fatal(err)
	}

	defer network.TearDown()

	go network.StartWalletNetwork()

	for _, node := range network.Nodes() {
		go node.orderProcessor.Start()
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

	done1 := make(chan struct{})
	if err := network.Nodes()[1].Profile().SetProfile(&models.Profile{Name: "Buyer"}, done1); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done1:
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

	txSub1, err := network.Nodes()[1].eventBus.Subscribe(&events.TransactionReceived{})
	if err != nil {
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

	ratingSigAck, err := network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	paymentData := &models.PaymentData{
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

	fulfillSub, err := network.Nodes()[1].eventBus.Subscribe(&events.OrderFulfillment{})
	if err != nil {
		t.Fatal(err)
	}

	fulfillAck, err := network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done5 := make(chan struct{})
	fulfillments := []models.Fulfillment{
		{
			ItemIndex: 0,
			PhysicalDelivery: &models.PhysicalDelivery{
				TrackingNumber: "1234",
				Shipper:        "UPS",
			},
		},
	}
	if err := network.Nodes()[0].Order().FulfillOrder(orderID, fulfillments, done5); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done5:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-fulfillSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
	select {
	case <-fulfillAck.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	completeSub, err := network.Nodes()[0].eventBus.Subscribe(&events.OrderCompletion{})
	if err != nil {
		t.Fatal(err)
	}

	completeAck, err := network.Nodes()[1].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done6 := make(chan struct{})
	ratings := []models.Rating{
		{
			Description:     5,
			DeliverySpeed:   5,
			CustomerService: 5,
			Quality:         5,
			Overall:         5,
			Review:          "Excellent",
		},
	}
	if err := network.Nodes()[1].Order().CompleteOrder(orderID, iwallet.TransactionID(""), ratings, true, done6); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done6:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-completeSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
	select {
	case <-completeAck.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	var order1 models.Order
	err = network.Nodes()[1].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order1).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	complete, err := order1.OrderCompleteMessage()
	if err != nil {
		t.Fatal(err)
	}
	if complete.Ratings[0].Overall != uint32(ratings[0].Overall) {
		t.Errorf("Expected rating %d got %d", uint32(ratings[0].Overall), complete.Ratings[0].Overall)
	}
	if complete.Ratings[0].Quality != uint32(ratings[0].Quality) {
		t.Errorf("Expected rating %d got %d", uint32(ratings[0].Quality), complete.Ratings[0].Quality)
	}
	if complete.Ratings[0].CustomerService != uint32(ratings[0].CustomerService) {
		t.Errorf("Expected rating %d got %d", uint32(ratings[0].CustomerService), complete.Ratings[0].CustomerService)
	}
	if complete.Ratings[0].DeliverySpeed != uint32(ratings[0].DeliverySpeed) {
		t.Errorf("Expected rating %d got %d", uint32(ratings[0].DeliverySpeed), complete.Ratings[0].DeliverySpeed)
	}
	if complete.Ratings[0].Description != uint32(ratings[0].Description) {
		t.Errorf("Expected rating %d got %d", uint32(ratings[0].Description), complete.Ratings[0].Description)
	}
	if complete.Ratings[0].Review != ratings[0].Review {
		t.Errorf("Expected review %s got %s", ratings[0].Review, complete.Ratings[0].Review)
	}

	var order2 models.Order
	err = network.Nodes()[0].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order2).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	complete, err = order2.OrderCompleteMessage()
	if err != nil {
		t.Fatal(err)
	}

	wallet0, err := network.Nodes()[0].multiwallet.WalletForCurrencyCode(iwallet.CtMock.String())
	if err != nil {
		t.Fatal(err)
	}

	network.WalletNetwork().GenerateBlock()

	unconf0, conf0, err := wallet0.Balance()
	if err != nil {
		t.Fatal(err)
	}

	balance := unconf0.Add(conf0)

	if balance.Cmp(iwallet.NewAmount(0)) <= 0 {
		t.Errorf("Expected positive balance, got zero")
	}

	///////////////////////////
	// Now repeat everything with a moderated order and make sure funds release
	//////////////////////////

	orderSub0, err = network.Nodes()[0].eventBus.Subscribe(&events.NewOrder{})
	if err != nil {
		t.Fatal(err)
	}
	orderAckSub0, err = network.Nodes()[1].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	purchase = factory.NewPurchase()
	purchase.Items[0].ListingHash = index[0].CID

	// Address request direct order
	orderID, paymentAmount, err = network.Nodes()[1].Order().PurchaseListing(context.Background(), purchase)
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

	confirmSub, err = network.Nodes()[1].eventBus.Subscribe(&events.OrderConfirmation{})
	if err != nil {
		t.Fatal(err)
	}

	confirmAck, err = network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done4 = make(chan struct{})
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

	fundingSub0, err = network.Nodes()[0].eventBus.Subscribe(&events.OrderFunded{})
	if err != nil {
		t.Fatal(err)
	}

	fundingSub1, err = network.Nodes()[1].eventBus.Subscribe(&events.OrderPaymentReceived{})
	if err != nil {
		t.Fatal(err)
	}

	ratingSigAck, err = network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
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

	fulfillSub, err = network.Nodes()[1].eventBus.Subscribe(&events.OrderFulfillment{})
	if err != nil {
		t.Fatal(err)
	}

	fulfillAck, err = network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done5 = make(chan struct{})
	fulfillments = []models.Fulfillment{
		{
			ItemIndex: 0,
			PhysicalDelivery: &models.PhysicalDelivery{
				TrackingNumber: "1234",
				Shipper:        "UPS",
			},
		},
	}
	if err := network.Nodes()[0].Order().FulfillOrder(orderID, fulfillments, done5); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done5:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-fulfillSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
	select {
	case <-fulfillAck.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	completeSub, err = network.Nodes()[0].eventBus.Subscribe(&events.OrderCompletion{})
	if err != nil {
		t.Fatal(err)
	}

	completeAck, err = network.Nodes()[1].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done6 = make(chan struct{})
	ratings = []models.Rating{
		{
			Description:     5,
			DeliverySpeed:   5,
			CustomerService: 5,
			Quality:         5,
			Overall:         5,
			Review:          "Excellent",
		},
	}
	if err := network.Nodes()[1].Order().CompleteOrder(orderID, iwallet.TransactionID(""), ratings, true, done6); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done6:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-completeSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
	select {
	case <-completeAck.Out():
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

	complete, err = order3.OrderCompleteMessage()
	if err != nil {
		t.Fatal(err)
	}

	transactions, err := order3.GetTransactions()
	if err != nil {
		t.Fatal(err)
	}
	if len(transactions) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(transactions))
	}

	var order4 models.Order
	err = network.Nodes()[0].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&order4).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	complete, err = order4.OrderCompleteMessage()
	if err != nil {
		t.Fatal(err)
	}

	transactions, err = order4.GetTransactions()
	if err != nil {
		t.Fatal(err)
	}
	if len(transactions) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(transactions))
	}

	fulfillmentMsgs, err := order4.OrderFulfillmentMessages()
	if err != nil {
		t.Fatal(err)
	}

	if transactions[1].To[0].Address.String() != fulfillmentMsgs[0].ReleaseInfo.ToAddress {
		t.Errorf("Expected address %s got %s", fulfillmentMsgs[0].ReleaseInfo.ToAddress, transactions[1].To[0].Address.String())
	}

	if transactions[1].To[0].Amount.String() != fulfillmentMsgs[0].ReleaseInfo.ToAmount {
		t.Errorf("Expected amount %s got %s", fulfillmentMsgs[0].ReleaseInfo.ToAmount, transactions[1].To[0].Amount.String())
	}

	network.WalletNetwork().GenerateBlock()

	unconf, conf, err := wallet0.Balance()
	if err != nil {
		t.Fatal(err)
	}

	if unconf.Add(conf).Cmp(balance) <= 0 {
		t.Errorf("Failed to record new payout trasaction")
	}
}
