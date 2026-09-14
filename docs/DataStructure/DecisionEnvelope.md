# DecisionEnvelope

模型完整输出，回复与调度提案分离。

写入／生成：P1 DecisionValidator。存储：decision_record。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/DecisionEnvelope`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| context_id | string | 是 | UUID 标识 |
| reply | object | 是 | 见类型定义 |
| actions | array&lt;ActionProposal&gt; | 是 |  |
| controls | array&lt;Control&gt; | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

服务在外部绑定 intent_id、身份与授权，模型不可赋值。任何一项非法则不提交动作组，失败回复由程序生成。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。


## 实施设计修订 D11：待答问题生命周期

模型 `reply.questions?` 使用 QuestionProposals：最多 3 个 `{text,item_id}`，text 为 1–512 字符，item_id 为当前 Context/readset 已有 Item UUID 或 null。不得带 id、created_sequence、resolved 或其他字段；不关联本 Decision 尚未创建的事项。questions 数组任一 proposal 校验失败（重复、item_id 引用/revision 无效、超 512 字符、超 3 条）⇒ 整个 Decision 校验失败：零问题登记、零 Item/Task/World 副作用、不静默丢弃、不语义合并。proposal_index 固定为提交数组内下标。非 MASTER_CLI 携 questions（含空数组）明确拒绝；登记必须经过已认证主体及授权检查。

裁决：`review/D11-pending-question-deepseek.response.md`；原提案：`review/A09-pending-question-proposal.md`。无 DDL 变更。
