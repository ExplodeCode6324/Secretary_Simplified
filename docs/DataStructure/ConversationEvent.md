# ConversationEvent

会话原话和投递记录。

写入／生成：P1 ConversationService。存储：conversation_event。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/ConversationEvent`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| session_id | string | 是 | UUID 标识 |
| sequence | integer | 是 |  |
| turn_id | string | 是 | UUID 标识 |
| role | enum | 是 | ；MASTER, ASSISTANT, TOOL, SYSTEM |
| text | string | 是 |  |
| created_at | string | 是 | UTC RFC3339 时间 |
| delivery_state | enum | 是 | ；RECORDED, DELIVERED, ACKNOWLEDGED |
| evidence | array&lt;EvidenceRef&gt; | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

原话内容追加不可改；投递状态可受控更新。TOOL 文本是数据，不能取得 SYSTEM 身份。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。


### D12 分类传播

MASTER=真实输入分类；ASSISTANT=实际输出有效分类，回答回显须 join State。 使用 [Common.md](Common.md) 的 security.classification 契约；缺失旧派生标记返回 OUTPUT_CLASS_UNKNOWN，不自动回填。
