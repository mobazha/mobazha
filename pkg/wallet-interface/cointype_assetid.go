package wallet_interface

import (
	"fmt"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/assetid"
)

var canonicalAssetRegistry = assetid.DefaultRegistry()

const (
	btcMainnetGenesis = "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f"
	ltcMainnetGenesis = "12a765e31ffd4059bada1e25190f6e98c99d9714d334efa41a195a7e7e04bfe2"
)

// IsCanonicalCryptoAssetID reports whether this coin type is a canonical
// crypto asset id in the form of crypto:*.
func (ct CoinType) IsCanonicalCryptoAssetID() bool {
	raw := strings.TrimSpace(string(ct))
	if !strings.HasPrefix(strings.ToLower(raw), "crypto:") {
		return false
	}
	return assetid.IsCanonical(raw)
}

// PricingCurrencyCode returns the pricing/exchange currency code used by
// CurrencyDefinitions and exchange-rate lookup (e.g. BTC, BSCUSDT, USD).
func (ct CoinType) PricingCurrencyCode() (string, error) {
	if ct.IsFiatPayment() {
		return strings.ToUpper(ct.FiatBaseCurrency()), nil
	}

	def, isCanonical, err := lookupCanonicalAssetDefinition(ct)
	if err != nil {
		return "", err
	}
	if isCanonical {
		return strings.ToUpper(def.Code), nil
	}

	// Legacy fallback while the codebase is still being migrated.
	return strings.ToUpper(strings.TrimSpace(string(ct))), nil
}

func lookupCanonicalAssetDefinition(ct CoinType) (assetid.Definition, bool, error) {
	raw := strings.TrimSpace(string(ct))
	if !strings.HasPrefix(strings.ToLower(raw), "crypto:") {
		return assetid.Definition{}, false, nil
	}

	normalized, err := assetid.Normalize(raw)
	if err != nil {
		return assetid.Definition{}, true, err
	}

	def, err := canonicalAssetRegistry.Lookup(normalized)
	if err != nil {
		return assetid.Definition{}, true, err
	}
	return def, true, nil
}

func coinInfoFromCanonicalAssetID(coinType CoinType) (CoinInfo, error) {
	def, isCanonical, err := lookupCanonicalAssetDefinition(coinType)
	if err != nil {
		return CoinInfo{}, fmt.Errorf("invalid canonical asset id %q: %w", coinType, err)
	}
	if !isCanonical {
		return CoinInfo{}, fmt.Errorf("coin %q is not canonical asset id", coinType)
	}

	parsed, err := assetid.Parse(def.AssetID)
	if err != nil {
		return CoinInfo{}, fmt.Errorf("parse canonical asset id %q: %w", def.AssetID, err)
	}

	chain, err := chainTypeFromAssetID(parsed)
	if err != nil {
		return CoinInfo{}, err
	}

	info := CoinInfo{
		Chain:       chain,
		Symbol:      def.DisplaySymbol,
		IsNative:    parsed.IsNative(),
		Decimals:    def.Decimals,
		Description: def.DisplayName,
	}
	if !parsed.IsNative() {
		info.Contract = parsed.AssetRef
	}
	return info, nil
}

func chainTypeFromAssetID(id assetid.ID) (ChainType, error) {
	switch id.Namespace {
	case assetid.NamespaceBIP122:
		switch id.ChainRef {
		case btcMainnetGenesis:
			return ChainBitcoin, nil
		case ltcMainnetGenesis:
			return ChainLitecoin, nil
		default:
			return "", fmt.Errorf("unsupported bip122 chain_ref %q", id.ChainRef)
		}
	case assetid.NamespaceEIP155:
		switch id.ChainRef {
		case "1":
			return ChainEthereum, nil
		case "56":
			return ChainBSC, nil
		case "137":
			return ChainPolygon, nil
		case "8453":
			return ChainBase, nil
		default:
			return "", fmt.Errorf("unsupported eip155 chain_ref %q", id.ChainRef)
		}
	case assetid.NamespaceTRON:
		if id.ChainRef != "mainnet" {
			return "", fmt.Errorf("unsupported tron chain_ref %q", id.ChainRef)
		}
		return ChainTRON, nil
	case assetid.NamespaceSolana:
		if id.ChainRef != "mainnet" {
			return "", fmt.Errorf("unsupported solana chain_ref %q", id.ChainRef)
		}
		return ChainSolana, nil
	default:
		return "", fmt.Errorf("unsupported namespace %q", id.Namespace)
	}
}
