# InputEnvelope

已认证的输入信封。

写入／生成：P1 ConversationService。存储：input_turn / request_receipt。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/InputEnvelope`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| request_id | string | 是 | UUID 标识 |
| session_id | string | 是 | UUID 标识 |
| principal_id | string | 是 | 由认证入口写入 |
| origin | enum | 是 | ；MASTER_CLI, SOURCE, SYSTEM |
| received_at | string | 是 | UTC RFC3339 时间 |
| text | string | 是 | 本轮原始输入 |
| attachment_refs | array&lt;ObjectRef&gt; | 是 |  |
| data_class | enum | 是 | ；SYNTHETIC, PERSONAL, SENSITIVE, SECRET |
| extensions | object | 是 | 有命名空间的可选扩展 |
| answer_to_question_id | string | 否 | 原 session 的待答问题 UUID；显式 null 拒绝 |

## 约束与使用

输入结构及显式回答目标校验通过后归档；服务分配 intent_id，模型不能指定主体或输入时间。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。


## 实施设计修订 D11：待答问题生命周期

可选 `answer_to_question_id` 为非 null UUID；省略表示普通输入。原始 JSON 显式 null 拒绝，不可由 DTO omitempty 吞掉。该指针参与请求语义 hash，相同 request_id 改指针为幂等冲突。只接受已认证 MASTER_CLI、同 principal、原 session 的现存未解问题；不猜测自由指代。受理前检查早于原话对象归档，事务受理和最终成功提交再次检查；受理后竞争允许保留原话审计，业务动作和问题变更必须全部回滚。

裁决：`review/D11-pending-question-deepseek.response.md`；原提案：`review/A09-pending-question-proposal.md`。无 DDL 变更。

## Issue #2 当前绑定与可见性

公共输入可省略 session_id，由适配器在严格内部 DTO 校验前绑定后端唯一会话；内部 DTO 的 session_id 仍必需。显式 foreign/null/非法值拒绝。旧精确 request 回放保留原语义和绑定，不重写历史。默认 PERSONAL；TUI 不上传历史。
