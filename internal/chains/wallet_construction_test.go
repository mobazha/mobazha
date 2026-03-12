package chains_test

import (
	"testing"

	"github.com/mobazha/mobazha3.0/internal/chains"
	"github.com/mobazha/mobazha3.0/internal/chains/base"
	"github.com/mobazha/mobazha3.0/internal/chains/utxo/bitcoin"
	ethWal "github.com/mobazha/mobazha3.0/internal/chains/evm" // registers EVM factory via init()
	"github.com/mobazha/mobazha3.0/internal/chains/utxo/litecoin"
	"github.com/mobazha/mobazha3.0/internal/chains/solana"
	"github.com/mobazha/mobazha3.0/pkg/evm"
	pkgsolana "github.com/mobazha/mobazha3.0/pkg/solana"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ── Wallet construction tests ───────────────────────────────────────────

func TestNewBitcoinWallet_NilChainClient(t *testing.T) {
	w, err := bitcoin.NewBitcoinWallet(&base.WalletConfig{
		NodeID:  "test-node",
		Testnet: true,
	})
	if err != nil {
		t.Fatalf("NewBitcoinWallet failed: %v", err)
	}
	if w.ChainClient != nil {
		t.Error("ChainClient should be nil (Blockbook removed)")
	}
}

func TestNewLitecoinWallet_NilChainClient(t *testing.T) {
	w, err := litecoin.NewLitecoinWallet(&base.WalletConfig{
		NodeID:  "test-node",
		Testnet: true,
	})
	if err != nil {
		t.Fatalf("NewLitecoinWallet failed: %v", err)
	}
	if w.ChainClient != nil {
		t.Error("ChainClient should be nil")
	}
}

func TestNewZCashWallet_NilChainClient(t *testing.T) {
	t.Skip("ZCash wallet needs NetConfig.GetExchangeRateProviders() — skip in unit test")
}

func TestNewETHWallet_NilChainClient(t *testing.T) {
	w, err := ethWal.NewETHWallet(iwallet.CoinType(iwallet.ChainEthereum), nil, &base.WalletConfig{
		Testnet: true,
	})
	if err != nil {
		t.Fatalf("NewETHWallet with nil client failed: %v", err)
	}
	// After the nil trap fix, ChainClient should be a true nil interface
	if w.ChainClient != nil {
		t.Error("ChainClient should be nil when constructed with nil *EthClient")
	}
	// Wallet should implement ChainClientSetter for later injection
	if _, ok := iwallet.Wallet(w).(base.ChainClientSetter); !ok {
		t.Error("ETHWallet should implement ChainClientSetter")
	}
}

func TestNewSolanaWallet_NilChainClient(t *testing.T) {
	w, err := solana.NewSolanaWallet(&base.WalletConfig{
		NodeID:  "test-node",
		Testnet: true,
	})
	if err != nil {
		t.Fatalf("NewSolanaWallet failed: %v", err)
	}
	if w == nil {
		t.Fatal("wallet should not be nil")
	}
	// ChainClient should be nil at construction (injected during Start)
	if w.ChainClient != nil {
		t.Error("ChainClient should be nil at construction")
	}
}

// ── Config tests ────────────────────────────────────────────────────────

func TestDefaultConfig_HasChainAPIs(t *testing.T) {
	var cfg chains.Config
	if err := cfg.Apply(chains.Defaults); err != nil {
		t.Fatalf("Config.Apply(Defaults) failed: %v", err)
	}
	// Default config should have ChainAPIs with RPC URLs for all EVM chains.
	// Chain clients are created during Start() using configs derived from
	// ChainAPIs (via extractEVMConfigs in builder.go), with
	// pkg/evm.GetDefaultConfigs as fallback.
	for _, chain := range []iwallet.ChainType{
		iwallet.ChainBSC, iwallet.ChainEthereum, iwallet.ChainPolygon,
		iwallet.ChainBase, iwallet.ChainConflux,
	} {
		api, ok := cfg.ChainAPIs[chain]
		if !ok {
			t.Errorf("chain %s: missing from ChainAPIs", chain)
			continue
		}
		if len(api.TestnetRpc) == 0 || api.TestnetRpc[0] == "" {
			t.Errorf("chain %s: TestnetRpc should have at least one non-empty URL", chain)
		}
	}
}

func TestEscrowAddresses(t *testing.T) {
	var cfg chains.Config
	err := cfg.Apply(
		chains.Defaults,
		chains.EscrowAddresses(map[iwallet.ChainType]string{
			iwallet.ChainEthereum: "0x1234567890abcdef1234567890abcdef12345678",
			iwallet.ChainBSC:      "0xabcdef1234567890abcdef1234567890abcdef12",
		}),
	)
	if err != nil {
		t.Fatalf("Config.Apply failed: %v", err)
	}

	ethAPI := cfg.ChainAPIs[iwallet.ChainEthereum]
	if ethAPI.MainnetEscrowAddress != "0x1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("ETH MainnetEscrowAddress = %q, want 0x1234...", ethAPI.MainnetEscrowAddress)
	}

	bscAPI := cfg.ChainAPIs[iwallet.ChainBSC]
	if bscAPI.MainnetEscrowAddress != "0xabcdef1234567890abcdef1234567890abcdef12" {
		t.Errorf("BSC MainnetEscrowAddress = %q, want 0xabcdef...", bscAPI.MainnetEscrowAddress)
	}
}

// ── EVM Client Factory tests ────────────────────────────────────────────

func TestEVMClientFactory_Registration(t *testing.T) {
	if evm.DefaultFactory == nil {
		t.Fatal("DefaultFactory should be registered via init()")
	}
}

// ── Solana Client Factory tests ─────────────────────────────────────────

func TestSolanaClientFactory_Registration(t *testing.T) {
	if pkgsolana.DefaultFactory == nil {
		t.Fatal("Solana DefaultFactory should be registered via init()")
	}
}

func TestSolanaDefaultConfig(t *testing.T) {
	cfg := pkgsolana.GetDefaultConfig(true)
	if cfg == nil {
		t.Fatal("GetDefaultConfig(testnet=true) returned nil")
	}
	if cfg.RpcURL == "" {
		t.Error("RpcURL should not be empty")
	}
	if cfg.RegistryAddress == "" {
		t.Error("RegistryAddress should not be empty")
	}
}

func TestEVMDefaultConfigs(t *testing.T) {
	// evm.GetDefaultConfigs is now a standalone function in pkg/evm/defaults.go
	// (no longer depends on the factory being registered)
	configs := evm.GetDefaultConfigs(true)
	if len(configs) == 0 {
		t.Fatal("GetDefaultConfigs(testnet=true) returned no configs")
	}

	expected := map[iwallet.ChainType]bool{
		iwallet.ChainBSC: false, iwallet.ChainEthereum: false,
		iwallet.ChainPolygon: false, iwallet.ChainBase: false,
		iwallet.ChainConflux: false,
	}
	for _, cfg := range configs {
		expected[cfg.ChainType] = true
		if cfg.RpcURL == "" {
			t.Errorf("chain %s: RpcURL should not be empty", cfg.ChainType)
		}
		if cfg.RegistryAddress == "" {
			t.Errorf("chain %s: RegistryAddress should not be empty", cfg.ChainType)
		}
	}
	for chain, found := range expected {
		if !found {
			t.Errorf("chain %s not found in configs", chain)
		}
	}
}
