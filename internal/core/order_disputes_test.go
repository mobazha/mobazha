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

func TestMobazhaNode_Dispute(t *testing.T) {
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
	if err := network.Nodes()[2].SetProfile(&models.Profile{Name: "Ron Paul"}, done2); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done2:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	done1 := make(chan struct{})
	if err := network.Nodes()[1].SetProfile(&models.Profile{Name: "Buyer"}, done1); err != nil {
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
	if err := network.Nodes()[2].SetSelfAsModerator(context.Background(), modInfo, done3); err != nil {
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

	purchase := factory.NewPurchase()
	purchase.Items[0].ListingHash = index[0].CID

	// Address request direct order
	orderID, paymentAmount, err := network.Nodes()[1].PurchaseListing(context.Background(), purchase)
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
	if err := network.Nodes()[0].ConfirmOrder(orderID, "", "abcd", done4); err != nil {
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

	select {
	case <-ratingSigAck.Out():
		ratingSigAck.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	caseOpenSub, err := network.Nodes()[2].eventBus.Subscribe(&events.CaseOpen{})
	if err != nil {
		t.Fatal(err)
	}
	caseUpdateSub, err := network.Nodes()[2].eventBus.Subscribe(&events.CaseUpdate{})
	if err != nil {
		t.Fatal(err)
	}

	caseUpdateAckSub, err := network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	disputeOpenSub, err := network.Nodes()[0].eventBus.Subscribe(&events.DisputeOpen{})
	if err != nil {
		t.Fatal(err)
	}

	disputeOpenAckModeratorSub, err := network.Nodes()[1].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	disputeOpenAckVendorSub, err := network.Nodes()[1].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done5 := make(chan struct{})
	if err := network.Nodes()[1].OpenDispute(orderID, "Got scammed", done5); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done5:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-disputeOpenSub.Out():
		disputeOpenSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-caseOpenSub.Out():
		caseOpenSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-caseUpdateSub.Out():
		caseUpdateSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-disputeOpenAckModeratorSub.Out():
		disputeOpenAckModeratorSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-disputeOpenAckVendorSub.Out():
		disputeOpenAckVendorSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-caseUpdateAckSub.Out():
		caseUpdateAckSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	var order1 models.Order
	err = network.Nodes()[1].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id =?", orderID.String()).First(&order1).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order1.SerializedDisputeOpen == nil {
		t.Error("Buyer dispute open is nil")
	}
	if order1.DisputeOpenOtherPartyAcked == false {
		t.Error("Buyer dispute open other party ack is false")
	}
	if order1.DisputeOpenModeratorAcked == false {
		t.Error("Buyer dispute open moderator ack is false")
	}

	var order0 models.Order
	err = network.Nodes()[0].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id =?", orderID.String()).First(&order0).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order0.SerializedDisputeOpen == nil {
		t.Error("Vendor dispute open is nil")
	}
	if order0.SerializedDisputeUpdate == nil {
		t.Error("Vendor dispute update is nil")
	}
	if order0.DisputeUpdateAcked == false {
		t.Error("Vendor dispute update ack is false")
	}

	var case2 models.Case
	err = network.Nodes()[2].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&case2).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if case2.SerializedDisputeOpen == nil {
		t.Error("Moderator dispute open is nil")
	}
	if case2.SerializedBuyerContract == nil {
		t.Error("Moderation buyer contract is nil")
	}
	if case2.SerializedVendorContract == nil {
		t.Error("Moderator vendor contract is nil")
	}

	disputeCloseBuyerSub, err := network.Nodes()[1].eventBus.Subscribe(&events.DisputeClose{})
	if err != nil {
		t.Fatal(err)
	}
	disputeCloseVendorSub, err := network.Nodes()[0].eventBus.Subscribe(&events.DisputeClose{})
	if err != nil {
		t.Fatal(err)
	}

	disputeCloseAckSub, err := network.Nodes()[2].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done6 := make(chan struct{})
	if err := network.Nodes()[2].CloseDispute(orderID, 50, 50, "Resolve dispute", done6); err != nil {
		t.Fatal(err)
	}

	select {
	case <-disputeCloseBuyerSub.Out():
		disputeCloseBuyerSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-disputeCloseVendorSub.Out():
		disputeCloseVendorSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-disputeCloseAckSub.Out():
		disputeCloseAckSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	err = network.Nodes()[0].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id =?", orderID.String()).First(&order0).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	disputeClose0, err := order0.DisputeClosedMessage()
	if err != nil {
		t.Error("Vendor dispute close is nil")
	}
	if len(disputeClose0.ReleaseInfo.Outpoints) == 0 {
		t.Error("No outpoint in release info")
	}
	if len(disputeClose0.ReleaseInfo.EscrowSignatures) == 0 {
		t.Error("No moderator signature in release info")
	}

	disputeAcceptSub, err := network.Nodes()[0].eventBus.Subscribe(&events.DisputeAccepted{})
	if err != nil {
		t.Fatal(err)
	}

	disputeAcceptAckSub, err := network.Nodes()[1].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done7 := make(chan struct{})
	if err := network.Nodes()[1].ReleaseFunds(orderID, iwallet.TransactionID(""), done7); err != nil {
		t.Fatal(err)
	}

	select {
	case <-disputeAcceptSub.Out():
		disputeAcceptSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-disputeAcceptAckSub.Out():
		disputeAcceptAckSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	network.WalletNetwork().GenerateBlock()

	releaseInfo := disputeClose0.ReleaseInfo

	balance0, err := getNodeTotalBalance(network, 0)
	if err != nil {
		t.Fatal(err)
	}

	if balance0.String() != releaseInfo.VendorAmount {
		t.Errorf("Expected vendor balance")
	}

	balance2, err := getNodeTotalBalance(network, 2)
	if err != nil {
		t.Fatal(err)
	}

	if balance2.String() != releaseInfo.ModeratorAmount {
		t.Errorf("Expected moderator balance")
	}

	balance1, err := getNodeTotalBalance(network, 1)
	if err != nil {
		t.Fatal(err)
	}

	fee := iwallet.NewAmount(releaseInfo.TransactionFee)
	total := balance0.Add(balance1).Add(balance2).Add(fee)

	// buyer spent one other fee when do purchase
	t.Logf("Balance, total: %s, buyer: %s, vendor: %s, moderator: %s, fee: %s", total, balance1, balance0, balance2, releaseInfo.TransactionFee)
}

func getNodeTotalBalance(network *Mocknet, index int) (iwallet.Amount, error) {
	wallet, err := network.Nodes()[index].multiwallet.WalletForCurrencyCode(iwallet.CtMock.String())
	if err != nil {
		return iwallet.NewAmount(0), err
	}

	unconf, conf, err := wallet.Balance()
	if err != nil {
		return iwallet.NewAmount(0), err
	}

	return unconf.Add(conf), nil
}

func TestMobazhaNode_ReleaseFundsAfterTimeout(t *testing.T) {
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
	if err := network.Nodes()[2].SetProfile(&models.Profile{Name: "Ron Paul"}, done2); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done2:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	done1 := make(chan struct{})
	if err := network.Nodes()[1].SetProfile(&models.Profile{Name: "Buyer"}, done1); err != nil {
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
	if err := network.Nodes()[2].SetSelfAsModerator(context.Background(), modInfo, done3); err != nil {
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
	network.Nodes()[0].testnet = true
	listing.Metadata.EscrowTimeoutHours = 1

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

	purchase := factory.NewPurchase()
	purchase.Items[0].ListingHash = index[0].CID

	// Address request direct order
	orderID, paymentAmount, err := network.Nodes()[1].PurchaseListing(context.Background(), purchase)
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
	if err := network.Nodes()[0].ConfirmOrder(orderID, "", "abcd", done4); err != nil {
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

	select {
	case <-ratingSigAck.Out():
		ratingSigAck.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	caseOpenSub, err := network.Nodes()[2].eventBus.Subscribe(&events.CaseOpen{})
	if err != nil {
		t.Fatal(err)
	}
	caseUpdateSub, err := network.Nodes()[2].eventBus.Subscribe(&events.CaseUpdate{})
	if err != nil {
		t.Fatal(err)
	}

	caseUpdateAckSub, err := network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	disputeOpenSub, err := network.Nodes()[0].eventBus.Subscribe(&events.DisputeOpen{})
	if err != nil {
		t.Fatal(err)
	}

	disputeOpenAckModeratorSub, err := network.Nodes()[1].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	disputeOpenAckVendorSub, err := network.Nodes()[1].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done5 := make(chan struct{})
	if err := network.Nodes()[1].OpenDispute(orderID, "Got scammed", done5); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done5:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-disputeOpenSub.Out():
		disputeOpenSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-caseOpenSub.Out():
		caseOpenSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-caseUpdateSub.Out():
		caseUpdateSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-disputeOpenAckModeratorSub.Out():
		disputeOpenAckModeratorSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-disputeOpenAckVendorSub.Out():
		disputeOpenAckVendorSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-caseUpdateAckSub.Out():
		caseUpdateAckSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	var order1 models.Order
	err = network.Nodes()[1].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id =?", orderID.String()).First(&order1).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order1.SerializedDisputeOpen == nil {
		t.Error("Buyer dispute open is nil")
	}
	if order1.DisputeOpenOtherPartyAcked == false {
		t.Error("Buyer dispute open other party ack is false")
	}
	if order1.DisputeOpenModeratorAcked == false {
		t.Error("Buyer dispute open moderator ack is false")
	}

	var order0 models.Order
	err = network.Nodes()[0].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id =?", orderID.String()).First(&order0).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if order0.SerializedDisputeOpen == nil {
		t.Error("Vendor dispute open is nil")
	}
	if order0.SerializedDisputeUpdate == nil {
		t.Error("Vendor dispute update is nil")
	}
	if order0.DisputeUpdateAcked == false {
		t.Error("Vendor dispute update ack is false")
	}

	paymentFinalizeSub, err := network.Nodes()[1].eventBus.Subscribe(&events.VendorFinalizedPayment{})
	if err != nil {
		t.Fatal(err)
	}

	paymentFinalizeAckSub, err := network.Nodes()[0].eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	// CAUTION: Haven't found a good way to advance time in go test, go-mpatch didn't work due to permission
	// mock clock from github.com/benbjohnson/clock need check source. To run this test, we need set a breakpoint here,
	// and manually change system time.
	done6 := make(chan struct{})
	if err := network.Nodes()[0].ReleaseFundsAfterTimeout(orderID, done6); err != nil {
		t.Fatal(err)
	}

	select {
	case <-paymentFinalizeSub.Out():
		paymentFinalizeSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-paymentFinalizeAckSub.Out():
		paymentFinalizeAckSub.Close()
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	network.WalletNetwork().GenerateBlock()

	balance0, err := getNodeTotalBalance(network, 0)
	if err != nil {
		t.Fatal(err)
	}
	balance1, err := getNodeTotalBalance(network, 1)
	if err != nil {
		t.Fatal(err)
	}
	balance2, err := getNodeTotalBalance(network, 2)
	if err != nil {
		t.Fatal(err)
	}

	if balance2.Cmp(iwallet.NewAmount(0)) != 0 {
		t.Error("Moderator amount is not 0")
	}

	t.Logf("Balance, buyer: %s, vendor: %s, moderator: %s", balance1, balance0, balance2)
}
