package core

import (
	"testing"

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

// featureTestDatabase is a tiny database.Database implementation used by
// private_distribution-safe store tests that need direct access to an in-memory GORM DB.
type featureTestDatabase struct {
	gormDB *gorm.DB
}

func newFeatureTestDatabase(t *testing.T, modelsToMigrate ...interface{}) *featureTestDatabase {
	t.Helper()

	db, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	for _, model := range modelsToMigrate {
		require.NoError(t, db.AutoMigrate(model))
	}
	return &featureTestDatabase{gormDB: db}
}

func (d *featureTestDatabase) View(fn func(database.Tx) error) error {
	return fn(&featureTestTx{db: d.gormDB})
}

func (d *featureTestDatabase) Update(fn func(database.Tx) error) error {
	return d.gormDB.Transaction(func(tx *gorm.DB) error {
		return fn(&featureTestTx{db: tx})
	})
}

func (d *featureTestDatabase) ComputePublicDataHash() (cid.Cid, error) {
	return cid.Undef, nil
}

func (d *featureTestDatabase) Close() error { return nil }

type featureTestTx struct {
	db *gorm.DB
}

func (t *featureTestTx) Read() *gorm.DB { return t.db }

func (t *featureTestTx) Save(i interface{}) error {
	if r, ok := i.(*models.InventoryReservation); ok && r.TenantID == "" {
		r.TenantID = database.StandaloneTenantID
	}
	return t.db.Save(i).Error
}

func (t *featureTestTx) Update(key string, value interface{}, where map[string]interface{}, model interface{}) error {
	q := t.db.Model(model)
	for k, v := range where {
		q = q.Where(k, v)
	}
	return q.UpdateColumn(key, value).Error
}

func (t *featureTestTx) UpdateColumns(values map[string]interface{}, where map[string]interface{}, model interface{}) (int64, error) {
	q := t.db.Model(model)
	for k, v := range where {
		q = q.Where(k, v)
	}
	res := q.UpdateColumns(values)
	return res.RowsAffected, res.Error
}

func (t *featureTestTx) Commit() error   { panic("managed tx") }
func (t *featureTestTx) Rollback() error { panic("managed tx") }

func (t *featureTestTx) Delete(key string, value interface{}, where map[string]interface{}, model interface{}) error {
	q := t.db.Where(key, value)
	for k, v := range where {
		q = q.Where(k, v)
	}
	return q.Delete(model).Error
}

func (t *featureTestTx) DeleteAll(interface{}) error { return nil }
func (t *featureTestTx) Migrate(model interface{}) error {
	return t.db.AutoMigrate(model)
}
func (t *featureTestTx) RegisterCommitHook(func()) {}

func (t *featureTestTx) GetProfile() (*models.Profile, error)    { return nil, nil }
func (t *featureTestTx) SetProfile(*models.Profile) error        { return nil }
func (t *featureTestTx) GetFollowers() (models.Followers, error) { return models.Followers{}, nil }
func (t *featureTestTx) SetFollowers(models.Followers) error     { return nil }
func (t *featureTestTx) GetFollowing() (models.Following, error) { return models.Following{}, nil }
func (t *featureTestTx) SetFollowing(models.Following) error     { return nil }
func (t *featureTestTx) GetListing(string) (*pb.SignedListing, error) {
	return nil, nil
}
func (t *featureTestTx) SetListing(*pb.SignedListing) error                         { return nil }
func (t *featureTestTx) GetEncryptedListing(string) ([]byte, error)                 { return nil, nil }
func (t *featureTestTx) SetEncryptedListing(string, []byte) error                   { return nil }
func (t *featureTestTx) DeleteListing(string) error                                 { return nil }
func (t *featureTestTx) GetListingIndex() (models.ListingIndex, error)              { return nil, nil }
func (t *featureTestTx) SetListingIndex(models.ListingIndex) error                  { return nil }
func (t *featureTestTx) GetRatingIndex() (models.RatingIndex, error)                { return nil, nil }
func (t *featureTestTx) SetRatingIndex(models.RatingIndex) error                    { return nil }
func (t *featureTestTx) SetRating(*pb.Rating) error                                 { return nil }
func (t *featureTestTx) GetPostIndex() ([]models.PostData, error)                   { return nil, nil }
func (t *featureTestTx) SetPostIndex([]models.PostData) error                       { return nil }
func (t *featureTestTx) AddPost(*postsPb.SignedPost) error                          { return nil }
func (t *featureTestTx) DeletePost(string) error                                    { return nil }
func (t *featureTestTx) PostExist(string) bool                                      { return false }
func (t *featureTestTx) GetPost(string) (*postsPb.SignedPost, error)                { return nil, nil }
func (t *featureTestTx) SetImage(models.Image) error                                { return nil }
func (t *featureTestTx) GetImageByName(models.ImageSize, string) ([]byte, error)    { return nil, nil }
func (t *featureTestTx) GetMediaByCID(string) ([]byte, string, error)               { return nil, "", nil }
func (t *featureTestTx) IndexMediaCID(string, string, string, string, string) error { return nil }
func (t *featureTestTx) SetUploadedFile(models.UploadedFile) error                  { return nil }
func (t *featureTestTx) SetIntroVideo(models.IntroVideo) error                      { return nil }
