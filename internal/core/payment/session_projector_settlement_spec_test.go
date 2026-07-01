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

func TestDeriveFundingTarget_BCHIncludesAmountInQRPayload(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{PaymentAddress: "ppu9yncdpjgwmq8h5khefmkhrat6pdp08sqsjd0mrc"}

	mode, target := p.deriveFundingTarget(order,
		"crypto:bitcoincash:mainnet:native",
		"0.00016522",
		testProjectInput(order),
	)

	require.Equal(t, pkpayment.SettlementModeAddressMonitored, mode)
	require.Equal(t, "bitcoincash:ppu9yncdpjgwmq8h5khefmkhrat6pdp08sqsjd0mrc?amount=0.00016522", target.QRPayload)
}

func TestDeriveFundingTarget_UTXOQRPayloadUsesCoinScheme(t *testing.T) {
	tests := []struct {
		name string
		coin string
		addr string
		want string
	}{
		{
			name: "btc",
			coin: "crypto:bip122:000000000019d6689c085ae165831e93:native",
			addr: "bc1qtest",
			want: "bitcoin:bc1qtest?amount=0.001",
		},
		{
			name: "ltc",
			coin: "crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native",
			addr: "ltc1qtest",
			want: "litecoin:ltc1qtest?amount=0.001",
		},
		{
			name: "zec",
			coin: "crypto:zcash:mainnet:native",
			addr: "t1test",
			want: "zcash:t1test?amount=0.001",
		},
	}

	p := &PaymentSessionProjector{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := &models.Order{PaymentAddress: tt.addr}
			_, target := p.deriveFundingTarget(order, tt.coin, "0.001", testProjectInput(order))
			require.Equal(t, tt.want, target.QRPayload)
		})
	}
}

func TestDeriveFundingTarget_EVMIncludesAmountInQRPayload(t *testing.T) {
	p := &PaymentSessionProjector{}
	addr := "0x259d0C6C6c53a746Fd8EA025AB5b47dfd842baCB"
	order := &models.Order{PaymentAddress: addr}

	mode, target := p.deriveFundingTarget(order,
		"crypto:eip155:1:native",
		"0.011",
		testProjectInput(order),
	)

	require.Equal(t, pkpayment.SettlementModeAddressMonitored, mode)
	require.Equal(t, "ethereum:"+addr+"@1?value=0.011e18", target.QRPayload)
}

func TestDeriveFundingTarget_SolanaIncludesAmountInQRPayload(t *testing.T) {
	p := &PaymentSessionProjector{}
	addr := "7EqQDM5s8MWTD5M9s8MWTD5M9s8MWTD5M9s8MWTD5M9"
	order := &models.Order{PaymentAddress: addr}

	mode, target := p.deriveFundingTarget(order,
		"crypto:solana:mainnet:native",
		"0.5",
		testProjectInput(order),
	)

	require.Equal(t, pkpayment.SettlementModeAddressMonitored, mode)
	require.Equal(t, "solana:"+addr+"?amount=0.5", target.QRPayload)
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

func TestProject_UsesPendingObservationAmountForPaymentProgress(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{PaymentAddress: "bitcoincash:qtest"}
	order.PaymentVerificationStatus = models.PaymentVerificationStatusPending
	require.NoError(t, order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
		Coin:               "crypto:bitcoincash:mainnet:native",
		Amount:             16414,
		Script:             "ab",
		ConfirmationPolicy: models.PaymentConfirmationPolicyMempoolAccepted,
		SettlementSpec:     pkpayment.NewUTXOSpec(true).ToPending(),
	}))

	observedAt := time.Date(2026, 5, 29, 5, 57, 38, 0, time.UTC)
	session, err := p.Project(&projectOrderInput{
		order:             order,
		observedAmountRaw: "16414",
		obsCount:          1,
		lastObsAt:         &observedAt,
		observations: []models.PaymentObservation{
			testSessionObservation("obs-bch-pending", "", "", "16414", observedAt, models.PaymentObservationStatusPending),
		},
		orderOpen: &pb.OrderOpen{
			Amount:      "16414",
			Timestamp:   timestamppb.New(time.Now()),
			PricingCoin: "USD",
		},
	})
	require.NoError(t, err)
	require.Equal(t, pkpayment.SessionStatusFundedPendingVerification, session.Status)
	require.Equal(t, models.PaymentConfirmationPolicyMempoolAccepted, session.ConfirmationPolicy)
	require.Equal(t, "0.00016414", session.PaymentProgress.ObservedAmount)
	require.Equal(t, "0.00016414", session.PaymentProgress.RequiredAmount)
	require.Equal(t, "0", session.PaymentProgress.RemainingAmount)
	require.Equal(t, 1, session.PaymentProgress.ObservationCount)
	require.Equal(t, &observedAt, session.PaymentProgress.LastObservedAt)
	require.Len(t, session.PaymentProgress.Observations, 1)
	require.Equal(t, "obs-bch-pending-tx", session.PaymentProgress.Observations[0].TxHash)
	require.Equal(t, "0.00016414", session.PaymentProgress.Observations[0].Amount)
}

func TestProject_UsesPaymentSentFundingFactsForPaymentProgress(t *testing.T) {
	p := &PaymentSessionProjector{}
	order := &models.Order{ID: models.OrderID("order-facts"), PaymentAddress: "0xmanagedescrow"}
	order.PaymentVerificationStatus = models.PaymentVerificationStatusVerified
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Coin:           "crypto:eip155:1:native",
		Address:        "0xmanagedescrow",
		Amount:         1000,
		SettlementSpec: pkpayment.NewManagedEscrowSpec(false).ToPending(),
	}))
	observedAt := time.Date(2026, 5, 31, 8, 0, 0, 0, time.UTC)
	paymentSent := &pb.PaymentSent{
		TransactionID:  "0xtx-2",
		Coin:           "crypto:eip155:1:native",
		Amount:         "1000",
		ToAddress:      "0xmanagedescrow",
		SettlementSpec: pkpayment.NewManagedEscrowSpec(false).ToPaymentSent(),
		FundingFacts: []*pb.PaymentSent_FundingFact{
			{
				Id:             "fact-1",
				ChainNamespace: "eip155",
				ChainReference: "1",
				TxHash:         "0xtx-1",
				TxHashSource:   models.PaymentTxHashSourceChainTx,
				EventType:      models.PaymentEventManagedEscrowReceived,
				ToAddress:      "0xmanagedescrow",
				Amount:         "400",
				Status:         models.PaymentObservationStatusConfirmed,
				ObservedAt:     timestamppb.New(observedAt),
			},
			{
				Id:             "fact-2",
				ChainNamespace: "eip155",
				ChainReference: "1",
				TxHash:         "0xtx-2",
				TxHashSource:   models.PaymentTxHashSourceChainTx,
				EventType:      models.PaymentEventManagedEscrowReceived,
				ToAddress:      "0xmanagedescrow",
				Amount:         "600",
				Status:         models.PaymentObservationStatusConfirmed,
				ObservedAt:     timestamppb.New(observedAt.Add(time.Minute)),
			},
		},
	}
	session, err := p.Project(&projectOrderInput{
		order:       order,
		paymentSent: paymentSent,
		orderOpen: &pb.OrderOpen{
			Amount:      "1000",
			Timestamp:   timestamppb.New(time.Now()),
			PricingCoin: "USD",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "0.000000000000001", session.PaymentProgress.ObservedAmount)
	require.Equal(t, 2, session.PaymentProgress.ObservationCount)
	require.Len(t, session.PaymentProgress.Observations, 2)
	require.Equal(t, "0xtx-1", session.PaymentProgress.Observations[0].TxHash)
	require.Equal(t, "0.0000000000000004", session.PaymentProgress.Observations[0].Amount)
	require.Equal(t, "0xtx-2", session.PaymentProgress.Observations[1].TxHash)
	require.Equal(t, "0.0000000000000006", session.PaymentProgress.Observations[1].Amount)
}

func TestQueryObservationProgress_IsTenantScoped(t *testing.T) {
	db := newVerifierTestDB(t)
	p := NewPaymentSessionProjector(db)

	firstSeen := time.Date(2026, 5, 30, 9, 0, 0, 0, time.UTC)
	otherSeen := firstSeen.Add(time.Minute)
	rows := []models.PaymentObservation{
		testSessionObservation("obs-tenant-a", "tenant-a", "order-shared", "100", firstSeen, models.PaymentObservationStatusPending),
		testSessionObservation("obs-tenant-b", "tenant-b", "order-shared", "900", otherSeen, models.PaymentObservationStatusPending),
		testSessionObservation("obs-tenant-a-reverted", "tenant-a", "order-shared", "500", otherSeen, models.PaymentObservationStatusReverted),
	}
	require.NoError(t, db.gormDB.Create(&rows).Error)

	total, count, lastSeen, observations, err := p.queryObservationProgress("tenant-a", "order-shared")
	require.NoError(t, err)
	require.Equal(t, "100", total)
	require.Equal(t, 1, count)
	require.Len(t, observations, 1)
	require.Equal(t, "obs-tenant-a", observations[0].ID)
	require.NotNil(t, lastSeen)
	require.True(t, firstSeen.Equal(*lastSeen))
}

func TestQueryObservationProgress_RejectsMissingTenant(t *testing.T) {
	p := NewPaymentSessionProjector(newVerifierTestDB(t))

	_, _, _, _, err := p.queryObservationProgress("", "order-1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "tenantID and orderID must be set")
}

func testSessionObservation(id, tenantID, orderID, amount string, blockTime time.Time, status string) models.PaymentObservation {
	return models.PaymentObservation{
		TenantID:       tenantID,
		ID:             id,
		OrderID:        orderID,
		ChainNamespace: "eip155",
		ChainReference: "11155111",
		TxHash:         id + "-tx",
		EventIndex:     0,
		EventType:      models.PaymentEventManagedEscrowReceived,
		FromAddress:    "0xfeedfeedfeedfeedfeedfeedfeedfeedfeedfeed",
		ToAddress:      "0x111122223333444455556666777788889999aaaa",
		Amount:         amount,
		BlockNumber:    123,
		BlockTime:      blockTime,
		Confirmations:  0,
		Source:         models.PaymentObservationSourceMonitor,
		Observer:       "monitor:" + id,
		Status:         status,
	}
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
