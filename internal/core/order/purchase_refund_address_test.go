//go:build !private_distribution

package order

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	orderprocessor "github.com/mobazha/mobazha3.0/internal/orders"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"gorm.io/gorm"
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

	var shared models.SharedPaymentIntent
	err = svc.db.View(func(tx database.Tx) error {
		return tx.Read().Session(&gorm.Session{NewDB: true}).Where("order_id = ?", orderID).First(&shared).Error
	})
	require.NoError(t, err)
	assert.Equal(t, refundAddr, shared.RefundAddress)
}

func TestSetOrderRefundAddressForPayment_ReplaysParkedMessages(t *testing.T) {
	privKey, _, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	marshaledKey, err := libp2pcrypto.MarshalPrivateKey(privKey)
	require.NoError(t, err)
	signer, err := contracts.NewKeyPairSignerFromMarshaledKey(marshaledKey)
	require.NoError(t, err)

	svc := newTestOrderAppService(t, OrderAppServiceConfig{Signer: signer})
	svc.orderProcessor = orderprocessor.NewOrderProcessor(&orderprocessor.Config{
		Db:       svc.db,
		Signer:   signer,
		EventBus: svc.eventBus,
	})

	const orderID = "test-order-refund-replay-parked"
	const refundAddr = "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"

	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{
			{
				Listing: &pb.Listing{
					VendorID: &pb.ID{PeerID: signer.PeerID().String()},
					Item: &pb.Listing_Item{
						Images: []*pb.Image{{Tiny: "tiny", Small: "small"}},
					},
				},
			},
		},
	}
	declineMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_DECLINE,
		Message: mustAny(t, &pb.OrderDecline{
			Type:   pb.OrderDecline_VALIDATION_ERROR,
			Reason: "test replay",
		}),
	}
	require.NoError(t, utils.SignOrderMessage(declineMsg, signer))

	err = svc.db.Update(func(tx database.Tx) error {
		order := &models.Order{
			ID:     models.OrderID(orderID),
			MyRole: string(models.RoleBuyer),
		}
		order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
		if err := order.PutMessage(&npb.OrderMessage{
			Signature:   []byte("order-open-sig"),
			Message:     mustAny(t, orderOpen),
			MessageType: npb.OrderMessage_ORDER_OPEN,
		}); err != nil {
			return err
		}
		if err := order.ParkMessage(declineMsg); err != nil {
			return err
		}
		return tx.Save(order)
	})
	require.NoError(t, err)

	err = svc.SetOrderRefundAddressForPayment(context.Background(), orderID, iwallet.CoinType("crypto:eip155:1:native"), refundAddr)
	require.NoError(t, err)

	var got models.Order
	err = svc.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&got).Error
	})
	require.NoError(t, err)
	assert.Equal(t, refundAddr, got.RefundAddress)
	assert.NotNil(t, got.SerializedOrderDecline)

	parked, err := got.GetParkedMessages()
	require.NoError(t, err)
	assert.Empty(t, parked.Messages)
}

func TestSetOrderRefundAddressForPayment_EmptyCryptoRejected(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})

	err := svc.SetOrderRefundAddressForPayment(context.Background(), "any-order", iwallet.CoinType("crypto:eip155:1:native"), "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, coreiface.ErrBadRequest))
	assert.True(t, errors.Is(err, models.ErrRefundAddressRequired))
}

func mustAny(t *testing.T, msg proto.Message) *anypb.Any {
	t.Helper()
	a := &anypb.Any{}
	require.NoError(t, a.MarshalFrom(msg))
	return a
}
