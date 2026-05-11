// Package config — FeatureManager (legacy DefaultValue facade)
//
// 历史背景（FF-1 阶段引入）：保留 *FeatureManager 字段签名以减少调用点 churn，
// 仅从全局 Registry 读取 DefaultValue，不持有任何可变状态。
//
// 当前定位：FF-2/FF-3 已落地的真正 SSOT 是 Resolver / ResolverInterface
// （pkg/config/resolver.go），它叠加 PlatformGlobal / Tenant / NodeRuntime
// 三层 Scope，处理依赖级联、错误降级与指标。**业务代码应优先依赖
// pkgconfig.ResolverInterface**（通过 contracts.FeaturesProvider.Features()
// 注入），而不是这里的 *FeatureManager。
//
// 本类型保留只做两件事：
//  1. 兜底查询单个 Feature 的 DefaultValue（启动早期，Resolver 还未装配）。
//  2. Snapshot() 给少数报表场景快速返回所有 feature 的默认值。
//
// 待办：TD-098 计划退役本类型，将 Resolver 重命名为 FeatureManager 以统一
// 入口，详见 hosting/docs/TECH_DEBT.md。**新代码不应再依赖此类型。**
package config

import (
	"sync"
)

// FeatureManager 仅作为 Registry.DefaultValue 的只读适配器；不持有任何可变状态。
//
// 业务请改用 pkgconfig.ResolverInterface（多层 Scope + 依赖 + 错误降级）。
// 详见包注释和 TD-098。
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

// IsEnabled 返回 feature 的 Registry DefaultValue。
//
// 注意：本方法只看 DefaultValue，不参与 PlatformGlobal / Tenant /
// NodeRuntime 三层 Scope 的合并与 Dependency 级联。**业务代码请改用
// pkgconfig.ResolverInterface.IsEnabled(ctx, key)**。本方法仅在启动早期
// （Resolver 未装配）或快照场景使用。
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

// Snapshot 返回所有已注册 feature 的 DefaultValue 快照。
//
// 注意：结果等同于 DefaultValue 映射，**不反映 Resolver 多层合并结果**。
// 需要真实启用状态请改用 ResolverInterface.List(ctx)。
func (m *FeatureManager) Snapshot() map[string]bool {
	all := ListFeatures()
	out := make(map[string]bool, len(all))
	for _, f := range all {
		out[f.Key] = m.IsEnabled(f)
	}
	return out
}
