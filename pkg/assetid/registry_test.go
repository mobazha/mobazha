package assetid

import "testing"

func TestNewRegistry_DuplicateAssetID(t *testing.T) {
	_, err := NewRegistry([]Definition{
		{Code: "A", AssetID: "crypto:eip155:1:native"},
		{Code: "B", AssetID: "crypto:eip155:1:native"},
	})
	if err == nil {
		t.Fatal("expected duplicate asset id error")
	}
}

func TestNewRegistry_DuplicateTuple(t *testing.T) {
	_, err := NewRegistry([]Definition{
		{Code: "A", AssetID: "crypto:eip155:1:erc20:0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"},
		{Code: "B", AssetID: "crypto:eip155:1:erc20:0xA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48"},
	})
	if err == nil {
		t.Fatal("expected duplicate tuple error")
	}
}

func TestNewRegistry_InvalidPriceSource(t *testing.T) {
	_, err := NewRegistry([]Definition{
		{
			Code:    "A",
			AssetID: "crypto:eip155:1:native",
			Pricing: PricingMeta{
				Key:     "ETH",
				Sources: []PriceSource{{Provider: "coingecko", ID: ""}},
			},
		},
	})
	if err == nil {
		t.Fatal("expected invalid price source error")
	}
}

func TestNewRegistry_PricingRequiresKeyAndSourcesTogether(t *testing.T) {
	_, err := NewRegistry([]Definition{
		{
			Code:    "A",
			AssetID: "crypto:eip155:1:native",
			Pricing: PricingMeta{
				Key: "ETH",
			},
		},
	})
	if err == nil {
		t.Fatal("expected pricing key/sources validation error")
	}

	_, err = NewRegistry([]Definition{
		{
			Code:    "A",
			AssetID: "crypto:eip155:1:native",
			Pricing: PricingMeta{
				Sources: []PriceSource{{Provider: "coingecko", ID: "ethereum"}},
			},
		},
	})
	if err == nil {
		t.Fatal("expected pricing key/sources validation error")
	}
}

func TestNewRegistry_DuplicatePriceSourceProvider(t *testing.T) {
	_, err := NewRegistry([]Definition{
		{
			Code:    "A",
			AssetID: "crypto:eip155:1:native",
			Pricing: PricingMeta{
				Key: "ETH",
				Sources: []PriceSource{
					{Provider: "coingecko", ID: "ethereum"},
					{Provider: "CoinGecko", ID: "ethereum"},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate price source provider error")
	}
}

func TestDefaultRegistryLookup(t *testing.T) {
	r := DefaultRegistry()

	def, err := r.Lookup("crypto:tron:mainnet:trc20:TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t")
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if def.Code != "TRXUSDT" {
		t.Fatalf("unexpected code: %s", def.Code)
	}
	if def.Decimals != 6 {
		t.Fatalf("unexpected decimals: %d", def.Decimals)
	}
	if def.Pricing.Key != "USDT" {
		t.Fatalf("unexpected price key: %s", def.Pricing.Key)
	}
	if def.Pricing.PeggedTo != "USD" {
		t.Fatalf("unexpected pegged to: %s", def.Pricing.PeggedTo)
	}
	priceID, ok := def.PriceIDForProvider("coingecko")
	if !ok {
		t.Fatal("expected coingecko price source")
	}
	if priceID != "tether" {
		t.Fatalf("unexpected price id: %s", priceID)
	}

	eth, err := r.Lookup("crypto:eip155:1:native")
	if err != nil {
		t.Fatalf("lookup eth failed: %v", err)
	}
	binanceETH, ok := eth.PriceIDForProvider("binance")
	if !ok {
		t.Fatal("expected ETH binance price source")
	}
	if binanceETH != "ETHUSDT" {
		t.Fatalf("unexpected ETH binance id: %s", binanceETH)
	}

	baseETH, err := r.Lookup("crypto:eip155:8453:native")
	if err != nil {
		t.Fatalf("lookup baseeth failed: %v", err)
	}
	binanceBaseETH, ok := baseETH.PriceIDForProvider("binance")
	if !ok {
		t.Fatal("expected BASEETH binance price source")
	}
	if binanceBaseETH != "ETHUSDT" {
		t.Fatalf("unexpected BASEETH binance id: %s", binanceBaseETH)
	}

	dai, err := r.Lookup("crypto:eip155:1:erc20:0x6B175474E89094C44Da98b954EedeAC495271d0F")
	if err != nil {
		t.Fatalf("lookup dai failed: %v", err)
	}
	if dai.Code != "DAI" {
		t.Fatalf("unexpected DAI code: %s", dai.Code)
	}

	bch, err := r.Lookup("crypto:bitcoincash:mainnet:native")
	if err != nil {
		t.Fatalf("lookup bch failed: %v", err)
	}
	if bch.Code != "BCH" {
		t.Fatalf("unexpected BCH code: %s", bch.Code)
	}

	zec, err := r.Lookup("crypto:zcash:mainnet:native")
	if err != nil {
		t.Fatalf("lookup zec failed: %v", err)
	}
	if zec.Code != "ZEC" {
		t.Fatalf("unexpected ZEC code: %s", zec.Code)
	}

	busd, err := r.Lookup("crypto:eip155:56:erc20:0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56")
	if err != nil {
		t.Fatalf("lookup busd failed: %v", err)
	}
	if busd.Code != "BUSD" {
		t.Fatalf("unexpected BUSD code: %s", busd.Code)
	}

	cfx, err := r.Lookup("crypto:eip155:1030:native")
	if err != nil {
		t.Fatalf("lookup cfx failed: %v", err)
	}
	if cfx.Code != "CFX" {
		t.Fatalf("unexpected CFX code: %s", cfx.Code)
	}

	_, err = r.Lookup("TRXUSDT")
	if err == nil {
		t.Fatal("expected lookup failure for non-canonical input")
	}
}

func TestDefaultRegistrySingleton(t *testing.T) {
	r1 := DefaultRegistry()
	r2 := DefaultRegistry()
	if r1 != r2 {
		t.Fatal("expected DefaultRegistry to return cached singleton instance")
	}
}

func TestDefinition_PriceIDForProvider(t *testing.T) {
	def := Definition{
		Code:    "ETH",
		AssetID: "crypto:eip155:1:native",
		Pricing: PricingMeta{
			Key: "ETH",
			Sources: []PriceSource{
				{Provider: "coingecko", ID: "ethereum"},
				{Provider: "binance", ID: "ETHUSDT"},
			},
		},
	}

	priceID, ok := def.PriceIDForProvider("CoinGecko")
	if !ok {
		t.Fatal("expected coingecko source")
	}
	if priceID != "ethereum" {
		t.Fatalf("unexpected coingecko id: %s", priceID)
	}

	binanceID, ok := def.PriceIDForProvider("BINANCE")
	if !ok {
		t.Fatal("expected binance source")
	}
	if binanceID != "ETHUSDT" {
		t.Fatalf("unexpected binance id: %s", binanceID)
	}
}
