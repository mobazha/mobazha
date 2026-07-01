package frontend

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	payload := parseRuntimeConfig(t, body)

	assert.Equal(t, "application/javascript", resp.Header.Get("Content-Type"))
	assert.Equal(t, float64(RuntimeConfigSchemaVersion), payload["schemaVersion"])
	assert.Equal(t, "standalone", payload["authMode"])
	assert.Equal(t, "https://app.mobazha.org", payload["saasUrl"])
	assert.Equal(t, map[string]any{
		"mode":                   "standalone",
		"allowExternalResources": true,
	}, payload["deployment"])
	assert.Equal(t, map[string]any{"kind": "store"}, payload["experience"])
	assert.Equal(t, map[string]any{}, payload["features"], "nil callback → empty features map, not null")
	assert.Equal(t, map[string]any{
		"commerce": map[string]any{
			"storefront": true,
			"storeAdmin": true,
			"checkout":   true,
		},
		"marketplace": map[string]any{
			"discovery":         false,
			"operator":          false,
			"curation":          false,
			"sellerReview":      false,
			"customDomains":     false,
			"releasePublishing": false,
			"attribution":       false,
		},
		"private_distribution": map[string]any{
			"isolatedRuntime": false,
			"managedFleet":    false,
		},
		"payments": map[string]any{"methods": []any{}},
	}, payload["capabilities"], "nil callback → empty capabilities, not null")
}

func TestSPAHandler_RuntimeConfig_HostedProductComposition(t *testing.T) {
	h := NewRuntimeConfigHandler(ServerConfig{
		Deployment: RuntimeDeployment{Mode: RuntimeDeploymentHosted},
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/runtime-config.js")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	payload := parseRuntimeConfig(t, body)

	assert.Equal(t, "hosted", payload["authMode"])
	assert.Equal(t, map[string]any{
		"mode":                   "hosted",
		"allowExternalResources": true,
	}, payload["deployment"])
	assert.Equal(t, map[string]any{"kind": "platform"}, payload["experience"])
	marketplace := payload["capabilities"].(map[string]any)["marketplace"].(map[string]any)
	assert.Equal(t, true, marketplace["discovery"])
	assert.Equal(t, true, marketplace["operator"])
	assert.Equal(t, true, marketplace["releasePublishing"])
}

func TestSPAHandler_RuntimeConfig_DedicatedMarketplaceExperience(t *testing.T) {
	payload := BuildRuntimeConfigPayload(context.Background(), ServerConfig{
		Deployment: RuntimeDeployment{Mode: RuntimeDeploymentHosted},
		Experience: RuntimeExperience{
			Kind:                  RuntimeExperienceMarketplace,
			MarketplaceIdentifier: "m2-wilson",
		},
	})

	assert.Equal(t, RuntimeExperienceMarketplace, payload.Experience.Kind)
	assert.Equal(t, "m2-wilson", payload.Experience.MarketplaceIdentifier)
}

func TestSPAHandler_RuntimeConfig_CapabilitiesSnapshotInjection(t *testing.T) {
	capabilitiesFn := func(context.Context) RuntimeCapabilities {
		return RuntimeCapabilities{Payments: PaymentCapabilities{Methods: []PaymentCapability{
			{ID: "BTC", Kind: "crypto", Flow: "address-transfer"},
			{ID: "ZEC", Kind: "crypto", Flow: "address-transfer", AddressMode: "transparent"},
			{ID: "stripe", Kind: "fiat", Flow: "provider-session"},
		}}}
	}

	h := NewRuntimeConfigHandler(ServerConfig{CapabilitiesSnapshotFn: capabilitiesFn})
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/runtime-config.js")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	payload := parseRuntimeConfig(t, body)
	capabilities := payload["capabilities"].(map[string]any)
	payments := capabilities["payments"].(map[string]any)
	methods := payments["methods"].([]any)
	require.Len(t, methods, 3)
	assert.Equal(t, "BTC", methods[0].(map[string]any)["id"])
	assert.Equal(t, "transparent", methods[1].(map[string]any)["addressMode"])
	assert.Equal(t, "provider-session", methods[2].(map[string]any)["flow"])
}

func TestSPAHandler_RuntimeConfig_FeaturesSnapshotInjection(t *testing.T) {
	var captured context.Context
	snapshotFn := func(ctx context.Context) []FeatureSnapshot {
		captured = ctx
		return []FeatureSnapshot{
			{Key: "guestCheckout", Effective: true, Overridable: []string{"platform_global", "tenant"}},
			{Key: "fiatPayments", Effective: false, Overridable: nil},
			{Key: "", Effective: true}, // empty key should be dropped
		}
	}

	h := NewHandler(ServerConfig{FeaturesSnapshotFn: snapshotFn})
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/runtime-config.js")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	payload := parseRuntimeConfig(t, body)

	require.NotNil(t, captured, "callback must run and receive a context")

	features, ok := payload["features"].(map[string]any)
	require.True(t, ok, "features must be an object")

	gc, ok := features["guestCheckout"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, gc["effective"])
	assert.Equal(t, []any{"platform_global", "tenant"}, gc["overridable"])

	fp, ok := features["fiatPayments"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, fp["effective"])
	// nil Overridable is normalized to [], never null, to save
	// every frontend caller an unnecessary null check.
	assert.Equal(t, []any{}, fp["overridable"])

	_, hasEmpty := features[""]
	assert.False(t, hasEmpty, "empty-key features must be dropped by the handler")
}

func TestSPAHandler_RuntimeConfig_BrandNetworkSnapshot(t *testing.T) {
	// White-label brand with the "Market Place" preset: surface
	// diagnostics + node pool UI but keep custom node entry off.
	brand := &BrandSnapshot{
		Name: "Example Market",
		Network: &NetworkSnapshot{
			AllowUserCustomNode:     false,
			ShowAdvancedDiagnostics: true,
			ShowNodePoolUI:          true,
			AllowDiscoverToggle:     true,
		},
	}
	h := NewHandler(ServerConfig{
		Deployment: RuntimeDeployment{Mode: RuntimeDeploymentPrivateDistribution},
		Brand:      brand,
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/runtime-config.js")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	payload := parseRuntimeConfig(t, body)
	assert.Equal(t, "private_distribution", payload["deployment"].(map[string]any)["mode"])
	assert.Equal(t, false, payload["deployment"].(map[string]any)["allowExternalResources"])
	assert.Equal(t, true, payload["capabilities"].(map[string]any)["private_distribution"].(map[string]any)["isolatedRuntime"])

	brandObj, ok := payload["brand"].(map[string]any)
	require.True(t, ok, "brand must serialize as a JSON object")
	assert.Equal(t, "Example Market", brandObj["name"])

	netObj, ok := brandObj["network"].(map[string]any)
	require.True(t, ok, "brand.network must be present when any flag is set")
	// AllowUserCustomNode=false → omitempty drops the field.
	_, hasCustom := netObj["allowUserCustomNode"]
	assert.False(t, hasCustom, "false flags must be omitted to keep the payload minimal")
	assert.Equal(t, true, netObj["showAdvancedDiagnostics"])
	assert.Equal(t, true, netObj["showNodePoolUI"])
	assert.Equal(t, true, netObj["allowDiscoverToggle"])
}

func TestSPAHandler_RuntimeConfig_BrandWithoutNetworkSection(t *testing.T) {
	// Branded build with NO network gates opted in: brand.network must
	// be omitted entirely so an attacker can't distinguish "feature
	// gated off" from "feature absent" by reading runtime-config.js.
	brand := &BrandSnapshot{Name: "Example Market"}
	h := NewHandler(ServerConfig{
		Deployment: RuntimeDeployment{Mode: RuntimeDeploymentPrivateDistribution},
		Brand:      brand,
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/runtime-config.js")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	payload := parseRuntimeConfig(t, body)

	brandObj, ok := payload["brand"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Example Market", brandObj["name"])
	_, hasNetwork := brandObj["network"]
	assert.False(t, hasNetwork, "brand.network must not appear when no flags are set")
}

// parseRuntimeConfig strips the `window.__RUNTIME_CONFIG__=` prefix and
// trailing `;` so tests can assert on structured JSON rather than raw bytes.
func parseRuntimeConfig(t *testing.T, body []byte) map[string]any {
	t.Helper()
	raw := strings.TrimSpace(string(body))
	raw = strings.TrimPrefix(raw, "window.__RUNTIME_CONFIG__=")
	raw = strings.TrimSuffix(raw, ";")
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &out), "runtime-config.js must be valid JSON after stripping the window assignment; got %s", raw)
	return out
}
