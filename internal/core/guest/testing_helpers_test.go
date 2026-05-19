package guest

import (
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// testDatabase is a minimal database.Database implementation backed by an
// in-memory SQLite database, sufficient for unit-testing guest checkout flows.
type testDatabase struct {
	gormDB *gorm.DB
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

// testTx implements database.Tx with the minimum set of methods needed.
type testTx struct {
	db *gorm.DB
}

func (t *testTx) Read() *gorm.DB { return t.db }

func (t *testTx) Save(i interface{}) error { return t.db.Save(i).Error }

func (t *testTx) Update(key string, value interface{}, where map[string]interface{}, model interface{}) error {
	q := t.db.Model(model)
	for k, v := range where {
		q = q.Where(k+" = ?", v)
	}
	return q.Update(key, value).Error
}

func (t *testTx) UpdateColumns(values map[string]interface{}, where map[string]interface{}, model interface{}) (int64, error) {
	q := t.db.Model(model)
	for k, v := range where {
		q = q.Where(k, v)
	}
	res := q.UpdateColumns(values)
	return res.RowsAffected, res.Error
}

func (t *testTx) Delete(key string, value interface{}, where map[string]interface{}, model interface{}) error {
	q := t.db.Where(key+" = ?", value)
	for k, v := range where {
		q = q.Where(k+" = ?", v)
	}
	return q.Delete(model).Error
}

func (t *testTx) Commit() error               { panic("managed tx") }
func (t *testTx) Rollback() error             { panic("managed tx") }
func (t *testTx) DeleteAll(interface{}) error { return nil }
func (t *testTx) Migrate(model interface{}) error {
	return t.db.AutoMigrate(model)
}
func (t *testTx) RegisterCommitHook(func()) {}

// PublicData stubs — not exercised by guest checkout tests.
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

const testTenantID = "_default"

func newGuestTestDB(t *testing.T) *testDatabase {
	t.Helper()
	db, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(
		&models.GuestOrder{},
		&models.GuestOrderItem{},
		&models.InventoryReservation{},
		&models.SweepTask{},
		&models.UserPreferences{},
	))
	return &testDatabase{gormDB: db}
}

func seedGuestOrder(t *testing.T, db *testDatabase, id int, order models.GuestOrder) {
	t.Helper()
	order.ID = id
	order.TenantID = testTenantID
	if order.ExpiresAt.IsZero() {
		order.ExpiresAt = time.Now().Add(time.Hour)
	}
	require.NoError(t, db.gormDB.Create(&order).Error)
}

func seedReservation(t *testing.T, db *testDatabase, id int, r models.InventoryReservation) {
	t.Helper()
	r.ID = id
	r.TenantID = testTenantID
	require.NoError(t, db.gormDB.Create(&r).Error)
}

func loadGuestOrder(t *testing.T, db *testDatabase, token string) models.GuestOrder {
	t.Helper()
	var o models.GuestOrder
	require.NoError(t, db.gormDB.Where("order_token = ?", token).First(&o).Error)
	return o
}

func loadReservation(t *testing.T, db *testDatabase, id int) models.InventoryReservation {
	t.Helper()
	var r models.InventoryReservation
	require.NoError(t, db.gormDB.Where("id = ?", id).First(&r).Error)
	return r
}
