# WaitSubscription

持久等待和原子唤醒条件。

写入／生成：P1 TaskService。存储：wait_subscription。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/WaitSubscription`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| task_id | string | 是 | UUID 标识 |
| generation | integer | 是 |  |
| event_type | string | 是 |  |
| entity_id | string | 是 | UUID 标识 |
| expected_state | string | 是 |  |
| cursor_seq | integer | 是 |  |
| deadline_at | string | 是 | UTC RFC3339 时间 |
| state | enum | 是 | ；ARMED, WOKEN, EXPIRED, CANCELLED |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

登记时先在事务内检查现状，旧 generation 无效。事件类型和 expected_state 均由注册表验证。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
