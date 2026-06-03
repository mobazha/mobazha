package config

import (
	"strings"
	"testing"
)

// TestRegistry_LookupFeature — 已声明的 feature 必须可查到，且元数据与声明一致。
func TestRegistry_LookupFeature(t *testing.T) {
	f, ok := LookupFeature(FeatureGuestCheckoutEnabled.Key)
	if !ok {
		t.Fatalf("FeatureGuestCheckoutEnabled not found in registry")
	}
	if f != FeatureGuestCheckoutEnabled {
		t.Errorf("LookupFeature returned a different pointer than package-level var")
	}
	if f.Category != "payment" || f.Stability != StabilityBeta {
		t.Errorf("unexpected metadata: category=%q stability=%q", f.Category, f.Stability)
	}
	if len(f.AllowedScopes) != 3 {
		t.Errorf("FeatureGuestCheckoutEnabled should allow 3 scopes, got %d", len(f.AllowedScopes))
	}
}

// TestRegistry_ListFeatures — 列表至少包含三个 default feature。
func TestRegistry_ListFeatures(t *testing.T) {
	all := ListFeatures()
	if len(all) < 3 {
		t.Fatalf("expected >=3 features registered, got %d", len(all))
	}
	keys := make(map[string]bool, len(all))
	for _, f := range all {
		keys[f.Key] = true
	}
	for _, k := range []string{"walletBuiltinDisabled", "privacyLocalEncryptedStorageEnabled", "guestCheckout"} {
		if !keys[k] {
			t.Errorf("feature %q missing from ListFeatures()", k)
		}
	}
}

func TestRegistry_SupplyAvailabilityFeature(t *testing.T) {
	f, ok := LookupFeature(FeatureSupplyAvailabilityEnabled.Key)
	if !ok {
		t.Fatalf("FeatureSupplyAvailabilityEnabled not found in registry")
	}
	if f.DefaultValue {
		t.Fatal("supply availability must default off for shadow rollout")
	}
	if f.ClientVisible {
		t.Fatal("SA-0 must not expose a client-visible flag before API/UI exists")
	}
	if f.Stability != StabilityExperimental {
		t.Fatalf("unexpected stability: %s", f.Stability)
	}
	if len(f.AllowedScopes) != 3 {
		t.Fatalf("expected 3 allowed scopes, got %d", len(f.AllowedScopes))
	}
}

// TestRegistry_ValidateRegistry — 默认注册表应校验通过。
func TestRegistry_ValidateRegistry(t *testing.T) {
	// 重置的 validated 状态不影响——Validate 是幂等的在同一个 Registry 上。
	// 但 defaultRegistry 可能已被其他测试 validate 过；我们不关心副作用，
	// 只关心调用不返回错误。
	if err := ValidateRegistry(); err != nil {
		t.Fatalf("ValidateRegistry() failed on default registry: %v", err)
	}
}

// --- Registry 行为测试（使用独立 Registry，避免污染全局）---

// 通过直接构造 *Registry 测试 register / validate 行为。
// 注意：registerFeature 只作用于 defaultRegistry；此处走 r.register 私有方法。

func TestRegistry_DuplicateKey_Panics(t *testing.T) {
	r := &Registry{features: map[string]*Feature{}}
	r.register(&Feature{
		Key: "dupA", DisplayName: "A", Description: "a",
		Stability: StabilityStable, AllowedScopes: []Scope{ScopeNodeRuntime},
	})
	defer func() {
		if rec := recover(); rec == nil {
			t.Fatalf("expected panic on duplicate registration")
		}
	}()
	r.register(&Feature{
		Key: "dupA", DisplayName: "A2", Description: "a2",
		Stability: StabilityStable, AllowedScopes: []Scope{ScopeNodeRuntime},
	})
}

func TestRegistry_InvalidKey_Panics(t *testing.T) {
	r := &Registry{features: map[string]*Feature{}}
	defer func() {
		if rec := recover(); rec == nil {
			t.Fatalf("expected panic on invalid key")
		} else if !strings.Contains(rec.(string), "invalid Key") {
			t.Errorf("unexpected panic message: %v", rec)
		}
	}()
	r.register(&Feature{
		Key: "Bad_Key", DisplayName: "X", Description: "x",
		Stability: StabilityStable, AllowedScopes: []Scope{ScopeNodeRuntime},
	})
}

func TestRegistry_DependencyMissing(t *testing.T) {
	r := &Registry{features: map[string]*Feature{}}
	r.register(&Feature{
		Key: "child", DisplayName: "C", Description: "c",
		Stability: StabilityStable, AllowedScopes: []Scope{ScopeNodeRuntime},
		Dependencies: []string{"ghostParent"},
	})
	err := r.Validate()
	if err == nil {
		t.Fatalf("expected validation error on missing dependency")
	}
	if !strings.Contains(err.Error(), "ghostParent") {
		t.Errorf("expected missing-dep error to mention key, got: %v", err)
	}
}

func TestRegistry_CircularDependency(t *testing.T) {
	r := &Registry{features: map[string]*Feature{}}
	r.register(&Feature{
		Key: "alpha", DisplayName: "A", Description: "a",
		Stability: StabilityStable, AllowedScopes: []Scope{ScopeNodeRuntime},
		Dependencies: []string{"beta"},
	})
	r.register(&Feature{
		Key: "beta", DisplayName: "B", Description: "b",
		Stability: StabilityStable, AllowedScopes: []Scope{ScopeNodeRuntime},
		Dependencies: []string{"alpha"},
	})
	err := r.Validate()
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle detection error, got: %v", err)
	}
}

func TestRegistry_SelfDependency(t *testing.T) {
	r := &Registry{features: map[string]*Feature{}}
	r.register(&Feature{
		Key: "solo", DisplayName: "S", Description: "s",
		Stability: StabilityStable, AllowedScopes: []Scope{ScopeNodeRuntime},
		Dependencies: []string{"solo"},
	})
	err := r.Validate()
	if err == nil || !strings.Contains(err.Error(), "itself") {
		t.Fatalf("expected self-dependency error, got: %v", err)
	}
}

func TestRegistry_RegisterAfterValidate_Panics(t *testing.T) {
	r := &Registry{features: map[string]*Feature{}}
	r.register(&Feature{
		Key: "foo", DisplayName: "F", Description: "f",
		Stability: StabilityStable, AllowedScopes: []Scope{ScopeNodeRuntime},
	})
	if err := r.Validate(); err != nil {
		t.Fatalf("unexpected validate err: %v", err)
	}
	defer func() {
		if rec := recover(); rec == nil {
			t.Fatalf("expected panic on post-validate registration")
		}
	}()
	r.register(&Feature{
		Key: "bar", DisplayName: "B", Description: "b",
		Stability: StabilityStable, AllowedScopes: []Scope{ScopeNodeRuntime},
	})
}

func TestRegistry_InvalidScope_Panics(t *testing.T) {
	r := &Registry{features: map[string]*Feature{}}
	defer func() {
		if rec := recover(); rec == nil {
			t.Fatalf("expected panic on invalid scope")
		}
	}()
	r.register(&Feature{
		Key: "badScope", DisplayName: "B", Description: "b",
		Stability: StabilityStable, AllowedScopes: []Scope{"nonsense"},
	})
}

func TestRegistry_DuplicateScope_Panics(t *testing.T) {
	r := &Registry{features: map[string]*Feature{}}
	defer func() {
		if rec := recover(); rec == nil {
			t.Fatalf("expected panic on duplicate scope in AllowedScopes")
		}
	}()
	r.register(&Feature{
		Key: "dupScope", DisplayName: "D", Description: "d",
		Stability:     StabilityStable,
		AllowedScopes: []Scope{ScopeNodeRuntime, ScopeNodeRuntime},
	})
}
