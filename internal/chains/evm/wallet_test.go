package evm

import (
	"testing"

	"github.com/mobazha/mobazha/internal/chains/base"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

type stubChainClient struct{}

func (s *stubChainClient) GetTransaction(id iwallet.TransactionID, coinType iwallet.CoinType) (*iwallet.Transaction, error) {
	return nil, nil
}
func (s *stubChainClient) EstimateFee(txsize int) (map[iwallet.FeeLevel]iwallet.EstimateFeeRes, error) {
	return nil, nil
}
func (s *stubChainClient) Broadcast(serializedTx []byte) error { return nil }

func TestETHWallet_SetChainClient_DefaultsToBorrowed(t *testing.T) {
	wallet, err := NewETHWallet(iwallet.CtMock, nil, &base.WalletConfig{})
	if err != nil {
		t.Fatalf("NewETHWallet failed: %v", err)
	}
	client := &stubChainClient{}

	wallet.SetChainClient(client)

	if wallet.ChainClient != client {
		t.Fatal("expected chain client to be injected")
	}
	if wallet.ownsChainClient {
		t.Fatal("SetChainClient should default to borrowed ownership")
	}
}

func TestETHWallet_SetChainClientWithOwnership_TracksOwnership(t *testing.T) {
	wallet, err := NewETHWallet(iwallet.CtMock, nil, &base.WalletConfig{})
	if err != nil {
		t.Fatalf("NewETHWallet failed: %v", err)
	}
	client := &stubChainClient{}

	wallet.SetChainClientWithOwnership(client, true)
	if !wallet.ownsChainClient {
		t.Fatal("expected owned chain client")
	}

	wallet.SetChainClientWithOwnership(client, false)
	if wallet.ownsChainClient {
		t.Fatal("expected borrowed chain client")
	}
}
