//go:build !private_distribution

package core

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	coreorder "github.com/mobazha/mobazha3.0/internal/core/order"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func orderWithChaincode(t *testing.T, id, chaincode string) *models.Order {
	t.Helper()
	oo := &pb.OrderOpen{Chaincode: chaincode}
	data, err := protojson.Marshal(oo)
	require.NoError(t, err)

	order := &models.Order{ID: models.OrderID(id)}
	order.SerializedOrderOpen = data
	return order
}

// ── BuildPaymentSentProto ──────────────────────────────────────────────

func TestBuildPaymentSentProto_FiatPayment(t *testing.T) {
	order := orderWithChaincode(t, "order-fiat-1", "abcdef1234")

	now := time.Now()
	pd := &models.PaymentData{
		OrderID:       "order-fiat-1",
		TransactionID: "pi_test_123",
		Coin:          "fiat:stripe:usd",
		Amount:        999,
		Method:        pb.PaymentSent_FIAT,
		ProviderID:    "stripe",
		Timestamp:     now,
	}
	pd.PaymentMethod.Type = "card"
	pd.PaymentMethod.Brand = "visa"
	pd.PaymentMethod.Last4 = "4242"

	ps, err := coreorder.BuildPaymentSentProto(order, pd)
	require.NoError(t, err)
	assert.Equal(t, "pi_test_123", ps.TransactionID)
	assert.Equal(t, "fiat:stripe:USD", ps.Coin)
	require.NotNil(t, ps.GetSettlementSpec())
	assert.Equal(t, pb.PaymentSent_FIAT, ps.GetSettlementSpec().GetMethod())
	assert.Equal(t, "999", ps.Amount)
	assert.Equal(t, "abcdef1234", ps.Chaincode)
	assert.Equal(t, "card", ps.PaymentMethod.Type)
	assert.Equal(t, "visa", ps.PaymentMethod.Brand)
	assert.Equal(t, "4242", ps.PaymentMethod.Last4)
	require.NotNil(t, ps.SettlementSpec)
	assert.Equal(t, pb.PaymentSent_FIAT, ps.SettlementSpec.Method)
	assert.Equal(t, "provider", ps.SettlementSpec.PayMode)
	assert.Equal(t, "fiat_provider", ps.SettlementSpec.EscrowType)
}

func TestBuildPaymentSentProto_CryptoPayment(t *testing.T) {
	order := orderWithChaincode(t, "order-evm-1", "deadbeef")

	pd := &models.PaymentData{
		OrderID:         "order-evm-1",
		TransactionID:   "0xabc123",
		Coin:            "ETH",
		Amount:          1000000,
		Method:          pb.PaymentSent_DIRECT,
		ContractAddress: "0xContractAddr",
		PayerAddress:    "0xPayerAddr",
		ToAddress:       "0xToAddr",
		Timestamp:       time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC),
	}

	ps, err := coreorder.BuildPaymentSentProto(order, pd)
	require.NoError(t, err)
	assert.Equal(t, "0xabc123", ps.TransactionID)
	assert.Equal(t, "crypto:eip155:1:native", ps.Coin)
	require.NotNil(t, ps.GetSettlementSpec())
	assert.Equal(t, pb.PaymentSent_DIRECT, ps.GetSettlementSpec().GetMethod())
	assert.Equal(t, "0xContractAddr", ps.ContractAddress)
	assert.Equal(t, "0xPayerAddr", ps.PayerAddress)
	assert.Equal(t, "0xToAddr", ps.ToAddress)
	assert.Equal(t, "deadbeef", ps.Chaincode)
	assert.Nil(t, ps.PaymentMethod, "empty crypto payment metadata should not materialize an empty paymentMethod object")
	require.NotNil(t, ps.SettlementSpec)
	assert.Equal(t, pb.PaymentSent_DIRECT, ps.SettlementSpec.Method)
	assert.Equal(t, "address_monitored", ps.SettlementSpec.PayMode)
	assert.Equal(t, "none", ps.SettlementSpec.EscrowType)
}

func TestBuildPaymentSentProto_ByteIdentical(t *testing.T) {
	order := orderWithChaincode(t, "order-dup-1", "cc-hex")

	pd := &models.PaymentData{
		OrderID:       "order-dup-1",
		TransactionID: "tx_abc",
		Coin:          "fiat:stripe:usd",
		Amount:        500,
		Method:        pb.PaymentSent_FIAT,
		Timestamp:     time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC),
	}

	ps1, err := coreorder.BuildPaymentSentProto(order, pd)
	require.NoError(t, err)
	ps2, err := coreorder.BuildPaymentSentProto(order, pd)
	require.NoError(t, err)

	assert.Equal(t, ps1.TransactionID, ps2.TransactionID)
	assert.Equal(t, ps1.Coin, ps2.Coin)
	assert.Equal(t, ps1.Amount, ps2.Amount)
	assert.Equal(t, ps1.Chaincode, ps2.Chaincode)
	assert.Equal(t, ps1.Timestamp.AsTime(), ps2.Timestamp.AsTime())
}

func TestBuildPaymentSentProto_NoOrderOpen_Error(t *testing.T) {
	order := &models.Order{ID: "order-no-open"}

	pd := &models.PaymentData{
		OrderID:       "order-no-open",
		TransactionID: "tx_fail",
		Coin:          "fiat:stripe:usd",
		Amount:        100,
	}

	_, err := coreorder.BuildPaymentSentProto(order, pd)
	assert.Error(t, err)
}

// ── RelayPaymentToCounterparty ─────────────────────────────────────────

func TestRelayPaymentToCounterparty_NoMessenger_NoPanic(t *testing.T) {
	svc := coreorder.NewTestOrderAppService(t, coreorder.OrderAppServiceConfig{})

	buyerPeerID, _ := peer.Decode("QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG")
	pd := &models.PaymentData{
		OrderID:       "order-no-messenger",
		TransactionID: "pi_nolookup",
		Coin:          "fiat:stripe:usd",
		Amount:        500,
		Method:        pb.PaymentSent_FIAT,
	}

	assert.NotPanics(t, func() {
		svc.RelayPaymentToCounterparty(context.Background(), "order-no-messenger", buyerPeerID, pd)
	})
}

// ── RelayPaymentToBuyer ────────────────────────────────────────────────

func TestRelayPaymentToBuyer_OrderNotFound_NoPanic(t *testing.T) {
	svc := coreorder.NewTestOrderAppService(t, coreorder.OrderAppServiceConfig{})

	pd := &models.PaymentData{
		OrderID:       "nonexistent",
		TransactionID: "pi_notfound",
		Coin:          "fiat:stripe:usd",
		Amount:        100,
	}

	assert.NotPanics(t, func() {
		svc.RelayPaymentToBuyer(context.Background(), "nonexistent", pd)
	})
}

// ── buildFiatPaymentData ───────────────────────────────────────────────

func TestBuildFiatPaymentData_StripeWithCurrency(t *testing.T) {
	event := &contracts.WebhookEvent{
		OrderID:    "order-stripe-1",
		PaymentID:  "pi_1234",
		ProviderID: "stripe",
		Currency:   "USD",
		Amount:     2500,
	}

	pd, err := buildFiatPaymentData(event)
	require.NoError(t, err)

	assert.Equal(t, "order-stripe-1", pd.OrderID)
	assert.Equal(t, "pi_1234", pd.TransactionID)
	assert.Equal(t, "fiat:stripe:USD", string(pd.Coin))
	assert.Equal(t, uint64(2500), pd.Amount)
	assert.Equal(t, pb.PaymentSent_FIAT, pd.Method)
	assert.Equal(t, "stripe", pd.ProviderID)
}

func TestBuildFiatPaymentData_PayPalWithCurrency(t *testing.T) {
	event := &contracts.WebhookEvent{
		OrderID:    "order-paypal-1",
		PaymentID:  "PAYID-123",
		ProviderID: "paypal",
		Currency:   "EUR",
		Amount:     5000,
	}

	pd, err := buildFiatPaymentData(event)
	require.NoError(t, err)

	assert.Equal(t, "fiat:paypal:EUR", string(pd.Coin))
	assert.Equal(t, "paypal", pd.ProviderID)
}

func TestBuildFiatPaymentData_RejectsCoinOnlyFallback(t *testing.T) {
	event := &contracts.WebhookEvent{
		OrderID:   "order-coin-set",
		PaymentID: "tx_prebuilt",
		Coin:      "fiat:stripe:GBP",
		Amount:    800,
	}

	_, err := buildFiatPaymentData(event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider ID is empty")
}

func TestBuildFiatPaymentData_NoCoinNoCurrency(t *testing.T) {
	event := &contracts.WebhookEvent{
		OrderID:   "order-empty-coin",
		PaymentID: "tx_empty",
		Amount:    100,
	}

	_, err := buildFiatPaymentData(event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider ID is empty")
}

// ── FetchOrder ─────────────────────────────────────────────────────────

func TestFetchOrder_Found(t *testing.T) {
	svc := coreorder.NewTestOrderAppService(t, coreorder.OrderAppServiceConfig{})
	coreorder.SeedOrder(t, svc, "order-fetch-1", "vendor", models.OrderState_PENDING)

	order, err := svc.FetchOrder("order-fetch-1")
	require.NoError(t, err)
	assert.Equal(t, models.OrderID("order-fetch-1"), order.ID)
}

func TestFetchOrder_NotFound(t *testing.T) {
	svc := coreorder.NewTestOrderAppService(t, coreorder.OrderAppServiceConfig{})

	_, err := svc.FetchOrder("nonexistent")
	assert.Error(t, err)
}
