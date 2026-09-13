# ChangeEvent

状态变更与恢复消费事件。

写入／生成：各权威服务。存储：change_event / consumer_cursor / rule_state。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/ChangeEvent`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| seq | integer | 是 |  |
| root_id | string | 是 | UUID 标识 |
| entity_type | string | 是 |  |
| entity_id | string | 是 | UUID 标识 |
| entity_revision | integer | 是 |  |
| event_type | string | 是 |  |
| causation_id | string / null | 是 | 见类型定义 |
| origin | string | 是 |  |
| created_at | string | 是 | UTC RFC3339 时间 |
| change | object | 是 | 按 event_type 的增量 Schema 校验 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

事件与状态同事务追加，seq 全局递增；重复消费不产生重复业务效果。历史回放禁止真实派发。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。


## D06 实施缺陷修正（2026-09-14）

D06 日历跳过使用 scheduled_job.skipped 事件，before/after 为同 revision 的完整 ScheduledJob，after 含推进后的 next_due_at。origin=scheduler.calendar，runtime.calendar_skip 描述不存在的本地时刻，created_at 是发现/提交审计时间。稳定 ID 绑定 job、local_date 与规范规则身份；不伪造不存在时刻对应的 UTC 执行。

复核依据：`review/D06-calendar-skip.response.md`。
