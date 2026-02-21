package webhook

import (
	"strconv"
	"testing"
	"time"
)

func TestSign_ValidOutput(t *testing.T) {
	sig := SignWebhookPayload("secret123", "wh-1", time.Now().Unix(), []byte(`{"test":"data"}`))
	if sig == "" {
		t.Fatal("expected non-empty signature")
	}
	if len(sig) <= len(SignaturePrefix) {
		t.Fatalf("signature too short: %s", sig)
	}
	if sig[:len(SignaturePrefix)] != SignaturePrefix {
		t.Fatalf("expected prefix %q, got %q", SignaturePrefix, sig[:len(SignaturePrefix)])
	}
}

func TestSign_Deterministic(t *testing.T) {
	ts := time.Now().Unix()
	body := []byte(`{"order":"123"}`)
	s1 := SignWebhookPayload("secret", "wh-1", ts, body)
	s2 := SignWebhookPayload("secret", "wh-1", ts, body)
	if s1 != s2 {
		t.Fatalf("expected identical signatures, got %s and %s", s1, s2)
	}
}

func TestSign_DifferentSecrets(t *testing.T) {
	ts := time.Now().Unix()
	body := []byte(`{"order":"456"}`)
	s1 := SignWebhookPayload("secret-a", "wh-1", ts, body)
	s2 := SignWebhookPayload("secret-b", "wh-1", ts, body)
	if s1 == s2 {
		t.Fatal("expected different signatures for different secrets")
	}
}

func TestVerify_Success(t *testing.T) {
	secret := "test-secret"
	webhookID := "wh-verify"
	ts := time.Now().Unix()
	body := []byte(`{"msg":"hello"}`)

	sig := SignWebhookPayload(secret, webhookID, ts, body)
	if !VerifyWebhookSignature(secret, webhookID, sig, strconv.FormatInt(ts, 10), body) {
		t.Fatal("expected verification to pass")
	}
}

func TestVerify_TamperedBody(t *testing.T) {
	secret := "test-secret"
	webhookID := "wh-tamper"
	ts := time.Now().Unix()

	sig := SignWebhookPayload(secret, webhookID, ts, []byte(`{"original":"data"}`))
	if VerifyWebhookSignature(secret, webhookID, sig, strconv.FormatInt(ts, 10), []byte(`{"tampered":"data"}`)) {
		t.Fatal("expected verification to fail for tampered body")
	}
}

func TestVerify_ReplayAttack(t *testing.T) {
	secret := "test-secret"
	webhookID := "wh-replay"
	oldTs := time.Now().Add(-10 * time.Minute).Unix()
	body := []byte(`{"msg":"old"}`)

	sig := SignWebhookPayload(secret, webhookID, oldTs, body)
	if VerifyWebhookSignature(secret, webhookID, sig, strconv.FormatInt(oldTs, 10), body) {
		t.Fatal("expected verification to fail for replay (old timestamp)")
	}
}

func TestSign_EmptyBody(t *testing.T) {
	sig := SignWebhookPayload("secret", "wh-empty", time.Now().Unix(), []byte{})
	if sig == "" {
		t.Fatal("expected non-empty signature for empty body")
	}
}

func TestSign_SpecialChars(t *testing.T) {
	body := []byte(`{"name":"日本語テスト","emoji":"🎉"}`)
	sig := SignWebhookPayload("secret", "wh-unicode", time.Now().Unix(), body)
	if sig == "" {
		t.Fatal("expected non-empty signature for unicode body")
	}
}

func TestRetryBackoff_Calculation(t *testing.T) {
	tests := []struct {
		attempts int
		expected time.Duration
	}{
		{0, 30 * time.Second},
		{1, 60 * time.Second},
		{2, 120 * time.Second},
		{3, 240 * time.Second},
		{4, 480 * time.Second},
		{10, time.Hour}, // capped at 1 hour
	}
	for _, tt := range tests {
		got := RetryBackoff(tt.attempts)
		if got != tt.expected {
			t.Errorf("RetryBackoff(%d) = %v, want %v", tt.attempts, got, tt.expected)
		}
	}
}
