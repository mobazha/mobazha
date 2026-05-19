//go:build !private_distribution

package payment

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	pkpayment "github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/stretchr/testify/require"
)

func testProjectInput(order *models.Order, legacyManagedEscrow bool) *projectOrderInput {
	input := &projectOrderInput{order: order, isManagedEscrowOrder: legacyManagedEscrow}
	if spec, ok := pkpayment.ResolveSettlementSpecFromOrder(order); ok {
		input.hasSpec = true
		input.spec = spec
	}
	return input
}

func TestDerivePaymentInfo_ManagedEscrowPendingUsesSettlementSpecMethod(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{}
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Coin:    "crypto:eth:eth",
		Address: "0xmanagedescrow",
		SettlementSpec: &models.PendingSettlementSpec{
			Method:     "MODERATED",
			PayMode:    "address_monitored",
			EscrowType: "managed_escrow",
		},
		Moderated: false,
	}))

	coin, mode, kind := p.derivePaymentInfo(order, nil, nil)
	require.Equal(t, "crypto:eth:eth", coin)
	require.Equal(t, pkpayment.ProductModeModerated, mode)
	require.Empty(t, kind)
}

func TestDeriveFundingTarget_ManagedEscrowPendingUsesAddressMonitored(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{PaymentAddress: "0xmanagedescrow"}
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Type:    "managed_escrow",
		Address: "0xmanagedescrow",
		SettlementSpec: pkpayment.NewManagedEscrowSpec(false).ToPending(),
	}))

	mode, target := p.deriveFundingTarget(order, "crypto:eth:eth", "1000", testProjectInput(order, false))
	require.Equal(t, pkpayment.SettlementModeAddressMonitored, mode)
	require.Equal(t, "0xmanagedescrow", target.Address)
}

func TestDeriveFundingTarget_ClientSignedPendingUsesEscrowV1(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{PaymentAddress: "0xescrow"}
	require.NoError(t, order.SetPendingClientSignedPaymentInfo(&models.PendingClientSignedPaymentInfo{
		Coin:          "crypto:eip155:1:native",
		EscrowAddress: "0xescrow",
		SettlementSpec: pkpayment.NewClientSignedEVMSpec(false).ToPending(),
	}))

	mode, _ := p.deriveFundingTarget(order, "crypto:eip155:1:native", "1000", testProjectInput(order, false))
	require.Equal(t, pkpayment.SettlementModeEscrowV1, mode)
}

func TestDerivePaymentInfo_ClientSignedPendingUsesPendingCoin(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{}
	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	require.NoError(t, order.SetPendingClientSignedPaymentInfo(&models.PendingClientSignedPaymentInfo{
		Coin:           "crypto:eip155:1:native",
		EscrowAddress:  "0xescrow",
		SettlementSpec: pkpayment.NewClientSignedEVMSpec(false).ToPending(),
	}))

	coin, mode, kind := p.derivePaymentInfo(order, orderOpen, nil)
	require.Equal(t, "crypto:eip155:1:native", coin)
	require.Equal(t, pkpayment.ProductModeCancelable, mode)
	require.Empty(t, kind)
}

func TestDeriveFundingTarget_UTXOPendingUsesAddressMonitored(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{PaymentAddress: "bc1qtest"}
	require.NoError(t, order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
		Coin:           "BTC",
		Script:         "ab",
		SettlementSpec: pkpayment.NewUTXOSpec(true).ToPending(),
	}))

	mode, _ := p.deriveFundingTarget(order, "BTC", "50000", testProjectInput(order, false))
	require.Equal(t, pkpayment.SettlementModeAddressMonitored, mode)
}

func TestDerivePaymentInfo_PaymentSentDirectUsesDirectProductMode(t *testing.T) {
	p := &PaymentSessionProjector{}
	ps := &pb.PaymentSent{
		Coin:   "BTC",
		Method: pb.PaymentSent_DIRECT,
	}
	_, mode, kind := p.derivePaymentInfo(&models.Order{}, nil, ps)
	require.Equal(t, pkpayment.ProductModeDirect, mode)
	require.Equal(t, "PAYMENT_SENT_DIRECT", kind)
}

func TestDerivePaymentInfo_PendingManagedEscrowOverridesLegacyDirectPaymentSent(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{}
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Coin:           "crypto:eip155:1:native",
		Address:        "0xmanagedescrow",
		SettlementSpec: pkpayment.NewManagedEscrowSpec(false).ToPending(),
	}))
	ps := &pb.PaymentSent{
		Coin:            "crypto:eip155:1:native",
		Method:          pb.PaymentSent_DIRECT,
		ContractAddress: "0xmanagedescrow",
		ToAddress:       "0xmanagedescrow",
	}
	_, mode, kind := p.derivePaymentInfo(order, nil, ps)
	require.Equal(t, pkpayment.ProductModeCancelable, mode)
	require.Equal(t, "PAYMENT_SENT_CANCELABLE", kind)
}
