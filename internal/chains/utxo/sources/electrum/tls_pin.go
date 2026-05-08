package electrum

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"strings"
)

// TLSConfigWithPin creates a tls.Config that verifies the server's leaf
// certificate matches the provided SHA256 fingerprint. If fingerprint is empty,
// standard system CA verification is used.
func TLSConfigWithPin(fingerprint string) *tls.Config {
	if fingerprint == "" {
		return &tls.Config{MinVersion: tls.VersionTLS12}
	}

	expected := strings.ToLower(strings.ReplaceAll(fingerprint, ":", ""))

	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return fmt.Errorf("no certificate presented by server")
			}
			actual := sha256.Sum256(rawCerts[0])
			actualHex := hex.EncodeToString(actual[:])
			if actualHex != expected {
				return fmt.Errorf("TLS certificate fingerprint mismatch: got %s, want %s", actualHex, expected)
			}
			return nil
		},
	}
}
