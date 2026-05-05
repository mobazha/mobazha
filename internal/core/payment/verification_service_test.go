//go:build !private_distribution

package payment

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingFiatQuery struct {
	lastProviderID string
	lastPaymentID  string
	result         *contracts.PaymentDetail
	err            error
}

func (q *recordingFiatQuery) GetPayment(_ context.Context, providerID string, paymentID string) (*contracts.PaymentDetail, error) {
	q.lastProviderID = providerID
	q.lastPaymentID = paymentID
	return q.result, q.err
}

func TestPaymentVerificationService_ValidateMessage_FiatCanonicalCoin(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID: "pi_canonical_001",
		Coin:          "fiat:stripe:USD",
		Amount:        "1999",
		Method:        pb.PaymentSent_FIAT,
	}

	err := svc.ValidateMessage(iwallet.CoinType(paymentSent.Coin), orderOpen, paymentSent, 0)
	require.NoError(t, err)
}

func TestPaymentVerificationService_ValidateMessage_RejectsLegacyStripeAlias(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID: "pi_legacy_alias_001",
		Coin:          "STRIPE_USD",
		Amount:        "1999",
		Method:        pb.PaymentSent_FIAT,
	}

	err := svc.ValidateMessage(iwallet.CoinType(paymentSent.Coin), orderOpen, paymentSent, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canonical")
}

func TestPaymentVerificationService_ValidateMessage_RejectsMissingProviderSegment(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID: "pi_missing_provider_001",
		Coin:          "fiat:USD",
		Amount:        "1999",
		Method:        pb.PaymentSent_FIAT,
	}

	err := svc.ValidateMessage(iwallet.CoinType(paymentSent.Coin), orderOpen, paymentSent, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canonical format")
}

func TestPaymentVerificationService_ValidateMessage_RejectsLegacyCryptoCoin(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID: "tx_legacy_crypto_001",
		Coin:          "BSCUSDT",
		Amount:        "1999",
		Method:        pb.PaymentSent_DIRECT,
	}

	err := svc.ValidateMessage(iwallet.CoinType(paymentSent.Coin), orderOpen, paymentSent, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid payment coin")
}

func TestPaymentVerificationService_ValidateMessage_RejectsFiatMethodMismatch(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID: "tx_fiat_mismatch_001",
		Coin:          "crypto:eip155:1:native",
		Amount:        "1999",
		Method:        pb.PaymentSent_FIAT,
	}

	err := svc.ValidateMessage(iwallet.CoinType(paymentSent.Coin), orderOpen, paymentSent, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires canonical fiat coin")
}

func TestPaymentVerificationService_FetchAndVerify_CanonicalStripeCoinUsesFiatQuery(t *testing.T) {
	query := &recordingFiatQuery{
		result: &contracts.PaymentDetail{
			PaymentID:       "CAP-CANONICAL-001",
			Status:          "succeeded",
			Amount:          1999,
			Currency:        "USD",
			SellerAccountID: "seller_canonical",
		},
	}

	svc := NewPaymentVerificationService(nil, nil, query)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID: "ORDER-CANONICAL-001",
		Coin:          "fiat:stripe:USD",
		Amount:        "1999",
		Method:        pb.PaymentSent_FIAT,
	}

	vp, err := svc.FetchAndVerify(context.Background(), orderOpen, paymentSent, "")
	require.NoError(t, err)
	require.NotNil(t, vp)

	assert.Equal(t, "stripe", query.lastProviderID)
	assert.Equal(t, "ORDER-CANONICAL-001", query.lastPaymentID)
	assert.Equal(t, iwallet.TransactionID("CAP-CANONICAL-001"), vp.Transaction.ID)
	assert.Equal(t, int64(1999), vp.Transaction.Value.Int64())
}
