//go:build !private_distribution

package payment

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/events"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	paymentpkg "github.com/mobazha/mobazha3.0/pkg/payment"
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

type recordingManagedEscrowVerifier struct {
	verifyCalls int
	lastParams  paymentpkg.DepositVerifyParams
}

func (*recordingManagedEscrowVerifier) Model() paymentpkg.PaymentModel {
	return paymentpkg.PaymentModelMonitored
}
func (*recordingManagedEscrowVerifier) Capabilities() paymentpkg.ChainCapabilities {
	return paymentpkg.ChainCapabilities{}
}
func (*recordingManagedEscrowVerifier) SetupPayment(context.Context, paymentpkg.PaymentSetupParams) (*paymentpkg.ActionResult, error) {
	return nil, nil
}
func (*recordingManagedEscrowVerifier) Confirm(context.Context, paymentpkg.ActionParams) (*paymentpkg.ActionResult, error) {
	return nil, nil
}
func (*recordingManagedEscrowVerifier) Cancel(context.Context, paymentpkg.ActionParams) (*paymentpkg.ActionResult, error) {
	return nil, nil
}
func (*recordingManagedEscrowVerifier) Complete(context.Context, paymentpkg.ActionParams) (*paymentpkg.ActionResult, error) {
	return nil, nil
}
func (*recordingManagedEscrowVerifier) DisputeRelease(context.Context, paymentpkg.ActionParams) (*paymentpkg.ActionResult, error) {
	return nil, nil
}
func (*recordingManagedEscrowVerifier) GetActionStatus(context.Context, string) (*paymentpkg.ActionStatus, error) {
	return nil, nil
}
func (*recordingManagedEscrowVerifier) AutoConfirm(context.Context, *events.CancelablePaymentReady) error {
	return nil
}
func (*recordingManagedEscrowVerifier) SignEscrowRelease(context.Context, paymentpkg.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	return nil, nil
}
func (*recordingManagedEscrowVerifier) EstimateEscrowFee(string, int, int, iwallet.FeeLevel) (iwallet.Amount, error) {
	return iwallet.NewAmount(0), nil
}
func (v *recordingManagedEscrowVerifier) VerifyDeposit(_ context.Context, params paymentpkg.DepositVerifyParams) error {
	v.verifyCalls++
	v.lastParams = params
	return nil
}
func (*recordingManagedEscrowVerifier) ValidatePaymentMessage(paymentpkg.PaymentMessageParams) error {
	return nil
}
func (*recordingManagedEscrowVerifier) VerifyPreRelease(context.Context, paymentpkg.PreReleaseParams) error {
	return nil
}

func TestPaymentVerificationService_ValidateMessage_FiatCanonicalCoin(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID:  "pi_canonical_001",
		Coin:           "fiat:stripe:USD",
		Amount:         "1999",
		SettlementSpec: paymentpkg.NewFiatSpec().ToPaymentSent(),
	}

	err := svc.ValidateMessage(iwallet.CoinType(paymentSent.Coin), orderOpen, paymentSent, 0)
	require.NoError(t, err)
}

func TestPaymentVerificationService_ValidateMessage_RejectsLegacyStripeAlias(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID:  "pi_legacy_alias_001",
		Coin:           "STRIPE_USD",
		Amount:         "1999",
		SettlementSpec: paymentpkg.NewFiatSpec().ToPaymentSent(),
	}

	err := svc.ValidateMessage(iwallet.CoinType(paymentSent.Coin), orderOpen, paymentSent, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canonical")
}

func TestPaymentVerificationService_ValidateMessage_RejectsMissingProviderSegment(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID:  "pi_missing_provider_001",
		Coin:           "fiat:USD",
		Amount:         "1999",
		SettlementSpec: paymentpkg.NewFiatSpec().ToPaymentSent(),
	}

	err := svc.ValidateMessage(iwallet.CoinType(paymentSent.Coin), orderOpen, paymentSent, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canonical format")
}

func TestPaymentVerificationService_ValidateMessage_RejectsLegacyCryptoCoin(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID:  "tx_legacy_crypto_001",
		Coin:           "BSCUSDT",
		Amount:         "1999",
		SettlementSpec: paymentpkg.NewDirectSpec().ToPaymentSent(),
	}

	err := svc.ValidateMessage(iwallet.CoinType(paymentSent.Coin), orderOpen, paymentSent, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid payment coin")
}

func TestPaymentVerificationService_ValidateMessage_RejectsFiatMethodMismatch(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID:  "tx_fiat_mismatch_001",
		Coin:           "crypto:eip155:1:native",
		Amount:         "1999",
		SettlementSpec: paymentpkg.NewFiatSpec().ToPaymentSent(),
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
		TransactionID:  "ORDER-CANONICAL-001",
		Coin:           "fiat:stripe:USD",
		Amount:         "1999",
		SettlementSpec: paymentpkg.NewFiatSpec().ToPaymentSent(),
	}

	vp, err := svc.FetchAndVerify(context.Background(), orderOpen, paymentSent, "")
	require.NoError(t, err)
	require.NotNil(t, vp)

	assert.Equal(t, "stripe", query.lastProviderID)
	assert.Equal(t, "ORDER-CANONICAL-001", query.lastPaymentID)
	assert.Equal(t, iwallet.TransactionID("CAP-CANONICAL-001"), vp.Transaction.ID)
	assert.Equal(t, int64(1999), vp.Transaction.Value.Int64())
}

func TestPaymentVerificationService_FetchAndVerify_MonitorRelayedManagedEscrowPayment(t *testing.T) {
	registry := paymentpkg.NewRegistry()
	verifier := &recordingManagedEscrowVerifier{}
	registry.RegisterV2(iwallet.ChainEthereum, verifier)
	svc := NewPaymentVerificationService(registry, nil, nil)

	paymentSent := &pb.PaymentSent{
		TransactionID:   "0xmanagedescrow",
		Coin:            "crypto:eip155:1:native",
		ContractAddress: "0x1111111111111111111111111111111111111111",
		ToAddress:       "0x1111111111111111111111111111111111111111",
		Amount:          "12345",
		SettlementSpec:  paymentpkg.NewManagedEscrowSpec(false).ToPaymentSent(),
	}

	vp, err := svc.FetchAndVerify(context.Background(), &pb.OrderOpen{}, paymentSent, paymentSent.ToAddress)
	require.NoError(t, err)
	require.NotNil(t, vp)
	assert.Equal(t, iwallet.CoinType(paymentSent.Coin), vp.CoinType)
	assert.Equal(t, iwallet.TransactionID("0xmanagedescrow"), vp.Transaction.ID)
	assert.Equal(t, int64(12345), vp.Transaction.Value.Int64())
	require.Len(t, vp.Transaction.To, 1)
	assert.Equal(t, paymentSent.ToAddress, vp.Transaction.To[0].Address.String())
	assert.Equal(t, 1, verifier.verifyCalls)
	assert.Equal(t, iwallet.CoinType(paymentSent.Coin), verifier.lastParams.CoinType)
	assert.Equal(t, paymentSent.TransactionID, verifier.lastParams.TxHash)
	assert.Equal(t, paymentSent.ContractAddress, verifier.lastParams.ContractAddr)
	assert.Equal(t, paymentSent.Amount, verifier.lastParams.PaymentAmount)
}

func TestPaymentVerificationService_FetchAndVerify_MonitorRelayedManagedEscrowPaymentRequiresRegistry(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	paymentSent := &pb.PaymentSent{
		TransactionID:   "0xmanagedescrow",
		Coin:            "crypto:eip155:1:native",
		ContractAddress: "0x1111111111111111111111111111111111111111",
		ToAddress:       "0x1111111111111111111111111111111111111111",
		Amount:          "12345",
		SettlementSpec:  paymentpkg.NewManagedEscrowSpec(false).ToPaymentSent(),
	}

	_, err := svc.FetchAndVerify(context.Background(), &pb.OrderOpen{}, paymentSent, paymentSent.ToAddress)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry not configured")
}

func TestPaymentVerificationService_FetchAndVerify_MonitorRelayedManagedEscrowPaymentRequiresTxID(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	paymentSent := &pb.PaymentSent{
		Coin:            "crypto:eip155:1:native",
		ContractAddress: "0x1111111111111111111111111111111111111111",
		ToAddress:       "0x1111111111111111111111111111111111111111",
		Amount:          "12345",
		SettlementSpec:  paymentpkg.NewManagedEscrowSpec(false).ToPaymentSent(),
	}

	_, err := svc.FetchAndVerify(context.Background(), &pb.OrderOpen{}, paymentSent, paymentSent.ToAddress)
	require.ErrorIs(t, err, ErrPaymentNotConfirmed)
}
