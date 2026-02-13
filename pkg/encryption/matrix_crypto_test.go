package encryption

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveMatrixEncryptionKey(t *testing.T) {
	key := []byte("test-private-key-bytes-32-chars!!")
	derived := DeriveMatrixEncryptionKey(key)

	assert.Len(t, derived, 32) // SHA-256 produces 32 bytes

	// Deterministic
	assert.Equal(t, derived, DeriveMatrixEncryptionKey(key))

	// Different input → different output
	other := DeriveMatrixEncryptionKey([]byte("other-key"))
	assert.NotEqual(t, derived, other)
}

func TestDeriveMatrixPassword(t *testing.T) {
	key := []byte("test-private-key-bytes-32-chars!!")
	pwd, err := DeriveMatrixPassword(key)
	require.NoError(t, err)
	assert.NotEmpty(t, pwd)

	// Deterministic
	pwd2, _ := DeriveMatrixPassword(key)
	assert.Equal(t, pwd, pwd2)

	// Different input → different password
	other, _ := DeriveMatrixPassword([]byte("other-key"))
	assert.NotEqual(t, pwd, other)
}

func TestAESGCM_RoundTrip(t *testing.T) {
	key := DeriveMatrixEncryptionKey([]byte("my-key"))
	plaintext := []byte("hello, matrix E2EE keys!")

	ciphertext, err := EncryptAESGCM(key, plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := DecryptAESGCM(key, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAESGCM_WrongKey(t *testing.T) {
	key1 := DeriveMatrixEncryptionKey([]byte("key1"))
	key2 := DeriveMatrixEncryptionKey([]byte("key2"))

	ct, err := EncryptAESGCM(key1, []byte("secret"))
	require.NoError(t, err)

	_, err = DecryptAESGCM(key2, ct)
	assert.Error(t, err)
}

func TestDecryptAESGCM_TooShort(t *testing.T) {
	key := DeriveMatrixEncryptionKey([]byte("key"))
	_, err := DecryptAESGCM(key, []byte("short"))
	assert.Error(t, err)
}

func TestCountKeysInJSON(t *testing.T) {
	assert.Equal(t, 0, CountKeysInJSON(`{}`))
	assert.Equal(t, 1, CountKeysInJSON(`{"room_id":"!abc"}`))
	assert.Equal(t, 2, CountKeysInJSON(`[{"room_id":"!a"},{"room_id":"!b"}]`))
}

func TestMatrixUserIDFromPeerID(t *testing.T) {
	id := MatrixUserIDFromPeerID("QmABC", "matrix.mobazha.org")
	assert.Equal(t, "@peer_qmabc:matrix.mobazha.org", id)
}
