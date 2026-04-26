package apitoken

import (
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	raw, tok, err := Generate("test-token", []string{"listings:read"}, nil)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if raw == "" || tok == nil {
		t.Fatal("expected non-nil results")
	}
	if !IsAPIToken(raw) {
		t.Errorf("raw token %q should start with mbz_ prefix", raw)
	}
	if tok.Name != "test-token" {
		t.Errorf("expected name 'test-token', got %q", tok.Name)
	}
	if len(tok.TokenPrefix) != 8 {
		t.Errorf("expected 8-char prefix, got %d chars: %q", len(tok.TokenPrefix), tok.TokenPrefix)
	}
	if tok.TokenHash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestHashAndVerify(t *testing.T) {
	raw, tok, err := Generate("verify-test", []string{"ai:use"}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !Verify(raw, tok.TokenHash) {
		t.Error("Verify should return true for matching token")
	}
	if Verify("mbz_wrong_token", tok.TokenHash) {
		t.Error("Verify should return false for wrong token")
	}
}

func TestExtractPrefix(t *testing.T) {
	raw, _, err := Generate("prefix-test", nil, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	prefix, err := ExtractPrefix(raw)
	if err != nil {
		t.Fatalf("ExtractPrefix: %v", err)
	}
	if len(prefix) != 8 {
		t.Errorf("expected 8-char prefix, got %d", len(prefix))
	}

	if _, err := ExtractPrefix("short"); err == nil {
		t.Error("expected error for short token")
	}
	if _, err := ExtractPrefix("xyz_12345678_secret"); err == nil {
		t.Error("expected error for wrong prefix")
	}
}

func TestIsActive(t *testing.T) {
	tok := &Token{CreatedAt: time.Now()}
	if !tok.IsActive() {
		t.Error("fresh token should be active")
	}

	now := time.Now()
	tok.RevokedAt = &now
	if tok.IsActive() {
		t.Error("revoked token should not be active")
	}

	past := time.Now().Add(-time.Hour)
	tok.RevokedAt = nil
	tok.ExpiresAt = &past
	if tok.IsActive() {
		t.Error("expired token should not be active")
	}

	future := time.Now().Add(time.Hour)
	tok.ExpiresAt = &future
	if !tok.IsActive() {
		t.Error("token with future expiry should be active")
	}
}

func TestMarshalUnmarshalScopes(t *testing.T) {
	scopes := []string{"listings:read", "orders:manage", "ai:use"}
	data, err := MarshalScopes(scopes)
	if err != nil {
		t.Fatalf("MarshalScopes: %v", err)
	}
	got, err := UnmarshalScopes(data)
	if err != nil {
		t.Fatalf("UnmarshalScopes: %v", err)
	}
	if len(got) != len(scopes) {
		t.Fatalf("expected %d scopes, got %d", len(scopes), len(got))
	}
	for i := range scopes {
		if got[i] != scopes[i] {
			t.Errorf("scope[%d]: expected %q, got %q", i, scopes[i], got[i])
		}
	}
}
