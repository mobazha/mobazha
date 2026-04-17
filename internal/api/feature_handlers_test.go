// Unit tests for Feature flag HTTP handlers (Phase FF-impl-api §4.1).
//
// We avoid the full MobazhaNode and instead compose:
//
//   - A real pkg/config.Resolver with controlled Providers/Store.
//   - A minimal stub that embeds contracts.NodeService and additionally
//     implements FeaturesProvider + FeatureAdminProvider.
//
// The feature `guestCheckout` (registered by pkg/config/features_defined.go)
// is tenant-scoped, so it's the natural fixture for these tests.

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

// memTenantStore is an in-memory implementation of pkgconfig.TenantFeatureStore
// suitable for unit tests. It records the last Set call so tests can assert
// actor/value propagation.
type memTenantStore struct {
	values   map[string]bool // keyed by "tenantID|key"
	lastSet  struct {
		tenant string
		key    string
		value  bool
		actor  string
	}
	setErr error
}

func newMemTenantStore() *memTenantStore {
	return &memTenantStore{values: map[string]bool{}}
}

func (m *memTenantStore) Get(_ context.Context, tenantID, key string) (bool, bool, error) {
	v, ok := m.values[tenantID+"|"+key]
	return v, ok, nil
}

func (m *memTenantStore) Set(_ context.Context, tenantID, key string, value bool, actor string) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.values[tenantID+"|"+key] = value
	m.lastSet.tenant = tenantID
	m.lastSet.key = key
	m.lastSet.value = value
	m.lastSet.actor = actor
	return nil
}

func (m *memTenantStore) List(_ context.Context, tenantID string) (map[string]bool, error) {
	out := map[string]bool{}
	prefix := tenantID + "|"
	for k, v := range m.values {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			out[k[len(prefix):]] = v
		}
	}
	return out, nil
}

// togglePlatformProvider returns a fixed enabled value for every key. Used to
// simulate a platform_global layer that disables a feature (triggering 409 in
// handlePUTFeatureSetting).
type togglePlatformProvider struct{ enabled bool }

func (p togglePlatformProvider) IsEnabled(_ context.Context, _ string) (bool, error) {
	return p.enabled, nil
}

// ---------------------------------------------------------------------------
// Stub NodeService that satisfies FeaturesProvider + FeatureAdminProvider.
// ---------------------------------------------------------------------------

type featureTestNode struct {
	contracts.NodeService // embedded interface; other methods will panic if called
	resolver              pkgconfig.ResolverInterface
	store                 pkgconfig.TenantFeatureStore
}

func (n *featureTestNode) Features() pkgconfig.ResolverInterface   { return n.resolver }
func (n *featureTestNode) TenantFeatureStore() pkgconfig.TenantFeatureStore { return n.store }

// nilAdminNode implements FeaturesProvider but deliberately returns nil for
// TenantFeatureStore(), modelling the SaaS proxy shim case.
type nilAdminNode struct {
	contracts.NodeService
	resolver pkgconfig.ResolverInterface
}

func (n *nilAdminNode) Features() pkgconfig.ResolverInterface              { return n.resolver }
func (n *nilAdminNode) TenantFeatureStore() pkgconfig.TenantFeatureStore   { return nil }

// ---------------------------------------------------------------------------
// Test server helpers
// ---------------------------------------------------------------------------

// featureTestServer wires a Gateway-less router so we can avoid the auth
// middleware (which depends on g.config). We mount the handlers directly on
// mux.Router paths — the handlers themselves fetch contracts.NodeService via
// getNodeService(r), which reads from the request context populated below.
func featureTestServer(t *testing.T, node contracts.NodeService) *httptest.Server {
	t.Helper()

	g := &Gateway{}

	r := mux.NewRouter()
	r.HandleFunc("/v1/features", g.handleGETFeatures).Methods("GET")
	r.HandleFunc("/v1/settings/features/{key}", g.handlePUTFeatureSetting).Methods("PUT")

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), nodeContextKey, node)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts
}

// (doReq is defined in discount_handlers_test.go — reuse it.)

// buildResolver constructs a real Resolver with controlled providers.
func buildResolver(platform pkgconfig.PlatformGlobalProvider, store pkgconfig.TenantFeatureStore) pkgconfig.ResolverInterface {
	opts := []pkgconfig.ResolverOption{
		pkgconfig.WithTenantStore(store),
		pkgconfig.WithNodeProvider(pkgconfig.AllowAllNodeProvider{}),
	}
	if platform != nil {
		opts = append(opts, pkgconfig.WithPlatformProvider(platform))
	} else {
		opts = append(opts, pkgconfig.WithPlatformProvider(pkgconfig.AllowAllPlatformProvider{}))
	}
	return pkgconfig.NewResolver(opts...)
}

// ---------------------------------------------------------------------------
// GET /v1/features
// ---------------------------------------------------------------------------

func TestGETFeatures_Success(t *testing.T) {
	store := newMemTenantStore()
	// Seed a tenant override so we can assert the tenant layer is honored.
	_ = store.Set(context.Background(), database.StandaloneTenantID, pkgconfig.FeatureGuestCheckout.Key, true, "seed")

	resolver := buildResolver(nil, store)
	node := &featureTestNode{resolver: resolver, store: store}

	ts := featureTestServer(t, node)
	resp, body := doReq(t, ts, "GET", "/v1/features", nil)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d; body=%s", resp.StatusCode, body)
	}

	var envelope struct {
		Data []map[string]any `json:"data"`
		Meta map[string]any   `json:"meta"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if len(envelope.Data) == 0 {
		t.Fatal("expected at least one feature in response")
	}

	// Locate guestCheckout in the response and sanity-check shape + values.
	var found map[string]any
	for _, f := range envelope.Data {
		if f["key"] == pkgconfig.FeatureGuestCheckout.Key {
			found = f
			break
		}
	}
	if found == nil {
		t.Fatalf("guestCheckout not present in response; got %d features", len(envelope.Data))
	}

	if found["effective"] != true {
		t.Errorf("expected guestCheckout.effective=true (tenant override enabled), got %v", found["effective"])
	}
	if _, ok := found["overridable"].([]any); !ok {
		t.Errorf("expected overridable to be an array, got %T", found["overridable"])
	}
	if _, ok := found["resolution"].(map[string]any); !ok {
		t.Errorf("expected resolution to be an object, got %T", found["resolution"])
	}
}

func TestGETFeatures_ServiceUnavailable_Returns501(t *testing.T) {
	// NodeService that doesn't implement FeaturesProvider — handler should
	// degrade to 501.
	node := &struct{ contracts.NodeService }{}
	ts := featureTestServer(t, node)

	resp, _ := doReq(t, ts, "GET", "/v1/features", nil)
	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("want 501, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// PUT /v1/settings/features/{key}
// ---------------------------------------------------------------------------

func TestPUTFeatureSetting_Enable(t *testing.T) {
	store := newMemTenantStore()
	resolver := buildResolver(nil, store)
	node := &featureTestNode{resolver: resolver, store: store}

	ts := featureTestServer(t, node)

	body, _ := json.Marshal(map[string]bool{"enabled": true})
	resp, respBody := doReq(t, ts, "PUT", "/v1/settings/features/"+pkgconfig.FeatureGuestCheckout.Key, body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d; body=%s", resp.StatusCode, respBody)
	}

	// Verify the store received the write with the expected args.
	if store.lastSet.tenant != database.StandaloneTenantID {
		t.Errorf("tenant mismatch: got %q, want %q", store.lastSet.tenant, database.StandaloneTenantID)
	}
	if store.lastSet.key != pkgconfig.FeatureGuestCheckout.Key {
		t.Errorf("key mismatch: got %q", store.lastSet.key)
	}
	if !store.lastSet.value {
		t.Error("value mismatch: expected true")
	}
	if store.lastSet.actor == "" {
		t.Error("actor should not be empty")
	}

	// Response envelope should contain the updated Evaluation.
	var envelope struct {
		Data struct {
			Key     string `json:"key"`
			Enabled bool   `json:"enabled"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, respBody)
	}
	if envelope.Data.Key != pkgconfig.FeatureGuestCheckout.Key {
		t.Errorf("key in response: got %q", envelope.Data.Key)
	}
	if !envelope.Data.Enabled {
		t.Error("expected enabled=true in response")
	}
}

func TestPUTFeatureSetting_UnknownKey_Returns404(t *testing.T) {
	store := newMemTenantStore()
	resolver := buildResolver(nil, store)
	node := &featureTestNode{resolver: resolver, store: store}

	ts := featureTestServer(t, node)

	body, _ := json.Marshal(map[string]bool{"enabled": true})
	resp, respBody := doReq(t, ts, "PUT", "/v1/settings/features/nonExistentFeatureXYZ", body)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d; body=%s", resp.StatusCode, respBody)
	}
	assertErrorCode(t, respBody, "NOT_FOUND")
}

func TestPUTFeatureSetting_NotTenantScoped_Returns400(t *testing.T) {
	// FeatureNoBuildinWallet has only ScopeNodeRuntime → writing to it via the
	// tenant-layer endpoint must be rejected.
	store := newMemTenantStore()
	resolver := buildResolver(nil, store)
	node := &featureTestNode{resolver: resolver, store: store}

	ts := featureTestServer(t, node)

	body, _ := json.Marshal(map[string]bool{"enabled": true})
	resp, respBody := doReq(t, ts, "PUT", "/v1/settings/features/"+pkgconfig.FeatureNoBuildinWallet.Key, body)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d; body=%s", resp.StatusCode, respBody)
	}
}

func TestPUTFeatureSetting_PlatformDisabled_Returns409(t *testing.T) {
	store := newMemTenantStore()
	// Platform disables everything → tenant cannot turn the feature back on.
	resolver := buildResolver(togglePlatformProvider{enabled: false}, store)
	node := &featureTestNode{resolver: resolver, store: store}

	ts := featureTestServer(t, node)

	body, _ := json.Marshal(map[string]bool{"enabled": true})
	resp, respBody := doReq(t, ts, "PUT", "/v1/settings/features/"+pkgconfig.FeatureGuestCheckout.Key, body)

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("want 409, got %d; body=%s", resp.StatusCode, respBody)
	}

	// Platform rejected the write → store must be untouched.
	if store.lastSet.key != "" {
		t.Errorf("expected no Set call, but got key=%q", store.lastSet.key)
	}
}

func TestPUTFeatureSetting_MalformedBody_Returns400(t *testing.T) {
	store := newMemTenantStore()
	resolver := buildResolver(nil, store)
	node := &featureTestNode{resolver: resolver, store: store}

	ts := featureTestServer(t, node)

	resp, respBody := doReq(t, ts, "PUT", "/v1/settings/features/"+pkgconfig.FeatureGuestCheckout.Key, []byte("{not-json"))

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d; body=%s", resp.StatusCode, respBody)
	}
}

func TestPUTFeatureSetting_AdminUnavailable_Returns501(t *testing.T) {
	store := newMemTenantStore()
	resolver := buildResolver(nil, store)
	node := &nilAdminNode{resolver: resolver}

	ts := featureTestServer(t, node)

	body, _ := json.Marshal(map[string]bool{"enabled": true})
	resp, _ := doReq(t, ts, "PUT", "/v1/settings/features/"+pkgconfig.FeatureGuestCheckout.Key, body)

	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("want 501 (admin provider unavailable), got %d", resp.StatusCode)
	}
}

// assertErrorCode unmarshals the response envelope and asserts the `error.code`
// string matches expected.
func assertErrorCode(t *testing.T, body []byte, expected string) {
	t.Helper()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("unmarshal error envelope: %v; body=%s", err, body)
	}
	if envelope.Error.Code != expected {
		t.Errorf("error code: got %q, want %q", envelope.Error.Code, expected)
	}
}