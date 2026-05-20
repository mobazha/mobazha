package evm

import (
	"testing"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// TestGetDefaultConfigs_TestnetSubsetSmallerThanMainnet verifies testnet mode only
// registers chains with explicit testnet RPC URLs (ManagedEscrow runtime subset guard).
func TestGetDefaultConfigs_TestnetSubsetSmallerThanMainnet(t *testing.T) {
	mainnet := GetDefaultConfigs(false)
	testnet := GetDefaultConfigs(true)
	if len(mainnet) == 0 {
		t.Fatal("GetDefaultConfigs(testnet=false) returned no configs")
	}
	if len(testnet) == 0 {
		t.Fatal("GetDefaultConfigs(testnet=true) returned no configs")
	}
	if len(testnet) >= len(mainnet) {
		t.Fatalf("testnet chain count %d should be smaller than mainnet %d", len(testnet), len(mainnet))
	}

	expectedTestnet := map[iwallet.ChainType]bool{
		iwallet.ChainBSC: false, iwallet.ChainEthereum: false,
		iwallet.ChainPolygon: false, iwallet.ChainBase: false,
		iwallet.ChainConflux: false,
	}
	for _, cfg := range testnet {
		if !cfg.Testnet {
			t.Errorf("chain %s: Testnet flag should be true", cfg.ChainType)
		}
		if cfg.RpcURL == "" {
			t.Errorf("chain %s: RpcURL should not be empty", cfg.ChainType)
		}
		expectedTestnet[cfg.ChainType] = true
	}
	for chain, found := range expectedTestnet {
		if !found {
			t.Errorf("expected testnet chain %s missing from GetDefaultConfigs(true)", chain)
		}
	}
}
