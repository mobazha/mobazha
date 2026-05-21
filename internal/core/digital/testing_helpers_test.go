package digital

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"sync"
	"testing"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// In-memory database
// ---------------------------------------------------------------------------

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

type testTx struct {
	db *gorm.DB
}

func (t *testTx) Read() *gorm.DB { return t.db }

func (t *testTx) Save(i interface{}) error {
	injectTestTenantID(i)
	return t.db.Save(i).Error
}

// injectTestTenantID mimics production Tx.Save behavior: set a non-empty
// TenantID so GORM recognises the composite PK as "existing" and emits
// UPDATE (upsert) instead of INSERT.
func injectTestTenantID(i interface{}) {
	v := reflect.ValueOf(i)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	f := v.FieldByName("TenantID")
	if f.IsValid() && f.CanSet() && f.String() == "" {
		f.SetString(database.StandaloneTenantID)
	}
}

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
func (t *testTx) Migrate(m interface{}) error { return t.db.AutoMigrate(m) }
func (t *testTx) RegisterCommitHook(func())   {}

// PublicData stubs.
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

func newDigitalTestDB(t *testing.T) *testDatabase {
	t.Helper()
	db, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(
		&models.DigitalAsset{},
		&models.DigitalLicenseKey{},
		&models.LicenseActivation{},
		&models.DownloadGrant{},
		&models.DigitalDownloadLog{},
		&models.GuestOrder{},
	))
	return &testDatabase{gormDB: db}
}

// ---------------------------------------------------------------------------
// In-memory BlobStore
// ---------------------------------------------------------------------------

type memBlobStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMemBlobStore() *memBlobStore {
	return &memBlobStore{data: make(map[string][]byte)}
}

func (s *memBlobStore) Put(_ context.Context, key string, data []byte, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = append([]byte{}, data...)
	return nil
}

// PutStream mirrors the streaming contract of contracts.BlobStore by
// reading the source into memory. Tests deal with small payloads where this
// simpler implementation matches the production semantics (LocalFS uses a
// temp file + atomic rename; R2 uses S3 multipart). The tests assert
// round-trip correctness, which the in-memory copy preserves.
func (s *memBlobStore) PutStream(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	buf, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = buf
	return nil
}

func (s *memBlobStore) Get(_ context.Context, key string) (io.ReadCloser, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.data[key]
	if !ok {
		return nil, "", contracts.ErrBlobNotFound
	}
	return io.NopCloser(bytes.NewReader(d)), "application/octet-stream", nil
}

func (s *memBlobStore) Exists(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[key]
	return ok, nil
}

func (s *memBlobStore) PublicURL(_ string) string { return "" }

// ---------------------------------------------------------------------------
// Mock KeyProvider (only DigitalContentMasterKey is exercised)
// ---------------------------------------------------------------------------

type testKeyProvider struct {
	masterKey []byte
}

func newTestKeyProvider() *testKeyProvider {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return &testKeyProvider{masterKey: key}
}

func (p *testKeyProvider) DigitalContentMasterKey(_ int) ([]byte, error) {
	out := make([]byte, len(p.masterKey))
	copy(out, p.masterKey)
	return out, nil
}

func (p *testKeyProvider) EVMMasterKey() (*btcec.PrivateKey, error)     { return nil, nil }
func (p *testKeyProvider) SolanaMasterKey() (*solana.PrivateKey, error) { return nil, nil }
func (p *testKeyProvider) EscrowMasterKey() (*btcec.PrivateKey, error)  { return nil, nil }
func (p *testKeyProvider) RatingMasterKey() (*btcec.PrivateKey, error)  { return nil, nil }
func (p *testKeyProvider) TRONMasterKey() (*btcec.PrivateKey, error)    { return nil, nil }

// ---------------------------------------------------------------------------
// Mock ResolverInterface (pkgconfig.ResolverInterface)
// ---------------------------------------------------------------------------

type testFeatureResolver struct {
	enabled map[string]bool
}

func newTestFeatureResolver(flags map[string]bool) *testFeatureResolver {
	return &testFeatureResolver{enabled: flags}
}

func (r *testFeatureResolver) IsEnabled(_ context.Context, key string) bool {
	return r.enabled[key]
}

func (r *testFeatureResolver) Evaluate(_ context.Context, key string) pkgconfig.Evaluation {
	return pkgconfig.Evaluation{Enabled: r.enabled[key]}
}

func (r *testFeatureResolver) List(_ context.Context) []pkgconfig.EffectiveFeature {
	return nil
}

// ---------------------------------------------------------------------------
// Mock OrderQuerier
// ---------------------------------------------------------------------------

type testOrderQuerier struct {
	contractType  string
	listingSlug   string
	variantSKU    string
	buyerPeerID   string
	sellerPeerID  string
	paymentMethod string
	lineItems     []OrderLineItem
}

func (q *testOrderQuerier) GetOrderMetadata(_ string) (*OrderMetadata, error) {
	lineItems := q.lineItems
	if len(lineItems) == 0 {
		lineItems = []OrderLineItem{
			{ListingSlug: q.listingSlug, VariantSKU: q.variantSKU, Quantity: 1},
		}
	}
	return &OrderMetadata{
		ContractType:  q.contractType,
		BuyerPeerID:   q.buyerPeerID,
		SellerPeerID:  q.sellerPeerID,
		PaymentMethod: q.paymentMethod,
		LineItems:     lineItems,
	}, nil
}
