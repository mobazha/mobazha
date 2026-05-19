package guest

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// GuestPaymentCapability describes which parts of the guest checkout closure
// path are implemented for a coin on this node. BuyerVisible must be true
// before a coin is offered to buyers or accepted on POST /v1/guest/orders.
type GuestPaymentCapability struct {
	Coin               string
	Chain              iwallet.ChainType
	CanAllocateAddress bool
	CanDetectPayment   bool
	CanConfirmPayment  bool
	CanSettleFunds     bool
	BuyerVisible       bool
	Reason             string
	Err                error
}

// guestEVMSettlementEnabled gates buyer-visible EVM guest checkout until an
// EVM sweep / settlement path is implemented (see GUEST_CHECKOUT_CRYPTO_CLOSURE_PLAN).
const guestEVMSettlementEnabled = false

// isSweepableUTXOChain reports whether guest auto-sweep can sign for the chain's
// default address type (P2WPKH / BIP-143). Mirrors internal/core.isSweepableP2WPKHChain.
func isSweepableUTXOChain(chain iwallet.ChainType) bool {
	return chain.GetDefaultDerivationType() == iwallet.DerivationNativeSegwit
}

func (s *GuestOrderAppService) evaluateGuestPaymentCapability(coinType iwallet.CoinType, coinInfo iwallet.CoinInfo) GuestPaymentCapability {
	cap := GuestPaymentCapability{
		Coin:  string(coinType),
		Chain: coinInfo.Chain,
	}

	switch {
	case coinInfo.Chain.IsUTXOChain():
		cap.CanAllocateAddress = true
		if err := s.evaluateUTXOClosureReadiness(coinType, coinInfo); err != nil {
			cap.Err = err
			cap.Reason = err.Error()
			return cap
		}
		cap.CanDetectPayment = true
		cap.CanConfirmPayment = true
		cap.CanSettleFunds = true
		cap.BuyerVisible = true
		return cap

	case coinInfo.IsEthTypeChain():
		if err := s.validateCoinAvailability(coinType, coinInfo); err != nil {
			cap.Err = err
			cap.Reason = err.Error()
			return cap
		}
		cap.CanAllocateAddress = true
		cap.CanDetectPayment = true
		cap.CanConfirmPayment = true
		if guestEVMSettlementEnabled {
			cap.CanSettleFunds = true
			cap.BuyerVisible = true
			return cap
		}
		cap.Reason = "EVM guest checkout settlement is not implemented"
		return cap

	case coinInfo.Chain == iwallet.ChainTRON:
		cap.Err = fmt.Errorf("%w: TRON guest checkout is not implemented", contracts.ErrCoinUnsupported)
		cap.Reason = cap.Err.Error()
		return cap

	case coinInfo.Chain == iwallet.ChainSolana:
		cap.Err = fmt.Errorf("%w: Solana guest checkout is not verified end-to-end", contracts.ErrCoinUnsupported)
		cap.Reason = cap.Err.Error()
		return cap

	case coinInfo.Chain == iwallet.ChainExternalPayment:
		if err := s.validateCoinAvailability(coinType, coinInfo); err != nil {
			cap.Err = err
			cap.Reason = err.Error()
			return cap
		}
		// EXTERNAL_PAYMENT pays directly to seller subaddresses; no UTXO/EVM sweep task.
		cap.CanAllocateAddress = true
		cap.CanDetectPayment = true
		cap.CanConfirmPayment = true
		cap.CanSettleFunds = true
		cap.BuyerVisible = true
		return cap

	default:
		cap.Err = fmt.Errorf("%w: coin %q has no guest checkout handler", contracts.ErrCoinUnsupported, coinType)
		cap.Reason = cap.Err.Error()
		return cap
	}
}

func (s *GuestOrderAppService) validateBuyerVisibleCoin(coinType iwallet.CoinType, coinInfo iwallet.CoinInfo, displayCoin string) error {
	cap := s.evaluateGuestPaymentCapability(coinType, coinInfo)
	if cap.BuyerVisible {
		return nil
	}
	if cap.Err != nil {
		if errors.Is(cap.Err, contracts.ErrCoinUnavailable) ||
			errors.Is(cap.Err, contracts.ErrCoinUnsupported) ||
			errors.Is(cap.Err, contracts.ErrInvalidGuestRequest) {
			return cap.Err
		}
	}
	if coinInfo.IsEthTypeChain() {
		label := guestSettlementCoinLabel(displayCoin, coinInfo)
		return fmt.Errorf("%w: guest checkout for %s is not settlement-ready",
			contracts.ErrInvalidGuestRequest, label)
	}
	if cap.Reason != "" {
		return fmt.Errorf("%w: %s", contracts.ErrInvalidGuestRequest, cap.Reason)
	}
	return fmt.Errorf("%w: guest checkout is not available for %q",
		contracts.ErrInvalidGuestRequest, displayCoin)
}

func guestSettlementCoinLabel(displayCoin string, coinInfo iwallet.CoinInfo) string {
	trimmed := strings.TrimSpace(displayCoin)
	if trimmed != "" && !strings.HasPrefix(strings.ToLower(trimmed), "crypto:") {
		return strings.ToUpper(trimmed)
	}
	switch coinInfo.Chain {
	case iwallet.ChainEthereum:
		return "ETH"
	default:
		return string(coinInfo.Chain)
	}
}
