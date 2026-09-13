# WorldFact

版本化长期事实。

写入／生成：仅 WorldCommitService。存储：world_fact_version / world_fact_head。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/WorldFact`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| revision | integer | 是 |  |
| entity_id | string | 是 | UUID 标识 |
| predicate | enum | 是 | ；master.preference, project.background, entity.relation |
| value | object / null | 是 | 见类型定义 |
| status | enum | 是 | ；CANDIDATE, ACTIVE, CONTESTED, RETRACTED, SUPERSEDED |
| valid_from | string / null | 是 | 见类型定义 |
| valid_until | string / null | 是 | 见类型定义 |
| evidence | array&lt;EvidenceRef&gt; | 是 |  |
| proposal_id | string | 是 | UUID 标识 |
| acceptance_rule | string | 是 |  |
| policy_revision | integer | 是 |  |
| conflict_group | string / null | 是 | 见类型定义 |
| supersedes | VersionRef / null | 是 | 见类型定义 |
| created_at | string | 是 | UTC RFC3339 时间 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

事实版本只追加；ACTIVE 必须有可验证证据与准入规则。scope 和 policy 决定可写范围，不用置信度替代许可。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
