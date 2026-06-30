package adapters

import (
	"context"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

var _ contracts.ReceiptVerifier = (*ReceiptVerifierRegistry)(nil)

// ReceiptVerifierRegistry dispatches receipt verification to chain-specific
// verifiers via a map[ChainType]ReceiptVerifier. Unregistered chains are
// treated as noop (return nil).
//
// This replaces the former CompositeReceiptVerifier which hardcoded
// EVM + Solana dispatch, silently excluding TRON.
type ReceiptVerifierRegistry struct {
	verifiers map[iwallet.ChainType]contracts.ReceiptVerifier
}

// NewReceiptVerifierRegistry creates the Open Core verifier registry. Private
// distribution chains verify through their own V2 strategies.
func NewReceiptVerifierRegistry(mw contracts.WalletOperator) *ReceiptVerifierRegistry {
	evmVerifier := NewEVMReceiptVerifier(mw)
	return &ReceiptVerifierRegistry{
		verifiers: map[iwallet.ChainType]contracts.ReceiptVerifier{
			iwallet.ChainEthereum: evmVerifier,
			iwallet.ChainBSC:      evmVerifier,
			iwallet.ChainPolygon:  evmVerifier,
			iwallet.ChainBase:     evmVerifier,
			iwallet.ChainConflux:  evmVerifier,
			iwallet.ChainTRON:     NewTRONReceiptVerifier(mw),
		},
	}
}

// NewReceiptVerifierRegistryFromMap creates a registry from an explicit verifier map.
func NewReceiptVerifierRegistryFromMap(verifiers map[iwallet.ChainType]contracts.ReceiptVerifier) *ReceiptVerifierRegistry {
	return &ReceiptVerifierRegistry{verifiers: verifiers}
}

func (r *ReceiptVerifierRegistry) resolveVerifier(coinCode string) contracts.ReceiptVerifier {
	coinInfo, err := payment.SettlementCoinInfoForCoin(iwallet.CoinType(coinCode))
	if err != nil {
		return nil
	}
	return r.verifiers[coinInfo.Chain]
}

func (r *ReceiptVerifierRegistry) VerifyTransactionReceipt(ctx context.Context, coinCode string, txHash string) error {
	v := r.resolveVerifier(coinCode)
	if v == nil {
		return nil
	}
	return v.VerifyTransactionReceipt(ctx, coinCode, txHash)
}

func (r *ReceiptVerifierRegistry) WaitAndVerifyReceipt(ctx context.Context, coinCode string, txHash string) error {
	v := r.resolveVerifier(coinCode)
	if v == nil {
		return nil
	}
	return v.WaitAndVerifyReceipt(ctx, coinCode, txHash)
}

// CompositeReceiptVerifier is a type alias for backward compatibility.
// Deprecated: use ReceiptVerifierRegistry directly.
type CompositeReceiptVerifier = ReceiptVerifierRegistry

// NewCompositeReceiptVerifier is a backward-compatible constructor.
// Deprecated: use NewReceiptVerifierRegistry directly.
func NewCompositeReceiptVerifier(mw contracts.WalletOperator) *ReceiptVerifierRegistry {
	return NewReceiptVerifierRegistry(mw)
}
