package digital

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/encryption"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestAssetService constructs a DigitalAssetAppService backed by
// in-memory SQLite and mock BlobStore / KeyProvider.
func newTestAssetService(t *testing.T) (*DigitalAssetAppService, *testDatabase) {
	t.Helper()
	db := newDigitalTestDB(t)
	blob := newMemBlobStore()
	keys := newTestKeyProvider()
	svc := NewDigitalAssetAppService(db, blob, keys)
	return svc, db
}

func seedBuyerPortalAccess(t *testing.T, db *testDatabase, orderID string) string {
	t.Helper()
	token := "bpt_test_" + orderID
	sum := sha256.Sum256([]byte(token))
	expiresAt := time.Now().Add(time.Hour)
	order := &models.GuestOrder{
		OrderToken:                orderID,
		State:                     models.GuestOrderFunded,
		BuyerPortalTokenHash:      hex.EncodeToString(sum[:]),
		BuyerPortalTokenExpiresAt: &expiresAt,
		BuyerPortalTokenVersion:   1,
		ExpiresAt:                 expiresAt,
	}
	order.TenantID = database.StandaloneTenantID
	require.NoError(t, db.gormDB.Create(order).Error)
	return token
}

// ---------------------------------------------------------------------------
// 1. Encryption & Signing
// ---------------------------------------------------------------------------

func TestCrypto_EncryptDecryptFile(t *testing.T) {
	keys := newTestKeyProvider()
	crypto := encryption.NewDigitalCrypto(keys)

	plaintext := []byte("Hello, digital world! 🌍")
	assetID := "test-asset-001"
	keyVersion := 1

	ciphertext, err := crypto.EncryptFile(plaintext, assetID, keyVersion)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := crypto.DecryptFile(ciphertext, assetID, keyVersion)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestCrypto_DifferentAssetIDs_ProduceDifferentKeys(t *testing.T) {
	keys := newTestKeyProvider()
	crypto := encryption.NewDigitalCrypto(keys)

	k1, err := crypto.DeriveAssetKey("asset-a", 1)
	require.NoError(t, err)
	k2, err := crypto.DeriveAssetKey("asset-b", 1)
	require.NoError(t, err)

	assert.NotEqual(t, k1, k2, "different asset IDs must derive different keys")
}

func TestCrypto_EncryptDecryptLicenseKey(t *testing.T) {
	keys := newTestKeyProvider()
	crypto := encryption.NewDigitalCrypto(keys)

	licenseKey := []byte("XXXX-YYYY-ZZZZ-1234")
	assetID := "lic-asset-001"
	keyVersion := 1

	encrypted, err := crypto.EncryptLicenseKey(licenseKey, assetID, keyVersion)
	require.NoError(t, err)

	decrypted, err := crypto.DecryptLicenseKey(encrypted, assetID, keyVersion)
	require.NoError(t, err)
	assert.Equal(t, licenseKey, decrypted)
}

func TestCrypto_SignVerifyDownloadURL(t *testing.T) {
	keys := newTestKeyProvider()
	crypto := encryption.NewDigitalCrypto(keys)

	orderID := "order-001"
	nonce := "abc123"
	assetID := "asset-001"
	expiryTs := time.Now().Add(time.Hour).Unix()
	grantVersion := 1
	keyVersion := 1

	sig, err := crypto.SignDownloadURL(orderID, nonce, assetID, expiryTs, grantVersion, keyVersion)
	require.NoError(t, err)

	ok, err := crypto.VerifyDownloadURL(orderID, nonce, assetID, expiryTs, grantVersion, keyVersion, sig)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = crypto.VerifyDownloadURL("wrong-order", nonce, assetID, expiryTs, grantVersion, keyVersion, sig)
	require.NoError(t, err)
	assert.False(t, ok, "tampered data must fail verification")
}

// ---------------------------------------------------------------------------
// 2. File Upload + Download round-trip
// ---------------------------------------------------------------------------

func TestAssetService_UploadAndDownloadFile(t *testing.T) {
	svc, _ := newTestAssetService(t)

	ctx := context.Background()
	plaintext := []byte("PDF content here")

	assetInfo, err := svc.UploadFileAssetStream(ctx, "listing-1", "", "test.pdf", "application/pdf", bytes.NewReader(plaintext), int64(len(plaintext)))
	require.NoError(t, err)
	assert.Equal(t, models.AssetTypeFile, assetInfo.AssetType)

	asset, err := svc.getAssetModelByID(assetInfo.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, asset.FileHash)

	downloaded, err := svc.DownloadFile(ctx, asset)
	require.NoError(t, err)
	assert.Equal(t, plaintext, downloaded)
}

// ---------------------------------------------------------------------------
// 3. Link Reveal
// ---------------------------------------------------------------------------

func TestAssetService_CreateAndRevealLink(t *testing.T) {
	svc, _ := newTestAssetService(t)

	url := "https://notion.so/my-template"
	assetInfo, err := svc.CreateLinkAsset("listing-link", "", url)
	require.NoError(t, err)
	assert.Equal(t, models.AssetTypeLink, assetInfo.AssetType)

	linkAsset, err := svc.getAssetModelByID(assetInfo.ID)
	require.NoError(t, err)
	revealed, err := svc.RevealLink(linkAsset)
	require.NoError(t, err)
	assert.Equal(t, url, revealed)
}

func TestAssetService_RejectsVariantSpecificAssetsInPhase1(t *testing.T) {
	svc, _ := newTestAssetService(t)

	_, err := svc.CreateLinkAsset("listing-link", "sku-blue", "https://example.com")
	require.ErrorIs(t, err, contracts.ErrDigitalVariantUnsupported)

	_, err = svc.CreateLicenseKeyAsset("listing-lic", "sku-blue", "app-test")
	require.ErrorIs(t, err, contracts.ErrDigitalVariantUnsupported)

	_, err = svc.ImportLicenseKeys("listing-lic", "sku-blue", "app-test", []string{"KEY-001"}, "perpetual", 1, time.Time{})
	require.ErrorIs(t, err, contracts.ErrDigitalVariantUnsupported)

	_, err = svc.UploadFileAssetStream(context.Background(), "listing-file", "sku-blue", "file.zip", "application/zip", bytes.NewReader([]byte("x")), 1)
	require.ErrorIs(t, err, contracts.ErrDigitalVariantUnsupported)
}

// ---------------------------------------------------------------------------
// 4. License Key Pool — import, allocate, validate, activate, deactivate
// ---------------------------------------------------------------------------

func TestAssetService_ImportAndAllocateLicenseKeys(t *testing.T) {
	svc, _ := newTestAssetService(t)

	keys := []string{"KEY-001", "KEY-002", "KEY-003"}
	imported, err := svc.ImportLicenseKeys("listing-lic", "", "app-test", keys, "perpetual", 3, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, 3, imported)

	stats, err := svc.GetLicenseKeyPoolStats("listing-lic", "")
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.Available)
	assert.Equal(t, int64(0), stats.Dispensed)
	assert.Equal(t, int64(0), stats.Revoked)

	lk, err := svc.AllocateLicenseKey("listing-lic", "", "order-1", "buyer-1")
	require.NoError(t, err)
	assert.Equal(t, models.LicenseKeyStatusDispensed, lk.Status)
	assert.Equal(t, "order-1", lk.OrderID)

	stats, err = svc.GetLicenseKeyPoolStats("listing-lic", "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), stats.Available)
	assert.Equal(t, int64(1), stats.Dispensed)
}

func TestAssetService_AllocateLicenseKey_ExhaustsPool(t *testing.T) {
	svc, _ := newTestAssetService(t)

	_, err := svc.ImportLicenseKeys("listing-x", "", "app-x", []string{"ONLY-KEY"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)

	_, err = svc.AllocateLicenseKey("listing-x", "", "order-a", "buyer-a")
	require.NoError(t, err)

	_, err = svc.AllocateLicenseKey("listing-x", "", "order-b", "buyer-b")
	assert.Error(t, err, "should fail when pool is exhausted")
}

func TestAssetService_ValidateLicense(t *testing.T) {
	svc, _ := newTestAssetService(t)

	_, err := svc.ImportLicenseKeys("listing-val", "", "app-val", []string{"LIC-ABC"}, "perpetual", 2, time.Time{})
	require.NoError(t, err)

	result, err := svc.ValidateLicense("LIC-ABC", "app-val")
	require.NoError(t, err)
	assert.False(t, result.Valid, "available (undispensed) key must not validate as valid")
	assert.Equal(t, "not_issued", result.Reason)

	_, err = svc.AllocateLicenseKey("listing-val", "", "order-v", "buyer-v")
	require.NoError(t, err)

	result, err = svc.ValidateLicense("LIC-ABC", "app-val")
	require.NoError(t, err)
	assert.True(t, result.Valid, "dispensed key is valid")
	assert.Equal(t, "perpetual", result.LicenseType)
}

func TestAssetService_ActivateDeactivateLicense(t *testing.T) {
	svc, _ := newTestAssetService(t)

	_, err := svc.ImportLicenseKeys("listing-act", "", "app-act", []string{"ACT-KEY"}, "perpetual", 2, time.Time{})
	require.NoError(t, err)
	_, err = svc.AllocateLicenseKey("listing-act", "", "order-act", "buyer-act")
	require.NoError(t, err)

	activation, err := svc.ActivateLicense("ACT-KEY", "app-act", "device-123", "MacBook", "127.0.0.1")
	require.NoError(t, err)
	assert.True(t, activation.IsActive)
	assert.Equal(t, "device-123", activation.Fingerprint)

	reactivation, err := svc.ActivateLicense("ACT-KEY", "app-act", "device-123", "MacBook", "127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, activation.ID, reactivation.ID, "same fingerprint returns existing activation")

	err = svc.DeactivateLicense("ACT-KEY", "app-act", "device-123")
	require.NoError(t, err)

	result, err := svc.ValidateLicense("ACT-KEY", "app-act")
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Equal(t, int64(0), result.Activations, "deactivated device should not count")
}

func TestAssetService_ActivateLicense_MaxActivationsEnforced(t *testing.T) {
	svc, _ := newTestAssetService(t)

	_, err := svc.ImportLicenseKeys("listing-max", "", "app-max", []string{"MAX-KEY"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)
	_, err = svc.AllocateLicenseKey("listing-max", "", "order-max", "buyer-max")
	require.NoError(t, err)

	_, err = svc.ActivateLicense("MAX-KEY", "app-max", "device-1", "First", "")
	require.NoError(t, err)

	_, err = svc.ActivateLicense("MAX-KEY", "app-max", "device-2", "Second", "")
	assert.Error(t, err, "should reject activation beyond max")
}

// ---------------------------------------------------------------------------
// 5. Download Grant lifecycle
// ---------------------------------------------------------------------------

func TestAssetService_CreateGrant_WithEntitlementSnapshot(t *testing.T) {
	svc, _ := newTestAssetService(t)

	ctx := context.Background()
	assetInfo, err := svc.UploadFileAssetStream(ctx, "listing-snap", "", "file.zip", "application/zip", bytes.NewReader([]byte("data")), int64(len([]byte("data"))))
	require.NoError(t, err)

	snapAsset, err := svc.getAssetModelByID(assetInfo.ID)
	require.NoError(t, err)
	grant, err := svc.CreateDownloadGrant(snapAsset, "order-snap", "buyer-snap", models.GrantStatusActive)
	require.NoError(t, err)
	assert.Equal(t, models.GrantStatusActive, grant.Status)
	assert.NotEmpty(t, grant.AssetSnapshot)

	var snap models.AssetSnapshot
	err = json.Unmarshal(grant.AssetSnapshot, &snap)
	require.NoError(t, err)
	assert.Equal(t, models.AssetTypeFile, snap.AssetType)
	assert.Equal(t, snapAsset.FileHash, snap.FileHash)
	assert.Equal(t, snapAsset.FileName, snap.FileName)
}

func TestAssetService_FreezeAndRestoreGrants(t *testing.T) {
	svc, _ := newTestAssetService(t)

	ctx := context.Background()
	ai, err := svc.UploadFileAssetStream(ctx, "listing-frz", "", "f.pdf", "application/pdf", bytes.NewReader([]byte("x")), int64(len([]byte("x"))))
	require.NoError(t, err)
	asset, err := svc.getAssetModelByID(ai.ID)
	require.NoError(t, err)

	_, err = svc.CreateDownloadGrant(asset, "order-frz", "buyer", models.GrantStatusActive)
	require.NoError(t, err)

	err = svc.FreezeGrantsByOrder("order-frz", "dispute_opened")
	require.NoError(t, err)

	grants, err := svc.GetGrantsByOrder("order-frz")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, models.GrantStatusFrozen, grants[0].Status)
	assert.Equal(t, models.GrantStatusActive, grants[0].PreviousStatus)
}

func TestAssetService_RevokeGrants(t *testing.T) {
	svc, _ := newTestAssetService(t)

	ctx := context.Background()
	ai, err := svc.UploadFileAssetStream(ctx, "listing-rev", "", "f.pdf", "application/pdf", bytes.NewReader([]byte("x")), int64(len([]byte("x"))))
	require.NoError(t, err)
	asset, err := svc.getAssetModelByID(ai.ID)
	require.NoError(t, err)

	_, err = svc.CreateDownloadGrant(asset, "order-rev", "buyer", models.GrantStatusActive)
	require.NoError(t, err)

	err = svc.RevokeGrantsByOrder("order-rev", "refunded")
	require.NoError(t, err)

	grants, err := svc.GetGrantsByOrder("order-rev")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, models.GrantStatusRevoked, grants[0].Status)
}

func TestAssetService_RecordDownload(t *testing.T) {
	svc, _ := newTestAssetService(t)

	ctx := context.Background()
	ai, err := svc.UploadFileAssetStream(ctx, "listing-dl", "", "f.pdf", "application/pdf", bytes.NewReader([]byte("x")), int64(len([]byte("x"))))
	require.NoError(t, err)
	asset, err := svc.getAssetModelByID(ai.ID)
	require.NoError(t, err)

	grant, err := svc.CreateDownloadGrant(asset, "order-dl", "buyer", models.GrantStatusActive)
	require.NoError(t, err)

	err = svc.RecordDownload(grant, "buyer", "ip-hash", "Mozilla/5.0", true, "")
	require.NoError(t, err)

	updated, err := svc.GetGrantByNonce(grant.Nonce)
	require.NoError(t, err)
	assert.Equal(t, 1, updated.DownloadCount)
}

// ---------------------------------------------------------------------------
// 6. Entitlement Service — OrderConfirmation event-driven tests
// ---------------------------------------------------------------------------

func newTestEntitlementService(t *testing.T) (
	*DigitalEntitlementAppService,
	*DigitalAssetAppService,
	events.Bus,
	*testOrderQuerier,
) {
	t.Helper()
	db := newDigitalTestDB(t)
	blob := newMemBlobStore()
	keys := newTestKeyProvider()
	assetSvc := NewDigitalAssetAppService(db, blob, keys)

	bus := events.NewBus()
	features := newTestFeatureResolver(map[string]bool{
		pkgconfig.FeatureDigitalAutoDeliveryEnabled.Key: true,
	})
	orderQ := &testOrderQuerier{
		contractType:  "DIGITAL_GOOD",
		listingSlug:   "listing-ent",
		variantSKU:    "",
		buyerPeerID:   "buyer-peer",
		paymentMethod: "CANCELABLE",
	}

	entSvc := NewDigitalEntitlementAppService(context.Background(), db, features, assetSvc, orderQ, bus)
	return entSvc, assetSvc, bus, orderQ
}

func TestEntitlement_OrderConfirmation_CreatesActiveGrant_CANCELABLE(t *testing.T) {
	entSvc, assetSvc, bus, _ := newTestEntitlementService(t)

	ctx := context.Background()
	_, err := assetSvc.UploadFileAssetStream(ctx, "listing-ent", "", "file.zip", "application/zip", bytes.NewReader([]byte("test")), int64(len([]byte("test"))))
	require.NoError(t, err)

	require.NoError(t, entSvc.Start())

	bus.Emit(&events.OrderConfirmation{OrderID: "order-ent-1"})
	time.Sleep(100 * time.Millisecond)

	grants, err := assetSvc.GetGrantsByOrder("order-ent-1")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, models.GrantStatusActive, grants[0].Status)
	assert.NotEmpty(t, grants[0].AssetSnapshot, "EntitlementSnapshot must be present")
}

func TestEntitlement_OrderConfirmation_CreatesProtectedGrant_MODERATED(t *testing.T) {
	entSvc, assetSvc, bus, orderQ := newTestEntitlementService(t)
	orderQ.paymentMethod = "MODERATED"

	ctx := context.Background()
	_, err := assetSvc.UploadFileAssetStream(ctx, "listing-ent", "", "file.zip", "application/zip", bytes.NewReader([]byte("test")), int64(len([]byte("test"))))
	require.NoError(t, err)

	require.NoError(t, entSvc.Start())

	bus.Emit(&events.OrderConfirmation{OrderID: "order-mod-1"})
	time.Sleep(100 * time.Millisecond)

	grants, err := assetSvc.GetGrantsByOrder("order-mod-1")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, models.GrantStatusProtected, grants[0].Status)
}

func TestEntitlement_OrderConfirmation_AllocatesLicenseKey(t *testing.T) {
	entSvc, assetSvc, bus, _ := newTestEntitlementService(t)

	_, err := assetSvc.ImportLicenseKeys("listing-ent", "", "app-ent", []string{"ENT-LIC-001"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)

	require.NoError(t, entSvc.Start())

	bus.Emit(&events.OrderConfirmation{OrderID: "order-lic-1"})
	time.Sleep(100 * time.Millisecond)

	grants, err := assetSvc.GetGrantsByOrder("order-lic-1")
	require.NoError(t, err)
	require.Len(t, grants, 1)

	poolStats, err := assetSvc.GetLicenseKeyPoolStats("listing-ent", "")
	require.NoError(t, err)
	assert.Equal(t, int64(0), poolStats.Available)
	assert.Equal(t, int64(1), poolStats.Dispensed)
}

// TestEntitlement_GuestOrder_CreatesGrant verifies the guest checkout path:
// when an order is funded with empty BuyerPeerID and PaymentMethod="DIRECT"
// (the values produced by dbOrderQuerier.getGuestOrderMetadata), the
// entitlement service still creates a download grant. The grant uses the
// orderToken as OrderID; the buyer-portal endpoint still requires the
// independent buyerPortalToken to read the granted secrets.
func TestEntitlement_GuestOrder_CreatesGrant(t *testing.T) {
	entSvc, assetSvc, bus, orderQ := newTestEntitlementService(t)
	orderQ.buyerPeerID = "" // anonymous guest
	orderQ.paymentMethod = "DIRECT"

	ctx := context.Background()
	_, err := assetSvc.UploadFileAssetStream(ctx, "listing-ent", "", "guest.zip", "application/zip", bytes.NewReader([]byte("guest-payload")), int64(len("guest-payload")))
	require.NoError(t, err)

	require.NoError(t, entSvc.Start())

	const orderToken = "gst_0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab"
	bus.Emit(&events.OrderConfirmation{OrderID: orderToken})
	time.Sleep(100 * time.Millisecond)

	grants, err := assetSvc.GetGrantsByOrder(orderToken)
	require.NoError(t, err)
	require.Len(t, grants, 1, "guest order must produce exactly one grant")
	assert.Equal(t, models.GrantStatusActive, grants[0].Status, "DIRECT payment yields active grant (no escrow protection)")
	assert.Empty(t, grants[0].BuyerPeerID, "anonymous buyer: BuyerPeerID stays empty")
	assert.Equal(t, orderToken, grants[0].OrderID)
	assert.NotEmpty(t, grants[0].AssetSnapshot, "EntitlementSnapshot must be present")
}

func TestEntitlement_SkipsNonDigitalOrders(t *testing.T) {
	entSvc, assetSvc, bus, orderQ := newTestEntitlementService(t)
	orderQ.contractType = "PHYSICAL_GOOD"

	ctx := context.Background()
	_, err := assetSvc.UploadFileAssetStream(ctx, "listing-ent", "", "file.zip", "application/zip", bytes.NewReader([]byte("test")), int64(len([]byte("test"))))
	require.NoError(t, err)

	require.NoError(t, entSvc.Start())

	bus.Emit(&events.OrderConfirmation{OrderID: "order-phys-1"})
	time.Sleep(100 * time.Millisecond)

	grants, err := assetSvc.GetGrantsByOrder("order-phys-1")
	require.NoError(t, err)
	assert.Empty(t, grants, "non-digital orders should not create grants")
}

func TestEntitlement_SkipsWhenFeatureFlagDisabled(t *testing.T) {
	db := newDigitalTestDB(t)
	blob := newMemBlobStore()
	keys := newTestKeyProvider()
	assetSvc := NewDigitalAssetAppService(db, blob, keys)

	bus := events.NewBus()
	features := newTestFeatureResolver(map[string]bool{
		pkgconfig.FeatureDigitalAutoDeliveryEnabled.Key: false,
	})
	orderQ := &testOrderQuerier{
		contractType:  "DIGITAL_GOOD",
		listingSlug:   "listing-ff",
		paymentMethod: "CANCELABLE",
	}
	entSvc := NewDigitalEntitlementAppService(context.Background(), db, features, assetSvc, orderQ, bus)

	ctx := context.Background()
	_, err := assetSvc.UploadFileAssetStream(ctx, "listing-ff", "", "f.zip", "application/zip", bytes.NewReader([]byte("x")), int64(len([]byte("x"))))
	require.NoError(t, err)

	require.NoError(t, entSvc.Start())

	bus.Emit(&events.OrderConfirmation{OrderID: "order-ff-1"})
	time.Sleep(100 * time.Millisecond)

	grants, err := assetSvc.GetGrantsByOrder("order-ff-1")
	require.NoError(t, err)
	assert.Empty(t, grants, "disabled feature flag should prevent grant creation")
}

// ---------------------------------------------------------------------------
// 7. Dispute freeze / Dispute close / Refund revoke
// ---------------------------------------------------------------------------

func TestEntitlement_DisputeOpen_FreezesGrants_SuspendsLicenses(t *testing.T) {
	entSvc, assetSvc, bus, _ := newTestEntitlementService(t)

	_, err := assetSvc.ImportLicenseKeys("listing-ent", "", "app-ent", []string{"DISP-LIC"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)

	require.NoError(t, entSvc.Start())

	bus.Emit(&events.OrderConfirmation{OrderID: "order-disp"})
	time.Sleep(100 * time.Millisecond)

	bus.Emit(&events.DisputeOpen{OrderID: "order-disp"})
	time.Sleep(100 * time.Millisecond)

	grants, err := assetSvc.GetGrantsByOrder("order-disp")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, models.GrantStatusFrozen, grants[0].Status)
}

func TestEntitlement_DisputeClose_SellerWins_RestoresGrants(t *testing.T) {
	entSvc, assetSvc, bus, _ := newTestEntitlementService(t)

	ctx := context.Background()
	_, err := assetSvc.UploadFileAssetStream(ctx, "listing-ent", "", "f.pdf", "application/pdf", bytes.NewReader([]byte("x")), int64(len([]byte("x"))))
	require.NoError(t, err)

	require.NoError(t, entSvc.Start())

	bus.Emit(&events.OrderConfirmation{OrderID: "order-dc"})
	time.Sleep(100 * time.Millisecond)

	bus.Emit(&events.DisputeOpen{OrderID: "order-dc"})
	time.Sleep(100 * time.Millisecond)

	bus.Emit(&events.DisputeClose{OrderID: "order-dc", BuyerRefunded: false})
	time.Sleep(100 * time.Millisecond)

	grants, err := assetSvc.GetGrantsByOrder("order-dc")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, models.GrantStatusActive, grants[0].Status, "seller won — grants should be restored")
}

func TestEntitlement_DisputeClose_BuyerWins_RevokesGrants(t *testing.T) {
	entSvc, assetSvc, bus, _ := newTestEntitlementService(t)

	ctx := context.Background()
	_, err := assetSvc.UploadFileAssetStream(ctx, "listing-ent", "", "f.pdf", "application/pdf", bytes.NewReader([]byte("x")), int64(len([]byte("x"))))
	require.NoError(t, err)

	require.NoError(t, entSvc.Start())

	bus.Emit(&events.OrderConfirmation{OrderID: "order-dc2"})
	time.Sleep(100 * time.Millisecond)

	bus.Emit(&events.DisputeOpen{OrderID: "order-dc2"})
	time.Sleep(100 * time.Millisecond)

	bus.Emit(&events.DisputeClose{OrderID: "order-dc2", BuyerRefunded: true})
	time.Sleep(100 * time.Millisecond)

	grants, err := assetSvc.GetGrantsByOrder("order-dc2")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, models.GrantStatusRevoked, grants[0].Status, "buyer won — grants should be revoked")
}

func TestEntitlement_Refund_RevokesAll(t *testing.T) {
	entSvc, assetSvc, bus, _ := newTestEntitlementService(t)

	_, err := assetSvc.ImportLicenseKeys("listing-ent", "", "app-ent", []string{"REF-LIC"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)

	require.NoError(t, entSvc.Start())

	bus.Emit(&events.OrderConfirmation{OrderID: "order-ref"})
	time.Sleep(100 * time.Millisecond)

	bus.Emit(&events.Refund{OrderID: "order-ref"})
	time.Sleep(100 * time.Millisecond)

	grants, err := assetSvc.GetGrantsByOrder("order-ref")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, models.GrantStatusRevoked, grants[0].Status)
}

// ---------------------------------------------------------------------------
// 8. Buyer Portal
// ---------------------------------------------------------------------------

func TestAssetService_GetBuyerDigitalAssets_FileEntry(t *testing.T) {
	svc, db := newTestAssetService(t)
	token := seedBuyerPortalAccess(t, db, "order-bp")

	ctx := context.Background()
	ai, err := svc.UploadFileAssetStream(ctx, "listing-bp", "", "report.pdf", "application/pdf", bytes.NewReader([]byte("data")), int64(len([]byte("data"))))
	require.NoError(t, err)
	asset, err := svc.getAssetModelByID(ai.ID)
	require.NoError(t, err)

	_, err = svc.CreateDownloadGrant(asset, "order-bp", "buyer-bp", models.GrantStatusActive)
	require.NoError(t, err)

	entries, err := svc.GetBuyerDigitalAssets("order-bp", token, "", false, 3600)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	assert.Equal(t, models.AssetTypeFile, entries[0].AssetType)
	assert.NotEmpty(t, entries[0].DownloadURL)
	assert.Contains(t, entries[0].DownloadURL, "digital-download")
}

func TestAssetService_GetBuyerDigitalAssets_LinkEntry(t *testing.T) {
	svc, db := newTestAssetService(t)
	token := seedBuyerPortalAccess(t, db, "order-bpl")

	url := "https://notion.so/template-123"
	ai, err := svc.CreateLinkAsset("listing-bpl", "", url)
	require.NoError(t, err)
	asset, err := svc.getAssetModelByID(ai.ID)
	require.NoError(t, err)

	_, err = svc.CreateDownloadGrant(asset, "order-bpl", "buyer-bpl", models.GrantStatusActive)
	require.NoError(t, err)

	entries, err := svc.GetBuyerDigitalAssets("order-bpl", token, "", false, 3600)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	assert.Equal(t, models.AssetTypeLink, entries[0].AssetType)
	assert.Equal(t, url, entries[0].DeliveryURL)
}

func TestAssetService_GetBuyerDigitalAssets_AuthenticatedWithoutPortalToken(t *testing.T) {
	svc, _ := newTestAssetService(t)

	url := "https://example.com/authenticated-download"
	ai, err := svc.CreateLinkAsset("listing-auth-bp", "", url)
	require.NoError(t, err)
	asset, err := svc.getAssetModelByID(ai.ID)
	require.NoError(t, err)

	_, err = svc.CreateDownloadGrant(asset, "order-auth-bp", "buyer-auth-bp", models.GrantStatusActive)
	require.NoError(t, err)

	_, err = svc.GetBuyerDigitalAssets("order-auth-bp", "", "", false, 3600)
	require.ErrorIs(t, err, contracts.ErrBuyerPortalAccess)

	entries, err := svc.GetBuyerDigitalAssets("order-auth-bp", "", "", true, 3600)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, url, entries[0].DeliveryURL)

	entries, err = svc.GetBuyerDigitalAssets("order-auth-bp", "", "buyer-auth-bp", false, 3600)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	_, err = svc.GetBuyerDigitalAssets("order-auth-bp", "", "other-buyer", false, 3600)
	require.ErrorIs(t, err, contracts.ErrBuyerPortalAccess)
}

func TestAssetService_GetBuyerDigitalAssets_LicenseKeyEntry(t *testing.T) {
	svc, db := newTestAssetService(t)
	token := seedBuyerPortalAccess(t, db, "order-bpk")

	_, err := svc.ImportLicenseKeys("listing-bpk", "", "app-bpk", []string{"BUYER-KEY-001"}, "perpetual", 2, time.Time{})
	require.NoError(t, err)

	lk, err := svc.AllocateLicenseKey("listing-bpk", "", "order-bpk", "buyer-bpk")
	require.NoError(t, err)

	assetModels, err := svc.getAssetModelsByListing("listing-bpk", "")
	require.NoError(t, err)
	require.NotEmpty(t, assetModels)

	_, err = svc.CreateDownloadGrant(&assetModels[0], "order-bpk", "buyer-bpk", models.GrantStatusActive)
	require.NoError(t, err)

	entries, err := svc.GetBuyerDigitalAssets("order-bpk", token, "", false, 3600)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	assert.Equal(t, models.AssetTypeLicenseKey, entries[0].AssetType)
	require.Len(t, entries[0].LicenseKeys, 1)
	assert.Equal(t, "BUYER-KEY-001", entries[0].LicenseKeys[0].LicenseKey)
	_ = lk
}

// ---------------------------------------------------------------------------
// 9. determineGrantStatus (unit)
// ---------------------------------------------------------------------------

func TestDetermineGrantStatus(t *testing.T) {
	tests := []struct {
		method string
		want   string
	}{
		{"CANCELABLE", models.GrantStatusActive},
		{"FIAT", models.GrantStatusActive},
		{"DIRECT", models.GrantStatusActive},
		{"MODERATED", models.GrantStatusProtected},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			assert.Equal(t, tt.want, determineGrantStatus(tt.method))
		})
	}
}
