package payment

import (
	"fmt"
	"math/big"
	"strings"

	wallet "github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/models"
	settlementpayment "github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type managedEscrowFeeQuote struct {
	ReleaseFeeAmount string
	CancelFeeAmount  string
	PlatformAddr     string
}

// quoteManagedEscrowFees locks a provider-declared service fee at payment
// intent creation. Settlement consumes the persisted amounts and never
// re-prices from volatile chain state.
func (s *PaymentAppService) quoteManagedEscrowFees(
	coinType iwallet.CoinType,
	paymentAmount uint64,
	policy settlementpayment.ManagedEscrowFeePolicy,
) (managedEscrowFeeQuote, error) {
	coinInfo, err := settlementpayment.SettlementCoinInfoForCoin(coinType)
	if err != nil {
		return managedEscrowFeeQuote{}, fmt.Errorf("managed escrow fee quote: decode coin %q: %w", coinType, err)
	}

	feeCents := policy.ReleaseFeeUSDCents
	if s.netConfig != nil {
		// This persisted config key retains its historical name for backward
		// compatibility; the pricing behavior is provider-neutral here.
		if override, ok := s.netConfig.GetManagedEscrowReleaseFeeUSDCents(coinInfo.Chain); ok {
			feeCents = override
		}
	}
	if feeCents == 0 {
		return managedEscrowFeeQuote{ReleaseFeeAmount: "0", CancelFeeAmount: "0"}, nil
	}

	platformAddr := s.managedEscrowPlatformFeeAddress(coinInfo.Chain)
	if strings.TrimSpace(platformAddr) == "" {
		return managedEscrowFeeQuote{}, fmt.Errorf("managed escrow fee quote: platform fee collector missing for chain %s", coinInfo.Chain)
	}
	releaseFee, err := s.convertUSDMinorToPaymentAmount(feeCents, coinType)
	if err != nil {
		return managedEscrowFeeQuote{}, err
	}
	if releaseFee.Sign() <= 0 {
		return managedEscrowFeeQuote{}, fmt.Errorf("managed escrow fee quote: computed zero fee for chain %s coin %s", coinInfo.Chain, coinType)
	}
	paymentAmountInt := new(big.Int).SetUint64(paymentAmount)
	if releaseFee.Cmp(paymentAmountInt) >= 0 {
		return managedEscrowFeeQuote{}, fmt.Errorf("managed escrow fee quote: fee %s >= payment amount %s for chain %s coin %s", releaseFee.String(), paymentAmountInt.String(), coinInfo.Chain, coinType)
	}
	cancelFee := "0"
	if policy.ChargeCancellation {
		cancelFee = releaseFee.String()
	}
	return managedEscrowFeeQuote{
		ReleaseFeeAmount: releaseFee.String(),
		CancelFeeAmount:  cancelFee,
		PlatformAddr:     platformAddr,
	}, nil
}

func (s *PaymentAppService) managedEscrowPlatformFeeAddress(chain iwallet.ChainType) string {
	if s.netConfig == nil {
		return ""
	}
	return strings.TrimSpace(s.netConfig.GetPlatformAddr(chain))
}

func (s *PaymentAppService) convertUSDMinorToPaymentAmount(usdMinor uint64, coinType iwallet.CoinType) (*big.Int, error) {
	if s.exchangeRates == nil {
		return nil, fmt.Errorf("managed escrow fee quote: exchange rates unavailable for coin %s", coinType)
	}
	usd, err := models.CurrencyDefinitions.Lookup("USD")
	if err != nil {
		return nil, fmt.Errorf("managed escrow fee quote: USD currency definition: %w", err)
	}
	paymentCurrency, err := models.CurrencyDefinitions.Lookup(string(coinType))
	if err != nil {
		return nil, fmt.Errorf("managed escrow fee quote: payment currency %s: %w", coinType, err)
	}
	converted, err := wallet.ConvertCurrencyAmount(
		&models.CurrencyValue{Amount: iwallet.NewAmount(usdMinor), Currency: usd},
		paymentCurrency,
		s.exchangeRates,
	)
	if err != nil {
		return nil, fmt.Errorf("managed escrow fee quote: convert USD fee to %s: %w", coinType, err)
	}
	out := big.Int(converted)
	return &out, nil
}
