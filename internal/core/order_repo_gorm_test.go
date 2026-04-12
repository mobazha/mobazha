package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// testDatabase is a lightweight database.Database wrapper around
// an in-memory *gorm.DB for integration testing. Only the methods
// required by GormOrderRepo are properly implemented.
type testDatabase struct {
	gormDB *gorm.DB
}

func newTestDatabase(t *testing.T) *testDatabase {
	t.Helper()
	db, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Order{}))
	return &testDatabase{gormDB: db}
}

func (d *testDatabase) View(fn func(database.Tx) error) error {
	return fn(&testTx{db: d.gormDB})
}

func (d *testDatabase) Update(fn func(database.Tx) error) error {
	return d.gormDB.Transaction(func(tx *gorm.DB) error {
		return fn(&testTx{db: tx})
	})
}

func (d *testDatabase) ComputePublicDataHash() (cid.Cid, error) {
	return cid.Undef, nil
}
func (d *testDatabase) Close() error { return nil }

// testTx implements database.Tx with the minimum set of methods
// needed by GormOrderRepo (Read, Save, Update).
type testTx struct {
	db *gorm.DB
}

func (t *testTx) Read() *gorm.DB { return t.db }

func (t *testTx) Save(i interface{}) error { return t.db.Save(i).Error }

func (t *testTx) Update(key string, value interface{}, where map[string]interface{}, model interface{}) error {
	q := t.db.Model(model)
	for k, v := range where {
		q = q.Where(k, v)
	}
	return q.UpdateColumn(key, value).Error
}

func (t *testTx) Commit() error   { panic("managed tx") }
func (t *testTx) Rollback() error { panic("managed tx") }
func (t *testTx) Delete(key string, value interface{}, where map[string]interface{}, model interface{}) error {
	q := t.db.Where(key, value)
	for k, v := range where {
		q = q.Where(k, v)
	}
	return q.Delete(model).Error
}
func (t *testTx) DeleteAll(interface{}) error { return nil }
func (t *testTx) Migrate(interface{}) error   { return nil }
func (t *testTx) RegisterCommitHook(func())   {}

// PublicData stubs — not used by OrderRepo
func (t *testTx) GetProfile() (*models.Profile, error)    { return nil, nil }
func (t *testTx) SetProfile(*models.Profile) error        { return nil }
func (t *testTx) GetFollowers() (models.Followers, error) { return models.Followers{}, nil }
func (t *testTx) SetFollowers(models.Followers) error     { return nil }
func (t *testTx) GetFollowing() (models.Following, error) { return models.Following{}, nil }
func (t *testTx) SetFollowing(models.Following) error     { return nil }

func (t *testTx) GetListing(string) (*pb.SignedListing, error)               { return nil, nil }
func (t *testTx) SetListing(*pb.SignedListing) error                         { return nil }
func (t *testTx) GetEncryptedListing(string) ([]byte, error)                 { return nil, nil }
func (t *testTx) SetEncryptedListing(string, []byte) error                   { return nil }
func (t *testTx) DeleteListing(string) error                                 { return nil }
func (t *testTx) GetListingIndex() (models.ListingIndex, error)              { return nil, nil }
func (t *testTx) SetListingIndex(models.ListingIndex) error                  { return nil }
func (t *testTx) GetRatingIndex() (models.RatingIndex, error)                { return nil, nil }
func (t *testTx) SetRatingIndex(models.RatingIndex) error                    { return nil }
func (t *testTx) SetRating(*pb.Rating) error                                 { return nil }
func (t *testTx) GetPostIndex() ([]models.PostData, error)                   { return nil, nil }
func (t *testTx) SetPostIndex([]models.PostData) error                       { return nil }
func (t *testTx) AddPost(*postsPb.SignedPost) error                          { return nil }
func (t *testTx) DeletePost(string) error                                    { return nil }
func (t *testTx) PostExist(string) bool                                      { return false }
func (t *testTx) GetPost(string) (*postsPb.SignedPost, error)                { return nil, nil }
func (t *testTx) SetImage(models.Image) error                                { return nil }
func (t *testTx) GetImageByName(models.ImageSize, string) ([]byte, error)    { return nil, nil }
func (t *testTx) GetMediaByCID(string) ([]byte, string, error)               { return nil, "", nil }
func (t *testTx) IndexMediaCID(string, string, string, string, string) error { return nil }
func (t *testTx) SetUploadedFile(models.UploadedFile) error                  { return nil }
func (t *testTx) SetIntroVideo(models.IntroVideo) error                      { return nil }

// ── Setup helpers ───────────────────────────────────────────────

func newTestRepo(t *testing.T) (*GormOrderRepo, *testDatabase) {
	t.Helper()
	db := newTestDatabase(t)
	return NewGormOrderRepo(db), db
}

func seedRepoOrder(t *testing.T, db *testDatabase, id string, role string, state models.OrderState) *models.Order {
	t.Helper()
	return seedRepoOrderAt(t, db, id, role, state, time.Time{})
}

func seedRepoOrderAt(t *testing.T, db *testDatabase, id string, role string, state models.OrderState, createdAt time.Time) *models.Order {
	t.Helper()
	order := &models.Order{
		ID:        models.OrderID(id),
		MyRole:    role,
		Open:      true,
		Read:      false,
		CreatedAt: createdAt,
	}
	order.TenantID = database.StandaloneTenantID
	order.SetFSMState(state)
	require.NoError(t, db.gormDB.Create(order).Error)
	return order
}

// ═══════════════════════════════════════════════════════════════
// FindByID
// ═══════════════════════════════════════════════════════════════

func TestGormOrderRepo_FindByID_Success(t *testing.T) {
	repo, db := newTestRepo(t)
	seedRepoOrder(t, db, "order-1", "buyer", models.OrderState_PENDING)

	order, err := repo.FindByID(context.Background(), "order-1")
	require.NoError(t, err)
	assert.Equal(t, models.OrderID("order-1"), order.ID)
}

func TestGormOrderRepo_FindByID_NotFound(t *testing.T) {
	repo, _ := newTestRepo(t)

	_, err := repo.FindByID(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ═══════════════════════════════════════════════════════════════
// FindPurchases / FindSales
// ═══════════════════════════════════════════════════════════════

func TestGormOrderRepo_FindPurchases_FiltersRole(t *testing.T) {
	repo, db := newTestRepo(t)
	seedRepoOrder(t, db, "purchase-1", "buyer", models.OrderState_PENDING)
	seedRepoOrder(t, db, "sale-1", "vendor", models.OrderState_PENDING)

	orders, count, err := repo.FindPurchases(context.Background(), contracts.OrderFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
	assert.Equal(t, models.OrderID("purchase-1"), orders[0].ID)
}

func TestGormOrderRepo_FindSales_FiltersRole(t *testing.T) {
	repo, db := newTestRepo(t)
	seedRepoOrder(t, db, "purchase-1", "buyer", models.OrderState_PENDING)
	seedRepoOrder(t, db, "sale-1", "vendor", models.OrderState_PENDING)

	orders, count, err := repo.FindSales(context.Background(), contracts.OrderFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
	assert.Equal(t, models.OrderID("sale-1"), orders[0].ID)
}

func TestGormOrderRepo_FindPurchases_StateFilter(t *testing.T) {
	repo, db := newTestRepo(t)
	seedRepoOrder(t, db, "p1", "buyer", models.OrderState_PENDING)
	seedRepoOrder(t, db, "p2", "buyer", models.OrderState_AWAITING_FULFILLMENT)

	orders, _, err := repo.FindPurchases(context.Background(), contracts.OrderFilter{
		StateFilter: []models.OrderState{models.OrderState_PENDING},
	})
	require.NoError(t, err)
	assert.Len(t, orders, 1)
	assert.Equal(t, models.OrderID("p1"), orders[0].ID)
}

func TestGormOrderRepo_FindPurchases_Empty(t *testing.T) {
	repo, _ := newTestRepo(t)

	orders, count, err := repo.FindPurchases(context.Background(), contracts.OrderFilter{})
	require.NoError(t, err)
	assert.Empty(t, orders)
	assert.Equal(t, int64(0), count)
}

func TestGormOrderRepo_FindPurchases_OffsetPagination(t *testing.T) {
	repo, db := newTestRepo(t)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		seedRepoOrderAt(t, db, fmt.Sprintf("p-%d", i), "buyer", models.OrderState_PENDING,
			base.Add(time.Duration(i)*time.Hour))
	}

	// Page 1: first 2
	orders, total, err := repo.FindPurchases(context.Background(), contracts.OrderFilter{
		Limit:         2,
		SortAscending: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, orders, 2)
	assert.Equal(t, models.OrderID("p-0"), orders[0].ID)
	assert.Equal(t, models.OrderID("p-1"), orders[1].ID)

	// Page 2: next 2
	orders, total, err = repo.FindPurchases(context.Background(), contracts.OrderFilter{
		Limit:         2,
		Offset:        2,
		SortAscending: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, orders, 2)
	assert.Equal(t, models.OrderID("p-2"), orders[0].ID)
	assert.Equal(t, models.OrderID("p-3"), orders[1].ID)

	// Page 3: last 1
	orders, total, err = repo.FindPurchases(context.Background(), contracts.OrderFilter{
		Limit:         2,
		Offset:        4,
		SortAscending: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, orders, 1)
	assert.Equal(t, models.OrderID("p-4"), orders[0].ID)
}

func TestGormOrderRepo_FindSales_OffsetWithStateFilter(t *testing.T) {
	repo, db := newTestRepo(t)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		seedRepoOrderAt(t, db, fmt.Sprintf("s-%d", i), "vendor", models.OrderState_AWAITING_FULFILLMENT,
			base.Add(time.Duration(i)*time.Hour))
	}
	seedRepoOrderAt(t, db, "s-completed", "vendor", models.OrderState_COMPLETED,
		base.Add(10*time.Hour))

	orders, total, err := repo.FindSales(context.Background(), contracts.OrderFilter{
		StateFilter:   []models.OrderState{models.OrderState_AWAITING_FULFILLMENT},
		Limit:         2,
		Offset:        1,
		SortAscending: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(4), total)
	assert.Len(t, orders, 2)
	assert.Equal(t, models.OrderID("s-1"), orders[0].ID)
	assert.Equal(t, models.OrderID("s-2"), orders[1].ID)
}

func TestGormOrderRepo_FindPurchases_TotalCountWithLimit(t *testing.T) {
	repo, db := newTestRepo(t)
	for i := 0; i < 10; i++ {
		seedRepoOrder(t, db, fmt.Sprintf("tc-%d", i), "buyer", models.OrderState_PENDING)
	}

	orders, total, err := repo.FindPurchases(context.Background(), contracts.OrderFilter{
		Limit: 3,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(10), total)
	assert.Len(t, orders, 3)
}

// ═══════════════════════════════════════════════════════════════
// FindUnverifiedPaymentOrders
// ═══════════════════════════════════════════════════════════════

func TestGormOrderRepo_FindUnverifiedPaymentOrders(t *testing.T) {
	repo, db := newTestRepo(t)

	// Matching: vendor, open, has payment sent, not verified
	o1 := &models.Order{
		ID:                    "unverified-1",
		MyRole:                "vendor",
		Open:                  true,
		OrderPaymentState:     models.OrderPaymentState{PaymentVerificationStatus: models.PaymentVerificationStatusPending},
		SerializedPaymentSent: []byte("some-data"),
	}
	o1.SetFSMState(models.OrderState_AWAITING_FULFILLMENT)
	require.NoError(t, db.gormDB.Create(o1).Error)

	// Not matching: verified
	o2 := &models.Order{
		ID:                    "verified-1",
		MyRole:                "vendor",
		Open:                  true,
		OrderPaymentState:     models.OrderPaymentState{PaymentVerificationStatus: models.PaymentVerificationStatusVerified},
		SerializedPaymentSent: []byte("some-data"),
	}
	o2.SetFSMState(models.OrderState_AWAITING_FULFILLMENT)
	require.NoError(t, db.gormDB.Create(o2).Error)

	// Not matching: buyer role
	o3 := &models.Order{
		ID:                    "buyer-1",
		MyRole:                "buyer",
		Open:                  true,
		OrderPaymentState:     models.OrderPaymentState{PaymentVerificationStatus: models.PaymentVerificationStatusPending},
		SerializedPaymentSent: []byte("some-data"),
	}
	o3.SetFSMState(models.OrderState_PENDING)
	require.NoError(t, db.gormDB.Create(o3).Error)

	orders, err := repo.FindUnverifiedPaymentOrders(context.Background())
	require.NoError(t, err)
	assert.Len(t, orders, 1)
	assert.Equal(t, models.OrderID("unverified-1"), orders[0].ID)
}

// ═══════════════════════════════════════════════════════════════
// Save
// ═══════════════════════════════════════════════════════════════

func TestGormOrderRepo_Save_CreateAndUpdate(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()

	order := &models.Order{ID: "save-1", MyRole: "buyer", Open: true}
	order.TenantID = database.StandaloneTenantID
	order.SetFSMState(models.OrderState_PENDING)
	require.NoError(t, repo.Save(ctx, order))

	loaded, err := repo.FindByID(ctx, "save-1")
	require.NoError(t, err)
	assert.Equal(t, models.OrderState_PENDING, loaded.State)

	order.SetFSMState(models.OrderState_AWAITING_FULFILLMENT)
	require.NoError(t, repo.Save(ctx, order))

	loaded, err = repo.FindByID(ctx, "save-1")
	require.NoError(t, err)
	assert.Equal(t, models.OrderState_AWAITING_FULFILLMENT, loaded.State)
}

// ═══════════════════════════════════════════════════════════════
// MarkAsRead
// ═══════════════════════════════════════════════════════════════

func TestGormOrderRepo_MarkAsRead(t *testing.T) {
	repo, db := newTestRepo(t)
	seedRepoOrder(t, db, "read-1", "buyer", models.OrderState_PENDING)

	require.NoError(t, repo.MarkAsRead(context.Background(), "read-1"))

	order, err := repo.FindByID(context.Background(), "read-1")
	require.NoError(t, err)
	assert.True(t, order.Read)
}

func TestGormOrderRepo_MarkAsRead_AlreadyRead(t *testing.T) {
	repo, db := newTestRepo(t)
	o := seedRepoOrder(t, db, "read-2", "buyer", models.OrderState_PENDING)
	o.Read = true
	require.NoError(t, db.gormDB.Save(o).Error)

	require.NoError(t, repo.MarkAsRead(context.Background(), "read-2"))
}

// ═══════════════════════════════════════════════════════════════
// UpdateState
// ═══════════════════════════════════════════════════════════════

func TestGormOrderRepo_UpdateState(t *testing.T) {
	repo, db := newTestRepo(t)
	seedRepoOrder(t, db, "state-1", "buyer", models.OrderState_PENDING)

	require.NoError(t, repo.UpdateState(context.Background(), "state-1", models.OrderState_AWAITING_FULFILLMENT))

	order, err := repo.FindByID(context.Background(), "state-1")
	require.NoError(t, err)
	assert.Equal(t, models.OrderState_AWAITING_FULFILLMENT, order.State)
}

// ═══════════════════════════════════════════════════════════════
// UpdateLastCheckTime
// ═══════════════════════════════════════════════════════════════

func TestGormOrderRepo_UpdateLastCheckTime(t *testing.T) {
	repo, db := newTestRepo(t)
	seedRepoOrder(t, db, "check-1", "vendor", models.OrderState_PENDING)

	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, repo.UpdateLastCheckTime(context.Background(), "check-1", now))

	order, err := repo.FindByID(context.Background(), "check-1")
	require.NoError(t, err)
	assert.Equal(t, now.Unix(), order.LastCheckForPayments.Unix())
}

// ═══════════════════════════════════════════════════════════════
// ExpirePaymentVerification
// ═══════════════════════════════════════════════════════════════

func TestGormOrderRepo_ExpirePaymentVerification(t *testing.T) {
	repo, db := newTestRepo(t)
	seedRepoOrder(t, db, "expire-1", "vendor", models.OrderState_PENDING)

	marker := time.Date(1970, 1, 2, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.ExpirePaymentVerification(context.Background(), "expire-1", marker))

	order, err := repo.FindByID(context.Background(), "expire-1")
	require.NoError(t, err)
	assert.False(t, order.Open)
	assert.Equal(t, marker.Unix(), order.LastCheckForPayments.Unix())
}
