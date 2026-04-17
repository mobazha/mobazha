// Package config — Feature Declarations (SSOT)
//
// 本文件是所有 feature flag 的**唯一声明处**。新增或修改 feature 时：
//
//  1. 在此文件中通过 `registerFeature(Feature{...})` 注册，并导出 *Feature 变量。
//  2. 业务代码引用 `pkgconfig.FeatureXxx` 获取指针后交给 Resolver。
//  3. 对应前端/后端文档（FEATURE_FLAGS_USAGE.md / ARCHITECTURE.md）同步登记。
//
// 禁止在其他包内声明 feature；Registry 通过 package-level var 求值一次性完成。
// 详见 docs/FEATURE_FLAG_ARCHITECTURE.md §13.6。
package config

// FeatureNoBuildinWallet — 禁用节点内建钱包（外部钱包交付场景）
var FeatureNoBuildinWallet = registerFeature(Feature{
	Key:          "noBuildinWallet",
	DisplayName:  "Disable built-in wallet",
	Description:  "When enabled, the node does not derive or operate an internal wallet; balances and signing are delegated to external wallets.",
	Category:     "wallet",
	Stability:    StabilityStable,
	DefaultValue: true,
	AllowedScopes: []Scope{
		ScopeNodeRuntime,
	},
	IntroducedIn: "pre-pm2",
})

// FeatureLocalEncryptedStorage — 本地加密存储（.enc 文件），Phase 2 私有商品基础
var FeatureLocalEncryptedStorage = registerFeature(Feature{
	Key:          "localEncryptedStorage",
	DisplayName:  "Local encrypted storage",
	Description:  "Encrypts listing payloads at rest on disk (.enc files); consumed by encrypted listings pipeline.",
	Category:     "privacy",
	Stability:    StabilityBeta,
	DefaultValue: false,
	AllowedScopes: []Scope{
		ScopeNodeRuntime,
	},
	IntroducedIn: "phase-2-encryption",
})

// FeatureGuestCheckout — 匿名游客支付（PM-2）
//
// 三层 Scope 均可控制：
//   - PlatformGlobal：SaaS 平台总开关
//   - Tenant：每个店铺自行启用（适配 merchant 自愿接单匿名订单）
//   - NodeRuntime：独立节点 CLI flag / repo.Config（运维人员可快速关停）
var FeatureGuestCheckout = registerFeature(Feature{
	Key:          "guestCheckout",
	DisplayName:  "Guest checkout",
	Description:  "Allows buyers to place anonymous orders via direct on-chain payment without creating an account (PM-2).",
	Category:     "payment",
	Stability:    StabilityBeta,
	DefaultValue: false,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
		ScopeTenant,
		ScopeNodeRuntime,
	},
	IntroducedIn: "pm-2",
})

// FeatureGroupMarketplace — 群组集市（Telegram / Discord 等平台的 group
// marketplace 能力）。目前仅作为 hosting 平台级 kill switch 存在，由
// hosting 在启动时决定是否初始化对应 PlatformVerifier 和注册 /platform/v1/
// product-groups、/platform/v1/group-marketplace/* 路由。
//
// Scope：仅 PlatformGlobal — 这是面向运营的基础设施开关，tenant 不应
// 自行绕过；如果将来需要租户粒度，可补充 ScopeTenant 再升级业务读取点。
var FeatureGroupMarketplace = registerFeature(Feature{
	Key:          "groupMarketplace",
	DisplayName:  "Group marketplace",
	Description:  "Enables Telegram/Discord-based group marketplace endpoints and platform verifiers on the hosting gateway.",
	Category:     "marketplace",
	Stability:    StabilityBeta,
	DefaultValue: false,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "pre-pm2",
})

// FeatureIsTestEnv — 环境标识（test vs prod）。
//
// 语义上不是"动态功能开关"，而是给前端/Telegram webhook 标注当前环境
// 以便做 UX 差异（测试环境横幅、Mini App 调试入口等）。部署时通过
// app.yaml `features.is_test_env` 固定；之所以登记进 Registry，是为了
// 满足 SSOT（Single Source of Truth）原则，让 `/platform/v1/features`
// 能一视同仁地枚举所有平台级标识。**强烈不建议运维通过 PATCH API
// 动态翻转它** — hosting 的 legacy `repo.Features.IsTestEnv` 字段仍然
// 只从 yaml 加载，Reload 不会覆盖。
var FeatureIsTestEnv = registerFeature(Feature{
	Key:          "isTestEnv",
	DisplayName:  "Test environment banner",
	Description:  "Marks the deployment as a test environment so clients and webhooks can render environment-specific UI hints.",
	Category:     "platform",
	Stability:    StabilityStable,
	DefaultValue: false,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "pre-pm2",
})
