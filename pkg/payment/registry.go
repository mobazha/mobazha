package payment

import (
	"fmt"
	"sync"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// Registry maps ChainType to ChainEscrow.
//
// Strategies are registered during node initialization and looked up at runtime
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
}

// NewRegistry creates an empty payment strategy registry.
func NewRegistry() *Registry {
	return &Registry{
		strategies: make(map[iwallet.ChainType]ChainEscrow),
	}
}

// Register adds or replaces a payment strategy for the given chain.
// If a strategy was previously registered for this chain, it is overwritten.
func (r *Registry) Register(chain iwallet.ChainType, strategy ChainEscrow) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.strategies[chain] = strategy
}

// ForChain returns the ChainEscrow registered for the given chain type.
// Returns an error if no strategy is registered for the chain.
func (r *Registry) ForChain(chain iwallet.ChainType) (ChainEscrow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.strategies[chain]
	if !ok {
		return nil, fmt.Errorf("no payment strategy registered for chain %s", chain)
	}
	return s, nil
}

// ForCoin resolves a CoinType to its ChainType and returns the corresponding strategy.
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

// Chains returns all chain types that have a registered strategy.
// The order is not guaranteed.
func (r *Registry) Chains() []iwallet.ChainType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	chains := make([]iwallet.ChainType, 0, len(r.strategies))
	for chain := range r.strategies {
		chains = append(chains, chain)
	}
	return chains
}
