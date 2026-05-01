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

	"github.com/go-chi/chi/v5"

	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
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

// togglePlatformProvider returns a fixed enabled value for every key,
// always configured=true so the Resolver uses it verbatim (matching the
// original semantic of "force-enable" / "force-disable the platform layer").
// Used to simulate a platform_global layer that disables a feature (triggering
// 409 in handlePUTFeatureSetting).
type togglePlatformProvider struct{ enabled bool }

func (p togglePlatformProvider) Get(_ context.Context, _ string) (bool, bool, error) {
	return p.enabled, true, nil
}

// ---------------------------------------------------------------------------
// Stub NodeService that satisfies FeaturesProvider + FeatureAdminProvider.
// ---------------------------------------------------------------------------

type featureTestNode struct {
	contracts.NodeService // embedded interface; other methods will panic if called
	resolver              pkgconfig.ResolverInterface
	store                 pkgconfig.TenantFeatureStore
	// auditLogger is optional — when non-nil the node satisfies
	// contracts.FeatureAuditProvider so handlePUTFeatureSetting's audit path
	// can be exercised end-to-end. Leaving it nil preserves backwards
	// compatibility for tests that only care about store writes.
	auditLogger contracts.FeatureAuditLogger
}

func (n *featureTestNode) Features() pkgconfig.ResolverInterface            { return n.resolver }
func (n *featureTestNode) TenantFeatureStore() pkgconfig.TenantFeatureStore { return n.store }

// FeatureAuditLogger implements contracts.FeatureAuditProvider. Returns nil
// unless the test explicitly injects a logger, in which case recordFeatureAudit
// will route audit entries through it.
func (n *featureTestNode) FeatureAuditLogger() contracts.FeatureAuditLogger {
	return n.auditLogger
}

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

	r := chi.NewMux()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), nodeContextKey, node)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/v1/features", g.handleGETFeatures)
	r.Put("/v1/settings/features/{key}", g.handlePUTFeatureSetting)

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts
}

// (doReq is defined in discount_handlers_test.go — reuse it.)

// buildResolver constructs a real Resolver with controlled providers.
//
// When platform=nil, use a togglePlatformProvider{enabled: true} so the
// tenant-centric tests in this file stay focused on the tenant layer —
// they assume "platform layer is out of the way" and only assert tenant
// overrides. Passing a concrete platform provider overrides this default.
//
// Note: we deliberately do NOT use NoopPlatformProvider (configured=false
// → fall back to DefaultValue) because several features used in these
// tests (e.g. guestCheckout) have DefaultValue=false and would otherwise
// short-circuit at the platform layer regardless of tenant intent.
func buildResolver(platform pkgconfig.PlatformGlobalProvider, store pkgconfig.TenantFeatureStore) pkgconfig.ResolverInterface {
	opts := []pkgconfig.ResolverOption{
		pkgconfig.WithTenantStore(store),
		pkgconfig.WithNodeProvider(pkgconfig.AllowAllNodeProvider{}),
	}
	if platform != nil {
		opts = append(opts, pkgconfig.WithPlatformProvider(platform))
	} else {
		opts = append(opts, pkgconfig.WithPlatformProvider(togglePlatformProvider{enabled: true}))
	}
	return pkgconfig.NewResolver(opts...)
}

// ---------------------------------------------------------------------------
// GET /v1/features
// ---------------------------------------------------------------------------

func TestGETFeatures_Success(t *testing.T) {
	store := newMemTenantStore()
	// Seed a tenant override so we can assert the tenant layer is honored.
	_ = store.Set(context.Background(), database.StandaloneTenantID, pkgconfig.FeatureGuestCheckoutEnabled.Key, true, "seed")

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
		if f["key"] == pkgconfig.FeatureGuestCheckoutEnabled.Key {
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
	resp, respBody := doReq(t, ts, "PUT", "/v1/settings/features/"+pkgconfig.FeatureGuestCheckoutEnabled.Key, body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d; body=%s", resp.StatusCode, respBody)
	}

	// Verify the store received the write with the expected args.
	if store.lastSet.tenant != database.StandaloneTenantID {
		t.Errorf("tenant mismatch: got %q, want %q", store.lastSet.tenant, database.StandaloneTenantID)
	}
	if store.lastSet.key != pkgconfig.FeatureGuestCheckoutEnabled.Key {
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
	if envelope.Data.Key != pkgconfig.FeatureGuestCheckoutEnabled.Key {
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
	// FeatureWalletBuiltinDisabled has only ScopeNodeRuntime → writing to it via the
	// tenant-layer endpoint must be rejected.
	store := newMemTenantStore()
	resolver := buildResolver(nil, store)
	node := &featureTestNode{resolver: resolver, store: store}

	ts := featureTestServer(t, node)

	body, _ := json.Marshal(map[string]bool{"enabled": true})
	resp, respBody := doReq(t, ts, "PUT", "/v1/settings/features/"+pkgconfig.FeatureWalletBuiltinDisabled.Key, body)

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
	resp, respBody := doReq(t, ts, "PUT", "/v1/settings/features/"+pkgconfig.FeatureGuestCheckoutEnabled.Key, body)

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

	resp, respBody := doReq(t, ts, "PUT", "/v1/settings/features/"+pkgconfig.FeatureGuestCheckoutEnabled.Key, []byte("{not-json"))

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
	resp, _ := doReq(t, ts, "PUT", "/v1/settings/features/"+pkgconfig.FeatureGuestCheckoutEnabled.Key, body)

	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("want 501 (admin provider unavailable), got %d", resp.StatusCode)
	}
}

// TestPUTFeatureSetting_AuditLoggerInvoked —— 集成测试：验证完整
// PUT /v1/settings/features/{key} 链路会把变更事件写入 FeatureAuditLogger。
// 涵盖 HANDOFF §ff-impl-tests 要求的"审计日志"路径——handler 的 store 写入
// 成功后必须调用 FeatureAuditLogger.AppendAudit（best-effort），并传入正确
// 的 scope、tenant_id、feature_key、old/new value、actor 等字段。
func TestPUTFeatureSetting_AuditLoggerInvoked(t *testing.T) {
	store := newMemTenantStore()
	resolver := buildResolver(nil, store)
	logger := &fakeAuditLogger{}
	node := &featureTestNode{resolver: resolver, store: store, auditLogger: logger}

	ts := featureTestServer(t, node)

	reqBody, _ := json.Marshal(map[string]bool{"enabled": true})
	resp, respBody := doReq(t, ts, "PUT", "/v1/settings/features/"+pkgconfig.FeatureGuestCheckoutEnabled.Key, reqBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d; body=%s", resp.StatusCode, respBody)
	}

	if len(logger.calls) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(logger.calls))
	}
	entry := logger.calls[0]
	if entry.Scope != string(pkgconfig.ScopeTenant) {
		t.Errorf("scope: got %q, want %q", entry.Scope, string(pkgconfig.ScopeTenant))
	}
	if entry.TenantID != database.StandaloneTenantID {
		t.Errorf("tenant_id: got %q, want %q", entry.TenantID, database.StandaloneTenantID)
	}
	if entry.FeatureKey != pkgconfig.FeatureGuestCheckoutEnabled.Key {
		t.Errorf("feature_key: got %q", entry.FeatureKey)
	}
	if !entry.NewValue {
		t.Error("new_value: want true")
	}
	if entry.CreatedAt.IsZero() {
		t.Error("created_at should be populated by handler")
	}
}

// TestPUTFeatureSetting_AuditLoggerErrorDoesNotFailRequest —— logger 失败
// 必须被 handler 吞掉：商业操作（store 写入）已成功，API 仍应返回 200。
// 守护 FeatureAuditLogger 接口的 best-effort 语义。
func TestPUTFeatureSetting_AuditLoggerErrorDoesNotFailRequest(t *testing.T) {
	store := newMemTenantStore()
	resolver := buildResolver(nil, store)
	logger := &fakeAuditLogger{returnErr: context.DeadlineExceeded}
	node := &featureTestNode{resolver: resolver, store: store, auditLogger: logger}

	ts := featureTestServer(t, node)

	reqBody, _ := json.Marshal(map[string]bool{"enabled": true})
	resp, respBody := doReq(t, ts, "PUT", "/v1/settings/features/"+pkgconfig.FeatureGuestCheckoutEnabled.Key, reqBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 despite logger error, got %d; body=%s", resp.StatusCode, respBody)
	}
	if len(logger.calls) != 1 {
		t.Fatalf("expected AppendAudit to still be attempted once, got %d", len(logger.calls))
	}
	// Store must still reflect the successful write — audit failure should
	// never roll the business op back.
	if !store.values[database.StandaloneTenantID+"|"+pkgconfig.FeatureGuestCheckoutEnabled.Key] {
		t.Error("expected store to contain tenant override even though audit failed")
	}
}

// TestPUTFeatureSetting_PlatformDisabled_NoAuditWrite —— 被 platform 层拒
// 绝（409）的请求，不应触发 audit 写入。防止失败操作污染审计日志。
func TestPUTFeatureSetting_PlatformDisabled_NoAuditWrite(t *testing.T) {
	store := newMemTenantStore()
	resolver := buildResolver(togglePlatformProvider{enabled: false}, store)
	logger := &fakeAuditLogger{}
	node := &featureTestNode{resolver: resolver, store: store, auditLogger: logger}

	ts := featureTestServer(t, node)

	reqBody, _ := json.Marshal(map[string]bool{"enabled": true})
	resp, _ := doReq(t, ts, "PUT", "/v1/settings/features/"+pkgconfig.FeatureGuestCheckoutEnabled.Key, reqBody)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("want 409, got %d", resp.StatusCode)
	}
	if len(logger.calls) != 0 {
		t.Errorf("expected zero audit writes on rejected request, got %d", len(logger.calls))
	}
}

// ---------------------------------------------------------------------------
// recordFeatureAudit — helper used by handlePUTFeatureSetting to persist
// audit rows. Tests here cover the three shapes of `node`:
//
//  1. Non-provider (e.g. minimal NodeService stub) — no-op, no panic.
//  2. Provider returning nil logger (e.g. SaaS proxy shim) — no-op, no panic.
//  3. Provider returning a working logger — AppendAudit is invoked with the
//     supplied entry. Logger errors are swallowed (best-effort contract).
// ---------------------------------------------------------------------------

// fakeAuditLogger records every AppendAudit call for assertions. If returnErr
// is non-nil, AppendAudit returns it to simulate a transient DB failure.
type fakeAuditLogger struct {
	calls     []*models.FeatureFlagAuditLog
	returnErr error
}

func (f *fakeAuditLogger) AppendAudit(_ context.Context, entry *models.FeatureFlagAuditLog) error {
	f.calls = append(f.calls, entry)
	return f.returnErr
}

// auditProviderNode implements contracts.FeatureAuditProvider on top of the
// minimal NodeService stub. The logger may be nil to simulate a shim.
type auditProviderNode struct {
	contracts.NodeService
	logger contracts.FeatureAuditLogger
}

func (a *auditProviderNode) FeatureAuditLogger() contracts.FeatureAuditLogger {
	return a.logger
}

func TestRecordFeatureAudit_NonProviderNode_NoOp(t *testing.T) {
	// Node does not implement FeatureAuditProvider — must not panic, must not
	// attempt to call anything.
	node := struct{ contracts.NodeService }{}
	recordFeatureAudit(node, &models.FeatureFlagAuditLog{
		Scope: "tenant", FeatureKey: "x", NewValue: true, Actor: "a",
	})
}

func TestRecordFeatureAudit_NilLogger_NoOp(t *testing.T) {
	// Node implements the provider but returns nil — must not panic.
	node := &auditProviderNode{logger: nil}
	recordFeatureAudit(node, &models.FeatureFlagAuditLog{
		Scope: "tenant", FeatureKey: "x", NewValue: true, Actor: "a",
	})
}

func TestRecordFeatureAudit_WritesEntry(t *testing.T) {
	logger := &fakeAuditLogger{}
	node := &auditProviderNode{logger: logger}

	oldValue := false
	entry := &models.FeatureFlagAuditLog{
		Scope:      "tenant",
		TenantID:   "_default",
		FeatureKey: "guestCheckout",
		OldValue:   &oldValue,
		NewValue:   true,
		Actor:      "admin-1",
	}
	recordFeatureAudit(node, entry)

	if len(logger.calls) != 1 {
		t.Fatalf("expected 1 AppendAudit call, got %d", len(logger.calls))
	}
	got := logger.calls[0]
	if got.FeatureKey != "guestCheckout" {
		t.Errorf("feature_key: got %q", got.FeatureKey)
	}
	if got.TenantID != "_default" {
		t.Errorf("tenant_id: got %q", got.TenantID)
	}
	if !got.NewValue {
		t.Error("new_value: want true")
	}
}

func TestRecordFeatureAudit_LoggerError_Swallowed(t *testing.T) {
	// AppendAudit failures are best-effort — recordFeatureAudit must swallow
	// the error and not panic. We verify the call was attempted.
	logger := &fakeAuditLogger{returnErr: context.DeadlineExceeded}
	node := &auditProviderNode{logger: logger}

	recordFeatureAudit(node, &models.FeatureFlagAuditLog{
		Scope: "tenant", FeatureKey: "x", NewValue: true, Actor: "a",
	})
	if len(logger.calls) != 1 {
		t.Fatalf("expected AppendAudit to be called once even on error, got %d", len(logger.calls))
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