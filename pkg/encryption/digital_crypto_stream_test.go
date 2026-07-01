package encryption

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"strings"
	"testing"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/gagliardetto/solana-go"
)

// fakeKeyProvider implements contracts.KeyProvider with a deterministic
// digital-content master key. Other domains return errors and must not be
// touched by digital-crypto stream tests.
type fakeKeyProvider struct{ master []byte }

func (f *fakeKeyProvider) DigitalContentMasterKey(version int) ([]byte, error) {
	out := make([]byte, len(f.master))
	copy(out, f.master)
	return out, nil
}
func (f *fakeKeyProvider) EVMMasterKey() (*btcec.PrivateKey, error)    { return nil, errNYI }
func (f *fakeKeyProvider) SolanaMasterKey() (*solana.PrivateKey, error) { return nil, errNYI }
func (f *fakeKeyProvider) EscrowMasterKey() (*btcec.PrivateKey, error)  { return nil, errNYI }
func (f *fakeKeyProvider) RatingMasterKey() (*btcec.PrivateKey, error)  { return nil, errNYI }
func (f *fakeKeyProvider) TRONMasterKey() (*btcec.PrivateKey, error)    { return nil, errNYI }

var errNYI = errors.New("not implemented in fakeKeyProvider")

func newTestCrypto(t *testing.T) *DigitalCrypto {
	t.Helper()
	master := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, master); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return NewDigitalCrypto(&fakeKeyProvider{master: master})
}

func TestStreamRoundTrip_VariousSizes(t *testing.T) {
	dc := newTestCrypto(t)
	const chunk = 64 * 1024
	cases := []struct {
		name string
		size int
	}{
		{"empty", 0},
		{"one_byte", 1},
		{"under_chunk", chunk - 1},
		{"exact_chunk", chunk},
		{"chunk_plus_one", chunk + 1},
		{"two_chunks_aligned", chunk * 2},
		{"two_chunks_partial", chunk*2 + 123},
		{"five_chunks_partial", chunk*5 + 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plain := make([]byte, tc.size)
			if _, err := io.ReadFull(rand.Reader, plain); err != nil {
				t.Fatalf("seed plaintext: %v", err)
			}
			assetID := "asset-" + tc.name
			cipher, err := dc.EncryptFileStreamBytes(plain, assetID, 1, chunk)
			if err != nil {
				t.Fatalf("encrypt: %v", err)
			}
			if !bytes.Equal(cipher[:streamMagicLen], streamMagicV1[:]) {
				t.Fatalf("expected stream magic at start, got % x", cipher[:streamMagicLen])
			}
			got, err := dc.DecryptFileStreamBytes(cipher, assetID, 1)
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}
			if !bytes.Equal(got, plain) {
				t.Fatalf("plaintext mismatch (size=%d, got=%d)", len(plain), len(got))
			}
		})
	}
}

func TestStreamReaderRoundTrip(t *testing.T) {
	dc := newTestCrypto(t)
	const size = 5 * 1024 * 1024 // 5 MiB
	plain := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, plain); err != nil {
		t.Fatalf("seed: %v", err)
	}
	assetID := "asset-reader"
	encR, err := dc.EncryptFileStreamReader(bytes.NewReader(plain), assetID, 1, 1024*1024)
	if err != nil {
		t.Fatalf("encrypt reader: %v", err)
	}
	cipher, err := io.ReadAll(encR)
	encR.Close()
	if err != nil {
		t.Fatalf("read cipher: %v", err)
	}

	decR, err := dc.DecryptFileStreamReader(bytes.NewReader(cipher), assetID, 1)
	if err != nil {
		t.Fatalf("decrypt reader: %v", err)
	}
	got, err := io.ReadAll(decR)
	decR.Close()
	if err != nil {
		t.Fatalf("read plain: %v", err)
	}
	if sha256.Sum256(got) != sha256.Sum256(plain) {
		t.Fatalf("hash mismatch")
	}
}

func TestStreamRejectsTampering(t *testing.T) {
	dc := newTestCrypto(t)
	plain := bytes.Repeat([]byte{'A'}, 1024)
	cipher, err := dc.EncryptFileStreamBytes(plain, "asset-tamper", 1, 64*1024)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// Flip a byte in the ciphertext body (after the 20-byte header)
	if len(cipher) <= streamHeaderLen+streamTagLen {
		t.Fatalf("ciphertext shorter than expected: %d", len(cipher))
	}
	cipher[streamHeaderLen] ^= 0xFF
	if _, err := dc.DecryptFileStreamBytes(cipher, "asset-tamper", 1); err == nil {
		t.Fatalf("expected decrypt error after tampering, got nil")
	}
}

func TestStreamRejectsTruncation(t *testing.T) {
	dc := newTestCrypto(t)
	plain := bytes.Repeat([]byte{'B'}, 200_000)
	cipher, err := dc.EncryptFileStreamBytes(plain, "asset-trunc", 1, 64*1024)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// Drop the trailing chunk (final empty-or-partial chunk lives at the end).
	truncated := cipher[:len(cipher)-streamTagLen-100]
	_, err = dc.DecryptFileStreamBytes(truncated, "asset-trunc", 1)
	if err == nil {
		t.Fatalf("expected decrypt error after truncation, got nil")
	}
	if !strings.Contains(err.Error(), "truncated") &&
		!strings.Contains(err.Error(), "decrypt chunk") &&
		!strings.Contains(err.Error(), "ciphertext too short") {
		t.Fatalf("unexpected truncation error message: %v", err)
	}
}

func TestStreamRejectsWrongKeyVersion(t *testing.T) {
	dc := newTestCrypto(t)
	plain := []byte("license content")
	cipher, err := dc.EncryptFileStreamBytes(plain, "asset-key", 1, 64*1024)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// Decrypt with wrong key version → AAD mismatch
	if _, err := dc.DecryptFileStreamBytes(cipher, "asset-key", 2); err == nil {
		t.Fatalf("expected decrypt error with wrong key version")
	}
	// Decrypt with wrong assetID → AAD mismatch
	if _, err := dc.DecryptFileStreamBytes(cipher, "asset-other", 1); err == nil {
		t.Fatalf("expected decrypt error with wrong assetID")
	}
}

func TestStreamRejectsInvalidHeader(t *testing.T) {
	dc := newTestCrypto(t)
	cases := []struct {
		name   string
		cipher []byte
	}{
		{"empty", []byte{}},
		{"too_short", []byte{1, 2, 3}},
		{"wrong_magic", append([]byte{'X', 'X', 'X', 'X', 0x01}, make([]byte, 100)...)},
		{
			name: "tiny_chunk_size",
			cipher: func() []byte {
				b := make([]byte, streamHeaderLen+100)
				copy(b, streamMagicV1[:])
				// chunk_size = 1024 < MinStreamChunkSize
				b[5], b[6], b[7], b[8] = 0, 0, 0x04, 0x00
				return b
			}(),
		},
		{
			name: "huge_chunk_size",
			cipher: func() []byte {
				b := make([]byte, streamHeaderLen+100)
				copy(b, streamMagicV1[:])
				// chunk_size = 0xFFFFFFFF > MaxStreamChunkSize
				b[5], b[6], b[7], b[8] = 0xFF, 0xFF, 0xFF, 0xFF
				return b
			}(),
		},
		{
			name: "nonzero_reserved",
			cipher: func() []byte {
				b := make([]byte, streamHeaderLen+100)
				copy(b, streamMagicV1[:])
				b[5], b[6], b[7], b[8] = 0, 0x10, 0, 0 // 1 MiB chunk
				b[16] = 0x42                            // reserved byte not zero
				return b
			}(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := dc.DecryptFileStreamBytes(tc.cipher, "x", 1); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}
