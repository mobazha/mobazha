package net

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	defaultHeartbeatInterval = 5 * time.Minute
	heartbeatTimeout         = 10 * time.Second
	storeRegistrationPrefix  = "mobazha-store-registration-v1"
)

type storeRegistrationRequest struct {
	PeerID       string `json:"peer_id"`
	EndpointURL  string `json:"endpoint_url,omitempty"`
	Domain       string `json:"domain,omitempty"`
	Connectivity string `json:"connectivity,omitempty"`
	Timestamp    int64  `json:"timestamp"`
	Nonce        string `json:"nonce"`
	Signature    string `json:"signature"`
}

// StoreHeartbeatConfig holds configuration for the heartbeat sender.
type StoreHeartbeatConfig struct {
	SaaSURL     string // SaaS platform base URL (e.g. https://app.mobazha.org)
	PeerID      string // this node's peer ID
	EndpointURL string // this node's public API endpoint (empty for NAT-only stores)
	APIKey      string // API key obtained during registration
	Version     string // node version string

	OwnerUserIDFn func() string // returns the Casdoor owner user ID (may be nil)
	// OnUnauthorized replaces a rejected credential through a fresh signed
	// registration and returns the new API key.
	OnUnauthorized func(context.Context) (string, error)

	Interval time.Duration // heartbeat interval (defaults to 5 minutes)
}

// StoreHeartbeatSender periodically sends heartbeat pings to the SaaS platform
// so the cross-store proxy knows this standalone store is online and reachable.
type StoreHeartbeatSender struct {
	cfg    StoreHeartbeatConfig
	client *http.Client
}

// NewStoreHeartbeatSender creates a heartbeat sender for a standalone store.
func NewStoreHeartbeatSender(cfg StoreHeartbeatConfig) *StoreHeartbeatSender {
	if cfg.Interval == 0 {
		cfg.Interval = defaultHeartbeatInterval
	}
	return &StoreHeartbeatSender{
		cfg: cfg,
		client: &http.Client{
			Timeout: heartbeatTimeout,
		},
	}
}

// Start begins sending heartbeats in a background goroutine.
// The goroutine exits when ctx is cancelled.
func (s *StoreHeartbeatSender) Start(ctx context.Context) {
	go func() {
		s.sendHeartbeat(ctx)

		ticker := time.NewTicker(s.cfg.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.sendHeartbeat(ctx)
			case <-ctx.Done():
				log.Info("Store heartbeat sender stopped")
				return
			}
		}
	}()
	log.Infof("Store heartbeat sender started (interval=%s, saas=%s)", s.cfg.Interval, s.cfg.SaaSURL)
}

func (s *StoreHeartbeatSender) sendHeartbeat(ctx context.Context) {
	s.sendHeartbeatWithRecovery(ctx, true)
}

func (s *StoreHeartbeatSender) sendHeartbeatWithRecovery(ctx context.Context, allowRecovery bool) {
	payload := map[string]string{
		"peer_id": s.cfg.PeerID,
	}
	if s.cfg.EndpointURL != "" {
		payload["endpoint_url"] = s.cfg.EndpointURL
	}
	if s.cfg.Version != "" {
		payload["version"] = s.cfg.Version
	}
	if s.cfg.OwnerUserIDFn != nil {
		if ownerID := s.cfg.OwnerUserIDFn(); ownerID != "" {
			payload["owner_user_id"] = ownerID
		}
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		log.Errorf("heartbeat marshal: %v", err)
		return
	}

	url := fmt.Sprintf("%s/platform/v1/stores/heartbeat", s.cfg.SaaSURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		log.Errorf("heartbeat request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Standalone-Store-Key", s.cfg.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		log.Warningf("heartbeat failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		log.Debugf("heartbeat sent successfully to %s", s.cfg.SaaSURL)
	} else if resp.StatusCode == http.StatusUnauthorized && allowRecovery && s.cfg.OnUnauthorized != nil {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		newAPIKey, recoveryErr := s.cfg.OnUnauthorized(ctx)
		if recoveryErr != nil {
			log.Warningf("heartbeat credential recovery failed after %d: %v, body=%s", resp.StatusCode, recoveryErr, string(respBody))
			return
		}
		if newAPIKey == "" {
			log.Warning("heartbeat credential recovery returned an empty API key")
			return
		}
		s.cfg.APIKey = newAPIKey
		log.Info("Store API key rotated after heartbeat rejection")
		s.sendHeartbeatWithRecovery(ctx, false)
	} else {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		log.Warningf("heartbeat response: %d, body=%s", resp.StatusCode, string(respBody))
	}
}

// RegisterWithSaaS performs proof-of-possession registration with the SaaS
// platform. The registration request is signed by the store's libp2p identity
// key so an arbitrary caller cannot reserve another store's Peer ID.
// On success, it returns the API key to be used for subsequent heartbeats.
func RegisterWithSaaS(
	ctx context.Context,
	saasURL, peerID, endpointURL, domain, connectivity string,
	privateKey libp2pcrypto.PrivKey,
) (string, error) {
	body, err := newStoreRegistrationRequest(peerID, endpointURL, domain, connectivity, privateKey, time.Now())
	if err != nil {
		return "", err
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("%s/platform/v1/stores/register", saasURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("register: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("register failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data struct {
			APIKey string `json:"api_key"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	if result.Data.APIKey == "" {
		return "", fmt.Errorf("empty api_key in response")
	}

	return result.Data.APIKey, nil
}

func newStoreRegistrationRequest(
	peerID, endpointURL, domain, connectivity string,
	privateKey libp2pcrypto.PrivKey,
	now time.Time,
) (*storeRegistrationRequest, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("store registration requires a private key")
	}
	derivedPeerID, err := peer.IDFromPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("derive peer ID: %w", err)
	}
	if derivedPeerID.String() != peerID {
		return nil, fmt.Errorf("private key does not match peer ID")
	}

	nonceBytes := make([]byte, 24)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, fmt.Errorf("generate registration nonce: %w", err)
	}
	req := &storeRegistrationRequest{
		PeerID:       peerID,
		EndpointURL:  endpointURL,
		Domain:       domain,
		Connectivity: connectivity,
		Timestamp:    now.Unix(),
		Nonce:        base64.RawURLEncoding.EncodeToString(nonceBytes),
	}
	payload := storeRegistrationSignaturePayload(req)
	signature, err := privateKey.Sign([]byte(payload))
	if err != nil {
		return nil, fmt.Errorf("sign store registration: %w", err)
	}
	req.Signature = base64.StdEncoding.EncodeToString(signature)
	return req, nil
}

func storeRegistrationSignaturePayload(req *storeRegistrationRequest) string {
	encode := base64.RawURLEncoding.EncodeToString
	return fmt.Sprintf(
		"%s:%s:%s:%s:%s:%d:%s",
		storeRegistrationPrefix,
		encode([]byte(req.PeerID)),
		encode([]byte(req.EndpointURL)),
		encode([]byte(req.Domain)),
		encode([]byte(req.Connectivity)),
		req.Timestamp,
		req.Nonce,
	)
}
