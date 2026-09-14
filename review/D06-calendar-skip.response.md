> 模型限定：本报告来自 Ayanami 使用 `gpt-5.6-luna` 的历史复核。Master 最新指定必须由 `deepseek-v4.1-flash` 复核，因此本报告仅为 model-qualified 历史证据，不能充当最终复核裁决；原问题、结论与测试结果保留，待 DeepSeek 重新检视。

D06 复核结论（只读；未修改文件，未读取 resources，未连接 ELIZA）

1. DST gap 缺陷成立  
证据：
- `src/store/runtime_schedule.go:71-84`：不存在的 local time 被跳过并寻找下一次 occurrence，没有审计记录。
- `src/store/runtime_schedule.go:223-287`：当前逻辑只会生成合法 UTC `JobRun`，无法表达 DST gap。
- `docs/ExecutionProtocol.md:28`、`docs/DataStructure/JobRun.md:18-20`、`contracts.schema.json:4663-4666`：`JobRun.scheduled_for` 必须是 UTC RFC3339，不能伪造不存在时刻。
结论：同意采用 calendar-skip event，不创建伪造的 `JobRun`。

2. 事件名称  
提议的 `schedule.skipped`：反对直接采用。`Schedule` 是 `ScheduledJob` 内嵌结构，现有 TypeRegistry 使用实体前缀，例如 `scheduled_job.updated`。  
明确替代：使用：

`scheduled_job.skipped`

映射：

`entity_type=scheduled_job` → `ScheduledJob`

`contract.ValidateEvent` 当前仅白名单 `scheduled_job.updated`（`src/contract/contract.go:143-169`），必须同步登记新事件。

3. before/after 与同 revision  
结论：同 revision 可以，但必须单独声明为“非业务变更的运行审计事件”。

必要语义：
- `before`：事务内读取的完整 ScheduledJob；
- `after`：推进 `next_due_at` 后的完整 ScheduledJob；
- 两者 `revision` 相同，`entity_revision` 等于该 revision；
- `after` 只能在运行字段上变化，例如 `next_due_at`、必要的 `updated_at`；不得改变 schedule、command、task_template 等业务配置；
- 如果所谓“同revision before/after”是指两份 DTO 必须完全相同，则反对：next_due 推进后无法重建调度游标。

该例外必须写入 `TypeRegistry.md`、`ChangeEvent.md`、`ScheduledJob.md`，避免被消费者误当作普通 `scheduled_job.updated`。

4. calendar_skip 扩展  
方案同意，但当前实现会拒绝该扩展：

- `contracts.schema.json:5123-5131` 只提供通用 namespaced object；
- `src/contract/contract.go:174-215` 对扩展名称和结构执行程序白名单，尚无 `runtime.calendar_skip`。

必须注册严格结构：

```json
{
  "local_date": "YYYY-MM-DD",
  "timezone": "IANA timezone",
  "local_time": "HH:MM",
  "reason": "DST_GAP"
}
```

禁止额外字段；`local_date`、`local_time`、IANA timezone 均需程序校验。`created_at` 使用扫描时间，不能使用 local time 转出的伪 UTC。

5. 去重与事务  
方案同意，但“稳定 ID/UNIQUE”应具体实现为：

`contract.DeriveID("scheduled_job.calendar_skip.v1:" + job_id + ":" + local_date)`

使用现有 `change_event.id UNIQUE`，不新增 DDL。不能只依赖普通 INSERT 报唯一冲突，否则会回滚 `next_due_at` 推进。

必要条件：
- 事件插入、`next_due_at` 推进和计划持久化必须同一事务；
- 已存在相同确定性 ID 时，校验 payload 一致并视为幂等成功；
- `job_id + local_date` 的“一天最多一条 gap 审计”语义必须明确。若计划修订后同一天需要分别审计不同 timezone/local_time，则稳定 ID 必须扩展为包含 canonical rule identity。

6. 防止事件自触发  
必须单独声明：
- `scheduled_job.skipped` 是 audit-only，不等价于 `scheduled_job.updated`；
- 不得由该事件再次推进同一计划或生成同一计划的自触发 occurrence；
- 若允许其他计划显式订阅该事件，必须遵守 `ExecutionProtocol.md:82` 的 root/causation/cooldown 规则，并禁止同一 job 订阅自身的 calendar-skip 事件；
- `origin` 建议固定为 `scheduler.calendar`。

7. 最小文档增补  
需要修改并在 README 登记路径：

- `docs/ExecutionProtocol.md`：增加 DST gap 不创建 JobRun 的例外，补充 audit-only 与自触发规则；
- `docs/DataStructure/TypeRegistry.md`：登记 `scheduled_job.skipped`、完整 DTO 与扩展；
- `docs/DataStructure/ChangeEvent.md`：声明同 revision 运行审计事件；
- `docs/DataStructure/ScheduledJob.md`：说明 next_due 游标推进与 calendar skip；
- `docs/DataStructure/JobRun.md`：明确不存在 local time 时不生成 JobRun；
- `docs/Acceptance.md`：补充 DST gap 去重、原子提交、重启恢复测试。

`schema.sql` 无需修改；`contracts.schema.json` 字段结构可不变，但扩展形状需在文档和程序注册表中明确。

总裁决：核心方案同意；事件名改为 `scheduled_job.skipped`；同 revision before/after 仅在“运行审计、非业务变更、after 反映 next_due 推进”的明确例外下通过。上述事务幂等、扩展注册和防自触发条件满足后，可按授权修订。由 Codex 保存至 `review/D06-calendar-skip.response.md`。
