package order

import (
	"context"
	"errors"
	"testing"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestResolveFiatProvider_FallsBackToPaymentSentCoin(t *testing.T) {
	orderOpenJSON, err := protojson.Marshal(&pb.OrderOpen{PricingCoin: "USD"})
	require.NoError(t, err)

	order := &models.Order{SerializedOrderOpen: orderOpenJSON}
	paymentSent := &pb.PaymentSent{Coin: "fiat:paypal:USD"}

	assert.Equal(t, "paypal", resolveFiatProvider(order, paymentSent))
}

func TestResolveFiatProvider_FallsBackToFiatMetadata(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.MergeFiatMetadata(map[string]string{
		"fiat_provider":   "paypal",
		"fiat_session_id": "sess_123",
	}))

	assert.Equal(t, "paypal", resolveFiatProvider(order, nil))
}

func TestResolveFiatProvider_UnknownWhenNoCanonicalSource(t *testing.T) {
	order := &models.Order{}
	paymentSent := &pb.PaymentSent{Coin: "fiat:USD"}
	assert.Equal(t, "", resolveFiatProvider(order, paymentSent))
}

type mockFiatOpsForOrderTest struct {
	lastProviderID string
	lastParams     contracts.RefundParams
	refundResult   *contracts.RefundResult
	refundErr      error
	disburseResult *contracts.DisbursePaymentResult
	disburseErr    error
	disburseParams contracts.DisbursePaymentParams
	capabilities   contracts.FiatProviderCapabilities
}

func (m *mockFiatOpsForOrderTest) ProviderCapabilities(context.Context, string) (contracts.FiatProviderCapabilities, error) {
	return m.capabilities, nil
}

func (m *mockFiatOpsForOrderTest) DisbursePayment(_ context.Context, providerID string, params contracts.DisbursePaymentParams) (*contracts.DisbursePaymentResult, error) {
	m.lastProviderID = providerID
	m.disburseParams = params
	return m.disburseResult, m.disburseErr
}

func (m *mockFiatOpsForOrderTest) RefundPayment(_ context.Context, providerID string, params contracts.RefundParams) (*contracts.RefundResult, error) {
	m.lastProviderID = providerID
	m.lastParams = params
	return m.refundResult, m.refundErr
}

func (*mockFiatOpsForOrderTest) CancelPayment(context.Context, string, string) error {
	return nil
}

func (*mockFiatOpsForOrderTest) GetPaymentStatus(context.Context, string, string) (string, error) {
	return "", nil
}

func TestRefundFiatPayment_AlreadyRefundedIsIdempotent(t *testing.T) {
	orderOpenJSON, err := protojson.Marshal(&pb.OrderOpen{PricingCoin: "USD"})
	require.NoError(t, err)

	order := &models.Order{
		ID:                  "order_already_refunded",
		SerializedOrderOpen: orderOpenJSON,
	}
	paymentSent := &pb.PaymentSent{
		TransactionID: "CAPTURE-123",
		Coin:          "fiat:paypal:USD",
		Amount:        "2599",
	}

	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	ops := &mockFiatOpsForOrderTest{refundErr: contracts.ErrAlreadyRefunded}
	svc.SetFiatOps(ops)

	result, err := svc.refundFiatPayment(context.Background(), order, paymentSent, "requested_by_customer")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "paypal", ops.lastProviderID)
	assert.Equal(t, "CAPTURE-123", ops.lastParams.PaymentID)
	assert.Equal(t, "CAPTURE-123", result.RefundID)
	assert.Equal(t, "succeeded", result.Status)
	assert.Equal(t, int64(2599), result.Amount)
	assert.Equal(t, "USD", result.Currency)
}

func TestRefundFiatPayment_ProviderErrorIsReturned(t *testing.T) {
	orderOpenJSON, err := protojson.Marshal(&pb.OrderOpen{PricingCoin: "USD"})
	require.NoError(t, err)

	order := &models.Order{
		ID:                  "order_refund_error",
		SerializedOrderOpen: orderOpenJSON,
	}
	paymentSent := &pb.PaymentSent{
		TransactionID: "CAPTURE-ERR",
		Coin:          "fiat:paypal:USD",
		Amount:        "1000",
	}

	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	ops := &mockFiatOpsForOrderTest{refundErr: errors.New("provider down")}
	svc.SetFiatOps(ops)

	_, err = svc.refundFiatPayment(context.Background(), order, paymentSent, "requested_by_customer")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fiat refund for order order_refund_error")
}

func TestRefundFiatPayment_RequiresResolvedProvider(t *testing.T) {
	orderOpenJSON, err := protojson.Marshal(&pb.OrderOpen{PricingCoin: "USD"})
	require.NoError(t, err)

	order := &models.Order{
		ID:                  "order_refund_missing_provider",
		SerializedOrderOpen: orderOpenJSON,
	}
	paymentSent := &pb.PaymentSent{
		TransactionID: "CAPTURE-NO-PROVIDER",
		Coin:          "fiat:USD",
		Amount:        "1000",
	}

	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	ops := &mockFiatOpsForOrderTest{}
	svc.SetFiatOps(ops)

	_, err = svc.refundFiatPayment(context.Background(), order, paymentSent, "requested_by_customer")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fiat provider not resolved from payment")
}

func TestDisburseFiatPayment_BindsOrderAndCapture(t *testing.T) {
	order := &models.Order{ID: "order-disburse"}
	paymentSent := &pb.PaymentSent{TransactionID: "CAPTURE-123", Coin: "fiat:paypal:USD"}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	ops := &mockFiatOpsForOrderTest{disburseResult: &contracts.DisbursePaymentResult{DisbursementID: "PAYOUT-123", Status: "success"}}
	svc.SetFiatOps(ops)

	result, err := svc.disburseFiatPayment(context.Background(), order, paymentSent, "complete")
	require.NoError(t, err)
	assert.Equal(t, "PAYOUT-123", result.DisbursementID)
	assert.Equal(t, "paypal", ops.lastProviderID)
	assert.Equal(t, "CAPTURE-123", ops.disburseParams.PaymentID)
	assert.Equal(t, "order-disburse", ops.disburseParams.OrderID)
	assert.Equal(t, "order-disburse:order-disburse:complete", ops.disburseParams.IdempotencyKey)
}

func TestDisburseFiatPayment_RejectsTerminalProviderStatus(t *testing.T) {
	order := &models.Order{ID: "order-disburse-failed"}
	paymentSent := &pb.PaymentSent{TransactionID: "CAPTURE-FAILED", Coin: "fiat:paypal:USD"}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	svc.SetFiatOps(&mockFiatOpsForOrderTest{disburseResult: &contracts.DisbursePaymentResult{Status: "FAILED"}})

	_, err := svc.disburseFiatPayment(context.Background(), order, paymentSent, "complete")
	require.ErrorContains(t, err, "terminal status")
}

func TestRequireFiatDisputeResolution_FailsBeforeChainSettlement(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	svc.SetFiatOps(&mockFiatOpsForOrderTest{
		capabilities: contracts.FiatProviderCapabilities{
			ModeratedMode: contracts.FiatModeratedModeDelayedDisbursement,
		},
	})
	err := svc.requireFiatDisputeResolution(context.Background(), nil, &pb.PaymentSent{
		SettlementSpec: &pb.PaymentSent_SettlementSpec{
			Method: pb.PaymentSent_MODERATED, PayMode: "provider", EscrowType: "fiat_provider",
		},
		Coin: "fiat:paypal:USD",
	}, "fiat:paypal:USD")
	require.ErrorContains(t, err, "not atomic buyer/seller/moderator dispute allocation")
}
