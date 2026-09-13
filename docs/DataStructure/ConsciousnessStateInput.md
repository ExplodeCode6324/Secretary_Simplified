# ConsciousnessStateInput

每 24 小时认知整理的完整输入。

写入／生成：P1 memory.refresh。存储：随快照产物保存引用。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/ConsciousnessStateInput`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| slot | integer | 是 |  |
| snapshot_seq | integer | 是 |  |
| as_of | string | 是 | UTC RFC3339 时间 |
| changes_from_seq | integer | 是 |  |
| previous_snapshot_id | string / null | 是 | 见类型定义 |
| world | WorldModelInput | 是 | 见类型定义 |
| live | LiveWorldStateInput | 是 | 见类型定义 |
| open_task_refs | array&lt;VersionRef&gt; | 是 |  |
| recent_changes | array&lt;ChangeEvent&gt; | 是 |  |
| missed_slots | integer | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |
| previous_snapshot | ConsciousnessState / null | 是 | 见类型定义 |
| tasks | array&lt;Task&gt; | 是 |  |

## 约束与使用

不只输入旧摘要；changes_from_seq 应覆盖上次有效快照之后的变化，停机时允许超过 24 小时。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
