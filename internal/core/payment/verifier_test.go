//go:build !private_distribution

package payment

import (
	"context"
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
	require.NoError(t, db.AutoMigrate(&models.Order{}, &models.PaymentObservation{}))
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

	require.Len(t, bus.emitted, 1)
	verified, ok := bus.emitted[0].(events.PaymentVerified)
	require.True(t, ok, "expected PaymentVerified event")
	require.Equal(t, database.StandaloneTenantID, verified.TenantID)
	require.Equal(t, "order-1", verified.OrderID)

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
	require.Equal(t, "USDC", ps.Coin)
	require.Equal(t, "0xrefund", ps.RefundAddress)
	// The latest observation is obs-2 (later block time), so its tx
	// hash and payer address represent the envelope.
	require.Equal(t, "0xtx-2", ps.TransactionID)
	require.Equal(t, "0xpayer-2", ps.PayerAddress)
	require.Equal(t, "0xmanagedescrow", ps.ToAddress)
	require.Equal(t, "0xusdc", ps.PaymentTokenAddress)
	require.Equal(t, pb.PaymentSent_DIRECT, ps.Method)
	require.Equal(t, frozen.Unix(), ps.Timestamp.AsTime().Unix())
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
	require.Equal(t, pb.PaymentSent_CANCELABLE, ps.Method)
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
	require.Equal(t, pb.PaymentSent_MODERATED, intent.method)
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
	require.Equal(t, pb.PaymentSent_CANCELABLE, intent.method)
	require.Equal(t, "0xmanagedescrow", intent.contractAddress)

	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Address:   "0xmanagedescrow",
		Moderated: true,
	}))
	intent = resolveAggregatedPaymentIntent(order, []models.PaymentObservation{{
		ChainNamespace: "eip155",
	}})
	require.Equal(t, pb.PaymentSent_MODERATED, intent.method)
	require.Equal(t, "0xmanagedescrow", intent.contractAddress)
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
	require.Equal(t, pb.PaymentSent_MODERATED, intent.method)
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
	require.Equal(t, pb.PaymentSent_MODERATED, intent.method)
	require.Equal(t, "5221bch...", intent.script)
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
	require.Equal(t, frozen.Unix(), ps.Timestamp.AsTime().Unix())
}
