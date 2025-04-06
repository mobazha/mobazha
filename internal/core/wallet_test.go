package core

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/events"
	"github.com/mobazha/mobazha3.0/internal/models"
	"github.com/mobazha/mobazha3.0/internal/models/factory"
	iwallet "github.com/mobazha/mobazha3.0/internal/multiwallet/wallet-interface"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
)

func TestOpenBazaarNode_SaveAndGetTransactionMetadata(t *testing.T) {
	mockNode, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	orderOpen, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	var order models.Order
	order.ID = models.OrderID("123")
	order.PaymentAddress = orderOpen.Payment.Address
	if err := order.PutMessage(utils.MustWrapOrderMessage(orderOpen)); err != nil {
		t.Fatal(err)
	}

	err = mockNode.repo.DB().Update(func(tx database.Tx) error {
		return tx.Save(&order)
	})
	if err != nil {
		t.Fatal(err)
	}

	memo := "If taxation without consent is not robbery, then any band of robbers have only to declare themselves a government, and all their robberies are legalized."
	err = mockNode.SaveTransactionMetadata(&models.TransactionMetadata{
		PaymentAddress: orderOpen.Payment.Address,
		Memo:           memo,
		Txid:           "abc",
	})
	if err != nil {
		t.Fatal(err)
	}

	metadata, err := mockNode.GetTransactionMetadata(iwallet.TransactionID("abc"))
	if err != nil {
		t.Fatal(err)
	}

	if metadata.Txid != "abc" {
		t.Errorf("Expected txid of abc, got %s", metadata.Txid)
	}
	if metadata.Memo != memo {
		t.Errorf("Expected memo of %s, got %s", memo, metadata.Memo)
	}
	if metadata.PaymentAddress != orderOpen.Payment.Address {
		t.Errorf("Expected payment address of %s, got %s", orderOpen.Payment.Address, metadata.PaymentAddress)
	}
	if metadata.OrderID.String() != order.ID.String() {
		t.Errorf("Expected order ID of %s, got %s", order.ID.String(), metadata.OrderID.String())
	}
	if metadata.Thumbnail != orderOpen.Listings[0].Listing.Item.Images[0].Tiny {
		t.Errorf("Expected thumbnail of %s, got %s", orderOpen.Listings[0].Listing.Item.Images[0].Tiny, metadata.Thumbnail)
	}
}

func TestTransactionReceivedMarshal(t *testing.T) {
	event := events.TransactionReceived{
		Transaction:  iwallet.Transaction{},
		CurrencyCode: "LTC",
	}
	eventJson, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	err = checkJsonContainsCurrencyCode(eventJson, event.CurrencyCode)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSpendFromPaymentAddressMarshal(t *testing.T) {
	event := events.SpendFromPaymentAddress{
		Transaction:  iwallet.Transaction{},
		CurrencyCode: "LTC",
	}
	eventJson, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	err = checkJsonContainsCurrencyCode(eventJson, event.CurrencyCode)
	if err != nil {
		t.Fatal(err)
	}
}

func checkJsonContainsCurrencyCode(inputJson []byte, currencyCode string) error {
	currencyCodeStruct := struct {
		CurrencyCode string
	}{}

	err := json.Unmarshal(inputJson, &currencyCodeStruct)
	if err != nil {
		return err
	}

	if currencyCodeStruct.CurrencyCode != "LTC" {
		return errors.New("Expected CurrencyCode is not found in inputJson")
	}
	return nil
}
