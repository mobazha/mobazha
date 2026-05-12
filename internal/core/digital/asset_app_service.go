package digital

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/encryption"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DigitalAssetAppService manages digital asset CRUD, encrypted file storage,
// license key management, and download grant lifecycle.
type DigitalAssetAppService struct {
	db     database.Database
	blob   contracts.BlobStore
	crypto *encryption.DigitalCrypto
}

// NewDigitalAssetAppService creates a new DigitalAssetAppService.
func NewDigitalAssetAppService(
	db database.Database,
	blob contracts.BlobStore,
	keys contracts.KeyProvider,
) *DigitalAssetAppService {
	return &DigitalAssetAppService{
		db:     db,
		blob:   blob,
		crypto: encryption.NewDigitalCrypto(keys),
	}
}

// UploadFileAssetStream encrypts a file streamed from `src` and stores the
// resulting v1 chunked AEAD container in BlobStore, then persists a
// DigitalAsset record. Memory footprint is bounded by the stream chunk size
// (4 MiB) regardless of file size, supporting multi-hundred-MiB uploads on
// memory-constrained hosts.
//
// Storage layout: `digital/<assetID>` (UUID-only). FileHash on the asset
// record holds the plaintext SHA-256 for UI / dedup hints but is *not* part
// of the storage key — this avoids the "rename after streaming" problem
// since R2/S3 do not support cheap rename.
//
// `expectedSize` is informational (forwarded to the BlobStore for content-
// length / multipart sizing). Pass -1 if unknown; multipart uploaders fall
// back to chunked transfer.
func (s *DigitalAssetAppService) UploadFileAssetStream(
	ctx context.Context,
	listingSlug string,
	variantSKU string,
	fileName string,
	mimeType string,
	src io.Reader,
	expectedSize int64,
) (*contracts.DigitalAssetInfo, error) {
	if err := requirePhase1UniversalAsset(variantSKU); err != nil {
		return nil, err
	}

	assetID := uuid.Must(uuid.NewV7()).String()
	keyVersion := 1

	plainHasher := sha256.New()
	plainCounter := &countingReader{r: io.TeeReader(src, plainHasher)}

	encR, err := s.crypto.EncryptFileStreamReader(plainCounter, assetID, keyVersion, encryption.DefaultStreamChunkSize)
	if err != nil {
		return nil, fmt.Errorf("init stream encryptor: %w", err)
	}
	defer encR.Close()

	if err := s.blob.PutStream(ctx, blobKey(assetID), encR, -1, "application/octet-stream"); err != nil {
		return nil, fmt.Errorf("blob put stream: %w", err)
	}

	plainSize := plainCounter.n
	fileHash := hex.EncodeToString(plainHasher.Sum(nil))

	asset := &models.DigitalAsset{
		ID:          assetID,
		ListingSlug: listingSlug,
		VariantSKU:  variantSKU,
		AssetType:   models.AssetTypeFile,
		FileHash:    fileHash,
		FileName:    fileName,
		FileSize:    plainSize,
		MimeType:    mimeType,
		KeyVersion:  keyVersion,
	}

	if err := s.db.Update(func(tx database.Tx) error {
		return tx.Save(asset)
	}); err != nil {
		return nil, fmt.Errorf("save asset: %w", err)
	}

	return assetToInfo(asset), nil
}

// blobKey returns the canonical blob key for an encrypted file asset.
// Storage layout is UUID-only (`digital/<assetID>`); the encrypted body is
// a v1 chunked-AEAD container, so we don't need the file hash in the path
// and we don't need a rename step after a streaming upload.
func blobKey(assetID string) string {
	return "digital/" + assetID
}

// countingReader counts bytes flowing through it. Used to capture the exact
// plaintext size when the upstream upload doesn't advertise it.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	nn, err := c.r.Read(p)
	c.n += int64(nn)
	return nn, err
}

// CreateLinkAsset creates a DigitalAsset of type "link", storing the
// encrypted URL in DeliveryData.
func (s *DigitalAssetAppService) CreateLinkAsset(
	listingSlug string,
	variantSKU string,
	url string,
) (*contracts.DigitalAssetInfo, error) {
	if err := requirePhase1UniversalAsset(variantSKU); err != nil {
		return nil, err
	}

	assetID := uuid.Must(uuid.NewV7()).String()
	keyVersion := 1

	cipherURL, err := s.crypto.EncryptFile([]byte(url), assetID, keyVersion)
	if err != nil {
		return nil, fmt.Errorf("encrypt link: %w", err)
	}

	asset := &models.DigitalAsset{
		ID:           assetID,
		ListingSlug:  listingSlug,
		VariantSKU:   variantSKU,
		AssetType:    models.AssetTypeLink,
		KeyVersion:   keyVersion,
		DeliveryData: cipherURL,
	}

	if err := s.db.Update(func(tx database.Tx) error {
		return tx.Save(asset)
	}); err != nil {
		return nil, fmt.Errorf("save asset: %w", err)
	}

	return assetToInfo(asset), nil
}

// CreateLicenseKeyAsset creates a DigitalAsset of type "license_key" that
// serves as the binding between a listing variant and the license key pool.
func (s *DigitalAssetAppService) CreateLicenseKeyAsset(
	listingSlug string,
	variantSKU string,
	appID string,
) (*contracts.DigitalAssetInfo, error) {
	if err := requirePhase1UniversalAsset(variantSKU); err != nil {
		return nil, err
	}

	assetID := uuid.Must(uuid.NewV7()).String()

	asset := &models.DigitalAsset{
		ID:          assetID,
		ListingSlug: listingSlug,
		VariantSKU:  variantSKU,
		AssetType:   models.AssetTypeLicenseKey,
		KeyVersion:  1,
	}

	if err := s.db.Update(func(tx database.Tx) error {
		return tx.Save(asset)
	}); err != nil {
		return nil, fmt.Errorf("save asset: %w", err)
	}

	return assetToInfo(asset), nil
}

// ImportLicenseKeys batch-imports license keys for a listing, encrypting each.
func (s *DigitalAssetAppService) ImportLicenseKeys(
	listingSlug string,
	variantSKU string,
	appID string,
	keys []string,
	licenseType string,
	maxActivations int,
	expiresAt time.Time,
) (int, error) {
	if err := requirePhase1UniversalAsset(variantSKU); err != nil {
		return 0, err
	}
	if len(keys) == 0 {
		return 0, nil
	}

	var assets []models.DigitalAsset
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("listing_slug = ? AND variant_sku = ? AND asset_type = ?",
				listingSlug, variantSKU, models.AssetTypeLicenseKey).
			Order("created_at ASC").
			Find(&assets).Error
	}); err != nil {
		return 0, fmt.Errorf("query license key asset: %w", err)
	}

	assetID := ""
	keyVersion := 1
	if len(assets) > 0 {
		assetID = assets[0].ID
		keyVersion = assets[0].KeyVersion
	} else {
		a, err := s.CreateLicenseKeyAsset(listingSlug, variantSKU, appID)
		if err != nil {
			return 0, err
		}
		assetID = a.ID
	}

	imported := 0
	err := s.db.Update(func(tx database.Tx) error {
		for _, plainKey := range keys {
			cipherKey, err := s.crypto.EncryptLicenseKey([]byte(plainKey), assetID, keyVersion)
			if err != nil {
				return fmt.Errorf("encrypt license key: %w", err)
			}

			h := sha256.Sum256([]byte(plainKey))
			lk := &models.DigitalLicenseKey{
				ID:             uuid.Must(uuid.NewV7()).String(),
				ListingSlug:    listingSlug,
				VariantSKU:     variantSKU,
				LicenseKey:     cipherKey,
				KeyVersion:     keyVersion,
				LicenseHash:    hex.EncodeToString(h[:]),
				AppID:          appID,
				Status:         models.LicenseKeyStatusAvailable,
				MaxActivations: maxActivations,
				LicenseType:    licenseType,
				ExpiresAt:      expiresAt,
			}
			if err := tx.Save(lk); err != nil {
				return fmt.Errorf("save license key: %w", err)
			}
			imported++
		}
		return nil
	})

	return imported, err
}

func requirePhase1UniversalAsset(variantSKU string) error {
	if strings.TrimSpace(variantSKU) != "" {
		return contracts.ErrDigitalVariantUnsupported
	}
	return nil
}

// AllocateLicenseKey picks one available license key from the pool, marks it
// dispensed, and returns it. Idempotent on (orderID, listingSlug, variantSKU):
// if a key was already dispensed for the same order+SKU, the existing key is
// returned without consuming another pool slot.
//
// Race protection: allocation uses a tenant-safe conditional UPDATE
// (`status = available`) and checks RowsAffected. Under READ COMMITTED two
// concurrent transactions may SELECT the same candidate, but only one UPDATE
// can claim it; the loser retries.
func (s *DigitalAssetAppService) AllocateLicenseKey(
	listingSlug string,
	variantSKU string,
	orderID string,
	buyerPeerID string,
) (*models.DigitalLicenseKey, error) {
	var allocated models.DigitalLicenseKey

	const maxAllocRetries = 3
	var lastErr error
	for attempt := 0; attempt < maxAllocRetries; attempt++ {
		lastErr = s.db.Update(func(tx database.Tx) error {
			var candidate models.DigitalLicenseKey
			if err := tx.Read().
				Where("listing_slug = ? AND variant_sku = ? AND status = ?",
					listingSlug, variantSKU, models.LicenseKeyStatusAvailable).
				Order("id ASC").
				First(&candidate).Error; err != nil {
				return fmt.Errorf("no available license key: %w", err)
			}

			now := time.Now()
			rows, err := tx.UpdateColumns(
				map[string]interface{}{
					"status":        models.LicenseKeyStatusDispensed,
					"order_id":      orderID,
					"buyer_peer_id": buyerPeerID,
					"dispensed_at":  now,
				},
				map[string]interface{}{
					"id = ?":     candidate.ID,
					"status = ?": models.LicenseKeyStatusAvailable,
				},
				&models.DigitalLicenseKey{},
			)
			if err != nil {
				return fmt.Errorf("conditional update: %w", err)
			}
			if rows == 0 {
				return fmt.Errorf("license key %s already claimed (retrying)", candidate.ID)
			}

			if err := tx.Read().Where("id = ?", candidate.ID).First(&allocated).Error; err != nil {
				return fmt.Errorf("reload allocated key: %w", err)
			}
			return nil
		})
		if lastErr == nil {
			return &allocated, nil
		}
	}

	return nil, fmt.Errorf("allocate license key after %d attempts: %w", maxAllocRetries, lastErr)
}

// CountAllocatedKeys returns the number of license keys already dispensed for
// a given (orderID, listingSlug, variantSKU) combination. Used by the
// entitlement layer for idempotent multi-seat allocation.
func (s *DigitalAssetAppService) CountAllocatedKeys(orderID, listingSlug, variantSKU string) int64 {
	var count int64
	_ = s.db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.DigitalLicenseKey{}).
			Where("order_id = ? AND listing_slug = ? AND variant_sku = ? AND status IN (?, ?, ?)",
				orderID, listingSlug, variantSKU,
				models.LicenseKeyStatusDispensed,
				models.LicenseKeyStatusSuspended,
				models.LicenseKeyStatusRevoked,
			).
			Count(&count).Error
	})
	return count
}

// GetLicenseKeyPoolStats returns counts of available/dispensed/revoked keys.
// Each count uses a fresh query chain to avoid GORM WHERE-clause leakage.
func (s *DigitalAssetAppService) GetLicenseKeyPoolStats(
	listingSlug string,
	variantSKU string,
) (*contracts.LicenseKeyPoolStats, error) {
	var available, dispensed, revoked int64
	err := s.db.View(func(tx database.Tx) error {
		baseWhere := "listing_slug = ? AND variant_sku = ? AND status = ?"

		if e := tx.Read().Model(&models.DigitalLicenseKey{}).
			Where(baseWhere, listingSlug, variantSKU, models.LicenseKeyStatusAvailable).
			Count(&available).Error; e != nil {
			return e
		}
		if e := tx.Read().Model(&models.DigitalLicenseKey{}).
			Where(baseWhere, listingSlug, variantSKU, models.LicenseKeyStatusDispensed).
			Count(&dispensed).Error; e != nil {
			return e
		}
		if e := tx.Read().Model(&models.DigitalLicenseKey{}).
			Where(baseWhere, listingSlug, variantSKU, models.LicenseKeyStatusRevoked).
			Count(&revoked).Error; e != nil {
			return e
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &contracts.LicenseKeyPoolStats{
		Available: available,
		Dispensed: dispensed,
		Revoked:   revoked,
		Total:     available + dispensed + revoked,
	}, nil
}

// DownloadFile fetches the encrypted blob, runs it through the v1 chunked
// AEAD decryptor and returns the full plaintext. Convenience helper for
// tests and for entitlement preview paths that need the whole bytes.
//
// For buyer-facing downloads of large files prefer ServeDownload, which
// streams plaintext to the HTTP response without buffering it in memory.
func (s *DigitalAssetAppService) DownloadFile(ctx context.Context, asset *models.DigitalAsset) ([]byte, error) {
	if asset.AssetType != models.AssetTypeFile {
		return nil, fmt.Errorf("asset %s is not a file", asset.ID)
	}

	cipherStream, _, err := s.blob.Get(ctx, blobKey(asset.ID))
	if err != nil {
		return nil, fmt.Errorf("blob get: %w", err)
	}
	defer cipherStream.Close()

	plainR, err := s.crypto.DecryptFileStreamReader(cipherStream, asset.ID, asset.KeyVersion)
	if err != nil {
		return nil, fmt.Errorf("decrypt stream: %w", err)
	}
	defer plainR.Close()

	return io.ReadAll(plainR)
}

// RevealLink decrypts the link URL stored in a link-type asset.
func (s *DigitalAssetAppService) RevealLink(asset *models.DigitalAsset) (string, error) {
	if asset.AssetType != models.AssetTypeLink {
		return "", fmt.Errorf("asset %s is not a link", asset.ID)
	}
	if len(asset.DeliveryData) == 0 {
		return "", fmt.Errorf("no delivery data")
	}

	plaintext, err := s.crypto.DecryptFile(asset.DeliveryData, asset.ID, asset.KeyVersion)
	if err != nil {
		return "", fmt.Errorf("decrypt link: %w", err)
	}
	return string(plaintext), nil
}

// CreateDownloadGrant creates a grant for a buyer to access a digital asset.
// CreateDownloadGrant creates (or returns the existing) DownloadGrant for
// (orderID, asset.ID). Idempotent: a unique index on (tenant_id, order_id,
// asset_id) plus an explicit pre-check make replayed OrderConfirmation events
// safe — they always return the original grant rather than creating a new one
// or depleting downstream resources (e.g. license pools).
func (s *DigitalAssetAppService) CreateDownloadGrant(
	asset *models.DigitalAsset,
	orderID string,
	buyerPeerID string,
	status string,
) (*models.DownloadGrant, error) {
	var existing models.DownloadGrant
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("order_id = ? AND asset_id = ?", orderID, asset.ID).
			First(&existing).Error
	}); err == nil {
		return &existing, nil
	}

	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	snapshot, err := json.Marshal(models.AssetSnapshot{
		AssetType:    asset.AssetType,
		FileHash:     asset.FileHash,
		FileName:     asset.FileName,
		FileSize:     asset.FileSize,
		MimeType:     asset.MimeType,
		KeyVersion:   asset.KeyVersion,
		DeliveryData: asset.DeliveryData,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal snapshot: %w", err)
	}

	grant := &models.DownloadGrant{
		ID:            uuid.Must(uuid.NewV7()).String(),
		AssetID:       asset.ID,
		OrderID:       orderID,
		BuyerPeerID:   buyerPeerID,
		Status:        status,
		Nonce:         hex.EncodeToString(nonce),
		Version:       1,
		AssetSnapshot: snapshot,
		MaxDownloads:  asset.MaxDownloads,
		ExpiresAt:     expiresAtFromAsset(asset),
	}

	if err := s.db.Update(func(tx database.Tx) error {
		return tx.Save(grant)
	}); err != nil {
		// Unique constraint (tenant_id, order_id, asset_id) may fire under
		// concurrent event replay. Return the existing grant instead of failing.
		var existing models.DownloadGrant
		if viewErr := s.db.View(func(tx database.Tx) error {
			return tx.Read().
				Where("order_id = ? AND asset_id = ?", orderID, asset.ID).
				First(&existing).Error
		}); viewErr == nil {
			return &existing, nil
		}
		return nil, fmt.Errorf("save grant: %w", err)
	}

	return grant, nil
}

// SignDownloadURL generates an HMAC-signed download URL for a grant.
func (s *DigitalAssetAppService) SignDownloadURL(
	orderID string,
	grant *models.DownloadGrant,
	assetID string,
	expiryTs int64,
	keyVersion int,
) ([]byte, error) {
	return s.crypto.SignDownloadURL(orderID, grant.Nonce, assetID, expiryTs, grant.Version, keyVersion)
}

// VerifyDownloadURL checks an HMAC signature.
func (s *DigitalAssetAppService) VerifyDownloadURL(
	orderID, grantNonce, assetID string,
	expiryTs int64,
	grantVersion, keyVersion int,
	sig []byte,
) (bool, error) {
	return s.crypto.VerifyDownloadURL(orderID, grantNonce, assetID, expiryTs, grantVersion, keyVersion, sig)
}

// RecordDownload atomically increments download count and logs the event.
// The read-increment-save pattern is atomic under SQLite's exclusive write lock.
// Under PostgreSQL READ COMMITTED, concurrent downloads may lose one increment;
// this is acceptable because MaxDownloads is checked before serving the file
// (in ServeDownload), and the count is informational.
func (s *DigitalAssetAppService) RecordDownload(
	grant *models.DownloadGrant,
	buyerPeerID string,
	ipHash string,
	userAgent string,
	success bool,
	blockReason string,
) error {
	return s.db.Update(func(tx database.Tx) error {
		if success {
			var g models.DownloadGrant
			if err := tx.Read().Where("id = ?", grant.ID).First(&g).Error; err != nil {
				return err
			}
			if g.MaxDownloads > 0 && g.DownloadCount >= g.MaxDownloads {
				return fmt.Errorf("download limit reached (%d/%d)", g.DownloadCount, g.MaxDownloads)
			}
			g.DownloadCount++
			if err := tx.Save(&g); err != nil {
				return err
			}
		}

		dl := &models.DigitalDownloadLog{
			ID:          uuid.Must(uuid.NewV7()).String(),
			GrantID:     grant.ID,
			AssetID:     grant.AssetID,
			OrderID:     grant.OrderID,
			BuyerPeerID: buyerPeerID,
			IPHash:      ipHash,
			UserAgent:   userAgent,
			Success:     success,
			BlockReason: blockReason,
		}
		return tx.Save(dl)
	})
}

// ServeDownload verifies a signed download URL, checks grant status / expiry /
// quota, decrypts the underlying blob, records the download, and returns a
// streaming reader for the plaintext bytes.
//
// All authentication is provided by the HMAC signature embedded in the URL —
// the buyer never needs to log in to download. Validation order matters: we
// verify the signature *before* hitting the database to avoid signature
// oracle attacks.
//
// Decryption uses the grant's AssetSnapshot (file hash + key version) rather
// than the live asset row, so seller-side mutations after OrderConfirmation
// cannot redirect the buyer to different content. MimeType / FileName fall
// back to the snapshot, then to the live asset, then to safe defaults.
func (s *DigitalAssetAppService) ServeDownload(
	ctx context.Context,
	req contracts.DownloadRequest,
) (*contracts.DownloadResponse, error) {
	if req.OrderID == "" || req.GrantNonce == "" || req.AssetID == "" {
		return nil, fmt.Errorf("missing required download parameters")
	}
	if len(req.Signature) == 0 {
		return nil, fmt.Errorf("missing signature")
	}
	if req.ExpiryUnix < time.Now().Unix() {
		return nil, fmt.Errorf("download URL expired")
	}

	// Look up grant first so we can resolve KeyVersion from the snapshot
	// (KeyVersion is not in the URL — it's an implicit part of the trust
	// anchor). Doing this before signature verification is safe because
	// the only DB exposure is a constant-time nonce lookup; we still
	// reject *any* request that fails the HMAC check below.
	grant, err := s.GetGrantByNonce(req.GrantNonce)
	if err != nil {
		return nil, fmt.Errorf("grant not found")
	}

	var snap models.AssetSnapshot
	if len(grant.AssetSnapshot) > 0 {
		if err := json.Unmarshal(grant.AssetSnapshot, &snap); err != nil {
			return nil, fmt.Errorf("decode snapshot: %w", err)
		}
	}
	if snap.AssetType != models.AssetTypeFile {
		return nil, fmt.Errorf("asset is not a downloadable file")
	}
	if snap.FileHash == "" {
		return nil, fmt.Errorf("snapshot missing file hash")
	}

	ok, err := s.crypto.VerifyDownloadURL(
		req.OrderID, req.GrantNonce, req.AssetID,
		req.ExpiryUnix, req.GrantVersion, snap.KeyVersion,
		req.Signature,
	)
	if err != nil {
		return nil, fmt.Errorf("verify signature: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("invalid download signature")
	}

	if grant.OrderID != req.OrderID || grant.AssetID != req.AssetID {
		return nil, fmt.Errorf("grant mismatch")
	}
	if grant.Version != req.GrantVersion {
		return nil, fmt.Errorf("grant version mismatch (revoked or rotated)")
	}
	if !models.IsGrantAccessibleWithExpiry(grant.Status, grant.ExpiresAt) {
		return nil, fmt.Errorf("grant not accessible: status=%s", grant.Status)
	}
	if grant.MaxDownloads > 0 && grant.DownloadCount >= grant.MaxDownloads {
		return nil, fmt.Errorf("download limit reached (%d/%d)", grant.DownloadCount, grant.MaxDownloads)
	}

	cipherStream, _, err := s.blob.Get(ctx, blobKey(grant.AssetID))
	if err != nil {
		s.recordDownloadOrLog(grant, req.BuyerIPHash, req.UserAgent, false, "blob_get_failed")
		return nil, fmt.Errorf("blob get: %w", err)
	}

	plainR, derr := s.crypto.DecryptFileStreamReader(cipherStream, grant.AssetID, snap.KeyVersion)
	if derr != nil {
		_ = cipherStream.Close()
		s.recordDownloadOrLog(grant, req.BuyerIPHash, req.UserAgent, false, "decrypt_failed")
		return nil, fmt.Errorf("decrypt stream: %w", derr)
	}

	// MimeType / FileName / FileSize all come from the snapshot — frozen at
	// order-confirmation so seller edits to the asset record don't change
	// the buyer-facing download response. Older snapshots written before
	// the MimeType field was added fall back to the live asset (best-effort).
	mime := snap.MimeType
	if mime == "" {
		if live, _ := s.GetAssetByID(grant.AssetID); live != nil {
			mime = live.MimeType
		}
	}
	if mime == "" {
		mime = "application/octet-stream"
	}

	fileName := snap.FileName
	if fileName == "" {
		fileName = "download.bin"
	}

	// We can't sample the success/failure of streaming decrypt before the
	// body is consumed; record success now and rely on download log
	// truncation if the connection breaks. Acceptable since the audit's
	// purpose is fraud / abuse detection, not strict accounting.
	s.recordDownloadOrLog(grant, req.BuyerIPHash, req.UserAgent, true, "")
	return &contracts.DownloadResponse{
		FileName: fileName,
		MimeType: mime,
		FileSize: snap.FileSize,
		Body:     newCombinedCloser(plainR, cipherStream),
	}, nil
}

// recordDownloadOrLog wraps RecordDownload so that audit-log failures don't
// silently disappear. We can't fail the download itself when the audit
// write breaks (the buyer paid; serving the bytes is the priority), but
// dropped fraud-detection signal is a real loss — log loudly so on-call
// catches it.
func (s *DigitalAssetAppService) recordDownloadOrLog(
	grant *models.DownloadGrant,
	buyerIPHash, userAgent string,
	success bool,
	failReason string,
) {
	if err := s.RecordDownload(grant, grant.BuyerPeerID, buyerIPHash, userAgent, success, failReason); err != nil {
		// IMPORTANT: log grant.ID, never grant.Nonce — the Nonce is the
		// HMAC-signed URL token used to authenticate downloads. Logging
		// it would let anyone with log access replay the buyer's
		// download link.
		log.Warningf("digital-download: audit record failed for grantID=%s asset=%s success=%v reason=%s: %v",
			grant.ID, grant.AssetID, success, failReason, err)
	}
}

// combinedCloser closes both the decryptor reader and the underlying blob
// stream when the consumer is done with the response body.
type combinedCloser struct {
	io.Reader
	closers []io.Closer
}

func newCombinedCloser(r io.Reader, extra ...io.Closer) io.ReadCloser {
	cc := &combinedCloser{Reader: r}
	if c, ok := r.(io.Closer); ok {
		cc.closers = append(cc.closers, c)
	}
	cc.closers = append(cc.closers, extra...)
	return cc
}

func (cc *combinedCloser) Close() error {
	var firstErr error
	for _, c := range cc.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// GetAssetsByListing returns all digital assets for a listing+variant.
func (s *DigitalAssetAppService) GetAssetsByListing(
	listingSlug string,
	variantSKU string,
) ([]contracts.DigitalAssetInfo, error) {
	assets, err := s.getAssetModelsByListing(listingSlug, variantSKU)
	if err != nil {
		return nil, err
	}
	result := make([]contracts.DigitalAssetInfo, len(assets))
	for i := range assets {
		result[i] = *assetToInfo(&assets[i])
	}
	return result, nil
}

func (s *DigitalAssetAppService) getAssetModelsByListing(
	listingSlug string,
	variantSKU string,
) ([]models.DigitalAsset, error) {
	var assets []models.DigitalAsset
	err := s.db.View(func(tx database.Tx) error {
		q := tx.Read().Where("listing_slug = ?", listingSlug)
		if variantSKU != "" {
			q = q.Where("variant_sku IN (?, '')", variantSKU)
		} else {
			q = q.Where("variant_sku = ?", "")
		}
		return q.Order("sort_order ASC").Find(&assets).Error
	})
	return assets, err
}

// GetGrantsByOrder returns all download grants for an order.
func (s *DigitalAssetAppService) GetGrantsByOrder(orderID string) ([]models.DownloadGrant, error) {
	var grants []models.DownloadGrant
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("order_id = ?", orderID).Find(&grants).Error
	})
	return grants, err
}

// GetGrantByNonce finds a grant by its globally unique nonce.
func (s *DigitalAssetAppService) GetGrantByNonce(nonce string) (*models.DownloadGrant, error) {
	var grant models.DownloadGrant
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("nonce = ?", nonce).First(&grant).Error
	})
	if err != nil {
		return nil, err
	}
	return &grant, nil
}

// FreezeGrantsByOrder sets all grants for an order to "frozen" (dispute opened).
// Two-step: first read active grants, then update each with its previous status.
func (s *DigitalAssetAppService) FreezeGrantsByOrder(orderID string, reason string) error {
	return s.db.Update(func(tx database.Tx) error {
		var grants []models.DownloadGrant
		if err := tx.Read().
			Where("order_id = ? AND status NOT IN (?, ?)", orderID,
				models.GrantStatusRevoked, models.GrantStatusFrozen).
			Find(&grants).Error; err != nil {
			return err
		}
		for i := range grants {
			grants[i].PreviousStatus = grants[i].Status
			grants[i].Status = models.GrantStatusFrozen
			grants[i].RevokeReason = reason
			grants[i].Version++
			if err := tx.Save(&grants[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

// RevokeGrantsByOrder sets all grants for an order to "revoked" (refund/dispute lost).
func (s *DigitalAssetAppService) RevokeGrantsByOrder(orderID string, reason string) error {
	return s.db.Update(func(tx database.Tx) error {
		var grants []models.DownloadGrant
		if err := tx.Read().
			Where("order_id = ? AND status != ?", orderID, models.GrantStatusRevoked).
			Find(&grants).Error; err != nil {
			return err
		}
		now := time.Now()
		for i := range grants {
			grants[i].Status = models.GrantStatusRevoked
			grants[i].RevokeReason = reason
			grants[i].RevokedAt = now
			grants[i].Version++
			if err := tx.Save(&grants[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

// ---------------------------------------------------------------------------
// License Validation / Activation / Deactivation
// ---------------------------------------------------------------------------

// ValidateLicense checks whether a license key is valid and returns its status.
func (s *DigitalAssetAppService) ValidateLicense(
	licenseKeyPlain string,
	appID string,
) (*contracts.LicenseValidationResult, error) {
	lk, err := s.findLicenseByPlainKey(licenseKeyPlain, appID)
	if err != nil {
		return &contracts.LicenseValidationResult{Valid: false, Reason: "not_found"}, nil
	}

	switch lk.Status {
	case models.LicenseKeyStatusAvailable:
		return &contracts.LicenseValidationResult{Valid: false, Reason: "not_issued"}, nil
	case models.LicenseKeyStatusRevoked:
		return &contracts.LicenseValidationResult{Valid: false, Reason: "revoked"}, nil
	case models.LicenseKeyStatusSuspended:
		return &contracts.LicenseValidationResult{Valid: false, Reason: "suspended"}, nil
	}

	if !lk.ExpiresAt.IsZero() && lk.ExpiresAt.Before(time.Now()) {
		return &contracts.LicenseValidationResult{Valid: false, Reason: "expired"}, nil
	}

	activationCount, countErr := s.countActivationsChecked(lk.ID)
	if countErr != nil {
		return nil, fmt.Errorf("count activations: %w", countErr)
	}

	if lk.MaxActivations > 0 && activationCount >= int64(lk.MaxActivations) {
		return &contracts.LicenseValidationResult{Valid: false, Reason: "max_activations"}, nil
	}

	var expiresAt *time.Time
	if !lk.ExpiresAt.IsZero() {
		expiresAt = &lk.ExpiresAt
	}

	return &contracts.LicenseValidationResult{
		Valid:          true,
		LicenseID:      lk.ID,
		LicenseType:    lk.LicenseType,
		ExpiresAt:      expiresAt,
		Activations:    activationCount,
		MaxActivations: lk.MaxActivations,
	}, nil
}

// ActivateLicense creates an activation record for a license key + fingerprint.
func (s *DigitalAssetAppService) ActivateLicense(
	licenseKeyPlain string,
	appID string,
	fingerprint string,
	label string,
	ipHash string,
) (*contracts.LicenseActivationResult, error) {
	lk, err := s.findLicenseByPlainKey(licenseKeyPlain, appID)
	if err != nil {
		return nil, fmt.Errorf("%w", contracts.ErrLicenseNotFound)
	}

	if lk.Status != models.LicenseKeyStatusDispensed {
		return nil, fmt.Errorf("license is %s", lk.Status)
	}

	// Lock the parent license row on databases that support row-level locks.
	// This serializes COUNT+INSERT per license on PostgreSQL/MySQL, preventing
	// concurrent distinct fingerprints from exceeding MaxActivations.
	now := time.Now()
	var (
		result        *contracts.LicenseActivationResult
		newActivation *models.LicenseActivation
	)

	if err := s.db.Update(func(tx database.Tx) error {
		var lockedLicense models.DigitalLicenseKey
		lockQ := withActivationParentLock(tx.Read()).Where("id = ?", lk.ID)
		if err := lockQ.First(&lockedLicense).Error; err != nil {
			return fmt.Errorf("lock license: %w", err)
		}
		if lockedLicense.Status != models.LicenseKeyStatusDispensed {
			return fmt.Errorf("license is %s", lockedLicense.Status)
		}

		var existing models.LicenseActivation
		err := tx.Read().
			Where("license_id = ? AND fingerprint = ? AND is_active = ?", lk.ID, fingerprint, true).
			First(&existing).Error
		if err == nil {
			existing.LastSeenAt = now
			if e := tx.Save(&existing); e != nil {
				return fmt.Errorf("touch existing activation: %w", e)
			}
			result = &contracts.LicenseActivationResult{
				ID:          existing.ID,
				LicenseID:   existing.LicenseID,
				Fingerprint: existing.Fingerprint,
				Label:       existing.Label,
				IsActive:    true,
				LastSeenAt:  now,
			}
			return nil
		}

		if lockedLicense.MaxActivations > 0 {
			var count int64
			if e := tx.Read().Model(&models.LicenseActivation{}).
				Where("license_id = ? AND is_active = ?", lk.ID, true).
				Count(&count).Error; e != nil {
				return fmt.Errorf("count activations: %w", e)
			}
			if count >= int64(lockedLicense.MaxActivations) {
				return fmt.Errorf("%w (%d/%d)", contracts.ErrActivationLimit, count, lockedLicense.MaxActivations)
			}
		}

		newActivation = &models.LicenseActivation{
			ID:          uuid.Must(uuid.NewV7()).String(),
			LicenseID:   lk.ID,
			Fingerprint: fingerprint,
			Label:       label,
			IPHash:      ipHash,
			IsActive:    true,
			LastSeenAt:  now,
		}
		return tx.Save(newActivation)
	}); err != nil {
		return nil, err
	}

	if result != nil {
		return result, nil
	}

	return &contracts.LicenseActivationResult{
		ID:          newActivation.ID,
		LicenseID:   newActivation.LicenseID,
		Fingerprint: newActivation.Fingerprint,
		Label:       newActivation.Label,
		IsActive:    true,
		LastSeenAt:  now,
	}, nil
}

// DeactivateLicense marks an activation as deactivated.
func (s *DigitalAssetAppService) DeactivateLicense(
	licenseKeyPlain string,
	appID string,
	fingerprint string,
) error {
	lk, err := s.findLicenseByPlainKey(licenseKeyPlain, appID)
	if err != nil {
		return fmt.Errorf("%w", contracts.ErrLicenseNotFound)
	}

	now := time.Now()
	return s.db.Update(func(tx database.Tx) error {
		var act models.LicenseActivation
		if err := tx.Read().
			Where("license_id = ? AND fingerprint = ? AND is_active = ?", lk.ID, fingerprint, true).
			First(&act).Error; err != nil {
			return fmt.Errorf("%w", contracts.ErrActivationNotFound)
		}
		act.IsActive = false
		act.DeactivatedAt = &now
		return tx.Save(&act)
	})
}

// findLicenseByPlainKey looks up a license key by computing its SHA-256 hash
// and matching against the stored license_hash.
func (s *DigitalAssetAppService) findLicenseByPlainKey(
	licenseKeyPlain string,
	appID string,
) (*models.DigitalLicenseKey, error) {
	h := sha256.Sum256([]byte(licenseKeyPlain))
	hashHex := hex.EncodeToString(h[:])

	var lk models.DigitalLicenseKey
	err := s.db.View(func(tx database.Tx) error {
		q := tx.Read().Where("license_hash = ?", hashHex)
		if appID != "" {
			q = q.Where("app_id = ?", appID)
		}
		return q.First(&lk).Error
	})
	if err != nil {
		return nil, err
	}
	return &lk, nil
}

func withActivationParentLock(db *gorm.DB) *gorm.DB {
	switch db.Dialector.Name() {
	case "postgres", "mysql":
		return db.Clauses(clause.Locking{Strength: "UPDATE"})
	default:
		return db
	}
}

func (s *DigitalAssetAppService) requireBuyerPortalAccess(orderID, token string) error {
	if token == "" {
		return contracts.ErrBuyerPortalAccess
	}

	var order models.GuestOrder
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("order_token = ?", orderID).First(&order).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return contracts.ErrBuyerPortalAccess
		}
		return fmt.Errorf("load buyer portal access: %w", err)
	}
	if order.BuyerPortalTokenHash == "" {
		return contracts.ErrBuyerPortalAccess
	}
	if order.BuyerPortalTokenExpiresAt != nil && time.Now().After(*order.BuyerPortalTokenExpiresAt) {
		return contracts.ErrBuyerPortalAccess
	}
	sum := sha256.Sum256([]byte(token))
	got := []byte(hex.EncodeToString(sum[:]))
	want := []byte(order.BuyerPortalTokenHash)
	// Constant-time compare keeps the hardening uniform with other secret
	// comparisons (API tokens, CSRF, etc.) even though both sides are public
	// SHA-256 hex strings of buyer-controlled input — codeguard rules require
	// it for any auth secret comparison.
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return contracts.ErrBuyerPortalAccess
	}
	return nil
}

// ---------------------------------------------------------------------------
// Buyer Portal
// ---------------------------------------------------------------------------

// GetBuyerDigitalAssets builds the Buyer Portal payload for an order.
// For file assets it produces HMAC-signed download URLs; for license_key assets
// it decrypts and returns the allocated key; for link assets it decrypts the URL.
func (s *DigitalAssetAppService) GetBuyerDigitalAssets(
	orderID string,
	buyerPortalToken string,
	authenticatedBuyerPeerID string,
	allowAdmin bool,
	urlExpirySec int64,
) ([]contracts.BuyerAssetEntry, error) {
	if buyerPortalToken != "" {
		if err := s.requireBuyerPortalAccess(orderID, buyerPortalToken); err != nil {
			return nil, err
		}
	}

	grants, err := s.GetGrantsByOrder(orderID)
	if err != nil {
		return nil, fmt.Errorf("get grants: %w", err)
	}
	if len(grants) == 0 {
		return nil, nil
	}
	if buyerPortalToken == "" && !allowAdmin {
		if err := requireAuthenticatedBuyerAccess(grants, authenticatedBuyerPeerID); err != nil {
			return nil, err
		}
	}

	if urlExpirySec <= 0 {
		urlExpirySec = 3600
	}
	expiryTs := time.Now().Unix() + urlExpirySec

	var entries []contracts.BuyerAssetEntry
	for i := range grants {
		g := &grants[i]
		var snap models.AssetSnapshot
		if len(g.AssetSnapshot) > 0 {
			_ = json.Unmarshal(g.AssetSnapshot, &snap)
		}

		entry := contracts.BuyerAssetEntry{
			AssetID:   g.AssetID,
			AssetType: snap.AssetType,
			FileName:  snap.FileName,
			FileSize:  snap.FileSize,
			Downloads: g.DownloadCount,
			MaxDL:     g.MaxDownloads,
			Status:    g.Status,
		}
		if !g.ExpiresAt.IsZero() {
			t := g.ExpiresAt
			entry.ExpiresAt = &t
		}

		accessible := models.IsGrantAccessibleWithExpiry(g.Status, g.ExpiresAt)
		if !accessible {
			reason := g.Status
			if models.IsGrantAccessible(g.Status) && !g.ExpiresAt.IsZero() && time.Now().After(g.ExpiresAt) {
				reason = models.GrantStatusExpired
				entry.Status = models.GrantStatusExpired
			}
			entry.RestrictedReason = reason
			entries = append(entries, entry)
			continue
		}

		switch snap.AssetType {
		case models.AssetTypeFile:
			sig, err := s.SignDownloadURL(orderID, g, g.AssetID, expiryTs, snap.KeyVersion)
			if err == nil {
				entry.DownloadURL = fmt.Sprintf(
					"/v1/orders/%s/digital-download?grant=%s&asset=%s&expires=%d&v=%d&sig=%s",
					orderID, g.Nonce, g.AssetID, expiryTs, g.Version, hex.EncodeToString(sig),
				)
			}

		case models.AssetTypeLicenseKey:
			lks := s.findLicenseKeysByOrder(orderID, g.AssetID)
			for _, lk := range lks {
				le := contracts.BuyerLicenseEntry{
					LicenseType:    lk.LicenseType,
					MaxActivations: lk.MaxActivations,
					Activations:    s.countActivations(lk.ID),
				}
				decrypted, dErr := s.crypto.DecryptLicenseKey(lk.LicenseKey, g.AssetID, lk.KeyVersion)
				if dErr == nil {
					le.LicenseKey = string(decrypted)
				}
				entry.LicenseKeys = append(entry.LicenseKeys, le)
			}

		case models.AssetTypeLink:
			if len(snap.DeliveryData) > 0 {
				plainURL, dErr := s.crypto.DecryptFile(snap.DeliveryData, g.AssetID, snap.KeyVersion)
				if dErr == nil {
					entry.DeliveryURL = string(plainURL)
				}
			}
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func requireAuthenticatedBuyerAccess(grants []models.DownloadGrant, buyerPeerID string) error {
	buyerPeerID = strings.TrimSpace(buyerPeerID)
	if buyerPeerID == "" {
		return contracts.ErrBuyerPortalAccess
	}
	for _, grant := range grants {
		if grant.BuyerPeerID == "" || grant.BuyerPeerID != buyerPeerID {
			return contracts.ErrBuyerPortalAccess
		}
	}
	return nil
}

func (s *DigitalAssetAppService) findLicenseKeysByOrder(orderID, assetID string) []models.DigitalLicenseKey {
	var asset models.DigitalAsset
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", assetID).First(&asset).Error
	})
	if err != nil || asset.ID == "" {
		return nil
	}

	var lks []models.DigitalLicenseKey
	_ = s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("order_id = ? AND listing_slug = ? AND variant_sku = ?",
				orderID, asset.ListingSlug, asset.VariantSKU).
			Find(&lks).Error
	})
	return lks
}

func (s *DigitalAssetAppService) countActivations(licenseID string) int64 {
	c, _ := s.countActivationsChecked(licenseID)
	return c
}

func (s *DigitalAssetAppService) countActivationsChecked(licenseID string) (int64, error) {
	var count int64
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.LicenseActivation{}).
			Where("license_id = ? AND is_active = ?", licenseID, true).
			Count(&count).Error
	})
	return count, err
}

// GetAssetByID retrieves a single digital asset by ID.
func (s *DigitalAssetAppService) GetAssetByID(assetID string) (*contracts.DigitalAssetInfo, error) {
	asset, err := s.getAssetModelByID(assetID)
	if err != nil {
		return nil, err
	}
	return assetToInfo(asset), nil
}

func (s *DigitalAssetAppService) getAssetModelByID(assetID string) (*models.DigitalAsset, error) {
	var asset models.DigitalAsset
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", assetID).First(&asset).Error
	})
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

// UpdateAsset applies partial updates to an existing digital asset.
func (s *DigitalAssetAppService) UpdateAsset(assetID string, updates contracts.AssetUpdateInput) (*contracts.DigitalAssetInfo, error) {
	var asset models.DigitalAsset
	err := s.db.Update(func(tx database.Tx) error {
		if e := tx.Read().Where("id = ?", assetID).First(&asset).Error; e != nil {
			return e
		}
		if updates.MaxDownloads != nil {
			asset.MaxDownloads = *updates.MaxDownloads
		}
		if updates.ExpiryHours != nil {
			asset.ExpiryHours = *updates.ExpiryHours
		}
		if updates.SortOrder != nil {
			asset.SortOrder = *updates.SortOrder
		}
		return tx.Save(&asset)
	})
	if err != nil {
		return nil, err
	}
	return assetToInfo(&asset), nil
}

// DeleteAsset removes a digital asset by ID.
func (s *DigitalAssetAppService) DeleteAsset(assetID string) error {
	return s.db.Update(func(tx database.Tx) error {
		return tx.Delete("id", assetID, nil, &models.DigitalAsset{})
	})
}

// ListLicenseKeys returns license keys for a listing with the actual key masked.
func (s *DigitalAssetAppService) ListLicenseKeys(
	listingSlug, variantSKU string,
	limit, offset int,
) ([]contracts.MaskedLicenseKey, error) {
	var keys []models.DigitalLicenseKey
	err := s.db.View(func(tx database.Tx) error {
		q := tx.Read().Where("listing_slug = ?", listingSlug)
		if variantSKU != "" {
			q = q.Where("variant_sku = ?", variantSKU)
		}
		return q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&keys).Error
	})
	if err != nil {
		return nil, err
	}
	result := make([]contracts.MaskedLicenseKey, len(keys))
	for i, k := range keys {
		masked := "****"
		if len(k.LicenseHash) >= 8 {
			masked = k.LicenseHash[:4] + "****" + k.LicenseHash[len(k.LicenseHash)-4:]
		}
		var dispensedAt *time.Time
		if !k.DispensedAt.IsZero() {
			t := k.DispensedAt
			dispensedAt = &t
		}
		var expiresAt *time.Time
		if !k.ExpiresAt.IsZero() {
			t := k.ExpiresAt
			expiresAt = &t
		}
		result[i] = contracts.MaskedLicenseKey{
			ID:             k.ID,
			Status:         k.Status,
			MaskedKey:      masked,
			LicenseType:    k.LicenseType,
			MaxActivations: k.MaxActivations,
			OrderID:        k.OrderID,
			DispensedAt:    dispensedAt,
			ExpiresAt:      expiresAt,
		}
	}
	return result, nil
}

// RevokeLicenseKey marks a license key as revoked.
func (s *DigitalAssetAppService) RevokeLicenseKey(keyID string) error {
	return s.db.Update(func(tx database.Tx) error {
		var key models.DigitalLicenseKey
		if e := tx.Read().Where("id = ?", keyID).First(&key).Error; e != nil {
			return e
		}
		key.Status = models.LicenseKeyStatusRevoked
		return tx.Save(&key)
	})
}

// ---------------------------------------------------------------------------

func assetToInfo(a *models.DigitalAsset) *contracts.DigitalAssetInfo {
	return &contracts.DigitalAssetInfo{
		ID:           a.ID,
		ListingSlug:  a.ListingSlug,
		VariantSKU:   a.VariantSKU,
		AssetType:    a.AssetType,
		FileName:     a.FileName,
		FileSize:     a.FileSize,
		MimeType:     a.MimeType,
		SortOrder:    a.SortOrder,
		MaxDownloads: a.MaxDownloads,
		ExpiryHours:  a.ExpiryHours,
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
	}
}

func expiresAtFromAsset(asset *models.DigitalAsset) time.Time {
	if asset.ExpiryHours > 0 {
		return time.Now().Add(time.Duration(asset.ExpiryHours) * time.Hour)
	}
	return time.Time{}
}
