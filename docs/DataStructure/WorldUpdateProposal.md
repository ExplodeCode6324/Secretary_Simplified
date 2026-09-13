# WorldUpdateProposal

提交给专用执行器的长期事实修改提案。

写入／生成：P1 提案服务 / world.update。存储：world_proposal。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/WorldUpdateProposal`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| request_id | string | 是 | UUID 标识 |
| entity_id | string | 是 | UUID 标识 |
| predicate | enum | 是 | 事实类型注册名；master.preference, project.background, entity.relation |
| operation | enum | 是 | ；ASSERT, CORRECT, RETRACT |
| fact_id | string | 是 | UUID 标识 |
| expected_revision | integer | 是 | 新事实为 0 |
| value | object / null | 是 | 见类型定义 |
| evidence | array&lt;EvidenceRef&gt; | 是 |  |
| basis | enum | 是 | ；MASTER_EXPLICIT, SOURCE_RULE, INFERENCE |
| policy_revision | integer | 是 |  |
| reason | string | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

此结构没有授权布尔值或执行 token。RETRACT 的 value 必须 null，ASSERT/CORRECT 按 predicate Schema 校验；执行时取得程序许可。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
