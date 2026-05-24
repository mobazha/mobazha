//go:build !private_distribution

package core

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestPrivKey(t *testing.T) crypto.PrivKey {
	t.Helper()
	privKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	return privKey
}

func TestLoadSaveMatrixProvisionState(t *testing.T) {
	dir := t.TempDir()

	_, err := loadMatrixProvisionState(dir)
	assert.Error(t, err, "should fail when file does not exist")

	state := &matrixProvisionState{
		HomeserverURL: "https://matrix.example.org",
		ServerName:    "matrix.example.org",
		Provisioned:   true,
	}
	err = saveMatrixProvisionState(dir, state)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, matrixProvisionFile))
	require.NoError(t, err)
	assert.Contains(t, string(data), "matrix.example.org")

	loaded, err := loadMatrixProvisionState(dir)
	require.NoError(t, err)
	assert.Equal(t, state.HomeserverURL, loaded.HomeserverURL)
	assert.Equal(t, state.ServerName, loaded.ServerName)
	assert.True(t, loaded.Provisioned)
}

func TestRequestMatrixProvision_Success(t *testing.T) {
	saasServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/platform/v1/stores/QmTestPeer/matrix/provision")
		assert.Equal(t, "test-api-key", r.Header.Get("X-Standalone-Store-Key"))

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.NotEmpty(t, body["password_hash"])

		resp := map[string]any{
			"data": map[string]any{
				"homeserver_url": "https://matrix.test.org",
				"server_name":    "matrix.test.org",
				"provisioned":    true,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer saasServer.Close()

	privKey := generateTestPrivKey(t)

	result, err := requestMatrixProvision(saasServer.URL, "test-api-key", "QmTestPeer", privKey)
	require.NoError(t, err)
	assert.Equal(t, "https://matrix.test.org", result.HomeserverURL)
	assert.Equal(t, "matrix.test.org", result.ServerName)
}

func TestRequestMatrixProvision_APIError(t *testing.T) {
	saasServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"message": "internal error",
			},
		})
	}))
	defer saasServer.Close()

	privKey := generateTestPrivKey(t)

	_, err := requestMatrixProvision(saasServer.URL, "test-api-key", "QmTestPeer", privKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestRequestMatrixProvision_NotProvisioned(t *testing.T) {
	saasServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": map[string]any{
				"homeserver_url": "https://matrix.test.org",
				"server_name":    "matrix.test.org",
				"provisioned":    false,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer saasServer.Close()

	privKey := generateTestPrivKey(t)

	_, err := requestMatrixProvision(saasServer.URL, "test-api-key", "QmTestPeer", privKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provisioned=false")
}
