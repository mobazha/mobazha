// Resolver 单元测试 —— 覆盖三层 AND、依赖级联、错误降级、上下文注入。
package config

import (
	"context"
	"errors"
	"testing"
)

// ---------------------------------------------------------------------------
// Fakes — 可注入的 provider 实现
// ---------------------------------------------------------------------------

type fakePlatform struct {
	values map[string]bool
	err    error
}

func (f fakePlatform) IsEnabled(_ context.Context, key string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	v, ok := f.values[key]
	if !ok {
		return true, nil // 默认放行
	}
	return v, nil
}

type fakeTenantStore struct {
	// tenant -> key -> configured value
	// 若 key 不存在则 configured=false
	data map[string]map[string]bool
	err  error
}

func (f *fakeTenantStore) Get(_ context.Context, tenantID, key string) (bool, bool, error) {
	if f.err != nil {
		return false, false, f.err
	}
	if ts, ok := f.data[tenantID]; ok {
		if v, ok2 := ts[key]; ok2 {
			return v, true, nil
		}
	}
	return false, false, nil
}

func (f *fakeTenantStore) Set(_ context.Context, tenantID, key string, value bool, _ string) error {
	if f.data == nil {
		f.data = map[string]map[string]bool{}
	}
	if _, ok := f.data[tenantID]; !ok {
		f.data[tenantID] = map[string]bool{}
	}
	f.data[tenantID][key] = value
	return nil
}

func (f *fakeTenantStore) List(_ context.Context, tenantID string) (map[string]bool, error) {
	if ts, ok := f.data[tenantID]; ok {
		out := map[string]bool{}
		for k, v := range ts {
			out[k] = v
		}
		return out, nil
	}
	return map[string]bool{}, nil
}

type fakeNode struct {
	values map[string]bool
}

func (f fakeNode) IsEnabled(_ context.Context, key string) bool {
	v, ok := f.values[key]
	if !ok {
		return true
	}
	return v
}

// ---------------------------------------------------------------------------
// Helper: 构造一个带少量 feature 的独立 registry
// ---------------------------------------------------------------------------

func newTestRegistry(t *testing.T, features ...*Feature) *Registry {
	t.Helper()
	r := &Registry{features: map[string]*Feature{}}
	for _, f := range features {
		r.register(f)
	}
	return r
}

// ---------------------------------------------------------------------------
// 基础行为
// ---------------------------------------------------------------------------

// TestResolver_DefaultProviders_RegisteredFeatureTrue —— 默认 provider 全放行，
// feature DefaultValue=true → Resolver 返回 true。
func TestResolver_DefaultProviders_RegisteredFeatureTrue(t *testing.T) {
	reg := newTestRegistry(t, &Feature{
		Key: "alpha", DisplayName: "A", Description: "a",
		Stability: StabilityStable, DefaultValue: true,
		AllowedScopes: []Scope{ScopePlatformGlobal, ScopeTenant, ScopeNodeRuntime},
	})
	r := NewResolver(WithRegistry(reg))

	if !r.IsEnabled(context.Background(), "alpha") {
		t.Error("expected enabled=true")
	}
}

// TestResolver_UnregisteredFeature_False —— 未注册 feature 一律 false（§3.3）。
func TestResolver_UnregisteredFeature_False(t *testing.T) {
	reg := newTestRegistry(t)
	r := NewResolver(WithRegistry(reg))

	eval := r.Evaluate(context.Background(), "missing")
	if eval.Enabled {
		t.Error("expected enabled=false for unregistered key")
	}
	if eval.Reason == "" {
		t.Error("expected non-empty reason for unregistered key")
	}
}

// TestResolver_TenantScope_Disabled —— tenant 显式 configured=false → 关闭。
func TestResolver_TenantScope_Disabled(t *testing.T) {
	reg := newTestRegistry(t, &Feature{
		Key: "alpha", DisplayName: "A", Description: "a",
		Stability: StabilityStable, DefaultValue: true,
		AllowedScopes: []Scope{ScopeTenant},
	})
	store := &fakeTenantStore{
		data: map[string]map[string]bool{
			"tenant-1": {"alpha": false},
		},
	}
	r := NewResolver(WithRegistry(reg), WithTenantStore(store))

	ctx := ContextWithTenantID(context.Background(), "tenant-1")
	eval := r.Evaluate(ctx, "alpha")
	if eval.Enabled {
		t.Error("expected enabled=false when tenant override=false")
	}
	if eval.DeniedAtLayer != ScopeTenant {
		t.Errorf("expected DeniedAtLayer=tenant, got %q", eval.DeniedAtLayer)
	}
}

// TestResolver_TenantScope_NoCtxTenantID_SkipsLayer —— ctx 无 tenantID 且
// feature 仅允许 tenant scope → 跳过该层 → 其它层（无）都默认放行 → true。
func TestResolver_TenantScope_NoCtxTenantID_SkipsLayer(t *testing.T) {
	reg := newTestRegistry(t, &Feature{
		Key: "alpha", DisplayName: "A", Description: "a",
		Stability: StabilityStable, DefaultValue: true,
		AllowedScopes: []Scope{ScopeTenant},
	})
	store := &fakeTenantStore{
		data: map[string]map[string]bool{
			"tenant-1": {"alpha": false},
		},
	}
	r := NewResolver(WithRegistry(reg), WithTenantStore(store))

	// 不注入 tenantID，tenant-1 的 false 不应生效
	if !r.IsEnabled(context.Background(), "alpha") {
		t.Error("expected enabled=true when no tenantID in ctx (tenant layer skipped)")
	}
}

// TestResolver_TenantScope_ConfiguredFalse_NotOverridden —— configured=false
// 回落到 DefaultValue。
func TestResolver_TenantScope_ConfiguredFalse_NotOverridden(t *testing.T) {
	reg := newTestRegistry(t, &Feature{
		Key: "alpha", DisplayName: "A", Description: "a",
		Stability: StabilityStable, DefaultValue: true,
		AllowedScopes: []Scope{ScopeTenant},
	})
	store := &fakeTenantStore{} // 无数据
	r := NewResolver(WithRegistry(reg), WithTenantStore(store))

	ctx := ContextWithTenantID(context.Background(), "tenant-1")
	if !r.IsEnabled(ctx, "alpha") {
		t.Error("expected DefaultValue=true when tenant not configured")
	}
}

// TestResolver_PlatformScope_Disabled —— platform 层关闭。
func TestResolver_PlatformScope_Disabled(t *testing.T) {
	reg := newTestRegistry(t, &Feature{
		Key: "alpha", DisplayName: "A", Description: "a",
		Stability: StabilityStable, DefaultValue: true,
		AllowedScopes: []Scope{ScopePlatformGlobal, ScopeTenant},
	})
	platform := fakePlatform{values: map[string]bool{"alpha": false}}
	r := NewResolver(WithRegistry(reg), WithPlatformProvider(platform))

	ctx := ContextWithTenantID(context.Background(), "tenant-1")
	eval := r.Evaluate(ctx, "alpha")
	if eval.Enabled {
		t.Error("expected enabled=false when platform disabled")
	}
	if eval.DeniedAtLayer != ScopePlatformGlobal {
		t.Errorf("expected DeniedAtLayer=platform_global, got %q", eval.DeniedAtLayer)
	}
}

// TestResolver_NodeScope_Disabled —— node_runtime 层关闭。
func TestResolver_NodeScope_Disabled(t *testing.T) {
	reg := newTestRegistry(t, &Feature{
		Key: "alpha", DisplayName: "A", Description: "a",
		Stability: StabilityStable, DefaultValue: true,
		AllowedScopes: []Scope{ScopePlatformGlobal, ScopeNodeRuntime},
	})
	node := fakeNode{values: map[string]bool{"alpha": false}}
	r := NewResolver(WithRegistry(reg), WithNodeProvider(node))

	eval := r.Evaluate(context.Background(), "alpha")
	if eval.Enabled {
		t.Error("expected enabled=false when node disabled")
	}
	if eval.DeniedAtLayer != ScopeNodeRuntime {
		t.Errorf("expected DeniedAtLayer=node_runtime, got %q", eval.DeniedAtLayer)
	}
}

// TestResolver_ThreeLayerAND —— 所有层都 true → 结果 true。
func TestResolver_ThreeLayerAND(t *testing.T) {
	reg := newTestRegistry(t, &Feature{
		Key: "alpha", DisplayName: "A", Description: "a",
		Stability: StabilityStable, DefaultValue: true,
		AllowedScopes: []Scope{ScopePlatformGlobal, ScopeTenant, ScopeNodeRuntime},
	})
	store := &fakeTenantStore{data: map[string]map[string]bool{"t1": {"alpha": true}}}
	r := NewResolver(
		WithRegistry(reg),
		WithPlatformProvider(fakePlatform{values: map[string]bool{"alpha": true}}),
		WithTenantStore(store),
		WithNodeProvider(fakeNode{values: map[string]bool{"alpha": true}}),
	)
	ctx := ContextWithTenantID(context.Background(), "t1")
	eval := r.Evaluate(ctx, "alpha")
	if !eval.Enabled {
		t.Error("expected enabled=true when all layers true")
	}
	if eval.Resolution.PlatformGlobal == nil || !*eval.Resolution.PlatformGlobal {
		t.Error("expected platform resolution true")
	}
	if eval.Resolution.Tenant == nil || !*eval.Resolution.Tenant {
		t.Error("expected tenant resolution true")
	}
	if eval.Resolution.NodeRuntime == nil || !*eval.Resolution.NodeRuntime {
		t.Error("expected node resolution true")
	}
}

// ---------------------------------------------------------------------------
// 依赖级联
// ---------------------------------------------------------------------------

// TestResolver_Dependencies_DisabledCascade —— 依赖 feature 关闭 → 当前
// feature 关闭（§13.5）。
func TestResolver_Dependencies_DisabledCascade(t *testing.T) {
	reg := newTestRegistry(t,
		&Feature{
			Key: "parent", DisplayName: "Parent", Description: "p",
			Stability: StabilityStable, DefaultValue: true,
			AllowedScopes: []Scope{ScopeNodeRuntime},
		},
		&Feature{
			Key: "child", DisplayName: "Child", Description: "c",
			Stability: StabilityStable, DefaultValue: true,
			AllowedScopes: []Scope{ScopeNodeRuntime},
			Dependencies:  []string{"parent"},
		},
	)
	// 显式关闭 parent 的 node 层 → 依赖级联让 child 也关闭
	r := NewResolver(WithRegistry(reg), WithNodeProvider(fakeNode{values: map[string]bool{"parent": false}}))

	eval := r.Evaluate(context.Background(), "child")
	if eval.Enabled {
		t.Error("expected child disabled because parent disabled at node layer")
	}
	if len(eval.Dependencies) != 1 || eval.Dependencies[0].Key != "parent" {
		t.Errorf("expected one dependency 'parent', got %+v", eval.Dependencies)
	}
	if eval.Dependencies[0].Enabled {
		t.Error("expected parent dependency reported as disabled")
	}
}

// TestResolver_Dependencies_AllEnabled —— 依赖都启用 → 当前 feature 正常评估。
func TestResolver_Dependencies_AllEnabled(t *testing.T) {
	reg := newTestRegistry(t,
		&Feature{
			Key: "parent", DisplayName: "Parent", Description: "p",
			Stability: StabilityStable, DefaultValue: true,
			AllowedScopes: []Scope{ScopeNodeRuntime},
		},
		&Feature{
			Key: "child", DisplayName: "Child", Description: "c",
			Stability: StabilityStable, DefaultValue: true,
			AllowedScopes: []Scope{ScopeNodeRuntime},
			Dependencies:  []string{"parent"},
		},
	)
	r := NewResolver(WithRegistry(reg))

	if !r.IsEnabled(context.Background(), "child") {
		t.Error("expected child enabled when parent enabled")
	}
}

// ---------------------------------------------------------------------------
// 错误降级
// ---------------------------------------------------------------------------

// TestResolver_PlatformProviderError_FallbackToDefault —— platform provider
// 返回错误 → 降级到 DefaultValue（§13.11）。
func TestResolver_PlatformProviderError_FallbackToDefault(t *testing.T) {
	reg := newTestRegistry(t,
		&Feature{
			Key: "alpha", DisplayName: "A", Description: "a",
			Stability: StabilityStable, DefaultValue: true,
			AllowedScopes: []Scope{ScopePlatformGlobal},
		},
		&Feature{
			Key: "beta", DisplayName: "B", Description: "b",
			Stability: StabilityStable, DefaultValue: false,
			AllowedScopes: []Scope{ScopePlatformGlobal},
		},
	)
	platform := fakePlatform{err: errors.New("db down")}
	r := NewResolver(WithRegistry(reg), WithPlatformProvider(platform))

	if !r.IsEnabled(context.Background(), "alpha") {
		t.Error("expected alpha=true (DefaultValue=true) on provider error")
	}
	if r.IsEnabled(context.Background(), "beta") {
		t.Error("expected beta=false (DefaultValue=false) on provider error")
	}
}

// TestResolver_TenantStoreError_FallbackToDefault —— tenant store 错误 → 降级。
func TestResolver_TenantStoreError_FallbackToDefault(t *testing.T) {
	reg := newTestRegistry(t, &Feature{
		Key: "alpha", DisplayName: "A", Description: "a",
		Stability: StabilityStable, DefaultValue: true,
		AllowedScopes: []Scope{ScopeTenant},
	})
	store := &fakeTenantStore{err: errors.New("db down")}
	r := NewResolver(WithRegistry(reg), WithTenantStore(store))

	ctx := ContextWithTenantID(context.Background(), "t1")
	if !r.IsEnabled(ctx, "alpha") {
		t.Error("expected DefaultValue=true on tenant store error")
	}
}

// ---------------------------------------------------------------------------
// Evaluate 诊断信息
// ---------------------------------------------------------------------------

// TestResolver_Evaluate_ResolutionPopulated —— Resolution 字段应反映每层
// 是否评估 / 评估值。
func TestResolver_Evaluate_ResolutionPopulated(t *testing.T) {
	reg := newTestRegistry(t, &Feature{
		Key: "alpha", DisplayName: "A", Description: "a",
		Stability: StabilityStable, DefaultValue: true,
		AllowedScopes: []Scope{ScopeNodeRuntime}, // 仅 node 层参与
	})
	r := NewResolver(
		WithRegistry(reg),
		WithNodeProvider(fakeNode{values: map[string]bool{"alpha": true}}),
	)
	eval := r.Evaluate(context.Background(), "alpha")
	if eval.Resolution.PlatformGlobal != nil {
		t.Error("expected platform resolution nil (not in AllowedScopes)")
	}
	if eval.Resolution.Tenant != nil {
		t.Error("expected tenant resolution nil (not in AllowedScopes)")
	}
	if eval.Resolution.NodeRuntime == nil || !*eval.Resolution.NodeRuntime {
		t.Error("expected node resolution true")
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// TestResolver_List_AllFeatures —— List 返回所有注册 feature 的 effective。
func TestResolver_List_AllFeatures(t *testing.T) {
	reg := newTestRegistry(t,
		&Feature{
			Key: "alpha", DisplayName: "A", Description: "a",
			Stability: StabilityStable, DefaultValue: true,
			AllowedScopes: []Scope{ScopeNodeRuntime},
		},
		&Feature{
			Key: "beta", DisplayName: "B", Description: "b",
			Stability: StabilityStable, DefaultValue: false,
			AllowedScopes: []Scope{ScopeNodeRuntime},
		},
	)
	// 用 fakeNode 明确关闭 beta，验证 List 反映 provider 评估结果
	r := NewResolver(
		WithRegistry(reg),
		WithNodeProvider(fakeNode{values: map[string]bool{"beta": false}}),
	)
	list := r.List(context.Background())
	if len(list) != 2 {
		t.Fatalf("expected 2 features, got %d", len(list))
	}
	byKey := map[string]bool{}
	for _, ef := range list {
		byKey[ef.Feature.Key] = ef.Effective
	}
	if !byKey["alpha"] {
		t.Error("expected alpha=true")
	}
	if byKey["beta"] {
		t.Error("expected beta=false when node provider returns false")
	}
}

// ---------------------------------------------------------------------------
// Context helpers
// ---------------------------------------------------------------------------

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()
	if got := TenantIDFromContext(ctx); got != "" {
		t.Errorf("expected empty tenantID, got %q", got)
	}
	ctx = ContextWithTenantID(ctx, "t1")
	if got := TenantIDFromContext(ctx); got != "t1" {
		t.Errorf("expected t1, got %q", got)
	}

	id, role := ActorFromContext(ctx)
	if id != "" || role != "" {
		t.Errorf("expected empty actor, got id=%q role=%q", id, role)
	}
	ctx = ContextWithActor(ctx, "user-1", "tenant_admin")
	id, role = ActorFromContext(ctx)
	if id != "user-1" || role != "tenant_admin" {
		t.Errorf("expected user-1/tenant_admin, got id=%q role=%q", id, role)
	}
}
