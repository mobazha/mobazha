//go:build !private_distribution

package order

import (
	"context"
	"errors"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidatePurchaseRefundAddress_EmptyCryptoAllowed verifies that crypto
// orders may defer refund address collection until payment setup/confirmation.
func TestValidatePurchaseRefundAddress_EmptyCryptoAllowed(t *testing.T) {
	purchase := &models.Purchase{
		PricingCoin:   "crypto:eip155:1:native",
		RefundAddress: "",
	}
	require.NoError(t, validatePurchaseRefundAddress(purchase))
	assert.Empty(t, purchase.RefundAddress)

	purchase.RefundAddress = "   \t\n  "
	require.NoError(t, validatePurchaseRefundAddress(purchase))
	assert.Empty(t, purchase.RefundAddress)
}

// TestValidatePurchaseRefundAddress_EmptyPricingCoin verifies the caller-bug
// fallback: when PricingCoin is empty we skip validation rather than reject,
// because createOrder will surface a clearer error downstream.
func TestValidatePurchaseRefundAddress_EmptyPricingCoin(t *testing.T) {
	purchase := &models.Purchase{
		PricingCoin:   "",
		RefundAddress: "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
	}
	require.NoError(t, validatePurchaseRefundAddress(purchase))
}

// TestValidatePurchaseRefundAddress_ValidEVM happy path — well-formed
// RefundAddress is accepted and trimmed onto the Purchase.
func TestValidatePurchaseRefundAddress_ValidEVM(t *testing.T) {
	purchase := &models.Purchase{
		PricingCoin:   "crypto:eip155:1:native",
		RefundAddress: "  0x742d35Cc6634C0532925a3b844Bc454e4438f44e  ",
	}
	require.NoError(t, validatePurchaseRefundAddress(purchase))
	// Whitespace must be normalized away so the DB row matches what
	// downstream verifiers will read back.
	assert.Equal(t, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", purchase.RefundAddress)
}

// TestValidatePurchaseRefundAddress_InvalidEVM_BadRequest verifies invalid
// addresses surface as ErrBadRequest so HTTP handlers return 400.
func TestValidatePurchaseRefundAddress_InvalidEVM_BadRequest(t *testing.T) {
	purchase := &models.Purchase{
		PricingCoin:   "crypto:eip155:1:native",
		RefundAddress: "not-a-hex-address",
	}
	err := validatePurchaseRefundAddress(purchase)
	require.Error(t, err)
	assert.True(t, errors.Is(err, coreiface.ErrBadRequest), "expected ErrBadRequest, got %v", err)
	assert.True(t, errors.Is(err, models.ErrRefundAddressInvalid), "expected wrapped ErrRefundAddressInvalid")
}

// TestValidatePurchaseRefundAddress_ZeroAddressRejected guards against burn
// refunds — 0x000...0 must always be rejected even though it parses as
// valid hex.
func TestValidatePurchaseRefundAddress_ZeroAddressRejected(t *testing.T) {
	purchase := &models.Purchase{
		PricingCoin:   "crypto:eip155:1:native",
		RefundAddress: "0x0000000000000000000000000000000000000000",
	}
	err := validatePurchaseRefundAddress(purchase)
	require.Error(t, err)
	assert.True(t, errors.Is(err, coreiface.ErrBadRequest))
	assert.True(t, errors.Is(err, models.ErrRefundAddressInvalid))
}

// TestValidatePurchaseRefundAddress_FiatSkipped ensures fiat orders bypass
// validation entirely — refunds go through FiatProvider, not on-chain.
func TestValidatePurchaseRefundAddress_FiatSkipped(t *testing.T) {
	purchase := &models.Purchase{
		PricingCoin:   "fiat:stripe:USD",
		RefundAddress: "anything-goes-here",
	}
	require.NoError(t, validatePurchaseRefundAddress(purchase))
	assert.Empty(t, purchase.RefundAddress)

	purchase.RefundAddress = ""
	require.NoError(t, validatePurchaseRefundAddress(purchase))

	purchase = &models.Purchase{
		PricingCoin:   "USD",
		RefundAddress: "not-an-on-chain-address",
	}
	require.NoError(t, validatePurchaseRefundAddress(purchase))
	assert.Empty(t, purchase.RefundAddress)
}

// TestPersistOrderRefundAddress_HappyPath verifies the helper writes
// RefundAddress to the existing local Order row and that a subsequent read
// observes the value (round-trip via the same DB).
func TestPersistOrderRefundAddress_HappyPath(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	const orderID = "test-order-refund-1"
	const refundAddr = "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"

	// Seed an order so persistOrderRefundAddress has something to load.
	seedOrder(t, svc, orderID, "buyer", models.OrderState_AWAITING_PAYMENT)

	err := svc.db.Update(func(tx database.Tx) error {
		return persistOrderRefundAddress(tx, orderID, refundAddr)
	})
	require.NoError(t, err)

	// Read back through the same DB layer the AggregatingVerifier uses.
	var got models.Order
	err = svc.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&got).Error
	})
	require.NoError(t, err)
	assert.Equal(t, refundAddr, got.RefundAddress)
}

// TestPersistOrderRefundAddress_OrderNotFound makes sure the helper surfaces
// the missing-row error rather than silently no-op'ing (caller bug).
func TestPersistOrderRefundAddress_OrderNotFound(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})

	err := svc.db.Update(func(tx database.Tx) error {
		return persistOrderRefundAddress(tx, "non-existent-order", "0x1234")
	})
	require.Error(t, err)
}

// TestPersistOrderRefundAddress_TrimsWhitespace covers a guard that prevents
// trailing newlines / spaces (e.g. from a shell-pasted address) from leaking
// into Order.RefundAddress.
func TestPersistOrderRefundAddress_TrimsWhitespace(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	const orderID = "test-order-refund-2"
	const refundAddrRaw = "  0x742d35Cc6634C0532925a3b844Bc454e4438f44e\n"
	const refundAddrTrimmed = "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"

	seedOrder(t, svc, orderID, "buyer", models.OrderState_AWAITING_PAYMENT)

	err := svc.db.Update(func(tx database.Tx) error {
		return persistOrderRefundAddress(tx, orderID, refundAddrRaw)
	})
	require.NoError(t, err)

	var got models.Order
	err = svc.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&got).Error
	})
	require.NoError(t, err)
	assert.Equal(t, refundAddrTrimmed, got.RefundAddress)
}

// TestSetOrderRefundAddressForPayment_UsesActualPaymentCoin covers USD-priced
// orders that are funded with crypto: validation must use the selected payment
// coin, not the earlier listing pricing currency.
func TestSetOrderRefundAddressForPayment_UsesActualPaymentCoin(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	const orderID = "test-order-refund-actual-coin"
	const refundAddr = "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"

	seedOrder(t, svc, orderID, "buyer", models.OrderState_AWAITING_PAYMENT)

	err := svc.SetOrderRefundAddressForPayment(context.Background(), orderID, iwallet.CoinType("crypto:eip155:1:native"), "  "+refundAddr+"  ")
	require.NoError(t, err)

	var got models.Order
	err = svc.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&got).Error
	})
	require.NoError(t, err)
	assert.Equal(t, refundAddr, got.RefundAddress)
}

func TestSetOrderRefundAddressForPayment_EmptyCryptoRejected(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})

	err := svc.SetOrderRefundAddressForPayment(context.Background(), "any-order", iwallet.CoinType("crypto:eip155:1:native"), "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, coreiface.ErrBadRequest))
	assert.True(t, errors.Is(err, models.ErrRefundAddressRequired))
}
