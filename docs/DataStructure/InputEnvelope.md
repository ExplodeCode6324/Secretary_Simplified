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

## 约束与使用

输入结构验证通过后先归档；服务分配 intent_id，模型不能指定主体或输入时间。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
