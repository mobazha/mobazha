package core

import (
	"context"
	"testing"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestMobazhaNode_RequestAddress(t *testing.T) {
	network, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}
	defer network.TearDown()

	address, err := network.Nodes()[0].RequestAddress(context.Background(), network.Nodes()[1].Identity(), iwallet.CtMock)
	if err != nil {
		t.Fatal(err)
	}

	if address.CoinType() != iwallet.CtMock {
		t.Errorf("Incorrect cointype expected MCK got %s", address.CoinType().CurrencyCode())
	}
	if len(address.String()) != 40 {
		t.Errorf("Expected address length of 20 got %d", len(address.String()))
	}

	_, err = network.Nodes()[0].RequestAddress(context.Background(), network.Nodes()[1].Identity(), iwallet.CtBitcoin)
	if err == nil {
		t.Error("Expected request for unknown cointype to error")
	}
}
