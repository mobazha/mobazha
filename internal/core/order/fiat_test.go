package order

import (
	"context"
	"errors"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
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
