# ConversationState

有界会话状态。

写入／生成：P1 会话整理。存储：conversation_session。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/ConversationState`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | 会话 ID |
| revision | integer | 是 |  |
| through_sequence | integer | 是 | 摘要覆盖水位 |
| summary_from_sequence | integer | 是 |  |
| summary | string | 是 |  |
| recent_event_ids | array&lt;string&gt; | 是 |  |
| focus_entity_ids | array&lt;string&gt; | 是 |  |
| pending_questions | array&lt;object&gt; | 是 |  |
| commitment_item_ids | array&lt;string&gt; | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

摘要水位不得越过未覆盖原话；承诺引用独立 Item。摘要不直接决定业务状态。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
