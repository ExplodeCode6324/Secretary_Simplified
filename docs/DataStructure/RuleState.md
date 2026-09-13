# RuleState

事件规则去重与冷却状态。

写入／生成：共享规则服务。存储：rule_state。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/RuleState`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| rule_id | string | 是 |  |
| root_id | string | 是 | UUID 标识 |
| cursor_seq | integer | 是 |  |
| next_allowed_at | string / null | 是 | 见类型定义 |
| no_progress_count | integer | 是 |  |
| last_effect_hash | string / null | 是 | 见类型定义 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

根预算与冷却同时检查，重启不清零。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
