package assetid

import (
	"fmt"
	"sort"
)

type Definition struct {
	// Code is the current business-facing asset code used in existing flows
	// (for example BTC, TRXUSDT). It remains useful while callers migrate.
	Code string

	// AssetID is the canonical crypto:* identifier.
	AssetID string

	DisplaySymbol string
	DisplayName   string
	Decimals      uint8
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
	r, err := NewRegistry(defaultDefinitions)
	if err != nil {
		panic(err)
	}
	return r
}

var defaultDefinitions = []Definition{
	{Code: "BTC", AssetID: "crypto:bip122:000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f:native", DisplaySymbol: "BTC", DisplayName: "Bitcoin", Decimals: 8},
	{Code: "LTC", AssetID: "crypto:bip122:12a765e31ffd4059bada1e25190f6e98c99d9714d334efa41a195a7e7e04bfe2:native", DisplaySymbol: "LTC", DisplayName: "Litecoin", Decimals: 8},
	{Code: "ETH", AssetID: "crypto:eip155:1:native", DisplaySymbol: "ETH", DisplayName: "Ethereum", Decimals: 18},
	{Code: "ETHUSDT", AssetID: "crypto:eip155:1:erc20:0xF36BFeE8fd7F1950c0129714Faf6d1e1F94a66AA", DisplaySymbol: "USDT", DisplayName: "Tether USD", Decimals: 6},
	{Code: "ETHUSDC", AssetID: "crypto:eip155:1:erc20:0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", DisplaySymbol: "USDC", DisplayName: "USD Coin", Decimals: 6},
	{Code: "DAI", AssetID: "crypto:eip155:1:erc20:0x6B175474E89094C44Da98b954EedeAC495271d0F", DisplaySymbol: "DAI", DisplayName: "Dai Stablecoin", Decimals: 18},
	{Code: "BNB", AssetID: "crypto:eip155:56:native", DisplaySymbol: "BNB", DisplayName: "BNB", Decimals: 18},
	{Code: "BSCUSDT", AssetID: "crypto:eip155:56:erc20:0x55d398326f99059fF775485246999027B3197955", DisplaySymbol: "USDT", DisplayName: "Tether USD on BSC", Decimals: 6},
	{Code: "BSCUSDC", AssetID: "crypto:eip155:56:erc20:0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d", DisplaySymbol: "USDC", DisplayName: "USD Coin on BSC", Decimals: 6},
	{Code: "BUSD", AssetID: "crypto:eip155:56:erc20:0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56", DisplaySymbol: "BUSD", DisplayName: "Binance USD on BSC", Decimals: 18},
	{Code: "BASEETH", AssetID: "crypto:eip155:8453:native", DisplaySymbol: "ETH", DisplayName: "Ethereum on Base", Decimals: 18},
	{Code: "BASEUSDT", AssetID: "crypto:eip155:8453:erc20:0xfde4C96c8593536E31F229EA8f37b2ADa2699bb2", DisplaySymbol: "USDT", DisplayName: "Tether USD on Base", Decimals: 6},
	{Code: "BASEUSDC", AssetID: "crypto:eip155:8453:erc20:0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913", DisplaySymbol: "USDC", DisplayName: "USD Coin on Base", Decimals: 6},
	{Code: "MATIC", AssetID: "crypto:eip155:137:native", DisplaySymbol: "MATIC", DisplayName: "Polygon", Decimals: 18},
	{Code: "MATICUSDT", AssetID: "crypto:eip155:137:erc20:0xc2132D05D31c914a87C6611C10748AEb04B58e8F", DisplaySymbol: "USDT", DisplayName: "Tether USD on Polygon", Decimals: 6},
	{Code: "MATICUSDC", AssetID: "crypto:eip155:137:erc20:0x3c499c542cEF5E3811e1192ce70d8cC03d5c3359", DisplaySymbol: "USDC", DisplayName: "USD Coin on Polygon", Decimals: 6},
	{Code: "SOL", AssetID: "crypto:solana:mainnet:native", DisplaySymbol: "SOL", DisplayName: "Solana", Decimals: 9},
	{Code: "SOLUSDT", AssetID: "crypto:solana:mainnet:spl:Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB", DisplaySymbol: "USDT", DisplayName: "Tether USD on Solana", Decimals: 6},
	{Code: "SOLUSDC", AssetID: "crypto:solana:mainnet:spl:EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", DisplaySymbol: "USDC", DisplayName: "USD Coin on Solana", Decimals: 6},
	{Code: "TRX", AssetID: "crypto:tron:mainnet:native", DisplaySymbol: "TRX", DisplayName: "TRON", Decimals: 6},
	{Code: "TRXUSDT", AssetID: "crypto:tron:mainnet:trc20:TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", DisplaySymbol: "USDT", DisplayName: "Tether USD on TRON", Decimals: 6},
}
