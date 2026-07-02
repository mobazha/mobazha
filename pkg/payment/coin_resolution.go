package payment

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mobazha/mobazha3.0/pkg/assetid"
	"github.com/mobazha/mobazha3.0/pkg/evm"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
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
// back to their canonical managed escrow chain family because the backend settlement
// registry is keyed by chain family, not a per-testnet asset ID.
func NormalizeSettlementPaymentCoin(raw string) (iwallet.CoinType, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) == 4 &&
		strings.EqualFold(parts[0], "crypto") &&
		strings.EqualFold(parts[1], "eip155") &&
		strings.EqualFold(parts[3], "native") {
		if chainID, err := strconv.ParseUint(parts[2], 10, 64); err == nil {
			if chain, ok := evm.ChainTypeForID(chainID); ok {
				canonicalChainID, _ := evm.ChainIDForNetwork(chain, false)
				if canonicalChainID != 0 && canonicalChainID != chainID {
					if coin, ok := iwallet.CanonicalNativeCoinType(chain); ok {
						return coin, true
					}
				}
			}
		}
	}

	if normalized, err := assetid.Normalize(strings.TrimSpace(raw)); err == nil {
		return iwallet.CoinType(normalized), true
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
			if chain, ok := evm.ChainTypeForID(chainID); ok {
				if coin, ok := iwallet.CanonicalNativeCoinType(chain); ok {
					return coin, true
				}
			}
		}
	}

	return "", false
}

// SettlementChainForCoin returns the chain family used for settlement routing.
// It accepts canonical payment coins and runtime asset IDs whose token contract
// or mint comes from local deployment state or store payment sessions.
func SettlementChainForCoin(coin iwallet.CoinType) (iwallet.ChainType, error) {
	info, err := SettlementCoinInfoForCoin(coin)
	if err != nil {
		return "", err
	}
	return info.Chain, nil
}

// SettlementCoinInfoForCoin returns chain metadata for settlement routing. In
// addition to registered payment coins, it accepts runtime crypto asset IDs so
// order/payment flows can carry the exact token contract or mint selected by a
// store payment session without requiring a hard-coded wallet-interface catalog
// entry.
func SettlementCoinInfoForCoin(coin iwallet.CoinType) (iwallet.CoinInfo, error) {
	normalized, ok := NormalizeSettlementPaymentCoin(coin.String())
	if !ok {
		return iwallet.CoinInfo{}, fmt.Errorf("invalid settlement payment coin %q", coin)
	}
	if info, err := normalized.CoinInfo(); err == nil {
		return info, nil
	}
	if runtimeCoin, chain, ok := runtimeEIP155ERC20Coin(normalized.String()); ok {
		parts := strings.Split(runtimeCoin.String(), ":")
		contract := common.HexToAddress(parts[4]).Hex()
		return iwallet.CoinInfo{
			Chain:           chain,
			Symbol:          "ERC20",
			IsNative:        false,
			Contract:        contract,
			TestnetContract: contract,
			Description:     "Runtime ERC-20",
		}, nil
	}
	return iwallet.CoinInfo{}, fmt.Errorf("unsupported settlement payment coin %q", coin)
}

func runtimeEIP155ERC20Coin(raw string) (iwallet.CoinType, iwallet.ChainType, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 5 ||
		!strings.EqualFold(parts[0], "crypto") ||
		!strings.EqualFold(parts[1], "eip155") ||
		!strings.EqualFold(parts[3], "erc20") {
		return "", "", false
	}
	chainID, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return "", "", false
	}
	chain, ok := evm.ChainTypeForID(chainID)
	if !ok || !common.IsHexAddress(parts[4]) {
		return "", "", false
	}
	coin := iwallet.CoinType(fmt.Sprintf("crypto:eip155:%d:erc20:%s", chainID, common.HexToAddress(parts[4]).Hex()))
	return coin, chain, true
}

// PendingPaymentCoinFromOrder returns the coin locked in the payment intent.
// Callers use it when constructing a PaymentSent envelope. Runtime
// settlement/order actions must use PaymentSent.Coin itself so data-integrity
// failures do not get hidden by fallback guesses.
func PendingPaymentCoinFromOrder(order *models.Order) (iwallet.CoinType, bool) {
	if order == nil {
		return "", false
	}
	if managedInfo, err := order.GetPendingManagedEscrowInfo(); err == nil && managedInfo != nil {
		if coin, ok := NormalizeSettlementPaymentCoin(managedInfo.Coin); ok {
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
		if chain, ok := evm.ChainTypeForID(chainID); ok {
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
