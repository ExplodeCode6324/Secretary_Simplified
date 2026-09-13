# Observation

带时效和证据的观测。

写入／生成：P1 IngestService。存储：observation。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/Observation`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| source_record_id | string / null | 是 | 见类型定义 |
| entity_id | string | 是 | UUID 标识 |
| kind | enum | 是 | 观测类型注册名；device.availability, source.health |
| value | object / null | 是 | 见类型定义 |
| observed_at | string | 是 | UTC RFC3339 时间 |
| valid_until | string / null | 是 | 见类型定义 |
| certainty | enum | 是 | ；OBSERVED, INFERRED, UNKNOWN |
| evidence | array&lt;EvidenceRef&gt; | 是 |  |
| data_class | enum | 是 | ；SYNTHETIC, PERSONAL, SENSITIVE, SECRET |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

value 由观测类型 Schema 约束；UNKNOWN 时为 null。时间过期不能改写原始 observed_at。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
