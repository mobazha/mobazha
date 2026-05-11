// Package config — Feature Declarations (SSOT)
//
// 本文件是所有 feature flag 的**唯一声明处**。新增或修改 feature 时：
//
//  1. 在此文件中通过 `registerFeature(Feature{...})` 注册，并导出 *Feature 变量。
//  2. 业务代码引用 `pkgconfig.FeatureXxx` 获取指针后交给 Resolver。
//  3. 对应前端/后端文档（FEATURE_FLAGS_USAGE.md / ARCHITECTURE.md）同步登记。
//
// 禁止在其他包内声明 feature；Registry 通过 package-level var 求值一次性完成。
//
// 命名规范（详见 docs/FEATURE_FLAG_ARCHITECTURE.md §3.6）：
//
//		{domain}{Feature}[Suffix]
//
//	  - domain 必选（wallet / privacy / payment / group / platform / storefronts /
//	    multistore / tg / staff / saas / identity / kill）
//	  - Suffix 纯 gating 用 Enabled；kill/wallet 反向开关用 Disabled；
//	    形容词自包含语义（Enforced / SubdomainRouting）则不加 Enabled。
package config

// ---------------------------------------------------------------------------
// Wallet
// ---------------------------------------------------------------------------

// FeatureWalletBuiltinDisabled — 禁用节点内建钱包（外部钱包交付场景）
var FeatureWalletBuiltinDisabled = registerFeature(Feature{
	Key:          "walletBuiltinDisabled",
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

// ---------------------------------------------------------------------------
// Privacy
// ---------------------------------------------------------------------------

// FeaturePrivacyLocalEncryptedStorageEnabled — 本地加密存储（.enc 文件），Phase 2 私有商品基础
var FeaturePrivacyLocalEncryptedStorageEnabled = registerFeature(Feature{
	Key:          "privacyLocalEncryptedStorageEnabled",
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

// ---------------------------------------------------------------------------
// Payment
// ---------------------------------------------------------------------------

// FeatureGuestCheckoutEnabled — 匿名游客支付（PM-2）
//
// 三层 Scope 均可控制：
//   - PlatformGlobal：SaaS 平台总开关
//   - Tenant：每个店铺自行启用（适配 merchant 自愿接单匿名订单）
//   - NodeRuntime：独立节点 CLI flag / repo.Config（运维人员可快速关停）
var FeatureGuestCheckoutEnabled = registerFeature(Feature{
	Key:           "guestCheckout",
	DisplayName:   "Guest checkout",
	Description:   "Allows buyers to place anonymous orders via direct on-chain payment without creating an account (PM-2).",
	Category:      "payment",
	Stability:     StabilityBeta,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
		ScopeTenant,
		ScopeNodeRuntime,
	},
	IntroducedIn: "pm-2",
})

// ---------------------------------------------------------------------------
// Group marketplace
// ---------------------------------------------------------------------------

// FeatureGroupMarketplaceEnabled — 群组集市（Telegram / Discord 等平台的 group
// marketplace 能力）。目前仅作为 hosting 平台级 kill switch 存在，由
// hosting 在启动时决定是否初始化对应 PlatformVerifier 和注册 /platform/v1/
// product-groups、/platform/v1/group-marketplace/* 路由。
//
// Scope：仅 PlatformGlobal — 这是面向运营的基础设施开关，tenant 不应
// 自行绕过；如果将来需要租户粒度，可补充 ScopeTenant 再升级业务读取点。
var FeatureGroupMarketplaceEnabled = registerFeature(Feature{
	Key:           "groupMarketplaceEnabled",
	DisplayName:   "Group marketplace",
	Description:   "Enables Telegram/Discord-based group marketplace endpoints and platform verifiers on the hosting gateway.",
	Category:      "group",
	Stability:     StabilityBeta,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "pre-pm2",
})

// ---------------------------------------------------------------------------
// Platform environment
// ---------------------------------------------------------------------------

// FeaturePlatformTestEnvEnabled — 环境标识（test vs prod）。
//
// 语义上不是"动态功能开关"，而是给前端/Telegram webhook 标注当前环境
// 以便做 UX 差异（测试环境横幅、Mini App 调试入口等）。部署时通过
// app.yaml `features.is_test_env` 固定；之所以登记进 Registry，是为了
// 满足 SSOT（Single Source of Truth）原则，让 `/platform/v1/features`
// 能一视同仁地枚举所有平台级标识。**强烈不建议运维通过 PATCH API
// 动态翻转它** — hosting 的 legacy `repo.Features.IsTestEnv` 字段仍然
// 只从 yaml 加载，Reload 不会覆盖。
var FeaturePlatformTestEnvEnabled = registerFeature(Feature{
	Key:          "platformTestEnvEnabled",
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

// ---------------------------------------------------------------------------
// Phase MS (Multi-Store) feature catalog — platform-global gates for
// hosting-only rollout of the multi-store evolution (v2.4).
//
// 每个 feature 当前仅需 PlatformGlobal scope — hosting operator 在 Platform
// Console 一键切换，租户/节点级细化等 Phase MS 业务稳定后再扩展。
//
// 业务消费：通过 contracts.FeaturesProvider 注入 ResolverInterface，
// 然后调用 `node.Features().IsEnabled(ctx, pkgconfig.FeatureXxx.Key)`。
// 详见 docs/MULTI_STORE_DESIGN.md §Feature Flags v2.4。
// ---------------------------------------------------------------------------

// ---------- MS-Phase-1 (1 User × N Stores) ----------

// FeatureMultistoreMyStoresUIEnabled — MS-Phase-1 "My Stores" UI 入口
var FeatureMultistoreMyStoresUIEnabled = registerFeature(Feature{
	Key:           "multistoreMyStoresUIEnabled",
	DisplayName:   "My Stores UI (MS-Phase-1)",
	Description:   "Exposes the Platform Console \"My Stores\" entry point so operators can browse the stores they own across tenants.",
	Category:      "multistore",
	Stability:     StabilityBeta,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-1",
})

// FeatureMultistoreClaimStoreEnabled — MS-Phase-1 认领店铺流程（/store-registry/claim）
var FeatureMultistoreClaimStoreEnabled = registerFeature(Feature{
	Key:           "multistoreClaimStoreEnabled",
	DisplayName:   "Claim store (MS-Phase-1)",
	Description:   "Opens the claim-store endpoint allowing verified owners to bind an existing peer to their SaaS account.",
	Category:      "multistore",
	Stability:     StabilityBeta,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-1",
})

// FeatureMultistoreOwnerReputationBadgeEnabled — MS-Phase-1 店主信誉徽章
var FeatureMultistoreOwnerReputationBadgeEnabled = registerFeature(Feature{
	Key:           "multistoreOwnerReputationBadgeEnabled",
	DisplayName:   "Owner reputation badge",
	Description:   "Exposes /platform/v1/stores/{peerID}/owner-reputation so storefronts can render trust signals.",
	Category:      "multistore",
	Stability:     StabilityExperimental,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-1",
})

// ---------- MS-Phase-2a (Storefront Lite) ----------

// FeatureStorefrontsEnabled — MS-Phase-2a Storefront 轻量抽象总开关
var FeatureStorefrontsEnabled = registerFeature(Feature{
	Key:           "storefrontsEnabled",
	DisplayName:   "Storefronts (MS-Phase-2a)",
	Description:   "Enables the Storefront Lite abstraction — per-store subdomain routing, theme editor and collection filters.",
	Category:      "storefronts",
	Stability:     StabilityBeta,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-2a",
})

// FeatureStorefrontsSubdomainRouting — MS-Phase-2a 子域名路由（依赖 Storefront 总开关）
//
// 行为选择器（未来可能支持多种路由模式），语义自包含，不加 Enabled 后缀。
var FeatureStorefrontsSubdomainRouting = registerFeature(Feature{
	Key:           "storefrontsSubdomainRouting",
	DisplayName:   "Storefront subdomain routing",
	Description:   "Routes *.mobazha.shop (and custom domains) to per-tenant storefronts via the host router middleware.",
	Category:      "storefronts",
	Stability:     StabilityBeta,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-2a",
})

// FeatureStorefrontsProgressivePrice — MS-Phase-2a 累进定价规则引擎
//
// 行为选择器（定价规则引擎），语义自包含，不加 Enabled 后缀。
var FeatureStorefrontsProgressivePrice = registerFeature(Feature{
	Key:           "storefrontsProgressivePrice",
	DisplayName:   "Storefront progressive pricing",
	Description:   "Injects X-Storefront-PriceRule headers so downstream listing APIs can apply per-storefront progressive discounts.",
	Category:      "storefronts",
	Stability:     StabilityExperimental,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-2a",
})

// FeatureStorefrontsThemeEditorEnabled — MS-Phase-2a 主题编辑器
var FeatureStorefrontsThemeEditorEnabled = registerFeature(Feature{
	Key:           "storefrontsThemeEditorEnabled",
	DisplayName:   "Storefront theme editor",
	Description:   "Opens the visual theme editor (colors, fonts, hero section) for per-storefront customization.",
	Category:      "storefronts",
	Stability:     StabilityBeta,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-2a",
})

// ---------- MS-Phase-2b (Telegram Distribution Matrix) ----------

// FeatureTgSellerBotWizardEnabled — MS-Phase-2b 自建 Seller Bot 向导
var FeatureTgSellerBotWizardEnabled = registerFeature(Feature{
	Key:           "tgSellerBotWizardEnabled",
	DisplayName:   "Telegram seller bot wizard",
	Description:   "Enables the self-service seller bot wizard (per-store Telegram webhook + onboarding flow) independent of Group Marketplace.",
	Category:      "tg",
	Stability:     StabilityBeta,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-2b",
})

// FeatureTgBridgeBotV2Enabled — MS-Phase-2b Bridge Bot v2 middleware
var FeatureTgBridgeBotV2Enabled = registerFeature(Feature{
	Key:           "tgBridgeBotV2Enabled",
	DisplayName:   "Telegram bridge bot v2",
	Description:   "Switches the hostRouterMiddleware to the v2 bridge bot flow (X-Store-* headers + storefront-aware routing).",
	Category:      "tg",
	Stability:     StabilityExperimental,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-2b",
})

// FeatureTgBotClusterEnabled — MS-Phase-2b Bot Cluster（多 bot 实例调度）
var FeatureTgBotClusterEnabled = registerFeature(Feature{
	Key:           "tgBotClusterEnabled",
	DisplayName:   "Telegram bot cluster",
	Description:   "Enables the Telegram Bot Cluster supervisor so multiple bot instances can be orchestrated under a single storefront.",
	Category:      "tg",
	Stability:     StabilityExperimental,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-2b",
})

// FeatureTgChannelEmbedEnabled — MS-Phase-2b Telegram Channel 嵌入
var FeatureTgChannelEmbedEnabled = registerFeature(Feature{
	Key:           "tgChannelEmbedEnabled",
	DisplayName:   "Telegram channel embed",
	Description:   "Enables Telegram channel embedding so public broadcast posts can surface inside the storefront.",
	Category:      "tg",
	Stability:     StabilityExperimental,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-2b",
})

// ---------- MS-Phase-3 (Staff Accounts + RBAC) ----------

// FeatureStaffAccountsEnabled — MS-Phase-3 员工账号基础
var FeatureStaffAccountsEnabled = registerFeature(Feature{
	Key:           "staffAccountsEnabled",
	DisplayName:   "Staff accounts",
	Description:   "Enables the staff account system — per-store delegated logins with role-based access control.",
	Category:      "staff",
	Stability:     StabilityExperimental,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-3",
})

// FeatureStaffAuditLogEnabled — MS-Phase-3 员工操作审计
var FeatureStaffAuditLogEnabled = registerFeature(Feature{
	Key:           "staffAuditLogEnabled",
	DisplayName:   "Staff audit log",
	Description:   "Records every staff action to the audit log store and exposes the audit trail UI for store owners.",
	Category:      "staff",
	Stability:     StabilityExperimental,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-3",
})

// FeatureStaffStepUpAuthEnabled — MS-Phase-3 员工敏感操作二次验证
var FeatureStaffStepUpAuthEnabled = registerFeature(Feature{
	Key:           "staffStepUpAuthEnabled",
	DisplayName:   "Staff step-up authentication",
	Description:   "Requires staff members to pass a second authentication factor (TOTP/passkey) before performing sensitive operations.",
	Category:      "staff",
	Stability:     StabilityExperimental,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-3",
})

// ---------- MS-Phase-4 (SaaS Multi-Node) ----------

// FeatureSaasMultiNodeEnabled — MS-Phase-4 SaaS 多节点（突破 1:1 tenant↔node）
var FeatureSaasMultiNodeEnabled = registerFeature(Feature{
	Key:           "saasMultiNodeEnabled",
	DisplayName:   "SaaS multi-node",
	Description:   "Allows a SaaS tenant to own multiple peer nodes instead of the legacy 1:1 tenant-to-node mapping.",
	Category:      "saas",
	Stability:     StabilityExperimental,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-4",
})

// FeatureSaasPlanQuotaEnforced — MS-Phase-4 订阅套餐配额强制执行
//
// 语义自包含（Enforced 本身即开关），不加 Enabled 后缀。
var FeatureSaasPlanQuotaEnforced = registerFeature(Feature{
	Key:           "saasPlanQuotaEnforced",
	DisplayName:   "SaaS plan quota enforcement",
	Description:   "Enforces per-plan resource quotas (stores, peers, listings, media) hard-fail at write paths instead of log-only soft limits.",
	Category:      "saas",
	Stability:     StabilityExperimental,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-4",
})

// ---------- MS-Phase-5 (Wallet-Anchored Identity) ----------

// FeatureIdentityWalletAnchorEnabled — MS-Phase-5 钱包锚定身份
var FeatureIdentityWalletAnchorEnabled = registerFeature(Feature{
	Key:           "identityWalletAnchorEnabled",
	DisplayName:   "Wallet-anchored identity",
	Description:   "Anchors the global user identity to a self-custodial wallet address rather than SaaS account IDs; enables cross-node store ownership proofs.",
	Category:      "identity",
	Stability:     StabilityExperimental,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-5",
})

// FeatureIdentityCrossStoreAnalyticsEnabled — MS-Phase-5 跨店铺分析
var FeatureIdentityCrossStoreAnalyticsEnabled = registerFeature(Feature{
	Key:           "identityCrossStoreAnalyticsEnabled",
	DisplayName:   "Cross-store analytics",
	Description:   "Aggregates GMV, orders and conversion metrics across all stores owned by the same wallet-anchored identity.",
	Category:      "identity",
	Stability:     StabilityExperimental,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-5",
})

// FeatureIdentityTaxAggregationEnabled — MS-Phase-5 跨店铺税务合并
var FeatureIdentityTaxAggregationEnabled = registerFeature(Feature{
	Key:           "identityTaxAggregationEnabled",
	DisplayName:   "Cross-store tax aggregation",
	Description:   "Aggregates tax liabilities across stores under the same wallet-anchored identity and exposes unified tax reports.",
	Category:      "identity",
	Stability:     StabilityExperimental,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-5",
})

// FeatureIdentityMatrixProxyEnabled — MS-Phase-5 身份级 Matrix 代理
var FeatureIdentityMatrixProxyEnabled = registerFeature(Feature{
	Key:           "identityMatrixProxyEnabled",
	DisplayName:   "Identity Matrix proxy",
	Description:   "Proxies Matrix chat sessions through the wallet-anchored identity so one user can answer messages across all their stores from a unified inbox.",
	Category:      "identity",
	Stability:     StabilityExperimental,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-5",
})

// ---------------------------------------------------------------------------
// Kill switches (emergency operator overrides) — prefixed with `kill*` so ops
// can locate them instantly in monitoring dashboards and incident runbooks.
//
// 命名约定：`kill{Target}Disabled` — `Disabled` 显式表达"启用 = 关停目标能力"。
// ---------------------------------------------------------------------------

// FeatureKillStorefrontRoutingDisabled — 紧急关停 Storefront 路由
var FeatureKillStorefrontRoutingDisabled = registerFeature(Feature{
	Key:           "killStorefrontRoutingDisabled",
	DisplayName:   "Kill switch: disable storefront routing",
	Description:   "Emergency kill switch — when enabled, host router skips all storefront dispatch and treats traffic as default tenant.",
	Category:      "kill",
	Stability:     StabilityStable,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-2a",
})

// FeatureKillMultistoreReadsDisabled — 紧急关停多店铺读取路径
var FeatureKillMultistoreReadsDisabled = registerFeature(Feature{
	Key:           "killMultistoreReadsDisabled",
	DisplayName:   "Kill switch: disable multistore reads",
	Description:   "Emergency kill switch — when enabled, all multistore read paths (My Stores, claim-store listings, owner reputation) return 503 so operators can quarantine a broken rollout.",
	Category:      "kill",
	Stability:     StabilityStable,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-1",
})

// FeatureKillBotClusterIngestDisabled — 紧急关停 Bot Cluster 入站消息
var FeatureKillBotClusterIngestDisabled = registerFeature(Feature{
	Key:           "killBotClusterIngestDisabled",
	DisplayName:   "Kill switch: disable bot cluster ingest",
	Description:   "Emergency kill switch — when enabled, the Telegram Bot Cluster stops accepting inbound webhook updates so operators can drain the queue safely.",
	Category:      "kill",
	Stability:     StabilityStable,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
	},
	IntroducedIn: "ms-phase-2b",
})

// ---------------------------------------------------------------------------
// Digital Goods
// ---------------------------------------------------------------------------

// FeatureDigitalAutoDeliveryEnabled — 数字商品自动交付（DG-1）
//
// 门控点：
//   - EventBus OrderConfirmation 监听 — 自动创建 DownloadGrant / License 分配
//   - ShipOrder 自动调用 — 写入 Buyer Portal 摘要
var FeatureDigitalAutoDeliveryEnabled = registerFeature(Feature{
	Key:           "digitalAutoDeliveryEnabled",
	DisplayName:   "Digital auto-delivery",
	Description:   "Automatically delivers digital assets (files, links, license keys) to buyers upon order confirmation.",
	Category:      "payment",
	Stability:     StabilityBeta,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
		ScopeTenant,
		ScopeNodeRuntime,
	},
	IntroducedIn: "dg-1",
})

// FeatureDigitalLicenseValidationEnabled — License 验证 API（DG-1）
//
// 门控点：
//   - /v1/stores/{storeID}/licenses/validate 端点
//   - /v1/stores/{storeID}/licenses/activate 端点
//   - /v1/stores/{storeID}/licenses/deactivate 端点
var FeatureDigitalLicenseValidationEnabled = registerFeature(Feature{
	Key:           "digitalLicenseValidationEnabled",
	DisplayName:   "License validation API",
	Description:   "Exposes public license validation, activation, and deactivation endpoints for software products.",
	Category:      "payment",
	Stability:     StabilityBeta,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
		ScopeTenant,
		ScopeNodeRuntime,
	},
	IntroducedIn: "dg-1",
})

// FeatureDigitalTokenGatingEnabled — Token Gating 验证（Phase 2）
//
// 门控点：
//   - /v1/listings/{slug}/verify-token-gate 端点
//   - Token Gate claim → 零价 DIRECT 订单
var FeatureDigitalTokenGatingEnabled = registerFeature(Feature{
	Key:           "digitalTokenGatingEnabled",
	DisplayName:   "Token gating",
	Description:   "Enables NFT/Token-based gating for digital content — wallet signature verification and on-chain balance checks.",
	Category:      "payment",
	Stability:     StabilityExperimental,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
		ScopeTenant,
		ScopeNodeRuntime,
	},
	IntroducedIn: "dg-2",
})

// ---------------------------------------------------------------------------
// Supply Chain
// ---------------------------------------------------------------------------

// FeatureSupplyChainEnabled — 供应链集成（POD/Dropshipping 履约 + 数字商品交付）
//
// 门控点：
//   - API 路由注册（flag 关闭时返回 404）
//   - EventBus OrderFunded 监听（flag 关闭时不注册 listener）
//   - Worker（重试/轮询/库存/成本）（flag 关闭时不启动）
var FeatureSupplyChainEnabled = registerFeature(Feature{
	Key:           "supplyChainEnabled",
	DisplayName:   "Supply chain integration",
	Description:   "Enables fulfillment provider integration (Printful, Printify, CJ, etc.) for POD/dropshipping auto-fulfillment and digital goods auto-delivery.",
	Category:      "platform",
	Stability:     StabilityBeta,
	DefaultValue:  false,
	ClientVisible: true,
	AllowedScopes: []Scope{
		ScopePlatformGlobal,
		ScopeTenant,
		ScopeNodeRuntime,
	},
	IntroducedIn: "supply-chain",
})
