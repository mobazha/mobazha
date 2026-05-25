//go:build !private_distribution

package payment

import (
	"testing"
	"time"

	testutil "github.com/mobazha/mobazha3.0/internal/orders/testutil"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	pkpayment "github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func testProjectInput(order *models.Order) *projectOrderInput {
	return &projectOrderInput{order: order}
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

	coin, mode := p.derivePaymentInfo(order, nil, nil)
	require.Equal(t, "crypto:eth:eth", coin)
	require.Equal(t, pkpayment.ProductModeModerated, mode)
}

func TestDeriveFundingTarget_ManagedEscrowPendingUsesAddressMonitored(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{PaymentAddress: "0xmanagedescrow"}
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Type:           "managed_escrow",
		Address:        "0xmanagedescrow",
		SettlementSpec: pkpayment.NewManagedEscrowSpec(false).ToPending(),
	}))

	mode, target := p.deriveFundingTarget(order, "crypto:eth:eth", "1000", testProjectInput(order))
	require.Equal(t, pkpayment.SettlementModeAddressMonitored, mode)
	require.Equal(t, "0xmanagedescrow", target.Address)
}

func TestDeriveFundingTarget_RetiredClientSignedPendingProjectsAsAddressMonitored(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{PaymentAddress: "0xescrow"}
	require.NoError(t, order.SetPendingEscrowPaymentInfo(&models.PendingEscrowPaymentInfo{
		Coin:           "crypto:eip155:1:native",
		EscrowAddress:  "0xescrow",
		SettlementSpec: pkpayment.NewLegacyEVMContractSpec(false).ToPending(),
	}))

	mode, target := p.deriveFundingTarget(order, "crypto:eip155:1:native", "1000", testProjectInput(order))
	require.Equal(t, pkpayment.SettlementModeAddressMonitored, mode)
	require.Equal(t, "0xescrow", target.Address)
}

func TestDeriveFundingTarget_SolanaDefaultsToAddressMonitored(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{}

	mode, target := p.deriveFundingTarget(order, "crypto:solana:mainnet:native", "1000", testProjectInput(order))
	require.Equal(t, pkpayment.SettlementModeAddressMonitored, mode)
	require.Empty(t, target.Address)
}

func TestDeriveFundingTarget_EVMWithoutAddressDefaultsToAddressMonitored(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{}

	mode, target := p.deriveFundingTarget(order, "crypto:eip155:11155111:native", "1000", testProjectInput(order))
	require.Equal(t, pkpayment.SettlementModeAddressMonitored, mode)
	require.Empty(t, target.Address)
}

func TestDerivePaymentInfo_LegacyContractPendingUsesPendingCoin(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{}
	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	require.NoError(t, order.SetPendingEscrowPaymentInfo(&models.PendingEscrowPaymentInfo{
		Coin:           "crypto:eip155:1:native",
		EscrowAddress:  "0xescrow",
		SettlementSpec: pkpayment.NewLegacyEVMContractSpec(false).ToPending(),
	}))

	coin, mode := p.derivePaymentInfo(order, orderOpen, nil)
	require.Equal(t, "crypto:eip155:1:native", coin)
	require.Equal(t, pkpayment.ProductModeCancelable, mode)
}

func TestDerivePaymentInfo_DoesNotGuessPaymentCoinFromPricingCoin(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{}
	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	require.NoError(t, order.SetPendingEscrowPaymentInfo(&models.PendingEscrowPaymentInfo{
		EscrowAddress:  "0xescrow",
		SettlementSpec: pkpayment.NewLegacyEVMContractSpec(false).ToPending(),
	}))

	coin, mode := p.derivePaymentInfo(order, orderOpen, nil)
	require.Empty(t, coin)
	require.Equal(t, pkpayment.ProductModeCancelable, mode)
}

func TestDeriveFundingTarget_UTXOPendingUsesAddressMonitored(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{PaymentAddress: "bc1qtest"}
	require.NoError(t, order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
		Coin:           "BTC",
		Script:         "ab",
		SettlementSpec: pkpayment.NewUTXOSpec(true).ToPending(),
	}))

	mode, _ := p.deriveFundingTarget(order, "BTC", "50000", testProjectInput(order))
	require.Equal(t, pkpayment.SettlementModeAddressMonitored, mode)
}

func TestProject_FormatsUTXOAmountsAsDecimalStrings(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{PaymentAddress: "bc1qtest"}
	order.TotalReceived = "15058"
	order.PaymentVerificationStatus = models.PaymentVerificationStatusPending
	require.NoError(t, order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
		Coin:           "crypto:bip122:000000000019d6689c085ae165831e93:native",
		Amount:         30116,
		Script:         "ab",
		SettlementSpec: pkpayment.NewUTXOSpec(true).ToPending(),
	}))

	session, err := p.Project(&projectOrderInput{
		order: order,
		orderOpen: &pb.OrderOpen{
			Amount:      "30116",
			Timestamp:   timestamppb.New(time.Now()),
			PricingCoin: "USD",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "0.00030116", session.ExpectedAmount)
	require.Equal(t, "0.00030116", session.FundingTarget.Amount)
	require.Equal(t, "0.00015058", session.PaymentProgress.ObservedAmount)
	require.Equal(t, "0.00030116", session.PaymentProgress.RequiredAmount)
	require.Equal(t, "0.00015058", session.PaymentProgress.RemainingAmount)
}

func TestProject_UsesLockedUTXOPendingAmountOverOrderOpenAmount(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{PaymentAddress: "bc1qtest"}
	require.NoError(t, order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
		Coin:           "crypto:bip122:000000000019d6689c085ae165831e93:native",
		Amount:         30070,
		Script:         "ab",
		SettlementSpec: pkpayment.NewUTXOSpec(false).ToPending(),
	}))

	session, err := p.Project(&projectOrderInput{
		order: order,
		orderOpen: &pb.OrderOpen{
			Amount:      "110000000",
			Timestamp:   timestamppb.New(time.Now()),
			PricingCoin: "crypto:eip155:1:native",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "0.0003007", session.ExpectedAmount)
	require.Equal(t, "0.0003007", session.FundingTarget.Amount)
	require.Equal(t, "0.0003007", session.PaymentProgress.RequiredAmount)
	require.Equal(t, "0.0003007", session.PaymentProgress.RemainingAmount)
}

func TestProject_UsesPaymentSentAmountWhenAddressMonitoredOrderIsAlreadyPaid(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{PaymentAddress: "bc1qtest"}
	paymentSent := &pb.PaymentSent{
		Amount:         "29838",
		Coin:           "crypto:bip122:000000000019d6689c085ae165831e93:native",
		TransactionID:  "tx-1",
		SettlementSpec: pkpayment.NewUTXOSpec(false).ToPaymentSent(),
	}
	require.NoError(t, order.PutMessage(testutil.MustWrapOrderMessage(paymentSent)))

	session, err := p.Project(&projectOrderInput{
		order: order,
		orderOpen: &pb.OrderOpen{
			Amount:      "11000000000000000",
			Timestamp:   timestamppb.New(time.Now()),
			PricingCoin: "ETH",
		},
		paymentSent: paymentSent,
	})
	require.NoError(t, err)
	require.Equal(t, "0.00029838", session.ExpectedAmount)
	require.Equal(t, "0.00029838", session.FundingTarget.Amount)
	require.Equal(t, "0.00029838", session.PaymentProgress.ObservedAmount)
	require.Equal(t, "0.00029838", session.PaymentProgress.RequiredAmount)
	require.Equal(t, "0", session.PaymentProgress.RemainingAmount)
}

func TestProject_FormatsManagedEscrowAmountsAsDecimalStrings(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{
		PaymentAddress: "0xmanagedescrow",
	}
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Type:           "managed_escrow",
		Address:        "0xmanagedescrow",
		Coin:           "crypto:eip155:11155111:native",
		Amount:         7022669176100452,
		SettlementSpec: pkpayment.NewManagedEscrowSpec(false).ToPending(),
	}))

	session, err := p.Project(&projectOrderInput{
		order: order,
		orderOpen: &pb.OrderOpen{
			Amount:      "7022669176100452",
			Timestamp:   timestamppb.New(time.Now()),
			PricingCoin: "USD",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "0.007022669176100452", session.ExpectedAmount)
	require.Equal(t, "0.007022669176100452", session.FundingTarget.Amount)
	require.Equal(t, "0.007022669176100452", session.PaymentProgress.RequiredAmount)
}

func TestProject_UsesLockedManagedEscrowPendingAmountOverOrderOpenAmount(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{PaymentAddress: "0xmanagedescrow"}
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Type:           "managed_escrow",
		Address:        "0xmanagedescrow",
		Coin:           "crypto:eip155:11155111:native",
		Amount:         7022669176100452,
		SettlementSpec: pkpayment.NewManagedEscrowSpec(false).ToPending(),
	}))

	session, err := p.Project(&projectOrderInput{
		order: order,
		orderOpen: &pb.OrderOpen{
			Amount:      "11",
			Timestamp:   timestamppb.New(time.Now()),
			PricingCoin: "USD",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "0.007022669176100452", session.ExpectedAmount)
	require.Equal(t, "0.007022669176100452", session.FundingTarget.Amount)
	require.Equal(t, "0.007022669176100452", session.PaymentProgress.RequiredAmount)
}

func TestDerivePaymentInfo_PaymentSentDirectUsesDirectProductMode(t *testing.T) {
	p := &PaymentSessionProjector{}
	ps := &pb.PaymentSent{
		Coin:           "BTC",
		SettlementSpec: pkpayment.NewDirectSpec().ToPaymentSent(),
	}
	_, mode := p.derivePaymentInfo(&models.Order{}, nil, ps)
	require.Equal(t, pkpayment.ProductModeDirect, mode)
}

func TestDerivePaymentInfo_FiatMetadataUsesSettlementSpecProductMode(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{}
	specJSON, err := pkpayment.FiatMetadataSettlementSpecJSON()
	require.NoError(t, err)
	require.NoError(t, order.MergeFiatMetadata(map[string]string{
		"fiat_provider":   "stripe",
		"fiat_currency":   "USD",
		"settlement_spec": specJSON,
	}))

	coin, mode := p.derivePaymentInfo(order, nil, nil)
	require.Equal(t, "fiat:stripe:USD", coin)
	require.Equal(t, pkpayment.ProductModeCancelable, mode)
}

func TestDerivePaymentInfo_CryptoPendingOverridesStaleFiatMetadata(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{PaymentAddress: "AyoATDTgoSU9PTDw7xgqQWh6Tnz4iMGNxhETbkK5i7G3"}
	specJSON, err := pkpayment.FiatMetadataSettlementSpecJSON()
	require.NoError(t, err)
	require.NoError(t, order.MergeFiatMetadata(map[string]string{
		"fiat_provider":    "stripe",
		"fiat_currency":    "USD",
		"fiat_session_id":  "pi_default_stripe",
		"settlement_spec":  specJSON,
		"stripe_intent_id": "pi_default_stripe",
	}))
	require.NoError(t, order.SetPendingEscrowPaymentInfo(&models.PendingEscrowPaymentInfo{
		Coin:           "crypto:solana:mainnet:native",
		EscrowAddress:  order.PaymentAddress,
		SettlementSpec: pkpayment.NewSolanaEscrowSpec(false).ToPending(),
	}))

	coin, mode := p.derivePaymentInfo(order, nil, nil)
	require.Equal(t, "crypto:solana:mainnet:native", coin)
	require.Equal(t, pkpayment.ProductModeCancelable, mode)
}

func TestDerivePaymentInfo_PendingManagedEscrowOverridesPaymentSentEnvelope(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{}
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Coin:           "crypto:eip155:1:native",
		Address:        "0xmanagedescrow",
		SettlementSpec: pkpayment.NewManagedEscrowSpec(false).ToPending(),
	}))
	ps := &pb.PaymentSent{
		Coin:            "crypto:eip155:1:native",
		ContractAddress: "0xmanagedescrow",
		ToAddress:       "0xmanagedescrow",
		SettlementSpec:  pkpayment.NewDirectSpec().ToPaymentSent(),
	}
	_, mode := p.derivePaymentInfo(order, nil, ps)
	require.Equal(t, pkpayment.ProductModeCancelable, mode)
}
