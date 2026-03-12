package paypal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrWebhookIDNotConfigured  = errors.New("paypal webhook ID not configured")
	ErrMissingWebhookHeaders   = errors.New("missing required PayPal webhook headers")
	ErrInvalidWebhookSignature = errors.New("invalid PayPal webhook signature")
)

type webhookVerifyRequest struct {
	AuthAlgo         string          `json:"auth_algo"`
	CertURL          string          `json:"cert_url"`
	TransmissionID   string          `json:"transmission_id"`
	TransmissionSig  string          `json:"transmission_sig"`
	TransmissionTime string          `json:"transmission_time"`
	WebhookID        string          `json:"webhook_id"`
	WebhookEvent     json.RawMessage `json:"webhook_event"`
}

type webhookVerifyResponse struct {
	VerificationStatus string `json:"verification_status"`
}

type signatureCache struct {
	mu    sync.RWMutex
	items map[string]time.Time
}

const signatureCacheTTL = 5 * time.Minute
const signatureCacheMaxSize = 100

func newSignatureCache() *signatureCache {
	return &signatureCache{
		items: make(map[string]time.Time),
	}
}

func (c *signatureCache) isVerified(transmissionID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if t, ok := c.items[transmissionID]; ok {
		return time.Since(t) < signatureCacheTTL
	}
	return false
}

func (c *signatureCache) markVerified(transmissionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[transmissionID] = time.Now()

	if len(c.items) > signatureCacheMaxSize {
		now := time.Now()
		for k, t := range c.items {
			if now.Sub(t) > signatureCacheTTL {
				delete(c.items, k)
			}
		}
	}
}

// verifyWebhookViaAPI calls PayPal's POST /v1/notifications/verify-webhook-signature
// endpoint to cryptographically verify a webhook event is authentic.
func (p *Provider) verifyWebhookViaAPI(ctx context.Context, payload []byte, headers map[string]string) error {
	webhookID := p.config.WebhookID
	if webhookID == "" {
		return ErrWebhookIDNotConfigured
	}

	transmissionID := getHeader(headers, "Paypal-Transmission-Id")
	transmissionSig := getHeader(headers, "Paypal-Transmission-Sig")
	transmissionTime := getHeader(headers, "Paypal-Transmission-Time")
	authAlgo := getHeader(headers, "Paypal-Auth-Algo")
	certURL := getHeader(headers, "Paypal-Cert-Url")

	if transmissionID == "" || transmissionSig == "" {
		return ErrMissingWebhookHeaders
	}

	if p.sigCache.isVerified(transmissionID) {
		return nil
	}

	verifyReq := webhookVerifyRequest{
		AuthAlgo:         authAlgo,
		CertURL:          certURL,
		TransmissionID:   transmissionID,
		TransmissionSig:  transmissionSig,
		TransmissionTime: transmissionTime,
		WebhookID:        webhookID,
		WebhookEvent:     json.RawMessage(payload),
	}

	var verifyResp webhookVerifyResponse
	if err := p.client.doJSON(ctx, "POST", "/v1/notifications/verify-webhook-signature", verifyReq, &verifyResp); err != nil {
		return fmt.Errorf("paypal verify webhook API: %w", err)
	}

	if verifyResp.VerificationStatus != "SUCCESS" {
		return ErrInvalidWebhookSignature
	}

	p.sigCache.markVerified(transmissionID)

	return nil
}
