# ExecutionPermit

绑定执行尝试的短期程序许可。

写入／生成：P2 PolicyService；WorldCommitService 消费。存储：execution_permit。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/ExecutionPermit`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| run_id | string | 是 | UUID 标识 |
| grant_id | string | 是 | UUID 标识 |
| grant_revision | integer | 是 |  |
| fencing_token | integer | 是 |  |
| cancel_generation | integer | 是 |  |
| capability | string | 是 |  |
| proposal_hash | string / null | 是 | 见类型定义 |
| expires_at | string | 是 | UTC RFC3339 时间 |
| consumed_at | string / null | 是 | 见类型定义 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

传输凭证作为独立认证元数据，不进入模型生成结构；许可与事实提交绑定。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
