package payment

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
)

func TestSettlementSpec_Validate(t *testing.T) {
	valid := []SettlementSpec{
		NewDirectSpec(),
		NewUTXOSpec(false),
		NewUTXOSpec(true),
		NewManagedEscrowSpec(false),
		NewManagedEscrowSpec(true),
		NewClientSignedEVMSpec(false),
		NewClientSignedEVMSpec(true),
		NewClientSignedSolanaSpec(false),
		NewFiatSpec(),
	}
	for _, spec := range valid {
		require.NoError(t, spec.Validate(), spec)
	}

	invalid := []SettlementSpec{
		{Method: pb.PaymentSent_DIRECT, PayMode: PayModeClientSigned, EscrowType: EscrowTypeNone},
		{Method: pb.PaymentSent_CANCELABLE, PayMode: PayModeAddressMonitored, EscrowType: EscrowTypeEVMContract},
		{Method: pb.PaymentSent_MODERATED, PayMode: PayModeProvider, EscrowType: EscrowTypeFiatProvider},
		{Method: pb.PaymentSent_FIAT, PayMode: PayModeAddressMonitored, EscrowType: EscrowTypeFiatProvider},
		{Method: pb.PaymentSent_RWA_ESCROW, PayMode: PayModeAddressMonitored, EscrowType: EscrowTypeNone},
	}
	for _, spec := range invalid {
		require.Error(t, spec.Validate(), spec)
	}
}

func TestSettlementSpec_Helpers(t *testing.T) {
	safe := NewManagedEscrowSpec(true)
	require.True(t, managed_escrow.IsModerated())
	require.True(t, managed_escrow.RequiresModerator())
	require.True(t, managed_escrow.IsAddressMonitored())
	require.True(t, managed_escrow.UsesManagedEscrow())
	require.False(t, managed_escrow.IsDirect())

	evm := NewClientSignedEVMSpec(false)
	require.True(t, evm.IsClientSigned())
	require.True(t, evm.UsesLegacyContract())
	require.False(t, evm.UsesManagedEscrow())
}

func TestSettlementSpecFromPending_RoundTrip(t *testing.T) {
	orig := NewUTXOSpec(true)
	pending := orig.ToPending()
	got, err := SettlementSpecFromPending(pending)
	require.NoError(t, err)
	require.Equal(t, orig, got)
}

func TestResolveSettlementSpecFromPending_Fallback(t *testing.T) {
	utxoSpec, ok := ResolveSettlementSpecFromPendingUTXO(&models.PendingUTXOPaymentInfo{
		Moderator: "mod-peer",
	})
	require.True(t, ok)
	require.Equal(t, NewUTXOSpec(true), utxoSpec)

	managed_escrowSpec, ok := ResolveSettlementSpecFromPendingManagedEscrow(&models.PendingManagedEscrowPaymentInfo{
		Moderated: false,
	})
	require.True(t, ok)
	require.Equal(t, NewManagedEscrowSpec(false), managed_escrowSpec)
}

func TestResolveSettlementSpecFromOrder(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.MergeFiatMetadata(map[string]string{
		"fiat_provider": "stripe",
	}))
	spec, ok := ResolveSettlementSpecFromOrder(order)
	require.True(t, ok)
	require.Equal(t, NewFiatSpec(), spec)
}

func TestSettlementSpecFromPaymentData_ClientSignedEVM(t *testing.T) {
	pd := &models.PaymentData{
		Method: pb.PaymentSent_CANCELABLE,
		Coin:   "crypto:eip155:1:native",
	}
	spec, ok := SettlementSpecFromPaymentData(pd)
	require.True(t, ok)
	require.Equal(t, NewClientSignedEVMSpec(false), spec)
}

func TestResolveSettlementSpecFromOrder_ClientSignedPending(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingClientSignedPaymentInfo(&models.PendingClientSignedPaymentInfo{
		Coin:           "crypto:eip155:1:native",
		EscrowAddress:  "0xescrow",
		SettlementSpec: NewClientSignedEVMSpec(true).ToPending(),
	}))
	spec, ok := ResolveSettlementSpecFromOrder(order)
	require.True(t, ok)
	require.Equal(t, NewClientSignedEVMSpec(true), spec)
}

func TestSettlementSpecFromFiatMetadata(t *testing.T) {
	specJSON, err := FiatMetadataSettlementSpecJSON()
	require.NoError(t, err)
	order := &models.Order{}
	require.NoError(t, order.MergeFiatMetadata(map[string]string{
		"fiat_provider":   "stripe",
		"settlement_spec": specJSON,
	}))
	spec, ok := ResolveSettlementSpecFromOrder(order)
	require.True(t, ok)
	require.Equal(t, NewFiatSpec(), spec)
}

func TestMethodIsDirect(t *testing.T) {
	require.True(t, MethodIsDirect(pb.PaymentSent_DIRECT))
	require.False(t, MethodIsDirect(pb.PaymentSent_CANCELABLE))
	require.False(t, MethodIsDirect(pb.PaymentSent_MODERATED))
}

func TestEffectivePaymentMethod_LegacyDirectWithEscrowFields(t *testing.T) {
	ps := &pb.PaymentSent{
		Method:          pb.PaymentSent_DIRECT,
		ContractAddress: "0xmanagedescrow",
		ToAddress:       "0xmanagedescrow",
	}
	require.Equal(t, pb.PaymentSent_CANCELABLE, EffectivePaymentMethod(ps))

	ps.Moderator = "mod-peer"
	ps.ModeratorAddress = "0xmod"
	require.Equal(t, pb.PaymentSent_MODERATED, EffectivePaymentMethod(ps))

	ps = &pb.PaymentSent{
		Method: pb.PaymentSent_DIRECT,
		Script: "5221...",
	}
	require.Equal(t, pb.PaymentSent_CANCELABLE, EffectivePaymentMethod(ps))
}

func TestEffectivePaymentMethod_DirectTokenTransferStaysDirect(t *testing.T) {
	ps := &pb.PaymentSent{
		Method:          pb.PaymentSent_DIRECT,
		ContractAddress: "0xtoken",
		ToAddress:       "0xmerchant",
	}
	require.Equal(t, pb.PaymentSent_DIRECT, EffectivePaymentMethod(ps))
}

func TestIsNonEscrowDirectPayment_PendingManagedEscrowOverridesLegacyDirectEnvelope(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Address:        "0xmanagedescrow",
		SettlementSpec: NewManagedEscrowSpec(false).ToPending(),
	}))
	ps := &pb.PaymentSent{Method: pb.PaymentSent_DIRECT, ContractAddress: "0xmanagedescrow"}
	require.False(t, IsNonEscrowDirectPayment(order, ps))
}

func TestResolvedPaymentMethod_PendingSpecOverridesLegacyDirectEnvelope(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Address:        "0xmanagedescrow",
		SettlementSpec: NewManagedEscrowSpec(true).ToPending(),
	}))
	ps := &pb.PaymentSent{Method: pb.PaymentSent_DIRECT, ContractAddress: "0xmanagedescrow"}
	require.Equal(t, pb.PaymentSent_MODERATED, ResolvedPaymentMethod(order, ps))
}

func TestResolvedPaymentMethod_PendingClientSignedOverridesDirectTokenLikeEnvelope(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingClientSignedPaymentInfo(&models.PendingClientSignedPaymentInfo{
		Coin:           "crypto:eip155:1:native",
		EscrowAddress:  "0xescrow",
		SettlementSpec: NewClientSignedEVMSpec(false).ToPending(),
	}))
	ps := &pb.PaymentSent{
		Method:          pb.PaymentSent_DIRECT,
		ContractAddress: "0xtoken",
		ToAddress:       "0xmerchant",
	}
	require.Equal(t, pb.PaymentSent_CANCELABLE, ResolvedPaymentMethod(order, ps))
}

func TestResolveSettlementSpecFromPending_ExplicitSpec(t *testing.T) {
	explicit := NewManagedEscrowSpec(true).ToPending()
	spec, ok := ResolveSettlementSpecFromPendingManagedEscrow(&models.PendingManagedEscrowPaymentInfo{
		SettlementSpec: explicit,
		Moderated:      false, // legacy field ignored when spec present
	})
	require.True(t, ok)
	require.Equal(t, NewManagedEscrowSpec(true), spec)
}

func TestUsesUTXOScriptEscrow_DistinguishesManagedEscrowFromAddressMonitored(t *testing.T) {
	ps := &pb.PaymentSent{
		Method: pb.PaymentSent_CANCELABLE,
		Coin:   "crypto:eip155:11155111:native",
	}

	managed_escrowOrder := &models.Order{}
	require.NoError(t, managed_escrowOrder.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Coin:           ps.Coin,
		Address:        "0xmanagedescrow",
		SettlementSpec: NewManagedEscrowSpec(false).ToPending(),
	}))
	require.True(t, UsesAddressMonitoredPayMode(managed_escrowOrder, ps))
	require.False(t, UsesUTXOScriptEscrow(managed_escrowOrder, ps))

	utxoPS := &pb.PaymentSent{
		Method: pb.PaymentSent_CANCELABLE,
		Coin:   "BTC",
	}
	utxoOrder := &models.Order{}
	require.NoError(t, utxoOrder.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
		Coin:           utxoPS.Coin,
		Script:         "5221",
		SettlementSpec: NewUTXOSpec(false).ToPending(),
	}))
	require.True(t, UsesAddressMonitoredPayMode(utxoOrder, utxoPS))
	require.True(t, UsesUTXOScriptEscrow(utxoOrder, utxoPS))
}

func TestUsesClientSignedPayMode_DistinguishesManagedEscrowFromLegacyContract(t *testing.T) {
	managed_escrowPS := &pb.PaymentSent{
		Method: pb.PaymentSent_CANCELABLE,
		Coin:   "crypto:eip155:11155111:native",
	}
	managed_escrowOrder := &models.Order{}
	require.NoError(t, managed_escrowOrder.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Coin:           managed_escrowPS.Coin,
		Address:        "0xmanagedescrow",
		SettlementSpec: NewManagedEscrowSpec(false).ToPending(),
	}))
	require.False(t, UsesClientSignedPayMode(managed_escrowOrder, managed_escrowPS))

	clientSignedPS := &pb.PaymentSent{
		Method: pb.PaymentSent_CANCELABLE,
		Coin:   "crypto:eip155:1:native",
	}
	clientSignedOrder := &models.Order{}
	require.NoError(t, clientSignedOrder.SetPendingClientSignedPaymentInfo(&models.PendingClientSignedPaymentInfo{
		Coin:           clientSignedPS.Coin,
		EscrowAddress:  "0xlegacy",
		SettlementSpec: NewClientSignedEVMSpec(false).ToPending(),
	}))
	require.True(t, UsesClientSignedPayMode(clientSignedOrder, clientSignedPS))
}
