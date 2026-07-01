package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateTestRSACert() (certPEM string, privKey *rsa.PrivateKey) {
	privKey, _ = rsa.GenerateKey(rand.Reader, 2048)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-casdoor"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &privKey.PublicKey, privKey)
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	return
}

func generateTestECDSACert() (certPEM string, privKey *ecdsa.PrivateKey) {
	privKey, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-casdoor-ec"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &privKey.PublicKey, privKey)
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	return
}

func signToken(claims jwt.Claims, key interface{}) string {
	var method jwt.SigningMethod
	switch key.(type) {
	case *rsa.PrivateKey:
		method = jwt.SigningMethodRS256
	case *ecdsa.PrivateKey:
		method = jwt.SigningMethodES256
	}
	t := jwt.NewWithClaims(method, claims)
	s, _ := t.SignedString(key)
	return s
}

func TestJWTValidator_RSA_ValidToken(t *testing.T) {
	certPEM, privKey := generateTestRSACert()
	localPeer := "12D3KooWTestPeerID"

	v, err := NewJWTValidator(certPEM, localPeer, "")
	if err != nil {
		t.Fatalf("NewJWTValidator: %v", err)
	}

	claims := &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
		Name:       "seller1",
		Properties: map[string]string{"peerID": localPeer},
	}
	tokenStr := signToken(claims, privKey)

	parsed, err := v.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	if parsed.Subject != "user-123" {
		t.Errorf("Subject = %q, want %q", parsed.Subject, "user-123")
	}
	if parsed.PeerID() != localPeer {
		t.Errorf("PeerID() = %q, want %q", parsed.PeerID(), localPeer)
	}
	if !v.IsAdmin(parsed) {
		t.Error("IsAdmin should be true when peerID matches (legacy fallback)")
	}
}

func TestJWTValidator_ECDSA_ValidToken(t *testing.T) {
	certPEM, privKey := generateTestECDSACert()
	localPeer := "12D3KooWECTestPeer"

	v, err := NewJWTValidator(certPEM, localPeer, "")
	if err != nil {
		t.Fatalf("NewJWTValidator: %v", err)
	}

	claims := &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "ec-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
		Properties: map[string]string{"peerID": localPeer},
	}
	tokenStr := signToken(claims, privKey)

	parsed, err := v.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if !v.IsAdmin(parsed) {
		t.Error("IsAdmin should be true for ECDSA signed token (legacy fallback)")
	}
}

func TestJWTValidator_OwnerUserID_Primary(t *testing.T) {
	certPEM, privKey := generateTestRSACert()
	ownerUserID := "casdoor-user-abc123"

	v, err := NewJWTValidator(certPEM, "12D3KooWAnyPeer", ownerUserID)
	if err != nil {
		t.Fatalf("NewJWTValidator: %v", err)
	}

	claims := &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   ownerUserID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
		Id:         ownerUserID,
		Properties: map[string]string{"peerID": "12D3KooWDifferentPeer"},
	}
	tokenStr := signToken(claims, privKey)

	parsed, err := v.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if !v.IsAdmin(parsed) {
		t.Error("IsAdmin should be true when claims.Id matches ownerUserID, even with wrong peerID")
	}
}

func TestJWTValidator_OwnerUserID_WrongUser(t *testing.T) {
	certPEM, privKey := generateTestRSACert()
	ownerUserID := "casdoor-user-abc123"

	v, err := NewJWTValidator(certPEM, "12D3KooWAnyPeer", ownerUserID)
	if err != nil {
		t.Fatalf("NewJWTValidator: %v", err)
	}

	claims := &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "other-user-xyz",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
		Id: "other-user-xyz",
	}
	tokenStr := signToken(claims, privKey)

	parsed, err := v.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if v.IsAdmin(parsed) {
		t.Error("IsAdmin should be false when claims.Id does not match ownerUserID")
	}
}

func TestJWTValidator_UpdateOwnerUserID(t *testing.T) {
	certPEM, privKey := generateTestRSACert()

	v, err := NewJWTValidator(certPEM, "12D3KooWAnyPeer", "")
	if err != nil {
		t.Fatalf("NewJWTValidator: %v", err)
	}

	claims := &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
		Id: "user-123",
	}
	tokenStr := signToken(claims, privKey)

	parsed, err := v.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if v.IsAdmin(parsed) {
		t.Error("IsAdmin should be false before UpdateOwnerUserID (no peerID, no ownerUserID)")
	}

	v.UpdateOwnerUserID("user-123")
	if !v.IsAdmin(parsed) {
		t.Error("IsAdmin should be true after UpdateOwnerUserID matches claims.Id")
	}
}

func TestJWTValidator_ExpiredToken(t *testing.T) {
	certPEM, privKey := generateTestRSACert()

	v, err := NewJWTValidator(certPEM, "some-peer", "")
	if err != nil {
		t.Fatalf("NewJWTValidator: %v", err)
	}

	claims := &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "expired-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}
	tokenStr := signToken(claims, privKey)

	_, err = v.ValidateToken(tokenStr)
	if err == nil {
		t.Fatal("Expected error for expired token, got nil")
	}
}

func TestJWTValidator_WrongSignature(t *testing.T) {
	certPEM, _ := generateTestRSACert()
	_, otherPrivKey := generateTestRSACert()

	v, err := NewJWTValidator(certPEM, "some-peer", "")
	if err != nil {
		t.Fatalf("NewJWTValidator: %v", err)
	}

	claims := &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "wrong-sig",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	tokenStr := signToken(claims, otherPrivKey)

	_, err = v.ValidateToken(tokenStr)
	if err == nil {
		t.Fatal("Expected error for wrong signature, got nil")
	}
}

func TestJWTValidator_IsAdmin_WrongPeerID(t *testing.T) {
	certPEM, privKey := generateTestRSACert()
	localPeer := "12D3KooWMyStore"

	v, err := NewJWTValidator(certPEM, localPeer, "")
	if err != nil {
		t.Fatalf("NewJWTValidator: %v", err)
	}

	claims := &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "other-seller",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
		Properties: map[string]string{"peerID": "12D3KooWOtherStore"},
	}
	tokenStr := signToken(claims, privKey)

	parsed, err := v.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if v.IsAdmin(parsed) {
		t.Error("IsAdmin should be false when peerID does not match")
	}
}

func TestJWTValidator_IsAdmin_NoPeerID(t *testing.T) {
	certPEM, privKey := generateTestRSACert()

	v, err := NewJWTValidator(certPEM, "12D3KooWMyStore", "")
	if err != nil {
		t.Fatalf("NewJWTValidator: %v", err)
	}

	claims := &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "buyer-no-peer",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	tokenStr := signToken(claims, privKey)

	parsed, err := v.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if v.IsAdmin(parsed) {
		t.Error("IsAdmin should be false when no peerID in properties")
	}
}

func TestJWTValidator_InvalidCertPEM(t *testing.T) {
	_, err := NewJWTValidator("not-a-valid-pem", "peer", "")
	if err == nil {
		t.Fatal("Expected error for invalid PEM, got nil")
	}
}

func TestJWTValidator_UpdateCertificate(t *testing.T) {
	certPEM1, privKey1 := generateTestRSACert()
	certPEM2, privKey2 := generateTestRSACert()
	localPeer := "12D3KooWRotate"

	v, err := NewJWTValidator(certPEM1, localPeer, "")
	if err != nil {
		t.Fatalf("NewJWTValidator: %v", err)
	}

	claims := &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "rotate-test",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
		Properties: map[string]string{"peerID": localPeer},
	}

	token1 := signToken(claims, privKey1)
	if _, err := v.ValidateToken(token1); err != nil {
		t.Fatalf("token1 should be valid: %v", err)
	}

	token2 := signToken(claims, privKey2)
	if _, err := v.ValidateToken(token2); err == nil {
		t.Fatal("token2 should be invalid before cert rotation")
	}

	if err := v.UpdateCertificate(certPEM2); err != nil {
		t.Fatalf("UpdateCertificate: %v", err)
	}

	if _, err := v.ValidateToken(token2); err != nil {
		t.Fatalf("token2 should be valid after cert rotation: %v", err)
	}
	if _, err := v.ValidateToken(token1); err == nil {
		t.Fatal("token1 should be invalid after cert rotation")
	}
}
