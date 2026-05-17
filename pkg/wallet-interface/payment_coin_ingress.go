package wallet_interface

import (
	"fmt"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/assetid"
)

// TryNormalizePaymentCoin converts raw payment coin strings into canonical
// CoinType values when unambiguous:
//   - crypto:* asset IDs — casing / segments normalized via assetid.Normalize
//   - fiat:{provider}:{currency} — provider lowercased, currency uppercased
//   - legacy native tickers known to map 1:1 to a chain (btc, eth, sol, …)
//
// It returns ("", false) when raw is empty, malformed, incomplete (e.g.
// fiat:stripe without currency), or ambiguous (unknown ticker).
//
// Used by projection layers for best-effort canonical outputs without failing
// the entire session view on legacy rows.
func TryNormalizePaymentCoin(raw string) (CoinType, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}

	lower := strings.ToLower(s)

	if strings.HasPrefix(lower, "crypto:") {
		norm, err := assetid.Normalize(s)
		if err != nil {
			return "", false
		}
		ct := CoinType(norm)
		if err := ct.ValidateCanonicalPaymentCoin(); err != nil {
			return "", false
		}
		return ct, true
	}

	if strings.HasPrefix(lower, "fiat:") {
		parts := strings.SplitN(s, ":", 3)
		if len(parts) != 3 {
			return "", false
		}
		prov := strings.TrimSpace(parts[1])
		curr := strings.TrimSpace(parts[2])
		if prov == "" || curr == "" {
			return "", false
		}
		ct := CoinType(fmt.Sprintf("fiat:%s:%s", strings.ToLower(prov), strings.ToUpper(curr)))
		if err := ct.ValidateCanonicalPaymentCoin(); err != nil {
			return "", false
		}
		return ct, true
	}

	ticker := strings.ToLower(s)
	if chain, ok := legacyTickerToChain[ticker]; ok {
		canon, ok := CanonicalNativeCoinType(chain)
		if !ok {
			return "", false
		}
		return canon, true
	}

	return "", false
}

// NormalizePaymentCoinIngress normalizes caller-supplied payment coins at HTTP /
// RPC ingress. Legacy native tickers supported by TryNormalizePaymentCoin are
// accepted; ambiguous values return an error instructing callers to pass a
// canonical crypto:* or fiat:{provider}:{currency} ID.
//
// Phase PS B4 — API ingress canonical policy.
func NormalizePaymentCoinIngress(raw string) (CoinType, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("payment coin is empty")
	}
	if ct, ok := TryNormalizePaymentCoin(s); ok {
		return ct, nil
	}
	return "", fmt.Errorf(
		"ambiguous or unsupported payment coin %q; use a canonical crypto asset id (crypto:…) or fiat:provider:CURRENCY",
		raw,
	)
}

// legacyTickerToChain maps legacy single-chain native tickers to ChainType.
// Kept in sync with internal/core/payment/session_projector.normalizeCoinBestEffort.
var legacyTickerToChain = map[string]ChainType{
	"btc":   ChainBitcoin,
	"bch":   ChainBitcoinCash,
	"ltc":   ChainLitecoin,
	"zec":   ChainZCash,
	"external_payment":   ChainExternalPayment,
	"eth":   ChainEthereum,
	"bnb":   ChainBSC,
	"sol":   ChainSolana,
	"trx":   ChainTRON,
	"matic": ChainPolygon,
	"base":  ChainBase,
	"arb":   ChainArbitrum,
	"op":    ChainOptimism,
	"avax":  ChainAvalanche,
}
