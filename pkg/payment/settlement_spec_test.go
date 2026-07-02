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
		NewSolanaEscrowSpec(false),
		NewSolanaEscrowSpec(true),
		NewLegacyEVMContractSpec(false),
		NewLegacyEVMContractSpec(true),
		NewLegacySolanaProgramSpec(false),
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
	managed := NewManagedEscrowSpec(true)
	require.True(t, managed.IsModerated())
	require.True(t, managed.RequiresModerator())
	require.True(t, managed.IsAddressMonitored())
	require.True(t, managed.UsesManagedEscrow())
	require.False(t, managed.IsDirect())

	solanaEscrow := NewSolanaEscrowSpec(false)
	require.True(t, solanaEscrow.IsAddressMonitored())
	require.True(t, solanaEscrow.UsesSolanaEscrow())
	require.False(t, solanaEscrow.UsesLegacyContract())

	evm := NewLegacyEVMContractSpec(false)
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

	managedEscrowSpec, ok := ResolveSettlementSpecFromPendingManagedEscrow(&models.PendingManagedEscrowInfo{
		Moderated: false,
	})
	require.True(t, ok)
	require.Equal(t, NewManagedEscrowSpec(false), managedEscrowSpec)
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

func TestSettlementSpecFromPaymentData_EVMRequiresExplicitSpec(t *testing.T) {
	pd := &models.PaymentData{
		Method: pb.PaymentSent_CANCELABLE,
		Coin:   "crypto:eip155:1:native",
	}
	spec, ok := SettlementSpecFromPaymentData(pd)
	require.False(t, ok)
	require.Equal(t, SettlementSpec{}, spec)

	pd.SettlementSpec = NewManagedEscrowSpec(false).ToPending()
	spec, ok = SettlementSpecFromPaymentData(pd)
	require.True(t, ok)
	require.Equal(t, NewManagedEscrowSpec(false), spec)
}

func TestResolveSettlementSpec_PaymentSentManagedEnvelope(t *testing.T) {
	ps := &pb.PaymentSent{
		Coin:            "crypto:eip155:1:native",
		ContractAddress: "0x1111111111111111111111111111111111111111",
		ToAddress:       "0x1111111111111111111111111111111111111111",
		SettlementSpec:  NewManagedEscrowSpec(false).ToPaymentSent(),
	}

	spec, ok := ResolveSettlementSpec(nil, ps)
	require.True(t, ok)
	require.Equal(t, NewManagedEscrowSpec(false), spec)
}

func TestResolveSettlementSpec_PaymentSentLegacyEVMEnvelope(t *testing.T) {
	ps := &pb.PaymentSent{
		Coin:            "crypto:eip155:1:native",
		ContractAddress: "0x1111111111111111111111111111111111111111",
		ToAddress:       "0x2222222222222222222222222222222222222222",
		Script:          "5221",
		SettlementSpec:  NewLegacyEVMContractSpec(false).ToPaymentSent(),
	}

	spec, ok := ResolveSettlementSpec(nil, ps)
	require.True(t, ok)
	require.Equal(t, NewLegacyEVMContractSpec(false), spec)
}

func TestResolveSettlementSpec_PaymentSentDoesNotInferManagedRouteFromShape(t *testing.T) {
	ps := &pb.PaymentSent{
		Coin:            "crypto:eip155:1:native",
		ContractAddress: "0x1111111111111111111111111111111111111111",
		ToAddress:       "0x1111111111111111111111111111111111111111",
	}

	spec, ok := ResolveSettlementSpec(nil, ps)
	require.False(t, ok)
	require.Equal(t, SettlementSpec{}, spec)
}

func TestResolveSettlementSpecFromOrder_LegacyEVMContractPending(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingEscrowPaymentInfo(&models.PendingEscrowPaymentInfo{
		Coin:           "crypto:eip155:1:native",
		EscrowAddress:  "0xescrow",
		SettlementSpec: NewLegacyEVMContractSpec(true).ToPending(),
	}))
	spec, ok := ResolveSettlementSpecFromOrder(order)
	require.True(t, ok)
	require.Equal(t, NewLegacyEVMContractSpec(true), spec)
}

func TestResolveSettlementSpecFromPendingEscrow_ExplicitSolanaEscrowSpec(t *testing.T) {
	info := &models.PendingEscrowPaymentInfo{
		Coin:           "crypto:solana:native",
		EscrowAddress:  "So11111111111111111111111111111111111111112",
		SettlementSpec: NewSolanaEscrowSpec(true).ToPending(),
	}

	spec, ok := ResolveSettlementSpecFromPendingEscrow(info)
	require.True(t, ok)
	require.Equal(t, NewSolanaEscrowSpec(true), spec)
}

func TestResolveSettlementSpecFromPendingEscrow_RequiresExplicitSpec(t *testing.T) {
	info := &models.PendingEscrowPaymentInfo{
		Coin:          "crypto:eip155:1:native",
		EscrowAddress: "0xescrow",
		Moderator:     "mod-peer",
	}

	spec, ok := ResolveSettlementSpecFromPendingEscrow(info)

	require.False(t, ok)
	require.Equal(t, SettlementSpec{}, spec)
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

func TestPaymentSentSettlementSpec_UsesExplicitMethodWithoutFieldInference(t *testing.T) {
	ps := &pb.PaymentSent{
		ContractAddress:  "0xmanagedescrow",
		ToAddress:        "0xmanagedescrow",
		Moderator:        "mod-peer",
		ModeratorAddress: "0xmod",
		Script:           "5221...",
		SettlementSpec:   NewDirectSpec().ToPaymentSent(),
	}
	require.NotNil(t, ps.GetSettlementSpec())
	require.Equal(t, pb.PaymentSent_DIRECT, ps.GetSettlementSpec().GetMethod())

	ps.SettlementSpec = NewManagedEscrowSpec(false).ToPaymentSent()
	require.Equal(t, pb.PaymentSent_CANCELABLE, ps.GetSettlementSpec().GetMethod())

	ps.SettlementSpec = NewManagedEscrowSpec(true).ToPaymentSent()
	require.Equal(t, pb.PaymentSent_MODERATED, ps.GetSettlementSpec().GetMethod())
}

func TestPaymentSentSettlementSpec_DirectTokenTransferStaysDirect(t *testing.T) {
	ps := &pb.PaymentSent{
		ContractAddress: "0xtoken",
		ToAddress:       "0xmerchant",
		SettlementSpec:  NewDirectSpec().ToPaymentSent(),
	}
	require.NotNil(t, ps.GetSettlementSpec())
	require.Equal(t, pb.PaymentSent_DIRECT, ps.GetSettlementSpec().GetMethod())
}

func TestIsNonEscrowDirectPayment_PendingManagedEscrowSpecOverridesPaymentSentEnvelope(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingManagedEscrowInfo(&models.PendingManagedEscrowInfo{
		Address:        "0xmanagedescrow",
		SettlementSpec: NewManagedEscrowSpec(false).ToPending(),
	}))
	ps := &pb.PaymentSent{ContractAddress: "0xmanagedescrow", SettlementSpec: NewDirectSpec().ToPaymentSent()}
	require.False(t, IsNonEscrowDirectPayment(order, ps))
}

func TestResolvedPaymentMethod_PendingSpecOverridesPaymentSentEnvelope(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingManagedEscrowInfo(&models.PendingManagedEscrowInfo{
		Address:        "0xmanagedescrow",
		SettlementSpec: NewManagedEscrowSpec(true).ToPending(),
	}))
	ps := &pb.PaymentSent{ContractAddress: "0xmanagedescrow", SettlementSpec: NewDirectSpec().ToPaymentSent()}
	method, ok := ResolvedPaymentMethod(order, ps)
	require.True(t, ok)
	require.Equal(t, pb.PaymentSent_MODERATED, method)
}

func TestResolvedPaymentMethod_PendingEscrowOverridesDirectTokenLikeEnvelope(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingEscrowPaymentInfo(&models.PendingEscrowPaymentInfo{
		Coin:           "crypto:eip155:1:native",
		EscrowAddress:  "0xescrow",
		SettlementSpec: NewLegacyEVMContractSpec(false).ToPending(),
	}))
	ps := &pb.PaymentSent{
		ContractAddress: "0xtoken",
		ToAddress:       "0xmerchant",
		SettlementSpec:  NewDirectSpec().ToPaymentSent(),
	}
	method, ok := ResolvedPaymentMethod(order, ps)
	require.True(t, ok)
	require.Equal(t, pb.PaymentSent_CANCELABLE, method)
}

func TestResolveSettlementSpecFromPending_ExplicitSpec(t *testing.T) {
	explicit := NewManagedEscrowSpec(true).ToPending()
	spec, ok := ResolveSettlementSpecFromPendingManagedEscrow(&models.PendingManagedEscrowInfo{
		SettlementSpec: explicit,
		Moderated:      false, // legacy field ignored when spec present
	})
	require.True(t, ok)
	require.Equal(t, NewManagedEscrowSpec(true), spec)
}

func TestUsesUTXOScriptEscrow_DistinguishesManagedFromAddressMonitored(t *testing.T) {
	ps := &pb.PaymentSent{
		Coin:           "crypto:eip155:11155111:native",
		SettlementSpec: NewManagedEscrowSpec(false).ToPaymentSent(),
	}

	managedEscrowOrder := &models.Order{}
	require.NoError(t, managedEscrowOrder.SetPendingManagedEscrowInfo(&models.PendingManagedEscrowInfo{
		Coin:           ps.Coin,
		Address:        "0xmanagedescrow",
		SettlementSpec: NewManagedEscrowSpec(false).ToPending(),
	}))
	require.True(t, UsesAddressMonitoredPayMode(managedEscrowOrder, ps))
	require.False(t, UsesUTXOScriptEscrow(managedEscrowOrder, ps))

	utxoPS := &pb.PaymentSent{
		Coin:           "BTC",
		SettlementSpec: NewUTXOSpec(false).ToPaymentSent(),
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

func TestUsesClientSignedPayMode_DistinguishesManagedFromLegacyContract(t *testing.T) {
	managedEscrowPS := &pb.PaymentSent{
		Coin:           "crypto:eip155:11155111:native",
		SettlementSpec: NewManagedEscrowSpec(false).ToPaymentSent(),
	}
	managedEscrowOrder := &models.Order{}
	require.NoError(t, managedEscrowOrder.SetPendingManagedEscrowInfo(&models.PendingManagedEscrowInfo{
		Coin:           managedEscrowPS.Coin,
		Address:        "0xmanagedescrow",
		SettlementSpec: NewManagedEscrowSpec(false).ToPending(),
	}))
	require.False(t, UsesClientSignedPayMode(managedEscrowOrder, managedEscrowPS))

	legacyPS := &pb.PaymentSent{
		Coin:           "crypto:eip155:1:native",
		SettlementSpec: NewLegacyEVMContractSpec(false).ToPaymentSent(),
	}
	legacyOrder := &models.Order{}
	require.NoError(t, legacyOrder.SetPendingEscrowPaymentInfo(&models.PendingEscrowPaymentInfo{
		Coin:           legacyPS.Coin,
		EscrowAddress:  "0xlegacy",
		SettlementSpec: NewLegacyEVMContractSpec(false).ToPending(),
	}))
	require.True(t, UsesClientSignedPayMode(legacyOrder, legacyPS))
}
