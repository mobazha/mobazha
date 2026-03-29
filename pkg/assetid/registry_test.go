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

	_, err = r.Lookup("TRXUSDT")
	if err == nil {
		t.Fatal("expected lookup failure for non-canonical input")
	}
}
