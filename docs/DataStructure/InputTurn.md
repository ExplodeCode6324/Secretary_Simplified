# InputTurn

可恢复输入轮次。

写入／生成：P1 ConversationService。存储：input_turn。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/InputTurn`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| session_id | string | 是 | UUID 标识 |
| principal_id | string | 是 |  |
| request_id | string | 是 | UUID 标识 |
| intent_id | string | 是 | UUID 标识 |
| state | enum | 是 | ；PENDING, PROCESSING, COMMITTED, FAILED |
| input | InputEnvelope | 是 | 见类型定义 |
| reply | object / null | 是 | 见类型定义 |
| committed_operation_keys | array&lt;string&gt; | 是 |  |
| processing_owner | string / null | 是 | 见类型定义 |
| lease_until | string / null | 是 | 见类型定义 |
| updated_at | string | 是 | UTC RFC3339 时间 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

PROCESSING 持有有界租约；恢复时先查提交回执而非重新创造业务意图。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。


## 实施设计修订 D11：待答问题生命周期

模型 Decision.reply.questions 的两字段 proposal 与最终 InputTurn.reply.questions 是不同形状。最终 reply.questions 最多 3 个既有 PendingQuestion 五字段对象；成功显式回答可带程序生成 answered_question_id。decision_record 保存原始模型 proposal，不用最终回复替换。最终文本、ASSISTANT event、InputTurn、request receipt、pending_questions 和业务动作同事务提交；失败 fallback 不改变问题生命周期。

裁决：`review/D11-pending-question-deepseek.response.md`；原提案：`review/A09-pending-question-proposal.md`。无 DDL 变更。


### D12 分类传播

最终输出extension与输入原始data_class分开；原话标签不因高分类Context而重写。 使用 [Common.md](Common.md) 的 security.classification 契约；缺失旧派生标记返回 OUTPUT_CLASS_UNKNOWN，不自动回填。

## Issue #2 当前绑定与可见性

当前主会话 turn 关联 authority_turn 的持久受理序号与冻结轮次状态。受理原话追加不推进认知 revision；单消费者只运行队头。客户端通过原 request_id/turn_id 查询与恢复，不以超时创建新 turn。
