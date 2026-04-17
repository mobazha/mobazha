// Package config — FeatureResolver 实现（§3.3 + §13.11 + §13.5）
//
// Resolver 是**业务代码查询 feature 的唯一入口**。它封装：
//
//   1. 注册表查找（未注册 → false + WARN）
//   2. 依赖预检（依赖未启用 → false，§13.5 仅做关闭级联）
//   3. 三层 AND 合并（platform_global / tenant / node_runtime）
//      - 仅评估 feature.AllowedScopes 声明允许的层
//      - tenant 层在 ctx 无 tenantID 时跳过（§13.2）
//   4. 错误降级（provider 故障 → feature.DefaultValue + WARN，§13.11）
//
// 不做缓存：缓存是各 provider 内部的关注点（§13.4）。Resolver 自身
// 在 FF-2 保持纯逻辑，便于单元测试。
package config

import (
	"context"
	"fmt"
	"log"
)

// ResolverInterface 是 Resolver 对外暴露的契约。
//
// 业务代码应**只依赖这个接口**（便于 handler 测试 mock）。
type ResolverInterface interface {
	IsEnabled(ctx context.Context, key string) bool
	Evaluate(ctx context.Context, key string) Evaluation
	List(ctx context.Context) []EffectiveFeature
}

// Evaluation 是 feature 评估的诊断结果，供管理端 API、调试工具消费。
type Evaluation struct {
	Key           string             `json:"key"`
	Enabled       bool               `json:"enabled"`
	DeniedAtLayer Scope              `json:"deniedAtLayer,omitempty"` // 关闭时指出哪一层决定
	Reason        string             `json:"reason,omitempty"`
	Dependencies  []DependencyStatus `json:"dependencies,omitempty"`
	Resolution    LayerResolution    `json:"resolution"` // 三层各自的 effective 值（被跳过的层为 nil）
}

// LayerResolution 携带各层的原始返回（用于 UI 展示与故障排查）。
// 指针语义：nil 表示该层未参与（AllowedScopes 未声明 / tenant 无 ctx / provider 未注入）
type LayerResolution struct {
	PlatformGlobal *bool `json:"platformGlobal,omitempty"`
	Tenant         *bool `json:"tenant,omitempty"`
	NodeRuntime    *bool `json:"nodeRuntime,omitempty"`
}

// DependencyStatus 描述单个依赖 feature 的状态。
type DependencyStatus struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

// EffectiveFeature 是 `GET /v1/features` 单项响应（§4.1）。
// 实际序列化字段由 API 层定义；pkg/config 仅提供业务层视图。
type EffectiveFeature struct {
	Feature   *Feature
	Effective bool
	Eval      Evaluation
}

// ---------------------------------------------------------------------------
// Resolver 实现
// ---------------------------------------------------------------------------

// Resolver 默认实现。
//
// 组装方式见 NewResolver。生产代码通常在 main 组装一次并注入 App Services；
// 测试代码直接构造并按需替换 provider。
type Resolver struct {
	registry  *Registry // nil → 使用全局 defaultRegistry
	platformP PlatformGlobalProvider
	tenantS   TenantFeatureStore
	nodeP     NodeFeatureProvider

	// 依赖评估的最大递归深度，防御性兜底（Registry.Validate 已保证无环，
	// 但非默认 registry 可能绕过）。
	maxDepth int
}

// ResolverOption 配置 Resolver。
type ResolverOption func(*Resolver)

// WithRegistry 替换 Resolver 使用的 Registry（主要给单元测试）。
func WithRegistry(r *Registry) ResolverOption {
	return func(rs *Resolver) { rs.registry = r }
}

// WithPlatformProvider 注入 platform_global 层 provider。
// 未注入时使用 AllowAllPlatformProvider（独立节点默认行为）。
func WithPlatformProvider(p PlatformGlobalProvider) ResolverOption {
	return func(rs *Resolver) { rs.platformP = p }
}

// WithTenantStore 注入 tenant 层 store。
// 未注入时使用 NoopTenantStore（等同于"所有 feature 用 DefaultValue"）。
func WithTenantStore(s TenantFeatureStore) ResolverOption {
	return func(rs *Resolver) { rs.tenantS = s }
}

// WithNodeProvider 注入 node_runtime 层 provider。
// 未注入时使用 AllowAllNodeProvider（SaaS 默认行为）。
func WithNodeProvider(p NodeFeatureProvider) ResolverOption {
	return func(rs *Resolver) { rs.nodeP = p }
}

// NewResolver 构造 Resolver。缺省值：
//   - Registry = 全局 defaultRegistry
//   - PlatformProvider = AllowAllPlatformProvider
//   - TenantStore = NoopTenantStore
//   - NodeProvider = AllowAllNodeProvider
//
// 生产代码根据部署形态 override：
//   - SaaS hosting: WithPlatformProvider(HostingGlobalProvider{...})
//                   WithTenantStore(TenantFeatureStoreGORM{...})
//                   WithNodeProvider(AllowAllNodeProvider{})
//   - 独立节点:     WithPlatformProvider(AllowAllPlatformProvider{})
//                   WithTenantStore(TenantFeatureStoreGORM{固定 _default})
//                   WithNodeProvider(NodeRuntimeProvider{repo.Config})
func NewResolver(opts ...ResolverOption) *Resolver {
	r := &Resolver{
		platformP: AllowAllPlatformProvider{},
		tenantS:   NoopTenantStore{},
		nodeP:     AllowAllNodeProvider{},
		maxDepth:  16,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// IsEnabled 返回 feature 的 effective 值。
//
// 简化路径：不返回 Evaluation 细节。内部仍然走完整评估，但仅输出布尔。
// 记录一次 feature_flag_evaluations_total 指标。
func (r *Resolver) IsEnabled(ctx context.Context, key string) bool {
	eval := r.evaluate(ctx, key, 0)
	RecordFeatureEvaluation(eval)
	return eval.Enabled
}

// Evaluate 返回带诊断信息的评估结果。
// 记录一次 feature_flag_evaluations_total 指标。
func (r *Resolver) Evaluate(ctx context.Context, key string) Evaluation {
	eval := r.evaluate(ctx, key, 0)
	RecordFeatureEvaluation(eval)
	return eval
}

// List 枚举当前 ctx 下所有已注册 feature 的 effective 值。
//
// 用于 `GET /v1/features` 装配响应；feature 数量固定（注册表静态），
// 性能可接受。每个 feature 记录一次 evaluation 指标。
func (r *Resolver) List(ctx context.Context) []EffectiveFeature {
	features := r.listFeatures()
	out := make([]EffectiveFeature, 0, len(features))
	for _, f := range features {
		eval := r.evaluate(ctx, f.Key, 0)
		RecordFeatureEvaluation(eval)
		out = append(out, EffectiveFeature{
			Feature:   f,
			Effective: eval.Enabled,
			Eval:      eval,
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// 内部评估核心
// ---------------------------------------------------------------------------

func (r *Resolver) evaluate(ctx context.Context, key string, depth int) Evaluation {
	eval := Evaluation{Key: key}

	if depth > r.maxDepth {
		// 防御性兜底：Registry.Validate 应已杜绝循环依赖
		eval.Reason = fmt.Sprintf("dependency depth exceeds %d (likely cycle)", r.maxDepth)
		log.Printf("[feature] %s: %s", key, eval.Reason)
		return eval
	}

	feature := r.lookup(key)
	if feature == nil {
		eval.Reason = "feature not registered"
		log.Printf("[feature] %s: not registered; returning false", key)
		return eval
	}

	// 1. Dependencies 预检：任一 dep=false → feature 视为 false（§13.5）
	if len(feature.Dependencies) > 0 {
		eval.Dependencies = make([]DependencyStatus, 0, len(feature.Dependencies))
		for _, depKey := range feature.Dependencies {
			depEval := r.evaluate(ctx, depKey, depth+1)
			eval.Dependencies = append(eval.Dependencies, DependencyStatus{
				Key:     depKey,
				Enabled: depEval.Enabled,
			})
			if !depEval.Enabled {
				eval.Reason = fmt.Sprintf("dependency %q is disabled", depKey)
				return eval
			}
		}
	}

	// 2. 三层 AND 评估（仅走 AllowedScopes 声明的层）
	var (
		platformVal *bool
		tenantVal   *bool
		nodeVal     *bool
	)

	for _, scope := range feature.AllowedScopes {
		switch scope {
		case ScopePlatformGlobal:
			v := r.evalPlatform(ctx, feature)
			platformVal = &v
			if !v {
				eval.DeniedAtLayer = ScopePlatformGlobal
				eval.Reason = "platform_global disabled"
				eval.Resolution = LayerResolution{PlatformGlobal: platformVal, Tenant: tenantVal, NodeRuntime: nodeVal}
				return eval
			}
		case ScopeTenant:
			tenantID := TenantIDFromContext(ctx)
			if tenantID == "" {
				// §13.2 无 tenantID → 跳过该层；Resolution 保持 nil
				continue
			}
			v := r.evalTenant(ctx, tenantID, feature)
			tenantVal = &v
			if !v {
				eval.DeniedAtLayer = ScopeTenant
				eval.Reason = "tenant disabled"
				eval.Resolution = LayerResolution{PlatformGlobal: platformVal, Tenant: tenantVal, NodeRuntime: nodeVal}
				return eval
			}
		case ScopeNodeRuntime:
			v := r.evalNode(ctx, feature)
			nodeVal = &v
			if !v {
				eval.DeniedAtLayer = ScopeNodeRuntime
				eval.Reason = "node_runtime disabled"
				eval.Resolution = LayerResolution{PlatformGlobal: platformVal, Tenant: tenantVal, NodeRuntime: nodeVal}
				return eval
			}
		}
	}

	eval.Enabled = true
	eval.Resolution = LayerResolution{PlatformGlobal: platformVal, Tenant: tenantVal, NodeRuntime: nodeVal}
	return eval
}

// evalPlatform 评估 platform_global 层，错误降级到 DefaultValue（§13.11）。
func (r *Resolver) evalPlatform(ctx context.Context, f *Feature) bool {
	v, err := r.platformP.IsEnabled(ctx, f.Key)
	if err != nil {
		log.Printf("[feature] %s: platform provider error: %v; falling back to default=%v", f.Key, err, f.DefaultValue)
		return f.DefaultValue
	}
	return v
}

// evalTenant 评估 tenant 层。configured=false 时回落到 DefaultValue（§3.3 step 2）。
func (r *Resolver) evalTenant(ctx context.Context, tenantID string, f *Feature) bool {
	v, configured, err := r.tenantS.Get(ctx, tenantID, f.Key)
	if err != nil {
		log.Printf("[feature] %s: tenant store error (tenant=%s): %v; falling back to default=%v", f.Key, tenantID, err, f.DefaultValue)
		return f.DefaultValue
	}
	if !configured {
		return f.DefaultValue
	}
	return v
}

// evalNode 评估 node_runtime 层。NodeFeatureProvider 没有 error —— 从进程内
// struct 读取不会失败。
func (r *Resolver) evalNode(ctx context.Context, f *Feature) bool {
	return r.nodeP.IsEnabled(ctx, f.Key)
}

// ---------------------------------------------------------------------------
// Registry 访问辅助（支持注入自定义 registry 给测试）
// ---------------------------------------------------------------------------

func (r *Resolver) lookup(key string) *Feature {
	if r.registry != nil {
		f, _ := r.registry.Lookup(key)
		return f
	}
	f, _ := LookupFeature(key)
	return f
}

func (r *Resolver) listFeatures() []*Feature {
	if r.registry != nil {
		return r.registry.List()
	}
	return ListFeatures()
}
