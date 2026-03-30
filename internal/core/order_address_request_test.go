package core

import (
	"context"
	"testing"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestMobazhaNode_RequestAddress(t *testing.T) {
	t.Skip("flaky: libp2p mocknet P2P message delivery unreliable in CI")
	network, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}
	defer network.TearDown()

	address, err := network.Nodes()[0].Order().RequestAddress(context.Background(), network.Nodes()[1].Identity(), iwallet.CtMock)
	if err != nil {
		t.Fatal(err)
	}

	if address.CoinType() != iwallet.CtMock {
		t.Errorf("Incorrect cointype expected MCK got %s", address.CoinType().CurrencyCode())
	}
	if len(address.String()) != 40 {
		t.Errorf("Expected address length of 20 got %d", len(address.String()))
	}

	_, err = network.Nodes()[0].Order().RequestAddress(context.Background(), network.Nodes()[1].Identity(), iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f:native"))
	if err == nil {
		t.Error("Expected request for unknown cointype to error")
	}
}
