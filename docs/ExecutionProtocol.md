# 调度与执行协议

## 1. 对象分工

Item 是业务事项；Task 是具有固定完成条件的委托；ScheduledJob 是何时运行某个能力的持久计划；JobRun 是一次触发；ExecutionAttempt 是该触发的一次执行尝试。循环计划的每个 occurrence 创建独立 Task 实例并复制任务模板的完成条件，模板本身不标为完成。所有实例带 job_id 与 occurrence_key。

Command 是已校验参数的执行命令。DecisionEnvelope 只提交命令提案，程序分配 ID、授权和运行状态。Task 的 criterion 在接收时保存不可变版本及哈希；重新规划只能换执行办法，修改目标必须创建新的 intent/criterion 版本并保留旧记录。

## 2. 稳定身份与原子登记

InputEnvelope.request_id 由客户端生成并重试复用。Core 为该输入分配 intent_id；非交互事件使用稳定因果根。一个意图中的动作由 operation_key 区分，唯一键为 `(intent_id, operation_key)`，不能包含本轮模型调用 ID。模型第一次合法输出后固定 operation_key 到动作语义的绑定，重算不能靠换 key 重复同一动作；新增动作必须在同一意图的持久 action ledger 检查语义和预算。

每个请求保存 canonical_payload_hash。相同键、相同规范载荷返回原回执；相同键、不同载荷返回 IDEMPOTENCY_CONFLICT。摘要哈希使用契约定义的确定性 JSON 编码，键排序、无无关空白、数组保序；禁止 NaN/Infinity。

发生业务修改、任务登记、计划登记时，所有修改与 ChangeEvent、request_receipt 同事务。时间触发以 `(job_id, scheduled_for_utc)` 形成 occurrence_key；事件触发以 `(job_id, event_id)` 形成 occurrence_key；手动触发以 `(job_id, request_id)` 形成 occurrence_key。job_revision 作为执行快照保存，不进入重复触发身份，避免编辑计划后重发同一时点。重试 attempt 沿用同一 run_id 和 external_idempotency_key。

## 3. 触发规则

| 类型 | 必填输入 | 规则 |
|---|---|---|
| immediate | command | 提交后立即创建 run |
| once | at | RFC3339 时间，转成 UTC |
| interval | anchor_at、every_seconds | 固定经过时长，every_seconds >= 60 |
| daily | local_time、timezone | IANA 时区，每日本地时刻 |
| weekly | local_time、timezone、weekdays | weekday 为 ISO 1—7，无重复 |
| event | event_type、filter、after_seq | 结构化字段过滤，禁止可执行表达式 |

日历时间不存在时跳过该次并记事件；重复时间选择第一次出现的 UTC 时刻，每个本地日期只触发一次。更改时区或规则递增 job revision，重新计算 next_due_at。已经派发的旧 run 保留原快照；尚未派发的旧 run 取消且保留已消费的 occurrence_key。若 Master 明确要在同一时点重新执行已登记的 occurrence，用新请求创建新的单次计划并显示替代关系，不靠改 revision 重发。默认 overlap=SKIP；新的 occurrence 仍保存 SKIPPED 回执，不能悄悄消失。

misfire 默认 SKIP，提醒默认 FIRE_ONCE_WITHIN_GRACE，宽限默认 5 分钟。周期计划最多补最近一个允许的 occurrence，其余记录聚合遗漏计数；不无限补历史任务。长时间停机不形成突发执行风暴。每日认知任务按 MemoryPolicy 的当前槽补偿例外规则。

## 4. 状态机

Task：`PENDING → RUNNING → VERIFYING → SUCCEEDED`。任一非终态可进入 WAITING、NEEDS_ATTENTION、FAILED、CANCELLED；WAITING 被合法事件或超时唤醒后回到 PENDING。VERIFYING 验证未通过可在剩余预算内重新规划，否则 NEEDS_ATTENTION。SUCCEEDED/FAILED/CANCELLED 终态不复用；新目标新建任务。

JobRun：`QUEUED → CLAIMED → RUNNING → SUCCEEDED | FAILED | RESULT_UNKNOWN | CANCELLED`；未执行的触发可以 SKIPPED。已过期 CLAIMED 如能证明未派发可回 QUEUED；否则进入 RESULT_UNKNOWN。Attempt 记录 PREPARED、DISPATCHED 和最终结果，不能仅靠进程消失判定未执行。

RESULT_UNKNOWN 经对账可到 SUCCEEDED/FAILED；只有证明未产生效果且授权有效时才重新进入 QUEUED。无法判明进入 NEEDS_ATTENTION 的 Task 并保留未知 run；禁止把超时统一视为失败自动重试。

取消递增 cancel_generation。未派发 run 直接取消；已派发发出 cancel 请求，并继续收集最终证据。返回的 cancel_ack 仅表示请求被接受；实际效果可能已经发生。若效果已发生，记录事实和取消竞争，不把运行改写为从未执行。

## 5. 租约与许可

默认租约 30 秒，心跳每 10 秒；每次重新领取递增 fencing_token。所有进度和回执必须匹配 run_id、attempt_no、fencing_token。旧 token 回执保留审计但不能覆盖当前运行状态，必要时触发对账。

派发前校验：capability 注册版本、授权有效期与 scope、authorization revision、task cancel_generation、任务剩余预算、executor 身份和当前 fencing_token。许可记录 permission_id 在同一短事务生成，scope 指定实体、字段、目标路径／来源以及允许操作；token 不进入模型 Context 或日志。

WorldCommitService 的许可校验和事实写入同事务，能防止检查后本地授权被撤回的竞争。外部设备或服务调用无法与本地事务原子提交：撤回生效后禁止新的派发，但此前已派发的效果必须如实对账；不得声称本地 fencing 可以阻止所有外部重复效果。

## 6. 能力契约

| 能力 ID | 运行位置 | 副作用和验证 |
|---|---|---|
| `notify.local` | P2 | 持久 CLI 通知；按通知 ID 去重，CLI 确认收取 |
| `alarm.play` | P2 | 本地文件播放；查播放会话；测试默认静音适配器 |
| `alarm.stop` / `alarm.snooze` | P2 控制 | 指定会话停止；延后创建新 once job 并去重 |
| `artifact.write` | P2 | 仅测试工作目录原子写文件，以哈希验证 |
| `source.sync` | P1 adapter | 来源页落库和 cursor；阶段 3 限 fixture |
| `briefing.build` | P1 内置 agent | 有引用的文字产物；不自动发送外部消息 |
| `memory.refresh` | P1 内置 agent | 更新指定 24 小时槽的派生快照 |
| `world.update` | P1 内置 agent + CommitService | 仅许可范围内事实更新，验证新版本和审计 |
| `memory.search` | P1 | 只读分页检索，服从披露和预算 |

每项登记参数 Schema、result Schema、side_effect_class、supports_idempotency、supports_query、supports_cancel、timeout_ms。默认最多 3 次尝试（含首次），两次重试间隔为 1 秒和 5 秒；memory.refresh 采用 MemoryPolicy 中较长的间隔。某能力不支持幂等和查询时，派发后未知结果不重试。P1 工作适配器也必须先登记 incoming run 再调用模型，重复派发返回同一工作结果。

P1 将收件写入 core_work，绑定 run_id、command_hash 和当前 fencing；重启从 checkpoint 恢复。WorldCommitService 在事实提交事务内同时更新 core_work 的最终结果，因此 P2 未收到通知时可查询，不会再次修改事实。旧 attempt 的已保存结果作为对账证据返回，P2 验证后登记当前对账结论；不能把旧 receipt 直接当作新 fencing 的执行回执。

内置 agent 默认每个根意图模型调用不超过 8 次、检索 3 次、动作 10 次、重规划 2 次、累计输出 token 16,000、执行活跃时间 5 分钟。等待期间不计活跃时间，但 Task 有 deadline_at。预算账本落库，触发子任务共享 root_id，不能通过生成新 ID 清零。每次模型调用先预留次数与输出上限再结算；崩溃后未结算的调用保留预留消耗，不能通过重启获得免费额度。

日历／间隔计划的每次独立 occurrence 由 Scheduler 分配新的执行 root，Task.parent_root_id 指向计划的触发根；这样正常周期计划不会因累计运行次数而永久耗尽单任务预算。事件引发的子任务沿用事件 root，不以新 occurrence 清零自激链预算。只有 Scheduler 的时间触发或新认证输入能开启新的独立根。

## 7. WAIT、REPLAN 与目标验收

结构化控制支持 WAIT、REPLAN、SUBMIT_ARTIFACT、REQUEST_COMPLETION 和 READ_MEMORY。WAIT 仅接受注册事件类型、对象 ID、状态值和超时；不执行模型生成的查询代码。

WAIT 登记在一个事务读取当前事件水位、检查条件、保存 wait generation 和 cursor；若条件已经满足，立即入队。消费者从 cursor 之后读取事件，再按当前状态复查，不把事件文本本身当作条件成立。触发与 Task 状态、游标提交原子化，旧 generation 不能唤醒新等待；重启补扫且超时持久化。REPLAN 不变更 criterion，SUBMIT_ARTIFACT 仅追加证据，REQUEST_COMPLETION 调用独立确定性 Verifier。

Verifier 初版覆盖 `artifact_hash_matches`、`notification_recorded`、`world_revision_matches`、`source_cursor_committed`、`master_confirmed`；自述成功不属于证据。业务标准无法机器验证时保持等待真实反馈，不能用另一个模型赞同替代。与数据库同一证据导出的两段文本算同一来源。

## 8. 防止事件循环

ChangeEvent 带 root_id、causation_id 和 origin。消费者忽略自己已处理的事件；订阅禁止由同一 root 的无状态变化事件再次触发相同规则。root 总动作预算、同一规则冷却默认 60 秒、连续 3 次无进展停止规则均持久化。重启不得重新授予预算。Replay 模式禁用所有真实执行器和外发，不能重发历史提醒。

## 实施过程中发现的缺陷 D02：周期通知的实例身份

2026-09-14 实施发现：周期计划若复用模板 notification_key，notification 全局唯一约束会使后续 occurrence 复用旧通知及验收证据。经本地 Ayanami 同意，Scheduler 在每个 occurrence 的原子物化事务内，将 notify.local 模板键实例化为 `occ:v1:<base64url(template_key)>:<base64url(occurrence_key)>`（无填充 Base64 URL 编码）。同一 occurrence 的重试/对账复用实例键，不包含 attempt、job_revision 或当前墙钟。模板键保持不变。

实例 Command.arguments.notification_key 与该实例 notification_recorded criterion.expected.notification_key 同步绑定后，才计算并冻结 criterion_hash。不得更改已存在 Task 的完成条件；修订模板仅作用于以后尚未物化的 occurrence。通知验收须同时核对实例键、当前 run 的持久通知及真实执行证据，不能用另一个 occurrence 的通知完成本次 Task。即时独立任务仍按其已接受的稳定通知键处理。

修订同时适用于时间、事件和手动 occurrence；跳过的 occurrence 不产生通知。复核依据：[Ayanami D02 讨论结论](../review/D02-notification-occurrence.response.md).

## 实施过程中发现的缺陷（D05，2026-09-14）

经 Ayanami 与实施方商议（`review/D05-consciousness-criterion.response.md`），补充 `consciousness_slot_committed`，其 expected 严格为 `{ "slot": 非负整数 }`。Verifier 必须按精确 slot 读取持久 ConsciousnessState，执行完整 DTO 校验并确认 DTO.slot 一致；不得以 MAX(slot) 或未来槽替代。仅程序 SlotController 可生成 memory.refresh，按固定 epoch/slot 派生稳定 root/request/intent 与 operation_key=memory_refresh；P2 登记持久任务，Core 使用命令目标 slot，不能重算成另一个槽。重复/重启复用既有命令；每槽最多三次，失败重试分别延后5分钟、30分钟。时钟前跳只登记当前槽并取消未派发旧槽，回拨不生成历史槽。初次 bootstrap 同样走持久命令。验收覆盖重复、精确槽、并发同槽与重启恢复；DDL 不变。


## D06 实施缺陷修正（2026-09-14）

实施过程中发现：仅计算下一个有效 UTC 时刻会跳过 DST gap 的审计。Ayanami 复核 D06 后增加 `scheduled_job.skipped` 审计事件。不存在的本地日期/时刻不伪造 JobRun.scheduled_for；事件与 next_due_at 推进同事务，before 为事务读到的完整计划，after 为推进运行字段后的完整计划，业务 revision 不增加。事件 origin 固定 scheduler.calendar，extensions.runtime.calendar_skip 记录 local_date、timezone、local_time、reason=DST_GAP。事件 ID 由 job_id、本地日期与规范规则哈希确定；相同事件重放核对语义后不阻断推进。该审计事件不能触发事件规则。

复核依据：`review/D06-calendar-skip.response.md`。
