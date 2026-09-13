# DecisionEnvelope

模型完整输出，回复与调度提案分离。

写入／生成：P1 DecisionValidator。存储：decision_record。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/DecisionEnvelope`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| context_id | string | 是 | UUID 标识 |
| reply | object | 是 | 见类型定义 |
| actions | array&lt;ActionProposal&gt; | 是 |  |
| controls | array&lt;Control&gt; | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

服务在外部绑定 intent_id、身份与授权，模型不可赋值。任何一项非法则不提交动作组，失败回复由程序生成。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
