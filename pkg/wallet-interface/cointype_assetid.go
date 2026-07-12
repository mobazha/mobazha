package wallet_interface

import (
	"fmt"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/assetid"
)

var canonicalAssetRegistry = assetid.DefaultRegistry()
var canonicalNativeCoinByChain = buildCanonicalNativeCoinByChain()
var canonicalBlockIntervalByChain = buildCanonicalBlockIntervalByChain()
var canonicalBIP44ByChain = buildCanonicalBIP44ByChain()

const (
	btcMainnetGenesis = "000000000019d6689c085ae165831e93"
	ltcMainnetGenesis = "12a765e31ffd4059bada1e25190f6e98"
)

func buildCanonicalNativeCoinByChain() map[ChainType]CoinType {
	result := make(map[ChainType]CoinType)
	for _, def := range canonicalAssetRegistry.List() {
		parsed, err := assetid.Parse(def.AssetID)
		if err != nil || !parsed.IsNative() {
			continue
		}
		chain, err := chainTypeFromAssetID(parsed)
		if err != nil {
			continue
		}
		result[chain] = CoinType(def.AssetID)
	}
	return result
}

// CanonicalNativeCoinType returns the canonical native crypto:* coin type for a chain.
// For chains that are not yet in assetid registry, bool will be false.
func CanonicalNativeCoinType(chain ChainType) (CoinType, bool) {
	coin, ok := canonicalNativeCoinByChain[chain]
	return coin, ok
}

func RequireCanonicalNativeCoinType(chain ChainType) (CoinType, error) {
	coin, ok := CanonicalNativeCoinType(chain)
	if !ok {
		return "", fmt.Errorf("no canonical native asset id configured for chain %s", chain)
	}
	return coin, nil
}

func CanonicalBlockInterval(chain ChainType) (time.Duration, bool) {
	interval, ok := canonicalBlockIntervalByChain[chain]
	return interval, ok
}

func buildCanonicalBIP44ByChain() map[ChainType]uint32 {
	result := make(map[ChainType]uint32)
	for _, def := range canonicalAssetRegistry.List() {
		parsed, err := assetid.Parse(def.AssetID)
		if err != nil || !parsed.IsNative() || def.Runtime.Bip44Code == 0 {
			continue
		}
		chain, err := chainTypeFromAssetID(parsed)
		if err != nil {
			continue
		}
		result[chain] = uint32(def.Runtime.Bip44Code)
	}
	// Bitcoin's BIP44 code is 0, which is skipped by the > 0 check above.
	// Hard-code it since 0 is a valid coin type for Bitcoin.
	if _, ok := result[ChainBitcoin]; !ok {
		result[ChainBitcoin] = 0
	}
	return result
}

// CanonicalBIP44CoinType returns the SLIP-0044 coin type for a chain's native
// asset, derived from the assetid registry. Returns false for chains without a
// registered BIP44 code (e.g. Solana uses ed25519, not BIP-44).
func CanonicalBIP44CoinType(chain ChainType) (uint32, bool) {
	code, ok := canonicalBIP44ByChain[chain]
	return code, ok
}

func buildCanonicalBlockIntervalByChain() map[ChainType]time.Duration {
	result := make(map[ChainType]time.Duration)
	for _, def := range canonicalAssetRegistry.List() {
		parsed, err := assetid.Parse(def.AssetID)
		if err != nil || !parsed.IsNative() || def.Runtime.BlockInterval <= 0 {
			continue
		}
		chain, err := chainTypeFromAssetID(parsed)
		if err != nil {
			continue
		}
		result[chain] = def.Runtime.BlockInterval
	}
	return result
}

// IsCanonicalCryptoAssetID reports whether this coin type is a canonical
// crypto asset id in the form of crypto:*.
func (ct CoinType) IsCanonicalCryptoAssetID() bool {
	raw := strings.TrimSpace(string(ct))
	if !strings.HasPrefix(strings.ToLower(raw), "crypto:") {
		return false
	}
	return assetid.IsCanonical(raw)
}

// IsCanonicalPaymentCoin reports whether coin can be used in payment flows.
// It accepts canonical fiat IDs, canonical crypto asset IDs, and test-only coins.
func (ct CoinType) IsCanonicalPaymentCoin() bool {
	if ct.IsCanonicalFiatPaymentCoin() {
		return true
	}
	if ct == CtMock || ct == CtTestCoin {
		return true
	}
	return ct.IsCanonicalCryptoAssetID()
}

// IsCanonicalFiatPaymentCoin reports whether coin matches fiat:{provider}:{currency}.
func (ct CoinType) IsCanonicalFiatPaymentCoin() bool {
	raw := strings.TrimSpace(string(ct))
	parts := strings.Split(raw, ":")
	if len(parts) != 3 {
		return false
	}
	if !strings.EqualFold(parts[0], "fiat") {
		return false
	}
	provider := strings.TrimSpace(parts[1])
	currency := strings.TrimSpace(parts[2])
	return provider != "" && currency != ""
}

// ValidateCanonicalPaymentCoin returns error when coin is not canonical for payment.
func (ct CoinType) ValidateCanonicalPaymentCoin() error {
	if ct.IsCanonicalPaymentCoin() {
		return nil
	}
	return fmt.Errorf("coin must be canonical payment coin (crypto:* / fiat:{provider}:{currency}), got %q", ct)
}

// PricingCurrencyCode returns the pricing/exchange currency code used by
// CurrencyDefinitions and exchange-rate lookup (e.g. BTC, BSCUSDT, USD).
func (ct CoinType) PricingCurrencyCode() (string, error) {
	if ct.IsFiatPayment() {
		if !ct.IsCanonicalFiatPaymentCoin() {
			return "", fmt.Errorf("fiat payment coin must use canonical format fiat:{provider}:{currency}, got %q", ct)
		}
		return strings.ToUpper(ct.FiatBaseCurrency()), nil
	}
	if ct == CtMock || ct == CtTestCoin {
		return strings.ToUpper(strings.TrimSpace(string(ct))), nil
	}

	def, isCanonical, err := lookupCanonicalAssetDefinition(ct)
	if err != nil {
		return "", err
	}
	if isCanonical {
		return strings.ToUpper(def.Code), nil
	}

	return "", fmt.Errorf("coin must be canonical payment coin (crypto:* / fiat:{provider}:{currency}), got %q", ct)
}

// MatchesPricingCurrency reports whether raw identifies this payment asset by
// either its canonical payment coin or its human-facing pricing code.
func (ct CoinType) MatchesPricingCurrency(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || (!ct.IsCanonicalPaymentCoin() && ct != CtMock && ct != CtTestCoin) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(string(ct)), raw) {
		return true
	}
	code, err := ct.PricingCurrencyCode()
	return err == nil && strings.EqualFold(code, raw)
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
	normalized, err := assetid.Normalize(string(coinType))
	if err != nil {
		return CoinInfo{}, fmt.Errorf("invalid canonical asset id %q: %w", coinType, err)
	}

	parsed, err := assetid.Parse(normalized)
	if err != nil {
		return CoinInfo{}, fmt.Errorf("parse canonical asset id %q: %w", normalized, err)
	}
	chain, err := chainTypeFromAssetID(parsed)
	if err != nil {
		return CoinInfo{}, err
	}

	if def, err := canonicalAssetRegistry.Lookup(normalized); err == nil {
		return coinInfoFromAssetDefinition(def, parsed, chain), nil
	}

	return runtimeCoinInfoFromAssetID(parsed, chain), nil
}

func coinInfoFromAssetDefinition(def assetid.Definition, parsed assetid.ID, chain ChainType) CoinInfo {
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
	return info
}

func runtimeCoinInfoFromAssetID(parsed assetid.ID, chain ChainType) CoinInfo {
	info := CoinInfo{
		Chain:       chain,
		Symbol:      strings.ToUpper(string(parsed.Standard)),
		IsNative:    parsed.IsNative(),
		Description: "Runtime " + string(parsed.Standard) + " asset",
	}
	if parsed.IsNative() {
		info.Symbol = string(chain)
		info.Description = "Runtime native asset"
		if canonicalNative, ok := CanonicalNativeCoinType(chain); ok {
			if def, _, err := lookupCanonicalAssetDefinition(canonicalNative); err == nil {
				info.Symbol = def.DisplaySymbol
				info.Decimals = def.Decimals
				info.Description = def.DisplayName
			}
		}
	} else {
		info.Contract = parsed.AssetRef
		info.TestnetContract = parsed.AssetRef
		info.Decimals = runtimeAssetDefaultDecimals(parsed.Standard)
	}
	return info
}

func runtimeAssetDefaultDecimals(standard assetid.Standard) uint8 {
	switch standard {
	case assetid.StandardERC20:
		return 18
	case assetid.StandardTRC20:
		return 6
	case assetid.StandardSPL:
		return 9
	default:
		return 0
	}
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
		case "1030":
			return ChainConflux, nil
		// Phase managed EVM v0.3.0 Sprint 1 D8 — promoted EVM L2 set.
		// EIP-155 chain ids match the public EVM network registry.
		case "10":
			return ChainOptimism, nil
		case "100":
			return ChainGnosis, nil
		case "324":
			return ChainZkSyncEra, nil
		case "5000":
			return ChainMantle, nil
		case "42161":
			return ChainArbitrum, nil
		case "42220":
			return ChainCelo, nil
		case "43114":
			return ChainAvalanche, nil
		case "59144":
			return ChainLinea, nil
		case "534352":
			return ChainScroll, nil
		default:
			return "", fmt.Errorf("unsupported eip155 chain_ref %q", id.ChainRef)
		}
	case assetid.NamespaceTRON:
		if id.ChainRef != "mainnet" && id.ChainRef != "shasta" && id.ChainRef != "nile" {
			return "", fmt.Errorf("unsupported tron chain_ref %q", id.ChainRef)
		}
		return ChainTRON, nil
	case assetid.NamespaceSolana:
		if id.ChainRef != "mainnet" && id.ChainRef != "devnet" && id.ChainRef != "testnet" {
			return "", fmt.Errorf("unsupported solana chain_ref %q", id.ChainRef)
		}
		return ChainSolana, nil
	case assetid.NamespaceBitcoinCash:
		if id.ChainRef != "mainnet" {
			return "", fmt.Errorf("unsupported bitcoincash chain_ref %q", id.ChainRef)
		}
		return ChainBitcoinCash, nil
	case assetid.NamespaceZCash:
		if id.ChainRef != "mainnet" {
			return "", fmt.Errorf("unsupported zcash chain_ref %q", id.ChainRef)
		}
		return ChainZCash, nil
	case assetid.NamespaceMonero:
		if id.ChainRef != "mainnet" {
			return "", fmt.Errorf("unsupported monero chain_ref %q", id.ChainRef)
		}
		return ChainMonero, nil
	default:
		return "", fmt.Errorf("unsupported namespace %q", id.Namespace)
	}
}
