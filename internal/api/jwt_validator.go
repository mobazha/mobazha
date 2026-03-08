package api

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"sync"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims represents the claims extracted from a SaaS Casdoor JWT.
// Only the fields needed for standalone admin authorization are included.
type JWTClaims struct {
	jwt.RegisteredClaims
	Owner      string            `json:"owner,omitempty"`
	Name       string            `json:"name,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

// PeerID returns the peerID from Properties, if set during OAuth binding.
func (c *JWTClaims) PeerID() string {
	if c.Properties == nil {
		return ""
	}
	return c.Properties["peerID"]
}

// JWTValidator validates JWT tokens issued by SaaS Casdoor using the
// Casdoor certificate. Thread-safe for concurrent use.
type JWTValidator struct {
	mu         sync.RWMutex
	publicKey  interface{} // *rsa.PublicKey or *ecdsa.PublicKey
	certPEM    string
	localPeer  string // this node's peerID, for admin authorization
}

// NewJWTValidator creates a validator from a PEM-encoded certificate.
// localPeerID is this standalone node's peer ID, used to check that
// the JWT holder is authorized to manage this specific store.
func NewJWTValidator(certPEM, localPeerID string) (*JWTValidator, error) {
	v := &JWTValidator{
		certPEM:   certPEM,
		localPeer: localPeerID,
	}
	if err := v.loadCertificate(certPEM); err != nil {
		return nil, fmt.Errorf("load casdoor certificate: %w", err)
	}
	return v, nil
}

func (v *JWTValidator) loadCertificate(certPEM string) error {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return errors.New("failed to decode PEM block")
	}

	switch block.Type {
	case "CERTIFICATE":
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("parse certificate: %w", err)
		}
		v.publicKey = cert.PublicKey
	case "PUBLIC KEY":
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("parse public key: %w", err)
		}
		v.publicKey = pub
	default:
		return fmt.Errorf("unsupported PEM type: %s", block.Type)
	}

	return nil
}

// ValidateToken verifies the JWT signature and returns the claims.
// It does NOT check admin authorization — call IsAdmin separately.
func (v *JWTValidator) ValidateToken(tokenString string) (*JWTClaims, error) {
	v.mu.RLock()
	pubKey := v.publicKey
	v.mu.RUnlock()

	if pubKey == nil {
		return nil, errors.New("no public key configured")
	}

	claims := &JWTClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		switch token.Method.(type) {
		case *jwt.SigningMethodRSA:
			if k, ok := pubKey.(*rsa.PublicKey); ok {
				return k, nil
			}
			return nil, fmt.Errorf("expected RSA public key, got %T", pubKey)
		case *jwt.SigningMethodECDSA:
			if k, ok := pubKey.(*ecdsa.PublicKey); ok {
				return k, nil
			}
			return nil, fmt.Errorf("expected ECDSA public key, got %T", pubKey)
		default:
			return nil, fmt.Errorf("unsupported signing method: %v", token.Header["alg"])
		}
	})
	if err != nil {
		return nil, fmt.Errorf("validate jwt: %w", err)
	}

	return claims, nil
}

// IsAdmin checks whether the JWT claims grant admin access to this store.
// The peerID in the JWT Properties must match this node's peerID.
func (v *JWTValidator) IsAdmin(claims *JWTClaims) bool {
	if claims == nil {
		return false
	}
	jwtPeer := claims.PeerID()
	return jwtPeer != "" && jwtPeer == v.localPeer
}

// UpdateCertificate hot-reloads the verification certificate without restart.
func (v *JWTValidator) UpdateCertificate(certPEM string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.loadCertificate(certPEM)
}
