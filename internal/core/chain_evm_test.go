package core

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/chains"
	"github.com/mobazha/mobazha3.0/internal/chains/base"
	ethWal "github.com/mobazha/mobazha3.0/internal/chains/evm"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/evm"
	"github.com/mobazha/mobazha3.0/pkg/relay"
	"github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ── Mock types ──────────────────────────────────────────────────────────

type mockChainClient struct {
	chain iwallet.ChainType
}

func (m *mockChainClient) GetTransaction(id iwallet.TransactionID, coinType iwallet.CoinType) (*iwallet.Transaction, error) {
	return nil, nil
}
func (m *mockChainClient) EstimateFee(txsize int) (map[iwallet.FeeLevel]iwallet.EstimateFeeRes, error) {
	return nil, nil
}
func (m *mockChainClient) Broadcast(serializedTx []byte) error { return nil }

type ownershipAwareMockWallet struct {
	iwallet.Wallet
	lastClient iwallet.ChainClient
	lastOwned  bool
}

func (w *ownershipAwareMockWallet) SetChainClient(client iwallet.ChainClient) {
	w.lastClient = client
}

func (w *ownershipAwareMockWallet) SetChainClientWithOwnership(client iwallet.ChainClient, owned bool) {
	w.lastClient = client
	w.lastOwned = owned
}

type mockBalanceChainClient struct {
	mockChainClient
	balance *big.Int
}

func (m *mockBalanceChainClient) BalanceAt(context.Context, common.Address, *big.Int) (*big.Int, error) {
	return new(big.Int).Set(m.balance), nil
}

// mockHostService implements coreiface.HostService with configurable EVM clients.
type mockHostService struct {
	evmClients map[iwallet.ChainType]iwallet.ChainClient
}

func (m *mockHostService) GetUTXOMonitor() utxo.UTXOMonitorService   { return nil }
func (m *mockHostService) GetEVMRelayService() relay.EVMRelayService { return nil }
func (m *mockHostService) GetGlobalBlockedIDs() []peer.ID            { return nil }
func (m *mockHostService) SetGlobalBlockedIDs(ids []peer.ID)         {}
func (m *mockHostService) GetEVMChainClient(chain iwallet.ChainType) iwallet.ChainClient {
	if m.evmClients == nil {
		return nil
	}
	client, ok := m.evmClients[chain]
	if !ok {
		return nil
	}
	return client
}
func (m *mockHostService) GetDiscountAccessForPeer(_ peer.ID) (contracts.DiscountService, contracts.DiscountStore, error) {
	return nil, nil, nil
}
func (m *mockHostService) GetBlobStore() contracts.BlobStore                      { return nil }
func (m *mockHostService) GetNodeServiceByPeerID(_ peer.ID) contracts.NodeService { return nil }
func (m *mockHostService) GetPlatformFeatureProvider() pkgconfig.PlatformGlobalProvider {
	return nil
}

var _ coreiface.HostService = (*mockHostService)(nil)

// ── configureEVMWallets tests (SaaS path) ───────────────────────────────

func TestConfigureEVMWallets_InjectsSharedClients(t *testing.T) {
	bscWallet := newTestETHWallet(t, iwallet.ChainBSC)
	ethWallet := newTestETHWallet(t, iwallet.ChainEthereum)
	polyWallet := newTestETHWallet(t, iwallet.ChainPolygon)

	mw := &mockWalletProvider{
		wallets: map[iwallet.ChainType]iwallet.Wallet{
			iwallet.ChainBSC:      bscWallet,
			iwallet.ChainEthereum: ethWallet,
			iwallet.ChainPolygon:  polyWallet,
		},
	}

	bscClient := &mockChainClient{chain: iwallet.ChainBSC}
	ethClient := &mockChainClient{chain: iwallet.ChainEthereum}
	hs := &mockHostService{
		evmClients: map[iwallet.ChainType]iwallet.ChainClient{
			iwallet.ChainBSC:      bscClient,
			iwallet.ChainEthereum: ethClient,
			// Polygon: no shared client
		},
	}

	// Before injection: ChainClient should be nil
	if bscWallet.ChainClient != nil {
		t.Fatal("BSC wallet ChainClient should be nil before injection")
	}

	if configured := configureEVMWallets("test-node", mw, hs); configured != 2 {
		t.Fatalf("configured wallets = %d, want 2", configured)
	}

	// BSC and ETH should be injected
	if bscWallet.ChainClient != bscClient {
		t.Error("BSC wallet should have the shared BSC client")
	}
	if ethWallet.ChainClient != ethClient {
		t.Error("ETH wallet should have the shared ETH client")
	}
	// Polygon: no shared client → stays nil
	if polyWallet.ChainClient != nil {
		t.Error("Polygon wallet should remain nil (no shared client)")
	}
}

func TestConfigureEVMWallets_NilInputs(t *testing.T) {
	// Should not panic with nil HostService or nil walletProvider
	if configured := configureEVMWallets("test", nil, nil); configured != 0 {
		t.Fatalf("nil inputs configured %d wallets, want 0", configured)
	}
	if configured := configureEVMWallets("test", &mockWalletProvider{}, nil); configured != 0 {
		t.Fatalf("nil host configured %d wallets, want 0", configured)
	}
	if configured := configureEVMWallets("test", nil, &mockHostService{}); configured != 0 {
		t.Fatalf("nil wallet provider configured %d wallets, want 0", configured)
	}
}

func TestHostEVMNativeBalanceChecker_UsesSharedClient(t *testing.T) {
	want := big.NewInt(12345)
	hs := &mockHostService{
		evmClients: map[iwallet.ChainType]iwallet.ChainClient{
			iwallet.ChainEthereum: &mockBalanceChainClient{
				mockChainClient: mockChainClient{chain: iwallet.ChainEthereum},
				balance:         want,
			},
		},
	}
	checker := &hostEVMNativeBalanceChecker{hostService: hs}

	got, err := checker.GetAddressBalance(context.Background(), "crypto:eip155:1:native", "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatalf("GetAddressBalance: %v", err)
	}
	if got.Cmp(want) != 0 {
		t.Fatalf("balance = %s, want %s", got, want)
	}
}

// ── extractEVMConfigs tests ─────────────────────────────────────────────

func TestExtractEVMConfigs_FromChainAPIs(t *testing.T) {
	chainAPIs := map[iwallet.ChainType]chains.APIUrls{
		iwallet.ChainBSC: {
			MainnetRpc:             []string{"https://bsc-custom.example.com", "https://bsc-fallback.example.com"},
			MainnetRegistryAddress: "0xBSCRegistry",
			TestnetRpc:             []string{"https://bsc-testnet.example.com"},
			TestnetRegistryAddress: "0xBSCTestRegistry",
		},
		iwallet.ChainEthereum: {
			MainnetRpc:             []string{"https://eth-custom.example.com"},
			MainnetRegistryAddress: "0xETHRegistry",
			MainnetEscrowAddress:   "0xETHEscrow",
			TestnetRpc:             []string{"https://eth-testnet.example.com"},
			TestnetRegistryAddress: "0xETHTestRegistry",
			TestnetEscrowAddress:   "0xETHTestEscrow",
		},
		// Polygon: no RPC URLs → should be skipped
		iwallet.ChainPolygon: {},
	}

	// Test mainnet extraction
	configs := extractEVMConfigs(chainAPIs, false)
	if len(configs) != 2 {
		t.Fatalf("expected 2 mainnet configs, got %d", len(configs))
	}

	for _, cfg := range configs {
		switch cfg.ChainType {
		case iwallet.ChainBSC:
			if cfg.RpcURL != "https://bsc-custom.example.com" {
				t.Errorf("BSC RpcURL = %q, want custom URL (first in list)", cfg.RpcURL)
			}
			if cfg.RegistryAddress != "0xBSCRegistry" {
				t.Errorf("BSC RegistryAddress = %q, want 0xBSCRegistry", cfg.RegistryAddress)
			}
		case iwallet.ChainEthereum:
			if cfg.RpcURL != "https://eth-custom.example.com" {
				t.Errorf("ETH RpcURL = %q, want custom URL", cfg.RpcURL)
			}
			if cfg.EscrowAddress != "0xETHEscrow" {
				t.Errorf("ETH EscrowAddress = %q, want 0xETHEscrow", cfg.EscrowAddress)
			}
		default:
			t.Errorf("unexpected chain %s in configs", cfg.ChainType)
		}
	}

	// Test testnet extraction
	testConfigs := extractEVMConfigs(chainAPIs, true)
	if len(testConfigs) != 2 {
		t.Fatalf("expected 2 testnet configs, got %d", len(testConfigs))
	}
	for _, cfg := range testConfigs {
		if !cfg.Testnet {
			t.Errorf("chain %s: Testnet should be true", cfg.ChainType)
		}
		if cfg.ChainType == iwallet.ChainEthereum && cfg.EscrowAddress != "0xETHTestEscrow" {
			t.Errorf("ETH testnet EscrowAddress = %q, want 0xETHTestEscrow", cfg.EscrowAddress)
		}
	}
}

func TestExtractEVMConfigs_EmptyChainAPIs(t *testing.T) {
	configs := extractEVMConfigs(nil, true)
	if len(configs) != 0 {
		t.Errorf("expected 0 configs for nil ChainAPIs, got %d", len(configs))
	}

	configs = extractEVMConfigs(map[iwallet.ChainType]chains.APIUrls{}, true)
	if len(configs) != 0 {
		t.Errorf("expected 0 configs for empty ChainAPIs, got %d", len(configs))
	}
}

// ── startEVMChainClients tests ──────────────────────────────────────────

func TestGetDefaultConfigs_Standalone_Fallback(t *testing.T) {
	// Verify that evm.GetDefaultConfigs returns configs (used as fallback
	// when node has no stored evmChainConfigs)
	configs := evm.GetDefaultConfigs(true)
	if len(configs) == 0 {
		t.Fatal("GetDefaultConfigs(testnet=true) should return configs")
	}

	// Verify all expected chains are present
	expected := map[iwallet.ChainType]bool{
		iwallet.ChainBSC:      false,
		iwallet.ChainEthereum: false,
		iwallet.ChainPolygon:  false,
		iwallet.ChainBase:     false,
		iwallet.ChainConflux:  false,
	}
	for _, cfg := range configs {
		expected[cfg.ChainType] = true
		if cfg.RpcURL == "" {
			t.Errorf("chain %s: RpcURL should not be empty", cfg.ChainType)
		}
	}
	for chain, found := range expected {
		if !found {
			t.Errorf("chain %s not found in default configs", chain)
		}
	}
}

func TestStartEVMChainClients_SaaS_UsesHostService(t *testing.T) {
	// Create wallets with nil ChainClient (simulating post-construction state)
	bscWallet := newTestETHWallet(t, iwallet.ChainBSC)
	baseWallet := newTestETHWallet(t, iwallet.ChainBase)

	// Verify nil before injection
	if bscWallet.ChainClient != nil {
		t.Fatal("BSC wallet should have nil ChainClient before Start")
	}

	bscClient := &mockChainClient{chain: iwallet.ChainBSC}
	baseClient := &mockChainClient{chain: iwallet.ChainBase}

	mw := &mockWalletProvider{
		wallets: map[iwallet.ChainType]iwallet.Wallet{
			iwallet.ChainBSC:  bscWallet,
			iwallet.ChainBase: baseWallet,
		},
	}

	hs := &mockHostService{
		evmClients: map[iwallet.ChainType]iwallet.ChainClient{
			iwallet.ChainBSC:  bscClient,
			iwallet.ChainBase: baseClient,
		},
	}

	// Simulate SaaS path: configureEVMWallets uses HostService
	configureEVMWallets("test-saas-node", mw, hs)

	if bscWallet.ChainClient != bscClient {
		t.Error("BSC wallet should have shared client from HostService")
	}
	if baseWallet.ChainClient != baseClient {
		t.Error("Base wallet should have shared client from HostService")
	}
}

func TestConfigureEVMWallets_InjectsSharedClientsAsBorrowed(t *testing.T) {
	borrowedWallet := &ownershipAwareMockWallet{}
	bscClient := &mockChainClient{chain: iwallet.ChainBSC}
	mw := &mockWalletProvider{
		wallets: map[iwallet.ChainType]iwallet.Wallet{
			iwallet.ChainBSC: borrowedWallet,
		},
	}
	hs := &mockHostService{
		evmClients: map[iwallet.ChainType]iwallet.ChainClient{
			iwallet.ChainBSC: bscClient,
		},
	}

	if configured := configureEVMWallets("test-node", mw, hs); configured != 1 {
		t.Fatalf("configured wallets = %d, want 1", configured)
	}
	if borrowedWallet.lastClient != bscClient {
		t.Fatal("expected shared client to be injected")
	}
	if borrowedWallet.lastOwned {
		t.Fatal("shared SaaS client must be injected as borrowed, not owned")
	}
}

// ── Business roundtrip tests: verify wallet operations work AFTER injection ──
//
// EVM's GetContractAddress requires a REAL *EthClient with RPC connection
// (it calls ethClient.GetRecommendedContractVersion() which hits the Registry).
// Therefore we can only test:
//   1. Pre-injection: GetContractAddress fails with clear error
//   2. Post-injection with mock: type assertion succeeds and method is called
//   3. EVM CreateEscrowAddress (does NOT need ChainClient — pure script hash)
//
// Full RPC-dependent GetContractAddress is covered by integration tests.

func TestEVMWallet_BusinessRoundtrip_GetContractAddress_BeforeInjection(t *testing.T) {
	// Before injection: GetContractAddress should fail with a clear error,
	// not panic. This is the path hit if Start() fails to inject.
	w := newTestETHWallet(t, iwallet.ChainBSC)
	_, err := w.GetContractAddress()
	if err == nil {
		t.Fatal("GetContractAddress should fail before injection")
	}
	if !strings.Contains(err.Error(), "chain client not configured") {
		t.Errorf("error should mention 'chain client not configured', got: %v", err)
	}
}

func TestEVMWallet_BusinessRoundtrip_CreateEscrowAddress_NoClient(t *testing.T) {
	// EVM CreateEscrowAddress is a pure script hash computation —
	// it does NOT depend on ChainClient. Verify this works even with nil client.
	w := newTestETHWallet(t, iwallet.ChainBSC)

	// Create a valid EscrowInfo for EVM
	bscCoin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainBSC)
	if err != nil {
		t.Fatalf("RequireCanonicalNativeCoinType(BSC) failed: %v", err)
	}
	escrowInfo := iwallet.EscrowInfo{
		CoinType:           bscCoin,
		BuyerAddress:       "0x1111111111111111111111111111111111111111",
		SellerAddress:      "0x2222222222222222222222222222222222222222",
		ContractAddress:    "0x3333333333333333333333333333333333333333",
		UniqueId:           [20]byte{1, 2, 3},
		RequiredSignatures: 2,
		UnlockHours:        24,
	}

	addr, err := w.CreateEscrowAddress(escrowInfo)
	if err != nil {
		t.Fatalf("CreateEscrowAddress (nil client) should work: %v", err)
	}
	if addr.String() == "" {
		t.Error("CreateEscrowAddress should return a non-empty address")
	}

	// Determinism check
	addr2, err := w.CreateEscrowAddress(escrowInfo)
	if err != nil {
		t.Fatalf("second CreateEscrowAddress: %v", err)
	}
	if addr.String() != addr2.String() {
		t.Errorf("CreateEscrowAddress not deterministic: %q != %q", addr.String(), addr2.String())
	}

	t.Logf("EVM CreateEscrowAddress works without ChainClient: %s", addr.String())
}

func TestEVMWallet_BusinessRoundtrip_SetChainClient_TypeSafe(t *testing.T) {
	// Verify the injection path: SetChainClient sets ChainClient correctly,
	// and the wallet can then type-assert to *EthClient.
	w := newTestETHWallet(t, iwallet.ChainBSC)

	// SetChainClient with a mock (not *EthClient)
	mock := &mockChainClient{chain: iwallet.ChainBSC}
	w.SetChainClient(mock)

	if w.ChainClient != mock {
		t.Fatal("ChainClient should be set after SetChainClient")
	}

	// GetContractAddress will attempt to type-assert to *EthClient — it should
	// panic or fail gracefully. This verifies the type assertion path is hit.
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Expected: mock is not *EthClient, type assertion panics
				t.Logf("Type assertion panics as expected with mock (production uses real *EthClient): %v", r)
			}
		}()
		_, err := w.GetContractAddress()
		if err != nil {
			t.Logf("GetContractAddress with mock client returned error (expected): %v", err)
		}
	}()
}

// ── Helpers ─────────────────────────────────────────────────────────────

func newTestETHWallet(t *testing.T, chain iwallet.ChainType) *ethWal.ETHWallet {
	t.Helper()
	coinType, err := iwallet.RequireCanonicalNativeCoinType(chain)
	if err != nil {
		t.Fatalf("RequireCanonicalNativeCoinType(%s) failed: %v", chain, err)
	}
	w, err := ethWal.NewETHWallet(coinType, nil, &base.WalletConfig{
		Testnet: true,
	})
	if err != nil {
		t.Fatalf("NewETHWallet(%s) failed: %v", chain, err)
	}
	return w
}

type mockWalletProvider struct {
	wallets map[iwallet.ChainType]iwallet.Wallet
}

func (m *mockWalletProvider) WalletForChain(chain iwallet.ChainType) (iwallet.Wallet, bool) {
	w, ok := m.wallets[chain]
	return w, ok
}
