# WorldModelInput

送给模型的长期事实只读投影。

写入／生成：P1 ContextBuilder。存储：嵌入 context_manifest。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/WorldModelInput`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| snapshot_seq | integer | 是 |  |
| as_of | string | 是 | UTC RFC3339 时间 |
| facts | array&lt;WorldFact&gt; | 是 |  |
| unresolved_conflict_ids | array&lt;string&gt; | 是 |  |
| omitted_count | integer | 是 |  |
| missing_reasons | array&lt;string&gt; | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

默认只选当前 ACTIVE/CONTESTED；CANDIDATE 只能明确标作推断材料。历史值不冒充当前值。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
