# 渠道账务权益框架

## 目标

渠道账务不再只表示一个“余额数字”，而是统一表示为一组上游权益项，用于支撑：

- 周 / 日 / 月 / 总额度
- 套餐或充值有效期
- 周期重置时间
- 到期提醒
- 低余额提醒
- 未来自动续费 / 自动升级 / 自动充值

## 标准权益项

当前统一落在 `channel_billing_snapshot_items`：

- `resource_type`
  - `quota`
  - `balance`
  - `credit`
  - `plan`
- `quota_type`
  - `daily`
  - `weekly`
  - `monthly`
  - `total`
  - `custom`
- `quota_label`
- `limit_amount`
- `used_amount`
- `remaining_amount`
- `currency`
- `reset_at`
- `expires_at`
- `status`
  - `active`
  - `low`
  - `depleted`
  - `expired`
- `source_ref`
- `metadata`

`amount` 保留为展示兼容字段，但新的采集和告警都基于 `limit_amount / used_amount / remaining_amount / reset_at / expires_at`。

## 快照模型

- `channel_billing_snapshots`
  - 表示一次采集事件
- `channel_billing_snapshot_items`
  - 表示本次采集得到的标准权益项

当前支持：

- `source_type=api`
- `source_type=manual`

## 告警事件

新增表：`channel_billing_alert_events`

用于记录每天去重后的提醒事件，避免同一渠道同一权益项在一天内重复通知。

当前事件类型：

- `expiring_soon`
- `low_remaining`

去重键：

- `channel_id`
- `event_type`
- `alert_key`
- `notify_date`

## 规则

当前默认规则来自 `channel_billing_profiles.notify_config`，为空时使用默认值：

- `expiry_notice_days = 7`
- `low_remaining_ratio = 0.2`

### 到期提醒

当权益项满足：

- `expires_at > now`
- `expires_at - now <= 7 天`

则每天提醒一次。

### 低余额提醒

当权益项满足：

- `limit_amount > 0`
- `remaining_amount > 0`
- `remaining_amount / limit_amount <= 0.2`

且 `quota_type` 属于：

- `daily`
- `weekly`
- `monthly`
- `total`

则每天提醒一次。

## 自动禁用

自动禁用不再绑定旧的“主余额数字 <= 0”。

当前实现改为：

- 以标准权益项里的 `total` 项为准
- 当 `total.remaining_amount <= 0` 时自动禁用渠道

## 采集器

Router 支持通过 `billing_service.base_url` 接入独立 Billing 服务。

- 已配置 Billing 服务时，自动刷新优先调用 `/api/v1/internal/billing:query`
- 请求使用新版 `adapter` 字段，不再使用旧 `provider` 字段
- Billing 服务返回的标准 `items` 会转换并保存到 `channel_billing_snapshot_items`
- Router 仍负责快照持久化、失败快照、告警、自动禁用和采购记录

配置边界：

- 渠道 `config.api_base_url` 只用于模型请求转发
- 账务 API 地址统一使用 Billing Profile 的 `billing_api_base_url`
- 刷新时 Router 使用渠道基础配置里的 API Key 作为 Billing 请求的 `credential`
- `billing_mode` 直接保存 Billing 服务返回的 adapter 名，不再使用 Router 内置模式名

当前已接入：

- `aixhan`
  - `cdk` 是 Aixhan 渠道的卡密 / 凭据字段
  - `usage/stats` 输出日 / 周 / 总额度
  - `card-info` 输出套餐有效期与套餐元信息
  - 日 / 周额度带 `reset_at`
  - 套餐到期提醒基于 `plan` 类型权益项
- `openai`
  - 输出总额度
  - 带套餐到期时间
- 其他渠道
  - 由 Billing 服务 adapter 列表声明，Router 不再内置账务接口

后续新增上游时，要求直接输出标准权益项，不再新增临时余额模型。

## CDK 字段映射

下沉到 Billing 服务后，Aixhan 由 `aixhan` adapter 采集并按如下方式落库：

- `usage/stats`
  - `remaining / consumed / resetAt` -> `daily`
  - `weeklyRemaining / weeklyConsumed / weeklyLimit / weeklyResetAt` -> `weekly`
  - `totalRemaining / totalConsumed / totalLimit` -> `total`
- `card-info`
  - `expiresAt / expireTime` -> `plan.expires_at`
  - `productName / categoryName / categoryPool / billingMode / limitConcurrentSessions ...`
    -> `plan.metadata`

说明：

- `daily.limit_amount` 优先使用 `card-info.dailyQuota`，没有时退回 `consumed + remaining`
- `CDK` 请求地址在快照里只记录脱敏后的 `request_url`，不落明文 `cdk`

## 后续扩展

后续如果接入自动续费 / 升级 / 充值，只允许在当前标准权益模型之上扩展：

- 增加新的 collector
- 增加新的 alert rule
- 增加新的 action executor

不再新增第二套账务数据结构。
