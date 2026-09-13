# ExecutorReceipt

执行器返回的结构化状态与证据。

写入／生成：P2 RunService。存储：executor_receipt。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/ExecutorReceipt`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| run_id | string | 是 | UUID 标识 |
| attempt_no | integer | 是 |  |
| fencing_token | integer | 是 |  |
| receipt_key | string | 是 |  |
| status | enum | 是 | ；RUNNING, SUCCEEDED, FAILED, RESULT_UNKNOWN, CANCELLED |
| external_operation_id | string / null | 是 | 见类型定义 |
| artifacts | array&lt;ObjectRef&gt; | 是 |  |
| evidence | array&lt;EvidenceRef&gt; | 是 |  |
| error_code | string / null | 是 | 见类型定义 |
| effect_observed | boolean | 是 |  |
| received_at | string | 是 | UTC RFC3339 时间 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

status=SUCCEEDED 仍需业务验收。相同 receipt_key 不同内容是冲突；旧 fencing 回执不覆盖新状态。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
