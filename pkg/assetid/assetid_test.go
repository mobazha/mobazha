package assetid

import (
	"strings"
	"sync"
	"testing"
)

func TestNormalizeEIP155Native(t *testing.T) {
	got, err := Normalize("  CRYPTO:EIP155:0001:NATIVE ")
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	want := "crypto:eip155:1:native"
	if got != want {
		t.Fatalf("unexpected normalized value: got %s want %s", got, want)
	}
}

func TestNormalizeEIP155ERC20Checksum(t *testing.T) {
	got, err := Normalize("crypto:eip155:56:erc20:0x55d398326f99059ff775485246999027b3197955")
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	want := "crypto:eip155:56:erc20:0x55d398326f99059fF775485246999027B3197955"
	if got != want {
		t.Fatalf("unexpected checksum normalization: got %s want %s", got, want)
	}
}

func TestNormalizeTRC20(t *testing.T) {
	got, err := Normalize("crypto:tron:mainnet:trc20:TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t")
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	want := "crypto:tron:mainnet:trc20:TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
	if got != want {
		t.Fatalf("unexpected normalized value: got %s want %s", got, want)
	}
}

func TestNormalizeSolanaSPL(t *testing.T) {
	got, err := Normalize("crypto:solana:MAINNET:spl:EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	want := "crypto:solana:mainnet:spl:EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	if got != want {
		t.Fatalf("unexpected normalized value: got %s want %s", got, want)
	}
}

func TestNormalizeBitcoinCashNative(t *testing.T) {
	got, err := Normalize("crypto:bitcoincash:MAINNET:native")
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	want := "crypto:bitcoincash:mainnet:native"
	if got != want {
		t.Fatalf("unexpected normalized value: got %s want %s", got, want)
	}
}

func TestNormalizeZCashNative(t *testing.T) {
	got, err := Normalize("crypto:zcash:MAINNET:native")
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	want := "crypto:zcash:mainnet:native"
	if got != want {
		t.Fatalf("unexpected normalized value: got %s want %s", got, want)
	}
}

func TestNormalizeRejectsLegacyChainToken(t *testing.T) {
	_, err := Normalize("TRXUSDT")
	if err == nil {
		t.Fatal("expected error for legacy CHAIN+TOKEN input")
	}
	if !IsCode(err, ErrCodeInvalidSegmentCount) {
		t.Fatalf("unexpected error code: %v", err)
	}
}

func TestNormalizeRejectsBadTRONAddress(t *testing.T) {
	_, err := Normalize("crypto:tron:mainnet:trc20:T123")
	if err == nil {
		t.Fatal("expected invalid tron address error")
	}
	if !IsCode(err, ErrCodeInvalidTRONAddress) {
		t.Fatalf("unexpected error code: %v", err)
	}
}

func TestNormalizeRejectsInvalidChainRef(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"solana invalid", "crypto:solana:foobar:native"},
		{"tron invalid", "crypto:tron:custom:native"},
		{"bch invalid", "crypto:bitcoincash:regtest:native"},
		{"zcash invalid", "crypto:zcash:regtest:native"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Normalize(tt.input)
			if err == nil {
				t.Fatalf("expected error for invalid chain_ref: %s", tt.input)
			}
			if !IsCode(err, ErrCodeInvalidChainRef) {
				t.Fatalf("expected ErrCodeInvalidChainRef, got: %v", err)
			}
		})
	}
}

func TestNormalizeAcceptsValidChainRefs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"solana mainnet", "crypto:solana:MAINNET:native", "crypto:solana:mainnet:native"},
		{"solana devnet", "crypto:solana:Devnet:native", "crypto:solana:devnet:native"},
		{"solana testnet", "crypto:solana:TESTNET:native", "crypto:solana:testnet:native"},
		{"tron mainnet", "crypto:tron:Mainnet:native", "crypto:tron:mainnet:native"},
		{"tron shasta", "crypto:tron:SHASTA:native", "crypto:tron:shasta:native"},
		{"tron nile", "crypto:tron:Nile:native", "crypto:tron:nile:native"},
		{"bch mainnet", "crypto:bitcoincash:MAINNET:native", "crypto:bitcoincash:mainnet:native"},
		{"bch testnet", "crypto:bitcoincash:TESTNET:native", "crypto:bitcoincash:testnet:native"},
		{"zcash mainnet", "crypto:zcash:MAINNET:native", "crypto:zcash:mainnet:native"},
		{"zcash testnet", "crypto:zcash:Testnet:native", "crypto:zcash:testnet:native"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Normalize(tt.input)
			if err != nil {
				t.Fatalf("Normalize(%s) failed: %v", tt.input, err)
			}
			if got != tt.expected {
				t.Fatalf("got %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestParseAndIsCanonical(t *testing.T) {
	raw := "crypto:eip155:137:erc20:0x3c499c542cef5e3811e1192ce70d8cc03d5c3359"
	id, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if id.Namespace != NamespaceEIP155 || id.Standard != StandardERC20 || id.ChainRef != "137" {
		t.Fatalf("unexpected parsed id: %+v", id)
	}
	if IsCanonical(raw) {
		t.Fatalf("expected non-canonical input to return false")
	}
	if !IsCanonical(id.String()) {
		t.Fatalf("expected normalized string to be canonical")
	}
}

func TestNormalizeRejectsEmpty(t *testing.T) {
	_, err := Normalize("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !IsCode(err, ErrCodeEmpty) {
		t.Fatalf("expected ErrCodeEmpty, got: %v", err)
	}
}

func TestNormalizeRejectsWhitespaceOnly(t *testing.T) {
	_, err := Normalize("   \t  ")
	if err == nil {
		t.Fatal("expected error for whitespace-only input")
	}
	if !IsCode(err, ErrCodeEmpty) {
		t.Fatalf("expected ErrCodeEmpty, got: %v", err)
	}
}

func TestNormalizeRejectsOverlongInput(t *testing.T) {
	long := "crypto:eip155:1:erc20:0x" + strings.Repeat("a", 500)
	_, err := Normalize(long)
	if err == nil {
		t.Fatal("expected error for overlong EVM address")
	}
}

func TestNormalizeRejectsWrongPrefix(t *testing.T) {
	_, err := Normalize("fiat:eip155:1:native")
	if err == nil {
		t.Fatal("expected error for non-crypto prefix")
	}
	if !IsCode(err, ErrCodeInvalidPrefix) {
		t.Fatalf("expected ErrCodeInvalidPrefix, got: %v", err)
	}
}

func TestNormalizeRejectsBadSegmentCounts(t *testing.T) {
	tests := []string{
		"crypto:eip155:1",
		"crypto:eip155:1:erc20:0xabc:extra",
		"crypto",
	}
	for _, input := range tests {
		_, err := Normalize(input)
		if err == nil {
			t.Fatalf("expected error for %q", input)
		}
		if !IsCode(err, ErrCodeInvalidSegmentCount) {
			t.Fatalf("input %q: expected ErrCodeInvalidSegmentCount, got: %v", input, err)
		}
	}
}

func TestNormalizeRejectsInvalidEVMAddress(t *testing.T) {
	tests := []string{
		"crypto:eip155:1:erc20:not-an-address",
		"crypto:eip155:1:erc20:0xZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ",
		"crypto:eip155:1:erc20:0x123",
	}
	for _, input := range tests {
		_, err := Normalize(input)
		if err == nil {
			t.Fatalf("expected error for invalid EVM address: %s", input)
		}
		if !IsCode(err, ErrCodeInvalidEVMAddress) {
			t.Fatalf("input %q: expected ErrCodeInvalidEVMAddress, got: %v", input, err)
		}
	}
}

func TestNormalizeRejectsInvalidBIP122ChainRef(t *testing.T) {
	tests := []string{
		"crypto:bip122:tooshort:native",
		"crypto:bip122:" + strings.Repeat("a", 64) + ":native",
		"crypto:bip122:" + strings.Repeat("g", 32) + ":native",
	}
	for _, input := range tests {
		_, err := Normalize(input)
		if err == nil {
			t.Fatalf("expected error for invalid bip122 chain_ref: %s", input)
		}
		if !IsCode(err, ErrCodeInvalidChainRef) {
			t.Fatalf("input %q: expected ErrCodeInvalidChainRef, got: %v", input, err)
		}
	}
}

func TestNormalizeRejectsInvalidEIP155ChainRef(t *testing.T) {
	tests := []string{
		"crypto:eip155:0:native",
		"crypto:eip155:abc:native",
		"crypto:eip155::native",
	}
	for _, input := range tests {
		_, err := Normalize(input)
		if err == nil {
			t.Fatalf("expected error for invalid eip155 chain_ref: %s", input)
		}
		if !IsCode(err, ErrCodeInvalidChainRef) {
			t.Fatalf("input %q: expected ErrCodeInvalidChainRef, got: %v", input, err)
		}
	}
}

func TestNormalizeRejectsUnsupportedStandard(t *testing.T) {
	_, err := Normalize("crypto:eip155:1:bep20:0x55d398326f99059ff775485246999027b3197955")
	if err == nil {
		t.Fatal("expected error for unsupported standard bep20 on eip155")
	}
	if !IsCode(err, ErrCodeInvalidStandard) {
		t.Fatalf("expected ErrCodeInvalidStandard, got: %v", err)
	}
}

func TestNormalizeIdempotent(t *testing.T) {
	inputs := []string{
		"crypto:eip155:1:native",
		"crypto:eip155:56:erc20:0x55d398326f99059fF775485246999027B3197955",
		"crypto:tron:mainnet:trc20:TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
		"crypto:solana:mainnet:spl:EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		"crypto:bip122:000000000019d6689c085ae165831e93:native",
		"crypto:bitcoincash:mainnet:native",
		"crypto:zcash:mainnet:native",
	}
	for _, input := range inputs {
		first, err := Normalize(input)
		if err != nil {
			t.Fatalf("Normalize(%s) first pass: %v", input, err)
		}
		second, err := Normalize(first)
		if err != nil {
			t.Fatalf("Normalize(%s) second pass: %v", first, err)
		}
		if first != second {
			t.Fatalf("idempotency failed: Normalize(%s)=%s, Normalize(%s)=%s", input, first, first, second)
		}
	}
}

func TestNormalizeConcurrentSafety(t *testing.T) {
	inputs := []string{
		"CRYPTO:EIP155:1:NATIVE",
		"crypto:eip155:56:erc20:0x55d398326f99059ff775485246999027b3197955",
		"crypto:tron:MAINNET:trc20:TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
		"crypto:solana:Mainnet:native",
		"crypto:bip122:000000000019D6689C085AE165831E93:native",
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			input := inputs[idx%len(inputs)]
			result, err := Normalize(input)
			if err != nil {
				t.Errorf("goroutine %d: Normalize(%s) failed: %v", idx, input, err)
				return
			}
			if result == "" {
				t.Errorf("goroutine %d: empty result", idx)
			}
		}(i)
	}
	wg.Wait()
}

func TestNormalizeEIP155ChecksumPreserved(t *testing.T) {
	input := "crypto:eip155:1:erc20:0xA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48"
	got, err := Normalize(input)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	expected := "crypto:eip155:1:erc20:0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
	if got != expected {
		t.Fatalf("EIP-55 checksum mismatch: got %s, want %s", got, expected)
	}
}

func TestNormalizeRejectsInvalidSolanaMint(t *testing.T) {
	_, err := Normalize("crypto:solana:mainnet:spl:INVALID!!MINT")
	if err == nil {
		t.Fatal("expected error for invalid Solana mint")
	}
	if !IsCode(err, ErrCodeInvalidSolanaMint) {
		t.Fatalf("expected ErrCodeInvalidSolanaMint, got: %v", err)
	}
}

func TestNormalizeRejectsTRONAddressPrefix(t *testing.T) {
	_, err := Normalize("crypto:tron:mainnet:trc20:0x55d398326f99059ff775485246999027b3197955")
	if err == nil {
		t.Fatal("expected error for EVM-style address on TRON")
	}
}
