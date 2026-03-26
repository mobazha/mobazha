package encryption

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestDeriveMatrixPickleKey(t *testing.T) {
	key := []byte("test-private-key-bytes-32-chars!!")
	derived := DeriveMatrixPickleKey(key)

	assert.Len(t, derived, 32)

	// Deterministic
	assert.Equal(t, derived, DeriveMatrixPickleKey(key))

	// Different input → different output
	other := DeriveMatrixPickleKey([]byte("other-key"))
	assert.NotEqual(t, derived, other)

	// Different from password derivation (domain separation)
	pwd, _ := DeriveMatrixPassword(key)
	assert.NotEqual(t, string(derived), pwd)
}

func TestMatrixUserIDFromPeerID(t *testing.T) {
	id := MatrixUserIDFromPeerID("QmABC", "matrix.mobazha.org")
	assert.Equal(t, "@peer_qmabc:matrix.mobazha.org", id)
}
