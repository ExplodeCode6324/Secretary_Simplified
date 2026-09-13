# ConsumerCursor

事件消费者水位。

写入／生成：消费者事务服务。存储：consumer_cursor。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/ConsumerCursor`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| consumer_id | string | 是 |  |
| last_seq | integer | 是 |  |
| revision | integer | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

cursor 与业务处理同事务推进。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
