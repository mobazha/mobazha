package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"time"
)

const (
	SignaturePrefix     = "sha256="
	ReplayWindowSeconds = 300 // 5 minutes
)

// SignWebhookPayload computes the HMAC-SHA256 signature for a webhook delivery.
// The signed message is "{webhookID}.{timestamp}.{body}" to prevent replay attacks.
// Returns the signature in "sha256=<hex>" format.
func SignWebhookPayload(secret, webhookID string, timestamp int64, body []byte) string {
	msg := fmt.Sprintf("%s.%d.%s", webhookID, timestamp, body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return SignaturePrefix + hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhookSignature verifies a webhook signature against the expected secret.
// It checks both the HMAC and the timestamp window to prevent replay attacks.
func VerifyWebhookSignature(secret, webhookID, signature, timestampStr string, body []byte) bool {
	ts, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return false
	}

	diff := time.Now().Unix() - ts
	if math.Abs(float64(diff)) > ReplayWindowSeconds {
		return false
	}

	expected := SignWebhookPayload(secret, webhookID, ts, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// WebhookHeaders returns the standard headers for a webhook delivery.
func WebhookHeaders(secret, webhookID string, body []byte) (signature string, timestamp string) {
	ts := time.Now().Unix()
	sig := SignWebhookPayload(secret, webhookID, ts, body)
	return sig, strconv.FormatInt(ts, 10)
}

// RetryBackoff calculates the next retry delay using exponential backoff.
// Formula: min(2^attempts * 30s, 1h).
func RetryBackoff(attempts int) time.Duration {
	base := 30 * time.Second
	delay := base * time.Duration(1<<uint(attempts))
	maxDelay := time.Hour
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}
