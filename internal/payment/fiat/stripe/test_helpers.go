package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

// SignWebhookForTest signs a payload using the webhook secret, producing a
// Stripe-Signature header value. Exported for cross-package integration tests.
func SignWebhookForTest(payload []byte, secret string) string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%s,v1=%s", ts, sig)
}
