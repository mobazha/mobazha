package core

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/encryption"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/rs/zerolog"
	"go.mau.fi/util/dbutil"
	"maunium.net/go/mautrix"
	mxcrypto "maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/crypto/verificationhelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const (
	matrixSyncTimeout     = 30 * time.Second
	matrixEventBufSize    = 256
	matrixDefaultServer   = "matrix.mobazha.org"
	matrixIdleTimeout     = 30 * time.Minute
	matrixIdleCheckPeriod = 5 * time.Minute
)

// MautrixChatServiceConfig holds configuration for creating a MautrixChatService.
type MautrixChatServiceConfig struct {
	DB                 database.Database
	PrivKey            crypto.PrivKey
	PeerID             peer.ID
	NodeCtx            context.Context // node lifecycle context; sync/idle goroutines exit when cancelled
	HomeserverURL      string          // e.g. "https://matrix.mobazha.org" or internal URL
	ServerName         string          // e.g. "matrix.mobazha.org"
	DBPath             string          // path for crypto state DB (SQLite) in standalone mode
	RegistrationSecret string          // Synapse shared secret for auto-registering Matrix users
	SDKLogLevel        string          // off|info|debug (defaults to off)

	// CryptoStore overrides the default SQLite crypto store when non-nil.
	// Accepts *dbutil.Database for shared PostgreSQL (SaaS multi-tenant).
	// When nil, falls back to SQLite at DBPath.
	CryptoStore interface{}
	// CryptoDBAccountID isolates crypto state per tenant in shared PG.
	// Typically set to peerID when CryptoStore is non-nil.
	CryptoDBAccountID string
}

// mautrixChatService implements contracts.MatrixChatService using mautrix-go.
// Supports lazy initialization (Start on first API call) and idle auto-stop
// (stops sync after matrixIdleTimeout of inactivity, auto-restarts on next call).
type mautrixChatService struct {
	config MautrixChatServiceConfig

	client       *mautrix.Client
	cryptoHelper *cryptohelper.CryptoHelper

	matrixUserID id.UserID
	password     string
	serverName   string
	pickleKey    []byte

	subs   []chan contracts.MatrixChatEvent
	subsMu sync.Mutex

	syncCtx    context.Context
	syncCancel context.CancelFunc

	ready   atomic.Bool
	stopped atomic.Bool

	lastActivity atomic.Int64
	parentCtx    context.Context
	idleCancel   context.CancelFunc

	chatSettings contracts.ChatSettings

	verifyHelper *verificationhelper.VerificationHelper

	verificationReady  atomic.Bool
	verificationReason atomic.Value

	// unreadCounts tracks per-room notification counts from the sync loop.
	// Updated on every /sync response; read by GetRooms.
	unreadCounts   map[id.RoomID]int
	unreadCountsMu sync.RWMutex

	// firstSyncCh is created when the service resumes from idle and closed
	// once the first /sync response is processed. ensureReady() waits on
	// this channel so that GetRooms() always reads fresh unreadCounts.
	firstSyncCh chan struct{}

	directRoomCreateMu sync.Mutex

	mu sync.RWMutex
}

type matrixSDKLogWriter struct {
	userID string
}

func (w matrixSDKLogWriter) Write(p []byte) (int, error) {
	return w.WriteLevel(zerolog.InfoLevel, p)
}

func (w matrixSDKLogWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}
	if w.userID != "" {
		msg = fmt.Sprintf("matrix-sdk user=%s %s", w.userID, msg)
	} else {
		msg = "matrix-sdk " + msg
	}

	switch level {
	case zerolog.TraceLevel, zerolog.DebugLevel:
		log.Debug(msg)
	case zerolog.InfoLevel:
		log.Info(msg)
	case zerolog.WarnLevel:
		log.Warning(msg)
	default:
		log.Error(msg)
	}
	return len(p), nil
}

func normalizeMatrixSDKLogLevel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "off":
		return "off"
	case "info":
		return "info"
	case "debug":
		return "debug"
	default:
		log.Warningf("Unknown matrix sdk log level %q, fallback to off", raw)
		return "off"
	}
}

func (s *mautrixChatService) setVerificationStatus(ready bool, reason string) {
	s.verificationReady.Store(ready)
	s.verificationReason.Store(strings.TrimSpace(reason))
}

func (s *mautrixChatService) verificationError(reason string) error {
	if reason != "" {
		return fmt.Errorf("%s: %w", reason, contracts.ErrVerificationUnavailable)
	}
	return contracts.ErrVerificationUnavailable
}

func (s *mautrixChatService) getVerificationStatus() (available bool, reason string) {
	if s.verifyHelper != nil && s.verificationReady.Load() {
		return true, ""
	}
	if raw := s.verificationReason.Load(); raw != nil {
		if msg, ok := raw.(string); ok && strings.TrimSpace(msg) != "" {
			return false, msg
		}
	}
	if s.verifyHelper == nil {
		return false, "verification helper not initialized"
	}
	return false, "cross-signing is not ready"
}

func (s *mautrixChatService) configureMautrixClientLogger() string {
	level := normalizeMatrixSDKLogLevel(s.config.SDKLogLevel)
	if level == "off" || s.client == nil {
		return level
	}

	zerologLevel := zerolog.InfoLevel
	if level == "debug" {
		zerologLevel = zerolog.DebugLevel
		s.client.SyncTraceLog = true
	}

	w := matrixSDKLogWriter{userID: s.matrixUserID.String()}
	s.client.Log = zerolog.New(w).Level(zerologLevel).With().Timestamp().Logger()
	return level
}

// NewMautrixChatService creates a new mautrix-backed Matrix chat service.
func NewMautrixChatService(cfg MautrixChatServiceConfig) (*mautrixChatService, error) {
	privKeyBytes, err := cfg.PrivKey.Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to get private key bytes: %w", err)
	}

	password, err := encryption.DeriveMatrixPassword(privKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to derive matrix password: %w", err)
	}

	pickleKey := encryption.DeriveMatrixPickleKey(privKeyBytes)

	serverName := cfg.ServerName
	if serverName == "" {
		serverName = matrixDefaultServer
	}
	homeserverURL := cfg.HomeserverURL
	if homeserverURL == "" {
		homeserverURL = "https://" + serverName
	}

	peerIDStr := cfg.PeerID.String()
	matrixUserID := id.UserID(encryption.MatrixUserIDFromPeerID(peerIDStr, serverName))

	return &mautrixChatService{
		config:       cfg,
		matrixUserID: matrixUserID,
		password:     password,
		pickleKey:    pickleKey,
		serverName:   serverName,
	}, nil
}

// Start initializes the mautrix client, logs in, sets up E2EE, and begins syncing.
// In practice, callers should not call Start directly; the service starts lazily
// on the first API call via ensureReady().
func (s *mautrixChatService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ready.Load() {
		return nil
	}
	return s.startLocked(ctx)
}

// startLocked performs the full initialization. Caller must hold s.mu.
func (s *mautrixChatService) startLocked(ctx context.Context) error {
	homeserverURL := s.config.HomeserverURL
	if homeserverURL == "" {
		homeserverURL = "https://" + s.serverName
	}

	client, err := mautrix.NewClient(homeserverURL, s.matrixUserID, "")
	if err != nil {
		return fmt.Errorf("failed to create mautrix client: %w", err)
	}
	s.client = client
	sdkLogLevel := s.configureMautrixClientLogger()
	if sdkLogLevel != "off" {
		log.Infof("Matrix SDK logs enabled: user=%s level=%s", s.matrixUserID, sdkLogLevel)
	}

	startFailed := true
	defer func() {
		if startFailed {
			s.client = nil
		}
	}()

	stableDeviceID := id.DeviceID("MBZ_" + s.config.PeerID.String())

	loginReq := &mautrix.ReqLogin{
		Type: mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{
			Type: mautrix.IdentifierTypeUser,
			User: s.matrixUserID.String(),
		},
		Password:         s.password,
		DeviceID:         stableDeviceID,
		StoreCredentials: true,
	}
	err = s.loginWithRetry(ctx, loginReq)
	if err != nil {
		if s.config.RegistrationSecret != "" && isForbiddenOrNotFound(err) {
			log.Infof("Matrix user %s does not exist, auto-registering...", s.matrixUserID)
			if regErr := s.registerUser(ctx); regErr != nil {
				return fmt.Errorf("matrix auto-register failed: %w (original login error: %v)", regErr, err)
			}
			err = s.loginWithRetry(ctx, loginReq)
		}
		if err != nil {
			return fmt.Errorf("matrix login failed: %w", err)
		}
	}
	s.persistMatrixCredentials()

	var cryptoStoreArg interface{}
	if s.usesSharedCryptoStore() {
		cryptoStoreArg = s.config.CryptoStore
	} else {
		dbPath := s.config.DBPath
		if dbPath == "" {
			dbPath = "mautrix_crypto.db"
		}
		cryptoDB, dbErr := openMatrixCryptoDB(dbPath)
		if dbErr != nil {
			return fmt.Errorf("failed to open matrix crypto DB: %w", dbErr)
		}
		cryptoStoreArg = cryptoDB
	}

	cryptoHelper, err := cryptohelper.NewCryptoHelper(s.client, s.pickleKey, cryptoStoreArg)
	if err != nil {
		return fmt.Errorf("failed to create crypto helper: %w", err)
	}
	if s.config.CryptoDBAccountID != "" {
		cryptoHelper.DBAccountID = s.config.CryptoDBAccountID
	}
	cryptoHelper.DecryptErrorCallback = s.handleDecryptError
	s.cryptoHelper = cryptoHelper

	if err := s.cryptoHelper.Init(ctx); err != nil {
		resettable := strings.Contains(err.Error(), "mismatching device ID") ||
			strings.Contains(err.Error(), "not marked as shared") ||
			strings.Contains(err.Error(), "keys seem to have disappeared")
		if resettable {
			log.Warningf("Crypto store state mismatch (device=%s, account=%s), resetting crypto state: %v",
				stableDeviceID, s.config.CryptoDBAccountID, err)
			if resetErr := s.resetCryptoDB(ctx, cryptoStoreArg); resetErr != nil {
				return fmt.Errorf("failed to reset crypto DB after state mismatch: %w (original: %v)", resetErr, err)
			}
		} else {
			return fmt.Errorf("failed to init crypto helper: %w", err)
		}
	}
	s.client.Crypto = s.cryptoHelper

	mach := s.cryptoHelper.Machine()

	// Trust by Architecture: Mobazha controls all device creation (single device
	// per user, server-managed). Cross-signing/SSSS failures must never block
	// key distribution. External Matrix users (federation) keep stricter checks.
	mach.ShareKeysMinTrust = id.TrustStateUnset
	mach.AllowKeyShare = func(ctx context.Context, device *id.Device, info event.RequestedKeyInfo) *mxcrypto.KeyShareRejection {
		if device.Trust == id.TrustStateBlacklisted {
			return &mxcrypto.KeyShareRejectBlacklisted
		}
		if device.UserID == s.client.UserID {
			return nil
		}
		// Mobazha users: unconditionally trust (single server-managed device)
		if isMobazhaUser(string(device.UserID)) {
			return nil
		}
		// External Matrix users: require cross-signed trust or prior sharing
		if device.Trust == id.TrustStateCrossSignedVerified || device.Trust == id.TrustStateCrossSignedTOFU {
			return nil
		}
		if !mach.DisableSharedGroupSessionTracking {
			shared, err := mach.CryptoStore.IsOutboundGroupSessionShared(ctx, device.UserID, device.IdentityKey, info.SessionID)
			if err == nil && shared {
				return nil
			}
		}
		return &mxcrypto.KeyShareRejectNoResponse
	}
	storeDesc := "SQLite"
	if s.usesSharedCryptoStore() {
		storeDesc = fmt.Sprintf("shared-PG(account=%s)", s.config.CryptoDBAccountID)
	}
	log.Infof("Matrix crypto initialized: user=%s device=%s store=%s ShareKeysMinTrust=Unset(TrustByArch)", s.client.UserID, s.client.DeviceID, storeDesc)

	s.importKeyBackup(ctx)

	// Best-effort: cross-signing benefits external federation peers but is not
	// required for Mobazha-internal messaging (Trust by Architecture).
	verificationSupportErr := s.ensureCrossSigningSupport(ctx, mach)
	if verificationSupportErr != nil {
		log.Warningf("Matrix cross-signing unavailable for %s (non-critical): %v", s.matrixUserID, verificationSupportErr)
	}

	vh := verificationhelper.NewVerificationHelper(
		s.client, mach, nil, &verificationCallbacks{svc: s},
		false, false, true,
	)
	verificationHelperErr := vh.Init(ctx)
	if verificationHelperErr != nil {
		log.Warningf("Failed to init verification helper: %v (verification features unavailable)", verificationHelperErr)
	} else {
		s.verifyHelper = vh
		s.client.Verification = vh
	}
	switch {
	case verificationSupportErr != nil:
		s.setVerificationStatus(false, verificationSupportErr.Error())
	case verificationHelperErr != nil:
		s.setVerificationStatus(false, verificationHelperErr.Error())
	default:
		s.setVerificationStatus(true, "")
	}

	s.registerEventHandlers()
	s.loadChatSettings(ctx)

	s.parentCtx = s.config.NodeCtx
	if s.parentCtx == nil {
		s.parentCtx = context.Background()
	}
	s.syncCtx, s.syncCancel = context.WithCancel(s.parentCtx)
	go s.syncLoop()

	startFailed = false
	s.ready.Store(true)
	s.stopped.Store(false)
	s.touchActivity()

	idleCtx, idleCancel := context.WithCancel(s.parentCtx)
	s.idleCancel = idleCancel
	go s.idleWatcher(idleCtx)

	log.Infof("Matrix chat service started for %s", s.matrixUserID)
	return nil
}

// ensureReady performs lazy initialization on first API call, or resumes sync
// after an idle timeout. This is the entry point for all public methods.
func (s *mautrixChatService) ensureReady(ctx context.Context) error {
	if s.ready.Load() {
		return nil
	}
	if s.stopped.Load() {
		return fmt.Errorf("matrix chat service permanently stopped")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ready.Load() {
		return nil
	}
	if s.stopped.Load() {
		return fmt.Errorf("matrix chat service permanently stopped")
	}

	if s.client != nil && s.parentCtx != nil {
		s.firstSyncCh = make(chan struct{})
		s.syncCtx, s.syncCancel = context.WithCancel(s.parentCtx)
		go s.syncLoop()
		s.ready.Store(true)
		s.touchActivity()
		log.Infof("Matrix chat service resumed from idle for %s", s.matrixUserID)
		return nil
	}

	return s.startLocked(ctx)
}

// awaitFirstSync blocks until the first /sync response has been processed
// after an idle resume, or until the context is cancelled / timeout. This
// guarantees that unreadCounts are fresh before GetRooms reads them.
func (s *mautrixChatService) awaitFirstSync(ctx context.Context) {
	s.mu.RLock()
	ch := s.firstSyncCh
	s.mu.RUnlock()
	if ch == nil {
		return
	}
	select {
	case <-ch:
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
		log.Warningf("Matrix idle-resume first sync timed out for %s", s.matrixUserID)
	}
}

// touchActivity updates the last activity timestamp.
func (s *mautrixChatService) touchActivity() {
	s.lastActivity.Store(time.Now().UnixNano())
}

// idleStop pauses the sync loop but keeps the client, crypto, and subscribers
// alive so the service can resume quickly on the next API call.
func (s *mautrixChatService) idleStop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready.Load() || s.stopped.Load() {
		return
	}

	s.ready.Store(false)
	if s.syncCancel != nil {
		s.syncCancel()
	}
	s.firstSyncCh = nil

	if s.cryptoHelper != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		s.exportKeyBackup(ctx)
		cancel()
	}

	log.Infof("Matrix chat service idle-paused for %s", s.matrixUserID)
	s.broadcast(contracts.MatrixChatEvent{
		Type: "chat.disconnected",
		Data: map[string]string{"reason": "idle_timeout"},
	})
}

// resetCryptoDB backs up then removes the crypto store DB and reinitializes the
// crypto helper. Backup allows forensic recovery of old E2EE keys if needed.
func (s *mautrixChatService) resetCryptoDB(ctx context.Context, cryptoStoreArg interface{}) error {
	if s.usesSharedCryptoStore() {
		return s.resetCryptoDBSharedPG(ctx, cryptoStoreArg)
	}
	return s.resetCryptoDBSQLite(ctx)
}

// resetCryptoDBSQLite backs up and deletes the local SQLite crypto database,
// then recreates it fresh. Used in standalone mode.
func (s *mautrixChatService) resetCryptoDBSQLite(ctx context.Context) error {
	dbPath := s.config.DBPath
	if dbPath == "" {
		dbPath = "mautrix_crypto.db"
	}

	backupDir := dbPath + ".backup." + time.Now().Format("20060102-150405")
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		log.Warningf("Failed to create crypto DB backup dir %s: %v", backupDir, err)
	} else {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			src := dbPath + suffix
			if _, statErr := os.Stat(src); statErr == nil {
				dst := filepath.Join(backupDir, filepath.Base(src))
				if cpErr := copyFile(src, dst); cpErr != nil {
					log.Warningf("Failed to backup %s → %s: %v", src, dst, cpErr)
				}
			}
		}
		log.Infof("Crypto DB backed up to %s before reset", backupDir)
	}

	pruneOldCryptoBackups(dbPath, 3)

	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(dbPath + suffix)
	}

	// Use a fresh device ID to avoid "keys on server" conflict with the old device.
	// The orphaned old device is harmless — peers discover the new device via /keys/query.
	loginDeviceID := id.DeviceID(fmt.Sprintf("MBZ_%s_%d",
		s.config.PeerID.String(), time.Now().Unix()))
	log.Infof("Crypto reset: using fresh device ID %s", loginDeviceID)

	loginErr := s.loginWithRetry(ctx, &mautrix.ReqLogin{
		Type: mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{
			Type: mautrix.IdentifierTypeUser,
			User: s.matrixUserID.String(),
		},
		Password:         s.password,
		DeviceID:         loginDeviceID,
		StoreCredentials: true,
	})
	if loginErr != nil {
		return fmt.Errorf("failed to re-login after crypto DB reset: %w", loginErr)
	}

	s.client.StateStore = nil
	s.client.Store = nil

	cryptoDB, dbErr := openMatrixCryptoDB(dbPath)
	if dbErr != nil {
		return fmt.Errorf("failed to reopen matrix crypto DB: %w", dbErr)
	}

	cryptoHelper, err := cryptohelper.NewCryptoHelper(s.client, s.pickleKey, cryptoDB)
	if err != nil {
		return fmt.Errorf("failed to recreate crypto helper: %w", err)
	}
	cryptoHelper.DecryptErrorCallback = s.handleDecryptError
	s.cryptoHelper = cryptoHelper
	if err := s.cryptoHelper.Init(ctx); err != nil {
		return fmt.Errorf("failed to init fresh crypto helper: %w", err)
	}
	s.client.Crypto = s.cryptoHelper
	log.Infof("Crypto DB reset successful (SQLite, device=%s), new device keys established", loginDeviceID)
	return nil
}

// resetCryptoDBSharedPG clears crypto state for this tenant in the shared
// PostgreSQL database, then recreates the CryptoHelper. Used in SaaS multi-tenant mode.
func (s *mautrixChatService) resetCryptoDBSharedPG(ctx context.Context, cryptoStoreArg interface{}) error {
	if db, ok := cryptoStoreArg.(*dbutil.Database); ok {
		accountID := s.config.CryptoDBAccountID
		if accountID == "" {
			accountID = s.matrixUserID.String()
		}
		tables := []string{
			"crypto_account",
			"crypto_olm_session",
			"crypto_megolm_inbound_session",
			"crypto_megolm_outbound_session",
			"crypto_olm_message_hash",
		}
		for _, table := range tables {
			if _, err := db.RawDB.ExecContext(ctx, "DELETE FROM "+table+" WHERE account_id=$1", accountID); err != nil {
				log.Warningf("Failed to clear %s for account %s (may not exist): %v", table, accountID, err)
			}
		}
		log.Infof("Cleared crypto state for account %s from shared PG", accountID)
	} else {
		log.Warningf("Cannot clear shared PG crypto state: unexpected store type %T, will attempt clean reinit", cryptoStoreArg)
	}

	if s.client.DeviceID != "" {
		s.deleteDeviceViaAdmin(ctx, s.client.DeviceID)
	}

	stableDeviceID := id.DeviceID("MBZ_" + s.config.PeerID.String())
	err := s.loginWithRetry(ctx, &mautrix.ReqLogin{
		Type: mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{
			Type: mautrix.IdentifierTypeUser,
			User: s.matrixUserID.String(),
		},
		Password:         s.password,
		DeviceID:         stableDeviceID,
		StoreCredentials: true,
	})
	if err != nil {
		return fmt.Errorf("failed to re-login after device deletion: %w", err)
	}

	s.client.StateStore = nil
	s.client.Store = nil

	cryptoHelper, err := cryptohelper.NewCryptoHelper(s.client, s.pickleKey, cryptoStoreArg)
	if err != nil {
		return fmt.Errorf("failed to recreate crypto helper: %w", err)
	}
	if s.config.CryptoDBAccountID != "" {
		cryptoHelper.DBAccountID = s.config.CryptoDBAccountID
	}
	cryptoHelper.DecryptErrorCallback = s.handleDecryptError
	s.cryptoHelper = cryptoHelper
	if err := s.cryptoHelper.Init(ctx); err != nil {
		return fmt.Errorf("failed to init fresh crypto helper on shared PG: %w", err)
	}
	s.client.Crypto = s.cryptoHelper
	log.Infof("Crypto DB reset successful (shared PG, account=%s), new device keys established", s.config.CryptoDBAccountID)
	return nil
}

// deleteDeviceViaAdmin removes a device from the Matrix server using the
// Synapse Admin API, bypassing the interactive user-authentication flow
// required by the standard client-server DELETE endpoint.
func (s *mautrixChatService) deleteDeviceViaAdmin(ctx context.Context, deviceID id.DeviceID) bool {
	homeserverURL := s.config.HomeserverURL
	secret := s.config.RegistrationSecret
	if homeserverURL == "" || secret == "" {
		log.Warningf("Cannot delete device %s via admin: missing homeserver URL or registration secret", deviceID)
		return false
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	adminToken, err := s.obtainAdminToken(ctx, httpClient, homeserverURL, secret)
	if err != nil {
		log.Warningf("Cannot delete device %s via admin: %v", deviceID, err)
		return false
	}

	escapedUserID := url.PathEscape(s.matrixUserID.String())
	delURL := homeserverURL + "/_synapse/admin/v2/users/" + escapedUserID + "/devices/" + url.PathEscape(string(deviceID))

	req, err := http.NewRequestWithContext(ctx, "DELETE", delURL, nil)
	if err != nil {
		log.Warningf("Cannot build admin delete-device request: %v", err)
		return false
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Warningf("Admin delete-device HTTP error for %s: %v", deviceID, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		log.Infof("Deleted device %s from Matrix server via admin API", deviceID)
		return true
	}
	body, _ := io.ReadAll(resp.Body)
	log.Warningf("Admin delete-device returned HTTP %d for %s: %s", resp.StatusCode, deviceID, string(body))
	return false
}

// loginWithRetry wraps client.Login with 429 rate-limit backoff.
// It respects Synapse's retry_after_ms when available, otherwise uses 15s default.
func (s *mautrixChatService) loginWithRetry(ctx context.Context, req *mautrix.ReqLogin) error {
	const maxAttempts = 5
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := 15 * time.Second
			if retryMs, ok := extractRetryAfterMs(err); ok && retryMs > 0 {
				delay = time.Duration(retryMs+500) * time.Millisecond
			}
			log.Infof("Matrix login rate limited, retrying in %s (attempt %d/%d)", delay, attempt+1, maxAttempts)
			time.Sleep(delay)
		}
		_, err = s.client.Login(ctx, req)
		if err == nil {
			return nil
		}
		var respErr mautrix.RespError
		if !errors.As(err, &respErr) || respErr.ErrCode != "M_LIMIT_EXCEEDED" {
			return err
		}
	}
	return err
}

func extractRetryAfterMs(err error) (int, bool) {
	var respErr mautrix.RespError
	if !errors.As(err, &respErr) {
		return 0, false
	}
	if v, ok := respErr.ExtraData["retry_after_ms"]; ok {
		switch n := v.(type) {
		case float64:
			return int(n), true
		case int:
			return n, true
		}
	}
	return 0, false
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// pruneOldCryptoBackups removes old crypto DB backup directories, keeping only
// the most recent `keep` backups. Backup dirs match the pattern
// "<dbPath>.backup.YYYYMMDD-HHMMSS". Also removes legacy .bak files.
func pruneOldCryptoBackups(dbPath string, keep int) {
	dir := filepath.Dir(dbPath)
	base := filepath.Base(dbPath) + ".backup."

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var backups []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), base) {
			backups = append(backups, filepath.Join(dir, e.Name()))
		}
	}

	sort.Strings(backups)

	if len(backups) > keep {
		for _, old := range backups[:len(backups)-keep] {
			if err := os.RemoveAll(old); err != nil {
				log.Warningf("Failed to remove old crypto backup %s: %v", old, err)
			} else {
				log.Infof("Pruned old crypto backup: %s", old)
			}
		}
	}

	for _, suffix := range []string{".bak", "-shm.bak", "-wal.bak"} {
		_ = os.Remove(dbPath + suffix)
	}
}

// idleWatcher periodically checks if the service has been idle longer than
// matrixIdleTimeout and pauses the sync loop to save resources.
func (s *mautrixChatService) idleWatcher(ctx context.Context) {
	ticker := time.NewTicker(matrixIdleCheckPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !s.ready.Load() || s.stopped.Load() {
				continue
			}
			last := time.Unix(0, s.lastActivity.Load())
			if time.Since(last) > matrixIdleTimeout {
				s.idleStop()
			}
		}
	}
}

func (s *mautrixChatService) usesSharedCryptoStore() bool {
	return s.config.CryptoStore != nil
}

func (s *mautrixChatService) ownsCryptoStore() bool {
	return !s.usesSharedCryptoStore()
}

// Stop gracefully shuts down the sync loop, crypto helper, and idle watcher.
// This is a permanent stop used during node shutdown.
func (s *mautrixChatService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped.Load() {
		return nil
	}

	s.ready.Store(false)
	s.stopped.Store(true)

	if s.idleCancel != nil {
		s.idleCancel()
	}

	if s.syncCancel != nil {
		s.syncCancel()
	}

	if s.cryptoHelper != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		s.exportKeyBackup(ctx)
		cancel()
		if s.ownsCryptoStore() {
			if err := s.cryptoHelper.Close(); err != nil {
				log.Errorf("Failed to close crypto helper: %v", err)
			}
		}
	}

	s.subsMu.Lock()
	for _, ch := range s.subs {
		close(ch)
	}
	s.subs = nil
	s.subsMu.Unlock()

	log.Infof("Matrix chat service stopped for %s", s.matrixUserID)
	return nil
}

func (s *mautrixChatService) persistMatrixCredentials() {
	if s.config.DB == nil {
		return
	}

	record := &models.MatrixCredentials{
		PeerID:       s.config.PeerID.String(),
		MatrixUserID: s.matrixUserID.String(),
		ServerName:   s.serverName,
		Registered:   true,
	}
	if record.PeerID == "" || record.MatrixUserID == "" {
		return
	}

	if err := s.config.DB.Update(func(tx database.Tx) error {
		return database.SaveByBusinessKey(tx, record, "peer_id = ?", record.PeerID)
	}); err != nil {
		log.Warningf("Failed to persist Matrix credentials for %s: %v", s.matrixUserID, err)
	}
}

func (s *mautrixChatService) unlockCrossSigningFromSSSS(ctx context.Context, mach *mxcrypto.OlmMachine) error {
	keyID, keyData, err := mach.SSSS.GetDefaultKeyData(ctx)
	if err != nil {
		return fmt.Errorf("failed to get default SSSS key: %w", err)
	}
	key, err := keyData.VerifyPassphrase(keyID, s.password)
	if err != nil {
		return fmt.Errorf("failed to unlock SSSS with node-derived passphrase: %w", err)
	}
	if err := mach.FetchCrossSigningKeysFromSSSS(ctx, key); err != nil {
		return fmt.Errorf("failed to fetch cross-signing keys from SSSS: %w", err)
	}
	return nil
}

func (s *mautrixChatService) signOwnDeviceAndMaster(ctx context.Context, mach *mxcrypto.OlmMachine) error {
	if err := mach.SignOwnDevice(ctx, mach.OwnIdentity()); err != nil {
		if !errors.Is(err, mxcrypto.ErrSelfSigningKeyNotCached) {
			return fmt.Errorf("failed to sign own device: %w", err)
		}
		if unlockErr := s.unlockCrossSigningFromSSSS(ctx, mach); unlockErr != nil {
			return fmt.Errorf("failed to restore cross-signing private keys: %w", unlockErr)
		}
		if retryErr := mach.SignOwnDevice(ctx, mach.OwnIdentity()); retryErr != nil {
			return fmt.Errorf("failed to sign own device after restore: %w", retryErr)
		}
	}
	if err := mach.SignOwnMasterKey(ctx); err != nil {
		return fmt.Errorf("failed to sign own master key: %w", err)
	}
	return nil
}

func (s *mautrixChatService) bootstrapCrossSigning(ctx context.Context, mach *mxcrypto.OlmMachine) error {
	if _, _, genErr := mach.GenerateAndUploadCrossSigningKeysWithPassword(ctx, s.password, s.password); genErr != nil {
		return fmt.Errorf("failed to bootstrap cross-signing keys: %w", genErr)
	}
	if err := s.signOwnDeviceAndMaster(ctx, mach); err != nil {
		return err
	}
	return nil
}

// ensureCrossSigningSupport is best-effort: cross-signing improves trust for
// external Matrix federation users but is NOT required for Mobazha-internal
// messaging (Trust by Architecture). Failures are logged, never fatal.
func (s *mautrixChatService) ensureCrossSigningSupport(ctx context.Context, mach *mxcrypto.OlmMachine) error {
	if mach == nil {
		return fmt.Errorf("olm machine is nil")
	}

	hasKeys, isVerified, err := mach.GetOwnVerificationStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to inspect cross-signing status: %w", err)
	}
	if !hasKeys {
		return s.bootstrapCrossSigning(ctx, mach)
	}
	if isVerified {
		return nil
	}
	err = s.signOwnDeviceAndMaster(ctx, mach)
	if err == nil {
		return nil
	}
	// SSSS data may be missing or corrupted (e.g. lost m.secret_storage.default_key).
	// Re-bootstrap so external federation peers can verify us via CrossSignedTOFU.
	// This is best-effort: Mobazha-internal peers trust unconditionally.
	log.Warningf("Cross-signing restoration failed for %s (%v), re-bootstrapping keys", s.matrixUserID, err)
	return s.bootstrapCrossSigning(ctx, mach)
}

// IsReady returns true when the client is logged in and syncing.
func (s *mautrixChatService) IsReady() bool {
	return s.ready.Load()
}

// GetStatus attempts to resume the service if idle-paused, then returns the
// current connection status. This ensures that a status poll from the frontend
// wakes a sleeping service rather than permanently reporting "not connected".
func (s *mautrixChatService) GetStatus(ctx context.Context) contracts.MatrixChatStatus {
	if !s.ready.Load() && !s.stopped.Load() {
		_ = s.ensureReady(ctx)
	}
	verificationAvailable, verificationReason := s.getVerificationStatus()
	if !s.ready.Load() || s.client == nil {
		return contracts.MatrixChatStatus{
			Connected:             false,
			VerificationAvailable: verificationAvailable,
			VerificationReason:    verificationReason,
		}
	}
	return contracts.MatrixChatStatus{
		Connected:             true,
		UserID:                s.matrixUserID.String(),
		DeviceID:              s.client.DeviceID.String(),
		ServerName:            s.serverName,
		SyncRunning:           s.ready.Load(),
		VerificationAvailable: verificationAvailable,
		VerificationReason:    verificationReason,
	}
}

// Subscribe returns a channel that receives real-time chat events.
func (s *mautrixChatService) Subscribe(ctx context.Context) (<-chan contracts.MatrixChatEvent, error) {
	ch := make(chan contracts.MatrixChatEvent, matrixEventBufSize)
	s.subsMu.Lock()
	s.subs = append(s.subs, ch)
	s.subsMu.Unlock()

	go func() {
		<-ctx.Done()
		s.subsMu.Lock()
		defer s.subsMu.Unlock()
		for i, sub := range s.subs {
			if sub == ch {
				s.subs = append(s.subs[:i], s.subs[i+1:]...)
				close(ch)
				break
			}
		}
	}()

	return ch, nil
}

// broadcast sends an event to all subscribers, dropping if the channel is full.
func (s *mautrixChatService) broadcast(evt contracts.MatrixChatEvent) {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	if strings.HasPrefix(evt.Type, "chat.verification.") {
		log.Infof("Broadcasting verification event: type=%s subscribers=%d", evt.Type, len(s.subs))
	}
	for _, ch := range s.subs {
		select {
		case ch <- evt:
		default:
			log.Warningf("Dropping chat event for slow subscriber: %s", evt.Type)
		}
	}
}

// syncLoop runs the Matrix /sync long-polling loop with exponential backoff
// reconnection on transient errors. Permanent stop via context cancellation.
func (s *mautrixChatService) syncLoop() {
	const (
		minBackoff = 2 * time.Second
		maxBackoff = 60 * time.Second
	)
	log.Infof("Matrix sync loop started")
	backoff := minBackoff
	for {
		syncStart := time.Now()
		err := s.client.SyncWithContext(s.syncCtx)
		if s.syncCtx.Err() != nil {
			break
		}
		if err == nil {
			backoff = minBackoff
			continue
		}
		if time.Since(syncStart) > maxBackoff {
			backoff = minBackoff
		}
		log.Errorf("Matrix sync error (retrying in %v): %v", backoff, err)
		s.broadcast(contracts.MatrixChatEvent{
			Type: "chat.disconnected",
			Data: map[string]string{"reason": err.Error()},
		})
		select {
		case <-s.syncCtx.Done():
			log.Infof("Matrix sync loop stopped (context cancelled during backoff)")
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
	log.Infof("Matrix sync loop stopped")
}

// registerEventHandlers sets up mautrix event handlers.
func (s *mautrixChatService) registerEventHandlers() {
	syncer := s.client.Syncer.(*mautrix.DefaultSyncer)

	syncer.OnSync(s.handleSyncResponse)

	syncer.OnEventType(event.EventMessage, func(ctx context.Context, evt *event.Event) {
		log.Debugf("EventMessage received: room=%s sender=%s eventID=%s", evt.RoomID, evt.Sender, evt.ID)

		// Skip sync echo of self-sent messages — frontend already displayed them
		// when the HTTP send call returned.
		if evt.Sender == s.client.UserID {
			return
		}

		content := evt.Content.AsMessage()
		if content != nil && content.MsgType == event.MsgVerificationRequest {
			log.Infof("Verification request EventMessage detected (handled by VerificationHelper): room=%s sender=%s to=%s", evt.RoomID, evt.Sender, content.To)
			return
		}
		msg := s.eventToMessage(evt)
		if content != nil && content.RelatesTo != nil && content.RelatesTo.Type == event.RelReplace {
			s.broadcast(contracts.MatrixChatEvent{
				Type: "chat.message_edit",
				Data: map[string]interface{}{
					"roomId":     evt.RoomID.String(),
					"eventId":    content.RelatesTo.EventID.String(),
					"newContent": msg.Content,
				},
			})
			return
		}
		s.broadcast(contracts.MatrixChatEvent{
			Type: "chat.message",
			Data: msg,
		})
	})

	// Decryption failures are surfaced via cryptoHelper.DecryptErrorCallback
	// (see handleDecryptError), NOT via syncer.OnEventType(EventEncrypted).
	// Registering a listener on EventEncrypted would fire for EVERY encrypted
	// event (including those that decrypt successfully), because DefaultSyncer
	// dispatches to all listeners for a type regardless of what earlier
	// listeners do — the crypto helper's HandleEncrypted does not consume the
	// event.

	syncer.OnEventType(event.EventReaction, func(ctx context.Context, evt *event.Event) {
		if evt.Content.Parsed == nil {
			_ = evt.Content.ParseRaw(evt.Type)
		}
		reaction := evt.Content.AsReaction()
		if reaction == nil || reaction.RelatesTo.EventID == "" {
			return
		}
		s.broadcast(contracts.MatrixChatEvent{
			Type: "chat.reaction",
			Data: map[string]string{
				"roomId":    evt.RoomID.String(),
				"eventId":   evt.ID.String(),
				"sender":    evt.Sender.String(),
				"targetId":  reaction.RelatesTo.EventID.String(),
				"key":       reaction.RelatesTo.Key,
				"timestamp": fmt.Sprintf("%d", evt.Timestamp),
			},
		})
	})

	syncer.OnEventType(event.EventRedaction, func(ctx context.Context, evt *event.Event) {
		s.broadcast(contracts.MatrixChatEvent{
			Type: "chat.message_redact",
			Data: map[string]string{
				"roomId":  evt.RoomID.String(),
				"eventId": evt.Redacts.String(),
			},
		})
	})

	syncer.OnEventType(event.EphemeralEventTyping, func(ctx context.Context, evt *event.Event) {
		content := evt.Content.AsTyping()
		userIDs := make([]string, len(content.UserIDs))
		for i, uid := range content.UserIDs {
			userIDs[i] = uid.String()
		}
		s.broadcast(contracts.MatrixChatEvent{
			Type: "chat.typing",
			Data: map[string]interface{}{
				"roomId":  evt.RoomID.String(),
				"userIds": userIDs,
			},
		})
	})

	syncer.OnEventType(event.EphemeralEventReceipt, func(ctx context.Context, evt *event.Event) {
		content := evt.Content.AsReceipt()
		if content == nil {
			return
		}
		for evtID, receipts := range *content {
			for _, userReceipts := range receipts {
				for userID := range userReceipts {
					s.broadcast(contracts.MatrixChatEvent{
						Type: "chat.read_receipt",
						Data: map[string]string{
							"roomId":  evt.RoomID.String(),
							"userId":  userID.String(),
							"eventId": evtID.String(),
						},
					})
				}
			}
		}
	})

	syncer.OnEventType(event.StateMember, func(ctx context.Context, evt *event.Event) {
		content := evt.Content.AsMember()
		if content.Membership == event.MembershipInvite && evt.GetStateKey() == s.client.UserID.String() {
			s.handleInvite(ctx, evt)
		}
		s.broadcast(contracts.MatrixChatEvent{
			Type: "chat.room_member",
			Data: map[string]string{
				"roomId":     evt.RoomID.String(),
				"userId":     evt.GetStateKey(),
				"membership": string(content.Membership),
			},
		})
	})
}

// handleSyncResponse extracts unread_notifications from each /sync response.
// Called by DefaultSyncer.OnSync before individual event handlers.
func (s *mautrixChatService) handleSyncResponse(_ context.Context, resp *mautrix.RespSync, _ string) bool {
	s.unreadCountsMu.Lock()
	if s.unreadCounts == nil {
		s.unreadCounts = make(map[id.RoomID]int, len(resp.Rooms.Join))
	}
	for roomID, joined := range resp.Rooms.Join {
		if joined == nil {
			continue
		}
		if joined.UnreadNotifications != nil {
			s.unreadCounts[roomID] = joined.UnreadNotifications.NotificationCount
		} else {
			s.unreadCounts[roomID] = 0
		}
	}
	for roomID := range resp.Rooms.Leave {
		delete(s.unreadCounts, roomID)
	}
	s.unreadCountsMu.Unlock()

	s.mu.Lock()
	if s.firstSyncCh != nil {
		close(s.firstSyncCh)
		s.firstSyncCh = nil
	}
	s.mu.Unlock()

	return true
}

// eventToMessage converts a mautrix event to our MatrixMessage type.
func (s *mautrixChatService) eventToMessage(evt *event.Event) contracts.MatrixMessage {
	if evt.Content.Parsed == nil {
		_ = evt.Content.ParseRaw(evt.Type)
	}
	content := evt.Content.AsMessage()
	if content == nil {
		return contracts.MatrixMessage{
			EventID:   evt.ID.String(),
			RoomID:    evt.RoomID.String(),
			Sender:    evt.Sender.String(),
			Timestamp: time.UnixMilli(evt.Timestamp),
		}
	}
	msg := contracts.MatrixMessage{
		EventID:   evt.ID.String(),
		RoomID:    evt.RoomID.String(),
		Sender:    evt.Sender.String(),
		Content:   content.Body,
		MsgType:   string(content.MsgType),
		Timestamp: time.UnixMilli(evt.Timestamp),
	}

	if content.RelatesTo != nil && content.RelatesTo.Type == event.RelReplace {
		if content.NewContent != nil {
			msg.Content = content.NewContent.Body
		}
		now := time.Now()
		msg.EditedAt = &now
	}

	if content.RelatesTo != nil && content.RelatesTo.InReplyTo != nil {
		msg.ReplyTo = content.RelatesTo.InReplyTo.EventID.String()
	}

	if content.URL != "" || content.File != nil {
		mediaInfo := &contracts.MatrixMediaInfo{
			MimeType: content.GetInfo().MimeType,
			Size:     int64(content.GetInfo().Size),
			Width:    content.GetInfo().Width,
			Height:   content.GetInfo().Height,
			Filename: content.Body,
		}
		if content.URL != "" {
			mediaInfo.URL = string(content.URL)
		} else if content.File != nil && content.File.URL != "" {
			mediaInfo.URL = string(content.File.URL)
		}
		if content.GetInfo().ThumbnailURL != "" {
			mediaInfo.ThumbnailURL = string(content.GetInfo().ThumbnailURL)
		} else if content.GetInfo().ThumbnailFile != nil && content.GetInfo().ThumbnailFile.URL != "" {
			mediaInfo.ThumbnailURL = string(content.GetInfo().ThumbnailFile.URL)
		}
		msg.Media = mediaInfo
	}

	return msg
}

// ── SetDisplayName / SetAvatar ─────────────────────────────────

func (s *mautrixChatService) SetDisplayName(ctx context.Context, name string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()
	return s.client.SetDisplayName(ctx, name)
}

func (s *mautrixChatService) SetAvatar(ctx context.Context, reader io.Reader, contentType string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read avatar data: %w", err)
	}
	resp, err := s.client.UploadBytes(ctx, data, contentType)
	if err != nil {
		return fmt.Errorf("failed to upload avatar: %w", err)
	}
	return s.client.SetAvatarURL(ctx, resp.ContentURI)
}

// ── DownloadMedia ──────────────────────────────────────────────

func (s *mautrixChatService) DownloadMedia(ctx context.Context, serverName, mediaID string) (io.ReadCloser, string, int64, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, "", 0, err
	}
	s.touchActivity()
	mxcURI := id.ContentURI{
		Homeserver: serverName,
		FileID:     mediaID,
	}
	data, err := s.client.DownloadBytes(ctx, mxcURI)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to download media: %w", err)
	}
	contentType := http.DetectContentType(data)
	reader := io.NopCloser(bytes.NewReader(data))
	return reader, contentType, int64(len(data)), nil
}

// ── Auto-registration ─────────────────────────────────────────

// isForbiddenOrNotFound returns true if the error indicates that the Matrix user
// doesn't exist or has an invalid password (eligible for auto-registration).
func isForbiddenOrNotFound(err error) bool {
	var respErr mautrix.RespError
	if errors.As(err, &respErr) {
		code := respErr.ErrCode
		return code == "M_FORBIDDEN" || code == "M_USER_DEACTIVATED" || code == "M_NOT_FOUND"
	}
	var httpErr mautrix.HTTPError
	if errors.As(err, &httpErr) && httpErr.RespError != nil {
		code := httpErr.RespError.ErrCode
		return code == "M_FORBIDDEN" || code == "M_USER_DEACTIVATED" || code == "M_NOT_FOUND"
	}
	s := err.Error()
	return strings.Contains(s, "M_FORBIDDEN") || strings.Contains(s, "M_USER_DEACTIVATED") || strings.Contains(s, "M_NOT_FOUND")
}

// registerUser registers this service's Matrix user via the Synapse admin shared-secret
// registration API. This is idempotent: if the user already exists, we update the password.
func (s *mautrixChatService) registerUser(ctx context.Context) error {
	homeserverURL := s.config.HomeserverURL
	if homeserverURL == "" {
		homeserverURL = "https://" + s.serverName
	}
	secret := s.config.RegistrationSecret
	localpart := s.matrixUserID.Localpart()
	httpClient := &http.Client{Timeout: 15 * time.Second}

	nonce, err := synapseGetNonce(ctx, httpClient, homeserverURL)
	if err != nil {
		return fmt.Errorf("get registration nonce: %w", err)
	}

	mac := synapseRegistrationMAC(nonce, localpart, s.password, false, secret)

	regBody, _ := json.Marshal(map[string]any{
		"nonce":    nonce,
		"username": localpart,
		"password": s.password,
		"admin":    false,
		"mac":      mac,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", homeserverURL+"/_synapse/admin/v1/register", strings.NewReader(string(regBody)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		log.Infof("Matrix user %s registered successfully", s.matrixUserID)
		return nil
	}

	if strings.Contains(string(body), "M_USER_IN_USE") {
		log.Infof("Matrix user %s already exists, updating password via admin API", s.matrixUserID)
		return s.updatePasswordViaAdmin(ctx, httpClient, homeserverURL, secret)
	}

	return fmt.Errorf("registration failed (HTTP %d): %s", resp.StatusCode, string(body))
}

// updatePasswordViaAdmin updates an existing Synapse user's password using
// the Synapse admin v2 users API. It first obtains an admin access token
// (from /shared/matrix-admin-token or by registering a temporary admin),
// then calls PUT /_synapse/admin/v2/users/{userId} to set the password.
func (s *mautrixChatService) updatePasswordViaAdmin(ctx context.Context, httpClient *http.Client, homeserverURL, secret string) error {
	adminToken, err := s.obtainAdminToken(ctx, httpClient, homeserverURL, secret)
	if err != nil {
		return fmt.Errorf("obtain admin token: %w", err)
	}

	putBody, _ := json.Marshal(map[string]any{"password": s.password})
	escapedUserID := url.PathEscape(s.matrixUserID.String())
	putURL := homeserverURL + "/_synapse/admin/v2/users/" + escapedUserID

	req, err := http.NewRequestWithContext(ctx, "PUT", putURL, bytes.NewReader(putBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Infof("Matrix user %s password updated via admin API", s.matrixUserID)
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("admin password update failed (HTTP %d): %s", resp.StatusCode, string(body))
}

// obtainAdminToken returns a Synapse admin access token. It first tries to
// read /shared/matrix-admin-token (populated by init-synapse.sh in Docker).
// If unavailable, it registers a temporary admin via the shared-secret
// registration endpoint.
func (s *mautrixChatService) obtainAdminToken(ctx context.Context, httpClient *http.Client, homeserverURL, secret string) (string, error) {
	if data, err := os.ReadFile("/shared/matrix-admin-token"); err == nil {
		token := strings.TrimSpace(string(data))
		if token != "" {
			return token, nil
		}
	}

	nonce, err := synapseGetNonce(ctx, httpClient, homeserverURL)
	if err != nil {
		return "", fmt.Errorf("get nonce: %w", err)
	}

	tmpUser := "mbz_admin_tmp"
	tmpPass := fmt.Sprintf("tmp-%d-%s", time.Now().UnixNano(), secret[:8])
	mac := synapseRegistrationMAC(nonce, tmpUser, tmpPass, true, secret)

	regBody, _ := json.Marshal(map[string]any{
		"nonce":    nonce,
		"username": tmpUser,
		"password": tmpPass,
		"admin":    true,
		"mac":      mac,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", homeserverURL+"/_synapse/admin/v1/register", bytes.NewReader(regBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("temp admin register failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse temp admin response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("temp admin register returned empty token")
	}
	return result.AccessToken, nil
}

func synapseGetNonce(ctx context.Context, httpClient *http.Client, homeserverURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", homeserverURL+"/_synapse/admin/v1/register", nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Nonce string `json:"nonce"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Nonce, nil
}

func synapseRegistrationMAC(nonce, username, password string, admin bool, secret string) string {
	adminStr := "notadmin"
	if admin {
		adminStr = "admin"
	}
	msg := nonce + "\x00" + username + "\x00" + password + "\x00" + adminStr
	h := hmac.New(sha1.New, []byte(secret))
	h.Write([]byte(msg))
	return hex.EncodeToString(h.Sum(nil))
}

// --- Chat Settings (Invite Policy) ---

const chatSettingsAccountDataType = "org.mobazha.chat_settings"

// loadChatSettings loads persisted settings from Matrix account data.
// Failures are non-fatal; defaults to auto_mobazha.
func (s *mautrixChatService) loadChatSettings(ctx context.Context) {
	s.chatSettings = contracts.ChatSettings{InvitePolicy: contracts.InvitePolicyAutoMobazha}

	var raw map[string]interface{}
	err := s.client.GetAccountData(ctx, chatSettingsAccountDataType, &raw)
	if err != nil {
		log.Debugf("No persisted chat settings, using defaults: %v", err)
		return
	}

	if policy, ok := raw["invitePolicy"].(string); ok {
		switch contracts.InvitePolicy(policy) {
		case contracts.InvitePolicyAutoAll, contracts.InvitePolicyAutoMobazha, contracts.InvitePolicyAlwaysConfirm:
			s.chatSettings.InvitePolicy = contracts.InvitePolicy(policy)
		}
	}
	log.Infof("Loaded chat settings: invitePolicy=%s", s.chatSettings.InvitePolicy)
}

func (s *mautrixChatService) GetChatSettings(ctx context.Context) (*contracts.ChatSettings, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := s.chatSettings
	return &cp, nil
}

func (s *mautrixChatService) SetChatSettings(ctx context.Context, settings *contracts.ChatSettings) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()

	switch settings.InvitePolicy {
	case contracts.InvitePolicyAutoAll, contracts.InvitePolicyAutoMobazha, contracts.InvitePolicyAlwaysConfirm:
	default:
		return fmt.Errorf("invalid invite policy: %s", settings.InvitePolicy)
	}

	err := s.client.SetAccountData(ctx, chatSettingsAccountDataType, map[string]interface{}{
		"invitePolicy": string(settings.InvitePolicy),
	})
	if err != nil {
		return fmt.Errorf("failed to persist chat settings: %w", err)
	}

	s.mu.Lock()
	s.chatSettings = *settings
	s.mu.Unlock()

	log.Infof("Chat settings updated: invitePolicy=%s", settings.InvitePolicy)
	return nil
}

// handleInvite applies the invite policy when the node user is invited to a room.
func (s *mautrixChatService) handleInvite(ctx context.Context, evt *event.Event) {
	roomID := evt.RoomID.String()
	inviter := evt.Sender.String()

	s.mu.RLock()
	policy := s.chatSettings.InvitePolicy
	s.mu.RUnlock()

	switch policy {
	case contracts.InvitePolicyAutoAll:
		go s.autoJoinInvite(roomID, inviter)
		return

	case contracts.InvitePolicyAutoMobazha:
		if isMobazhaUser(inviter) {
			go s.autoJoinInvite(roomID, inviter)
			return
		}
	}

	s.broadcast(contracts.MatrixChatEvent{
		Type: "chat.invite",
		Data: map[string]string{
			"roomId":  roomID,
			"inviter": inviter,
		},
	})
}

func (s *mautrixChatService) autoJoinInvite(roomID, inviter string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := s.client.JoinRoomByID(ctx, id.RoomID(roomID))
	if err != nil {
		log.Warningf("Auto-join room %s from %s failed: %v", roomID, inviter, err)
		return
	}
	// Matrix forbids setting custom state on behalf of other members, so each
	// user must publish their own canonical peerID after they successfully join.
	s.publishPeerIDState(ctx, id.RoomID(roomID), s.selfPeerIDAssignments())
	log.Infof("Auto-joined room %s (invited by %s)", roomID, inviter)
}

// isMobazhaUser checks if a Matrix user ID follows the Mobazha naming convention (@peer_xxx:server).
func isMobazhaUser(userID string) bool {
	// Strip leading '@' then check localpart prefix
	if len(userID) > 1 && userID[0] == '@' {
		localpart := userID[1:]
		idx := strings.Index(localpart, ":")
		if idx > 0 {
			localpart = localpart[:idx]
		}
		return strings.HasPrefix(localpart, "peer_")
	}
	return false
}

// ===================== SAS Verification =====================

// verificationCallbacks bridges mautrix-go VerificationHelper callbacks to
// WebSocket events so the frontend can drive the interactive SAS flow.
type verificationCallbacks struct {
	svc *mautrixChatService
}

var _ verificationhelper.RequiredCallbacks = (*verificationCallbacks)(nil)
var _ verificationhelper.ShowSASCallbacks = (*verificationCallbacks)(nil)

func (c *verificationCallbacks) VerificationRequested(ctx context.Context, txnID id.VerificationTransactionID, from id.UserID, fromDevice id.DeviceID) {
	// VerificationHelper validates the peer's MAC against the device signing
	// key in the local crypto store. A newly-created SaaS tenant may receive a
	// request before ordinary chat traffic has fetched that user's keys.
	if err := c.svc.prefetchVerificationKeys(ctx, from); err != nil {
		log.Warningf("Failed to prefetch verification keys for %s/%s: %v", from, fromDevice, err)
	}
	c.svc.subsMu.Lock()
	subCount := len(c.svc.subs)
	c.svc.subsMu.Unlock()
	log.Infof("VerificationRequested callback: txnID=%s from=%s device=%s subscribers=%d", txnID, from, fromDevice, subCount)
	c.svc.broadcast(contracts.MatrixChatEvent{
		Type: "chat.verification.request",
		Data: map[string]string{
			"transactionId": string(txnID),
			"userId":        from.String(),
			"deviceId":      fromDevice.String(),
		},
	})
}

func (s *mautrixChatService) prefetchVerificationKeys(ctx context.Context, userID id.UserID) error {
	if userID == "" {
		return fmt.Errorf("verification user ID is empty")
	}
	if s.cryptoHelper == nil || s.cryptoHelper.Machine() == nil {
		return fmt.Errorf("crypto machine is not initialized")
	}
	if _, err := s.cryptoHelper.Machine().FetchKeys(ctx, []id.UserID{userID}, false); err != nil {
		return fmt.Errorf("fetch device keys for %s: %w", userID, err)
	}
	return nil
}

func (c *verificationCallbacks) VerificationReady(_ context.Context, txnID id.VerificationTransactionID, otherDeviceID id.DeviceID, supportsSAS, _ bool, _ *verificationhelper.QRCode) {
	c.svc.broadcast(contracts.MatrixChatEvent{
		Type: "chat.verification.ready",
		Data: map[string]interface{}{
			"transactionId": string(txnID),
			"deviceId":      otherDeviceID.String(),
			"supportsSAS":   supportsSAS,
		},
	})
}

func (c *verificationCallbacks) VerificationCancelled(_ context.Context, txnID id.VerificationTransactionID, code event.VerificationCancelCode, reason string) {
	log.Infof("VerificationCancelled callback: txnID=%s code=%s reason=%s", txnID, code, reason)
	c.svc.broadcast(contracts.MatrixChatEvent{
		Type: "chat.verification.cancelled",
		Data: map[string]string{
			"transactionId": string(txnID),
			"code":          string(code),
			"reason":        reason,
		},
	})
}

func (c *verificationCallbacks) VerificationDone(_ context.Context, txnID id.VerificationTransactionID, method event.VerificationMethod) {
	log.Infof("VerificationDone callback: txnID=%s method=%s", txnID, method)
	c.svc.broadcast(contracts.MatrixChatEvent{
		Type: "chat.verification.done",
		Data: map[string]string{
			"transactionId": string(txnID),
		},
	})
}

func (c *verificationCallbacks) ShowSAS(_ context.Context, txnID id.VerificationTransactionID, emojis []rune, emojiDescriptions []string, decimals []int) {
	log.Infof("ShowSAS callback: txnID=%s emojiCount=%d", txnID, len(emojis))
	emojiList := make([]map[string]interface{}, len(emojis))
	for i, e := range emojis {
		desc := ""
		if i < len(emojiDescriptions) {
			desc = emojiDescriptions[i]
		}
		emojiList[i] = map[string]interface{}{
			"emoji":       string(e),
			"description": desc,
		}
	}
	c.svc.broadcast(contracts.MatrixChatEvent{
		Type: "chat.verification.show_sas",
		Data: map[string]interface{}{
			"transactionId": string(txnID),
			"emojis":        emojiList,
			"decimals":      decimals,
		},
	})
}

func (s *mautrixChatService) StartVerification(ctx context.Context, userID string) (string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return "", err
	}
	if available, reason := s.getVerificationStatus(); !available {
		return "", s.verificationError(reason)
	}
	targetUserID := id.UserID(userID)
	if err := s.prefetchVerificationKeys(ctx, targetUserID); err != nil {
		return "", fmt.Errorf("prepare verification peer: %w", err)
	}
	txnID, err := s.verifyHelper.StartVerification(ctx, targetUserID)
	if err != nil {
		return "", fmt.Errorf("failed to start verification: %w", err)
	}
	s.touchActivity()
	return string(txnID), nil
}

func (s *mautrixChatService) AcceptVerification(ctx context.Context, txnID string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if available, reason := s.getVerificationStatus(); !available {
		return s.verificationError(reason)
	}
	s.touchActivity()
	return s.verifyHelper.AcceptVerification(ctx, id.VerificationTransactionID(txnID))
}

func (s *mautrixChatService) StartSAS(ctx context.Context, txnID string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if available, reason := s.getVerificationStatus(); !available {
		return s.verificationError(reason)
	}
	s.touchActivity()
	return s.verifyHelper.StartSAS(ctx, id.VerificationTransactionID(txnID))
}

func (s *mautrixChatService) ConfirmSAS(ctx context.Context, txnID string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if available, reason := s.getVerificationStatus(); !available {
		return s.verificationError(reason)
	}
	s.touchActivity()
	return s.verifyHelper.ConfirmSAS(ctx, id.VerificationTransactionID(txnID))
}

func (s *mautrixChatService) CancelVerification(ctx context.Context, txnID string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if available, reason := s.getVerificationStatus(); !available {
		return s.verificationError(reason)
	}
	s.touchActivity()
	return s.verifyHelper.CancelVerification(ctx, id.VerificationTransactionID(txnID), event.VerificationCancelCodeUser, "user cancelled")
}

// ---------------------------------------------------------------------------
// Tier 3: Active key request + retry on decryption failure
// ---------------------------------------------------------------------------

const keyRequestRetryTimeout = 10 * time.Second

// handleDecryptError is wired into cryptoHelper.DecryptErrorCallback and is
// invoked by mautrix-go ONLY when an incoming encrypted event fails to
// decrypt. It broadcasts a placeholder "Unable to decrypt" message to the
// client and launches a goroutine to request the missing Megolm session and
// retry decryption (which, on success, will broadcast the real message and
// supersede the placeholder).
func (s *mautrixChatService) handleDecryptError(evt *event.Event, err error) {
	log.Warningf("Undecryptable event: room=%s sender=%s eventID=%s err=%v", evt.RoomID, evt.Sender, evt.ID, err)
	s.broadcast(contracts.MatrixChatEvent{
		Type: "chat.message",
		Data: contracts.MatrixMessage{
			EventID:   evt.ID.String(),
			RoomID:    evt.RoomID.String(),
			Sender:    evt.Sender.String(),
			Content:   "Unable to decrypt this message",
			MsgType:   "m.bad.encrypted",
			Timestamp: time.UnixMilli(evt.Timestamp),
		},
	})
	// syncCtx is written under s.mu by startLocked/resumeFromIdle and cancelled
	// under s.mu by idleStop/stop. Read it under RLock to avoid a data race on
	// the interface value, and skip the retry entirely if the service is idle
	// or shutting down — the retry goroutine would otherwise exit immediately
	// when the cancelled ctx reaches its first context-aware wait.
	s.mu.RLock()
	ctx := s.syncCtx
	s.mu.RUnlock()
	if ctx == nil || ctx.Err() != nil {
		return
	}
	go s.retryDecryptAfterKeyRequest(ctx, evt)
}

// retryDecryptAfterKeyRequest is launched as a goroutine when an EventEncrypted
// event fires (crypto helper failed to decrypt). It parses the encrypted
// payload, requests the missing Megolm session from the sender, waits up to
// keyRequestRetryTimeout for the key to arrive, and if successful, re-decrypts
// the event and broadcasts the corrected message to replace the placeholder.
func (s *mautrixChatService) retryDecryptAfterKeyRequest(parentCtx context.Context, evt *event.Event) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("retryDecryptAfterKeyRequest: PANIC recovered: %v", r)
		}
	}()

	if s.cryptoHelper == nil {
		return
	}
	mach := s.cryptoHelper.Machine()
	if mach == nil {
		return
	}

	enc := evt.Content.AsEncrypted()
	if enc == nil {
		// Content not yet parsed — try parsing raw.
		if err := evt.Content.ParseRaw(evt.Type); err != nil {
			log.Warningf("retryDecrypt: cannot parse encrypted content for %s: %v", evt.ID, err)
			return
		}
		enc = evt.Content.AsEncrypted()
	}
	if enc == nil || enc.SessionID == "" {
		return
	}

	trackKey := string(evt.RoomID) + "/" + string(enc.SessionID)
	roomKeyRequestTrackerLock.Lock()
	if last, ok := roomKeyRequestTracker[trackKey]; ok && time.Since(last) < roomKeyRequestCooldown {
		roomKeyRequestTrackerLock.Unlock()
		return
	}
	roomKeyRequestTracker[trackKey] = time.Now()
	roomKeyRequestTrackerLock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), keyRequestRetryTimeout+5*time.Second)
	defer cancel()

	targets := s.collectRoomKeyRequestTargets(ctx, mach, evt.RoomID)
	if len(targets) == 0 {
		targets = map[id.UserID][]id.DeviceID{evt.Sender: {"*"}}
	}

	if err := mach.SendRoomKeyRequest(ctx, evt.RoomID, enc.SenderKey, enc.SessionID, "", targets); err != nil {
		log.Warningf("retryDecrypt: SendRoomKeyRequest failed for session %s: %v", enc.SessionID, err)
		return
	}
	log.Infof("retryDecrypt: requested key for session %s in room %s from %d users, waiting %v", enc.SessionID, evt.RoomID, len(targets), keyRequestRetryTimeout)

	if !mach.WaitForSession(ctx, evt.RoomID, enc.SenderKey, enc.SessionID, keyRequestRetryTimeout) {
		log.Infof("retryDecrypt: key for session %s did not arrive within timeout", enc.SessionID)
		return
	}

	decrypted, err := s.client.Crypto.Decrypt(ctx, evt)
	if err != nil {
		log.Warningf("retryDecrypt: re-decrypt still failed for %s: %v", evt.ID, err)
		return
	}
	if decrypted.Type != event.EventMessage {
		return
	}

	msg := s.eventToMessage(decrypted)
	log.Infof("retryDecrypt: successfully decrypted %s after key request", evt.ID)
	s.broadcast(contracts.MatrixChatEvent{
		Type: "chat.message.decrypted",
		Data: msg,
	})
}

// collectRoomKeyRequestTargets queries all joined members of a room and
// fetches their device keys, returning a target map suitable for
// SendRoomKeyRequest. This ensures key requests reach ALL room participants
// (not just the message sender), which is critical for recovering self-sent
// message keys after a crypto reset — the other party holds a copy of the
// inbound session for our old outbound Megolm session.
func (s *mautrixChatService) collectRoomKeyRequestTargets(ctx context.Context, mach *mxcrypto.OlmMachine, roomID id.RoomID) map[id.UserID][]id.DeviceID {
	members, err := s.client.JoinedMembers(ctx, roomID)
	if err != nil {
		log.Warningf("collectKeyTargets: JoinedMembers(%s) failed: %v", roomID, err)
		return nil
	}

	userIDs := make([]id.UserID, 0, len(members.Joined))
	for uid := range members.Joined {
		userIDs = append(userIDs, uid)
	}
	if len(userIDs) == 0 {
		return nil
	}

	_, _ = mach.FetchKeys(ctx, userIDs, false)

	targets := make(map[id.UserID][]id.DeviceID, len(userIDs))
	for _, uid := range userIDs {
		devices, _ := mach.CryptoStore.GetDevices(ctx, uid)
		devIDs := make([]id.DeviceID, 0, len(devices))
		for devID := range devices {
			devIDs = append(devIDs, devID)
		}
		if len(devIDs) == 0 {
			devIDs = []id.DeviceID{"*"}
		}
		targets[uid] = devIDs
	}
	return targets
}

// ---------------------------------------------------------------------------
// Tier 2: Megolm session key backup (local encrypted file)
// ---------------------------------------------------------------------------

const keyBackupFilename = "megolm_keys.export"

// keyBackupPath returns the path for the Megolm key backup file. For standalone
// mode it's next to the crypto DB; for shared-PG (SaaS) it returns empty
// (key backup is only meaningful for standalone nodes with local crypto stores).
func (s *mautrixChatService) keyBackupPath() string {
	if s.usesSharedCryptoStore() {
		return ""
	}
	dbPath := s.config.DBPath
	if dbPath == "" {
		dbPath = "mautrix_crypto.db"
	}
	return filepath.Join(filepath.Dir(dbPath), keyBackupFilename)
}

// keyBackupPassphrase derives a stable passphrase for key export/import from
// the pickle key (which itself is derived from the user's private key).
func (s *mautrixChatService) keyBackupPassphrase() string {
	h := sha256.New()
	h.Write(s.pickleKey)
	h.Write([]byte("megolm-key-backup"))
	return hex.EncodeToString(h.Sum(nil))
}

// exportKeyBackup exports all inbound Megolm sessions to an encrypted file.
func (s *mautrixChatService) exportKeyBackup(ctx context.Context) {
	path := s.keyBackupPath()
	if path == "" {
		return
	}
	mach := s.cryptoHelper.Machine()
	if mach == nil {
		return
	}

	sessions := mach.CryptoStore.GetAllGroupSessions(ctx)
	data, err := mxcrypto.ExportKeysIter(s.keyBackupPassphrase(), sessions)
	if err != nil {
		if !errors.Is(err, mxcrypto.ErrNoSessionsForExport) {
			log.Warningf("exportKeyBackup: failed for %s: %v", s.matrixUserID, err)
		}
		return
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		log.Warningf("exportKeyBackup: write failed for %s: %v", path, err)
		return
	}
	log.Infof("exportKeyBackup: exported Megolm keys to %s for %s", path, s.matrixUserID)
}

// importKeyBackup imports Megolm sessions from a previously exported backup.
func (s *mautrixChatService) importKeyBackup(ctx context.Context) {
	path := s.keyBackupPath()
	if path == "" {
		return
	}
	mach := s.cryptoHelper.Machine()
	if mach == nil {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warningf("importKeyBackup: read failed for %s: %v", path, err)
		}
		return
	}

	imported, total, err := mach.ImportKeys(ctx, s.keyBackupPassphrase(), data)
	if err != nil {
		log.Warningf("importKeyBackup: import failed for %s: %v", s.matrixUserID, err)
		return
	}
	log.Infof("importKeyBackup: imported %d/%d Megolm sessions for %s", imported, total, s.matrixUserID)
}

// Ensure mautrixChatService implements contracts.MatrixChatService.
var _ contracts.MatrixChatService = (*mautrixChatService)(nil)
