package evm

import (
	"testing"

	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

func TestChainIDForNetwork(t *testing.T) {
	t.Parallel()

	if got, ok := ChainIDForNetwork(iwallet.ChainEthereum, false); !ok || got != 1 {
		t.Fatalf("Ethereum mainnet = %d/%v, want 1/true", got, ok)
	}
	if got, ok := ChainIDForNetwork(iwallet.ChainEthereum, true); !ok || got != 11155111 {
		t.Fatalf("Ethereum testnet = %d/%v, want 11155111/true", got, ok)
	}
	if _, ok := ChainIDForNetwork(iwallet.ChainOptimism, true); ok {
		t.Fatal("Optimism testnet must fail closed without an explicit mapping")
	}
}

func TestChainTypeForID(t *testing.T) {
	t.Parallel()

	for _, chainID := range []uint64{1, 11155111} {
		if got, ok := ChainTypeForID(chainID); !ok || got != iwallet.ChainEthereum {
			t.Fatalf("ChainTypeForID(%d) = %q/%v, want ETH/true", chainID, got, ok)
		}
	}
	if _, ok := ChainTypeForID(0); ok {
		t.Fatal("ChainTypeForID(0) must reject an unknown domain")
	}
}
