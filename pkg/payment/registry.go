package payment

import (
	"fmt"
	"sync"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// Registry maps ChainType to ChainEscrow.
//
// Chain escrow implementations are registered during node initialization and looked up at runtime
// when chain-specific payment operations are needed. The registry is safe for
// concurrent access.
//
// Usage:
//
//	reg := payment.NewRegistry()
//	reg.Register(iwallet.ChainBitcoin, utxoStrategy)
//	reg.Register(iwallet.ChainBSC, evmStrategy)
//
//	// Dispatch by coin type
//	strategy, err := reg.ForCoin(iwallet.CoinType("crypto:eip155:56:native")) // resolves coin → BSC chain → evmStrategy
type Registry struct {
	mu         sync.RWMutex
	strategies map[iwallet.ChainType]ChainEscrow

	// strategiesV2 holds V2-native registrations (Phase EVM-ManagedEscrow v0.3.0).
	// When a chain is registered via RegisterV2 the value is stored here
	// directly; ForCoinV2 / ForChainV2 prefer this map and fall back to
	// wrapping the V1 entry for chains that haven't migrated yet.
	strategiesV2 map[iwallet.ChainType]ChainEscrowV2

	// v2WrapCache memoizes V1AsV2 wrappers per V1 strategy pointer so
	// repeated ForCoinV2 calls return the same value. The cache is
	// invalidated whenever a V1 entry is overwritten via Register.
	v2WrapCache map[ChainEscrow]ChainEscrowV2
}

// NewRegistry creates an empty chain escrow registry.
func NewRegistry() *Registry {
	return &Registry{
		strategies:   make(map[iwallet.ChainType]ChainEscrow),
		strategiesV2: make(map[iwallet.ChainType]ChainEscrowV2),
		v2WrapCache:  make(map[ChainEscrow]ChainEscrowV2),
	}
}

// Register adds or replaces the ChainEscrow implementation for the given chain.
// If a ChainEscrow implementation was previously registered for this chain, it is overwritten.
//
// Registering via the V1 surface implicitly clears any cached V2 wrapper
// for the same chain so the next ForCoinV2 / ForChainV2 call observes
// the new strategy.
func (r *Registry) Register(chain iwallet.ChainType, strategy ChainEscrow) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if prev, ok := r.strategies[chain]; ok {
		delete(r.v2WrapCache, prev)
	}
	r.strategies[chain] = strategy

	// V1 takes precedence only when no explicit V2 registration exists.
	// Drop a stale V2-wrap entry so the next ForChainV2 lookup produces
	// a wrapper around the new V1 strategy.
	if _, hasNativeV2 := r.strategiesV2[chain]; !hasNativeV2 {
		delete(r.strategiesV2, chain)
	}
}

// RegisterV2 adds or replaces a V2-native ChainEscrowV2 implementation
// for the given chain (Phase EVM-ManagedEscrow v0.3.0). V2 registrations take
// precedence over V1 entries on V2 lookups; V1 lookups remain
// unaffected unless the V2 implementation also satisfies ChainEscrow
// (e.g., via embedding or a hand-written V2-to-V1 adapter).
//
// Use this for ManagedEscrowAdapter, SolanaAnchorAdapter, and any future
// action-centric strategy. V1-only strategies (TRON today) keep using
// Register and are auto-wrapped by ForCoinV2.
func (r *Registry) RegisterV2(chain iwallet.ChainType, strategy ChainEscrowV2) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.strategiesV2[chain] = strategy
}

// ForChain returns the ChainEscrow registered for the given chain type.
// Returns an error if no chain escrow is registered for the chain.
func (r *Registry) ForChain(chain iwallet.ChainType) (ChainEscrow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.strategies[chain]
	if !ok {
		return nil, fmt.Errorf("no chain escrow registered for chain %s", chain)
	}
	return s, nil
}

// ForCoin resolves a CoinType to its ChainType and returns the corresponding ChainEscrow implementation.
// This is the primary lookup method used by dispatchers, since events carry coin types.
//
// Example: ForCoin("crypto:eip155:56:native") → CoinInfo.Chain == ChainBSC → ForChain(ChainBSC)
func (r *Registry) ForCoin(coin iwallet.CoinType) (ChainEscrow, error) {
	info, err := coin.CoinInfo()
	if err != nil {
		return nil, fmt.Errorf("unknown coin %s: %w", coin, err)
	}
	return r.ForChain(info.Chain)
}

// Chains returns all chain types that have a registered ChainEscrow implementation.
// The order is not guaranteed.
func (r *Registry) Chains() []iwallet.ChainType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[iwallet.ChainType]struct{}, len(r.strategies)+len(r.strategiesV2))
	for chain := range r.strategies {
		seen[chain] = struct{}{}
	}
	for chain := range r.strategiesV2 {
		seen[chain] = struct{}{}
	}
	chains := make([]iwallet.ChainType, 0, len(seen))
	for chain := range seen {
		chains = append(chains, chain)
	}
	return chains
}

// ── V2 lookups ─────────────────────────────────────────────────

// ForChainV2 returns the V2 ChainEscrowV2 implementation for the given
// chain. The lookup order is:
//  1. A V2-native registration (RegisterV2) for the chain.
//  2. A V1 registration auto-wrapped via NewV1AsV2 (cached).
//
// Returns an error if neither is registered.
func (r *Registry) ForChainV2(chain iwallet.ChainType) (ChainEscrowV2, error) {
	r.mu.RLock()
	if v2, ok := r.strategiesV2[chain]; ok {
		r.mu.RUnlock()
		return v2, nil
	}
	v1, ok := r.strategies[chain]
	if !ok {
		r.mu.RUnlock()
		return nil, fmt.Errorf("no chain escrow registered for chain %s", chain)
	}
	if cached, ok := r.v2WrapCache[v1]; ok {
		r.mu.RUnlock()
		return cached, nil
	}
	r.mu.RUnlock()

	// Promote to a write lock to populate the cache. A second goroutine
	// may have populated it while we waited; re-check before storing.
	r.mu.Lock()
	defer r.mu.Unlock()
	if v2, ok := r.strategiesV2[chain]; ok {
		return v2, nil
	}
	v1Now, ok := r.strategies[chain]
	if !ok {
		return nil, fmt.Errorf("no chain escrow registered for chain %s", chain)
	}
	if cached, ok := r.v2WrapCache[v1Now]; ok {
		return cached, nil
	}
	wrapped := NewV1AsV2(v1Now)
	r.v2WrapCache[v1Now] = wrapped
	return wrapped, nil
}

// ForCoinV2 resolves a CoinType to its ChainType and returns the
// corresponding ChainEscrowV2 implementation, auto-wrapping V1 entries
// when needed. This is the V2 counterpart to ForCoin and the primary
// entry point for action-centric callers.
func (r *Registry) ForCoinV2(coin iwallet.CoinType) (ChainEscrowV2, error) {
	info, err := coin.CoinInfo()
	if err != nil {
		return nil, fmt.Errorf("unknown coin %s: %w", coin, err)
	}
	return r.ForChainV2(info.Chain)
}
