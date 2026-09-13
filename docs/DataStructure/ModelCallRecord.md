# ModelCallRecord

模型调用诊断记录。

写入／生成：P1 model adapter。存储：诊断对象。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/ModelCallRecord`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| call_id | string | 是 | UUID 标识 |
| root_id | string | 是 | UUID 标识 |
| context_id | string | 是 | UUID 标识 |
| provider_profile | string | 是 |  |
| model_id | string | 是 |  |
| request_hash | string | 是 | SHA-256 十六进制摘要 |
| output_ref | ObjectRef / null | 是 | 见类型定义 |
| started_at | string | 是 | UTC RFC3339 时间 |
| finished_at | string / null | 是 | 见类型定义 |
| input_tokens | integer | 是 |  |
| output_tokens | integer | 是 |  |
| count_mode | enum | 是 | ；TOKENIZER, CONSERVATIVE_ESTIMATE, PROVIDER |
| status | enum | 是 | ；RUNNING, SUCCEEDED, FAILED, CANCELLED |
| error_code | string / null | 是 | 见类型定义 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

服务商未报告 token 时记录估计方式；预算应先预留再结算，崩溃未结算按预留上限计费。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
