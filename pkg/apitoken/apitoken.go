package apitoken

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	Prefix     = "mbz_"
	PrefixLen  = 4
	SecretLen  = 32
	MaxPerUser = 20
)

var ErrTokenNotFound = errors.New("api token not found")

// Token stores a scoped API token. The raw token is returned to the user
// exactly once at creation; only the SHA-256 hash is persisted.
//
// Token format: mbz_<8-char hex prefix>_<64-char hex secret>
type Token struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"-"`
	TokenPrefix string   `json:"prefix"`
	Scopes     []string   `json:"scopes,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// IsActive returns true if the token is not revoked and not expired.
func (t *Token) IsActive() bool {
	if t.RevokedAt != nil {
		return false
	}
	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return false
	}
	return true
}

// MaskedDisplay returns "mbz_<prefix>_****" for safe display.
func (t *Token) MaskedDisplay() string {
	return Prefix + t.TokenPrefix + "_****"
}

// Generate creates a new token with cryptographically random bytes.
// Returns the raw token (show to user once) and the Token record.
func Generate(name string, scopes []string, expiresAt *time.Time) (rawToken string, record *Token, err error) {
	prefixBytes := make([]byte, PrefixLen)
	if _, err = rand.Read(prefixBytes); err != nil {
		return "", nil, fmt.Errorf("generate prefix: %w", err)
	}
	prefix := hex.EncodeToString(prefixBytes)

	secretBytes := make([]byte, SecretLen)
	if _, err = rand.Read(secretBytes); err != nil {
		return "", nil, fmt.Errorf("generate secret: %w", err)
	}
	secret := hex.EncodeToString(secretBytes)

	rawToken = Prefix + prefix + "_" + secret
	hash := HashRaw(rawToken)

	record = &Token{
		Name:        name,
		TokenHash:   hash,
		TokenPrefix: prefix,
		Scopes:      scopes,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now(),
	}
	return rawToken, record, nil
}

// HashRaw returns the SHA-256 hex digest of a raw token string.
func HashRaw(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(h[:])
}

// ExtractPrefix extracts the 8-char hex prefix from a raw "mbz_<prefix>_<secret>" token.
func ExtractPrefix(rawToken string) (string, error) {
	if len(rawToken) < 14 || rawToken[:4] != Prefix {
		return "", errors.New("invalid token format")
	}
	return rawToken[4:12], nil
}

// IsAPIToken checks whether a string looks like an API token (starts with "mbz_").
func IsAPIToken(s string) bool {
	return len(s) > 4 && s[:4] == Prefix
}

// Verify checks whether a raw token matches a stored hash using constant-time comparison.
func Verify(rawToken, storedHash string) bool {
	h := sha256.Sum256([]byte(rawToken))
	computed := hex.EncodeToString(h[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(storedHash)) == 1
}

// MarshalScopes serializes a string slice into JSON for storage.
func MarshalScopes(scopes []string) (string, error) {
	data, err := json.Marshal(scopes)
	if err != nil {
		return "", fmt.Errorf("marshal scopes: %w", err)
	}
	return string(data), nil
}

// UnmarshalScopes parses a JSON string into a string slice.
func UnmarshalScopes(s string) ([]string, error) {
	var scopes []string
	if err := json.Unmarshal([]byte(s), &scopes); err != nil {
		return nil, fmt.Errorf("parse scopes: %w", err)
	}
	return scopes, nil
}
