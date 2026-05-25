package payment

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/managedescrow"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// SettlementCoinFromPaymentSent resolves the chain asset declared by the
// immutable PaymentSent envelope. It may normalize legacy spellings carried
// inside PaymentSent.Coin itself, but it never falls back to order pricing,
// pending-payment rows, or observations.
func SettlementCoinFromPaymentSent(paymentSent *pb.PaymentSent) (iwallet.CoinType, error) {
	if paymentSent == nil {
		return "", fmt.Errorf("payment sent is required")
	}

	if coin, ok := NormalizeSettlementPaymentCoin(paymentSent.Coin); ok {
		return coin, nil
	}

	coin, err := iwallet.CanonicalPaymentCoinFromPaymentSent(paymentSent)
	if err != nil {
		return "", fmt.Errorf("invalid payment coin %q in PaymentSent: %w", paymentSent.Coin, err)
	}
	return coin, nil
}

// NormalizeSettlementPaymentCoin converts a persisted payment coin hint into the
// canonical coin used by chain adapters. Runtime testnet native EVM assets map
// back to their canonical ManagedEscrow chain family because the backend settlement
// registry is keyed by chain family, not a per-testnet asset ID.
func NormalizeSettlementPaymentCoin(raw string) (iwallet.CoinType, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) == 4 &&
		strings.EqualFold(parts[0], "crypto") &&
		strings.EqualFold(parts[1], "eip155") &&
		strings.EqualFold(parts[3], "native") {
		if chainID, err := strconv.ParseUint(parts[2], 10, 64); err == nil {
			if chain, ok := managed_escrow.ChainTypeForChainID(chainID); ok {
				canonicalChainID, _ := managed_escrow.ChainIDFor(chain)
				if canonicalChainID != 0 && canonicalChainID != chainID {
					if coin, ok := iwallet.CanonicalNativeCoinType(chain); ok {
						return coin, true
					}
				}
			}
		}
	}

	if coin, ok := iwallet.TryNormalizePaymentCoin(raw); ok {
		return coin, true
	}
	if coin := iwallet.CoinType(strings.TrimSpace(raw)); coin.ValidateCanonicalPaymentCoin() == nil {
		return coin, true
	}

	if len(parts) == 4 &&
		strings.EqualFold(parts[0], "crypto") &&
		strings.EqualFold(parts[1], "eip155") &&
		strings.EqualFold(parts[3], "native") {
		if chainID, err := strconv.ParseUint(parts[2], 10, 64); err == nil {
			if chain, ok := managed_escrow.ChainTypeForChainID(chainID); ok {
				if coin, ok := iwallet.CanonicalNativeCoinType(chain); ok {
					return coin, true
				}
			}
		}
	}

	return "", false
}

// PendingPaymentCoinFromOrder returns the coin locked in the payment intent.
// Callers use it when constructing a PaymentSent envelope. Runtime
// settlement/order actions must use PaymentSent.Coin itself so data-integrity
// failures do not get hidden by fallback guesses.
func PendingPaymentCoinFromOrder(order *models.Order) (iwallet.CoinType, bool) {
	if order == nil {
		return "", false
	}
	if managed_escrowInfo, err := order.GetPendingManagedEscrowPaymentInfo(); err == nil && managed_escrowInfo != nil {
		if coin, ok := NormalizeSettlementPaymentCoin(managed_escrowInfo.Coin); ok {
			return coin, true
		}
	}
	if escrowInfo, err := order.GetPendingEscrowPaymentInfo(); err == nil && escrowInfo != nil {
		if coin, ok := NormalizeSettlementPaymentCoin(escrowInfo.Coin); ok {
			return coin, true
		}
	}
	if utxoInfo, err := order.GetPendingPaymentInfo(); err == nil && utxoInfo != nil {
		if coin, ok := NormalizeSettlementPaymentCoin(utxoInfo.Coin); ok {
			return coin, true
		}
	}
	return "", false
}

// PaymentCoinFromObservation returns a canonical native coin from a confirmed
// chain observation when no pending payment intent is available.
func PaymentCoinFromObservation(obs models.PaymentObservation) (iwallet.CoinType, bool) {
	if strings.EqualFold(obs.ChainNamespace, "eip155") {
		chainID, err := strconv.ParseUint(strings.TrimSpace(obs.ChainReference), 10, 64)
		if err != nil {
			return "", false
		}
		if token := strings.TrimSpace(obs.TokenAddress); token != "" {
			coin := iwallet.CoinType("crypto:eip155:" + strconv.FormatUint(chainID, 10) + ":erc20:" + strings.ToLower(token))
			if err := coin.ValidateCanonicalPaymentCoin(); err == nil {
				return coin, true
			}
		}
		if chain, ok := managed_escrow.ChainTypeForChainID(chainID); ok {
			return iwallet.CanonicalNativeCoinType(chain)
		}
		return "", false
	}

	if strings.EqualFold(obs.ChainNamespace, "bitcoincash") {
		return iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoinCash)
	}
	if strings.EqualFold(obs.ChainNamespace, "zcash") {
		return iwallet.CanonicalNativeCoinType(iwallet.ChainZCash)
	}
	if strings.EqualFold(obs.ChainNamespace, "solana") {
		return iwallet.CanonicalNativeCoinType(iwallet.ChainSolana)
	}
	if strings.EqualFold(obs.ChainNamespace, "tron") {
		return iwallet.CanonicalNativeCoinType(iwallet.ChainTRON)
	}

	return "", false
}
