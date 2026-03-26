package core

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/encryption"
	"github.com/rs/zerolog"
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
	HomeserverURL      string // e.g. "https://matrix.mobazha.org" or internal URL
	ServerName         string // e.g. "matrix.mobazha.org"
	DBPath             string // path for crypto state DB (SQLite) in standalone mode
	RegistrationSecret string // Synapse shared secret for auto-registering Matrix users
	Debug              bool   // enables debug-level logging for mautrix-go client
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

	eventCh   chan contracts.MatrixChatEvent
	subs      []chan contracts.MatrixChatEvent
	subsMu    sync.Mutex

	syncCtx    context.Context
	syncCancel context.CancelFunc

	ready   atomic.Bool
	stopped atomic.Bool

	lastActivity atomic.Int64
	parentCtx    context.Context
	idleCancel   context.CancelFunc

	chatSettings contracts.ChatSettings

	verifyHelper *verificationhelper.VerificationHelper

	mu sync.RWMutex
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
		eventCh:      make(chan contracts.MatrixChatEvent, matrixEventBufSize),
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
	logLevel := zerolog.InfoLevel
	if s.config.Debug {
		logLevel = zerolog.DebugLevel
	}
	client.Log = zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.NoColor = true
		w.TimeFormat = "15:04:05"
	})).With().Str("matrix", s.matrixUserID.Localpart()).Timestamp().Logger().Level(logLevel)
	s.client = client

	startFailed := true
	defer func() {
		if startFailed {
			s.client = nil
		}
	}()

	_, err = s.client.Login(ctx, &mautrix.ReqLogin{
		Type: mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{
			Type: mautrix.IdentifierTypeUser,
			User: s.matrixUserID.String(),
		},
		Password:         s.password,
		StoreCredentials: true,
	})
	if err != nil {
		if s.config.RegistrationSecret != "" && isForbiddenOrNotFound(err) {
			log.Infof("Matrix user %s does not exist, auto-registering...", s.matrixUserID)
			if regErr := s.registerUser(ctx); regErr != nil {
				return fmt.Errorf("matrix auto-register failed: %w (original login error: %v)", regErr, err)
			}
			_, err = s.client.Login(ctx, &mautrix.ReqLogin{
				Type: mautrix.AuthTypePassword,
				Identifier: mautrix.UserIdentifier{
					Type: mautrix.IdentifierTypeUser,
					User: s.matrixUserID.String(),
				},
				Password:         s.password,
				StoreCredentials: true,
			})
		}
		if err != nil {
			return fmt.Errorf("matrix login failed: %w", err)
		}
	}

	dbDSN := s.config.DBPath
	if dbDSN == "" {
		dbDSN = "mautrix_crypto.db"
	}

	cryptoHelper, err := cryptohelper.NewCryptoHelper(s.client, s.pickleKey, dbDSN)
	if err != nil {
		return fmt.Errorf("failed to create crypto helper: %w", err)
	}
	s.cryptoHelper = cryptoHelper

	if err := s.cryptoHelper.Init(ctx); err != nil {
		if strings.Contains(err.Error(), "mismatching device ID") {
			log.Warningf("Crypto store device ID mismatch, resetting crypto DB: %v", err)
			if resetErr := s.resetCryptoDB(ctx, dbDSN); resetErr != nil {
				return fmt.Errorf("failed to reset crypto DB after device mismatch: %w (original: %v)", resetErr, err)
			}
		} else {
			return fmt.Errorf("failed to init crypto helper: %w", err)
		}
	}
	s.client.Crypto = s.cryptoHelper

	mach := s.cryptoHelper.Machine()
	mach.ShareKeysMinTrust = id.TrustStateCrossSignedTOFU
	mach.AllowKeyShare = func(_ context.Context, device *id.Device, _ event.RequestedKeyInfo) *mxcrypto.KeyShareRejection {
		if device.UserID == s.client.UserID {
			return nil
		}
		if device.Trust == id.TrustStateCrossSignedVerified || device.Trust == id.TrustStateCrossSignedTOFU {
			return nil
		}
		return &mxcrypto.KeyShareRejectNoResponse
	}
	log.Infof("Matrix crypto initialized: user=%s device=%s dbDSN=%s ShareKeysMinTrust=CrossSignedTOFU", s.client.UserID, s.client.DeviceID, dbDSN)

	vh := verificationhelper.NewVerificationHelper(
		s.client, mach, nil, &verificationCallbacks{svc: s},
		false, false, true,
	)
	if err := vh.Init(ctx); err != nil {
		log.Warningf("Failed to init verification helper: %v (verification features unavailable)", err)
	} else {
		s.verifyHelper = vh
		s.client.Verification = vh
	}

	s.registerEventHandlers()
	s.loadChatSettings(ctx)

	s.parentCtx = context.Background()
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
		s.syncCtx, s.syncCancel = context.WithCancel(s.parentCtx)
		go s.syncLoop()
		s.ready.Store(true)
		s.touchActivity()
		log.Infof("Matrix chat service resumed from idle for %s", s.matrixUserID)
		return nil
	}

	return s.startLocked(ctx)
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

	log.Infof("Matrix chat service idle-paused for %s", s.matrixUserID)
	s.broadcast(contracts.MatrixChatEvent{
		Type: "chat.disconnected",
		Data: map[string]string{"reason": "idle_timeout"},
	})
}

// resetCryptoDB removes the crypto store DB and reinitializes the crypto helper.
// This is necessary when the device ID changes (e.g., after container restart)
// and the old crypto store has a mismatching device ID.
func (s *mautrixChatService) resetCryptoDB(ctx context.Context, dbDSN string) error {
	dbPath := dbDSN
	if strings.HasPrefix(dbPath, "file:") {
		dbPath = strings.TrimPrefix(dbPath, "file:")
	}
	if idx := strings.Index(dbPath, "?"); idx >= 0 {
		dbPath = dbPath[:idx]
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(dbPath + suffix)
	}

	cryptoHelper, err := cryptohelper.NewCryptoHelper(s.client, s.pickleKey, dbDSN)
	if err != nil {
		return fmt.Errorf("failed to recreate crypto helper: %w", err)
	}
	s.cryptoHelper = cryptoHelper
	if err := s.cryptoHelper.Init(ctx); err != nil {
		return fmt.Errorf("failed to init fresh crypto helper: %w", err)
	}
	log.Infof("Crypto DB reset successful, new device keys established")
	return nil
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
		if err := s.cryptoHelper.Close(); err != nil {
			log.Errorf("Failed to close crypto helper: %v", err)
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

// IsReady returns true when the client is logged in and syncing.
func (s *mautrixChatService) IsReady() bool {
	return s.ready.Load()
}

// GetStatus returns the current connection status.
func (s *mautrixChatService) GetStatus() contracts.MatrixChatStatus {
	if !s.ready.Load() || s.client == nil {
		return contracts.MatrixChatStatus{Connected: false}
	}
	return contracts.MatrixChatStatus{
		Connected:   true,
		UserID:      s.matrixUserID.String(),
		DeviceID:    s.client.DeviceID.String(),
		ServerName:  s.serverName,
		SyncRunning: s.ready.Load(),
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
			break
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

	syncer.OnEventType(event.EventMessage, func(ctx context.Context, evt *event.Event) {
		msg := s.eventToMessage(evt)
		content := evt.Content.AsMessage()
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
		log.Infof("StateMember: room=%s sender=%s stateKey=%s membership=%s", evt.RoomID, evt.Sender, evt.GetStateKey(), content.Membership)
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

// eventToMessage converts a mautrix event to our MatrixMessage type.
func (s *mautrixChatService) eventToMessage(evt *event.Event) contracts.MatrixMessage {
	if evt.Content.Parsed == nil {
		_ = evt.Content.ParseRaw(evt.Type)
	}
	content := evt.Content.AsMessage()
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
		log.Infof("Matrix user %s already exists, updating password", s.matrixUserID)
		return s.updatePasswordViaRegister(ctx, httpClient, homeserverURL, secret)
	}

	return fmt.Errorf("registration failed (HTTP %d): %s", resp.StatusCode, string(body))
}

// updatePasswordViaRegister updates an existing Synapse user's password by re-registering
// with a fresh nonce. If the admin v1/register endpoint returns M_USER_IN_USE for a second
// time, we accept it (password was likely correct already).
func (s *mautrixChatService) updatePasswordViaRegister(ctx context.Context, httpClient *http.Client, homeserverURL, secret string) error {
	nonce, err := synapseGetNonce(ctx, httpClient, homeserverURL)
	if err != nil {
		return err
	}
	localpart := s.matrixUserID.Localpart()
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

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "M_USER_IN_USE") {
		return nil
	}
	return fmt.Errorf("password update failed (HTTP %d): %s", resp.StatusCode, string(body))
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

func (c *verificationCallbacks) VerificationRequested(_ context.Context, txnID id.VerificationTransactionID, from id.UserID, fromDevice id.DeviceID) {
	c.svc.broadcast(contracts.MatrixChatEvent{
		Type: "chat.verification.request",
		Data: map[string]string{
			"transactionId": string(txnID),
			"userId":        from.String(),
			"deviceId":      fromDevice.String(),
		},
	})
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
	c.svc.broadcast(contracts.MatrixChatEvent{
		Type: "chat.verification.cancelled",
		Data: map[string]string{
			"transactionId": string(txnID),
			"code":          string(code),
			"reason":        reason,
		},
	})
}

func (c *verificationCallbacks) VerificationDone(_ context.Context, txnID id.VerificationTransactionID, _ event.VerificationMethod) {
	c.svc.broadcast(contracts.MatrixChatEvent{
		Type: "chat.verification.done",
		Data: map[string]string{
			"transactionId": string(txnID),
		},
	})
}

func (c *verificationCallbacks) ShowSAS(_ context.Context, txnID id.VerificationTransactionID, emojis []rune, emojiDescriptions []string, decimals []int) {
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
	if s.verifyHelper == nil {
		return "", fmt.Errorf("verification not available")
	}
	txnID, err := s.verifyHelper.StartVerification(ctx, id.UserID(userID))
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
	if s.verifyHelper == nil {
		return fmt.Errorf("verification not available")
	}
	s.touchActivity()
	return s.verifyHelper.AcceptVerification(ctx, id.VerificationTransactionID(txnID))
}

func (s *mautrixChatService) StartSAS(ctx context.Context, txnID string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.verifyHelper == nil {
		return fmt.Errorf("verification not available")
	}
	s.touchActivity()
	return s.verifyHelper.StartSAS(ctx, id.VerificationTransactionID(txnID))
}

func (s *mautrixChatService) ConfirmSAS(ctx context.Context, txnID string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.verifyHelper == nil {
		return fmt.Errorf("verification not available")
	}
	s.touchActivity()
	return s.verifyHelper.ConfirmSAS(ctx, id.VerificationTransactionID(txnID))
}

func (s *mautrixChatService) CancelVerification(ctx context.Context, txnID string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.verifyHelper == nil {
		return fmt.Errorf("verification not available")
	}
	s.touchActivity()
	return s.verifyHelper.CancelVerification(ctx, id.VerificationTransactionID(txnID), event.VerificationCancelCodeUser, "user cancelled")
}

// Ensure mautrixChatService implements contracts.MatrixChatService.
var _ contracts.MatrixChatService = (*mautrixChatService)(nil)
