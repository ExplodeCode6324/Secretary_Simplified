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
