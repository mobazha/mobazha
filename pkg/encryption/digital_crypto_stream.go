package encryption

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// On-disk file format v1 — chunked AES-256-GCM streaming AEAD.
//
// Designed so that 512 MiB digital assets can be encrypted, stored, and
// decrypted with a constant ~4 MiB working set on the server, replacing the
// legacy "load whole file into RAM, AES-GCM seal once" path.
//
// Layout:
//
//	+----------------------------------------------------------+
//	| Header (20 bytes)                                        |
//	|   magic:        5 bytes  = "MBZE\x01"                    |
//	|   chunk_size:   4 bytes  = uint32 BE (plaintext bytes)   |
//	|   nonce_prefix: 7 bytes  (random per file)               |
//	|   reserved:     4 bytes  (must be zero)                  |
//	+----------------------------------------------------------+
//	| Chunk[0]: ciphertext_plaintext_size + 16-byte GCM tag    |
//	| Chunk[1]: ...                                            |
//	|   ...                                                    |
//	| Chunk[N-1] (last): ciphertext (≤ chunk_size) + tag       |
//	+----------------------------------------------------------+
//
// Per-chunk 12-byte nonce fed to AES-GCM:
//
//	[nonce_prefix (7)] [chunk_index uint32 BE (4)] [last_byte (1)]
//
// `last_byte` is 0x00 for non-final chunks and 0x01 for the final chunk —
// this is the standard "STREAM" AEAD construction (Hoang & Tessaro 2015,
// also used by Tink and age) that prevents truncation attacks.
//
// Per-chunk AAD (binds the ciphertext to its key context):
//
//	"MBZE-DA-v1\x00" || assetID (UTF-8) || uint32 BE keyVersion
//
// This is the *only* on-disk format for digital file assets — the v0
// whole-file AES-GCM path was removed before launch (see TD-110 in
// docs/TECH_DEBT.md). DownloadFile / ServeDownload assume v1 unconditionally.
const (
	streamMagicLen   = 5
	streamHeaderLen  = 20
	streamNonceLen   = 12
	streamPrefixLen  = 7
	streamTagLen     = 16
	streamReservedLen = 4

	// DefaultStreamChunkSize is the plaintext chunk size used for new
	// uploads. 4 MiB balances:
	//   - tag overhead (16 / 4 MiB ≈ 0.0004%)
	//   - resume granularity for partial-failure recovery
	//   - peak server memory (~ chunk_size per active upload)
	DefaultStreamChunkSize = 4 * 1024 * 1024

	// MinStreamChunkSize / MaxStreamChunkSize bound the chunk_size header
	// field on read so a malformed/malicious header cannot cause the
	// decryptor to allocate gigabytes per chunk.
	MinStreamChunkSize = 64 * 1024
	MaxStreamChunkSize = 64 * 1024 * 1024
)

var streamMagicV1 = [streamMagicLen]byte{'M', 'B', 'Z', 'E', 0x01}

// streamAAD returns the per-chunk associated data that binds a chunk to its
// asset and key version. Identical for every chunk in a given file but
// different across files — prevents cross-file chunk replay even if the
// nonce_prefix collides.
func streamAAD(assetID string, keyVersion int) []byte {
	const tag = "MBZE-DA-v1\x00"
	out := make([]byte, 0, len(tag)+len(assetID)+4)
	out = append(out, []byte(tag)...)
	out = append(out, []byte(assetID)...)
	out = binary.BigEndian.AppendUint32(out, uint32(keyVersion))
	return out
}

func streamNonce(prefix [streamPrefixLen]byte, chunkIdx uint32, last bool) [streamNonceLen]byte {
	var n [streamNonceLen]byte
	copy(n[:streamPrefixLen], prefix[:])
	binary.BigEndian.PutUint32(n[streamPrefixLen:streamPrefixLen+4], chunkIdx)
	if last {
		n[streamNonceLen-1] = 0x01
	}
	return n
}

// EncryptFileStream encrypts plaintext from src to dst using the v1 chunked
// AEAD container. The returned int64 is the number of ciphertext bytes
// written (header + per-chunk overhead included).
//
// Memory footprint is bounded by chunkSize + tag + small constants — the
// caller's plaintext source is never fully buffered.
//
// chunkSize must satisfy MinStreamChunkSize ≤ chunkSize ≤ MaxStreamChunkSize;
// pass 0 to use DefaultStreamChunkSize.
func EncryptFileStream(dst io.Writer, src io.Reader, key []byte, assetID string, keyVersion int, chunkSize int) (int64, error) {
	if chunkSize == 0 {
		chunkSize = DefaultStreamChunkSize
	}
	if chunkSize < MinStreamChunkSize || chunkSize > MaxStreamChunkSize {
		return 0, fmt.Errorf("chunk size %d out of range [%d,%d]", chunkSize, MinStreamChunkSize, MaxStreamChunkSize)
	}
	if len(key) != 32 {
		return 0, fmt.Errorf("invalid key length: expected 32, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, fmt.Errorf("aes.NewCipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return 0, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	var prefix [streamPrefixLen]byte
	if _, err := io.ReadFull(rand.Reader, prefix[:]); err != nil {
		return 0, fmt.Errorf("nonce prefix generation: %w", err)
	}

	header := make([]byte, streamHeaderLen)
	copy(header[:streamMagicLen], streamMagicV1[:])
	binary.BigEndian.PutUint32(header[streamMagicLen:streamMagicLen+4], uint32(chunkSize))
	copy(header[streamMagicLen+4:streamMagicLen+4+streamPrefixLen], prefix[:])
	// reserved bytes already zero

	var written int64
	if n, werr := dst.Write(header); werr != nil {
		return int64(n), fmt.Errorf("write header: %w", werr)
	} else {
		written += int64(n)
	}

	aad := streamAAD(assetID, keyVersion)
	plain := make([]byte, chunkSize)
	cipherOut := make([]byte, 0, chunkSize+streamTagLen)

	var chunkIdx uint32
	for {
		// Read up to chunkSize. io.ReadFull short-reads at EOF; the chunk it
		// returns is the last one (possibly empty for boundary-aligned files).
		nRead, rerr := io.ReadFull(src, plain)
		if rerr != nil && !errors.Is(rerr, io.EOF) && !errors.Is(rerr, io.ErrUnexpectedEOF) {
			return written, fmt.Errorf("read plaintext chunk %d: %w", chunkIdx, rerr)
		}

		isLast := errors.Is(rerr, io.EOF) || errors.Is(rerr, io.ErrUnexpectedEOF)
		nonce := streamNonce(prefix, chunkIdx, isLast)
		cipherOut = aead.Seal(cipherOut[:0], nonce[:], plain[:nRead], aad)

		if n, werr := dst.Write(cipherOut); werr != nil {
			return written, fmt.Errorf("write chunk %d: %w", chunkIdx, werr)
		} else {
			written += int64(n)
		}

		chunkIdx++
		if isLast {
			return written, nil
		}
		if chunkIdx == 0 {
			// chunk index wrapped around — file too large for u32.
			return written, errors.New("stream encrypt: too many chunks (chunk index overflow)")
		}
	}
}

// streamHeader is the parsed v1 header.
type streamHeader struct {
	chunkSize int
	prefix    [streamPrefixLen]byte
}

func readStreamHeader(src io.Reader) (*streamHeader, error) {
	buf := make([]byte, streamHeaderLen)
	if _, err := io.ReadFull(src, buf); err != nil {
		return nil, fmt.Errorf("read stream header: %w", err)
	}
	if !bytes.Equal(buf[:streamMagicLen], streamMagicV1[:]) {
		return nil, errors.New("invalid stream magic — not a v1 container")
	}
	chunkSize := int(binary.BigEndian.Uint32(buf[streamMagicLen : streamMagicLen+4]))
	if chunkSize < MinStreamChunkSize || chunkSize > MaxStreamChunkSize {
		return nil, fmt.Errorf("invalid chunk size in header: %d", chunkSize)
	}
	var prefix [streamPrefixLen]byte
	copy(prefix[:], buf[streamMagicLen+4:streamMagicLen+4+streamPrefixLen])
	// reserved bytes ignored on read for forward compatibility — but require zero so
	// future format extensions can repurpose them safely.
	for _, b := range buf[streamMagicLen+4+streamPrefixLen:] {
		if b != 0 {
			return nil, errors.New("stream header reserved bytes must be zero")
		}
	}
	return &streamHeader{chunkSize: chunkSize, prefix: prefix}, nil
}

// DecryptFileStream decrypts a v1 chunked AEAD container from src and writes
// plaintext to dst. Returns the number of plaintext bytes written.
//
// Validates per-chunk authentication tags and the final-chunk flag, so
// truncated, reordered, or modified ciphertexts are rejected before any
// plaintext bytes leak to dst… except for any plaintext that has already been
// flushed for previously-verified chunks. The caller is responsible for
// surfacing partial-write semantics to its consumer.
func DecryptFileStream(dst io.Writer, src io.Reader, key []byte, assetID string, keyVersion int) (int64, error) {
	if len(key) != 32 {
		return 0, fmt.Errorf("invalid key length: expected 32, got %d", len(key))
	}

	hdr, err := readStreamHeader(src)
	if err != nil {
		return 0, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, fmt.Errorf("aes.NewCipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return 0, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	aad := streamAAD(assetID, keyVersion)
	cipherBuf := make([]byte, hdr.chunkSize+streamTagLen)
	plainOut := make([]byte, 0, hdr.chunkSize)

	var written int64
	var chunkIdx uint32
	for {
		nRead, rerr := io.ReadFull(src, cipherBuf)
		// io.ReadFull returns:
		//   - len(buf)              + nil  if full read
		//   - 0                     + EOF  at end-of-stream
		//   - 0 < n < len(buf)      + ErrUnexpectedEOF on partial read
		// Both partial and full reads are valid for the last chunk;
		// only "0 + EOF" right at the start of a chunk is anomalous (no
		// final-flag chunk seen yet).
		if errors.Is(rerr, io.EOF) {
			return written, errors.New("stream decrypt: truncated (no final chunk)")
		}
		if rerr != nil && !errors.Is(rerr, io.ErrUnexpectedEOF) {
			return written, fmt.Errorf("read chunk %d: %w", chunkIdx, rerr)
		}
		if nRead < streamTagLen {
			return written, fmt.Errorf("read chunk %d: ciphertext too short (%d bytes)", chunkIdx, nRead)
		}

		// io.ReadFull semantics:
		//   nil err               + nRead == len(cipherBuf)   → full chunk (non-final by encryptor convention)
		//   io.ErrUnexpectedEOF   + nRead <  len(cipherBuf)   → partial chunk (always the final chunk)
		// The encryptor *always* emits a trailing empty-plaintext chunk for
		// boundary-aligned files, so the decryptor sees a full final chunk
		// only via the partial-read path (16-byte tag at minimum).
		isLast := errors.Is(rerr, io.ErrUnexpectedEOF)

		nonce := streamNonce(hdr.prefix, chunkIdx, isLast)
		plainOut, err = aead.Open(plainOut[:0], nonce[:], cipherBuf[:nRead], aad)
		if err != nil {
			return written, fmt.Errorf("decrypt chunk %d: %w", chunkIdx, err)
		}

		if n, werr := dst.Write(plainOut); werr != nil {
			return written, fmt.Errorf("write chunk %d: %w", chunkIdx, werr)
		} else {
			written += int64(n)
		}

		chunkIdx++
		if isLast {
			return written, nil
		}
		if chunkIdx == 0 {
			return written, errors.New("stream decrypt: chunk index overflow")
		}
	}
}

// EncryptFileStreamReader returns an io.Reader that yields the v1 chunked
// AEAD container produced from src. Useful for piping into BlobStore writers
// that accept io.Reader (S3, R2, local FS).
//
// Internally drives EncryptFileStream via io.Pipe + goroutine.
func (dc *DigitalCrypto) EncryptFileStreamReader(src io.Reader, assetID string, keyVersion int, chunkSize int) (io.ReadCloser, error) {
	key, err := dc.DeriveAssetKey(assetID, keyVersion)
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	go func() {
		defer ZeroBytes(key)
		_, err := EncryptFileStream(pw, src, key, assetID, keyVersion, chunkSize)
		pw.CloseWithError(err)
	}()
	return pr, nil
}

// DecryptFileStreamReader returns an io.Reader yielding plaintext for a v1
// chunked AEAD container read from src.
func (dc *DigitalCrypto) DecryptFileStreamReader(src io.Reader, assetID string, keyVersion int) (io.ReadCloser, error) {
	key, err := dc.DeriveAssetKey(assetID, keyVersion)
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	go func() {
		defer ZeroBytes(key)
		_, err := DecryptFileStream(pw, src, key, assetID, keyVersion)
		pw.CloseWithError(err)
	}()
	return pr, nil
}

// EncryptFileStreamWith is a convenience wrapper for tests / synchronous
// callers that already have all plaintext in memory.
func (dc *DigitalCrypto) EncryptFileStreamBytes(plaintext []byte, assetID string, keyVersion int, chunkSize int) ([]byte, error) {
	key, err := dc.DeriveAssetKey(assetID, keyVersion)
	if err != nil {
		return nil, err
	}
	defer ZeroBytes(key)
	var buf bytes.Buffer
	if _, err := EncryptFileStream(&buf, bytes.NewReader(plaintext), key, assetID, keyVersion, chunkSize); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecryptFileStreamBytes is the symmetric in-memory helper.
func (dc *DigitalCrypto) DecryptFileStreamBytes(ciphertext []byte, assetID string, keyVersion int) ([]byte, error) {
	key, err := dc.DeriveAssetKey(assetID, keyVersion)
	if err != nil {
		return nil, err
	}
	defer ZeroBytes(key)
	var buf bytes.Buffer
	if _, err := DecryptFileStream(&buf, bytes.NewReader(ciphertext), key, assetID, keyVersion); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
