# Feature Flag Ownership Matrix

> 自动化检查：`scripts/lint-features.sh` 确保本文件与 `features_defined.go` 一致。
> 架构文档：`mobazha_hosting/docs/FEATURE_FLAG_ARCHITECTURE.md`
> 使用指南：`mobazha_hosting/docs/FEATURE_FLAGS_USAGE.md`
> 治理规范：`mobazha_hosting/.cursor/rules/feature-flag-governance.mdc`
>
> 最后更新：2026-05-05

---

## 治理规则

1. **新增 flag 的 PR 必须同时更新本文件**（`scripts/lint-features.sh` 检测一致性）
2. 每个 flag 必须有明确的 **Owner**（负责 Phase/团队）和 **Kill Path**
3. `experimental` 阶段超过 6 个月未推进到 `beta` → 标记为清理候选
4. `deprecated` flag 保留 3 个月别名后删除
5. Kill switch（`kill*` 前缀）永不删除，但可以从 `ClientVisible` 降级为 internal-only

---

## Wallet

### `walletBuiltinDisabled`

- **Owner**: mobazha3.0 (Phase Privacy / External Wallet)
- **Default**: true
- **Stability**: stable
- **Lifecycle**: stable (since pre-pm2)
- **Scopes**: NodeRuntime
- **Consumers**:
  - mobazha3.0/internal/core/builder.go — 节点初始化时跳过钱包派生
  - mobazha3.0/internal/core/node.go — 钱包相关方法 early return
- **Kill Path**: 设 false → 节点启动时初始化内建钱包
- **Dependencies**: 无

---

## Privacy

### `privacyLocalEncryptedStorageEnabled`

- **Owner**: mobazha3.0 (Phase 2 Encryption)
- **Default**: false
- **Stability**: beta
- **Lifecycle**: beta (since phase-2-encryption)
- **Scopes**: NodeRuntime
- **Consumers**:
  - mobazha3.0/internal/core/ — 加密商品存储管道
- **Kill Path**: 设 false → 商品明文存储，不生成 .enc 文件
- **Dependencies**: 无

---

## Payment

### `guestCheckout`

- **Owner**: mobazha3.0 (Phase Privacy PM-2)
- **Default**: false
- **Stability**: beta
- **Lifecycle**: beta (since pm-2, target ga 2026-Q3)
- **Scopes**: PlatformGlobal, Tenant, NodeRuntime
- **Consumers**:
  - mobazha3.0/internal/api/guest_order_handlers.go — Guest Checkout 端点
  - mobazha_hosting/api/ — SaaS 匿名路由分发
  - mobazha-unified/apps/web/src/app/guest-checkout/page.tsx
  - mobazha-unified/apps/web/src/app/guest-order/page.tsx
  - mobazha-unified/apps/web/src/app/admin/settings/guest-checkout/page.tsx
  - mobazha-unified/apps/web/src/components/Order/OrderDetailMobile.tsx
- **Kill Path**: 设 false → guest 端点返回 403；前端自动隐藏入口
- **Dependencies**: 无

---

## Group Marketplace

### `groupMarketplaceEnabled`

- **Owner**: mobazha_hosting (Platform Infrastructure)
- **Default**: false
- **Stability**: beta
- **Lifecycle**: beta (since pre-pm2)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - mobazha_hosting/api/gateway.go:194 — 路由注册条件
  - mobazha_hosting/api/huma_api.go:151 — Huma operation group 注册
  - mobazha_hosting/api/config_handlers.go:127 — serverInfo 响应
  - mobazha-unified/apps/web/src/app/settings/access-control/ — 群组管理 UI
- **Kill Path**: 设 false → product-groups/group-marketplace 路由不注册（404）
- **Dependencies**: 无

---

## Platform

### `platformTestEnvEnabled`

- **Owner**: mobazha_hosting (DevOps)
- **Default**: false
- **Stability**: stable
- **Lifecycle**: stable (since pre-pm2)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - mobazha_hosting/api/login.go — /serverInfo features 返回
  - mobazha-unified/packages/core/config/index.ts — 环境标识
  - mobazha-unified/ — 测试环境横幅、Mini App 调试入口
- **Kill Path**: 环境标识，不建议动态翻转；由 app.yaml 固定
- **Dependencies**: 无
- **Note**: 这不是动态功能开关，而是部署态环境标记

### `supplyChainEnabled`

- **Owner**: mobazha_hosting (Phase Supply-Chain)
- **Default**: false
- **Stability**: beta
- **Lifecycle**: beta (since supply-chain, FF-0~FF-4 ✅)
- **Scopes**: PlatformGlobal, Tenant, NodeRuntime
- **Consumers**:
  - mobazha_hosting/api/ — 供应链路由注册（flag 关闭时 404）
  - mobazha3.0/internal/core/ — EventBus OrderFunded listener 注册
  - mobazha3.0/internal/core/ — Supply chain workers（重试/轮询/库存/价格监控）
  - mobazha-unified/apps/web/src/app/admin/sourcing/SourcingFeatureGuard.tsx
  - mobazha-unified/apps/web/src/app/admin/products/page.tsx
- **Kill Path**: 设 false → 供应链路由 404；EventBus listener 不注册；workers 不启动
- **Dependencies**: 无

---

## Multistore (MS-Phase-1)

### `multistoreMyStoresUIEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-1)
- **Default**: false
- **Stability**: beta
- **Lifecycle**: beta (since ms-phase-1)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - mobazha-unified/packages/core/services/featureFlags.ts
  - mobazha-unified/ — Platform Console "My Stores" 入口
- **Kill Path**: 设 false → My Stores UI 入口隐藏
- **Dependencies**: 无

### `multistoreClaimStoreEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-1)
- **Default**: false
- **Stability**: beta
- **Lifecycle**: beta (since ms-phase-1)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - mobazha_hosting/api/store_registry_handlers.go:550 — claim-store endpoint guard
- **Kill Path**: 设 false → /store-registry/claim 返回 503
- **Dependencies**: 无

### `multistoreOwnerReputationBadgeEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-1)
- **Default**: false
- **Stability**: experimental
- **Lifecycle**: experimental (since ms-phase-1)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - mobazha_hosting/api/store_registry_handlers.go:745 — owner-reputation endpoint guard
- **Kill Path**: 设 false → /stores/{peerID}/owner-reputation 返回 503
- **Dependencies**: 无

---

## Storefronts (MS-Phase-2a)

### `storefrontsEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-2a)
- **Default**: false
- **Stability**: beta
- **Lifecycle**: beta (since ms-phase-2a)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - mobazha_hosting/api/host_router_middleware.go:312 — 子域名路由分发总开关
  - mobazha_hosting/api/storefront_handlers.go:123 — Storefront CRUD guard
  - mobazha-unified/apps/web/src/app/admin/storefronts/ — Storefront 管理 UI
- **Kill Path**: 设 false → 所有 storefront 路由降级为默认 tenant
- **Dependencies**: 无

### `storefrontsSubdomainRouting`

- **Owner**: mobazha_hosting (Phase MS-Phase-2a)
- **Default**: false
- **Stability**: beta
- **Lifecycle**: beta (since ms-phase-2a)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - mobazha_hosting/api/host_router_middleware.go:313 — 子域名 host 匹配
- **Kill Path**: 设 false → *.mobazha.shop 按默认路由处理
- **Dependencies**: storefrontsEnabled

### `storefrontsProgressivePrice`

- **Owner**: mobazha_hosting (Phase MS-Phase-2a)
- **Default**: false
- **Stability**: experimental
- **Lifecycle**: experimental (since ms-phase-2a)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - mobazha_hosting/api/host_router_middleware.go:402 — X-Storefront-PriceRule header injection
- **Kill Path**: 设 false → 不注入定价规则 header
- **Dependencies**: storefrontsEnabled

### `storefrontsThemeEditorEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-2a)
- **Default**: false
- **Stability**: beta
- **Lifecycle**: beta (since ms-phase-2a)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - mobazha-unified/apps/web/src/components/admin/storefronts/StorefrontForm.tsx
- **Kill Path**: 设 false → 主题编辑器 UI 隐藏
- **Dependencies**: storefrontsEnabled

---

## Telegram (MS-Phase-2b)

### `tgSellerBotWizardEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-2b)
- **Default**: false
- **Stability**: beta
- **Lifecycle**: beta (since ms-phase-2b)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - mobazha_hosting/api/store_bot_handlers.go:644 — Seller Bot 向导 guard
- **Kill Path**: 设 false → Bot 向导端点返回 503
- **Dependencies**: 无（独立于 groupMarketplaceEnabled）

### `tgBridgeBotV2Enabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-2b)
- **Default**: false
- **Stability**: experimental
- **Lifecycle**: experimental (since ms-phase-2b)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - mobazha_hosting/api/host_router_middleware.go:290 — Bridge Bot v2 middleware 切换
- **Kill Path**: 设 false → 回退到 v1 bridge 路由
- **Dependencies**: 无

### `tgBotClusterEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-2b)
- **Default**: false
- **Stability**: experimental
- **Lifecycle**: experimental (since ms-phase-2b)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - mobazha_hosting/ — Bot Cluster supervisor 初始化
- **Kill Path**: 设 false → Bot Cluster 不启动
- **Dependencies**: 无

### `tgChannelEmbedEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-2b)
- **Default**: false
- **Stability**: experimental
- **Lifecycle**: experimental (since ms-phase-2b)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - mobazha-unified/ — Channel embed 渲染组件
- **Kill Path**: 设 false → Channel 嵌入 UI 隐藏
- **Dependencies**: 无

---

## Staff Accounts (MS-Phase-3)

### `staffAccountsEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-3)
- **Default**: false
- **Stability**: experimental
- **Lifecycle**: experimental (since ms-phase-3)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - (待实施) — Staff 登录、角色绑定路由
- **Kill Path**: 设 false → Staff 端点 404
- **Dependencies**: 无

### `staffAuditLogEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-3)
- **Default**: false
- **Stability**: experimental
- **Lifecycle**: experimental (since ms-phase-3)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - (待实施) — 审计日志写入 + 查询 UI
- **Kill Path**: 设 false → 不记录审计日志
- **Dependencies**: staffAccountsEnabled

### `staffStepUpAuthEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-3)
- **Default**: false
- **Stability**: experimental
- **Lifecycle**: experimental (since ms-phase-3)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - (待实施) — 敏感操作二次验证中间件
- **Kill Path**: 设 false → 跳过 step-up 认证
- **Dependencies**: staffAccountsEnabled

---

## SaaS Multi-Node (MS-Phase-4)

### `saasMultiNodeEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-4)
- **Default**: false
- **Stability**: experimental
- **Lifecycle**: experimental (since ms-phase-4)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - (待实施) — 突破 1:1 tenant↔node 映射
- **Kill Path**: 设 false → 恢复 1:1 映射约束
- **Dependencies**: AH-3 共享调度器完成

### `saasPlanQuotaEnforced`

- **Owner**: mobazha_hosting (Phase MS-Phase-4 / F12)
- **Default**: false
- **Stability**: experimental
- **Lifecycle**: experimental (since ms-phase-4)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - (待实施) — 写路径配额检查中间件
- **Kill Path**: 设 false → 配额检查降级为 log-only
- **Dependencies**: saasMultiNodeEnabled

---

## Identity (MS-Phase-5)

### `identityWalletAnchorEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-5)
- **Default**: false
- **Stability**: experimental
- **Lifecycle**: experimental (since ms-phase-5, 远期)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - (远期) — 钱包地址锚定身份系统
- **Kill Path**: 设 false → 身份继续基于 SaaS account ID
- **Dependencies**: saasMultiNodeEnabled

### `identityCrossStoreAnalyticsEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-5)
- **Default**: false
- **Stability**: experimental
- **Lifecycle**: experimental (since ms-phase-5, 远期)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - (远期) — 跨店铺 GMV/转化指标聚合
- **Kill Path**: 设 false → 分析仅限单店铺
- **Dependencies**: identityWalletAnchorEnabled

### `identityTaxAggregationEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-5)
- **Default**: false
- **Stability**: experimental
- **Lifecycle**: experimental (since ms-phase-5, 远期)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - (远期) — 跨店铺税务合并报表
- **Kill Path**: 设 false → 税务仅限单店铺
- **Dependencies**: identityWalletAnchorEnabled

### `identityMatrixProxyEnabled`

- **Owner**: mobazha_hosting (Phase MS-Phase-5)
- **Default**: false
- **Stability**: experimental
- **Lifecycle**: experimental (since ms-phase-5, 远期)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - (远期) — 统一收件箱 Matrix 代理
- **Kill Path**: 设 false → 聊天仅限单店铺
- **Dependencies**: identityWalletAnchorEnabled

---

## Kill Switches

### `killStorefrontRoutingDisabled`

- **Owner**: mobazha_hosting (DevOps / Incident Response)
- **Default**: false
- **Stability**: stable
- **Lifecycle**: permanent (never remove)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - mobazha_hosting/api/host_router_middleware.go:309 — 跳过 storefront 分发
  - mobazha_hosting/api/storefront_handlers.go:127 — CRUD 返回 503
- **Kill Path**: 设 true → 所有 storefront 流量降级为默认 tenant
- **Dependencies**: 无
- **Incident Runbook**: 当 storefront 路由导致 5xx 飙升时立即启用

### `killMultistoreReadsDisabled`

- **Owner**: mobazha_hosting (DevOps / Incident Response)
- **Default**: false
- **Stability**: stable
- **Lifecycle**: permanent (never remove)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - (待实施) — My Stores / claim-store / owner-reputation 返回 503
- **Kill Path**: 设 true → 多店铺读路径返回 503
- **Dependencies**: 无
- **Incident Runbook**: 多店铺功能导致数据异常时启用隔离

### `killBotClusterIngestDisabled`

- **Owner**: mobazha_hosting (DevOps / Incident Response)
- **Default**: false
- **Stability**: stable
- **Lifecycle**: permanent (never remove)
- **Scopes**: PlatformGlobal
- **Consumers**:
  - (待实施) — Bot Cluster webhook 入站拒绝
- **Kill Path**: 设 true → 停止接收 Telegram webhook 更新
- **Dependencies**: 无
- **Incident Runbook**: Bot Cluster 消息队列积压/异常时启用排水

---

## Digital Goods (DG-1)

### `digitalAutoDeliveryEnabled`

- **Category**: payment
- **Stability**: beta
- **Default**: true
- **Scopes**: Tenant
- **Client-visible**: yes
- **Introduced in**: dg-1
- **Consumers**:
  - `DigitalEntitlementAppService` — EventBus OrderConfirmation 监听，自动创建 DownloadGrant / License 分配
  - `SettlementService.HandleFiatPaymentReady` — Fiat 支付成功后自动 ShipOrder
- **Dependencies**: 无

### `digitalLicenseValidationEnabled`

- **Category**: payment
- **Stability**: beta
- **Default**: false
- **Scopes**: Tenant
- **Client-visible**: yes
- **Introduced in**: dg-1
- **Consumers**:
  - `/v1/stores/{storeID}/licenses/validate` 端点
  - `/v1/stores/{storeID}/licenses/activate` 端点
  - `/v1/stores/{storeID}/licenses/deactivate` 端点
- **Dependencies**: digitalAutoDeliveryEnabled (逻辑前置 — 需先有 license pool 数据)

### `digitalTokenGatingEnabled`

- **Category**: payment
- **Stability**: experimental
- **Default**: false (Phase 2 预注册 — 端点未实施，启用为空操作)
- **Scopes**: PlatformGlobal, Tenant, NodeRuntime
- **Client-visible**: yes
- **Introduced in**: dg-1 (Phase 2 预注册)
- **Consumers**:
  - (Phase 2) `/v1/listings/{slug}/verify-token-gate` 端点
  - (Phase 2) Token Gate claim → 零价 DIRECT 订单
- **Dependencies**: 无

---

## Smart Commerce Agent (Phase SCA)

### `scaEnabled`

- **Category**: platform
- **Stability**: experimental
- **Default**: false
- **Scopes**: PlatformGlobal, Tenant
- **Client-visible**: yes
- **Introduced in**: sca-foundation
- **Consumers**:
  - Agent Runtime Orchestrator 启动
  - Agent API handlers 注册
  - Agent MCP tools 注册
- **Dependencies**: 无

### `scaIntelPipelineEnabled`

- **Category**: platform
- **Stability**: experimental
- **Default**: false
- **Scopes**: PlatformGlobal
- **Client-visible**: no
- **Introduced in**: sca-0
- **Consumers**:
  - Intel Pipeline Collector / Analyzer 启动
  - Scheduler Intel tick job 注册
- **Dependencies**: scaEnabled

### `scaSelectionEngineEnabled`

- **Category**: platform
- **Stability**: experimental
- **Default**: false
- **Scopes**: PlatformGlobal, Tenant
- **Client-visible**: yes
- **Introduced in**: sca-2
- **Consumers**:
  - SelectionEngine API handler
  - OneClickAction listing draft creation
- **Dependencies**: scaEnabled, scaIntelPipelineEnabled

### `scaAgentExecuteEnabled`

- **Category**: platform
- **Stability**: experimental
- **Default**: false
- **Scopes**: PlatformGlobal, Tenant
- **Client-visible**: yes
- **Introduced in**: sca-3
- **Consumers**:
  - Agent write-action permission gate
  - Auto-draft listing / adjust pricing / send notifications
- **Dependencies**: scaEnabled

### `scaCommercialEnabled`

- **Category**: platform
- **Stability**: experimental
- **Default**: false
- **Scopes**: PlatformGlobal
- **Client-visible**: no
- **Introduced in**: sca-6
- **Consumers**:
  - Token quota metering middleware
  - 5% success fee attribution system
  - Tiered subscription enforcement
- **Dependencies**: scaEnabled

---

## Scheduler Workers (AH-3 Sprint 4 — 待注册)

> 以下 flag 将在 AH-3 Sprint 4 中注册，语义为 kill switch（default=true）。
> 当前 worker 已由 shared scheduler 统一管理，flag=false 时 scheduler tick 内 early return。

| 计划 Key | Worker | 间隔 | 涉资金 |
|---|---|---|---|
| `orderTimeoutEnabled` | OrderTimeoutScheduler | 1 min | ✅ |
| `outboxEnabled` | OutboxPoller | 5 sec | ❌ |
| `paymentVerificationEnabled` | PaymentVerificationLoop | 30 sec | ✅ |
| `webhookEnabled` | WebhookEngine Retry + Cleanup | 5s / 1h | ❌ |
| `fiatReconEnabled` | FiatReconciliation + Cleanup | 2min / 24h | ✅ |
| `netdbSyncEnabled` | NetDBSync | 10 min | ❌ |
| `supplyChainWorkersEnabled` | SupplyChain ×5 workers | 30s~30min | ✅(2) |

---

## 统计

| Category | Count | Stability 分布 |
|---|---|---|
| wallet | 1 | 1 stable |
| privacy | 1 | 1 beta |
| payment | 4 | 3 beta, 1 experimental |
| group | 1 | 1 beta |
| platform | 7 | 1 stable, 1 beta, 5 experimental |
| multistore | 3 | 2 beta, 1 experimental |
| storefronts | 4 | 3 beta, 1 experimental |
| tg | 4 | 1 beta, 3 experimental |
| staff | 3 | 3 experimental |
| saas | 2 | 2 experimental |
| identity | 4 | 4 experimental |
| kill | 3 | 3 stable |
| **Total** | **37** | 5 stable, 11 beta, 21 experimental |
