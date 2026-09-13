# ConversationSummaryDraft

会话摘要模型输出内容。

写入／生成：P1 会话整理。存储：合入 conversation_session。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/ConversationSummaryDraft`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| summary | string | 是 |  |
| focus_entity_ids | array&lt;string&gt; | 是 |  |
| pending_question_ids | array&lt;string&gt; | 是 |  |
| commitment_item_ids | array&lt;string&gt; | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

所有引用必须存在于输入范围；不能创造已确认承诺。程序执行摘要水位 CAS。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
