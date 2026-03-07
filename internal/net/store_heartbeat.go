package net

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultHeartbeatInterval = 5 * time.Minute
	heartbeatTimeout         = 10 * time.Second
)

// StoreHeartbeatConfig holds configuration for the heartbeat sender.
type StoreHeartbeatConfig struct {
	SaaSURL     string // SaaS platform base URL (e.g. https://api.mobazha.com)
	PeerID      string // this node's peer ID
	EndpointURL string // this node's public API endpoint (empty for NAT-only stores)
	APIKey      string // API key obtained during registration
	Version     string // node version string

	OwnerUserIDFn func() string // returns the Casdoor owner user ID (may be nil)

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
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		log.Debugf("heartbeat sent successfully to %s", s.cfg.SaaSURL)
	} else {
		log.Warningf("heartbeat response: %d", resp.StatusCode)
	}
}

// RegisterWithSaaS performs the initial store registration with the SaaS platform.
// On success, it returns the API key to be used for subsequent heartbeats.
func RegisterWithSaaS(ctx context.Context, saasURL, peerID, endpointURL, connectivity string) (string, error) {
	body := map[string]string{
		"peer_id": peerID,
	}
	if endpointURL != "" {
		body["endpoint_url"] = endpointURL
	}
	if connectivity != "" {
		body["connectivity"] = connectivity
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
