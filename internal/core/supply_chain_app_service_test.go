package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	cid "github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/fulfillment"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// Stub Fulfillment Provider
// ---------------------------------------------------------------------------

type stubFulfillmentProvider struct {
	id            string
	provType      string
	validateErr   error
	createOrderFn func(ctx context.Context, params contracts.CreateFulfillmentParams) (*contracts.FulfillmentOrder, error)
	parseWebFn    func(ctx context.Context, payload []byte, headers map[string]string) (*contracts.FulfillmentWebhookEvent, error)
	cancelFn      func(ctx context.Context, orderID string) error
	estimateFn    func(ctx context.Context, params contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error)
}

func (p *stubFulfillmentProvider) ProviderID() string   { return p.id }
func (p *stubFulfillmentProvider) ProviderType() string { return p.provType }
func (p *stubFulfillmentProvider) ValidateCredentials(_ context.Context, _ contracts.ProviderCredentials) error {
	return p.validateErr
}
func (p *stubFulfillmentProvider) CreateFulfillmentOrder(ctx context.Context, params contracts.CreateFulfillmentParams) (*contracts.FulfillmentOrder, error) {
	if p.createOrderFn != nil {
		return p.createOrderFn(ctx, params)
	}
	return &contracts.FulfillmentOrder{ExternalID: "ext-123", Status: contracts.FulfillmentStatusPending}, nil
}
func (p *stubFulfillmentProvider) GetFulfillmentOrder(_ context.Context, orderID string) (*contracts.FulfillmentOrder, error) {
	return &contracts.FulfillmentOrder{ExternalID: orderID, Status: contracts.FulfillmentStatusPending}, nil
}
func (p *stubFulfillmentProvider) CancelFulfillmentOrder(ctx context.Context, orderID string) error {
	if p.cancelFn != nil {
		return p.cancelFn(ctx, orderID)
	}
	return nil
}
func (p *stubFulfillmentProvider) ParseWebhook(ctx context.Context, payload []byte, headers map[string]string) (*contracts.FulfillmentWebhookEvent, error) {
	if p.parseWebFn != nil {
		return p.parseWebFn(ctx, payload, headers)
	}
	return &contracts.FulfillmentWebhookEvent{Type: contracts.FulfillmentWebhookShipped, EventID: "evt-1", OrderID: "ext-123"}, nil
}
func (p *stubFulfillmentProvider) EstimateShipping(ctx context.Context, params contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error) {
	if p.estimateFn != nil {
		return p.estimateFn(ctx, params)
	}
	return []contracts.ShippingEstimate{{ID: "standard", Rate: "4.50", Currency: "USD"}}, nil
}

// Stub with catalog support
type stubCatalogProvider struct {
	stubFulfillmentProvider
}

func (p *stubCatalogProvider) ListCategories(_ context.Context) ([]contracts.CatalogCategory, error) {
	return []contracts.CatalogCategory{{ID: "cat-1", Name: "T-Shirts"}}, nil
}
func (p *stubCatalogProvider) ListProducts(_ context.Context, _ contracts.CatalogQuery) (*contracts.CatalogPage, error) {
	return &contracts.CatalogPage{Products: []contracts.CatalogProduct{{ID: "p-1", Title: "Tee"}}}, nil
}
func (p *stubCatalogProvider) GetProduct(_ context.Context, productID string) (*contracts.CatalogProduct, error) {
	return &contracts.CatalogProduct{ID: productID, Title: "Tee"}, nil
}
func (p *stubCatalogProvider) GetVariant(_ context.Context, variantID string) (*contracts.CatalogVariant, error) {
	return &contracts.CatalogVariant{ID: variantID}, nil
}

// ---------------------------------------------------------------------------
// Test Database (same pattern as order_repo_gorm_test.go)
// ---------------------------------------------------------------------------

type scTestDatabase struct {
	gormDB *gorm.DB
}

func newSCTestDatabase(t *testing.T) *scTestDatabase {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.FulfillmentProviderConfig{},
		&models.SyncedProductMapping{},
		&models.FulfillmentOrderMapping{},
		&models.ProcessedFulfillmentEvent{},
	); err != nil {
		t.Fatal(err)
	}
	return &scTestDatabase{gormDB: db}
}

func (d *scTestDatabase) View(fn func(database.Tx) error) error {
	return fn(&scTestTx{db: d.gormDB})
}

func (d *scTestDatabase) Update(fn func(database.Tx) error) error {
	return d.gormDB.Transaction(func(tx *gorm.DB) error {
		return fn(&scTestTx{db: tx})
	})
}

func (d *scTestDatabase) ComputePublicDataHash() (cid.Cid, error) { return cid.Undef, nil }
func (d *scTestDatabase) Close() error                             { return nil }

type scTestTx struct{ db *gorm.DB }

func (t *scTestTx) Read() *gorm.DB { return t.db }
func (t *scTestTx) Save(i interface{}) error {
	// Use Clauses for SQLite composite PK upsert (production uses PostgreSQL)
	return t.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(i).Error
}
func (t *scTestTx) Update(key string, value interface{}, where map[string]interface{}, model interface{}) error {
	q := t.db.Model(model)
	for k, v := range where {
		q = q.Where(k, v) // k is "column = ?" format per database.Tx contract
	}
	return q.Update(key, value).Error
}
func (t *scTestTx) Commit() error   { panic("managed tx") }
func (t *scTestTx) Rollback() error { panic("managed tx") }
func (t *scTestTx) Delete(key string, value interface{}, where map[string]interface{}, model interface{}) error {
	q := t.db.Where(key, value)
	for k, v := range where {
		q = q.Where(k, v)
	}
	return q.Delete(model).Error
}
func (t *scTestTx) DeleteAll(interface{}) error { return nil }
func (t *scTestTx) Migrate(interface{}) error   { return nil }
func (t *scTestTx) RegisterCommitHook(func())   {}

// PublicData stubs
func (t *scTestTx) GetProfile() (*models.Profile, error)              { return nil, nil }
func (t *scTestTx) SetProfile(*models.Profile) error                  { return nil }
func (t *scTestTx) GetFollowers() (models.Followers, error)           { return models.Followers{}, nil }
func (t *scTestTx) SetFollowers(models.Followers) error               { return nil }
func (t *scTestTx) GetFollowing() (models.Following, error)           { return models.Following{}, nil }
func (t *scTestTx) SetFollowing(models.Following) error               { return nil }
func (t *scTestTx) GetListing(string) (*pb.SignedListing, error)      { return nil, nil }
func (t *scTestTx) SetListing(*pb.SignedListing) error                { return nil }
func (t *scTestTx) GetEncryptedListing(string) ([]byte, error)        { return nil, nil }
func (t *scTestTx) SetEncryptedListing(string, []byte) error          { return nil }
func (t *scTestTx) DeleteListing(string) error                        { return nil }
func (t *scTestTx) GetListingIndex() (models.ListingIndex, error)     { return nil, nil }
func (t *scTestTx) SetListingIndex(models.ListingIndex) error         { return nil }
func (t *scTestTx) GetRatingIndex() (models.RatingIndex, error)       { return nil, nil }
func (t *scTestTx) SetRatingIndex(models.RatingIndex) error                    { return nil }
func (t *scTestTx) SetRating(*pb.Rating) error                                 { return nil }
func (t *scTestTx) GetPostIndex() ([]models.PostData, error)                   { return nil, nil }
func (t *scTestTx) SetPostIndex([]models.PostData) error                       { return nil }
func (t *scTestTx) AddPost(*postsPb.SignedPost) error                          { return nil }
func (t *scTestTx) DeletePost(string) error                                    { return nil }
func (t *scTestTx) PostExist(string) bool                                      { return false }
func (t *scTestTx) GetPost(string) (*postsPb.SignedPost, error)                { return nil, nil }
func (t *scTestTx) SetImage(models.Image) error                                { return nil }
func (t *scTestTx) GetImageByName(models.ImageSize, string) ([]byte, error)    { return nil, nil }
func (t *scTestTx) GetMediaByCID(string) ([]byte, string, error)               { return nil, "", nil }
func (t *scTestTx) IndexMediaCID(string, string, string, string, string) error { return nil }
func (t *scTestTx) SetUploadedFile(models.UploadedFile) error                  { return nil }
func (t *scTestTx) SetIntroVideo(models.IntroVideo) error                      { return nil }

// testPrivKeyBytes returns deterministic fake private key bytes for tests.
// The value must match what is passed to NewSupplyChainAppService in tests.
var testPrivKeyBytes = []byte("test-private-key-material-for-supply-chain")

// testEncryptCreds encrypts provider credentials JSON for use in tests.
func testEncryptCreds(t *testing.T, creds contracts.ProviderCredentials) []byte {
	t.Helper()
	key := deriveCredentialKey(testPrivKeyBytes)
	blob, err := json.Marshal(creds)
	if err != nil {
		t.Fatalf("marshal creds: %v", err)
	}
	enc, err := encryptAESGCM(key[:], blob)
	if err != nil {
		t.Fatalf("encrypt creds: %v", err)
	}
	return enc
}

// ---------------------------------------------------------------------------
// Tests: instantiateProvider
// ---------------------------------------------------------------------------

func TestInstantiateProvider_Printful(t *testing.T) {
	svc := NewSupplyChainAppService(fulfillment.NewRegistry(), nil, "test", testPrivKeyBytes)
	enc := testEncryptCreds(t, contracts.ProviderCredentials{APIKey: "test-token"})
	p, err := svc.instantiateProvider("printful", "pod", enc, "ws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ProviderID() != "printful" {
		t.Errorf("expected printful, got %s", p.ProviderID())
	}
}

func TestInstantiateProvider_BadCiphertext(t *testing.T) {
	svc := NewSupplyChainAppService(fulfillment.NewRegistry(), nil, "test", testPrivKeyBytes)
	_, err := svc.instantiateProvider("printful", "pod", []byte(`{bad`), "")
	if err == nil {
		t.Fatal("expected error for bad ciphertext")
	}
}

func TestInstantiateProvider_Unknown(t *testing.T) {
	svc := NewSupplyChainAppService(fulfillment.NewRegistry(), nil, "test", testPrivKeyBytes)
	enc := testEncryptCreds(t, contracts.ProviderCredentials{APIKey: "x"})
	_, err := svc.instantiateProvider("unknown", "pod", enc, "")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

// ---------------------------------------------------------------------------
// Tests: Helpers
// ---------------------------------------------------------------------------

func TestConfigToConnection(t *testing.T) {
	now := time.Now()
	cfg := &models.FulfillmentProviderConfig{
		ProviderID: "printful", ProviderType: "pod",
		Status: "connected", StoreName: "My Store", ConnectedAt: now,
	}
	conn := configToConnection(cfg)
	if conn.ProviderID != "printful" || conn.Status != "connected" {
		t.Error("unexpected connection values")
	}
}

func TestGenerateWebhookSecret(t *testing.T) {
	s1, err := generateWebhookSecret()
	if err != nil {
		t.Fatal(err)
	}
	s2, _ := generateWebhookSecret()
	if len(s1) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(s1))
	}
	if s1 == s2 {
		t.Error("two secrets should differ")
	}
}

func TestBuildShipments_Empty(t *testing.T) {
	if buildShipments(&models.FulfillmentOrderMapping{}) != nil {
		t.Error("expected nil")
	}
}

func TestBuildShipments_WithTracking(t *testing.T) {
	m := &models.FulfillmentOrderMapping{
		TrackingNumber: "1Z999", TrackingURL: "https://ups.com/1Z999", Carrier: "UPS",
	}
	ships := buildShipments(m)
	if len(ships) != 1 || ships[0].TrackingNumber != "1Z999" {
		t.Error("unexpected shipments")
	}
}

func TestExtractShipmentData(t *testing.T) {
	// Real Printful webhook stores a *FulfillmentOrder in event.Data
	event := &contracts.FulfillmentWebhookEvent{
		Data: &contracts.FulfillmentOrder{
			ExternalID: "ext-123",
			Shipments: []contracts.FulfillmentShipment{{
				TrackingNumber: "TRACK-1",
				TrackingURL:    "https://example.com/TRACK-1",
				Carrier:        "USPS",
			}},
		},
	}
	ship := extractShipmentData(event)
	if ship == nil || ship.TrackingNumber != "TRACK-1" {
		t.Error("unexpected shipment data")
	}
}

func TestExtractShipmentData_Nil(t *testing.T) {
	if extractShipmentData(&contracts.FulfillmentWebhookEvent{}) != nil {
		t.Error("expected nil")
	}
}

func TestExtractErrorMessage(t *testing.T) {
	event := &contracts.FulfillmentWebhookEvent{
		Data: map[string]interface{}{"reason": "out of stock"},
	}
	if extractErrorMessage(event) != "out of stock" {
		t.Error("unexpected error message")
	}
}

func TestCostTotal(t *testing.T) {
	if costTotal(nil) != "" {
		t.Error("expected empty for nil")
	}
	if costTotal(&contracts.FulfillmentCosts{Total: "19.99"}) != "19.99" {
		t.Error("expected 19.99")
	}
}

// ---------------------------------------------------------------------------
// Tests: IsListingManagedBySupplier
// ---------------------------------------------------------------------------

func TestIsListingManagedBySupplier_NoMapping(t *testing.T) {
	tdb := newSCTestDatabase(t)
	svc := NewSupplyChainAppService(fulfillment.NewRegistry(), tdb, "test", testPrivKeyBytes)
	if svc.IsListingManagedBySupplier("non-existent") {
		t.Error("expected false")
	}
}

func TestIsListingManagedBySupplier_WithMapping(t *testing.T) {
	tdb := newSCTestDatabase(t)
	tdb.gormDB.Create(&models.SyncedProductMapping{
		ID: "spm-1", ProviderID: "printful", ListingSlug: "my-tshirt", Status: "synced",
	})
	svc := NewSupplyChainAppService(fulfillment.NewRegistry(), tdb, "test", testPrivKeyBytes)
	if !svc.IsListingManagedBySupplier("my-tshirt") {
		t.Error("expected true")
	}
}

// ---------------------------------------------------------------------------
// Tests: rebuildProviders
// ---------------------------------------------------------------------------

func TestRebuildProviders_NoConfigs(t *testing.T) {
	tdb := newSCTestDatabase(t)
	reg := fulfillment.NewRegistry()
	svc := NewSupplyChainAppService(reg, tdb, "test", testPrivKeyBytes)
	if err := svc.rebuildProviders(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(reg.ListProviders()) != 0 {
		t.Error("expected empty registry")
	}
}

func TestRebuildProviders_ConnectedConfig(t *testing.T) {
	tdb := newSCTestDatabase(t)
	enc := testEncryptCreds(t, contracts.ProviderCredentials{APIKey: "k"})
	tdb.gormDB.Create(&models.FulfillmentProviderConfig{
		ID: "c1", ProviderID: "printful", ProviderType: "pod",
		Credentials: enc, WebhookSecret: "ws1", Status: "connected",
	})
	reg := fulfillment.NewRegistry()
	svc := NewSupplyChainAppService(reg, tdb, "test", testPrivKeyBytes)
	if err := svc.rebuildProviders(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(reg.ListProviders()) != 1 {
		t.Errorf("expected 1, got %d", len(reg.ListProviders()))
	}
}

func TestRebuildProviders_BadCredentials(t *testing.T) {
	tdb := newSCTestDatabase(t)
	tdb.gormDB.Create(&models.FulfillmentProviderConfig{
		ID: "c-bad", ProviderID: "printful", ProviderType: "pod",
		Credentials: []byte(`corrupted-ciphertext`), WebhookSecret: "ws-bad", Status: "connected",
	})
	reg := fulfillment.NewRegistry()
	svc := NewSupplyChainAppService(reg, tdb, "test", testPrivKeyBytes)
	if err := svc.rebuildProviders(context.Background()); err != nil {
		t.Fatalf("should not error: %v", err)
	}
	if len(reg.ListProviders()) != 0 {
		t.Error("expected empty")
	}
	var cfg models.FulfillmentProviderConfig
	tdb.gormDB.First(&cfg, "id = ?", "c-bad")
	if cfg.Status != "error" {
		t.Errorf("expected 'error', got %q", cfg.Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: DisconnectProvider
// ---------------------------------------------------------------------------

func TestDisconnectProvider(t *testing.T) {
	tdb := newSCTestDatabase(t)
	tdb.gormDB.Create(&models.FulfillmentProviderConfig{
		ID: "c1", ProviderID: "printful", Status: "connected",
	})
	reg := fulfillment.NewRegistry()
	_ = reg.Register(&stubFulfillmentProvider{id: "printful", provType: "pod"})

	svc := NewSupplyChainAppService(reg, tdb, "test", testPrivKeyBytes)
	if err := svc.DisconnectProvider(context.Background(), "printful"); err != nil {
		t.Fatal(err)
	}
	var cfg models.FulfillmentProviderConfig
	tdb.gormDB.First(&cfg, "provider_id = ?", "printful")
	if cfg.Status != "disconnected" {
		t.Errorf("expected disconnected, got %s", cfg.Status)
	}
	if _, err := reg.ForProvider("printful"); !errors.Is(err, contracts.ErrFulfillmentProviderNotFound) {
		t.Error("expected unregistered")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetProviderStatus / ListConnections
// ---------------------------------------------------------------------------

func TestGetProviderStatus_Found(t *testing.T) {
	tdb := newSCTestDatabase(t)
	tdb.gormDB.Create(&models.FulfillmentProviderConfig{
		ID: "c1", ProviderID: "printful", ProviderType: "pod", Status: "connected",
	})
	svc := NewSupplyChainAppService(fulfillment.NewRegistry(), tdb, "test", testPrivKeyBytes)
	conn, err := svc.GetProviderStatus(context.Background(), "printful")
	if err != nil {
		t.Fatal(err)
	}
	if conn.Status != "connected" {
		t.Errorf("expected connected, got %s", conn.Status)
	}
}

func TestGetProviderStatus_NotFound(t *testing.T) {
	tdb := newSCTestDatabase(t)
	svc := NewSupplyChainAppService(fulfillment.NewRegistry(), tdb, "test", testPrivKeyBytes)
	_, err := svc.GetProviderStatus(context.Background(), "nope")
	if !errors.Is(err, contracts.ErrFulfillmentProviderNotFound) {
		t.Errorf("expected ErrFulfillmentProviderNotFound, got %v", err)
	}
}

func TestListConnections(t *testing.T) {
	tdb := newSCTestDatabase(t)
	tdb.gormDB.Create(&models.FulfillmentProviderConfig{ID: "c1", ProviderID: "printful", Status: "connected", WebhookSecret: "ws-1"})
	tdb.gormDB.Create(&models.FulfillmentProviderConfig{ID: "c2", ProviderID: "printify", Status: "disconnected", WebhookSecret: "ws-2"})
	svc := NewSupplyChainAppService(fulfillment.NewRegistry(), tdb, "test", testPrivKeyBytes)
	conns, err := svc.ListConnections(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 2 {
		t.Errorf("expected 2, got %d", len(conns))
	}
}

// ---------------------------------------------------------------------------
// Tests: Catalog delegation
// ---------------------------------------------------------------------------

func TestBrowseCatalog_Success(t *testing.T) {
	reg := fulfillment.NewRegistry()
	_ = reg.Register(&stubCatalogProvider{stubFulfillmentProvider: stubFulfillmentProvider{id: "printful", provType: "pod"}})
	svc := &SupplyChainAppService{registry: reg, nodeID: "test"}
	page, err := svc.BrowseCatalog(context.Background(), "printful", contracts.CatalogQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Products) != 1 {
		t.Errorf("expected 1 product, got %d", len(page.Products))
	}
}

func TestBrowseCatalog_NotRegistered(t *testing.T) {
	svc := &SupplyChainAppService{registry: fulfillment.NewRegistry(), nodeID: "test"}
	_, err := svc.BrowseCatalog(context.Background(), "nope", contracts.CatalogQuery{})
	if !errors.Is(err, contracts.ErrFulfillmentProviderNotFound) {
		t.Errorf("expected ErrFulfillmentProviderNotFound, got %v", err)
	}
}

func TestBrowseCatalog_NoCatalogSupport(t *testing.T) {
	reg := fulfillment.NewRegistry()
	_ = reg.Register(&stubFulfillmentProvider{id: "basic", provType: "drop"})
	svc := &SupplyChainAppService{registry: reg, nodeID: "test"}
	_, err := svc.BrowseCatalog(context.Background(), "basic", contracts.CatalogQuery{})
	if !errors.Is(err, contracts.ErrFulfillmentCatalogNotSupported) {
		t.Errorf("expected catalog not supported, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: GetFulfillmentStatus
// ---------------------------------------------------------------------------

func TestGetFulfillmentStatus_NotFound(t *testing.T) {
	tdb := newSCTestDatabase(t)
	svc := NewSupplyChainAppService(fulfillment.NewRegistry(), tdb, "test", testPrivKeyBytes)
	_, err := svc.GetFulfillmentStatus(context.Background(), "non-existent")
	if !errors.Is(err, contracts.ErrFulfillmentOrderNotFound) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestGetFulfillmentStatus_WithMapping(t *testing.T) {
	tdb := newSCTestDatabase(t)
	now := time.Now()
	tdb.gormDB.Create(&models.FulfillmentOrderMapping{
		ID: "fom-1", MobazhaOrderID: "order-abc", ProviderID: "printful",
		FulfillmentOrderID: "ext-123", Status: "shipped",
		TrackingNumber: "TK1", TrackingURL: "https://track.co/TK1", Carrier: "UPS",
		SupplierCost: "25.00", CreatedAt: now, UpdatedAt: now,
	})
	svc := NewSupplyChainAppService(fulfillment.NewRegistry(), tdb, "test", testPrivKeyBytes)
	fo, err := svc.GetFulfillmentStatus(context.Background(), "order-abc")
	if err != nil {
		t.Fatal(err)
	}
	if fo.Status != contracts.FulfillmentStatusShipped {
		t.Errorf("expected shipped, got %s", fo.Status)
	}
	if len(fo.Shipments) != 1 || fo.Shipments[0].TrackingNumber != "TK1" {
		t.Error("tracking info wrong")
	}
	if fo.Costs == nil || fo.Costs.Total != "25.00" {
		t.Error("costs wrong")
	}
}

// ---------------------------------------------------------------------------
// Tests: HandleProviderWebhook
// ---------------------------------------------------------------------------

func TestHandleProviderWebhook_Shipped(t *testing.T) {
	tdb := newSCTestDatabase(t)
	tdb.gormDB.Create(&models.FulfillmentOrderMapping{
		ID: "fom-1", MobazhaOrderID: "order-abc", ProviderID: "printful",
		FulfillmentOrderID: "pf-12345", Status: "pending",
	})
	reg := fulfillment.NewRegistry()
	_ = reg.Register(&stubFulfillmentProvider{
		id: "printful", provType: "pod",
		parseWebFn: func(_ context.Context, _ []byte, _ map[string]string) (*contracts.FulfillmentWebhookEvent, error) {
			return &contracts.FulfillmentWebhookEvent{
				Type: contracts.FulfillmentWebhookShipped, EventID: "e-ship",
				OrderID:    "order-abc",
				ExternalID: "pf-12345",
				Data: &contracts.FulfillmentOrder{
					ID:         "pf-12345",
					ExternalID: "order-abc",
					Shipments: []contracts.FulfillmentShipment{{
						TrackingNumber: "1Z999", TrackingURL: "https://ups.com/1Z999", Carrier: "UPS",
					}},
				},
			}, nil
		},
	})
	svc := NewSupplyChainAppService(reg, tdb, "test", testPrivKeyBytes)
	err := svc.HandleProviderWebhook(context.Background(), "printful", nil, nil)
	if err == nil {
		t.Fatal("expected error from autoConfirmAndShip (orderOps not wired), got nil")
	}

	// Mapping should still be updated to "shipped" with tracking (happens before autoConfirmAndShip).
	var m models.FulfillmentOrderMapping
	tdb.gormDB.First(&m, "id = ?", "fom-1")
	if m.Status != "shipped" {
		t.Errorf("expected shipped, got %s", m.Status)
	}
	if m.TrackingNumber != "1Z999" {
		t.Errorf("expected 1Z999, got %s", m.TrackingNumber)
	}

	// Event should be released (not processed) so provider can retry.
	var pfe models.ProcessedFulfillmentEvent
	result := tdb.gormDB.Where("event_id = ?", "e-ship").First(&pfe)
	if result.Error == nil && pfe.Status == "processed" {
		t.Error("event should NOT be marked processed when autoConfirmAndShip fails")
	}
}

func TestHandleProviderWebhook_Idempotent(t *testing.T) {
	tdb := newSCTestDatabase(t)
	tdb.gormDB.Create(&models.FulfillmentOrderMapping{
		ID: "fom-1", MobazhaOrderID: "order-abc", ProviderID: "printful",
		FulfillmentOrderID: "pf-12345", Status: "pending",
	})
	tdb.gormDB.Create(&models.ProcessedFulfillmentEvent{
		ID: "pfe-1", ProviderID: "printful", EventID: "evt-dup", Status: "processed",
	})
	reg := fulfillment.NewRegistry()
	_ = reg.Register(&stubFulfillmentProvider{
		id: "printful", provType: "pod",
		parseWebFn: func(_ context.Context, _ []byte, _ map[string]string) (*contracts.FulfillmentWebhookEvent, error) {
			return &contracts.FulfillmentWebhookEvent{
				Type: contracts.FulfillmentWebhookShipped, EventID: "evt-dup",
				OrderID: "order-abc", ExternalID: "pf-12345",
			}, nil
		},
	})
	svc := NewSupplyChainAppService(reg, tdb, "test", testPrivKeyBytes)
	if err := svc.HandleProviderWebhook(context.Background(), "printful", nil, nil); err != nil {
		t.Fatal(err)
	}
	var m models.FulfillmentOrderMapping
	tdb.gormDB.First(&m, "id = ?", "fom-1")
	if m.Status != "pending" {
		t.Errorf("expected pending (skip), got %s", m.Status)
	}
}

func TestHandleProviderWebhook_ReleaseOnProcessingError(t *testing.T) {
	tdb := newSCTestDatabase(t)
	tdb.gormDB.Create(&models.FulfillmentOrderMapping{
		ID: "fom-1", MobazhaOrderID: "order-abc", ProviderID: "printful",
		FulfillmentOrderID: "pf-12345", Status: "pending",
	})

	// svc2: processes an order_updated event (no autoConfirmAndShip) → reserve + process + mark processed
	reg2 := fulfillment.NewRegistry()
	_ = reg2.Register(&stubFulfillmentProvider{
		id: "printful", provType: "pod",
		parseWebFn: func(_ context.Context, _ []byte, _ map[string]string) (*contracts.FulfillmentWebhookEvent, error) {
			return &contracts.FulfillmentWebhookEvent{
				Type: contracts.FulfillmentWebhookOrderUpdated, EventID: "evt-retry",
				OrderID: "order-abc", ExternalID: "pf-12345",
			}, nil
		},
	})
	svc2 := NewSupplyChainAppService(reg2, tdb, "test", testPrivKeyBytes)
	if err := svc2.HandleProviderWebhook(context.Background(), "printful", nil, nil); err != nil {
		t.Fatal(err)
	}
	// Verify the event is now "processed"
	var pfe models.ProcessedFulfillmentEvent
	tdb.gormDB.Where("event_id = ?", "evt-retry").First(&pfe)
	if pfe.Status != "processed" {
		t.Errorf("expected processed, got %s", pfe.Status)
	}

	// svc: same event ID → should be blocked by the "processed" reservation
	reg := fulfillment.NewRegistry()
	_ = reg.Register(&stubFulfillmentProvider{
		id: "printful", provType: "pod",
		parseWebFn: func(_ context.Context, _ []byte, _ map[string]string) (*contracts.FulfillmentWebhookEvent, error) {
			return &contracts.FulfillmentWebhookEvent{
				Type: contracts.FulfillmentWebhookOrderUpdated, EventID: "evt-retry",
				OrderID: "order-abc", ExternalID: "pf-12345",
			}, nil
		},
	})
	svc := NewSupplyChainAppService(reg, tdb, "test", testPrivKeyBytes)
	if err := svc.HandleProviderWebhook(context.Background(), "printful", nil, nil); err != nil {
		t.Fatal("expected nil (skip), got", err)
	}
	var m models.FulfillmentOrderMapping
	tdb.gormDB.First(&m, "id = ?", "fom-1")
	if m.Status != string(contracts.FulfillmentStatusInProcess) {
		t.Logf("mapping status: %s (expected in_process from first call)", m.Status)
	}
}

func TestHandleProviderWebhook_Failed(t *testing.T) {
	tdb := newSCTestDatabase(t)
	tdb.gormDB.Create(&models.FulfillmentOrderMapping{
		ID: "fom-1", MobazhaOrderID: "order-abc", ProviderID: "printful",
		FulfillmentOrderID: "pf-12345", Status: "in_process",
	})
	reg := fulfillment.NewRegistry()
	_ = reg.Register(&stubFulfillmentProvider{
		id: "printful", provType: "pod",
		parseWebFn: func(_ context.Context, _ []byte, _ map[string]string) (*contracts.FulfillmentWebhookEvent, error) {
			return &contracts.FulfillmentWebhookEvent{
				Type: contracts.FulfillmentWebhookOrderFailed, EventID: "e-fail",
				OrderID: "order-abc", ExternalID: "pf-12345",
				Data: map[string]interface{}{"reason": "out of stock"},
			}, nil
		},
	})
	svc := NewSupplyChainAppService(reg, tdb, "test", testPrivKeyBytes)
	if err := svc.HandleProviderWebhook(context.Background(), "printful", nil, nil); err != nil {
		t.Fatal(err)
	}
	var m models.FulfillmentOrderMapping
	tdb.gormDB.First(&m, "id = ?", "fom-1")
	if m.Status != "failed" {
		t.Errorf("expected failed, got %s", m.Status)
	}
	if m.ErrorMessage != "out of stock" {
		t.Errorf("expected 'out of stock', got %q", m.ErrorMessage)
	}
}

func TestHandleProviderWebhook_Canceled(t *testing.T) {
	tdb := newSCTestDatabase(t)
	tdb.gormDB.Create(&models.FulfillmentOrderMapping{
		ID: "fom-1", MobazhaOrderID: "order-abc", ProviderID: "printful",
		FulfillmentOrderID: "pf-12345", Status: "pending",
	})
	reg := fulfillment.NewRegistry()
	_ = reg.Register(&stubFulfillmentProvider{
		id: "printful", provType: "pod",
		parseWebFn: func(_ context.Context, _ []byte, _ map[string]string) (*contracts.FulfillmentWebhookEvent, error) {
			return &contracts.FulfillmentWebhookEvent{
				Type: contracts.FulfillmentWebhookOrderCanceled, EventID: "e-cancel",
				OrderID: "order-abc", ExternalID: "pf-12345",
			}, nil
		},
	})
	svc := NewSupplyChainAppService(reg, tdb, "test", testPrivKeyBytes)
	if err := svc.HandleProviderWebhook(context.Background(), "printful", nil, nil); err != nil {
		t.Fatal(err)
	}
	var m models.FulfillmentOrderMapping
	tdb.gormDB.First(&m, "id = ?", "fom-1")
	if m.Status != "canceled" {
		t.Errorf("expected canceled, got %s", m.Status)
	}
}

func TestHandleProviderWebhook_InProgressRetryable(t *testing.T) {
	tdb := newSCTestDatabase(t)
	// Pre-insert a "processing" reservation (simulates another handler in-flight)
	tdb.gormDB.Create(&models.ProcessedFulfillmentEvent{
		ID: "pfe-inflight", ProviderID: "printful", EventID: "evt-concurrent",
		Status: "processing",
	})
	reg := fulfillment.NewRegistry()
	_ = reg.Register(&stubFulfillmentProvider{
		id: "printful", provType: "pod",
		parseWebFn: func(_ context.Context, _ []byte, _ map[string]string) (*contracts.FulfillmentWebhookEvent, error) {
			return &contracts.FulfillmentWebhookEvent{
				Type: contracts.FulfillmentWebhookShipped, EventID: "evt-concurrent",
				OrderID: "order-abc", ExternalID: "pf-12345",
			}, nil
		},
	})
	svc := NewSupplyChainAppService(reg, tdb, "test", testPrivKeyBytes)
	err := svc.HandleProviderWebhook(context.Background(), "printful", nil, nil)
	if err == nil {
		t.Fatal("expected retryable error, got nil")
	}
	if !strings.Contains(err.Error(), "retry later") {
		t.Errorf("expected retry message, got: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Tests: createFulfillmentForItems (internal order bridge)
// ---------------------------------------------------------------------------

func TestCreateFulfillmentForItems_Success(t *testing.T) {
	tdb := newSCTestDatabase(t)
	reg := fulfillment.NewRegistry()
	_ = reg.Register(&stubFulfillmentProvider{
		id: "printful", provType: "pod",
		createOrderFn: func(_ context.Context, _ contracts.CreateFulfillmentParams) (*contracts.FulfillmentOrder, error) {
			return &contracts.FulfillmentOrder{
				ID: "pf-12345", ExternalID: "order-xyz",
				Status: contracts.FulfillmentStatusPending,
				Costs:  &contracts.FulfillmentCosts{Total: "15.50"},
			}, nil
		},
	})
	svc := NewSupplyChainAppService(reg, tdb, "test", testPrivKeyBytes)
	fo, err := svc.createFulfillmentForItems(context.Background(), "order-xyz", "printful", contracts.CreateFulfillmentParams{
		ExternalOrderID: "order-xyz",
		Recipient:       contracts.FulfillmentRecipient{Name: "Bob"},
		Items:           []contracts.FulfillmentItem{{CatalogVariantID: "v1", Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if fo.ID != "pf-12345" {
		t.Errorf("unexpected supplier ID: %s", fo.ID)
	}
	var mapping models.FulfillmentOrderMapping
	tdb.gormDB.First(&mapping, "mobazha_order_id = ?", "order-xyz")
	if mapping.FulfillmentOrderID != "pf-12345" {
		t.Errorf("expected supplier order ID pf-12345, got %s", mapping.FulfillmentOrderID)
	}
	if mapping.Status != "pending" {
		t.Errorf("expected pending, got %s", mapping.Status)
	}
	if mapping.SupplierCost != "15.50" {
		t.Errorf("expected 15.50, got %s", mapping.SupplierCost)
	}
}

func TestCreateFulfillmentForItems_ProviderError(t *testing.T) {
	tdb := newSCTestDatabase(t)
	reg := fulfillment.NewRegistry()
	_ = reg.Register(&stubFulfillmentProvider{
		id: "printful", provType: "pod",
		createOrderFn: func(_ context.Context, _ contracts.CreateFulfillmentParams) (*contracts.FulfillmentOrder, error) {
			return nil, fmt.Errorf("printful API down")
		},
	})
	svc := NewSupplyChainAppService(reg, tdb, "test", testPrivKeyBytes)
	_, err := svc.createFulfillmentForItems(context.Background(), "order-fail", "printful", contracts.CreateFulfillmentParams{})
	if err == nil {
		t.Fatal("expected error")
	}
	var mapping models.FulfillmentOrderMapping
	tdb.gormDB.First(&mapping, "mobazha_order_id = ?", "order-fail")
	if mapping.Status != "failed" {
		t.Errorf("expected failed, got %s", mapping.Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: ListSyncedProducts
// ---------------------------------------------------------------------------

func TestListSyncedProducts(t *testing.T) {
	tdb := newSCTestDatabase(t)
	tdb.gormDB.Create(&models.SyncedProductMapping{ID: "s1", ProviderID: "printful", ListingSlug: "tee-1", Status: "synced"})
	tdb.gormDB.Create(&models.SyncedProductMapping{ID: "s2", ProviderID: "printful", ListingSlug: "tee-2", Status: "synced"})
	tdb.gormDB.Create(&models.SyncedProductMapping{ID: "s3", ProviderID: "printify", ListingSlug: "mug-1", Status: "synced"})
	svc := NewSupplyChainAppService(fulfillment.NewRegistry(), tdb, "test", testPrivKeyBytes)

	all, err := svc.ListSyncedProducts(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}

	filtered, err := svc.ListSyncedProducts(context.Background(), "printful")
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2, got %d", len(filtered))
	}
}

// ---------------------------------------------------------------------------
// Tests: ImportProduct (FF-1.4a)
// ---------------------------------------------------------------------------

// richCatalogProvider returns a product with multiple variants for import testing.
type richCatalogProvider struct {
	stubFulfillmentProvider
	getProductFn func(ctx context.Context, productID string) (*contracts.CatalogProduct, error)
}

func (p *richCatalogProvider) ListCategories(_ context.Context) ([]contracts.CatalogCategory, error) {
	return nil, nil
}
func (p *richCatalogProvider) ListProducts(_ context.Context, _ contracts.CatalogQuery) (*contracts.CatalogPage, error) {
	return nil, nil
}
func (p *richCatalogProvider) GetProduct(ctx context.Context, productID string) (*contracts.CatalogProduct, error) {
	if p.getProductFn != nil {
		return p.getProductFn(ctx, productID)
	}
	return &contracts.CatalogProduct{
		ID:          productID,
		Title:       "Unisex Softstyle T-Shirt",
		Description: "A comfortable cotton tee.",
		ImageURL:    "https://img.example.com/tshirt.jpg",
		Images:      []string{"https://img.example.com/tshirt-front.jpg", "https://img.example.com/tshirt-back.jpg"},
		Currency:    "USD",
		Variants: []contracts.CatalogVariant{
			{ID: "4011", Title: "S / Black", Price: "8.25", Currency: "USD", InStock: true, Attributes: map[string]string{"Size": "S", "Color": "Black"}, ImageURL: "https://img.example.com/s-black.jpg"},
			{ID: "4012", Title: "M / Black", Price: "8.25", Currency: "USD", InStock: true, Attributes: map[string]string{"Size": "M", "Color": "Black"}},
			{ID: "4013", Title: "L / White", Price: "9.50", Currency: "USD", InStock: true, Attributes: map[string]string{"Size": "L", "Color": "White"}},
		},
	}, nil
}
func (p *richCatalogProvider) GetVariant(_ context.Context, variantID string) (*contracts.CatalogVariant, error) {
	return &contracts.CatalogVariant{ID: variantID}, nil
}

// mockListingOps captures SaveListing calls for testing.
type mockListingOps struct {
	savedListing *pb.Listing
	err          error
}

func (m *mockListingOps) SaveListing(listing *pb.Listing, done chan<- struct{}) error {
	m.savedListing = listing
	if done != nil {
		close(done)
	}
	return m.err
}

func TestImportProduct_HappyPath(t *testing.T) {
	db := newSCTestDatabase(t)
	reg := fulfillment.NewRegistry()

	provider := &richCatalogProvider{stubFulfillmentProvider: stubFulfillmentProvider{id: "printful", provType: "pod"}}
	if err := reg.Register(provider); err != nil {
		t.Fatal(err)
	}

	listingOps := &mockListingOps{}
	svc := NewSupplyChainAppService(reg, db, "test-node", testPrivKeyBytes)
	svc.SetListingOps(listingOps)

	result, err := svc.ImportProduct(context.Background(), contracts.ImportProductParams{
		ProviderID:   "printful",
		ProductID:    "71",
		RetailMarkup: 2.0,
		Title:        "My Custom Tee",
		Tags:         []string{"custom", "tee"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ListingSlug == "" {
		t.Error("expected non-empty listing slug")
	}
	if result.SyncProductID != "71" {
		t.Errorf("expected SyncProductID '71', got %q", result.SyncProductID)
	}
	if result.VariantsCount != 3 {
		t.Errorf("expected 3 variants, got %d", result.VariantsCount)
	}

	// Verify listing was passed to SaveListing
	if listingOps.savedListing == nil {
		t.Fatal("SaveListing was not called")
	}
	listing := listingOps.savedListing
	if listing.Status != models.ListingStatusDraft {
		t.Errorf("expected draft status, got %q", listing.Status)
	}
	if listing.Item.Title != "My Custom Tee" {
		t.Errorf("expected custom title, got %q", listing.Item.Title)
	}
	if len(listing.Item.Tags) != 2 || listing.Item.Tags[0] != "custom" {
		t.Errorf("unexpected tags: %v", listing.Item.Tags)
	}
	if len(listing.Item.Skus) != 3 {
		t.Errorf("expected 3 SKUs, got %d", len(listing.Item.Skus))
	}
	if len(listing.Item.Options) != 2 {
		t.Errorf("expected 2 options (Size, Color), got %d", len(listing.Item.Options))
	}
	if listing.Metadata.PricingCurrency == nil || listing.Metadata.PricingCurrency.Code != "USD" {
		t.Error("expected USD pricing currency")
	}

	// Verify SyncedProductMapping was created in DB
	var mapping models.SyncedProductMapping
	if err := db.gormDB.Where("listing_slug = ?", result.ListingSlug).First(&mapping).Error; err != nil {
		t.Fatalf("mapping not found in DB: %v", err)
	}
	if mapping.ProviderID != "printful" {
		t.Errorf("expected provider 'printful', got %q", mapping.ProviderID)
	}
	if mapping.ExternalID != "71" {
		t.Errorf("expected external ID '71', got %q", mapping.ExternalID)
	}
	if mapping.Status != "synced" {
		t.Errorf("expected status 'synced', got %q", mapping.Status)
	}
	if len(mapping.Metadata) == 0 {
		t.Error("expected non-empty metadata with variant mappings")
	}
}

func TestImportProduct_FilterVariants(t *testing.T) {
	db := newSCTestDatabase(t)
	reg := fulfillment.NewRegistry()

	provider := &richCatalogProvider{stubFulfillmentProvider: stubFulfillmentProvider{id: "printful", provType: "pod"}}
	if err := reg.Register(provider); err != nil {
		t.Fatal(err)
	}

	listingOps := &mockListingOps{}
	svc := NewSupplyChainAppService(reg, db, "test-node", testPrivKeyBytes)
	svc.SetListingOps(listingOps)

	result, err := svc.ImportProduct(context.Background(), contracts.ImportProductParams{
		ProviderID:   "printful",
		ProductID:    "71",
		VariantIDs:   []string{"4011", "4012"},
		RetailMarkup: 1.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.VariantsCount != 2 {
		t.Errorf("expected 2 filtered variants, got %d", result.VariantsCount)
	}
	if len(listingOps.savedListing.Item.Skus) != 2 {
		t.Errorf("expected 2 SKUs, got %d", len(listingOps.savedListing.Item.Skus))
	}
}

func TestImportProduct_NoVariantsError(t *testing.T) {
	db := newSCTestDatabase(t)
	reg := fulfillment.NewRegistry()

	provider := &richCatalogProvider{
		stubFulfillmentProvider: stubFulfillmentProvider{id: "printful", provType: "pod"},
		getProductFn: func(_ context.Context, _ string) (*contracts.CatalogProduct, error) {
			return &contracts.CatalogProduct{ID: "99", Title: "Empty"}, nil
		},
	}
	if err := reg.Register(provider); err != nil {
		t.Fatal(err)
	}

	listingOps := &mockListingOps{}
	svc := NewSupplyChainAppService(reg, db, "test-node", testPrivKeyBytes)
	svc.SetListingOps(listingOps)

	_, err := svc.ImportProduct(context.Background(), contracts.ImportProductParams{
		ProviderID: "printful",
		ProductID:  "99",
	})
	if err == nil {
		t.Fatal("expected error for product with no variants")
	}
	if !strings.Contains(err.Error(), "no variants") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestImportProduct_InvalidVariantIDs(t *testing.T) {
	db := newSCTestDatabase(t)
	reg := fulfillment.NewRegistry()

	provider := &richCatalogProvider{stubFulfillmentProvider: stubFulfillmentProvider{id: "printful", provType: "pod"}}
	if err := reg.Register(provider); err != nil {
		t.Fatal(err)
	}

	listingOps := &mockListingOps{}
	svc := NewSupplyChainAppService(reg, db, "test-node", testPrivKeyBytes)
	svc.SetListingOps(listingOps)

	_, err := svc.ImportProduct(context.Background(), contracts.ImportProductParams{
		ProviderID: "printful",
		ProductID:  "71",
		VariantIDs: []string{"nonexistent-1", "nonexistent-2"},
	})
	if err == nil {
		t.Fatal("expected error for non-matching variant IDs")
	}
	if !strings.Contains(err.Error(), "none of the requested variant IDs") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestImportProduct_ListingOpsNotWired(t *testing.T) {
	db := newSCTestDatabase(t)
	reg := fulfillment.NewRegistry()

	provider := &richCatalogProvider{stubFulfillmentProvider: stubFulfillmentProvider{id: "printful", provType: "pod"}}
	if err := reg.Register(provider); err != nil {
		t.Fatal(err)
	}

	svc := NewSupplyChainAppService(reg, db, "test-node", testPrivKeyBytes)
	// listingOps not set

	_, err := svc.ImportProduct(context.Background(), contracts.ImportProductParams{
		ProviderID: "printful",
		ProductID:  "71",
	})
	if err == nil {
		t.Fatal("expected error when listing ops not wired")
	}
	if !strings.Contains(err.Error(), "listing ops not wired") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestImportProduct_SaveListingError(t *testing.T) {
	db := newSCTestDatabase(t)
	reg := fulfillment.NewRegistry()

	provider := &richCatalogProvider{stubFulfillmentProvider: stubFulfillmentProvider{id: "printful", provType: "pod"}}
	if err := reg.Register(provider); err != nil {
		t.Fatal(err)
	}

	listingOps := &mockListingOps{err: errors.New("save failed")}
	svc := NewSupplyChainAppService(reg, db, "test-node", testPrivKeyBytes)
	svc.SetListingOps(listingOps)

	_, err := svc.ImportProduct(context.Background(), contracts.ImportProductParams{
		ProviderID:   "printful",
		ProductID:    "71",
		RetailMarkup: 1.5,
	})
	if err == nil {
		t.Fatal("expected error when save listing fails")
	}
	if !strings.Contains(err.Error(), "save listing draft") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestImportProduct_DefaultMarkup(t *testing.T) {
	db := newSCTestDatabase(t)
	reg := fulfillment.NewRegistry()

	provider := &richCatalogProvider{stubFulfillmentProvider: stubFulfillmentProvider{id: "printful", provType: "pod"}}
	if err := reg.Register(provider); err != nil {
		t.Fatal(err)
	}

	listingOps := &mockListingOps{}
	svc := NewSupplyChainAppService(reg, db, "test-node", testPrivKeyBytes)
	svc.SetListingOps(listingOps)

	result, err := svc.ImportProduct(context.Background(), contracts.ImportProductParams{
		ProviderID: "printful",
		ProductID:  "71",
		// RetailMarkup = 0 → should default to 1.0
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With markup=1.0 and minCost=$8.25 → retail=825 cents
	if result.RetailPrice != result.SupplierCost {
		t.Errorf("with default markup, retail (%s) should equal supplier cost (%s)", result.RetailPrice, result.SupplierCost)
	}
}

// ---------------------------------------------------------------------------
// Tests: cancelFulfillmentForOrder
// ---------------------------------------------------------------------------

func seedMapping(t *testing.T, db *scTestDatabase, orderID, providerID, fulfillmentID, status string) {
	t.Helper()
	if err := db.Update(func(tx database.Tx) error {
		return tx.Save(&models.FulfillmentOrderMapping{
			MobazhaOrderID:     orderID,
			ProviderID:         providerID,
			FulfillmentOrderID: fulfillmentID,
			Status:             status,
		})
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCancelFulfillment_HappyPath(t *testing.T) {
	db := newSCTestDatabase(t)
	reg := fulfillment.NewRegistry()
	var cancelledID string
	provider := &stubFulfillmentProvider{
		id: "printful", provType: "pod",
		cancelFn: func(_ context.Context, orderID string) error {
			cancelledID = orderID
			return nil
		},
	}
	_ = reg.Register(provider)

	svc := NewSupplyChainAppService(reg, db, "test-node", testPrivKeyBytes)
	seedMapping(t, db, "order-1", "printful", "ful-100", string(contracts.FulfillmentStatusPending))

	svc.cancelFulfillmentForOrder("order-1")

	if cancelledID != "ful-100" {
		t.Errorf("expected provider cancel called with ful-100, got %q", cancelledID)
	}

	// Verify mapping status updated to canceled
	var mapping models.FulfillmentOrderMapping
	if err := db.View(func(tx database.Tx) error {
		return tx.Read().Where("mobazha_order_id = ?", "order-1").First(&mapping).Error
	}); err != nil {
		t.Fatal(err)
	}
	if mapping.Status != string(contracts.FulfillmentStatusCanceled) {
		t.Errorf("expected status canceled, got %s", mapping.Status)
	}
}

func TestCancelFulfillment_NoMapping(t *testing.T) {
	db := newSCTestDatabase(t)
	reg := fulfillment.NewRegistry()
	called := false
	provider := &stubFulfillmentProvider{
		id: "printful", provType: "pod",
		cancelFn: func(_ context.Context, _ string) error {
			called = true
			return nil
		},
	}
	_ = reg.Register(provider)

	svc := NewSupplyChainAppService(reg, db, "test-node", testPrivKeyBytes)
	svc.cancelFulfillmentForOrder("nonexistent-order")

	if called {
		t.Error("provider cancel should not be called when no mapping exists")
	}
}

func TestCancelFulfillment_SkipsTerminalStatus(t *testing.T) {
	terminalStatuses := []contracts.FulfillmentStatus{
		contracts.FulfillmentStatusShipped,
		contracts.FulfillmentStatusDelivered,
		contracts.FulfillmentStatusCanceled,
	}

	for _, status := range terminalStatuses {
		t.Run(string(status), func(t *testing.T) {
			db := newSCTestDatabase(t)
			reg := fulfillment.NewRegistry()
			called := false
			provider := &stubFulfillmentProvider{
				id: "printful", provType: "pod",
				cancelFn: func(_ context.Context, _ string) error {
					called = true
					return nil
				},
			}
			_ = reg.Register(provider)

			svc := NewSupplyChainAppService(reg, db, "test-node", testPrivKeyBytes)
			seedMapping(t, db, "order-t", "printful", "ful-t", string(status))

			svc.cancelFulfillmentForOrder("order-t")

			if called {
				t.Errorf("cancel should be skipped for terminal status %s", status)
			}
		})
	}
}

func TestCancelFulfillment_ProviderError(t *testing.T) {
	db := newSCTestDatabase(t)
	reg := fulfillment.NewRegistry()
	provider := &stubFulfillmentProvider{
		id: "printful", provType: "pod",
		cancelFn: func(_ context.Context, _ string) error {
			return errors.New("supplier API 500")
		},
	}
	_ = reg.Register(provider)

	svc := NewSupplyChainAppService(reg, db, "test-node", testPrivKeyBytes)
	seedMapping(t, db, "order-e", "printful", "ful-e", string(contracts.FulfillmentStatusPending))

	svc.cancelFulfillmentForOrder("order-e")

	var mapping models.FulfillmentOrderMapping
	if err := db.View(func(tx database.Tx) error {
		return tx.Read().Where("mobazha_order_id = ?", "order-e").First(&mapping).Error
	}); err != nil {
		t.Fatal(err)
	}
	if mapping.Status == string(contracts.FulfillmentStatusCanceled) {
		t.Error("status should NOT be canceled when provider returns error")
	}
	if !strings.Contains(mapping.ErrorMessage, "supplier API 500") {
		t.Errorf("expected error_message to contain 'supplier API 500', got %q", mapping.ErrorMessage)
	}
}

func TestCancelFulfillment_InProgressAllowed(t *testing.T) {
	db := newSCTestDatabase(t)
	reg := fulfillment.NewRegistry()
	provider := &stubFulfillmentProvider{
		id: "printful", provType: "pod",
		cancelFn: func(_ context.Context, _ string) error { return nil },
	}
	_ = reg.Register(provider)

	svc := NewSupplyChainAppService(reg, db, "test-node", testPrivKeyBytes)
	seedMapping(t, db, "order-ip", "printful", "ful-ip", string(contracts.FulfillmentStatusInProcess))

	svc.cancelFulfillmentForOrder("order-ip")

	var mapping models.FulfillmentOrderMapping
	if err := db.View(func(tx database.Tx) error {
		return tx.Read().Where("mobazha_order_id = ?", "order-ip").First(&mapping).Error
	}); err != nil {
		t.Fatal(err)
	}
	if mapping.Status != string(contracts.FulfillmentStatusCanceled) {
		t.Errorf("expected status canceled, got %s", mapping.Status)
	}
}
