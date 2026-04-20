// Package config — Feature Flag Registry (Phase FF-1)
//
// 设计文档：mobazha_hosting/docs/FEATURE_FLAG_ARCHITECTURE.md
//
// 本文件定义 Feature Flag 的核心类型与全局注册表：
//   - Scope         — 三层作用域（PlatformGlobal / Tenant / NodeRuntime）
//   - Stability     — 特性成熟度（Experimental / Beta / Stable / Deprecated）
//   - Feature       — 单个 feature 的完整元数据
//   - Registry      — 进程内全局注册表（SSOT）
//   - registerFeature / LookupFeature / ListFeatures / ValidateRegistry
//
// 使用模式：所有 feature 必须在 features_defined.go 的 `var (...)` 块
// 中通过 registerFeature(Feature{...}) 声明，**禁止**在 init() 里注册。
package config

import (
	"fmt"
	"regexp"
	"sync"
	"time"
)

// Scope 定义 feature flag 的作用域层级。
//
// 同一个 feature 可以在多个 Scope 中被独立控制；Resolver 对 scopes 做
// **AND** 合并（任一层关闭即整体关闭）。详见 §13.3 独立节点语义。
type Scope string

const (
	// ScopePlatformGlobal — 平台全局开关（hosting SaaS 控制台）
	ScopePlatformGlobal Scope = "platform_global"
	// ScopeTenant — 每个租户的独立开关（tenant_feature_settings 表）
	ScopeTenant Scope = "tenant"
	// ScopeNodeRuntime — 节点进程级开关（CLI flag / repo.Config）
	ScopeNodeRuntime Scope = "node_runtime"
)

// IsValid 报告 scope 是否为合法值。
func (s Scope) IsValid() bool {
	switch s {
	case ScopePlatformGlobal, ScopeTenant, ScopeNodeRuntime:
		return true
	}
	return false
}

// Stability 标记 feature 的成熟度。Deprecated 特性应设置 SunsetDate
// 并在日志中提示调用方迁移。
type Stability string

const (
	StabilityExperimental Stability = "experimental"
	StabilityBeta         Stability = "beta"
	StabilityStable       Stability = "stable"
	StabilityDeprecated   Stability = "deprecated"
)

// IsValid 报告 stability 是否为合法值。
func (s Stability) IsValid() bool {
	switch s {
	case StabilityExperimental, StabilityBeta, StabilityStable, StabilityDeprecated:
		return true
	}
	return false
}

// featureKeyPattern 限制 Feature.Key 必须是 camelCase，字母开头，长度 2–64，
// 允许数字但不允许连续数字段（对齐设计文档 §13.1）。
var featureKeyPattern = regexp.MustCompile(`^[a-z][a-zA-Z0-9]{1,63}$`)

// Feature 描述一个 feature flag 的完整元数据（SSOT 条目）。
//
// 字段值在注册时捕获一次，运行期不应被修改；Resolver 只读 Registry。
type Feature struct {
	// Key — 唯一标识（camelCase，如 "guestCheckout"）
	Key string

	// DisplayName — Admin UI 展示名
	DisplayName string

	// Description — 面向运营/开发者的一句话说明
	Description string

	// Category — 功能分类（如 "payment" / "privacy" / "search"），用于 Admin UI 分组
	Category string

	// Stability — 特性成熟度
	Stability Stability

	// DefaultValue — 默认启用状态（所有 Scope 都缺失时的兜底值）
	DefaultValue bool

	// AllowedScopes — 该 feature 允许在哪些 Scope 中被配置。
	// Resolver 仅在这些 Scope 上做评估；不在列表中的 Scope 视为透传。
	AllowedScopes []Scope

	// Dependencies — 依赖的其他 feature Key 列表。
	// 仅做**关闭级联**：依赖被关闭 => 本 feature 视为关闭（§13.5）。
	Dependencies []string

	// ClientVisible — true if this flag should be included in the login API
	// response (GET /server/info → features map) and consumed by the
	// frontend via useFeature() / useFeatureFlags(). Node-only or infra
	// flags (wallet, privacy, platform env) should leave this false.
	ClientVisible bool

	// IntroducedIn — 首次引入的版本（语义化版本或 build 号）
	IntroducedIn string

	// DeprecatedIn — 标记为 deprecated 的版本；未标注则为空
	DeprecatedIn string

	// SunsetDate — deprecated 特性的删除截止日；未计划时为零值
	SunsetDate time.Time
}

// Registry 是 feature flag 的进程内全局注册表。
//
// 并发安全：注册发生在 package-level var 求值阶段（单线程），运行期只读。
// 为对齐 §13.6，ValidateRegistry 应在 main 启动时显式调用。
type Registry struct {
	mu       sync.RWMutex
	features map[string]*Feature
	// validated 标记 ValidateRegistry 已成功执行；之后禁止再注册
	validated bool
}

// defaultRegistry 进程全局唯一的注册表实例。
var defaultRegistry = &Registry{
	features: make(map[string]*Feature),
}

// DefaultRegistry 暴露全局 Registry（主要给测试 / Resolver 使用）。
func DefaultRegistry() *Registry {
	return defaultRegistry
}

// registerFeature 向全局 Registry 注册一个 feature。
//
// 必须在 package-level var 块中调用，返回 *Feature 以供业务代码
// 通过 `pkgconfig.FeatureGuestCheckoutEnabled` 形式引用。
//
// 失败时 panic：
//   - Key 不符合 camelCase 规范
//   - Key 重复注册
//   - Scope / Stability 非法
//   - 在 ValidateRegistry 之后再注册
func registerFeature(f Feature) *Feature {
	return defaultRegistry.register(&f)
}

func (r *Registry) register(f *Feature) *Feature {
	if err := validateFeatureShape(f); err != nil {
		panic(fmt.Sprintf("feature flag registration error: %v", err))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.validated {
		panic(fmt.Sprintf("feature flag %q: registration after ValidateRegistry is forbidden", f.Key))
	}
	if _, exists := r.features[f.Key]; exists {
		panic(fmt.Sprintf("feature flag %q: duplicate registration", f.Key))
	}
	r.features[f.Key] = f
	return f
}

// validateFeatureShape 检查 Feature 字段的基本合法性（不检查 dependencies 是否存在——
// 那需要全量注册完成后由 ValidateRegistry 做拓扑/存在性校验）。
func validateFeatureShape(f *Feature) error {
	if !featureKeyPattern.MatchString(f.Key) {
		return fmt.Errorf("invalid Key %q: must be camelCase matching %s", f.Key, featureKeyPattern.String())
	}
	if f.DisplayName == "" {
		return fmt.Errorf("feature %q: DisplayName is required", f.Key)
	}
	if f.Description == "" {
		return fmt.Errorf("feature %q: Description is required", f.Key)
	}
	if !f.Stability.IsValid() {
		return fmt.Errorf("feature %q: invalid Stability %q", f.Key, f.Stability)
	}
	if len(f.AllowedScopes) == 0 {
		return fmt.Errorf("feature %q: AllowedScopes must contain at least one scope", f.Key)
	}
	seen := make(map[Scope]struct{}, len(f.AllowedScopes))
	for _, s := range f.AllowedScopes {
		if !s.IsValid() {
			return fmt.Errorf("feature %q: invalid scope %q", f.Key, s)
		}
		if _, dup := seen[s]; dup {
			return fmt.Errorf("feature %q: duplicate scope %q in AllowedScopes", f.Key, s)
		}
		seen[s] = struct{}{}
	}
	return nil
}

// Lookup 返回指定 key 的 Feature；未注册则返回 nil, false。
func (r *Registry) Lookup(key string) (*Feature, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.features[key]
	return f, ok
}

// LookupFeature 是 Lookup 在全局 Registry 上的便捷封装。
func LookupFeature(key string) (*Feature, bool) {
	return defaultRegistry.Lookup(key)
}

// List 返回 Registry 中所有 feature 的快照（拷贝切片，元素为原指针）。
func (r *Registry) List() []*Feature {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Feature, 0, len(r.features))
	for _, f := range r.features {
		out = append(out, f)
	}
	return out
}

// ListFeatures 是 List 在全局 Registry 上的便捷封装。
func ListFeatures() []*Feature {
	return defaultRegistry.List()
}

// ListClientVisible returns only features marked ClientVisible=true.
// Used by the login API to build the features snapshot sent to frontends,
// replacing the manual category whitelist that was prone to omissions.
func (r *Registry) ListClientVisible() []*Feature {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Feature, 0, len(r.features))
	for _, f := range r.features {
		if f.ClientVisible {
			out = append(out, f)
		}
	}
	return out
}

// ListClientVisibleFeatures 是 ListClientVisible 在全局 Registry 上的便捷封装。
func ListClientVisibleFeatures() []*Feature {
	return defaultRegistry.ListClientVisible()
}

// ValidateRegistry 校验整个 Registry 的完整性：
//   - 所有 Dependencies 必须是已注册的 feature Key
//   - 依赖图不能出现环（DFS 检测）
//
// 调用成功后 Registry 标记为 validated，后续再注册会 panic。
// 建议在进程启动（main）中调用一次；测试场景可重置 Registry。
func ValidateRegistry() error {
	return defaultRegistry.Validate()
}

// Validate 见 ValidateRegistry。
func (r *Registry) Validate() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. 依赖存在性检查
	for key, f := range r.features {
		for _, dep := range f.Dependencies {
			if _, ok := r.features[dep]; !ok {
				return fmt.Errorf("feature %q: dependency %q is not registered", key, dep)
			}
			if dep == key {
				return fmt.Errorf("feature %q: depends on itself", key)
			}
		}
	}

	// 2. 循环依赖检测（DFS 三色标记）
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(r.features))
	var stack []string
	var visit func(key string) error
	visit = func(key string) error {
		switch color[key] {
		case gray:
			return fmt.Errorf("feature dependency cycle detected: %v → %s", stack, key)
		case black:
			return nil
		}
		color[key] = gray
		stack = append(stack, key)
		for _, dep := range r.features[key].Dependencies {
			if err := visit(dep); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		color[key] = black
		return nil
	}
	for key := range r.features {
		if color[key] == white {
			if err := visit(key); err != nil {
				return err
			}
		}
	}

	r.validated = true
	return nil
}

// resetForTest 仅供测试使用：清空 Registry 并取消 validated 标记。
// 生产代码不应调用。
func (r *Registry) resetForTest() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.features = make(map[string]*Feature)
	r.validated = false
}
