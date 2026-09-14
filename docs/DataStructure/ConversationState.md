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


## 实施设计修订 D11：待答问题生命周期

pending_questions 的正式 ID 字段保持 `id`（MemoryPolicy 所称 question_id）。既有五字段不变，最多 20 槽。程序按 namespace+principal+session+request+proposal_index 派生稳定 UUID，created_sequence 使用真正 ASSISTANT ConversationEvent.sequence。新问题 resolved=false；显式回答成功提交才改 true，仅代表会话回答事实，不代表答案真实、事项 DONE 或外部执行完成。容量不足只回收最旧 resolved，不丢未解问题；满 20 个 unresolved 时拒绝新登记。历史原话及回执保留。生命周期变化递增 revision，不移动 through_sequence/summary_from_sequence；旧摘要 CAS 必须失败，摘要不得复活或擅自解决问题。

裁决：`review/D11-pending-question-deepseek.response.md`；原提案：`review/A09-pending-question-proposal.md`。无 DDL 变更。


### D12 分类传播

聚合摘要和全部问题文本生成分类，只升不降；真正空状态例外，pending_questions 不加字段。 使用 [Common.md](Common.md) 的 security.classification 契约；缺失旧派生标记返回 OUTPUT_CLASS_UNKNOWN，不自动回填。
