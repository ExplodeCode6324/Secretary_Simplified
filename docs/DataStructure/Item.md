# Item

业务事项与承诺。

写入／生成：P1 ItemService。存储：item / item_dependency。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/Item`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| revision | integer | 是 |  |
| domain | string | 是 | work / project / school / life 或注册扩展 |
| kind | enum | 是 | ；TASK, COMMITMENT, EVENT, NOTE |
| title | string | 是 |  |
| status | enum | 是 | ；OPEN, IN_PROGRESS, BLOCKED, DONE, CANCELLED |
| priority | integer | 是 | 0 最高，3 最低 |
| due_at | string / null | 是 | 见类型定义 |
| timezone | string | 是 | IANA 时区 |
| time_state | enum | 是 | ；CONFIRMED, UNKNOWN, CONTESTED, NOT_APPLICABLE |
| parent_id | string / null | 是 | 见类型定义 |
| dependency_ids | array&lt;string&gt; | 是 |  |
| evidence | array&lt;EvidenceRef&gt; | 是 |  |
| created_at | string | 是 | UTC RFC3339 时间 |
| updated_at | string | 是 | UTC RFC3339 时间 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

priority 限 0—3。CONTESTED 日期在证据与候选中保留，due_at 为 null；CONFIRMED 才可作为调度日期；修改日期不自动授权新的提醒计划。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。


### D12 分类传播

CREATE join 请求与来源；UPDATE join旧体，不重写 EvidenceRef 原字节。 使用 [Common.md](Common.md) 的 security.classification 契约；缺失旧派生标记返回 OUTPUT_CLASS_UNKNOWN，不自动回填。
