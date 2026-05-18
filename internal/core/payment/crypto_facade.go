//go:build !private_distribution

package payment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	porderpb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	paypb "github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// CryptoPaymentFacade wraps WalletService.GeneratePaymentInstructions to
// populate crypto funding targets (ManagedEscrow, UTXO, monitored flows) behind
// PaymentSessionService.CreateSession.
type CryptoPaymentFacade struct {
	projector *PaymentSessionProjector
	orderSvc  contracts.OrderService
	walletSvc contracts.WalletService
	exchange  contracts.ExchangeRateService
}

// NewCryptoPaymentFacade constructs CryptoPaymentFacade.
func NewCryptoPaymentFacade(
	db database.Database,
	orderSvc contracts.OrderService,
	walletSvc contracts.WalletService,
	exchange contracts.ExchangeRateService,
) *CryptoPaymentFacade {
	return &CryptoPaymentFacade{
		projector: NewPaymentSessionProjector(db),
		orderSvc:  orderSvc,
		walletSvc: walletSvc,
		exchange:  exchange,
	}
}

// CreateSession provisions crypto payment instructions on the order and
// returns the unified projection.
func (c *CryptoPaymentFacade) CreateSession(
	ctx context.Context,
	req contracts.CreatePaymentSessionRequest,
) (*paypb.PaymentSession, error) {
	coin := iwallet.CoinType(req.PaymentCoin)
	input, err := c.projector.fetchProjectInput(req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("crypto facade: load order %s: %w", req.OrderID, err)
	}
	order := input.order
	orderOpen := input.orderOpen
	if orderOpen == nil {
		return nil, fmt.Errorf("crypto facade: order open unavailable for order %s", req.OrderID)
	}

	if len(orderOpen.Listings) > 0 &&
		orderOpen.Listings[0].Listing != nil &&
		orderOpen.Listings[0].Listing.Metadata != nil &&
		orderOpen.Listings[0].Listing.Metadata.ContractType == porderpb.Listing_Metadata_RWA_TOKEN {
		return nil, fmt.Errorf("%w", ErrRWAPaymentUseLegacyInstructions)
	}

	// Validate refund address only when provided. If the client-signed path
	// sends payerAddress but omits refundAddress, use the payer as the default
	// refund target. Address-monitored flows can still omit both and let the
	// verifier infer only when the observed sender is unambiguous.
	refundAddr, err := normalizeCryptoRefundAddress(coin, req.RefundAddress, req.PayerAddress)
	if err != nil {
		return nil, err
	}

	initData, err := buildInitializeEscrowDataFromOrder(order, orderOpen, coin,
		refundAddr, req.PayerAddress, req.Moderator, c.exchange)
	if err != nil {
		return nil, fmt.Errorf("crypto facade: build escrow params: %w", err)
	}

	result, err := c.walletSvc.GeneratePaymentInstructions(ctx, initData)
	if err != nil {
		if result != nil && result.PaymentData != nil && errors.Is(err, coreiface.ErrCoinSwitchRequiresConfirmation) {
			return nil, fmt.Errorf("%w", coreiface.ErrCoinSwitchRequiresConfirmation)
		}
		return nil, fmt.Errorf("crypto facade: generate instructions: %w", err)
	}

	// Persist refund address when the buyer explicitly provided one or when
	// client-signed setup supplied a payerAddress we can safely default to.
	if refundAddr != "" {
		if err := c.orderSvc.SetOrderRefundAddressForPayment(ctx, req.OrderID, coin, refundAddr); err != nil {
			if errors.Is(err, coreiface.ErrBadRequest) || errors.Is(err, models.ErrRefundAddressRequired) || errors.Is(err, models.ErrRefundAddressInvalid) {
				return nil, err
			}
			return nil, fmt.Errorf("crypto facade: save refund address: %w", err)
		}
	}

	input2, err := c.projector.fetchProjectInput(req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("crypto facade: re-load order %s: %w", req.OrderID, err)
	}
	return c.projector.Project(input2)
}

func normalizeCryptoRefundAddress(coin iwallet.CoinType, refundAddress, payerAddress string) (string, error) {
	refundAddr := strings.TrimSpace(refundAddress)
	if refundAddr == "" {
		refundAddr = strings.TrimSpace(payerAddress)
	}
	if refundAddr == "" {
		return "", nil
	}
	if err := models.ValidateRefundAddress(coin, refundAddr); err != nil {
		return "", fmt.Errorf("%w: %w", coreiface.ErrBadRequest, err)
	}
	return refundAddr, nil
}

func buildInitializeEscrowDataFromOrder(
	order *models.Order,
	orderOpen *porderpb.OrderOpen,
	coin iwallet.CoinType,
	refundAddress, payerAddress, moderator string,
	ex contracts.ExchangeRateService,
) (models.InitializeEscrowData, error) {
	if order == nil {
		return models.InitializeEscrowData{}, errors.New("order is nil")
	}
	orderAmount := iwallet.NewAmount(orderOpen.Amount)
	pricingCoin := strings.ToUpper(orderOpen.PricingCoin)
	paymentCoinCode, err := coin.PricingCurrencyCode()
	if err != nil {
		return models.InitializeEscrowData{}, fmt.Errorf("coin type pricing code: %w", err)
	}

	var amt uint64
	if pricingCoin != "" && pricingCoin != paymentCoinCode {
		// Cross-currency order: pricing coin differs from payment coin.
		// ExchangeRateService is required; its adapter returns an informative error
		// (not a panic) when the underlying provider is nil, so errors propagate
		// cleanly to the caller as ErrExchangeRateUnavailable-wrapped messages.
		if ex == nil {
			return models.InitializeEscrowData{}, fmt.Errorf(
				"%w: order priced in %s but payment coin is %s",
				ErrExchangeRateUnavailable, pricingCoin, paymentCoinCode)
		}
		pricingCurrency, err := models.CurrencyDefinitions.Lookup(pricingCoin)
		if err != nil {
			return models.InitializeEscrowData{}, fmt.Errorf("unknown pricing currency %q: %w", pricingCoin, err)
		}
		paymentCurrency, err := models.CurrencyDefinitions.Lookup(paymentCoinCode)
		if err != nil {
			return models.InitializeEscrowData{}, fmt.Errorf("unknown payment currency %q: %w", paymentCoinCode, err)
		}
		converted, err := wallet.ConvertCurrencyAmount(
			&models.CurrencyValue{Amount: orderAmount, Currency: pricingCurrency},
			paymentCurrency,
			ex,
		)
		if err != nil {
			return models.InitializeEscrowData{}, fmt.Errorf("convert payment amount: %w", err)
		}
		amt = converted.Uint64()
	} else {
		amt = orderAmount.Uint64()
	}

	return models.InitializeEscrowData{
		OrderID:       order.ID.String(),
		PayerAddress:  payerAddress,
		RefundAddress: refundAddress,
		Moderator:     moderator,
		CoinType:      coin,
		Amount:        amt,
	}, nil
}
