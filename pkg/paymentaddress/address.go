package paymentaddress

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

var (
	// ErrRequired is returned when a crypto payment address is empty.
	ErrRequired = errors.New("payment address is required")
	// ErrInvalid is returned when a payment address fails lightweight format checks.
	ErrInvalid = errors.New("payment address has invalid format")
)

// Validate checks a payment address against the payment coin's chain family.
// It is intentionally lightweight and does not require a wallet instance.
//
// Fiat payment coins always pass because refunds/settlements route through the
// fiat provider. UTXO-family and unknown crypto coins use a permissive length
// and whitespace sanity check; wallet-backed dispatch remains the final
// chain-aware validator.
func Validate(coin iwallet.CoinType, addr string) error {
	if coin.IsFiatPayment() {
		return nil
	}

	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ErrRequired
	}

	info, err := coin.CoinInfo()
	if err != nil {
		return validatePermissiveLength(addr)
	}

	switch {
	case info.Chain == iwallet.ChainSolana:
		if _, err := solana.PublicKeyFromBase58(addr); err != nil {
			return fmt.Errorf("%w: solana address: %v", ErrInvalid, err)
		}
		return nil

	case info.Chain == iwallet.ChainTRON:
		if len(addr) != 34 || !strings.HasPrefix(addr, "T") {
			return fmt.Errorf("%w: tron address must be 34 chars starting with 'T'", ErrInvalid)
		}
		if !isBase58Alphabet(addr) {
			return fmt.Errorf("%w: tron address contains non-base58 characters", ErrInvalid)
		}
		return nil

	case info.Chain == iwallet.ChainMonero:
		if len(addr) < 95 || len(addr) > 110 {
			return fmt.Errorf("%w: monero address length %d outside 95-110 range", ErrInvalid, len(addr))
		}
		first := addr[0]
		if first != '4' && first != '8' {
			return fmt.Errorf("%w: monero address must start with '4' or '8'", ErrInvalid)
		}
		return nil

	case info.IsEthTypeChain():
		if !common.IsHexAddress(addr) {
			return fmt.Errorf("%w: EVM address must be 40 hex chars (with optional 0x prefix)", ErrInvalid)
		}
		if common.HexToAddress(addr) == (common.Address{}) {
			return fmt.Errorf("%w: EVM address must not be the zero address", ErrInvalid)
		}
		return nil

	default:
		return validatePermissiveLength(addr)
	}
}

// ValidatePaymentCoinAddressMap validates a payment coin -> address map.
// Empty addresses are ignored so callers can clean sparse form state separately.
func ValidatePaymentCoinAddressMap(addrs map[string]string) error {
	if len(addrs) == 0 {
		return nil
	}
	for rawCoin, rawAddr := range addrs {
		coinKey := strings.TrimSpace(rawCoin)
		addr := strings.TrimSpace(rawAddr)
		if coinKey == "" {
			return fmt.Errorf("%w: payment coin key must not be empty", ErrInvalid)
		}
		if addr == "" {
			continue
		}
		normalized, err := iwallet.NormalizePaymentCoinIngress(coinKey)
		if err != nil {
			return fmt.Errorf("%w: invalid payment coin %q: %v", ErrInvalid, coinKey, err)
		}
		if err := Validate(normalized, addr); err != nil {
			return fmt.Errorf("coin %s: %w", normalized, err)
		}
	}
	return nil
}

// CanonicalizePaymentCoinAddressMap normalizes map keys to canonical payment
// coins and drops empty addresses. Multiple raw keys may collapse to one coin
// only when they carry the same address; conflicting duplicates are rejected.
func CanonicalizePaymentCoinAddressMap(addrs map[string]string) (map[string]string, error) {
	if len(addrs) == 0 {
		return nil, nil
	}
	out := make(map[string]string)
	for rawCoin, rawAddr := range addrs {
		coinKey := strings.TrimSpace(rawCoin)
		addr := strings.TrimSpace(rawAddr)
		if coinKey == "" {
			return nil, fmt.Errorf("%w: payment coin key must not be empty", ErrInvalid)
		}
		if addr == "" {
			continue
		}
		normalized, err := iwallet.NormalizePaymentCoinIngress(coinKey)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid payment coin %q: %v", ErrInvalid, coinKey, err)
		}
		if existing, ok := out[string(normalized)]; ok && existing != addr {
			return nil, fmt.Errorf("%w: multiple addresses for payment coin %s", ErrInvalid, normalized)
		}
		out[string(normalized)] = addr
	}
	return out, nil
}

// LookupByPaymentCoin returns the value for a payment coin from a map that may
// contain either the exact key, a normalized payment coin key, or legacy aliases
// (e.g. ETH vs crypto:eip155:1:native).
func LookupByPaymentCoin(values map[string]string, coin iwallet.CoinType) string {
	if len(values) == 0 || coin == "" {
		return ""
	}
	if value := strings.TrimSpace(values[string(coin)]); value != "" {
		return value
	}
	normalized, err := iwallet.NormalizePaymentCoinIngress(string(coin))
	if err != nil {
		return ""
	}
	if value := strings.TrimSpace(values[string(normalized)]); value != "" {
		return value
	}
	for rawCoin, rawAddr := range values {
		keyNorm, err := iwallet.NormalizePaymentCoinIngress(strings.TrimSpace(rawCoin))
		if err != nil || keyNorm != normalized {
			continue
		}
		if value := strings.TrimSpace(rawAddr); value != "" {
			return value
		}
	}
	return ""
}

func validatePermissiveLength(addr string) error {
	const minLen = 25
	const maxLen = 90
	if len(addr) < minLen || len(addr) > maxLen {
		return fmt.Errorf("%w: address length %d outside %d-%d range", ErrInvalid, len(addr), minLen, maxLen)
	}
	for _, r := range addr {
		if r == ' ' || r == '\t' || r == '\n' {
			return fmt.Errorf("%w: address contains whitespace", ErrInvalid)
		}
	}
	return nil
}

func isBase58Alphabet(s string) bool {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for _, r := range s {
		if !strings.ContainsRune(alphabet, r) {
			return false
		}
	}
	return true
}
