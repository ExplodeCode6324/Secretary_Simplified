# Task

带完成标准的任务实例。

写入／生成：共享 TaskService；P1 验收。存储：task。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/Task`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| revision | integer | 是 |  |
| root_id | string | 是 | UUID 标识 |
| item_id | string / null | 是 | 见类型定义 |
| job_id | string / null | 是 | 见类型定义 |
| occurrence_key | string / null | 是 | 见类型定义 |
| goal | string | 是 |  |
| state | enum | 是 | ；PENDING, RUNNING, VERIFYING, WAITING, NEEDS_ATTENTION, SUCCEEDED, FAILED, CANCELLED |
| criteria | array&lt;Criterion&gt; | 是 |  |
| criterion_hash | string | 是 | SHA-256 十六进制摘要 |
| cancel_generation | integer | 是 |  |
| deadline_at | string / null | 是 | 见类型定义 |
| created_at | string | 是 | UTC RFC3339 时间 |
| updated_at | string | 是 | UTC RFC3339 时间 |
| extensions | object | 是 | 有命名空间的可选扩展 |
| parent_root_id | string / null | 是 | 见类型定义 |

## 约束与使用

criteria 至少 1 项。周期任务每 occurrence 一个实例，计划 payload 保存任务模板。等待不得清空预算。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。

## 实施过程中发现的缺陷 D02：周期通知的实例身份

2026-09-14 实施发现：周期计划若复用模板 notification_key，notification 全局唯一约束会使后续 occurrence 复用旧通知及验收证据。经本地 Ayanami 同意，Scheduler 在每个 occurrence 的原子物化事务内，将 notify.local 模板键实例化为 `occ:v1:<base64url(template_key)>:<base64url(occurrence_key)>`（无填充 Base64 URL 编码）。同一 occurrence 的重试/对账复用实例键，不包含 attempt、job_revision 或当前墙钟。模板键保持不变。

实例 Command.arguments.notification_key 与该实例 notification_recorded criterion.expected.notification_key 同步绑定后，才计算并冻结 criterion_hash。不得更改已存在 Task 的完成条件；修订模板仅作用于以后尚未物化的 occurrence。通知验收须同时核对实例键、当前 run 的持久通知及真实执行证据，不能用另一个 occurrence 的通知完成本次 Task。即时独立任务仍按其已接受的稳定通知键处理。

修订同时适用于时间、事件和手动 occurrence；跳过的 occurrence 不产生通知。复核依据：[Ayanami D02 讨论结论](../../review/D02-notification-occurrence.response.md).
