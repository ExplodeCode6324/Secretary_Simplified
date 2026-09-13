# JobRun

一次触发的持久运行状态。

写入／生成：P2 RunService。存储：job_run / execution_attempt。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/JobRun`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| task_id | string | 是 | UUID 标识 |
| job_id | string / null | 是 | 见类型定义 |
| job_revision | integer / null | 是 | 见类型定义 |
| occurrence_key | string | 是 |  |
| scheduled_for | string | 是 | UTC RFC3339 时间 |
| state | enum | 是 | ；QUEUED, CLAIMED, RUNNING, SUCCEEDED, FAILED, RESULT_UNKNOWN, CANCELLED, SKIPPED |
| attempt_no | integer | 是 |  |
| fencing_token | integer | 是 |  |
| lease_owner | string / null | 是 | 见类型定义 |
| lease_until | string / null | 是 | 见类型定义 |
| external_idempotency_key | string | 是 |  |
| command | Command | 是 | 见类型定义 |
| updated_at | string | 是 | UTC RFC3339 时间 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

即时任务 job_id/job_revision 为 null；attempt_no 从 1 开始执行，排队时为 0。重试保持 run_id 和外部幂等键。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。


## D06 实施缺陷修正（2026-09-14）

D06：不存在的本地时刻没有合法 scheduled_for UTC，因此不为 DST gap 伪造 JobRun。其遗漏事实通过原子 scheduled_job.skipped 审计保存；真正的 overlap/misfire occurrence 仍可生成 SKIPPED JobRun，二者不得混淆。

复核依据：`review/D06-calendar-skip.response.md`。
