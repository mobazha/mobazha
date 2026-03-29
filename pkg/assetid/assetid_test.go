package assetid

import "testing"

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
