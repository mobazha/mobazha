package electrum

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTLSConfigWithPin_EmptyFingerprint_UsesSystemCA(t *testing.T) {
	cfg := TLSConfigWithPin("")
	assert.False(t, cfg.InsecureSkipVerify)
	assert.Nil(t, cfg.VerifyPeerCertificate)
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
}

func TestTLSConfigWithPin_ValidFingerprint_AcceptsMatch(t *testing.T) {
	cert, certDER := selfSignedCert(t)
	fingerprint := sha256.Sum256(certDER)
	fpHex := hex.EncodeToString(fingerprint[:])

	cfg := TLSConfigWithPin(fpHex)
	require.NotNil(t, cfg.VerifyPeerCertificate)

	err := cfg.VerifyPeerCertificate([][]byte{certDER}, nil)
	assert.NoError(t, err)
	_ = cert
}

func TestTLSConfigWithPin_WrongFingerprint_Rejects(t *testing.T) {
	_, certDER := selfSignedCert(t)
	wrongFP := "0000000000000000000000000000000000000000000000000000000000000000"

	cfg := TLSConfigWithPin(wrongFP)
	err := cfg.VerifyPeerCertificate([][]byte{certDER}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fingerprint mismatch")
}

func TestTLSConfigWithPin_ColonSeparatedFingerprint(t *testing.T) {
	_, certDER := selfSignedCert(t)
	fingerprint := sha256.Sum256(certDER)
	fpHex := hex.EncodeToString(fingerprint[:])
	// Insert colons every 2 chars
	var colonFP string
	for i, ch := range fpHex {
		if i > 0 && i%2 == 0 {
			colonFP += ":"
		}
		colonFP += string(ch)
	}

	cfg := TLSConfigWithPin(colonFP)
	err := cfg.VerifyPeerCertificate([][]byte{certDER}, nil)
	assert.NoError(t, err)
}

func selfSignedCert(t *testing.T) (tls.Certificate, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	tlsCert := tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  key,
	}
	return tlsCert, derBytes
}
