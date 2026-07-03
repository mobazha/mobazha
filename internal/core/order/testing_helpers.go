package order

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha/pkg/posts/pb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// testDatabase is a lightweight database.Database wrapper for unit tests.
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

func (d *testDatabase) ComputePublicDataHash() (cid.Cid, error) { return cid.Undef, nil }
func (d *testDatabase) Close() error                            { return nil }

type testTx struct{ db *gorm.DB }

func (t *testTx) Read() *gorm.DB             { return t.db }
func (t *testTx) Save(i interface{}) error   { return t.db.Save(i).Error }
func (t *testTx) Create(i interface{}) error { return t.db.Create(i).Error }
func (t *testTx) Update(key string, value interface{}, where map[string]interface{}, model interface{}) error {
	q := t.db.Model(model)
	for k, v := range where {
		q = q.Where(k, v)
	}
	return q.UpdateColumn(key, value).Error
}
func (t *testTx) UpdateColumns(values map[string]interface{}, where map[string]interface{}, model interface{}) (int64, error) {
	q := t.db.Model(model)
	for k, v := range where {
		q = q.Where(k, v)
	}
	res := q.UpdateColumns(values)
	return res.RowsAffected, res.Error
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

// PublicData stubs
func (t *testTx) GetProfile() (*models.Profile, error)                       { return nil, nil }
func (t *testTx) SetProfile(*models.Profile) error                           { return nil }
func (t *testTx) GetFollowers() (models.Followers, error)                    { return models.Followers{}, nil }
func (t *testTx) SetFollowers(models.Followers) error                        { return nil }
func (t *testTx) GetFollowing() (models.Following, error)                    { return models.Following{}, nil }
func (t *testTx) SetFollowing(models.Following) error                        { return nil }
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

// mockEventBus is a lightweight events.Bus for unit tests within this package.
type mockEventBus struct {
	emitted []interface{}
}

func (b *mockEventBus) Subscribe(_ interface{}, _ ...events.SubscriptionOpt) (events.Subscription, error) {
	return nil, nil
}
func (b *mockEventBus) Emit(evt interface{}) { b.emitted = append(b.emitted, evt) }

// NewTestOrderAppService creates a configured OrderAppService for tests.
// Exported so integration tests in core/ can call it.
func NewTestOrderAppService(t *testing.T, cfg OrderAppServiceConfig) *OrderAppService {
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
	if cfg.NodeID == "" {
		cfg.NodeID = "test-order-svc"
	}
	return NewOrderAppService(cfg)
}

// SeedOrderWithBuyer creates a persisted order that has a valid BuyerID in
// SerializedOrderOpen, so order.Buyer() returns the given buyerPeerID.
func SeedOrderWithBuyer(t *testing.T, svc *OrderAppService, id string, buyerPeerID string, state models.OrderState) {
	t.Helper()
	oo := &pb.OrderOpen{
		BuyerID: &pb.ID{PeerID: buyerPeerID},
	}
	data, err := protojson.Marshal(oo)
	require.NoError(t, err)

	o := &models.Order{
		ID:     models.OrderID(id),
		MyRole: "vendor",
	}
	o.SerializedOrderOpen = data
	o.SetFSMState(state)
	err = svc.db.Update(func(tx database.Tx) error {
		return tx.Save(o)
	})
	require.NoError(t, err)
}

// SeedOrder creates a persisted order for tests.
func SeedOrder(t *testing.T, svc *OrderAppService, id string, role string, state models.OrderState) {
	t.Helper()
	o := &models.Order{
		ID:     models.OrderID(id),
		MyRole: role,
	}
	o.SetFSMState(state)
	err := svc.db.Update(func(tx database.Tx) error {
		return tx.Save(o)
	})
	require.NoError(t, err)
}

// SetEscrowForTesting allows integration tests to inject a mock escrow.
func (s *OrderAppService) SetEscrowForTesting(e contracts.EscrowOperations) {
	s.escrow = e
}

// RelayOrDirectForTesting exposes relayOrDirect for tests in other packages.
func (s *OrderAppService) RelayOrDirectForTesting(
	orderID models.OrderID,
	action string,
	coin iwallet.CoinType,
	instructions interface{},
	directFn func() error,
	relayedFn func(iwallet.TransactionID) error,
) error {
	return s.relayOrDirect(orderID, action, coin, instructions, directFn, relayedFn)
}

// FiatOpsForTesting returns the fiatOps dependency for test assertions.
func (s *OrderAppService) FiatOpsForTesting() contracts.FiatPaymentOperations {
	return s.fiatOps
}

// SetDiscountResolverForTesting allows integration tests to inject a mock.
func (s *OrderAppService) SetDiscountResolverForTesting(fn DiscountResolverFunc) {
	s.discountResolver = fn
}

// SetDiscountRedemptionRecorderForTesting allows integration tests to inject a mock.
func (s *OrderAppService) SetDiscountRedemptionRecorderForTesting(fn DiscountRedemptionRecorderFunc) {
	s.discountRedemptionRecorder = fn
}

// BuildRefundMessageForTesting exposes buildRefundMessage for integration tests.
func (s *OrderAppService) BuildRefundMessageForTesting(order *models.Order, wallet iwallet.Wallet, refundTxID iwallet.TransactionID) (iwallet.Tx, *npb.OrderMessage, error) {
	return s.buildRefundMessage(order, wallet, refundTxID)
}

// CreateOrderForTesting exposes createOrder for integration tests.
func (s *OrderAppService) CreateOrderForTesting(ctx context.Context, purchase *models.Purchase) (*pb.OrderOpen, *models.DiscountResult, error) {
	return s.createOrder(ctx, purchase)
}
