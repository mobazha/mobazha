//go:build !private_distribution

package payment

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
)

// ─────────────────────────────────────────────────────────────────────────
// Minimal test infrastructure for the verifier
// ─────────────────────────────────────────────────────────────────────────
//
// The verifier reaches through database.Database / database.Tx into the
// underlying *gorm.DB to issue SELECT FOR UPDATE and direct queries
// against payment_observations, so we can't reuse the repo fakes from
// observation_dispatcher_test.go. Instead we wire a real in-memory
// SQLite that AutoMigrates both Order and PaymentObservation, wrap it
// in the smallest database.Database / database.Tx surface that satisfies
// the verifier's call paths, and let GORM do the heavy lifting. This
// gives us realistic dialect detection ("sqlite" → no FOR UPDATE) and
// actual transactional semantics for the FSM-state guards we need to
// exercise.

type vTestDB struct {
	gormDB *gorm.DB
}

func newVerifierTestDB(t *testing.T) *vTestDB {
	t.Helper()
	db, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Order{}, &models.PaymentObservation{}, &models.SharedPaymentIntent{}))
	return &vTestDB{gormDB: db}
}

func (d *vTestDB) View(fn func(database.Tx) error) error {
	return fn(&vTestTx{db: d.gormDB})
}

func (d *vTestDB) Update(fn func(database.Tx) error) error {
	return d.gormDB.Transaction(func(tx *gorm.DB) error {
		return fn(&vTestTx{db: tx})
	})
}

func (d *vTestDB) ComputePublicDataHash() (cid.Cid, error) { return cid.Undef, nil }
func (d *vTestDB) Close() error                            { return nil }

type vTestTx struct{ db *gorm.DB }

func (t *vTestTx) Read() *gorm.DB           { return t.db }
func (t *vTestTx) Save(i interface{}) error { return t.db.Save(i).Error }
func (t *vTestTx) Update(key string, value interface{}, where map[string]interface{}, model interface{}) error {
	q := t.db.Model(model)
	for k, v := range where {
		q = q.Where(k, v)
	}
	return q.UpdateColumn(key, value).Error
}
func (t *vTestTx) UpdateColumns(values map[string]interface{}, where map[string]interface{}, model interface{}) (int64, error) {
	q := t.db.Model(model)
	for k, v := range where {
		q = q.Where(k, v)
	}
	res := q.UpdateColumns(values)
	return res.RowsAffected, res.Error
}
func (t *vTestTx) Commit() error   { panic("managed tx") }
func (t *vTestTx) Rollback() error { panic("managed tx") }
func (t *vTestTx) Delete(key string, value interface{}, where map[string]interface{}, model interface{}) error {
	q := t.db.Where(key, value)
	for k, v := range where {
		q = q.Where(k, v)
	}
	return q.Delete(model).Error
}
func (t *vTestTx) DeleteAll(interface{}) error { return nil }
func (t *vTestTx) Migrate(interface{}) error   { return nil }
func (t *vTestTx) RegisterCommitHook(func())   {}

// PublicData stubs — the verifier never touches PublicData but the
// database.Tx interface requires them.
func (t *vTestTx) GetProfile() (*models.Profile, error)                       { return nil, nil }
func (t *vTestTx) SetProfile(*models.Profile) error                           { return nil }
func (t *vTestTx) GetFollowers() (models.Followers, error)                    { return models.Followers{}, nil }
func (t *vTestTx) SetFollowers(models.Followers) error                        { return nil }
func (t *vTestTx) GetFollowing() (models.Following, error)                    { return models.Following{}, nil }
func (t *vTestTx) SetFollowing(models.Following) error                        { return nil }
func (t *vTestTx) GetListing(string) (*pb.SignedListing, error)               { return nil, nil }
func (t *vTestTx) SetListing(*pb.SignedListing) error                         { return nil }
func (t *vTestTx) GetEncryptedListing(string) ([]byte, error)                 { return nil, nil }
func (t *vTestTx) SetEncryptedListing(string, []byte) error                   { return nil }
func (t *vTestTx) DeleteListing(string) error                                 { return nil }
func (t *vTestTx) GetListingIndex() (models.ListingIndex, error)              { return nil, nil }
func (t *vTestTx) SetListingIndex(models.ListingIndex) error                  { return nil }
func (t *vTestTx) GetRatingIndex() (models.RatingIndex, error)                { return nil, nil }
func (t *vTestTx) SetRatingIndex(models.RatingIndex) error                    { return nil }
func (t *vTestTx) SetRating(*pb.Rating) error                                 { return nil }
func (t *vTestTx) GetPostIndex() ([]models.PostData, error)                   { return nil, nil }
func (t *vTestTx) SetPostIndex([]models.PostData) error                       { return nil }
func (t *vTestTx) AddPost(*postsPb.SignedPost) error                          { return nil }
func (t *vTestTx) DeletePost(string) error                                    { return nil }
func (t *vTestTx) PostExist(string) bool                                      { return false }
func (t *vTestTx) GetPost(string) (*postsPb.SignedPost, error)                { return nil, nil }
func (t *vTestTx) SetImage(models.Image) error                                { return nil }
func (t *vTestTx) GetImageByName(models.ImageSize, string) ([]byte, error)    { return nil, nil }
func (t *vTestTx) GetMediaByCID(string) ([]byte, string, error)               { return nil, "", nil }
func (t *vTestTx) IndexMediaCID(string, string, string, string, string) error { return nil }
func (t *vTestTx) SetUploadedFile(models.UploadedFile) error                  { return nil }
func (t *vTestTx) SetIntroVideo(models.IntroVideo) error                      { return nil }

// recordingBus captures every emitted event for assertion. The verifier
// only ever emits events.PaymentVerified so we keep the buffer
// deliberately untyped.
type recordingBus struct {
	emitted []interface{}
}

type verifiedHandlerCall struct {
	orderID     string
	paymentSent *pb.PaymentSent
}

const testUSDCAsset = "crypto:eip155:1:erc20:0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"

func (b *recordingBus) Subscribe(_ interface{}, _ ...events.SubscriptionOpt) (events.Subscription, error) {
	return nil, nil
}
func (b *recordingBus) Emit(evt interface{}) { b.emitted = append(b.emitted, evt) }

// seedOrder writes a minimal-but-valid Order row whose OrderOpen carries
// the supplied expected amount in smallest units. The order starts in
// "pending verification" state to mirror what the dispatcher would
// observe in production.
//
// Always seeded under database.StandaloneTenantID; cross-tenant tests
// use seedOrderForTenant to drop into a different tenant slice.
func seedOrder(t *testing.T, db *vTestDB, orderID, expectedAmount, refundAddress string) {
	t.Helper()
	seedOrderForTenant(t, db, database.StandaloneTenantID, orderID, expectedAmount, refundAddress)
}

// seedOrderForTenant is the multi-tenant flavour of seedOrder. It exists
// so the cross-tenant isolation test can plant the same OrderID under
// two distinct tenant slices and verify the verifier never crosses the
// (tenant_id, id) primary-key boundary.
func seedOrderForTenant(t *testing.T, db *vTestDB, tenantID, orderID, expectedAmount, refundAddress string) {
	t.Helper()
	oo := &pb.OrderOpen{
		Amount:      expectedAmount,
		PricingCoin: "USDC",
		Chaincode:   "11223344aabbccdd",
		BuyerID:     &pb.ID{PeerID: "buyer-peer"},
	}
	raw, err := protojson.Marshal(oo)
	require.NoError(t, err)

	order := &models.Order{
		TenantMixin:         models.TenantMixin{TenantID: tenantID},
		ID:                  models.OrderID(orderID),
		MyRole:              string(models.RoleVendor),
		SerializedOrderOpen: raw,
		RefundAddress:       refundAddress,
	}
	order.MarkPaymentVerificationPending()

	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))
}

func seedPendingUTXOInfo(t *testing.T, db *vTestDB, orderID, confirmationPolicy string) {
	t.Helper()
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().
			Where("tenant_id = ? AND id = ?", database.StandaloneTenantID, orderID).
			First(&order).Error; err != nil {
			return err
		}
		order.PaymentAddress = "bch-escrow"
		if err := order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
			Coin:               "crypto:bitcoincash:mainnet:native",
			Amount:             1000,
			Script:             "ab",
			ConfirmationPolicy: confirmationPolicy,
			SettlementSpec: &models.PendingSettlementSpec{
				Method:     "CANCELABLE",
				PayMode:    "address_monitored",
				EscrowType: "utxo",
			},
		}); err != nil {
			return err
		}
		return tx.Save(&order)
	}))
}

func seedOrderForRoleWithListing(t *testing.T, db *vTestDB, orderID, expectedAmount string, role models.OrderRole) {
	t.Helper()
	oo := &pb.OrderOpen{
		Amount:      expectedAmount,
		PricingCoin: "USDC",
		Chaincode:   "11223344aabbccdd",
		BuyerID:     &pb.ID{PeerID: "buyer-peer", Handle: "buyer"},
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				Slug: "deterministic-payment-listing",
				Metadata: &pb.Listing_Metadata{
					ContractType: pb.Listing_Metadata_PHYSICAL_GOOD,
				},
				Item: &pb.Listing_Item{
					Title:                      "Deterministic Payment Listing",
					CryptoListingPriceModifier: 0,
				},
			},
		}},
	}
	raw, err := protojson.Marshal(oo)
	require.NoError(t, err)

	order := &models.Order{
		TenantMixin:         models.TenantMixin{TenantID: database.StandaloneTenantID},
		ID:                  models.OrderID(orderID),
		MyRole:              string(role),
		SerializedOrderOpen: raw,
		RefundAddress:       "0xrefund",
	}
	order.MarkPaymentVerificationPending()

	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))
}

// loadOrderForTenant reads back a specific (tenant, id) row. Used by
// cross-tenant tests to assert that operations against tenant A never
// mutate tenant B's slice.
func loadOrderForTenant(t *testing.T, db *vTestDB, tenantID, orderID string) *models.Order {
	t.Helper()
	var order models.Order
	require.NoError(t, db.gormDB.
		Where("tenant_id = ? AND id = ?", tenantID, orderID).
		First(&order).Error)
	return &order
}

// insertObs writes an observation row directly via GORM. We bypass the
// repo so test setup stays self-contained.
func insertObs(t *testing.T, db *vTestDB, obs models.PaymentObservation) {
	t.Helper()
	if obs.TenantID == "" {
		obs.TenantID = database.StandaloneTenantID
	}
	if obs.Status == "" {
		obs.Status = models.PaymentObservationStatusConfirmed
	}
	if obs.BlockTime.IsZero() {
		obs.BlockTime = time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	}
	if obs.Source == "" {
		obs.Source = models.PaymentObservationSourceMonitor
	}
	if obs.Observer == "" {
		obs.Observer = "monitor:eip155-1:worker-A"
	}
	require.NoError(t, db.gormDB.Create(&obs).Error)
}

func loadOrder(t *testing.T, db *vTestDB, orderID string) *models.Order {
	t.Helper()
	var order models.Order
	require.NoError(t, db.gormDB.Where("id = ?", orderID).First(&order).Error)
	return &order
}

// ─────────────────────────────────────────────────────────────────────────
// Constructor
// ─────────────────────────────────────────────────────────────────────────

func TestNewAggregatingVerifier_NilDB_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on nil db")
		}
	}()
	NewAggregatingVerifier(nil, &recordingBus{})
}

func TestNewAggregatingVerifier_NilBus_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on nil bus")
		}
	}()
	NewAggregatingVerifier(newVerifierTestDB(t), nil)
}

// ─────────────────────────────────────────────────────────────────────────
// AggregateAndEmit — input validation
// ─────────────────────────────────────────────────────────────────────────

func TestAggregateAndEmit_BlankInputs_Reject(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	require.ErrorContains(t, v.AggregateAndEmit(context.Background(), "", "order-1"), "tenantID must be set")
	require.ErrorContains(t, v.AggregateAndEmit(context.Background(), "tenant", "  "), "orderID must be set")
	require.Empty(t, bus.emitted, "validation rejection must not emit events")
}

// ─────────────────────────────────────────────────────────────────────────
// AggregateAndEmit — log-and-skip cases
// ─────────────────────────────────────────────────────────────────────────

func TestAggregateAndEmit_UnknownOrder_NoOp(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "missing-order"))
	require.Empty(t, bus.emitted, "unknown order must be a silent no-op")
}

func TestAggregateAndEmit_NoObservations_StaysPending(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-1", "1000", "0xrefund")

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-1"))
	require.Empty(t, bus.emitted, "no observations means no verification, no event")

	got := loadOrder(t, db, "order-1")
	require.True(t, got.IsPaymentVerificationPending())
	require.Equal(t, "0", got.TotalReceived)
	require.Empty(t, got.OverpaidAmount)
}

func TestAggregateAndEmit_PendingUTXODefaultPolicyDoesNotVerify(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-1", "1000", "bch-refund")
	seedPendingUTXOInfo(t, db, "order-1", models.PaymentConfirmationPolicyChainConfirmed)
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-pending",
		OrderID:        "order-1",
		ChainNamespace: "bitcoincash",
		ChainReference: "mainnet",
		TxHash:         "bch-tx-pending",
		EventType:      models.PaymentEventUTXOFunding,
		FromAddress:    "bch-payer",
		ToAddress:      "bch-escrow",
		Amount:         "1000",
		Status:         models.PaymentObservationStatusPending,
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-1"))
	require.Empty(t, bus.emitted)

	got := loadOrder(t, db, "order-1")
	require.True(t, got.IsPaymentVerificationPending())
	require.Equal(t, "0", got.TotalReceived)
	require.Empty(t, got.SerializedPaymentSent)
}

func TestAggregateAndEmit_PendingUTXOMempoolAcceptedPolicyVerifies(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-1", "1000", "bch-refund")
	seedPendingUTXOInfo(t, db, "order-1", models.PaymentConfirmationPolicyMempoolAccepted)
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-pending",
		OrderID:        "order-1",
		ChainNamespace: "bitcoincash",
		ChainReference: "mainnet",
		TxHash:         "bch-tx-pending",
		EventType:      models.PaymentEventUTXOFunding,
		FromAddress:    "bch-payer",
		ToAddress:      "bch-escrow",
		Amount:         "1000",
		Status:         models.PaymentObservationStatusPending,
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-1"))
	require.Len(t, bus.emitted, 2)
	_, ok := bus.emitted[0].(events.PaymentVerified)
	require.True(t, ok, "first event remains the internal verification signal")
	ready, ok := bus.emitted[1].(*events.CancelablePaymentReady)
	require.True(t, ok, "UTXO cancelable payment should emit auto-confirm event")
	require.Equal(t, "order-1", ready.OrderID)
	require.Equal(t, uint64(1000), ready.Amount)

	got := loadOrder(t, db, "order-1")
	require.True(t, got.IsPaymentVerified())
	require.Equal(t, models.OrderState_PENDING, got.State)
	require.Equal(t, "1000", got.TotalReceived)
	require.NotEmpty(t, got.SerializedPaymentSent)

	ps, err := got.PaymentSentMessage()
	require.NoError(t, err)
	require.Equal(t, "bch-tx-pending", ps.TransactionID)
	require.Equal(t, "bch-escrow", ps.ToAddress)
	require.Equal(t, "crypto:bitcoincash:mainnet:native", ps.Coin)
	require.Equal(t, models.PaymentConfirmationPolicyMempoolAccepted, ps.ConfirmationPolicy)
	require.Len(t, ps.FundingFacts, 1)
	require.Equal(t, models.PaymentObservationStatusPending, ps.FundingFacts[0].Status)
}

// ─────────────────────────────────────────────────────────────────────────
// AggregateAndEmit — partial / exact / over flows
// ─────────────────────────────────────────────────────────────────────────

func TestAggregateAndEmit_Partial_TotalRecordedNoEmit(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-1", "1000", "0xrefund")
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-1",
		OrderID:        "order-1",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-partial",
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "300",
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-1"))
	require.Empty(t, bus.emitted)

	got := loadOrder(t, db, "order-1")
	require.True(t, got.IsPaymentVerificationPending())
	require.Equal(t, "300", got.TotalReceived)
	require.Empty(t, got.OverpaidAmount)
	require.Empty(t, got.SerializedPaymentSent, "partial path must not freeze envelope")
}

func TestAggregateAndEmit_ExactAmount_VerifiesAndEmits(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)
	frozen := time.Date(2026, 5, 14, 12, 30, 0, 0, time.UTC)
	v.SetClock(func() time.Time { return frozen })

	seedOrder(t, db, "order-1", "1000", "0xrefund")
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().
			Where("tenant_id = ? AND id = ?", database.StandaloneTenantID, "order-1").
			First(&order).Error; err != nil {
			return err
		}
		if err := order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
			Coin:    testUSDCAsset,
			Amount:  1000,
			Address: "0xmanagedescrow",
			SettlementSpec: &models.PendingSettlementSpec{
				Method:     "CANCELABLE",
				PayMode:    "address_monitored",
				EscrowType: "managed_escrow",
			},
		}); err != nil {
			return err
		}
		return tx.Save(&order)
	}))
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-1",
		OrderID:        "order-1",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-1",
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer-1",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "400",
	})
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-2",
		OrderID:        "order-1",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-2",
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer-2",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "600",
		BlockTime:      time.Date(2026, 5, 14, 12, 5, 0, 0, time.UTC),
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-1"))

	require.Len(t, bus.emitted, 2)
	verified, ok := bus.emitted[0].(events.PaymentVerified)
	require.True(t, ok, "expected PaymentVerified event")
	require.Equal(t, database.StandaloneTenantID, verified.TenantID)
	require.Equal(t, "order-1", verified.OrderID)
	ready, ok := bus.emitted[1].(*events.CancelablePaymentReady)
	require.True(t, ok, "vendor cancelable payment should emit auto-confirm event")
	require.Equal(t, "order-1", ready.OrderID)
	require.Equal(t, uint64(1000), ready.Amount)

	got := loadOrder(t, db, "order-1")
	require.True(t, got.IsPaymentVerified())
	require.Equal(t, models.OrderState_PENDING, got.State)
	require.Equal(t, "0xmanagedescrow", got.PaymentAddress)
	require.Equal(t, "1000", got.TotalReceived)
	require.Empty(t, got.OverpaidAmount, "exact match leaves OverpaidAmount empty")
	require.NotEmpty(t, got.SerializedPaymentSent)
	require.NotNil(t, got.PaidAt)

	ps, err := got.PaymentSentMessage()
	require.NoError(t, err)
	require.Equal(t, "1000", ps.Amount)
	require.Equal(t, testUSDCAsset, ps.Coin)
	require.Empty(t, ps.RefundAddress, "monitor-only envelope must not capture buyer-local refund metadata")
	require.Equal(t, "0xrefund", got.RefundAddress, "local order metadata remains available outside the shared envelope")
	// The latest observation is obs-2 (later block time), so its tx
	// hash and payer address represent the envelope.
	require.Equal(t, "0xtx-2", ps.TransactionID)
	require.Equal(t, "0xpayer-2", ps.PayerAddress)
	require.Equal(t, "0xmanagedescrow", ps.ToAddress)
	require.Equal(t, "0xusdc", ps.PaymentTokenAddress)
	require.Equal(t, pb.PaymentSent_CANCELABLE, ps.GetSettlementSpec().GetMethod())
	require.Equal(t, "0xmanagedescrow", ps.ContractAddress)
	require.Equal(t, int64(1778760300), ps.Timestamp.AsTime().Unix())
}

func TestBuildAggregatedPaymentSent_UsesDeterministicObservationTimestamp(t *testing.T) {
	order := &models.Order{ID: models.OrderID("order-deterministic-ts")}
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Coin:      "crypto:eip155:1:native",
		Amount:    1000,
		Address:   "0xmanagedescrow",
		Moderated: false,
	}))
	orderOpen := &pb.OrderOpen{Chaincode: "abcd"}
	blockTime := time.Date(2026, 5, 14, 12, 30, 0, 0, time.UTC)
	rows := []models.PaymentObservation{{
		ID:             "obs-deterministic",
		OrderID:        "order-deterministic-ts",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx",
		EventType:      models.PaymentEventManagedEscrowReceived,
		FromAddress:    "0xpayer",
		ToAddress:      "0xmanagedescrow",
		Amount:         "1000",
		BlockTime:      blockTime,
	}}

	psA, err := buildAggregatedPaymentSent(orderOpen, rows, big.NewInt(1000), order, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	psB, err := buildAggregatedPaymentSent(orderOpen, rows, big.NewInt(1000), order, time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, blockTime.Unix(), psA.Timestamp.AsTime().Unix())
	require.Equal(t, psA.Timestamp.AsTime().Unix(), psB.Timestamp.AsTime().Unix())
}

func TestBuildAggregatedPaymentSent_PrefersExistingSharedRefundAddress(t *testing.T) {
	order := &models.Order{ID: models.OrderID("order-shared-refund"), RefundAddress: "0xbuyer-local-only"}
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Coin:      "crypto:eip155:1:native",
		Amount:    1000,
		Address:   "0xmanagedescrow",
		Moderated: false,
	}))
	require.NoError(t, order.SetPaymentSent(&pb.PaymentSent{
		RefundAddress: "0xshared-refund",
	}))
	orderOpen := &pb.OrderOpen{Chaincode: "abcd"}
	rows := []models.PaymentObservation{{
		ID:             "obs-shared-refund",
		OrderID:        "order-shared-refund",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx",
		EventType:      models.PaymentEventManagedEscrowReceived,
		FromAddress:    "0xpayer",
		ToAddress:      "0xmanagedescrow",
		Amount:         "1000",
		BlockTime:      time.Date(2026, 5, 14, 12, 30, 0, 0, time.UTC),
	}}

	ps, err := buildAggregatedPaymentSent(orderOpen, rows, big.NewInt(1000), order, time.Now())
	require.NoError(t, err)
	require.Equal(t, "0xshared-refund", ps.RefundAddress)
}

func TestAggregateAndEmit_VendorVerifiedPaymentEmitsOrderFunded(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	handlerCalls := make(chan verifiedHandlerCall, 1)
	v := NewAggregatingVerifier(db, bus)
	v.SetPaymentVerifiedHandler(func(orderID string, paymentSent *pb.PaymentSent) {
		handlerCalls <- verifiedHandlerCall{orderID: orderID, paymentSent: paymentSent}
	})

	seedOrderForRoleWithListing(t, db, "order-vendor-funded", "1000", models.RoleVendor)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().
			Where("tenant_id = ? AND id = ?", database.StandaloneTenantID, "order-vendor-funded").
			First(&order).Error; err != nil {
			return err
		}
		if err := order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
			Coin:    testUSDCAsset,
			Amount:  1000,
			Address: "0xmanagedescrow",
			SettlementSpec: &models.PendingSettlementSpec{
				Method:     "CANCELABLE",
				PayMode:    "address_monitored",
				EscrowType: "managed_escrow",
			},
		}); err != nil {
			return err
		}
		return tx.Save(&order)
	}))
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-vendor-funded",
		OrderID:        "order-vendor-funded",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-vendor-funded",
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "1000",
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-vendor-funded"))

	require.Len(t, bus.emitted, 3)
	_, ok := bus.emitted[0].(events.PaymentVerified)
	require.True(t, ok, "first event remains the internal verification signal")
	funded, ok := bus.emitted[1].(*events.OrderFunded)
	require.True(t, ok, "vendor verification must emit order.funded for notifications/cache invalidation")
	require.Equal(t, database.StandaloneTenantID, funded.TenantID)
	require.Equal(t, "order-vendor-funded", funded.OrderID)
	require.Equal(t, "Deterministic Payment Listing", funded.Title)
	require.Equal(t, "deterministic-payment-listing", funded.Slug)
	ready, ok := bus.emitted[2].(*events.CancelablePaymentReady)
	require.True(t, ok, "cancelable verified payments still trigger auto-confirm")
	require.Equal(t, "order-vendor-funded", ready.OrderID)
	require.Equal(t, uint64(1000), ready.Amount)
	select {
	case call := <-handlerCalls:
		require.Equal(t, "order-vendor-funded", call.orderID)
		require.NotNil(t, call.paymentSent)
		require.Equal(t, "0xtx-vendor-funded", call.paymentSent.TransactionID)
	case <-time.After(time.Second):
		t.Fatal("vendor monitor verification did not call the cross-node payment verified handler")
	}
}

func TestAggregateAndEmit_VendorCancelableEmitsReadyWithoutFundedNotification(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-vendor-ready-no-listing", "1000", "btc-refund")
	seedPendingUTXOInfo(t, db, "order-vendor-ready-no-listing", "mempool_accepted")
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-vendor-ready-no-listing",
		OrderID:        "order-vendor-ready-no-listing",
		ChainNamespace: "bip122",
		ChainReference: "000000000019d6689c085ae165831e93",
		TxHash:         "btc-ready-tx",
		EventType:      models.PaymentEventUTXOFunding,
		FromAddress:    "btc-payer",
		ToAddress:      "bch-escrow",
		Amount:         "1000",
		Status:         models.PaymentObservationStatusConfirmed,
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-vendor-ready-no-listing"))

	require.Len(t, bus.emitted, 2)
	_, ok := bus.emitted[0].(events.PaymentVerified)
	require.True(t, ok, "first event remains the internal verification signal")
	ready, ok := bus.emitted[1].(*events.CancelablePaymentReady)
	require.True(t, ok, "financial auto-confirm event must not depend on listing notification payload")
	require.Equal(t, "order-vendor-ready-no-listing", ready.OrderID)
	require.Equal(t, uint64(1000), ready.Amount)
	require.Equal(t, "btc-ready-tx", ready.TransactionID)
}

func TestAggregateAndEmit_BuyerVerifiedPaymentEmitsPaymentReceived(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	handlerCalls := make(chan verifiedHandlerCall, 1)
	v := NewAggregatingVerifier(db, bus)
	v.SetPaymentVerifiedHandler(func(orderID string, paymentSent *pb.PaymentSent) {
		handlerCalls <- verifiedHandlerCall{orderID: orderID, paymentSent: paymentSent}
	})

	seedOrderForRoleWithListing(t, db, "order-buyer-funded", "1000", models.RoleBuyer)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().
			Where("tenant_id = ? AND id = ?", database.StandaloneTenantID, "order-buyer-funded").
			First(&order).Error; err != nil {
			return err
		}
		if err := order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
			Coin:    testUSDCAsset,
			Amount:  1000,
			Address: "0xmanagedescrow",
			SettlementSpec: &models.PendingSettlementSpec{
				Method:     "DIRECT",
				PayMode:    "address_monitored",
				EscrowType: "managed_escrow",
			},
		}); err != nil {
			return err
		}
		return tx.Save(&order)
	}))
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-buyer-funded",
		OrderID:        "order-buyer-funded",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-buyer-funded",
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "1000",
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-buyer-funded"))

	require.Len(t, bus.emitted, 2)
	_, ok := bus.emitted[0].(events.PaymentVerified)
	require.True(t, ok)
	received, ok := bus.emitted[1].(*events.OrderPaymentReceived)
	require.True(t, ok, "buyer verification must emit order.payment_received for notifications/cache invalidation")
	require.Equal(t, database.StandaloneTenantID, received.TenantID)
	require.Equal(t, "order-buyer-funded", received.OrderID)
	require.Equal(t, "1000", received.FundingTotal)
	require.Equal(t, testUSDCAsset, received.CoinType)
	select {
	case call := <-handlerCalls:
		require.Equal(t, "order-buyer-funded", call.orderID)
		require.NotNil(t, call.paymentSent)
		require.Equal(t, "0xtx-buyer-funded", call.paymentSent.TransactionID)
	case <-time.After(time.Second):
		t.Fatal("buyer monitor verification did not call the cross-node payment verified handler")
	}
}

func TestAggregateAndEmit_SyntheticBalancePollVerifiesWithoutExplorerTx(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-balance-poll", "1000", "0xrefund")
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-balance-poll",
		OrderID:        "order-balance-poll",
		ChainNamespace: "eip155",
		ChainReference: "11155111",
		TxHash:         "0xsyntheticbalancepollid",
		TxHashSource:   models.PaymentTxHashSourceBalancePoll,
		EventType:      models.PaymentEventManagedEscrowReceived,
		ToAddress:      "0xmanagedescrow",
		Amount:         "1000",
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-balance-poll"))

	got := loadOrder(t, db, "order-balance-poll")
	require.True(t, got.IsPaymentVerified())
	require.Equal(t, "1000", got.TotalReceived)

	ps, err := got.PaymentSentMessage()
	require.NoError(t, err)
	require.Empty(t, ps.TransactionID, "balance-poll ids are internal observation ids, not explorer-safe tx hashes")
	require.Equal(t, "0xmanagedescrow", ps.ToAddress)
	require.Equal(t, "1000", ps.Amount)
}

func TestAggregateAndEmit_MixedSyntheticAndChainTxPrefersChainTxForEnvelope(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-mixed", "1000", "0xrefund")
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-chain",
		OrderID:        "order-mixed",
		ChainNamespace: "eip155",
		ChainReference: "11155111",
		TxHash:         "0xrealtx",
		TxHashSource:   models.PaymentTxHashSourceChainTx,
		EventType:      models.PaymentEventManagedEscrowReceived,
		FromAddress:    "0xpayer",
		ToAddress:      "0xmanagedescrow",
		Amount:         "400",
	})
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-synthetic-later",
		OrderID:        "order-mixed",
		ChainNamespace: "eip155",
		ChainReference: "11155111",
		TxHash:         "0xsyntheticlater",
		TxHashSource:   models.PaymentTxHashSourceBalancePoll,
		EventType:      models.PaymentEventManagedEscrowReceived,
		ToAddress:      "0xmanagedescrow",
		Amount:         "600",
		BlockTime:      time.Date(2026, 5, 14, 12, 10, 0, 0, time.UTC),
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-mixed"))

	got := loadOrder(t, db, "order-mixed")
	ps, err := got.PaymentSentMessage()
	require.NoError(t, err)
	require.Equal(t, "0xrealtx", ps.TransactionID)
	require.Equal(t, "0xpayer", ps.PayerAddress)
	require.Equal(t, "1000", ps.Amount)
}

func TestAggregateAndEmit_PendingManagedEscrowAmountOverridesOrderOpenAmount(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-managed_escrow-amount", "1500", "0xrefund")
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().
			Where("tenant_id = ? AND id = ?", database.StandaloneTenantID, "order-managed_escrow-amount").
			First(&order).Error; err != nil {
			return err
		}
		if err := order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
			Coin:    "crypto:eip155:1:native",
			Amount:  1000,
			Address: "0xmanagedescrow",
		}); err != nil {
			return err
		}
		return tx.Save(&order)
	}))
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-managed_escrow-amount",
		OrderID:        "order-managed_escrow-amount",
		ChainNamespace: "eip155",
		ChainReference: "11155111",
		TxHash:         "0xmanagedescrow",
		EventType:      models.PaymentEventManagedEscrowReceived,
		FromAddress:    "0xpayer",
		ToAddress:      "0xmanagedescrow",
		Amount:         "1000",
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-managed_escrow-amount"))

	got := loadOrder(t, db, "order-managed_escrow-amount")
	require.True(t, got.IsPaymentVerified())
	require.Equal(t, "1000", got.TotalReceived)
	require.Empty(t, got.OverpaidAmount, "OrderOpen.Amount is pricing amount, not ManagedEscrow wei")

	ps, err := got.PaymentSentMessage()
	require.NoError(t, err)
	require.Equal(t, "1000", ps.Amount)
	require.Equal(t, "crypto:eip155:1:native", ps.Coin)
}

func TestBuildAggregatedPaymentSent_DerivesNativeCoinFromObservationWhenPricingIsFiat(t *testing.T) {
	order := &models.Order{ID: models.OrderID("order-observed-native")}
	orderOpen := &pb.OrderOpen{
		PricingCoin: "USD",
		Amount:      "1500",
	}
	rows := []models.PaymentObservation{{
		ID:             "obs-native",
		OrderID:        "order-observed-native",
		ChainNamespace: "eip155",
		ChainReference: "11155111",
		TxHash:         "0xtx-native",
		EventType:      models.PaymentEventManagedEscrowReceived,
		FromAddress:    "0xpayer",
		ToAddress:      "0xmanagedescrow",
		Amount:         "1000",
	}}

	ps, err := buildAggregatedPaymentSent(orderOpen, rows, big.NewInt(1000), order, time.Now())

	require.NoError(t, err)
	require.Equal(t, "crypto:eip155:1:native", ps.Coin)
}

func TestBuildAggregatedPaymentSent_DoesNotUsePricingCoinAsSettlementCoin(t *testing.T) {
	order := &models.Order{ID: models.OrderID("order-no-pricing-fallback")}
	orderOpen := &pb.OrderOpen{
		PricingCoin: "USD",
		Amount:      "1500",
	}
	rows := []models.PaymentObservation{{
		ID:             "obs-unknown-chain",
		OrderID:        "order-no-pricing-fallback",
		ChainNamespace: "unknown",
		ChainReference: "unknown",
		TxHash:         "0xtx-unknown",
		EventType:      models.PaymentEventManagedEscrowReceived,
		FromAddress:    "0xpayer",
		ToAddress:      "0xmanagedescrow",
		Amount:         "1000",
	}}

	_, err := buildAggregatedPaymentSent(orderOpen, rows, big.NewInt(1000), order, time.Now())

	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot determine PaymentSent.Coin")
}

func TestBuildAggregatedPaymentSent_PendingEscrowRequiresSettlementSpec(t *testing.T) {
	order := &models.Order{ID: models.OrderID("order-escrow-missing-spec")}
	require.NoError(t, order.SetPendingEscrowPaymentInfo(&models.PendingEscrowPaymentInfo{
		Coin:          "crypto:eip155:1:native",
		EscrowAddress: "0xescrow",
	}))
	orderOpen := &pb.OrderOpen{
		PricingCoin: "USD",
		Amount:      "1500",
	}
	rows := []models.PaymentObservation{{
		ID:             "obs-escrow",
		OrderID:        "order-escrow-missing-spec",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-escrow",
		FromAddress:    "0xpayer",
		ToAddress:      "0xescrow",
		Amount:         "1000",
	}}

	_, err := buildAggregatedPaymentSent(orderOpen, rows, big.NewInt(1000), order, time.Now())

	require.Error(t, err)
	require.Contains(t, err.Error(), "missing settlement spec")
}

func TestAggregateAndEmit_PendingUTXOCoinOverridesOrderOpenPricingCoin(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-utxo-coin", "1500", "btc-refund")
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().
			Where("tenant_id = ? AND id = ?", database.StandaloneTenantID, "order-utxo-coin").
			First(&order).Error; err != nil {
			return err
		}
		if err := order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
			Coin:   "BTC",
			Amount: 1000,
			Script: "5221...",
		}); err != nil {
			return err
		}
		return tx.Save(&order)
	}))
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-utxo-coin",
		OrderID:        "order-utxo-coin",
		ChainNamespace: "bip122",
		ChainReference: "000000000019d6689c085ae165831e93",
		TxHash:         "btc-tx",
		EventType:      models.PaymentEventUTXOFunding,
		FromAddress:    "btc-payer",
		ToAddress:      "p2sh-target",
		Amount:         "1000",
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-utxo-coin"))

	got := loadOrder(t, db, "order-utxo-coin")
	ps, err := got.PaymentSentMessage()
	require.NoError(t, err)
	require.Equal(t, "1000", ps.Amount)
	require.Equal(t, "crypto:bip122:000000000019d6689c085ae165831e93:native", ps.Coin)
	require.Equal(t, pb.PaymentSent_CANCELABLE, ps.GetSettlementSpec().GetMethod())
}

func TestResolveAggregatedPaymentIntent_ManagedEscrowUsesSettlementSpecWhenPresent(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Address:   "0xmanagedescrow",
		Moderated: false,
		SettlementSpec: &models.PendingSettlementSpec{
			Method:     "MODERATED",
			PayMode:    "address_monitored",
			EscrowType: "managed_escrow",
		},
	}))

	intent := resolveAggregatedPaymentIntent(order, []models.PaymentObservation{{
		ChainNamespace: "eip155",
	}})
	require.Equal(t, pb.PaymentSent_MODERATED, intent.settlementSpec.Method)
}

func TestResolveAggregatedPaymentIntent_ManagedEscrowUsesPendingTrustModel(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Address:   "0xmanagedescrow",
		Moderated: false,
	}))

	intent := resolveAggregatedPaymentIntent(order, []models.PaymentObservation{{
		ChainNamespace: "eip155",
	}})
	require.Equal(t, pb.PaymentSent_CANCELABLE, intent.settlementSpec.Method)
	require.Equal(t, "0xmanagedescrow", intent.contractAddress)

	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Address:          "0xmanagedescrow",
		Moderated:        true,
		Moderator:        "mod-peer",
		ModeratorAddress: "0xmoderator",
	}))
	intent = resolveAggregatedPaymentIntent(order, []models.PaymentObservation{{
		ChainNamespace: "eip155",
	}})
	require.Equal(t, pb.PaymentSent_MODERATED, intent.settlementSpec.Method)
	require.Equal(t, "0xmanagedescrow", intent.contractAddress)
	require.Equal(t, "mod-peer", intent.moderator)
	require.Equal(t, "0xmoderator", intent.moderatorAddress)
}

func TestResolveAggregatedPaymentIntent_UTXOUsesSettlementSpecWhenPresent(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
		Script:    "5221...",
		Moderator: "",
		SettlementSpec: &models.PendingSettlementSpec{
			Method:     "CANCELABLE",
			PayMode:    "address_monitored",
			EscrowType: "utxo_script",
		},
	}))

	intent := resolveAggregatedPaymentIntent(order, []models.PaymentObservation{{
		ChainNamespace: "bip122",
	}})
	require.Equal(t, pb.PaymentSent_CANCELABLE, intent.settlementSpec.Method)
	require.Equal(t, "5221...", intent.script)
}

func TestResolveAggregatedPaymentIntent_UTXOUsesPendingEscrowFields(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
		Script:          "5221...",
		Moderator:       "moderator-peer",
		ModeratorPubkey: "02abcdef",
		UnlockHours:     72,
	}))

	intent := resolveAggregatedPaymentIntent(order, []models.PaymentObservation{{
		ChainNamespace: "bip122",
	}})
	require.Equal(t, pb.PaymentSent_MODERATED, intent.settlementSpec.Method)
	require.Equal(t, "5221...", intent.script)
	require.Equal(t, "moderator-peer", intent.moderator)
	require.Equal(t, "02abcdef", intent.moderatorAddress)
	require.Equal(t, uint32(72), intent.escrowTimeoutHours)
}

func TestResolveAggregatedPaymentIntent_UTXOUsesPendingForBitcoinCashNamespace(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
		Script:    "5221bch...",
		Moderator: "moderator-peer",
	}))

	intent := resolveAggregatedPaymentIntent(order, []models.PaymentObservation{{
		ChainNamespace: "bitcoincash",
	}})
	require.Equal(t, pb.PaymentSent_MODERATED, intent.settlementSpec.Method)
	require.Equal(t, "5221bch...", intent.script)
}

func TestResolveAggregatedPaymentIntent_LegacyContractUsesSettlementSpecWhenPresent(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingEscrowPaymentInfo(&models.PendingEscrowPaymentInfo{
		Coin:            "ETH",
		ContractAddress: "0xcontract",
		EscrowAddress:   "0xescrow",
		SettlementSpec: &models.PendingSettlementSpec{
			Method:     "MODERATED",
			PayMode:    "client_signed",
			EscrowType: "evm_contract",
		},
	}))

	intent := resolveAggregatedPaymentIntent(order, []models.PaymentObservation{{
		ChainNamespace: "eip155",
	}})
	require.Equal(t, pb.PaymentSent_MODERATED, intent.settlementSpec.Method)
	require.Equal(t, "0xcontract", intent.contractAddress)
}

func TestResolveAggregatedPaymentIntent_EscrowFallsBackToEscrowAddressForLegacyRows(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingEscrowPaymentInfo(&models.PendingEscrowPaymentInfo{
		Coin:          "ETH",
		EscrowAddress: "0xescrow",
		SettlementSpec: &models.PendingSettlementSpec{
			Method:     "MODERATED",
			PayMode:    "client_signed",
			EscrowType: "evm_contract",
		},
	}))

	intent := resolveAggregatedPaymentIntent(order, []models.PaymentObservation{{
		ChainNamespace: "eip155",
	}})
	require.Equal(t, "0xescrow", intent.contractAddress)
}

func TestBuildAggregatedPaymentSent_SolanaEscrowPreservesProgramID(t *testing.T) {
	const programID = "AnD79RcbbS1GsvNZZHcQTGRvozVL1J9mr4GJiwm587pX"
	const escrowAddress = "RT38nT6ABNLfotNxwseiNNKukCKAXpFkZctJGn4EbFe"
	order := &models.Order{ID: models.OrderID("order-solana-escrow")}
	require.NoError(t, order.SetPendingEscrowPaymentInfo(&models.PendingEscrowPaymentInfo{
		Coin:            "crypto:solana:mainnet:native",
		Amount:          1000,
		ContractAddress: programID,
		EscrowAddress:   escrowAddress,
		SettlementSpec: &models.PendingSettlementSpec{
			Method:     "CANCELABLE",
			PayMode:    "address_monitored",
			EscrowType: "solana_escrow",
		},
	}))
	orderOpen := &pb.OrderOpen{Chaincode: "abcd", Amount: "1000"}
	rows := []models.PaymentObservation{{
		ID:             "obs-solana",
		OrderID:        "order-solana-escrow",
		ChainNamespace: "solana",
		ChainReference: "devnet",
		TxHash:         "5MB37D74PqcfycEhV6xYnkLTctcXgn2bYVgSxARJ9Ngf8sdKXosVarSwwhMyCUYC9QVDyaFJtC8YEK1uVwMwnUba",
		EventType:      models.PaymentEventSolanaTransfer,
		FromAddress:    "E1Cg7NbEpvRy7jjyyxAaCnEx7SssUFbxNGsfYVJNUJEn",
		ToAddress:      escrowAddress,
		Amount:         "1000",
		BlockNumber:    1,
		BlockTime:      time.Date(2026, 5, 28, 0, 29, 18, 0, time.UTC),
	}}

	ps, err := buildAggregatedPaymentSent(orderOpen, rows, big.NewInt(1000), order, time.Now())
	require.NoError(t, err)
	require.Equal(t, programID, ps.ContractAddress)
	require.Equal(t, escrowAddress, ps.ToAddress)
}

func TestResolveAggregatedPaymentIntent_EscrowWithoutSettlementSpecIsInvalid(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingEscrowPaymentInfo(&models.PendingEscrowPaymentInfo{
		Coin:          "ETH",
		EscrowAddress: "0xescrow",
		Moderator:     "mod-peer",
	}))

	intent := resolveAggregatedPaymentIntent(order, nil)
	require.False(t, intent.settlementSpecOK)
	require.Equal(t, "0xescrow", intent.contractAddress)
	require.Equal(t, "mod-peer", intent.moderator)
}

func TestResolveAggregatedPaymentIntent_NoPendingIntentFallsBackDirect(t *testing.T) {
	order := &models.Order{}
	intent := resolveAggregatedPaymentIntent(order, []models.PaymentObservation{{
		ChainNamespace: "eip155",
	}})
	require.Equal(t, pb.PaymentSent_DIRECT, intent.settlementSpec.Method)
}

func TestAggregateAndEmit_BackfillsRefundAddressFromUniqueObservedSender(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-empty-refund", "1000", "")
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-empty-refund",
		OrderID:        "order-empty-refund",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-refund",
		EventType:      models.PaymentEventManagedEscrowReceived,
		FromAddress:    "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ToAddress:      "0xmanagedescrow",
		Amount:         "1000",
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-empty-refund"))

	got := loadOrder(t, db, "order-empty-refund")
	require.True(t, got.IsPaymentVerified())
	require.Equal(t, "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", got.RefundAddress)

	ps, err := got.PaymentSentMessage()
	require.NoError(t, err)
	require.Equal(t, "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ps.RefundAddress)
	require.Equal(t, "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ps.PayerAddress)
}

func TestAggregateAndEmit_DoesNotBackfillRefundAddressFromAmbiguousSenders(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-ambiguous-refund", "1000", "")
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-ambiguous-1",
		OrderID:        "order-ambiguous-refund",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-ambiguous-1",
		EventType:      models.PaymentEventManagedEscrowReceived,
		FromAddress:    "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ToAddress:      "0xmanagedescrow",
		Amount:         "400",
	})
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-ambiguous-2",
		OrderID:        "order-ambiguous-refund",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-ambiguous-2",
		EventType:      models.PaymentEventManagedEscrowReceived,
		FromAddress:    "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ToAddress:      "0xmanagedescrow",
		Amount:         "600",
		BlockTime:      time.Date(2026, 5, 14, 12, 5, 0, 0, time.UTC),
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-ambiguous-refund"))

	got := loadOrder(t, db, "order-ambiguous-refund")
	require.True(t, got.IsPaymentVerified())
	require.Empty(t, got.RefundAddress)

	ps, err := got.PaymentSentMessage()
	require.NoError(t, err)
	require.Empty(t, ps.RefundAddress)
	require.Equal(t, "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ps.PayerAddress)
}

func TestAggregateAndEmit_DoesNotBackfillRefundAddressFromMissingSender(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-missing-sender", "1000", "")
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-missing-sender",
		OrderID:        "order-missing-sender",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-missing-sender",
		EventType:      models.PaymentEventManagedEscrowReceived,
		ToAddress:      "0xmanagedescrow",
		Amount:         "1000",
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-missing-sender"))

	got := loadOrder(t, db, "order-missing-sender")
	require.True(t, got.IsPaymentVerified())
	require.Empty(t, got.RefundAddress)

	ps, err := got.PaymentSentMessage()
	require.NoError(t, err)
	require.Empty(t, ps.RefundAddress)
	require.Empty(t, ps.PayerAddress)
}

func TestAggregateAndEmit_Overpaid_RecordsSurplus(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-1", "1000", "0xrefund")
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-1",
		OrderID:        "order-1",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-over",
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "1500",
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-1"))

	require.Len(t, bus.emitted, 1)
	got := loadOrder(t, db, "order-1")
	require.True(t, got.IsPaymentVerified())
	require.Equal(t, "1500", got.TotalReceived)
	require.Equal(t, "500", got.OverpaidAmount)
}

// ─────────────────────────────────────────────────────────────────────────
// AggregateAndEmit — observation source priority
// ─────────────────────────────────────────────────────────────────────────

func TestAggregateAndEmit_DedupesByObserverPriority(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-1", "1000", "0xrefund")
	// Two observations, same chain event, different observers. The
	// monitor row must win and its 1000 is what counts toward the
	// expected amount; if we accidentally summed both rows the total
	// would be 2000 and we'd record an OverpaidAmount.
	common := models.PaymentObservation{
		OrderID:        "order-1",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-shared",
		EventIndex:     0,
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "1000",
	}
	monitor := common
	monitor.ID = "obs-monitor"
	monitor.Source = models.PaymentObservationSourceMonitor
	monitor.Observer = "monitor:eip155-1:worker-A"
	insertObs(t, db, monitor)

	buyer := common
	buyer.ID = "obs-buyer"
	buyer.Source = models.PaymentObservationSourceBuyerReported
	buyer.Observer = "buyer:peer-1"
	insertObs(t, db, buyer)

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-1"))

	require.Len(t, bus.emitted, 1)
	got := loadOrder(t, db, "order-1")
	require.Equal(t, "1000", got.TotalReceived)
	require.Empty(t, got.OverpaidAmount, "dedupe must collapse the two observers to one row")
}

// ─────────────────────────────────────────────────────────────────────────
// AggregateAndEmit — idempotency
// ─────────────────────────────────────────────────────────────────────────

func TestAggregateAndEmit_AlreadyVerified_SkipsEmitButRefreshesTotals(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-1", "1000", "0xrefund")
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-1",
		OrderID:        "order-1",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-1",
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer-1",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "1000",
	})
	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-1"))
	require.Len(t, bus.emitted, 1)

	// Capture the envelope before the late deposit lands; we'll later
	// assert that AggregateAndEmit did NOT rewrite it, since the
	// envelope is the chain-of-trust target for downstream consumers.
	before := loadOrder(t, db, "order-1")
	frozenEnvelope := append([]byte(nil), before.SerializedPaymentSent...)

	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-2",
		OrderID:        "order-1",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-2",
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer-2",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "250",
		BlockTime:      time.Date(2026, 5, 14, 13, 0, 0, 0, time.UTC),
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-1"))
	require.Len(t, bus.emitted, 1, "second pass must not re-emit PaymentVerified")

	got := loadOrder(t, db, "order-1")
	require.True(t, got.IsPaymentVerified())
	require.Equal(t, "1250", got.TotalReceived, "late deposit must update TotalReceived")
	require.Equal(t, "250", got.OverpaidAmount, "late deposit becomes overpayment")
	require.Equal(t, frozenEnvelope, got.SerializedPaymentSent, "envelope is frozen at first verification")
}

// ─────────────────────────────────────────────────────────────────────────
// AggregateAndEmit — bad data surfaces errors
// ─────────────────────────────────────────────────────────────────────────

func TestAggregateAndEmit_InvalidExpectedAmount_Errors(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	// OrderOpen with a non-numeric amount.
	oo := &pb.OrderOpen{Amount: "abc", PricingCoin: "USDC"}
	raw, err := protojson.Marshal(oo)
	require.NoError(t, err)
	order := &models.Order{
		TenantMixin:         models.TenantMixin{TenantID: database.StandaloneTenantID},
		ID:                  "bad-amount-order",
		MyRole:              string(models.RoleVendor),
		SerializedOrderOpen: raw,
	}
	order.MarkPaymentVerificationPending()
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Save(order) }))

	err = v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "bad-amount-order")
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "not a decimal integer"), err.Error())
	require.Empty(t, bus.emitted)
}

func TestAggregateAndEmit_NegativeObservation_Errors(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-1", "1000", "0xrefund")
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-bad",
		OrderID:        "order-1",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-bad",
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "-100",
	})

	err := v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "negative amount")
	require.Empty(t, bus.emitted)
}

// ─────────────────────────────────────────────────────────────────────────
// AggregateAndEmit — multi-tenant isolation
// ─────────────────────────────────────────────────────────────────────────
//
// The Order primary key is (tenant_id, id). A SaaS deployment can host
// multiple tenants whose OrderID generators occasionally collide — UUIDs
// make it astronomically unlikely, but the WHERE clause should not
// depend on probability for correctness. This test plants the SAME
// orderID in two distinct tenant slices, drives the verifier on tenant
// A, and asserts:
//
//   - Tenant A's order transitions to verified and emits PaymentVerified
//     scoped to tenant A.
//   - Tenant B's order is untouched (still pending, no envelope written,
//     no overpaid amount, no PaymentVerified event for B).
//
// Without the (tenant_id, id) WHERE clause this test would fail because
// the SELECT would resolve to whichever row SQLite encountered first.
func TestAggregateAndEmit_CrossTenantIsolation(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	const sharedID = "order-shared"
	const tenantA = "tenant-A"
	const tenantB = "tenant-B"

	seedOrderForTenant(t, db, tenantA, sharedID, "1000", "0xrefund-A")
	seedOrderForTenant(t, db, tenantB, sharedID, "1000", "0xrefund-B")

	// Only tenant A receives an observation that fully funds the order.
	insertObs(t, db, models.PaymentObservation{
		TenantID:       tenantA,
		ID:             "obs-A",
		OrderID:        sharedID,
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-A",
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer-A",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "1000",
		Source:         models.PaymentObservationSourceMonitor,
		Observer:       "monitor:eip155-1:tenantA",
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), tenantA, sharedID))

	// A is verified and the event carries tenant A.
	require.Len(t, bus.emitted, 1)
	verified, ok := bus.emitted[0].(events.PaymentVerified)
	require.True(t, ok)
	require.Equal(t, tenantA, verified.TenantID)
	require.Equal(t, sharedID, verified.OrderID)

	gotA := loadOrderForTenant(t, db, tenantA, sharedID)
	require.True(t, gotA.IsPaymentVerified())
	require.Equal(t, "1000", gotA.TotalReceived)
	require.NotEmpty(t, gotA.SerializedPaymentSent)

	// B is untouched: still pending, still no envelope, still no totals.
	gotB := loadOrderForTenant(t, db, tenantB, sharedID)
	require.True(t, gotB.IsPaymentVerificationPending())
	require.Empty(t, gotB.TotalReceived)
	require.Empty(t, gotB.OverpaidAmount)
	require.Empty(t, gotB.SerializedPaymentSent)

	// Driving the verifier for tenant B should be a silent no-op (B has
	// no observations) and must not emit a second PaymentVerified.
	require.NoError(t, v.AggregateAndEmit(context.Background(), tenantB, sharedID))
	require.Len(t, bus.emitted, 1, "tenant B has no observations; no event")

	gotB = loadOrderForTenant(t, db, tenantB, sharedID)
	require.True(t, gotB.IsPaymentVerificationPending())
	require.Equal(t, "0", gotB.TotalReceived,
		"tenant B was driven through the verifier, so TotalReceived rolls forward to '0'")
	require.Empty(t, gotB.SerializedPaymentSent)
}

func TestAggregateAndEmit_SharedPaymentIntentAlignsHostedEnvelope(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	const sharedID = "order-mirror-safe"
	const tenantBuyer = "tenant-buyer"
	const tenantVendor = "tenant-vendor"

	seedOrderForTenant(t, db, tenantBuyer, sharedID, "1000", "0xbuyer-refund")
	seedOrderForTenant(t, db, tenantVendor, sharedID, "1000", "")

	require.NoError(t, db.Update(func(tx database.Tx) error {
		intent := &models.SharedPaymentIntent{
			OrderID:        models.OrderID(sharedID),
			PaymentAddress: "0xmanagedescrow",
			RefundAddress:  "0xbuyer-refund",
		}
		if err := intent.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
			Coin:    testUSDCAsset,
			Amount:  1000,
			Address: "0xmanagedescrow",
			SettlementSpec: &models.PendingSettlementSpec{
				Method:     "CANCELABLE",
				PayMode:    "address_monitored",
				EscrowType: "managed_escrow",
			},
		}); err != nil {
			return err
		}
		return tx.Save(intent)
	}))

	insertObs(t, db, models.PaymentObservation{
		TenantID:       tenantVendor,
		ID:             "obs-mirror-safe",
		OrderID:        sharedID,
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xmanagedescrow",
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "1000",
		Source:         models.PaymentObservationSourceMonitor,
		Observer:       "monitor:eip155-1:tenantVendor",
	})

	require.NoError(t, v.AggregateAndEmit(context.Background(), tenantVendor, sharedID))

	gotVendor := loadOrderForTenant(t, db, tenantVendor, sharedID)
	ps, err := gotVendor.PaymentSentMessage()
	require.NoError(t, err)
	require.Equal(t, pb.PaymentSent_CANCELABLE, ps.GetSettlementSpec().GetMethod())
	require.Equal(t, "0xmanagedescrow", ps.ContractAddress)
	require.Equal(t, "0xmanagedescrow", gotVendor.PaymentAddress)
	require.Equal(t, "0xbuyer-refund", gotVendor.RefundAddress)
}

// ─────────────────────────────────────────────────────────────────────────
// AggregateAndEmit — partial → partial accumulation
// ─────────────────────────────────────────────────────────────────────────
//
// Multi-deposit orders that don't yet reach the threshold need to roll
// TotalReceived forward across re-aggregations so the buyer's UI ("you've
// paid 6 of 10") never goes backwards. This test inserts two partial
// observations split across two AggregateAndEmit calls and asserts the
// running sum is preserved between passes.
func TestAggregateAndEmit_PartialThenPartial_AccumulatesTotal(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)

	seedOrder(t, db, "order-1", "1000", "0xrefund")

	// First partial deposit: 300 / 1000.
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-1",
		OrderID:        "order-1",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-1",
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer-1",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "300",
		BlockTime:      time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-1"))

	first := loadOrder(t, db, "order-1")
	require.True(t, first.IsPaymentVerificationPending())
	require.Equal(t, "300", first.TotalReceived)

	// Second partial deposit: 250 added to the pile.
	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-2",
		OrderID:        "order-1",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-2",
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer-2",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "250",
		BlockTime:      time.Date(2026, 5, 14, 12, 5, 0, 0, time.UTC),
	})
	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-1"))

	require.Empty(t, bus.emitted, "still under expected; no event yet")
	second := loadOrder(t, db, "order-1")
	require.True(t, second.IsPaymentVerificationPending())
	require.Equal(t, "550", second.TotalReceived)
	require.Empty(t, second.OverpaidAmount)
	require.Empty(t, second.SerializedPaymentSent)
}

// ─────────────────────────────────────────────────────────────────────────
// AggregateAndEmit — partial then full triggers a single emit
// ─────────────────────────────────────────────────────────────────────────
//
// The most common multi-deposit success path: buyer pays a chunk now,
// adds more later, the second pass crosses the threshold. We assert that:
//
//  1. The first AggregateAndEmit leaves the order in pending.
//  2. The second AggregateAndEmit, after a top-up arrives, transitions
//     to verified and emits exactly once.
//  3. The PaymentSent envelope is built from the LATEST observation
//     (BlockTime tie-break), not the first.
func TestAggregateAndEmit_PartialThenFull_EmitsOnce(t *testing.T) {
	db := newVerifierTestDB(t)
	bus := &recordingBus{}
	v := NewAggregatingVerifier(db, bus)
	frozen := time.Date(2026, 5, 14, 13, 0, 0, 0, time.UTC)
	v.SetClock(func() time.Time { return frozen })

	seedOrder(t, db, "order-1", "1000", "0xrefund")

	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-partial",
		OrderID:        "order-1",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-partial",
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer-1",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "400",
		BlockTime:      time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-1"))
	require.Empty(t, bus.emitted, "partial pass must not emit")

	insertObs(t, db, models.PaymentObservation{
		ID:             "obs-topup",
		OrderID:        "order-1",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xtx-topup",
		EventType:      models.PaymentEventERC20Transfer,
		FromAddress:    "0xpayer-2",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "0xusdc",
		Amount:         "600",
		BlockTime:      time.Date(2026, 5, 14, 12, 30, 0, 0, time.UTC),
	})
	require.NoError(t, v.AggregateAndEmit(context.Background(), database.StandaloneTenantID, "order-1"))

	require.Len(t, bus.emitted, 1, "exactly one PaymentVerified after threshold crossed")
	got := loadOrder(t, db, "order-1")
	require.True(t, got.IsPaymentVerified())
	require.Equal(t, "1000", got.TotalReceived)
	require.Empty(t, got.OverpaidAmount, "exact match leaves OverpaidAmount empty")
	require.NotEmpty(t, got.SerializedPaymentSent)

	ps, err := got.PaymentSentMessage()
	require.NoError(t, err)
	// Envelope must point at the LATEST observation (the top-up) — its
	// BlockTime is later, so buildAggregatedPaymentSent picks it as the
	// representative tx.
	require.Equal(t, "0xtx-topup", ps.TransactionID)
	require.Equal(t, "0xpayer-2", ps.PayerAddress)
	require.Equal(t, "1000", ps.Amount, "envelope amount is the aggregated total, not the latest tx")
	require.Equal(t, int64(1778761800), ps.Timestamp.AsTime().Unix())
	require.Len(t, ps.FundingFacts, 2)
	require.Equal(t, "obs-partial", ps.FundingFacts[0].Id)
	require.Equal(t, "0xtx-partial", ps.FundingFacts[0].TxHash)
	require.Equal(t, "400", ps.FundingFacts[0].Amount)
	require.Equal(t, "obs-topup", ps.FundingFacts[1].Id)
	require.Equal(t, "0xtx-topup", ps.FundingFacts[1].TxHash)
	require.Equal(t, "600", ps.FundingFacts[1].Amount)
}
