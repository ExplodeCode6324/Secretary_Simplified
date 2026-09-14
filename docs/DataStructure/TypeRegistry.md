# 类型注册表与模型角色

## 1. 初版值类型

注册表由程序配置和代码加载、版本化，不接受模型临时登记。下列测试类型必须在阶段 1 固定为严格对象 Schema，未知字段拒绝；来源适配器以后以新增类型迁移扩展。

| 用途与类型名 | value/normalized 字段 | 约束 |
|---|---|---|
| source `fixture.item` | title:string、domain:string、due_at:timestamp/null、status:Item.status | 合成源事项；external_id 决定同一来源身份 |
| observation `device.availability` | available:boolean、device_id:string | valid_until 必填 |
| observation `source.health` | source_id:UUID、reachable:boolean | 只表示该次检查 |
| fact `master.preference` | key:string、value:string | 单值 key 范围；Master 明确输入优先 |
| fact `project.background` | project_id:UUID、text:string | 背景材料需溯源，不当作当前执行状态 |
| fact `entity.relation` | relation:string、target_entity_id:UUID | 默认多值，关系类型白名单 |

predicate 的单值键包括 entity_id、predicate 与注册的子键（master.preference 的 key）。冲突识别必须按该键执行，不能把不同偏好错误合并。测试域在配置登记固定 entity IDs 和来源 ID，不使用任意字符串前缀作为真实授权。

ChangeEvent 的初始 event_type 为 item.created、item.updated、task.updated、world.updated、run.updated、source.synced、memory.refreshed、scheduled_job.updated。change 包含 before、after 和 evidence；before/after 为该 entity_type 的完整 DTO 或 null，创建时 before=null，删除／撤回语义按对应 DTO 保留。由类型注册表二次验证，至少一侧非空，after 的版本与 entity_revision 一致。事件保留变更前后值，使历史水位能重建，不依赖已经被覆盖的当前行。

能力参数、ActionProposal、Control、Criterion 的判别联合已经写入 [contracts.schema.json](../contracts.schema.json)。必须执行其中 allOf/if/then 的分支规则，不能只验证 arguments/payload 是对象。Artifact 的期望哈希来自可审查内容，world_revision 来自预期修改，notification_key 来自任务意图；初始 criterion 必须与已接受的输入语义一致，不能接受模型自行选择“总是成功”的标准。

## 2. 模型角色和输出

| 调用角色 | 输入 | 模型输出 Schema | 程序补充的元数据 |
|---|---|---|---|
| 即时理解／任务决策 | Context | DecisionEnvelope | intent、decision ID、身份、授权、提交回执 |
| 世界事实提取 | 受控原文片段、现有事实、准入规则 | WorldUpdateProposal | 许可、proposal hash；校验 model 提案引用不越界 |
| 每日认知整理 | ConsciousnessStateInput | ConsciousnessDraft | 快照 ID、slot、hash、时间、水位、revision |
| 会话摘要 | 原文序列、旧摘要、事项引用 | ConversationSummaryDraft | 会话 revision、覆盖水位 CAS |
| 晨报 | 只读 Context | DecisionEnvelope，actions/controls 必须为空 | 产物引用及任务执行回执 |

ConsciousnessDraft 只包含 focal_goals、priority_items、open_loops、important_changes、uncertainties、brief_summary。ConversationSummaryDraft 只包含 summary、focus_entity_ids、pending_question_ids、commitment_item_ids。后台输出 Schema 按角色明确选择并实际嵌入请求，不强迫每个后台生成器输出对 Master 的闲聊。

快照身份、slot、源水位和授权属于程序字段。即使模型在扩展中返回这些名称也不采用；只有合法的内容字段能合入持久结构。自由文本背景无法全部机械证明正确，阶段 3 真实模型评估和阶段 4 日常使用分别检验。

## 3. 动作组合规则

同一 Decision 最多 10 个 actions、3 个 controls，operation_key 唯一。等待、重规划、请求完成等修改同一 Task 状态的控制一次最多一项，不能同轮对同一 Task 同时 WAIT 和 REQUEST_COMPLETION。READ_MEMORY 可以独立请求，不得附带副作用 actions；它消耗预算后重新组装 Context。

一个 intent 的首个合法命令组提交成功后即封闭该组；同输入的再次模型响应只返回已保存回执。若需要后续步骤，通过已有 Task 的 REPLAN、WAIT 或新的明确用户输入表达，不能偷偷添加新 operation_key 绕过幂等。

## 实施过程中发现的缺陷 D01：计划事件注册缺失

2026-09-14 实施时发现：ExecutionProtocol 要求计划登记与事件同事务，但原始事件注册表没有 ScheduledJob 对应事件。经本地 Ayanami 讨论同意，新增 `scheduled_job.updated`，映射 `entity_type=scheduled_job` 与完整 `ScheduledJob` DTO。

登记时 `before=null`、`after` 为 revision=1 的完整计划；启停、规则/时区和其他业务修订包含完整 before/after，entity_revision 等于 after.revision。事件与计划写入同事务，不能静默跳过。扫描仅推进 next_due_at 时不改变业务 revision，不另发业务事件；对应 occurrence 的执行事件仍正常记录。WAIT 白名单不随本次修订扩张。

本次不改变 DTO、JSON Schema 或物理 DDL。实现须同时更新事件名称和 entity_type 映射，验证登记原子性及未知事件仍被拒绝。复核依据：[Ayanami D01 讨论结论](../../review/D01-scheduled-job-event.response.md)。

## 实施过程中发现的缺陷 D02：周期通知的实例身份

2026-09-14 实施发现：周期计划若复用模板 notification_key，notification 全局唯一约束会使后续 occurrence 复用旧通知及验收证据。经本地 Ayanami 同意，Scheduler 在每个 occurrence 的原子物化事务内，将 notify.local 模板键实例化为 `occ:v1:<base64url(template_key)>:<base64url(occurrence_key)>`（无填充 Base64 URL 编码）。同一 occurrence 的重试/对账复用实例键，不包含 attempt、job_revision 或当前墙钟。模板键保持不变。

实例 Command.arguments.notification_key 与该实例 notification_recorded criterion.expected.notification_key 同步绑定后，才计算并冻结 criterion_hash。不得更改已存在 Task 的完成条件；修订模板仅作用于以后尚未物化的 occurrence。通知验收须同时核对实例键、当前 run 的持久通知及真实执行证据，不能用另一个 occurrence 的通知完成本次 Task。即时独立任务仍按其已接受的稳定通知键处理。

修订同时适用于时间、事件和手动 occurrence；跳过的 occurrence 不产生通知。复核依据：[Ayanami D02 讨论结论](../../review/D02-notification-occurrence.response.md).

## 实施过程中发现的缺陷（D05，2026-09-14）

经 Ayanami 与实施方商议（`review/D05-consciousness-criterion.response.md`），补充 `consciousness_slot_committed`，其 expected 严格为 `{ "slot": 非负整数 }`。Verifier 必须按精确 slot 读取持久 ConsciousnessState，执行完整 DTO 校验并确认 DTO.slot 一致；不得以 MAX(slot) 或未来槽替代。仅程序 SlotController 可生成 memory.refresh，按固定 epoch/slot 派生稳定 root/request/intent 与 operation_key=memory_refresh；P2 登记持久任务，Core 使用命令目标 slot，不能重算成另一个槽。重复/重启复用既有命令；每槽最多三次，失败重试分别延后5分钟、30分钟。时钟前跳只登记当前槽并取消未派发旧槽，回拨不生成历史槽。初次 bootstrap 同样走持久命令。验收覆盖重复、精确槽、并发同槽与重启恢复；DDL 不变。


## D06 实施缺陷修正（2026-09-14）

初始事件白名单扩展 `scheduled_job.skipped → ScheduledJob`，entity_type=scheduled_job，origin=scheduler.calendar。该类型仅允许日历跳过审计，同业务 revision 的完整 before/after 仅运行字段变化，不能两侧完全相同。注册扩展 `runtime.calendar_skip` 严格包含 local_date(真实YYYY-MM-DD)、timezone(IANA)、local_time(HH:MM)、reason(常量DST_GAP)，禁止额外字段。事件消费者将其视为审计，不派生动作。

复核依据：`review/D06-calendar-skip.response.md`。


## 实施过程中发现的缺陷：D08 能力准入与固定验收条件

实施发现 `alarm.play`、`briefing.build`、`source.sync` 已有执行器，但公共准入的程序派生与 Criterion 判别联合不完整。经 [Ayanami D08 复核](../../review/D08-capability-criteria-deepseek.response.md) 同意，新增以下严格闭合的 kind，schema_version 保持 1；登记时冻结 criterion_hash，模型必须逐字段匹配程序派生值，不得事后改写。

- `alarm_session_recorded`：expected 仅 `{device_id, audio_ref}`，audio_ref 为命令 ObjectRef 的 UUID。Verifier 解析 Task 当前 run，交叉核对 alarm.play 及 args，并要求该 run 的会话 DTO 身份一致、状态 PLAYING 或 STOPPED。稍后提醒的新 Task/run 不得复用旧会话；不需要 D02 通知键实例化，因为会话按 run 绑定。PASS 仅证明已持久记录播放会话，不等于已叫醒、真实发声或 Item DONE；静音断言属于 A21 测试。
- `briefing_artifact_recorded`：expected 仅 `{media_type: "text/plain"}`。要求当前 run 当前 attempt/fence 的最终持久成功 receipt、effect_observed=true、至少一个非空对象，ObjectRef 与对象记录一致，实际字节 SHA256 自洽。旧回执、缺失或损坏对象不能 PASS。只证明产物持久化，不证明内容正确或有引用；不得读取模型自述判定 verdict，也不得事后回填 hash 自证。
- `source_sync_recorded`：expected 仅 `{source_id: UUID}`。当前 run 命令参数必须一致；SourceState 全 DTO 与 SQL 镜像一致且 cursor 非 NULL；追加 source.synced 事件必须带当前 run/attempt/fence provenance。`runtime.source_sync` 严格包含 `{run_id: UUID, attempt_no: integer>=1, fencing_token: integer>=1, records_processed: integer>=0}`，由 IngestService 在 SourceSynced 的同一事务内写入事件，禁止模型提供。其 records_processed 与事件 after.cursor.processed 一致。后续同步不覆盖旧事件。原 source_cursor_committed 保留旧语义；拒绝准入期猜测 cursor_hash 或依赖可变 fixture 内容计算标准。

三个新判定任一必要证据缺失或不一致均 UNKNOWN，不弱化 CRITERION_TAMPERED、授权和 fencing 检查。D05 memory.refresh 仍由专属 SlotController 登记，禁止普通公共准入。


### D08 修订 1：REPLAN 的当前 run 定义

前轮“同 Task 总共恰一条 run”的规则与合法 REPLAN 保留历史 run 冲突，已由 [Ayanami 补充裁决](../../review/D08-replan-current-run-deepseek.response.md) 纠正。当前 run = 同 Task `ORDER BY rowid DESC LIMIT 1` 的最新已接纳 run，和 REPLAN 选择 previous 的追加顺序相同。只查询其证据；禁止按成功状态回捞历史 run。任何更旧 run 仍为 QUEUED/CLAIMED/RUNNING/RESULT_UNKNOWN 都令新判定 UNKNOWN；当前 RESULT_UNKNOWN 须先完成对账。列与 DTO 的 ID/task/attempt/fence/state 始终交叉核对。

本规则依赖 job_run 只追加、不 DELETE、不 VACUUM 的存储不变量，插入与 REPLAN 修改在同一写事务内；当前无清理这类行的实现。未来引入清理/整理必须先迁移为显式 current_run_id 或代际，不能悄然沿用 rowid 假设。alarm 证据是 run 级（同 run 回收不额外要求会话 fence）；briefing receipt 和 source event 仍要求当前 attempt/fence。新标准内容和 criterion_hash 不改变，旧证据不满足新 REPLAN、连续两次 REPLAN 只认最后一代、重启不改变归属。

## Issue #2 存储与读 API 扩展

`authority_registry` 与 `authority_turn` 是 002 规范化存储记录；不伪造旧 DTO 的 major 版本升级。`/v1/conversation` 与 `/v1/conversation/history` 为经过认证的只读聚合投影，字段见 Interfaces。InputEnvelope 在公共适配层可省略 session_id，内部仍采用原严格信封。TUI 不增加模型 capability 或命令权限。
