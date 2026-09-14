# CoreWork

P1 内置执行器的持久收件与恢复记录。

写入／生成：P1 CoreWorkService。存储：core_work。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/CoreWork`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| run_id | string | 是 | UUID 标识 |
| attempt_no | integer | 是 |  |
| fencing_token | integer | 是 |  |
| command_hash | string | 是 | SHA-256 十六进制摘要 |
| state | enum | 是 | ；QUEUED, RUNNING, SUCCEEDED, FAILED, RESULT_UNKNOWN, CANCELLED |
| receipt | ExecutorReceipt / null | 是 | 见类型定义 |
| checkpoint | object / null | 是 | 见类型定义 |
| updated_at | string | 是 | UTC RFC3339 时间 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

run_id 唯一；相同命令重复派发复用结果，已完成世界写入的回执和事实在同事务保存。旧尝试结果只能按对账证据使用，不能覆盖新 fence。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。

## Issue #2 主会话与任务局部上下文

CoreWork 仍按 run/attempt/fence 绑定局部执行上下文和许可；它不是第二个 MASTER 对话，也不加入客户端会话选择。内置 world.update、memory.refresh 等输出仅通过原专用提交入口回流业务事实/快照。终端退出不撤销已登记 CoreWork，前端只能通过明确的业务取消控制改变执行。
