package models

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ErrRefundAddressRequired is returned when an order requires a buyer-declared
// refund address but the caller passed an empty string. The Monitor-Driven
// Payment model (docs/escrow/MONITOR_DRIVEN_PAYMENT.md §P0-3) makes RefundAddress
// a hard requirement for CEX direct-pay scenarios where the on-chain sender is an
// exchange omnibus address that must NEVER receive a refund.
var ErrRefundAddressRequired = errors.New("refund address is required for crypto orders")

// ErrRefundAddressInvalid is returned when a buyer-supplied refund address
// fails chain-specific format validation (hex / base58 / utxo).
var ErrRefundAddressInvalid = errors.New("refund address has invalid format")

// ValidateRefundAddress validates a buyer-declared refund address against the
// payment coin's chain family.
//
// Rules:
//   - Fiat payment (e.g. "fiat:stripe:USD"): always passes (no on-chain refund target).
//   - Empty crypto refund address: returns ErrRefundAddressRequired.
//   - EVM family (ETH/BSC/MATIC/...): must satisfy common.IsHexAddress
//     (40 hex digits with optional 0x prefix, mixed-case EIP-55 tolerated).
//   - Solana: must parse as a base58 32-byte public key.
//   - TRON: must start with 'T' and be 34 chars base58-ish.
//   - UTXO family (BTC/BCH/LTC/ZEC): permissive non-empty + length sanity
//     (between 25 and 90 chars, alphanumeric). Full chain-aware validation
//     is deferred — we accept that wrong-format strings will fail at refund
//     dispatch time rather than here.
//   - ExternalPayment: starts with '4' or '8', length 95+ (mainnet / testnet / integrated).
//
// Returns ErrRefundAddressRequired (for empty) or ErrRefundAddressInvalid
// (wrapped with chain-specific detail) on format failure.
func ValidateRefundAddress(coin iwallet.CoinType, addr string) error {
	// Fiat orders settle off-chain; refund routing handled by FiatProvider.
	if coin.IsFiatPayment() {
		return nil
	}

	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ErrRefundAddressRequired
	}

	info, err := coin.CoinInfo()
	if err != nil {
		// Unknown coin: fall through to length sanity below so we still
		// catch obviously-broken input. Don't hard-fail because Phase
		// EVM-ManagedEscrow registers new CAIP-2 coin codes that legacy callers
		// may not recognize yet.
		return validatePermissiveLength(addr)
	}

	switch {
	case info.Chain == iwallet.ChainSolana:
		if _, err := solana.PublicKeyFromBase58(addr); err != nil {
			return fmt.Errorf("%w: solana address: %v", ErrRefundAddressInvalid, err)
		}
		return nil

	case info.Chain == iwallet.ChainTRON:
		// TRON base58 addresses are 34 chars, starting with 'T'.
		// Full checksum validation needs the tron library; we keep
		// it lightweight here.
		if len(addr) != 34 || !strings.HasPrefix(addr, "T") {
			return fmt.Errorf("%w: tron address must be 34 chars starting with 'T'", ErrRefundAddressInvalid)
		}
		if !isBase58Alphabet(addr) {
			return fmt.Errorf("%w: tron address contains non-base58 characters", ErrRefundAddressInvalid)
		}
		return nil

	case info.Chain == iwallet.ChainExternalPayment:
		// EXTERNAL_PAYMENT addresses: 95 (standard), 106 (integrated/subaddress).
		if len(addr) < 95 || len(addr) > 110 {
			return fmt.Errorf("%w: external_payment address length %d outside 95-110 range", ErrRefundAddressInvalid, len(addr))
		}
		first := addr[0]
		if first != '4' && first != '8' {
			return fmt.Errorf("%w: external_payment address must start with '4' or '8'", ErrRefundAddressInvalid)
		}
		return nil

	case info.IsEthTypeChain():
		if !common.IsHexAddress(addr) {
			return fmt.Errorf("%w: EVM address must be 40 hex chars (with optional 0x prefix)", ErrRefundAddressInvalid)
		}
		// Reject the zero address — a refund to 0x000...0 would burn funds.
		if common.HexToAddress(addr) == (common.Address{}) {
			return fmt.Errorf("%w: EVM refund address must not be the zero address", ErrRefundAddressInvalid)
		}
		return nil

	default:
		// UTXO family + any other chain: permissive length check.
		// Chain-aware validation deferred to refund-dispatch time.
		return validatePermissiveLength(addr)
	}
}

// validatePermissiveLength enforces basic sanity for UTXO-family addresses
// (BTC/BCH/LTC/ZEC) without pulling in chain-specific dependencies. The
// trade-off is intentional: a wrong-network UTXO address will fail at
// refund-broadcast time, but we reject obviously malformed input here.
func validatePermissiveLength(addr string) error {
	const minUTXOLen = 25 // legacy P2PKH lower bound (1xxx...)
	const maxUTXOLen = 90 // bech32 upper bound (zec t1 + bc1q...)
	if len(addr) < minUTXOLen || len(addr) > maxUTXOLen {
		return fmt.Errorf("%w: address length %d outside %d-%d range", ErrRefundAddressInvalid, len(addr), minUTXOLen, maxUTXOLen)
	}
	for _, r := range addr {
		if r == ' ' || r == '\t' || r == '\n' {
			return fmt.Errorf("%w: address contains whitespace", ErrRefundAddressInvalid)
		}
	}
	return nil
}

// isBase58Alphabet returns true when every rune in s is part of the Bitcoin
// base58 alphabet (no '0', 'O', 'I', 'l'). Used for cheap TRON sanity checks.
func isBase58Alphabet(s string) bool {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for _, r := range s {
		if !strings.ContainsRune(alphabet, r) {
			return false
		}
	}
	return true
}
