// Package config — Context helpers for feature flag resolution (§13.2)
//
// Resolver 通过 context.Context 提取调用身份（tenantID / actor），
// 各 handler / 中间件统一调用 ContextWithTenantID / ContextWithActor
// 注入值；Resolver 只读，不修改 ctx。
//
// 空值语义：
//   - TenantIDFromContext 返回 "" 时，Resolver 将**跳过 tenant 层**（仅
//     platform + node 参与 AND 合并）。handler 必须在需要 tenant 粒度的
//     feature 上自行保证 ctx 已注入 tenantID。
package config

import "context"

// ctxKey 是 package-private 类型，避免与其他包的 context key 冲突。
type ctxKey int

const (
	ctxTenantID ctxKey = iota
	ctxActorID
	ctxActorRole
)

// ContextWithTenantID 将 tenantID 绑定到 ctx。
// 空串视为"未设置"；Resolver 在 tenant 层会跳过。
func ContextWithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ctxTenantID, tenantID)
}

// TenantIDFromContext 读取 ctx 中的 tenantID；未设置返回空串。
func TenantIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxTenantID).(string); ok {
		return v
	}
	return ""
}

// ContextWithActor 将调用方身份（id + role）绑定到 ctx。
//
// id —— 用户标识（peerID / Casdoor user ID / admin ID），用于审计日志；
// role —— 角色（"platform_admin" / "tenant_admin" / "anonymous"），
// 用于 §13.7 的 overridable 字段过滤。
func ContextWithActor(ctx context.Context, id, role string) context.Context {
	ctx = context.WithValue(ctx, ctxActorID, id)
	ctx = context.WithValue(ctx, ctxActorRole, role)
	return ctx
}

// ActorFromContext 读取 ctx 中的 actor 身份；未设置返回空串。
func ActorFromContext(ctx context.Context) (id, role string) {
	if v, ok := ctx.Value(ctxActorID).(string); ok {
		id = v
	}
	if v, ok := ctx.Value(ctxActorRole).(string); ok {
		role = v
	}
	return
}
