# ExecutionAttempt

副作用派发前后的执行尝试。

写入／生成：P2 RunService。存储：execution_attempt。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/ExecutionAttempt`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| run_id | string | 是 | UUID 标识 |
| attempt_no | integer | 是 |  |
| fencing_token | integer | 是 |  |
| dispatch_state | enum | 是 | ；PREPARED, DISPATCHED, FINISHED |
| prepared_at | string | 是 | UTC RFC3339 时间 |
| dispatched_at | string / null | 是 | 见类型定义 |
| finished_at | string / null | 是 | 见类型定义 |
| executor_id | string | 是 |  |
| command_hash | string | 是 | SHA-256 十六进制摘要 |
| last_error | string / null | 是 | 见类型定义 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

DISPATCHED 在调用外部服务前保存，因此崩溃后可能是结果未知。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
