// Package config — FeatureManager (Phase FF-1 transitional facade)
//
// FF-1 目标：保留 *FeatureManager 字段签名以减少调用点 churn，但：
//   - 移除旧的 Toggle{} / RegisterToggle / Enable 运行期可变 API；
//   - IsEnabled 改为接受 *Feature 指针，仅从全局 Registry 读取 DefaultValue；
//
// FF-2 将引入 FeatureResolver（三层 Scope + Dependencies），此结构体会退化为
// 对 Resolver 的瘦封装；FF-3 加接入点后将被彻底替换。为了让上层代码只改一次，
// 这里保持 *FeatureManager.IsEnabled(*Feature) bool 的形态。
package config

import (
	"sync"
)

// FeatureManager 提供对 feature flag 的运行期查询能力。
//
// FF-1 阶段：仅作为 Registry 的只读适配器；不持有任何可变状态。
// FF-2 接入 Resolver 后，此类型会注入 Scope provider。
type FeatureManager struct{}

var (
	globalFeatureManager     *FeatureManager
	globalFeatureManagerOnce sync.Once
)

// GetGlobalFeatureManager 返回进程级全局 FeatureManager 实例。
func GetGlobalFeatureManager() *FeatureManager {
	globalFeatureManagerOnce.Do(func() {
		globalFeatureManager = NewFeatureManager()
	})
	return globalFeatureManager
}

// NewFeatureManager 构造一个独立的 FeatureManager（主要给测试使用）。
func NewFeatureManager() *FeatureManager {
	return &FeatureManager{}
}

// IsEnabled 返回 feature 的当前启用状态。
//
// FF-1 语义：仅查询 Registry 中登记的 DefaultValue。
// FF-2 之后：会委托给 FeatureResolver，叠加 PlatformGlobal / Tenant /
// NodeRuntime 三层 Scope 的 AND 合并与 Dependency 级联。
//
// 约定：
//   - f == nil：返回 false（防御性兜底）
//   - 未注册（理论上不会发生，registerFeature 单线程在 package init）：返回 false
func (m *FeatureManager) IsEnabled(f *Feature) bool {
	if f == nil {
		return false
	}
	registered, ok := LookupFeature(f.Key)
	if !ok {
		return false
	}
	return registered.DefaultValue
}

// Snapshot 返回所有已注册 feature 的当前状态快照。
//
// FF-1 阶段结果等同于 DefaultValue 映射；FF-2 后会反映 Resolver 合并结果。
func (m *FeatureManager) Snapshot() map[string]bool {
	all := ListFeatures()
	out := make(map[string]bool, len(all))
	for _, f := range all {
		out[f.Key] = m.IsEnabled(f)
	}
	return out
}
