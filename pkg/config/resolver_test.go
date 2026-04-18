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

// fakePlatform 实现 PlatformGlobalProvider (values, configured, err) 三元组
// 契约。使用 values map：
//   - key 存在 → configured=true，返回 values[key]
//   - key 不存在 → configured=false，Resolver 将回落到 feature.DefaultValue
//
// 测试中想要精确模拟"平台未配置该 feature"时，留空 values 即可；想要
// 模拟"平台显式启用/关闭"时，在 values 中显式写入。
type fakePlatform struct {
	values map[string]bool
	err    error
}

func (f fakePlatform) Get(_ context.Context, key string) (bool, bool, error) {
	if f.err != nil {
		return false, false, f.err
	}
	v, ok := f.values[key]
	if !ok {
		return false, false, nil // 未配置：Resolver 回落到 DefaultValue
	}
	return v, true, nil
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

// TestResolver_MultiLayerDisabled_ShortCircuitsAtPlatform ——
// platform_global + tenant 都关闭时，短路于最靠前（AllowedScopes 首位）
// 的 platform_global，DeniedAtLayer=platform_global，tenant 层不再被评估
// （Resolution.Tenant == nil）。这守护 §3.3 "any layer false → false" 的
// 报告语义：DeniedAtLayer 始终是首次失败的层，而不是最后一层。
func TestResolver_MultiLayerDisabled_ShortCircuitsAtPlatform(t *testing.T) {
	reg := newTestRegistry(t, &Feature{
		Key: "alpha", DisplayName: "A", Description: "a",
		Stability: StabilityStable, DefaultValue: true,
		AllowedScopes: []Scope{ScopePlatformGlobal, ScopeTenant},
	})
	platform := fakePlatform{values: map[string]bool{"alpha": false}}
	store := &fakeTenantStore{data: map[string]map[string]bool{
		"t1": {"alpha": false}, // 也关闭，但不应被评估到
	}}
	r := NewResolver(
		WithRegistry(reg),
		WithPlatformProvider(platform),
		WithTenantStore(store),
	)

	ctx := ContextWithTenantID(context.Background(), "t1")
	eval := r.Evaluate(ctx, "alpha")

	if eval.Enabled {
		t.Error("expected enabled=false when platform disabled")
	}
	if eval.DeniedAtLayer != ScopePlatformGlobal {
		t.Errorf("expected DeniedAtLayer=platform_global (short-circuit), got %q", eval.DeniedAtLayer)
	}
	// 短路应发生在 tenant 层之前 → Resolution.Tenant 必须为 nil
	if eval.Resolution.Tenant != nil {
		t.Errorf("expected Resolution.Tenant=nil (not evaluated after short-circuit), got %v", *eval.Resolution.Tenant)
	}
	if eval.Resolution.PlatformGlobal == nil || *eval.Resolution.PlatformGlobal {
		t.Error("expected Resolution.PlatformGlobal=false (evaluated and rejected)")
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

// TestResolver_Dependencies_TransitiveCascade —— 多级依赖链（A → B → C），
// 最底层 C 在任一层关闭 → 整个链条都应被评估为关闭。守护 §13.5 的
// 递归语义以及 DependencyStatus 仅记录直接依赖（而非传递展开）这一契约。
func TestResolver_Dependencies_TransitiveCascade(t *testing.T) {
	reg := newTestRegistry(t,
		&Feature{
			Key: "grandchild", DisplayName: "C", Description: "c",
			Stability: StabilityStable, DefaultValue: true,
			AllowedScopes: []Scope{ScopeNodeRuntime},
		},
		&Feature{
			Key: "child", DisplayName: "B", Description: "b",
			Stability: StabilityStable, DefaultValue: true,
			AllowedScopes: []Scope{ScopeNodeRuntime},
			Dependencies:  []string{"grandchild"},
		},
		&Feature{
			Key: "parent", DisplayName: "A", Description: "a",
			Stability: StabilityStable, DefaultValue: true,
			AllowedScopes: []Scope{ScopeNodeRuntime},
			Dependencies:  []string{"child"},
		},
	)
	// 只关掉最底层 grandchild（通过 node 层）
	r := NewResolver(
		WithRegistry(reg),
		WithNodeProvider(fakeNode{values: map[string]bool{"grandchild": false}}),
	)

	// parent 应因 child → grandchild 关闭而被级联关闭
	eval := r.Evaluate(context.Background(), "parent")
	if eval.Enabled {
		t.Error("expected parent disabled due to transitive dependency on grandchild")
	}
	// DependencyStatus 只记录 parent 的直接依赖 child，且 child=false
	if len(eval.Dependencies) != 1 || eval.Dependencies[0].Key != "child" {
		t.Fatalf("expected parent.Dependencies=[child], got %+v", eval.Dependencies)
	}
	if eval.Dependencies[0].Enabled {
		t.Error("expected child dependency reported disabled (transitive)")
	}

	// child 自身也应因 grandchild 关闭被级联关闭
	childEval := r.Evaluate(context.Background(), "child")
	if childEval.Enabled {
		t.Error("expected child disabled due to grandchild")
	}
	if len(childEval.Dependencies) != 1 || childEval.Dependencies[0].Key != "grandchild" {
		t.Fatalf("expected child.Dependencies=[grandchild], got %+v", childEval.Dependencies)
	}
	if childEval.Dependencies[0].Enabled {
		t.Error("expected grandchild dependency reported disabled")
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

// TestResolver_PlatformProvider_NotConfigured_FallbackToDefault ——
// 平台未显式配置 feature（configured=false）时，Resolver 必须回落到
// feature.DefaultValue，而不是沿用 provider 的隐式默认。这守护 §3.3 /
// §13.11 的 single-source-of-truth 语义 —— 未配置的 feature 行为 100%
// 由 registerFeature(...) 的 DefaultValue 决定，不受 provider 实现影响。
//
// 这是 TD-069 FF-impl bugfix 的回归测试：修复前，独立节点的
// AllowAllPlatformProvider 恒返 true，会隐式开启 DefaultValue=false
// 的 feature。
func TestResolver_PlatformProvider_NotConfigured_FallbackToDefault(t *testing.T) {
	reg := newTestRegistry(t,
		&Feature{
			Key: "alphaOn", DisplayName: "A", Description: "a",
			Stability: StabilityStable, DefaultValue: true,
			AllowedScopes: []Scope{ScopePlatformGlobal},
		},
		&Feature{
			Key: "betaOff", DisplayName: "B", Description: "b",
			Stability: StabilityStable, DefaultValue: false,
			AllowedScopes: []Scope{ScopePlatformGlobal},
		},
	)
	// platform 为空 → 所有 key 都 configured=false
	platform := fakePlatform{values: map[string]bool{}}
	r := NewResolver(WithRegistry(reg), WithPlatformProvider(platform))

	if !r.IsEnabled(context.Background(), "alphaOn") {
		t.Error("expected alphaOn=true (DefaultValue=true, platform not configured)")
	}
	if r.IsEnabled(context.Background(), "betaOff") {
		t.Error("expected betaOff=false (DefaultValue=false, platform not configured) — this is the TD-069 bugfix regression")
	}
}

// TestResolver_PlatformProvider_ConfiguredExplicitOverride —— 平台显式
// 配置 true/false 时，覆盖 DefaultValue。
func TestResolver_PlatformProvider_ConfiguredExplicitOverride(t *testing.T) {
	reg := newTestRegistry(t,
		&Feature{
			Key: "alphaOverride", DisplayName: "A", Description: "a",
			Stability: StabilityStable, DefaultValue: false, // 默认关
			AllowedScopes: []Scope{ScopePlatformGlobal},
		},
		&Feature{
			Key: "betaOverride", DisplayName: "B", Description: "b",
			Stability: StabilityStable, DefaultValue: true, // 默认开
			AllowedScopes: []Scope{ScopePlatformGlobal},
		},
	)
	platform := fakePlatform{values: map[string]bool{
		"alphaOverride": true,  // 显式开
		"betaOverride":  false, // 显式关
	}}
	r := NewResolver(WithRegistry(reg), WithPlatformProvider(platform))

	if !r.IsEnabled(context.Background(), "alphaOverride") {
		t.Error("expected alphaOverride=true (platform override=true beats DefaultValue=false)")
	}
	if r.IsEnabled(context.Background(), "betaOverride") {
		t.Error("expected betaOverride=false (platform override=false beats DefaultValue=true)")
	}
}

// TestResolver_NoopPlatformProvider_UsesDefaultValue —— 默认 Noop provider
// （独立节点、未 WithPlatformProvider）应让所有 feature 回落到 DefaultValue。
func TestResolver_NoopPlatformProvider_UsesDefaultValue(t *testing.T) {
	reg := newTestRegistry(t,
		&Feature{
			Key: "alphaOn", DisplayName: "A", Description: "a",
			Stability: StabilityStable, DefaultValue: true,
			AllowedScopes: []Scope{ScopePlatformGlobal},
		},
		&Feature{
			Key: "betaOff", DisplayName: "B", Description: "b",
			Stability: StabilityStable, DefaultValue: false,
			AllowedScopes: []Scope{ScopePlatformGlobal},
		},
	)
	r := NewResolver(WithRegistry(reg)) // 默认 NoopPlatformProvider

	if !r.IsEnabled(context.Background(), "alphaOn") {
		t.Error("expected alphaOn=true (Noop platform → DefaultValue=true)")
	}
	if r.IsEnabled(context.Background(), "betaOff") {
		t.Error("expected betaOff=false (Noop platform → DefaultValue=false)")
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
