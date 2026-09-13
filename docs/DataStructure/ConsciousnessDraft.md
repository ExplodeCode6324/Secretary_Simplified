# ConsciousnessDraft

意识整理模型输出内容。

写入／生成：P1 memory.refresh。存储：合入 consciousness_snapshot。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/ConsciousnessDraft`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| focal_goals | array&lt;FocusEntry&gt; | 是 |  |
| priority_items | array&lt;FocusEntry&gt; | 是 |  |
| open_loops | array&lt;FocusEntry&gt; | 是 |  |
| important_changes | array&lt;FocusEntry&gt; | 是 |  |
| uncertainties | array&lt;string&gt; | 是 |  |
| brief_summary | string | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

仅内容字段来自模型，身份、时间、水位、slot 和哈希由程序填写。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
