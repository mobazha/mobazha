package mcp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

const (
	quoteTokenVersion = 1
	quoteTokenTTL     = 10 * time.Minute
)

// QuotePayload holds the immutable checkout parameters captured at prepare time.
// The confirm step re-reads the listing and verifies nothing material changed.
type QuotePayload struct {
	Version       int    `json:"v"`
	StorePeerID   string `json:"store"`
	Slug          string `json:"slug"`
	ListingHash   string `json:"hash"`
	Quantity      int    `json:"qty"`
	CoinType      string `json:"coin"`
	MaxTotalSats  string `json:"maxTotal"`
	ShippingOption string `json:"shipping,omitempty"`
	ExpiresAt     int64  `json:"exp"`
	Nonce         string `json:"nonce"`
}

// QuoteTokenSigner creates and verifies HMAC-signed quote tokens.
type QuoteTokenSigner struct {
	secret []byte
}

// NewQuoteTokenSigner creates a signer with the given HMAC secret.
// If secret is nil, a random 32-byte key is generated (suitable for single-process use).
func NewQuoteTokenSigner(secret []byte) *QuoteTokenSigner {
	if secret == nil {
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			panic("failed to generate HMAC secret: " + err.Error())
		}
	}
	return &QuoteTokenSigner{secret: secret}
}

// Sign creates a quoteToken from the given payload. It sets Version, ExpiresAt,
// and Nonce automatically if not already set.
func (s *QuoteTokenSigner) Sign(p *QuotePayload) (string, error) {
	if p.StorePeerID == "" || p.Slug == "" || p.Quantity <= 0 || p.CoinType == "" {
		return "", fmt.Errorf("incomplete quote payload: store, slug, quantity, and coin are required")
	}
	p.Version = quoteTokenVersion
	if p.ExpiresAt == 0 {
		p.ExpiresAt = time.Now().Add(quoteTokenTTL).Unix()
	}
	if p.Nonce == "" {
		nonce := make([]byte, 16)
		if _, err := rand.Read(nonce); err != nil {
			return "", fmt.Errorf("generate nonce: %w", err)
		}
		p.Nonce = base64.RawURLEncoding.EncodeToString(nonce)
	}

	payloadBytes, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal quote payload: %w", err)
	}

	mac := hmac.New(sha256.New, s.secret)
	mac.Write(payloadBytes)
	sig := mac.Sum(nil)

	token := base64.RawURLEncoding.EncodeToString(payloadBytes) + "." +
		base64.RawURLEncoding.EncodeToString(sig)
	return token, nil
}

// Verify validates the token signature and expiry, returning the payload.
func (s *QuoteTokenSigner) Verify(token string) (*QuotePayload, error) {
	parts := splitToken(token)
	if len(parts) != 2 {
		return nil, fmt.Errorf("malformed quote token")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}

	mac := hmac.New(sha256.New, s.secret)
	mac.Write(payloadBytes)
	expected := mac.Sum(nil)
	if !hmac.Equal(sigBytes, expected) {
		return nil, fmt.Errorf("invalid quote token signature")
	}

	var p QuotePayload
	if err := json.Unmarshal(payloadBytes, &p); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	if time.Now().Unix() > p.ExpiresAt {
		return nil, fmt.Errorf("quote token expired")
	}

	return &p, nil
}

func splitToken(token string) []string {
	for i := len(token) - 1; i >= 0; i-- {
		if token[i] == '.' {
			return []string{token[:i], token[i+1:]}
		}
	}
	return []string{token}
}
