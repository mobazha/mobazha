# 订单处理核心逻辑总结

## 1. 架构概览

### 1.1 支付策略模式（Strategy + Registry）

```
订单状态机（链无关）
     ↓ 查询
Payment Registry（ChainType → PaymentStrategy 映射）
     ↓ 分发
链特定策略实现（UTXO / EVM / Solana）
```

- **Registry**（`pkg/payment/`）：按 `ChainType` 注册和查找策略
- **PaymentStrategy 接口**：定义支付模型声明 + 自动确认 + 指令生成（4 个生命周期方法）
- **Adapter 实现**（每链一个文件，在 `internal/core/`，统一 `payment_` 前缀）：
  - `payment_strategy_utxo.go` — `utxoAutoConfirmAdapter`：UTXO 链（BTC/BCH/LTC/ZEC），后端监控+签名广播
  - `payment_strategy_evm.go` — `evmAutoConfirmAdapter`：EVM 链（ETH/BSC 等），客户端签名 + 平台 Relay
  - `payment_strategy_solana.go` — `solanaAutoConfirmAdapter`：Solana，客户端签名（Relay 暂未实现）

### 1.2 支付模型（PaymentModel）

| 模型 | 链类型 | 前端角色 | 后端角色 |
|------|--------|----------|----------|
| `monitored` | UTXO | 转账到地址 | 监控交易、签名广播 |
| `client_signed` | EVM/Solana | 连接钱包签名 | 生成指令、验证 txHash |
| `third_party` | 第三方 | 使用支付 SDK | 接收 Webhook |

### 1.3 链特定逻辑包

| 包 | 职责 |
|----|------|
| `internal/payment/evm/` | EVM 链的 Escrow 指令构建、参数构建、签名 |
| `internal/payment/solana/` | Solana 链的 Escrow 指令构建、参数构建、签名 |
| UTXO 逻辑 | 保留在 `internal/core/` 中（通过 wallet 接口调用） |

## 2. 订单生命周期

### 2.1 订单创建（`order_purchase.go`）
- 买家选择商品，验证 listing、卖家身份、价格
- 生成 OrderID、构建支付信息（含 escrow 参数）
- 根据支付方式生成 CANCELABLE 或 MODERATED 支付地址
- 发送 ORDER_OPEN 消息给卖家

### 2.2 支付方式

#### DIRECT（直接支付）
- 买家直接支付到卖家地址，无 escrow
- 退款时从卖家内置钱包直接发送（仅限 UTXO）
- EVM/Solana：前端通过 txid 记录退款

#### CANCELABLE（可取消支付）
- 1-of-2 多签地址，买家或卖家任一方可释放
- 买家支付后，后端通过 `CancelablePaymentReady` 事件触发自动确认流程
- UTXO：后端自动签名广播释放交易
- EVM：后端构建指令通过 Relay 服务中继交易
- Solana：需前端钱包手动确认

#### MODERATED（仲裁支付）
- 2-of-3 多签，买家+卖家+仲裁人
- 完成/退款/争议时需要两方签名
- 前端通过 `GetXxxInstructions` API 获取链特定指令

#### RWA_ESCROW / RWA_INSTANT（RWA 资产交易）
- `RWA_ESCROW`：通过 `createOrderFromListing` 锁定资金，需卖家确认
- `RWA_INSTANT`：通过 `instantBuy` 原子交换完成

### 2.3 订单拒绝（`order_reject.go`）
- **入口**: `RejectOrder` / `GetRefundOrderInstructions`
- 卖家拒绝订单（仅限订单已打开且未进一步操作）
- 若订单已付款且非 CANCELABLE：自动触发退款（`buildRefundMessage`）
- EVM/Solana：`GetRefundOrderInstructions` → 获取退款指令 → 前端钱包签名
- 发送 ORDER_REJECT + REFUND 消息给买家

### 2.4 订单确认（`order_confirm.go`）
- **入口**: `ConfirmOrder` / `GetConfirmOrderInstructions`
- 卖家确认订单并释放 CANCELABLE 资金
- UTXO：`releaseFromCancelableAddress` → 后端签名广播
- EVM：`GetConfirmOrderInstructions` → 构建指令 → 可选 Relay 中继
- Solana：`GetConfirmOrderInstructions` → 构建指令 → 前端钱包签名
- 发送 ORDER_CONFIRMATION 消息给买家

### 2.5 订单取消（`order_cancel.go`）
- **入口**: `CancelOrder` / `GetEscrowReleaseInstructions`
- 仅限 CANCELABLE 订单，买家发起
- UTXO：后端自动释放回买家地址
- EVM/Solana：前端获取指令后钱包签名
- 发送 ORDER_CANCEL 消息给卖家

### 2.6 订单履行（`order_fulfillment.go`）
- **入口**: `FulfillOrder`
- 卖家发送物流信息（物理商品）、数字交付（URL/密码）或加密货币交付（含 RWA Token）
- **MODERATED 订单**：卖家在履行时预签 escrow 释放（`buildEscrowRelease`），构建 `ReleaseInfo`
  - UTXO：计算 escrow 手续费后签名
  - EVM/Solana：手续费为 0，直接签名
- 发送 ORDER_FULFILLMENT（含 ReleaseInfo）消息给买家
- 可多次调用（部分履行 → 完全履行）

### 2.7 订单完成（`order_completion.go`）
- **入口**: `CompleteOrder` / `GetCompleteOrderInstructions`
- 买家确认收货、评价商品
- MODERATED 订单：`releaseCompleteEscrowFunds` 签名 escrow 释放
- 发送 ORDER_COMPLETE（含评价 + EscrowRelease）给卖家

### 2.8 退款（`order_refund.go`）
- **入口**: `RefundOrder` / `GetRefundOrderInstructions` / `RefundOrderViaRelay`
- 卖家主动退款
- **EVM/Solana**（所有支付方式，含 CANCELABLE 和 MODERATED）：
  - 路由：`GetRefundOrderInstructions` → `GetEscrowReleaseInstructions` → `GetCancelInstructions` → `BuildCancelableEscrowReleaseInstructions`
  - 该路径对 MODERATED 同样有效：智能合约的 **seller-refund 特殊路径** 允许卖家仅用自己的签名退款到买家地址（无需 2-of-3）
  - 合约验证逻辑：`destinations.length == 1 && destinations[0] == payerAddress && signature from seller` → 允许
  - **Desktop/自托管模式**：
    1. 前端调用 `/v1/instructions/order/refund` 获取合约交互指令
    2. 前端通过 AppKit 钱包签名并提交交易
    3. 前端将 txid 传给 `RefundOrder`，后端记录并发送 REFUND 消息
  - **Hosting 模式**（Relay 可用时）：
    1. `RefundOrderViaRelay` 内部调用 `GetRefundOrderInstructions` 获取指令
    2. 通过 Relay 服务代发交易（平台 gas wallet 付 gas）
    3. 自动调用 `RefundOrder(txid)` 完成流程
    4. 全程无需前端钱包交互
  - 买家收到 REFUND 消息后仅记录 txid，**无需任何链上操作**
- **UTXO DIRECT**：从卖家内置钱包直接转账退还买家
- **UTXO CANCELABLE**：不支持自动退款（资金已释放到外部钱包，卖家需手动退款）
- **UTXO MODERATED**：`buildEscrowRelease` 构建释放参数 + 卖家签名 → 发送 REFUND（含 ReleaseInfo）→ 买家节点自动补签+广播
- 发送 REFUND 消息给买家

### 2.9 争议处理（`order_disputes.go`）
- **发起争议**: `OpenDispute` — 买家或卖家发起，发送完整合约给仲裁人
- **关闭争议**: `CloseDispute`（仲裁人）— 决定资金分配（买家/卖家/仲裁人三方）、签名释放
- **接受裁决**: `ReleaseFunds` — 买家/卖家接受仲裁结果并执行释放
- **超时释放**: `ReleaseFundsAfterTimeout` — 争议超时后，仅限 UTXO 链支持 escrow timeout
- **入口（指令）**: `GetReleaseFundsInstructions`

## 3. Escrow 签名流程

### 3.1 签名参数构建（链特定）

```
EVM:  BuildEscrowReleaseParams(tos, redeemScript)
      → receivers (地址 bytes) + amounts + message (ethSignatureMessage)

Solana: BuildEscrowReleaseParams(tos, chainCode)
        → receivers (公钥 bytes) + amounts + message (solSignatureMessage)

UTXO: 通过 wallet.SignMultisigTransaction(txn, key, script) 直接签名
```

### 3.2 签名方法

```
EVM:    SignEscrowRelease(tos, script, ethMasterKey)    → secp256k1/ECDSA
Solana: SignEscrowRelease(tos, chainCode, solPrivKey)   → ed25519
UTXO:   escrowWallet.SignMultisigTransaction(txn, key, script) → chain-native
```

### 3.3 签名使用场景

| 场景 | 签名方 | 调用位置 |
|------|--------|----------|
| FulfillOrder（vendor 预签） | vendor | `buildEscrowRelease`（via `FulfillOrder`） |
| CompleteOrder（后端签名） | buyer | `releaseCompleteEscrowFunds` |
| CompleteOrder（构建指令） | buyer + vendor(预签) | `BuildCompleteEscrowInstructions` |
| RefundOrder（MODERATED） | vendor | `buildEscrowRelease`（via `buildRefundMessage`） |
| CloseDispute | moderator | `CloseDispute` |
| ReleaseFunds（后端签名） | buyer/vendor | `ReleaseFunds` |
| ReleaseFunds（指令） | buyer/vendor + moderator(预签) | `BuildDisputeReleaseInstructions` |

## 4. 指令生成 API

### 4.1 Node 层方法（`internal/core/`）

| Node 方法 | 用途 | 对应文件 |
|-----------|------|----------|
| `GetConfirmOrderInstructions` | 确认 CANCELABLE 订单 | `order_confirm.go` |
| `GetEscrowReleaseInstructions` | 取消 CANCELABLE 订单 | `order_cancel.go` |
| `GetCompleteOrderInstructions` | 完成 MODERATED 订单 | `order_completion.go` |
| `GetReleaseFundsInstructions` | 释放争议资金 | `order_disputes.go` |
| `GetRefundOrderInstructions` | 退款/拒绝订单 | `order_reject.go` |

### 4.2 PaymentStrategy 接口方法

每个方法通过 Registry 分发到链策略，返回 `InstructionResult`：
- `nil` Instructions → 后端处理（UTXO）
- 非 `nil` Instructions → 前端签名提交（EVM/Solana）

| 策略方法 | 用途 | UTXO | EVM/Solana |
|----------|------|------|------------|
| `GetConfirmInstructions` | 确认 CANCELABLE 订单 | nil（后端释放） | 返回 escrow 释放指令 |
| `GetCancelInstructions` | 取消 CANCELABLE 订单 | nil（后端释放） | 返回 escrow 释放指令 |
| `GetCompleteInstructions` | 完成 MODERATED 订单 | nil（后端签名广播） | 返回 escrow 释放指令 |
| `GetDisputeReleaseInstructions` | 释放争议资金 | nil（后端签名广播） | 返回 escrow 释放指令 |

## 5. ViaRelay 方法（Hosting 无前端模式）

### 5.1 设计思路

在 Hosting 模式下，用户通过 Telegram/Discord/Web 操作，没有 AppKit 钱包前端。
ViaRelay 方法将「获取指令 → Relay 代发 → 完成操作」封装为单次调用。

### 5.2 方法列表

| ViaRelay 方法 | 功能 | 对应文件 |
|---------------|------|----------|
| `RefundOrderViaRelay` | 退款：获取指令 → Relay → RefundOrder | `payment_relay.go` |
| `RejectOrderViaRelay` | 拒绝：获取退款指令 → Relay → RejectOrder | `payment_relay.go` |
| `CancelOrderViaRelay` | 取消：获取释放指令 → Relay → CancelOrder | `payment_relay.go` |

### 5.3 各链行为

| 链类型 | 行为 |
|--------|------|
| UTXO | 直接委托给标准方法（后端签名广播） |
| EVM | 构建指令 → Relay 服务代发（平台 gas wallet 付 gas） |
| Solana | 返回 `ErrRelayChainNotSupported`（待实现） |

### 5.4 API 自动路由

HTTP handler 根据请求体中 `transactionID` 字段自动选择路径：
- `transactionID != ""` → 前端签名模式（已有 txid，直接调用标准方法）
- `transactionID == ""` → Relay 模式（调用 ViaRelay 方法）

错误通过 `orderActionErrorResponse` 统一映射：

| 错误类型 | HTTP 状态码 | 含义 |
|----------|------------|------|
| `ErrRelayNotAvailable` | 503 | Relay 服务未配置 |
| `ErrRelayChainNotSupported` | 501 | 链类型暂不支持 Relay |
| `ErrBadRequest` | 400 | 订单状态不允许该操作 |
| `ErrNotFound` | 404 | 订单不存在 |
| 其他 | 500 | 内部错误 |

### 5.5 智能合约 Seller-Refund 特殊路径

EVM/Solana 智能合约的 `_verifyTransaction` / `verify_signatures_with_timelock` 中：
- 当 `sigV.length < threshold`（MODERATED 签名不足 2）
- 且 `destinations.length == 1 && destinations[0] == payerAddress`（退款到买家）
- 且签名来自 seller
- → **允许执行**（seller-refund 特殊路径）

这使得 MODERATED 订单的退款/拒绝可以仅用卖家签名完成，与 CANCELABLE 共用 `BuildCancelableEscrowReleaseInstructions` 路径。

## 6. CANCELABLE 自动确认机制

### 6.1 事件驱动分发

```
钱包监控检测到付款
  → 发布 CancelablePaymentReady 事件
    → subscribeCancelablePayments 接收
      → dispatchCancelablePayment 通过 Registry 查找策略
        → strategy.AutoConfirm() 执行链特定逻辑
```

### 6.2 各链行为

- **UTXO**: `handleCancelablePaymentForUTXO` → 释放资金 + 调用 `ConfirmOrder`
- **EVM**: `handleCancelablePaymentForEVM` → 构建指令 + Relay 中继 + 调用 `ConfirmOrder`
- **Solana**: 记录日志，需前端手动确认

### 6.3 并发控制
- `cancelableAutoConfirmInProgress` (sync.Map) 防止同一订单并发处理
- `tryLockAutoConfirm` 获取锁，返回 unlock 函数

## 7. 安全机制

### 7.1 身份与签名
- 订单消息通过 `utils.SignOrderMessage` 签名
- 评价通过 ECDSA (escrowMasterKey) + ed25519 (signer) 双重签名
- P2P 消息通过 libp2p 身份验证

### 7.2 Escrow 资金安全
- UTXO: 多签脚本（2-of-3 或 1-of-2）
- EVM: 智能合约 escrow（合约地址验证签名）
- Solana: 程序 escrow（PDA 验证签名）
- 私钥仅在签名时临时使用，不长期持有

### 7.3 多租户隔离
- 所有 DB 写操作通过 `database.Tx` 接口自动注入 `TenantID`
- 禁止使用 `tx.Read().Save()` 绕过 TenantID 注入

## 8. 关键数据结构

### 8.1 Proto 消息
- `OrderOpen`: 订单创建信息（Listings/Items/Payment/BuyerID）
- `PaymentSent`: 支付信息（含 Script/Chaincode/Method/Amount/RefundAddress）
- `OrderReject`: 卖家拒绝（含 Reason/TransactionID）
- `OrderConfirmation`: 卖家确认（含 TransactionID/PayoutAddress）
- `OrderCancel`: 买家取消（含 TransactionID）
- `OrderFulfillment`: 履行信息（含 FulfilledItems/ReleaseInfo）
- `OrderComplete`: 完成信息（含评价 Ratings + EscrowRelease）
- `Refund`: 退款信息（含 TransactionID 或 ReleaseInfo + Amount）
- `DisputeOpen`: 争议发起（含 Contract/Reason/PayoutAddress）
- `DisputeClose`: 争议关闭（含 ModeratedEscrowRelease + Verdict）
- `EscrowRelease`: Escrow 释放参数（ToAddress/Amount/Signatures/Outpoints/PlatformAddress）

### 8.2 支付方式枚举
- `PaymentSent_DIRECT` (0): 直接支付，无 escrow
- `PaymentSent_CANCELABLE` (1): 可取消支付（1-of-2 多签）
- `PaymentSent_MODERATED` (2): 仲裁支付（2-of-3 多签）
- `PaymentSent_RWA_ESCROW` (3): RWA 托管模式
- `PaymentSent_RWA_INSTANT` (4): RWA 即时原子交换模式

### 8.3 订单状态枚举
- `AWAITING_PAYMENT`: 等待付款
- `PENDING`: 待处理（已付款）
- `AWAITING_FULFILLMENT`: 等待履行（已确认）
- `PARTIALLY_FULFILLED`: 部分履行
- `FULFILLED`: 已履行
- `COMPLETED`: 已完成
- `CANCELED`: 已取消
- `DECLINED`: 已拒绝
- `DISPUTED`: 争议中
- `DECIDED`: 已裁决
- `RESOLVED`: 已解决
- `REFUNDED`: 已退款
- `PAYMENT_FINALIZED`: 支付已确认
- `PROCESSING_ERROR`: 处理错误

## 9. 文件结构

| 文件 | 职责 |
|------|------|
| `order_purchase.go` | 订单创建 |
| `order_reject.go` | 订单拒绝（卖家拒绝） |
| `order_confirm.go` | 订单确认（卖家确认 CANCELABLE） |
| `order_cancel.go` | 订单取消（买家取消 CANCELABLE） |
| `order_fulfillment.go` | 订单履行（卖家发货/交付） |
| `order_completion.go` | 订单完成（买家完成 MODERATED） |
| `order_refund.go` | 退款处理（DIRECT/MODERATED/EVM/Solana） |
| `order_disputes.go` | 争议处理（开启/关闭/释放/超时释放） |
| `order_address_request.go` | 订单支付地址请求 |
| `payment_dispatcher.go` | 策略注册 + 自动确认监控 + 共享辅助函数 |
| `payment_strategy_utxo.go` | UTXO 链支付策略适配器 |
| `payment_strategy_evm.go` | EVM 链支付策略适配器 |
| `payment_strategy_solana.go` | Solana 链支付策略适配器 |
| `payment_relay_evm.go` | EVM 链 relay 交易执行（HostService / HTTP） |
| `payment_relay.go` | ViaRelay 方法 + relayOrDirect + relayInstructions |
| `payment_monitor_utxo.go` | UTXO 支付监控 + 地址订阅 + 多笔聚合 |
| `payment_stripe.go` | Stripe 支付集成（Connect 账户 + PaymentIntent） |
| `payment_rwa.go` | RWA 即时购买监控 + 自动确认 |
| `payment_escrow.go` | Escrow 操作（获取指令、查询状态） |
| `payment_receiving_account.go` | 收款账户管理（CRUD + 默认切换） |
| `chain_evm.go` | EVM 链客户端初始化 + 共享注入 |
| `chain_solana.go` | Solana 链客户端初始化 + 合约配置 |
| `order.go` | 订单查询 |
| `order_utils.go` | 订单工具函数 |
| `moderation.go` | 仲裁人管理 |

## 10. 架构决策记录（ADR）

### ADR-1: 适配器放在 internal/core/ 而非 internal/payment/

**决策**：chain adapter（utxoAutoConfirmAdapter 等）保留在 `internal/core/` 而非移到 `internal/payment/utxo/`。

**原因**：适配器需要深度访问 `MobazhaNode` 的内部资源（wallet、DB、messenger、signer），如果放到独立包需要暴露大量内部接口。适配器只是薄薄的转发层，逻辑本身由 `MobazhaNode` 方法实现，留在 `internal/core/` 保持耦合显式可控。

**影响**：`internal/payment/evm/` 和 `internal/payment/solana/` 仅包含纯计算逻辑（BuildXxxInstructions / SignEscrowRelease），不依赖 MobazhaNode。

### ADR-2: InstructionParams.OrderData / ReleaseInfo 使用 any 类型

**决策**：`InstructionParams` 中的 `OrderData` 和 `ReleaseInfo` 字段使用 `any` 而非具体类型。

**原因**：`PaymentStrategy` 接口定义在 `pkg/payment/`（公共 API 层），如果引入 `*models.Order` 或 `*pb.EscrowRelease` 会导致 pkg 层依赖 pkg/models 和 pkg/orders/mbzpb，增加包耦合。使用 `any` + 适配器中类型断言保持了接口的轻量和独立性。

**权衡**：丧失编译时类型安全，但适配器中的类型断言提供了运行时保障，且错误信息清晰（如 "OrderData must be *models.Order"）。

### ADR-3: Seller-Refund 特殊路径复用 CANCELABLE Release Instructions

**决策**：MODERATED 订单的卖家退款使用与 CANCELABLE 相同的 `BuildCancelableEscrowReleaseInstructions`。

**原因**：智能合约（EVM Escrow.sol `_verifyTransaction()` / Solana `verify_signatures_with_timelock()`）实现了 "seller-refund" 特殊路径——当卖家将所有资金退回买家时，只需卖家一个签名即可，无需满足 2-of-3 条件。这意味着退款操作的合约调用数据与 CANCELABLE 释放完全相同。

**影响**：简化了 ViaRelay 退款流程，Hosting 环境无需协调买家签名。

### ADR-4: 不实现 ConfirmOrderViaRelay

**决策**：`OrderService` 接口中没有 `ConfirmOrderViaRelay` 方法。

**原因**：CANCELABLE 订单确认已由 `autoConfirmEVMCancelablePayment` 自动处理（事件驱动 + relay），无需 API 触发。MODERATED 订单确认需要买家通过钱包签名 multisig 交易（CompleteOrder），不适合纯后端执行。如果未来需要 gas-less 的 MODERATED 确认，可再添加。

### ADR-5: Solana Relay 延迟实现

**决策**：Solana relay 标记为 TODO，返回 `ErrRelayChainNotSupported`。

**原因**：Solana 交易构建依赖 recent blockhash + 特定 RPC 调用，与 EVM 的纯 calldata 模式不同。relay 服务需要额外的 Solana RPC 客户端集成。当前 Solana 交易量不足以优先开发。

**计划**：待 Solana 用户量增长后，在 `relayInstructions` 中添加 Solana 分支，复用 `relay.EVMRelayService` 的抽象（重命名为 `BlockchainRelayService`）。

### ADR-6: MobazhaNode 方法膨胀与 Service 提取演进路线

**现状**：`MobazhaNode` 有 311 个方法，分布在 81 个文件中（29K+ 行）。这是典型的 God Object，但因项目演进历史而形成，不适合一次性重构。

**命名约定（已实施）**：通过文件前缀按域分组，使目录中相关文件自然聚合：
- `payment_*` — 支付策略、relay、监控、escrow、stripe、rwa、收款账户
- `chain_*` — 链客户端初始化（EVM、Solana）
- 订单生命周期保持原名（confirm、cancel、refund、reject、completion、disputes）

**演进路线**：

- **阶段 1（已完成）**：文件命名统一，按前缀分组
- **阶段 2（中期，添加新链或支付模式时触发）**：提取 `PaymentService`
  - 定义窄接口 `PaymentNodeDeps`（DB、Multiwallet、EventBus、RelayConfig）
  - 将 `payment_*` 文件中的方法迁移到 `PaymentService` 结构体
  - `MobazhaNode` 持有 `*PaymentService` 并委托调用
- **阶段 3（长期，文件数超 100 或团队扩大时触发）**：按域提取更多 Service
  - `OrderService`（订单生命周期）、`SocialService`（聊天/关注/评价）、`StoreService`（商品/图片）
  - 每个 Service 通过窄接口依赖 Node 基础设施

**触发条件**：
- 支付域方法超过 80 个 → 启动阶段 2
- `internal/core/` 文件超过 100 个 → 启动阶段 3
- 多人并行开发同一域 → 优先提取该域的 Service

**原则**：渐进式重构，每次只提取一个域，确保编译通过 + 测试覆盖，避免大爆炸式重写。
