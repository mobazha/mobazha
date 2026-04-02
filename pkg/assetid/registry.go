package assetid

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type PriceSource struct {
	Provider string
	ID       string
}

type PricingMeta struct {
	// Key is provider-agnostic pricing semantics key (for example BTC/ETH/USDT).
	// Assets that should share one market price can share one key.
	Key string

	// Sources stores provider-native pricing identifiers in priority order.
	// The first element is primary for MVP flows; later elements are reserved for fallback.
	Sources []PriceSource

	// PeggedTo is optional and can describe peg relationship, for example "USD".
	PeggedTo string
}

type Definition struct {
	// Code is the current business-facing asset code used in existing flows
	// (for example BTC, TRXUSDT). It remains useful while callers migrate.
	Code string

	// AssetID is the canonical crypto:* identifier.
	AssetID string

	// Pricing contains provider-agnostic key plus provider-specific source mapping.
	Pricing PricingMeta

	DisplaySymbol string
	DisplayName   string
	Decimals      uint8
	Runtime       RuntimeMeta
}

type RuntimeMeta struct {
	Bip44Code     uint
	BlockInterval time.Duration
}

func (d Definition) PriceIDForProvider(provider string) (string, bool) {
	targetProvider := strings.TrimSpace(strings.ToLower(provider))
	if targetProvider == "" {
		return "", false
	}

	for _, source := range d.Pricing.Sources {
		sourceProvider := strings.TrimSpace(strings.ToLower(source.Provider))
		sourceID := strings.TrimSpace(source.ID)
		if sourceProvider == targetProvider && sourceID != "" {
			return sourceID, true
		}
	}
	return "", false
}

type Registry struct {
	byID    map[string]Definition
	byTuple map[string]string
}

func NewRegistry(definitions []Definition) (*Registry, error) {
	r := &Registry{
		byID:    make(map[string]Definition, len(definitions)),
		byTuple: make(map[string]string, len(definitions)),
	}

	for _, def := range definitions {
		if def.AssetID == "" {
			return nil, fmt.Errorf("asset code %q has empty asset id", def.Code)
		}
		normalized, err := Normalize(def.AssetID)
		if err != nil {
			return nil, fmt.Errorf("asset code %q has invalid asset id %q: %w", def.Code, def.AssetID, err)
		}
		def.AssetID = normalized

		if _, exists := r.byID[def.AssetID]; exists {
			return nil, fmt.Errorf("duplicate asset id %s", def.AssetID)
		}

		parsed, err := Parse(def.AssetID)
		if err != nil {
			return nil, fmt.Errorf("asset code %q parse failed: %w", def.Code, err)
		}
		tuple := tupleKey(parsed)
		if existing, exists := r.byTuple[tuple]; exists {
			return nil, fmt.Errorf("duplicate chain/standard/ref tuple for %s and %s", existing, def.AssetID)
		}

		def.Pricing.Key = strings.TrimSpace(strings.ToUpper(def.Pricing.Key))
		def.Pricing.PeggedTo = strings.TrimSpace(strings.ToUpper(def.Pricing.PeggedTo))

		hasPricingKey := def.Pricing.Key != ""
		hasPricingSources := len(def.Pricing.Sources) > 0
		if hasPricingKey != hasPricingSources {
			return nil, fmt.Errorf("asset code %q pricing key/sources must be configured together", def.Code)
		}
		if def.Pricing.PeggedTo != "" && !hasPricingKey {
			return nil, fmt.Errorf("asset code %q has peggedTo without pricing key", def.Code)
		}

		seenProviders := make(map[string]struct{}, len(def.Pricing.Sources))
		for _, source := range def.Pricing.Sources {
			provider := strings.TrimSpace(strings.ToLower(source.Provider))
			id := strings.TrimSpace(strings.ToLower(source.ID))

			if provider == "" || id == "" {
				return nil, fmt.Errorf("asset code %q has invalid price source provider=%q id=%q", def.Code, source.Provider, source.ID)
			}
			if _, exists := seenProviders[provider]; exists {
				return nil, fmt.Errorf("asset code %q has duplicate price source provider %q", def.Code, provider)
			}
			seenProviders[provider] = struct{}{}
		}

		r.byTuple[tuple] = def.AssetID
		r.byID[def.AssetID] = def
	}

	return r, nil
}

func (r *Registry) Lookup(assetID string) (Definition, error) {
	normalized, err := Normalize(assetID)
	if err != nil {
		return Definition{}, err
	}
	def, ok := r.byID[normalized]
	if !ok {
		return Definition{}, newError(ErrCodeUnknownAsset, fmt.Sprintf("asset id %q", normalized))
	}
	return def, nil
}

func (r *Registry) List() []Definition {
	result := make([]Definition, 0, len(r.byID))
	for _, def := range r.byID {
		result = append(result, def)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].AssetID < result[j].AssetID
	})
	return result
}

func tupleKey(id ID) string {
	if id.IsNative() {
		return fmt.Sprintf("%s|%s|native", id.Namespace, id.ChainRef)
	}
	return fmt.Sprintf("%s|%s|%s|%s", id.Namespace, id.ChainRef, id.Standard, id.AssetRef)
}

func DefaultRegistry() *Registry {
	defaultRegistryOnce.Do(func() {
		r, err := NewRegistry(defaultDefinitions)
		if err != nil {
			panic(err)
		}
		defaultRegistry = r
	})
	return defaultRegistry
}

var (
	defaultRegistryOnce sync.Once
	defaultRegistry     *Registry
)

func pricingMeta(key, coingeckoID string, additionalSources ...PriceSource) PricingMeta {
	sources := make([]PriceSource, 0, 1+len(additionalSources))
	sources = append(sources, PriceSource{Provider: "coingecko", ID: coingeckoID})
	sources = append(sources, additionalSources...)
	return PricingMeta{
		Key:     key,
		Sources: sources,
	}
}

func pricingMetaUSDPegged(key, coingeckoID string, additionalSources ...PriceSource) PricingMeta {
	meta := pricingMeta(key, coingeckoID, additionalSources...)
	meta.PeggedTo = "USD"
	return meta
}

func runtimeMeta(blockInterval time.Duration, bip44Code uint) RuntimeMeta {
	return RuntimeMeta{
		Bip44Code:     bip44Code,
		BlockInterval: blockInterval,
	}
}

var defaultDefinitions = []Definition{
	{Code: "BTC", AssetID: "crypto:bip122:000000000019d6689c085ae165831e93:native", Pricing: pricingMeta("BTC", "bitcoin", PriceSource{Provider: "binance", ID: "BTCUSDT"}), DisplaySymbol: "BTC", DisplayName: "Bitcoin", Decimals: 8, Runtime: runtimeMeta(10*time.Minute, 0)},
	{Code: "BCH", AssetID: "crypto:bitcoincash:mainnet:native", Pricing: pricingMeta("BCH", "bitcoin-cash"), DisplaySymbol: "BCH", DisplayName: "Bitcoin Cash", Decimals: 8, Runtime: runtimeMeta(10*time.Minute, 145)},
	{Code: "LTC", AssetID: "crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native", Pricing: pricingMeta("LTC", "litecoin", PriceSource{Provider: "binance", ID: "LTCUSDT"}), DisplaySymbol: "LTC", DisplayName: "Litecoin", Decimals: 8, Runtime: runtimeMeta(150*time.Second, 2)},
	{Code: "ZEC", AssetID: "crypto:zcash:mainnet:native", Pricing: pricingMeta("ZEC", "zcash"), DisplaySymbol: "ZEC", DisplayName: "Zcash", Decimals: 8, Runtime: runtimeMeta(150*time.Second, 133)},
	{Code: "ETH", AssetID: "crypto:eip155:1:native", Pricing: pricingMeta("ETH", "ethereum", PriceSource{Provider: "binance", ID: "ETHUSDT"}), DisplaySymbol: "ETH", DisplayName: "Ethereum", Decimals: 18, Runtime: runtimeMeta(12*time.Second, 60)},
	{Code: "ETHUSDT", AssetID: "crypto:eip155:1:erc20:0xF36BFeE8fd7F1950c0129714Faf6d1e1F94a66AA", Pricing: pricingMetaUSDPegged("USDT", "tether"), DisplaySymbol: "USDT", DisplayName: "Tether USD", Decimals: 6, Runtime: runtimeMeta(12*time.Second, 0)},
	{Code: "ETHUSDC", AssetID: "crypto:eip155:1:erc20:0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", Pricing: pricingMetaUSDPegged("USDC", "usd-coin"), DisplaySymbol: "USDC", DisplayName: "USD Coin", Decimals: 6, Runtime: runtimeMeta(12*time.Second, 0)},
	{Code: "DAI", AssetID: "crypto:eip155:1:erc20:0x6B175474E89094C44Da98b954EedeAC495271d0F", Pricing: pricingMeta("DAI", "dai"), DisplaySymbol: "DAI", DisplayName: "Dai Stablecoin", Decimals: 18, Runtime: runtimeMeta(12*time.Second, 0)},
	{Code: "BNB", AssetID: "crypto:eip155:56:native", Pricing: pricingMeta("BNB", "binancecoin", PriceSource{Provider: "binance", ID: "BNBUSDT"}), DisplaySymbol: "BNB", DisplayName: "BNB", Decimals: 18, Runtime: runtimeMeta(3*time.Second, 60)},
	{Code: "BSCUSDT", AssetID: "crypto:eip155:56:erc20:0x55d398326f99059fF775485246999027B3197955", Pricing: pricingMetaUSDPegged("USDT", "tether"), DisplaySymbol: "USDT", DisplayName: "Tether USD on BSC", Decimals: 6, Runtime: runtimeMeta(3*time.Second, 0)},
	{Code: "BSCUSDC", AssetID: "crypto:eip155:56:erc20:0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d", Pricing: pricingMetaUSDPegged("USDC", "usd-coin"), DisplaySymbol: "USDC", DisplayName: "USD Coin on BSC", Decimals: 6, Runtime: runtimeMeta(3*time.Second, 0)},
	{Code: "BUSD", AssetID: "crypto:eip155:56:erc20:0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56", Pricing: pricingMetaUSDPegged("BUSD", "binance-usd"), DisplaySymbol: "BUSD", DisplayName: "Binance USD on BSC", Decimals: 18, Runtime: runtimeMeta(3*time.Second, 0)},
	{Code: "BASEETH", AssetID: "crypto:eip155:8453:native", Pricing: pricingMeta("ETH", "ethereum", PriceSource{Provider: "binance", ID: "ETHUSDT"}), DisplaySymbol: "ETH", DisplayName: "Ethereum on Base", Decimals: 18, Runtime: runtimeMeta(3*time.Second, 60)},
	{Code: "BASEUSDT", AssetID: "crypto:eip155:8453:erc20:0xfde4C96c8593536E31F229EA8f37b2ADa2699bb2", Pricing: pricingMetaUSDPegged("USDT", "tether"), DisplaySymbol: "USDT", DisplayName: "Tether USD on Base", Decimals: 6, Runtime: runtimeMeta(3*time.Second, 0)},
	{Code: "BASEUSDC", AssetID: "crypto:eip155:8453:erc20:0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913", Pricing: pricingMetaUSDPegged("USDC", "usd-coin"), DisplaySymbol: "USDC", DisplayName: "USD Coin on Base", Decimals: 6, Runtime: runtimeMeta(3*time.Second, 0)},
	{Code: "CFX", AssetID: "crypto:eip155:1030:native", Pricing: pricingMeta("CFX", "conflux-token", PriceSource{Provider: "binance", ID: "CFXUSDT"}), DisplaySymbol: "CFX", DisplayName: "Conflux", Decimals: 18, Runtime: runtimeMeta(3*time.Second, 60)},
	{Code: "MATIC", AssetID: "crypto:eip155:137:native", Pricing: pricingMeta("MATIC", "matic-network", PriceSource{Provider: "binance", ID: "MATICUSDT"}), DisplaySymbol: "MATIC", DisplayName: "Polygon", Decimals: 18, Runtime: runtimeMeta(3*time.Second, 60)},
	{Code: "MATICUSDT", AssetID: "crypto:eip155:137:erc20:0xc2132D05D31c914a87C6611C10748AEb04B58e8F", Pricing: pricingMetaUSDPegged("USDT", "tether"), DisplaySymbol: "USDT", DisplayName: "Tether USD on Polygon", Decimals: 6, Runtime: runtimeMeta(3*time.Second, 0)},
	{Code: "MATICUSDC", AssetID: "crypto:eip155:137:erc20:0x3c499c542cEF5E3811e1192ce70d8cC03d5c3359", Pricing: pricingMetaUSDPegged("USDC", "usd-coin"), DisplaySymbol: "USDC", DisplayName: "USD Coin on Polygon", Decimals: 6, Runtime: runtimeMeta(3*time.Second, 0)},
	{Code: "SOL", AssetID: "crypto:solana:mainnet:native", Pricing: pricingMeta("SOL", "solana", PriceSource{Provider: "binance", ID: "SOLUSDT"}), DisplaySymbol: "SOL", DisplayName: "Solana", Decimals: 9, Runtime: runtimeMeta(400*time.Millisecond, 501)},
	{Code: "SOLUSDT", AssetID: "crypto:solana:mainnet:spl:Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB", Pricing: pricingMetaUSDPegged("USDT", "tether"), DisplaySymbol: "USDT", DisplayName: "Tether USD on Solana", Decimals: 6, Runtime: runtimeMeta(400*time.Millisecond, 0)},
	{Code: "SOLUSDC", AssetID: "crypto:solana:mainnet:spl:EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", Pricing: pricingMetaUSDPegged("USDC", "usd-coin"), DisplaySymbol: "USDC", DisplayName: "USD Coin on Solana", Decimals: 6, Runtime: runtimeMeta(400*time.Millisecond, 0)},
	{Code: "TRX", AssetID: "crypto:tron:mainnet:native", Pricing: pricingMeta("TRX", "tron", PriceSource{Provider: "binance", ID: "TRXUSDT"}), DisplaySymbol: "TRX", DisplayName: "TRON", Decimals: 6, Runtime: runtimeMeta(3*time.Second, 195)},
	{Code: "TRXUSDT", AssetID: "crypto:tron:mainnet:trc20:TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", Pricing: pricingMetaUSDPegged("USDT", "tether"), DisplaySymbol: "USDT", DisplayName: "Tether USD on TRON", Decimals: 6, Runtime: runtimeMeta(3*time.Second, 0)},
}
