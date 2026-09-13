# LiveWorldStateInput

送给模型的实时状态只读投影。

写入／生成：P1 ContextBuilder。存储：读取现有表，不单独落世界状态表。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/LiveWorldStateInput`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| snapshot_seq | integer | 是 |  |
| as_of | string | 是 | UTC RFC3339 时间 |
| items | array&lt;Item&gt; | 是 |  |
| task_refs | array&lt;VersionRef&gt; | 是 |  |
| observations | array&lt;Observation&gt; | 是 |  |
| source_states | array&lt;SourceState&gt; | 是 |  |
| freshness | array&lt;object&gt; | 是 |  |
| omitted_count | integer | 是 |  |
| missing_reasons | array&lt;string&gt; | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

CURRENT 只表示未过期；不表示观测必然真实。task_refs 的状态正文由 Context.tasks 提供。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
