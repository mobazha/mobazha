package frontend

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSPAHandler_FallbackToIndex(t *testing.T) {
	overrideDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(overrideDir, "index.html"), []byte("<html>SPA</html>"), 0644))

	h := NewHandler(ServerConfig{OverrideDir: overrideDir})
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/some/deep/route")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "SPA")
}

func TestSPAHandler_ServeStaticFile(t *testing.T) {
	overrideDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(overrideDir, "index.html"), []byte("<html>SPA</html>"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(overrideDir, "assets"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(overrideDir, "assets", "app.js"), []byte("console.log('hi')"), 0644))

	h := NewHandler(ServerConfig{OverrideDir: overrideDir})
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/assets/app.js")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "console.log")
}

func TestSPAHandler_BrotliPrecompressed(t *testing.T) {
	overrideDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(overrideDir, "index.html"), []byte("<html>SPA</html>"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(overrideDir, "assets"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(overrideDir, "assets", "app.js"), []byte("uncompressed"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(overrideDir, "assets", "app.js.br"), []byte("brotli-data"), 0644))

	h := NewHandler(ServerConfig{OverrideDir: overrideDir})
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/assets/app.js", nil)
	req.Header.Set("Accept-Encoding", "br, gzip")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "br", resp.Header.Get("Content-Encoding"))
	assert.Equal(t, "application/javascript", resp.Header.Get("Content-Type"))
}

func TestSPAHandler_OverrideTakePrecedence(t *testing.T) {
	overrideDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(overrideDir, "index.html"), []byte("<html>Override</html>"), 0644))

	h := NewHandler(ServerConfig{OverrideDir: overrideDir})
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "Override")
}

func TestSPAHandler_NoOverride_EmptyDist_Returns404(t *testing.T) {
	h := NewHandler(ServerConfig{})
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSPAHandler_RuntimeConfig_StandaloneAuthMode(t *testing.T) {
	h := NewHandler(ServerConfig{SaaSURL: "https://app.mobazha.org"})
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/runtime-config.js")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	js := string(body)

	assert.Equal(t, "application/javascript", resp.Header.Get("Content-Type"))
	assert.Contains(t, js, `authMode:"standalone"`)
	assert.Contains(t, js, `saasUrl:"https://app.mobazha.org"`)
}
