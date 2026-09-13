# RootBudget

跨子任务共享的持久预算。

写入／生成：PolicyService。存储：root_budget。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/RootBudget`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| root_id | string | 是 | UUID 标识 |
| revision | integer | 是 |  |
| model_calls_used | integer | 是 |  |
| retrievals_used | integer | 是 |  |
| actions_used | integer | 是 |  |
| replans_used | integer | 是 |  |
| output_tokens_used | integer | 是 |  |
| active_ms_used | integer | 是 |  |
| limits | object | 是 | 见类型定义 |
| cooldown_until | string / null | 是 | 见类型定义 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

调度一次周期 occurrence 可分配新的根预算，子任务沿用同一根；已登记的计划根只负责触发和限流，不能导致周期第 9 次必然耗尽旧执行预算。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
