package payment

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/mobazha/mobazha3.0/internal/config"
	orderutils "github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/internal/repo"
	walletpkg "github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"
)

// ── test helpers ────────────────────────────────────────────────────────

// testChainEscrow implements payment.ChainEscrow for unit testing.
type testChainEscrow struct {
	model        payment.PaymentModel
	genResult    *payment.PaymentSetupResult
	genErr       error
	genCallCount int
}

type testManagedEscrowStrategy struct {
	payment.ChainEscrowV2
	policy payment.ManagedEscrowFeePolicy
}

func (s testManagedEscrowStrategy) ManagedEscrowFeePolicy() payment.ManagedEscrowFeePolicy {
	return s.policy
}

func (s *testChainEscrow) Model() payment.PaymentModel { return s.model }
func (s *testChainEscrow) Capabilities() payment.ChainCapabilities {
	return payment.ChainCapabilities{}
}
func (s *testChainEscrow) AutoConfirm(_ context.Context, _ *events.CancelablePaymentReady) error {
	return nil
}
func (s *testChainEscrow) SignEscrowRelease(_ context.Context, _ payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	return nil, nil
}
func (s *testChainEscrow) EstimateEscrowFee(_ string, _, _ int, _ iwallet.FeeLevel) (iwallet.Amount, error) {
	return iwallet.NewAmount(0), nil
}
func (s *testChainEscrow) GeneratePaymentInstructions(_ context.Context, _ payment.PaymentSetupParams) (*payment.PaymentSetupResult, error) {
	s.genCallCount++
	return s.genResult, s.genErr
}
func (s *testChainEscrow) VerifyDeposit(_ context.Context, _ payment.DepositVerifyParams) error {
	return nil
}
func (s *testChainEscrow) ValidatePaymentMessage(_ payment.PaymentMessageParams) error {
	return nil
}
func (s *testChainEscrow) VerifyPreRelease(_ context.Context, _ payment.PreReleaseParams) error {
	return nil
}
func (s *testChainEscrow) GetConfirmInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{}, nil
}
func (s *testChainEscrow) GetCancelInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{}, nil
}
func (s *testChainEscrow) GetCompleteInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{}, nil
}
func (s *testChainEscrow) GetDisputeReleaseInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{}, nil
}

// newTestPaymentAppService creates a PaymentAppService with an in-memory DB
// suitable for unit testing. Only the DB and fields explicitly set via opts
// are populated; optional callbacks default to nil.
func newTestPaymentAppService(t *testing.T, cfg PaymentAppServiceConfig) *PaymentAppService {
	t.Helper()
	if cfg.DB == nil {
		db, err := repo.MockDB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		cfg.DB = db
	}
	if cfg.EventBus == nil {
		cfg.EventBus = events.NewBus()
	}
	if cfg.Shutdown == nil {
		ch := make(chan struct{})
		cfg.Shutdown = ch
	}
	if cfg.NodeID == "" {
		cfg.NodeID = "test-payment-svc"
	}
	return NewPaymentAppService(cfg)
}

// ── Constructor & Registry ──────────────────────────────────────────────

func TestPaymentAppService_NewPaymentAppService(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})
	assert.NotNil(t, svc)
	assert.Equal(t, "test-payment-svc", svc.nodeID)
}

func TestPaymentAppService_Registry_GetSet(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	assert.Nil(t, svc.Registry())

	reg := payment.NewRegistry()
	svc.SetRegistry(reg)
	assert.Same(t, reg, svc.Registry())
}

func TestGetUTXOEscrowKeys_UsesOrderBuyerPubkey(t *testing.T) {
	buyerKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)
	vendorKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)
	chaincode := []byte("0123456789abcdef0123456789abcdef")

	rawOpen, err := (protojson.MarshalOptions{}).Marshal(&pb.OrderOpen{
		BuyerID: &pb.ID{Pubkeys: &pb.ID_Pubkeys{
			Escrow: buyerKey.PubKey().SerializeCompressed(),
		}},
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				VendorID: &pb.ID{Pubkeys: &pb.ID_Pubkeys{
					Escrow: vendorKey.PubKey().SerializeCompressed(),
				}},
			},
		}},
		Chaincode: hex.EncodeToString(chaincode),
	})
	require.NoError(t, err)

	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{
		// Simulate provisioning from a seller-scoped service. The derived buyer
		// key must still come from the signed order open, not this local key.
		EscrowMasterPubKey: vendorKey.PubKey(),
	})
	keys, err := svc.GetUTXOEscrowKeys(context.Background(), &models.Order{
		ID:                  models.OrderID("order-utxo-buyer-key"),
		SerializedOrderOpen: rawOpen,
	}, "")
	require.NoError(t, err)

	expectedBuyer, err := orderutils.GenerateEscrowPublicKey(buyerKey.PubKey(), chaincode)
	require.NoError(t, err)
	expectedVendor, err := orderutils.GenerateEscrowPublicKey(vendorKey.PubKey(), chaincode)
	require.NoError(t, err)

	require.Equal(t, expectedBuyer.SerializeCompressed(), keys.BuyerKey.SerializeCompressed())
	require.Equal(t, expectedVendor.SerializeCompressed(), keys.VendorKey.SerializeCompressed())
	require.NotEqual(t, keys.BuyerKey.SerializeCompressed(), keys.VendorKey.SerializeCompressed())
}

type stubFiatPaymentQuery struct{}

func (*stubFiatPaymentQuery) GetPayment(_ context.Context, _ string, _ string) (*contracts.PaymentDetail, error) {
	return nil, nil
}

func TestPaymentAppService_SetFiatPaymentQuery_PropagatesToExistingVerificationService(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})
	pvs := NewPaymentVerificationService(nil, nil, nil)

	svc.SetVerificationService(pvs)

	fq := &stubFiatPaymentQuery{}
	svc.SetFiatPaymentQuery(fq)

	assert.Same(t, fq, svc.fiatPaymentQuery)
	assert.Same(t, fq, pvs.fiatPayment)
}

func TestPaymentAppService_SetVerificationService_BackfillsStoredFiatPaymentQuery(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	fq := &stubFiatPaymentQuery{}
	svc.SetFiatPaymentQuery(fq)

	pvs := NewPaymentVerificationService(nil, nil, nil)
	svc.SetVerificationService(pvs)

	assert.Same(t, fq, svc.fiatPaymentQuery)
	assert.Same(t, fq, pvs.fiatPayment)
}

// ── FetchOrderByID ──────────────────────────────────────────────────────

func TestPaymentAppService_FetchOrderByID_NotFound(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	_, err := svc.FetchOrderByID("nonexistent-order")
	assert.Error(t, err, "should return error for nonexistent order")
}

func TestPaymentAppService_FetchOrderByID_Found(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	order := &models.Order{ID: models.OrderID("test-order-123")}
	err := svc.db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	})
	require.NoError(t, err)

	got, err := svc.FetchOrderByID("test-order-123")
	require.NoError(t, err)
	assert.Equal(t, models.OrderID("test-order-123"), got.ID)
}

// ── GeneratePaymentInstructions ─────────────────────────────────────────

func TestPaymentAppService_GeneratePaymentInstructions_Success(t *testing.T) {
	reg := payment.NewRegistry()
	managed_escrowAddr := "0x111122223333444455556666777788889999aaaa"
	expectedResult := &payment.PaymentSetupResult{
		PaymentModel: payment.PaymentModelMonitored,
		EscrowAddr:   managed_escrowAddr,
	}
	strategy := &testChainEscrow{
		model:     payment.PaymentModelMonitored,
		genResult: expectedResult,
	}
	reg.RegisterV2(iwallet.ChainEthereum, payment.NewV1AsV2(strategy))

	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{
		PaymentRegistry: reg,
	})

	result, err := svc.GeneratePaymentInstructions(context.Background(), models.InitializeEscrowData{
		OrderID:  "order-1",
		CoinType: iwallet.CoinType("crypto:eip155:1:native"),
		Amount:   1000000,
	})
	require.NoError(t, err)
	assert.Equal(t, payment.PaymentModelMonitored, result.PaymentModel)
	assert.Equal(t, managed_escrowAddr, result.EscrowAddr)
	assert.Equal(t, 1, strategy.genCallCount)
}

func TestPaymentAppService_GeneratePaymentSetup_PersistsPolicySnapshot(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	rawProvider, ok := db.(interface{ RawDB() *gorm.DB })
	require.True(t, ok)
	raw := rawProvider.RawDB()
	require.NotNil(t, raw)

	reg := payment.NewRegistry()
	strategy := &testChainEscrow{
		model: payment.PaymentModelMonitored,
		genResult: &payment.PaymentSetupResult{
			PaymentModel: payment.PaymentModelMonitored,
			EscrowAddr:   "0x111122223333444455556666777788889999aaaa",
			PaymentData: &models.PaymentData{
				OrderID: "order-policy-setup",
			},
		},
	}
	reg.RegisterV2(iwallet.ChainEthereum, payment.NewV1AsV2(strategy))

	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{
		DB:              db,
		PaymentRegistry: reg,
	})
	_, err = svc.GeneratePaymentSetup(context.Background(), payment.PaymentSetupParams{
		OrderID:             "order-policy-setup",
		CoinType:            iwallet.CoinType("crypto:eip155:1:native"),
		Moderator:           "mod-peer",
		StorePolicyRevision: 42,
	})
	require.NoError(t, err)

	var shared models.SharedPaymentIntent
	require.NoError(t, raw.Where("order_id = ?", "order-policy-setup").First(&shared).Error)
	require.Equal(t, "mod-peer", shared.ModeratorPeerID)
	require.Equal(t, uint64(42), shared.StorePolicyRevision)
}

func TestPaymentAppService_GeneratePaymentSetup_AuthorizesBeforeStrategy(t *testing.T) {
	reg := payment.NewRegistry()
	strategy := &testChainEscrow{
		model: payment.PaymentModelMonitored,
		genResult: &payment.PaymentSetupResult{
			PaymentModel: payment.PaymentModelMonitored,
			EscrowAddr:   "0x111122223333444455556666777788889999aaaa",
		},
	}
	reg.RegisterV2(iwallet.ChainEthereum, payment.NewV1AsV2(strategy))

	orderOpen := managedCollectibleFirstSaleOrderOpen()
	rawOpen, err := (protojson.MarshalOptions{}).Marshal(orderOpen)
	require.NoError(t, err)
	expiresAt := time.Now().Add(time.Hour).UTC()
	order := &models.Order{
		ID:                  models.OrderID("order-source-policy"),
		SerializedOrderOpen: rawOpen,
		OrderTimeoutState: models.OrderTimeoutState{
			ExpiresAt: &expiresAt,
		},
	}
	wantErr := errors.New("source already reserved")
	called := false
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{PaymentRegistry: reg})
	svc.AddProvisioningPolicy(NewCollectibleFirstSaleProvisioningPolicy(func(_ context.Context, signal CollectibleFirstSaleAuthorizationSignal) error {
		called = true
		require.Equal(t, order.ID.String(), signal.OrderID)
		require.Equal(t, "crypto:eip155:1:native", signal.PaymentCoin)
		require.Equal(t, expiresAt, signal.ReservationExpiresAt)
		return wantErr
	}))

	_, err = svc.GeneratePaymentSetup(context.Background(), payment.PaymentSetupParams{
		OrderID:   order.ID.String(),
		CoinType:  iwallet.CoinType("crypto:eip155:1:native"),
		OrderData: order,
	})
	require.ErrorIs(t, err, ErrCollectibleFirstSalePreflight)
	require.ErrorIs(t, err, wantErr)
	require.True(t, called)
	require.Zero(t, strategy.genCallCount, "strategy must not create a funding target before authorization")
}

func TestPaymentAppService_GeneratePaymentSetup_RejectsLegacyEVMFundingHash(t *testing.T) {
	reg := payment.NewRegistry()
	legacyHash := "0xdfac9fe89ed092e0b27e5bf1a71639758d799a6cd301476e78475165e7a2b5ae"
	strategy := &testChainEscrow{
		model: payment.PaymentModelMonitored,
		genResult: &payment.PaymentSetupResult{
			PaymentModel: payment.PaymentModelMonitored,
			EscrowAddr:   legacyHash,
		},
	}
	reg.RegisterV2(iwallet.ChainEthereum, payment.NewV1AsV2(strategy))

	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{
		PaymentRegistry: reg,
	})

	_, err := svc.GeneratePaymentSetup(context.Background(), payment.PaymentSetupParams{
		OrderID:  "order-legacy-hash",
		CoinType: iwallet.CoinType("crypto:eip155:1:native"),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidEVMFundingAddress)
}

func TestPaymentAppService_GeneratePaymentSetup_RejectsLegacyEVMModel(t *testing.T) {
	reg := payment.NewRegistry()
	strategy := &testChainEscrow{
		model: payment.PaymentModelClientSigned,
		genResult: &payment.PaymentSetupResult{
			PaymentModel: payment.PaymentModelClientSigned,
			EscrowAddr:   "0x111122223333444455556666777788889999aaaa",
		},
	}
	reg.RegisterV2(iwallet.ChainEthereum, payment.NewV1AsV2(strategy))

	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{
		PaymentRegistry: reg,
	})

	_, err := svc.GeneratePaymentSetup(context.Background(), payment.PaymentSetupParams{
		OrderID:  "order-legacy-model",
		CoinType: iwallet.CoinType("crypto:eip155:1:native"),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLegacyEVMPaymentRetired)
}

func TestPaymentAppService_GeneratePaymentSetup_FailsWhenEVMStrategyNotRegistered(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{
		PaymentRegistry: payment.NewRegistry(),
	})

	_, err := svc.GeneratePaymentSetup(context.Background(), payment.PaymentSetupParams{
		OrderID:  "order-no-managed-route",
		CoinType: iwallet.CoinType("crypto:eip155:1:native"),
	})
	require.Error(t, err)
}

func TestPaymentAppService_GeneratePaymentSetup_RejectsRetiredTRONBeforeRegistryLookup(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{
		PaymentRegistry: payment.NewRegistry(),
	})

	_, err := svc.GeneratePaymentSetup(context.Background(), payment.PaymentSetupParams{
		OrderID:  "order-tron-retired",
		CoinType: iwallet.CoinType("crypto:tron:mainnet:native"),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTRONPaymentRetired)
}

func TestPaymentAppService_PersistManagedEscrowPaymentAddress_UpdatesAllTenantRows(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	rawProvider, ok := db.(interface{ RawDB() *gorm.DB })
	require.True(t, ok)
	raw := rawProvider.RawDB()
	require.NotNil(t, raw)

	require.NoError(t, raw.Create(&models.Order{
		TenantMixin: models.TenantMixin{TenantID: "tenant-buyer"},
		ID:          models.OrderID("order-safe"),
	}).Error)
	require.NoError(t, raw.Create(&models.Order{
		TenantMixin: models.TenantMixin{TenantID: "tenant-vendor"},
		ID:          models.OrderID("order-safe"),
	}).Error)

	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{DB: db})
	require.NoError(t, svc.persistManagedEscrowPaymentAddress(
		"order-safe",
		"crypto:eip155:11155111:native",
		"0xmanagedescrow",
		1000,
		"0x1111111111111111111111111111111111111111",
		false,
		"",
		"",
		"0",
		"",
		"0",
	))

	var orders []models.Order
	require.NoError(t, raw.
		Where("id = ?", "order-safe").
		Order("tenant_id ASC").
		Find(&orders).Error)
	require.Len(t, orders, 2)
	for i := range orders {
		require.Equal(t, "0xmanagedescrow", orders[i].PaymentAddress)
		require.Equal(t, "0x1111111111111111111111111111111111111111", orders[i].RefundAddress)
		info, err := orders[i].GetPendingManagedEscrowPaymentInfo()
		require.NoError(t, err)
		require.NotNil(t, info)
		require.Equal(t, uint64(1000), info.Amount)
		require.Equal(t, "crypto:eip155:11155111:native", info.Coin)
		require.Equal(t, "0xmanagedescrow", info.Address)
		require.Equal(t, "0", orders[i].CancelFeeAmount)
	}

	var shared models.SharedPaymentIntent
	require.NoError(t, raw.Where("order_id = ?", "order-safe").First(&shared).Error)
	require.Equal(t, "0xmanagedescrow", shared.PaymentAddress)
	require.Equal(t, "0x1111111111111111111111111111111111111111", shared.RefundAddress)
	info, err := shared.GetPendingManagedEscrowPaymentInfo()
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, uint64(1000), info.Amount)
	require.Equal(t, "crypto:eip155:11155111:native", info.Coin)
}

func TestPaymentAppService_PersistEscrowPaymentInfo_PreservesModeratorAddress(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	paymentData := &models.PaymentData{
		OrderID:          "order-solana-moderated",
		Coin:             iwallet.CoinType("crypto:solana:mainnet:native"),
		Amount:           1000,
		ContractAddress:  "AnD79RcbbS1GsvNZZHcQTGRvozVL1J9mr4GJiwm587pX",
		ToAddress:        "RT38nT6ABNLfotNxwseiNNKukCKAXpFkZctJGn4EbFe",
		Moderator:        "moderator-peer-id",
		ModeratorAddress: "Mod11111111111111111111111111111111111111111",
		UnlockTime:       12345,
		FundingDeadline:  12000,
		SettlementSpec:   payment.NewSolanaEscrowSpec(true).ToPending(),
	}
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{DB: db})
	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		return tx.Save(&models.Order{ID: models.OrderID(paymentData.OrderID)})
	}))
	require.NoError(t, svc.persistEscrowPaymentInfo(paymentData.OrderID, paymentData))

	var order models.Order
	require.NoError(t, svc.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", paymentData.OrderID).First(&order).Error
	}))
	info, err := order.GetPendingEscrowPaymentInfo()
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, paymentData.ModeratorAddress, info.ModeratorAddress)

	encoded, err := encodeSolanaAnchorPendingMetadata(paymentData)
	require.NoError(t, err)
	rawMetadata, err := hex.DecodeString(encoded)
	require.NoError(t, err)
	var metadata models.PendingEscrowPaymentInfo
	require.NoError(t, json.Unmarshal(rawMetadata, &metadata))
	require.Equal(t, paymentData.ModeratorAddress, metadata.ModeratorAddress)
}

func TestPaymentAppService_PersistSharedPaymentPolicySnapshot(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	rawProvider, ok := db.(interface{ RawDB() *gorm.DB })
	require.True(t, ok)
	raw := rawProvider.RawDB()
	require.NotNil(t, raw)

	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{DB: db})
	require.NoError(t, svc.persistSharedPaymentPolicySnapshot("order-policy", "mod-peer", 42))

	var shared models.SharedPaymentIntent
	require.NoError(t, raw.Where("order_id = ?", "order-policy").First(&shared).Error)
	require.Equal(t, "mod-peer", shared.ModeratorPeerID)
	require.Equal(t, uint64(42), shared.StorePolicyRevision)
}

func TestPaymentAppService_GeneratePaymentInstructions_LocksManagedEscrowReleaseFees(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	rawProvider, ok := db.(interface{ RawDB() *gorm.DB })
	require.True(t, ok)
	raw := rawProvider.RawDB()
	require.NotNil(t, raw)
	require.NoError(t, raw.Create(&models.Order{
		TenantMixin: models.TenantMixin{TenantID: "tenant-safe"},
		ID:          models.OrderID("order-managed_escrow-fee"),
	}).Error)

	const (
		platformAddr = "0x7777777777777777777777777777777777777777"
		feeWei       = "75000000000000" // $0.15 at $2,000/ETH
	)
	reg := payment.NewRegistry()
	strategy := &testChainEscrow{
		model: payment.PaymentModelMonitored,
		genResult: &payment.PaymentSetupResult{
			PaymentModel: payment.PaymentModelMonitored,
			PaymentData: &models.PaymentData{
				OrderID:   "order-managed_escrow-fee",
				Coin:      iwallet.CoinType("crypto:eip155:1:native"),
				Method:    pb.PaymentSent_CANCELABLE,
				Amount:    1_000_000_000_000_000_000,
				ToAddress: "0x1111111111111111111111111111111111111111",
			},
		},
	}
	reg.RegisterV2(iwallet.ChainEthereum, testManagedEscrowStrategy{
		ChainEscrowV2: payment.NewV1AsV2(strategy),
		policy: payment.ManagedEscrowFeePolicy{
			ReleaseFeeUSDCents: 15,
			ChargeCancellation: true,
		},
	})

	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{
		DB:              db,
		PaymentRegistry: reg,
		ExchangeRates: walletpkg.NewFixedRateProvider("ETH", map[models.CurrencyCode]iwallet.Amount{
			"USD": iwallet.NewAmount(200000),
		}),
		NetConfig: &config.NetConfig{
			PlatformAddrs: map[iwallet.ChainType]string{iwallet.ChainEthereum: platformAddr},
		},
	})

	result, err := svc.GeneratePaymentInstructions(context.Background(), models.InitializeEscrowData{
		OrderID:  "order-managed_escrow-fee",
		CoinType: iwallet.CoinType("crypto:eip155:1:native"),
		Amount:   1_000_000_000_000_000_000,
	})
	require.NoError(t, err)
	require.NotNil(t, result.PaymentData)
	require.Equal(t, feeWei, result.PaymentData.PlatformAmount)
	require.Equal(t, platformAddr, result.PaymentData.PlatformAddr)
	require.Equal(t, feeWei, result.PaymentData.CancelFeeAmount)

	var order models.Order
	require.NoError(t, raw.Where("id = ?", "order-managed_escrow-fee").First(&order).Error)
	require.Equal(t, feeWei, order.CancelFeeAmount)
	info, err := order.GetPendingManagedEscrowPaymentInfo()
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, feeWei, info.PlatformAmount)
	require.Equal(t, platformAddr, info.PlatformAddr)
	require.Equal(t, feeWei, info.CancelFeeAmount)
}

func TestPaymentAppService_GeneratePaymentInstructions_NoCoinStrategy(t *testing.T) {
	reg := payment.NewRegistry()
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{
		PaymentRegistry: reg,
	})

	_, err := svc.GeneratePaymentInstructions(context.Background(), models.InitializeEscrowData{
		CoinType: iwallet.CoinType("NONEXISTENT"),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no chain escrow")
}

func TestPaymentAppService_GeneratePaymentInstructions_StrategyError(t *testing.T) {
	reg := payment.NewRegistry()
	strategy := &testChainEscrow{
		model:  payment.PaymentModelMonitored,
		genErr: errors.New("escrow generation failed"),
	}
	reg.Register(iwallet.ChainBitcoin, strategy)

	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{
		PaymentRegistry: reg,
	})

	_, err := svc.GeneratePaymentInstructions(context.Background(), models.InitializeEscrowData{
		CoinType: iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native"),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "escrow generation failed")
}

func TestPaymentAppService_GeneratePaymentInstructions_MultipleChains(t *testing.T) {
	reg := payment.NewRegistry()

	utxoStrategy := &testChainEscrow{
		model:     payment.PaymentModelMonitored,
		genResult: &payment.PaymentSetupResult{PaymentModel: payment.PaymentModelMonitored},
	}
	evmStrategy := &testChainEscrow{
		model: payment.PaymentModelMonitored,
		genResult: &payment.PaymentSetupResult{
			PaymentModel: payment.PaymentModelMonitored,
			EscrowAddr:   "0x111122223333444455556666777788889999aaaa",
		},
	}

	reg.RegisterV2(iwallet.ChainBitcoin, payment.NewV1AsV2(utxoStrategy))
	reg.RegisterV2(iwallet.ChainEthereum, payment.NewV1AsV2(evmStrategy))

	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{
		PaymentRegistry: reg,
	})

	tests := []struct {
		name     string
		coin     iwallet.CoinType
		expected payment.PaymentModel
	}{
		{"BTC dispatches to UTXO strategy", iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native"), payment.PaymentModelMonitored},
		{"ETH dispatches to ManagedEscrow-monitored strategy", iwallet.CoinType("crypto:eip155:1:native"), payment.PaymentModelMonitored},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.GeneratePaymentInstructions(context.Background(), models.InitializeEscrowData{
				CoinType: tt.coin,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result.PaymentModel)
		})
	}
}

// IsEVMRelayAvailable and TryLockAutoConfirm tests live in
// internal/core/settlement/ as they test SettlementService methods.

// ReceivingAccount CRUD tests have been migrated to
// internal/core/receiving_account_service_test.go (OP-1.3)

func TestPaymentAppService_ReceivingAccount_MigratedPlaceholder(t *testing.T) {
	t.Skip("ReceivingAccount + GetAcceptedCurrencies tests migrated to receiving_account_service_test.go")
}

// ── TransactionMetadata ─────────────────────────────────────────────────

func TestPaymentAppService_TransactionMetadata_SaveAndGet(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	meta := &models.TransactionMetadata{
		Txid:    "txid-abc-123",
		OrderID: models.OrderID("order-456"),
	}
	err := svc.SaveTransactionMetadata(meta)
	require.NoError(t, err)

	got, err := svc.GetTransactionMetadata("txid-abc-123")
	require.NoError(t, err)
	assert.Equal(t, models.OrderID("order-456"), got.OrderID)
}

func TestPaymentAppService_TransactionMetadata_NotFound(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	_, err := svc.GetTransactionMetadata("nonexistent-tx")
	assert.Error(t, err)
}
