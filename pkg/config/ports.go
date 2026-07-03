// Package config — Feature flag ports (§3.4)
//
// 三层 Provider/Store 的接口定义。`pkg/config` 不提供生产实现（除
// AllowAll* 便捷适配器外），具体逻辑由消费仓库注入：
//
//   - mobazha_hosting — PlatformGlobalProvider 读 app.yaml + override DB
//   - mobazha      — TenantFeatureStore 走 GORM；NodeFeatureProvider 读 repo.Config / CLI flag
//
// 这样 `pkg/config` 保持零外部依赖，可被独立节点 / SaaS / 测试复用。
package config

import "context"

// PlatformGlobalProvider 提供 Scope=platform_global 层的值。
//
// 语义：`Get(ctx, key)` 返回该 feature 在 **平台全局** 层的配置状态。
// 签名与 TenantFeatureStore.Get 对称，让 Resolver 三层评估遵循统一规则：
//
//   - configured=true   → 平台有显式 override，Resolver 使用 value。
//   - configured=false  → 平台未显式配置，Resolver 回落到 feature.DefaultValue。
//                         这一语义确保 feature 的 `DefaultValue` 是 single
//                         source of truth —— registerFeature(...) 写什么，
//                         在未配置时就是什么，不依赖 provider 的隐式行为。
//
// 错误处理：provider 返回 error 时，Resolver 会降级到 feature.DefaultValue
// 并记录 metric + WARN 日志（§13.11）。provider 实现应避免传播瞬态错误
// 给上层业务。
type PlatformGlobalProvider interface {
	Get(ctx context.Context, key string) (value bool, configured bool, err error)
}

// TenantFeatureStore 提供 Scope=tenant 层的 CRUD 能力。
//
// `Get` 返回的 configured 表示是否存在显式配置：
//   - configured=false → Resolver 使用 feature.DefaultValue
//   - configured=true  → 使用 value
//
// `Set` 要求 actor 参数用于审计日志（FF-3 接入 feature_flag_audit_log）。
//
// `List` 返回某 tenant 下所有已配置 feature 的 (key, enabled) 映射，
// 供 `GET /v1/features` 端点组装响应用。
type TenantFeatureStore interface {
	Get(ctx context.Context, tenantID, key string) (value bool, configured bool, err error)
	Set(ctx context.Context, tenantID, key string, value bool, actor string) error
	List(ctx context.Context, tenantID string) (map[string]bool, error)
}

// NodeFeatureProvider 提供 Scope=node_runtime 层的值。
//
// 独立节点下通常从 CLI flag / repo.Config 读取；SaaS 节点注入
// AllowAllNodeProvider（该层恒 pass，§13.3）。
//
// 没有 error：node 层从进程内 struct 读取，不会 IO 失败。
type NodeFeatureProvider interface {
	IsEnabled(ctx context.Context, key string) bool
}

// ---------------------------------------------------------------------------
// 便捷适配器：independent 节点 + SaaS 的语义占位
// ---------------------------------------------------------------------------

// NoopPlatformProvider 让 platform_global 层始终返回 configured=false；
// Resolver 会回落到 feature.DefaultValue。
//
// 用于：
//   - 独立节点（没有平台管控，所有 feature 行为由 registerFeature 的
//     DefaultValue 决定）
//   - 单元测试（关注其他层的逻辑）
//
// 注意：此前版本为 AllowAllPlatformProvider（恒返回 true），会隐式覆盖
// DefaultValue=false 的 feature。新语义让 DefaultValue 成为唯一真源。
type NoopPlatformProvider struct{}

func (NoopPlatformProvider) Get(ctx context.Context, key string) (bool, bool, error) {
	return false, false, nil
}

// AllowAllPlatformProvider is a deprecated alias retained only for external
// symbol compatibility during the FF-impl migration. New code MUST use
// NoopPlatformProvider, which correctly expresses the independent-node
// semantics ("no platform configuration → fall back to DefaultValue").
//
// Deprecated: use NoopPlatformProvider.
type AllowAllPlatformProvider = NoopPlatformProvider

// AllowAllNodeProvider 让 node_runtime 层恒返回 true。
//
// 用于 SaaS 节点（多租户共享进程，CLI flag 语义无意义）。
type AllowAllNodeProvider struct{}

func (AllowAllNodeProvider) IsEnabled(ctx context.Context, key string) bool {
	return true
}

// NoopTenantStore 让 tenant 层始终返回 configured=false；Resolver 会
// 回落到 feature.DefaultValue。
//
// 用于：
//   - Registry 内单元测试
//   - Guest Checkout 之前的过渡状态（尚未建表）
type NoopTenantStore struct{}

func (NoopTenantStore) Get(ctx context.Context, tenantID, key string) (bool, bool, error) {
	return false, false, nil
}

func (NoopTenantStore) Set(ctx context.Context, tenantID, key string, value bool, actor string) error {
	return nil
}

func (NoopTenantStore) List(ctx context.Context, tenantID string) (map[string]bool, error) {
	return map[string]bool{}, nil
}
