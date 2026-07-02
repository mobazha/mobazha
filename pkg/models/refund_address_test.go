package models

import (
	"errors"
	"strings"
	"testing"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// TestValidateRefundAddress_Fiat covers the contract that fiat orders always
// pass validation — refund routing for Stripe / PayPal etc. is handled off-chain
// by the FiatProvider, not via Order.RefundAddress.
func TestValidateRefundAddress_Fiat(t *testing.T) {
	cases := []struct {
		name string
		coin iwallet.CoinType
		addr string
	}{
		{"stripe USD empty", "fiat:stripe:USD", ""},
		{"stripe USD garbage", "fiat:stripe:USD", "not-an-address"},
		{"paypal EUR", "fiat:paypal:EUR", "buyer@example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateRefundAddress(tc.coin, tc.addr); err != nil {
				t.Fatalf("expected fiat refund address validation to pass, got %v", err)
			}
		})
	}
}

// Canonical coin codes used across the tests — production callers always pass
// these CAIP-2-derived strings, never legacy aliases like "ETH" / "BTC".
const (
	canonicalETH = iwallet.CoinType("crypto:eip155:1:native")
	canonicalBTC = iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native")
	canonicalLTC = iwallet.CoinType("crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native")
	canonicalSOL = iwallet.CoinType("crypto:solana:mainnet:native")
	canonicalTRX = iwallet.CoinType("crypto:tron:mainnet:native")
	canonicalXMR = iwallet.CoinType("crypto:monero:mainnet:native")
)

// TestValidateRefundAddress_EmptyCrypto verifies that crypto orders require
// a non-empty refund address (Monitor-Driven Payment §P0-3).
func TestValidateRefundAddress_EmptyCrypto(t *testing.T) {
	cases := []iwallet.CoinType{canonicalETH, canonicalBTC, canonicalSOL, canonicalTRX}
	for _, coin := range cases {
		t.Run(string(coin), func(t *testing.T) {
			err := ValidateRefundAddress(coin, "")
			if !errors.Is(err, ErrRefundAddressRequired) {
				t.Fatalf("expected ErrRefundAddressRequired, got %v", err)
			}
			// Whitespace-only must be treated as empty too.
			if err := ValidateRefundAddress(coin, "   \t\n"); !errors.Is(err, ErrRefundAddressRequired) {
				t.Fatalf("whitespace-only address: expected ErrRefundAddressRequired, got %v", err)
			}
		})
	}
}

// TestValidateRefundAddress_EVM exercises the EIP-55 / lowercased / zero-address
// branches of the EVM validator.
func TestValidateRefundAddress_EVM(t *testing.T) {
	cases := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"valid lowercase", "0x742d35cc6634c0532925a3b844bc454e4438f44e", false},
		{"valid EIP-55", "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", false},
		{"valid no prefix", "742d35cc6634c0532925a3b844bc454e4438f44e", false},
		{"zero address rejected", "0x0000000000000000000000000000000000000000", true},
		{"too short", "0x742d35cc", true},
		{"non-hex", "0xZZZd35cc6634c0532925a3b844bc454e4438f44e", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRefundAddress(canonicalETH, tc.addr)
			if tc.wantErr {
				if !errors.Is(err, ErrRefundAddressInvalid) {
					t.Fatalf("expected ErrRefundAddressInvalid, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}

// TestValidateRefundAddress_Solana verifies base58 pubkey parsing matches the
// production rule used elsewhere in the codebase (solana.PublicKeyFromBase58).
func TestValidateRefundAddress_Solana(t *testing.T) {
	cases := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		// 32-byte base58 — system program ID
		{"valid system program", "11111111111111111111111111111111", false},
		// Random valid mainnet wallet (Token Program)
		{"valid token program", "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", false},
		{"too short", "1234567", true},
		{"contains O (invalid base58)", "11111111111O11111111111111111111", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRefundAddress(canonicalSOL, tc.addr)
			if tc.wantErr {
				if !errors.Is(err, ErrRefundAddressInvalid) {
					t.Fatalf("expected ErrRefundAddressInvalid, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}

// TestValidateRefundAddress_TRON verifies the prefix / length / base58 sanity
// checks specific to TRON addresses.
func TestValidateRefundAddress_TRON(t *testing.T) {
	cases := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		// Real mainnet address pattern (USDT-TRC20 contract)
		{"valid", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", false},
		{"missing T prefix", "BR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", true},
		{"too short", "TR7NHqj", true},
		{"too long", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6tEXTRA", true},
		{"non-base58 char (0)", "TR0NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRefundAddress(canonicalTRX, tc.addr)
			if tc.wantErr {
				if !errors.Is(err, ErrRefundAddressInvalid) {
					t.Fatalf("expected ErrRefundAddressInvalid, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}

// TestValidateRefundAddress_UTXO uses the permissive length check — chain-aware
// validation is intentionally deferred to refund-dispatch time.
func TestValidateRefundAddress_UTXO(t *testing.T) {
	cases := []struct {
		name    string
		coin    iwallet.CoinType
		addr    string
		wantErr bool
	}{
		// Bitcoin mainnet bech32 (BIP-173)
		{"BTC bech32", canonicalBTC, "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4", false},
		// Legacy P2PKH
		{"BTC P2PKH", canonicalBTC, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", false},
		// Litecoin bech32
		{"LTC bech32", canonicalLTC, "ltc1qw508d6qejxtdg4y5r3zarvary0c5xw7kgmn4n9", false},
		{"too short", canonicalBTC, "1abc", true},
		{"too long", canonicalBTC, strings.Repeat("a", 95), true},
		{"contains whitespace", canonicalBTC, "1A1zP1eP5QGefi2D MPTfTL5SLmv7Div", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRefundAddress(tc.coin, tc.addr)
			if tc.wantErr {
				if !errors.Is(err, ErrRefundAddressInvalid) {
					t.Fatalf("expected ErrRefundAddressInvalid, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}

// TestValidateRefundAddress_Monero covers the XMR length + prefix sanity rules.
func TestValidateRefundAddress_Monero(t *testing.T) {
	const valid = "44AFFq5kSiGBoZ4NMDwYtN18obc8AemS33DBLWs3H7otXft3XjrpDtQGv7SqSsaBYBb98uNbr2VBBEt7f2wfn3RVGQBEP3A"
	cases := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"valid mainnet", valid, false},
		{"too short", valid[:90], true},
		{"wrong prefix", "5" + valid[1:], true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRefundAddress(canonicalXMR, tc.addr)
			if tc.wantErr {
				if !errors.Is(err, ErrRefundAddressInvalid) {
					t.Fatalf("expected ErrRefundAddressInvalid, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}

// TestValidateRefundAddress_UnknownCoin verifies that unknown coin codes fall
// through to the permissive length validator instead of hard-failing — keeps
// the validator forward-compatible with future CAIP-2 codes.
func TestValidateRefundAddress_UnknownCoin(t *testing.T) {
	const unknownCoin iwallet.CoinType = "FUTURE-CAIP2-XYZ"
	const reasonableAddr = "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4"

	if err := ValidateRefundAddress(unknownCoin, reasonableAddr); err != nil {
		t.Fatalf("expected unknown coin with reasonable address to pass, got %v", err)
	}
	if err := ValidateRefundAddress(unknownCoin, "x"); !errors.Is(err, ErrRefundAddressInvalid) {
		t.Fatalf("expected ErrRefundAddressInvalid for short input, got %v", err)
	}
	if err := ValidateRefundAddress(unknownCoin, ""); !errors.Is(err, ErrRefundAddressRequired) {
		t.Fatalf("expected ErrRefundAddressRequired for empty input, got %v", err)
	}
}
