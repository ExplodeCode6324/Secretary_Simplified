复核回复（最终输出，供 Codex 保存为 review/D01-D07-deepseek-revalidation.response.md）

========================
D01–D07 独立有效性复核 — DeepSeek v4.1 Flash 重新裁决
========================

【0. 声明与证据基础】

- reviewer_model=deepseek-v4.1-flash，provider=opencode-go；未换模型，未 fallback。
- 上一轮我在完成只读取证后达到工具迭代上限，Hermes 强制摘要被 provider 以 400 MissingSessionID 拒绝（x-opencode-session 路由问题）。这是 Hermes/网关的传输层故障，不是设计否决，也不是模型结论；原始失败保留，本轮同一模型直接给出最终裁决。
- 本轮严格遵守：只读（未修改任何文件）、未委派子 agent、未跑任何测试/全仓测试、未读 resource/、未连 ELIZA、未调受测 API、未响铃。允许的“已复核”仅指 DOC+CODE 只读层；runtime 测试证据（PASS/FAIL）本轮一律未复跑，凡测试文件只作为存在性与断言指向，不作为我个人的执行证据。
- Luna（gpt-5.6-luna）历史结论未沿用；以下每项均由我基于当前文档与源码重新推导（其条件清单作为核对项使用）。
- 证据快照哈希（SHA-256 前 16 位，2026-09-14 07:34–07:37 HKT）：
  docs/contracts.schema.json=eb8de293fa9157c4（二次核对未变）；src/contract/contracts.schema.json=eb8de293fa9157c4（与 docs 一致）；docs/Acceptance.md=5f8ee236ebe92570；docs/MemoryPolicy.md=f0afd8fc0ebc0079；docs/ExecutionProtocol.md=3fde4e084539d69f；docs/DataStructure/{TypeRegistry=940aa30dbf966eeb, Context=23158165c5c39252, ContextManifest=ebe3816cd8bd7391, SourceRecord=d9aeb8a6f707a508, ChangeEvent=48d1ff1c8365ecab, ScheduledJob=89128c73eab81b27, JobRun=4149af438a7ee586}；src/store/{runtime.go=b8454b273fc489f7, runtime_schedule.go=4a77ce3ce26de7ca, runtime_memory.go=0c437f533e396248, runtime_local.go=28731263437d7025}；src/contract/contract.go=1503ffbca8757b45；src/ingest/fixture.go=032967bd43f56a51；src/context/context.go=740b802d772dd9cb；src/model/model.go=6a479e6241ae7113；src/cmd/secretaryd/main.go=a22eb1ced64291a5；README.md=8a7497645521e63e。
- 并发修改告警（须记录）：复核期间工作树处于活动编辑状态——docs/contracts.schema.json 从 6664 行/165326B 变为 6749 行/167491B（随后稳定于 eb8de293）；src/store/runtime_local.go 由 fb96c134… 变为 2873126…（改动后我重读了 D05 验证器区域，内容与之前一致）；src/contract/contract.go 由 cc0951b9… 变为 1503ffbca…。凡我引用的行号，均为上述哈希对应版本；Codex 保存前若工作树再变，需对引用区段做一次最小重核对。
- D08 本轮不重审，未加分毫。

【1. 逐项裁决】

————————————————————
D01｜scheduled_job.updated 登记事件与计划同 tx —— 裁决：认可
————————————————————
具体证据
- docs/DataStructure/TypeRegistry.md:42-48（D01 修订）：新增 `scheduled_job.updated`，entity_type=scheduled_job + 完整 ScheduledJob DTO；登记 before=null、after 为 revision=1 完整计划；启停/规则/时区等业务修订含完整 before/after，entity_revision=after.revision；事件与计划写入同事务；扫描仅推进 next_due_at 不发业务事件；不扩张 WAIT 白名单；无 DTO/Schema/DDL 变更。
- src/contract/contract.go:155 白名单已含 "scheduled_job.updated"→"ScheduledJob"（且含 skipped）；:159-177 before/after/evidence 字段封闭与 after.revision==entity_revision 强制；:180 entity_type 特例已含 `scheduled_job↔ScheduledJob`（下划线差异已解决）。
- 登记原子性：src/store/runtime_schedule.go:149-151（INSERT scheduled_job 后同 tx 调 rtEvent）；rtEvent（src/store/runtime.go:75-99）before=nil、after=tx 内 rtRead 的完整 DTO、entity_revision=after.revision、event_type=entity+".updated"。
- 业务修订：UpdateJobTx（runtime_schedule.go:383-458）tx 内 rtRead old + CAS（:455-457 REVISION_CONFLICT）+ rtChange(old,m) 同 tx；rtChange（runtime.go:298-309）before/after 完整、EntityRevision=after.revision。控制面 PATCH 同路径（runtime_control.go:286）。
- WAIT 白名单未扩张：runtime_wait.go:37-41 仍仅 item.updated/task.updated/source.synced（UNREGISTERED_WAIT_EVENT）。
结论
原 D01 三项前提（b 白名单/映射、c 删除吞事件路径、d 登记与推进边界）在源码与文档中全部落地，未发现矛盾。
低危观察（非阻塞）：登记与修订事件的 origin 由 rtEvent/rtChange 固定为 "runner"，TypeRegistry/ChangeEvent 文档未写明；若未来按 origin 过滤 consumers，建议补一句声明（最小修正：TypeRegistry.md D01 段加 origin 说明）。
未复验项：runtime_test.go:238-241 对 scheduled_job.updated 的断言存在但未执行；扫描推进“不发事件”未做动态验证。

————————————————————
D02｜occ:v1 确定性绑定并冻结 —— 裁决：认可
————————————————————
具体证据
- 文档六处同步（同一段落）：docs/DataStructure/Notification.md:32、ScheduledJob.md:37、Task.md:39、TypeRegistry.md:52、docs/DataFlow.md:53、docs/ExecutionProtocol.md:86——实例键 `occ:v1:<base64url(template_key)>:<base64url(occurrence_key)>`（无填充），不含 attempt/job_revision/墙钟，同 occurrence 重试复用，模板键保持不变；TypeRegistry.md:54 补“两处同步绑定后才计算并冻结 criterion_hash；不得更改已存在 Task 完成条件；模板修订仅作用于以后未物化 occurrence；跳过 occurrence 不产生通知”。
- 编码与深拷贝：src/store/runtime.go:239 使用 base64.RawURLEncoding（无填充）双段编码；rtMap（runtime.go:16-21）为 JSON 往返深拷贝 → createRunTx（:234-254）对 command 与 notification_recorded criterion 的改写只作用于副本，模板不被污染；criterion_hash 在绑定之后冻结（:263）。
- 模板一致性守卫：RegisterJobTx（runtime_schedule.go:106-122）与 UpdateJobTx（:395-411）均强制 command key == criterion key（NOTIFICATION_TEMPLATE_KEY_MISMATCH / NOTIFICATION_CRITERION_REQUIRED）。
- 断言存在：src/tests/runtime_test.go:380-437 断言 occ:v1 前缀、command/criterion 一致、REPLAN 不改 criterion_hash、重复扫描 n==0、两 occurrence 键不同、持久计划模板键仍为原始 "fixture-notice"（未执行）。M3-runtime-implementation-evidence.md:35 声称 TestRuntimeRecurringNotificationBinding 通过（转述，非本轮证据）。
结论
核心方案与全部条件（绑定先于冻结、两处 key 一致、模板不变、重试复用、不取消 UNIQUE）在地面实现中成立。
证据边界（不以未检内容充数）：本轮直接核实了时间调度路径（ScheduleStep→createRunTx）；事件/手动 occurrence 的各自调用点本轮未逐个重读，但绑定逻辑位于共享的 createRunTx 单一收口，且注册/修订两处模板校验独立成立。若必须逐路径背书，需补一次针对事件/手动触发的定向只读核查。
固定 criteria 原则：criteria 语义哈希在绑定后冻结、REPLAN 不改（runtime_controls.go:35 对 REQUEST_COMPLETION 强制 CRITERION_HASH_MISMATCH；验证器 runtime_local.go:192-193 CRITERION_TAMPERED），未弱化。

————————————————————
D03｜合法 deleted tombstone 允许 VALID normalized=null/error=null —— 裁决：认可
————————————————————
具体证据
- Schema（docs/contracts.schema.json:487-540，SourceRecord.allOf）：VALID ⇒ validation_error=null；嵌套 if deleted==true ⇒ normalized=null，else ⇒ normalized=FixtureItemValue；QUARANTINED ⇒ normalized=null 且 validation_error 非空。即：deleted tombstone 合法，非删除 VALID 仍强制对象，QUARANTINED 规则不变——与 D03 修正的精确条件一致，未见放宽度误伤。
- 文档：docs/DataStructure/SourceRecord.md:36-38 注明该修订与依据；:32 保留“删除必须有 tombstone 或完整对账证据”。
- DTO：src/contract/dto_generated.go:62-79，Normalized *FixtureItemValue、ValidationError *string → nil 序列化为 null。
- 接入路径：src/ingest/fixture.go:71-88，deleted 跳过 Value 解码、置 VALID、Normalized 留 nil；store 层 SaveSourceRecord（src/store/sources.go:22-45）先 Validate("SourceRecord")，同版本同哈希=duplicate、不同哈希=SOURCE_VERSION_CONFLICT（调用方 quarantine），否则 INSERT。
- 回归测试存在：src/ingest/fixture_test.go:43-77（tombstone 单独与重复均成功、object_ref==1）。未执行。
结论
原 M1 报告 D1（“VALID⇒对象”与 tombstone 冲突）已被最小条件分支修复，且未削弱 QUARANTINED 与非删除 VALID 约束。证据分层：DOC+CODE；动态复跑 NOT_RUN（本轮约束）。

————————————————————
D04｜A05 重复对象含 object_ref；先 dedup/验证再对象/quarantine —— 裁决：认可
————————————————————
具体证据
- docs/Acceptance.md:15（A05）已改为“重复页无重复对象（包括 source_record 与 object_ref）”；:62-64（D04 段）明确“接入应在原文对象创建前完成版本去重与结构校验；重复页不新增 ObjectRef；冲突/无效原文以内容去重 quarantine 保存；不将逐次新增原文对象解释为通过”——即口径与实现方向都被写死，没有“两义”。
- 顺序实现（src/ingest/fixture.go:42-98）：hash → LookupSourceVersion 去重（same hash ⇒ Duplicates++，continue，不建对象）→ 同版本异哈希 ⇒ 内容寻址 quarantine（不建对象）→ 结构校验（external_id 必填；非删除解码）→ 不合格 ⇒ 内容寻址 quarantine（不建对象）→ 才 PutObject + SaveSourceRecord。
- quarantine 内容去重：fixture.go:101-125，key=Hash(source_id+version+raw)，O_EXCL + IsExist⇒nil。
- 测试存在：fixture_test.go:13-41（同页重复/冲突 quarantine 1 文件）、:43-77（rejected 不建对象、object_ref 保持 1）、src/ingest/acceptance_test.go:12-60（空页新鲜度、失败页不清库且 object_ref/source_record==1）。未执行。
结论
A05 口径与实现同时收紧而非放宽（“逐次新增对象算通过”被明文否定）。未见矛盾。

————————————————————
D05｜consciousness_slot_committed：四条件闭环 —— 裁决：认可
————————————————————
具体证据（对四条强制条件逐条核对）
1) SlotController 唯一生成：src/store/runtime_memory.go:14-88 ScheduleMemorySlot 为注释声明的唯一准入路径；两处显式守卫拒绝其他来源——RegisterImmediateTx（src/store/runtime.go:161-163，MEMORY_REFRESH_REQUIRES_SLOT_CONTROLLER）与 REPLAN（src/store/runtime_controls.go:90-92）；DeriveCriteria（runtime_local.go:277-299）无 memory.refresh 分支，default 直接拒绝 → 模型无法铸出该命令。
2) 固定 epoch + 稳定身份：epoch.go:16-65 PinEpoch 以内容寻址对象持久 epoch，哈希不符即 EPOCH_MISMATCH（显式迁移），即“固定 epoch”；root=DderiveID("secretary.memory.slot.v1:<epoch>:<slot>")，intent/request/ledger/criterion 全部确定性派生（runtime_memory.go:22-24、65、77）；slot=(now-epoch)/24h 由程序计算；MAX(slot) 已覆盖则跳过；既有 intent 的 ledger+run 直接复用（:34-37，重复/重启复用既有命令）；低槽 QUEUED 旧命令在 tx 内取消（:41-63，时钟前跳只保留当前槽）。
3) command 目标 slot 不被忽略：src/core/work.go:84-96 从 run.Command.Arguments 解析 slot，校验非负整数（INVALID_SLOT），传 RefreshSlot(ctx,int(slot),now)；RefreshSlot（src/memory/memory.go:35-60）使用传入 slot，不重算、并在更高槽已提交时拒绝（SUPERSEDED_MEMORY_SLOT）。
4) Verifier 精确 slot+完整 DTO：runtime_local.go:229-236（改动后 2873126… 重读一致）`SELECT payload_json FROM consciousness_snapshot WHERE slot=?` + contract.Decode("ConsciousnessState") + snapshot.Slot==expected.slot；无 MAX(slot)/未来槽替代。
- Schema：docs/contracts.schema.json:2096（enum）与:2272-2297（expected 严格 {slot:int≥0}，required [slot]，additionalProperties:false）与本条一致；ConsciousnessState.slot≥0（既有）。
- 旁路移除：src/cmd/secretaryd/main.go:161（runner ticker 内改为 s.ScheduleMemorySlot(ctx, epoch, time.Now(), grantID) 登记持久任务），未再直接调用 memory.Service.Refresh。
- 重试预算：src/store/runtime_execution.go:328 maxAttempts=3（无 job 时默认），仅 DISPATCHED 才重试（:336）；memory.refresh 退避 5 分钟、attempt_no≥2 起 30 分钟（:343-347），retry 时间=回执时间+delay（:349-355）——与“每槽最多三次、分别延后5/30分钟”一致。
- 文档同步：docs/MemoryPolicy.md:66、ExecutionProtocol.md:94、TypeRegistry.md:60、Verification.md:30、ConsciousnessState.md:38、Acceptance.md:68 为同一段（含 DDL 不变）。
- 测试存在：src/tests/memory_scheduler_test.go（TestPersistentMemorySlotConcurrentAndExactVerification / TestPersistentMemorySlotSkipMissedAndFrozenCriterion / TestEpochDriftRejectedByDurableObject）、runtime_acceptance_test.go:15（A08 日界）、runtime_test.go:689（登记）。未执行。
结论
四条件全部成立，且准入面比原要求更严（双守卫）。DDL 未改。
注记（非阻塞）：src/memory/memory.go:25-31 仍保留按墙钟的 Refresh 包装；secretaryd 未用，但 src/tools/livereplay/main.go:125 会调用它——该工具若用于产出“证据”，须在报告中标注它不经过持久命令路径，避免证据分层混淆。另：SaveConsciousness 拒绝旧/重复槽的行为本轮未重读源码（作为既有实现引用），如需我背书该点，需一次定向核查。

————————————————————
D06｜scheduled_job.skipped：audit-only、同 revision 游标推进、严格扩展、幂等、防自触发 —— 裁决：认可（附 1 项低危边界修订，建议后置）
————————————————————
具体证据（逐条件）
- 注册与语义：docs/DataStructure/TypeRegistry.md:63-67（白名单扩展 scheduled_job.skipped→ScheduledJob；同业务 revision 完整 before/after 仅运行字段变化且不得两侧完全相同；runtime.calendar_skip 严格 4 键；消费者视为审计不派生动作）。ChangeEvent.md:36、ScheduledJob.md:46、JobRun.md:39、ExecutionProtocol.md:99、Acceptance.md:73 同步。白名单映射实现见 contract.go:155。
- 严格扩展：contract.go:240-256 强制恰好 4 键 + reason 常量 DST_GAP + local_date(YYYY-MM-DD)/local_time(HH:MM)/IANA 时区程序解析，多一键即拒。
- 同 tx + 游标推进 + 幂等：runtime_schedule.go:155-372 全程 s.Write；before=tx 内 DTO（:172），UPDATE scheduled_job（:343）与事件插入（:363-366）同 tx；事件 ID=DeriveID("calendar-gap:"+job_id+":"+date+":"+rtHash(sm))（:348，job+日期+规范规则身份）；已存在则核对 skip 载荷与 after.schedule 哈希，一致→continue（幂等），不一致→CALENDAR_SKIP_IDEMPOTENCY_CONFLICT（:350-358）——不是“靠唯一冲突回滚推进”。
- 严格 gap 判定 + 不造假 UTC：gap 由 NextOccurrence（:55-86）在真实时区上按分钟扫描、只匹配真实存在的墙钟，故不存在的本地时刻永不生成 JobRun；calendarGapDates（:461-502）仅 daily/weekly+IANA，逐日做互反检查确认“不存在”，并且只审 would-be occurrence 日（weekly 过滤星期）；事件 CreatedAt=扫描时间（:363），无伪造 local→UTC。JobRun.md:39 区分真 overlap/misfire 的 SKIPPED run 与 gap 审计。
- 防自触发：事件规则扫描跳过 origin=="scheduler.calendar"（:197-199），并跳过 runner/scheduler 自生成的同 job 事件（:202-204）；origin 固定 scheduler.calendar（:363）。Acceptance.md:73 明文“origin=scheduler.calendar 的审计不能触发 event job”。
- 测试存在：src/tests/calendar_oracle_test.go:11（固定 UTC oracle）等。未执行。
发现一条具体矛盾（源码推演，非测试）：
- 名称：跨 DST gap 的“追赶恢复”路径可能产生无审计的静默跳过。
- 矛盾点：ScheduledJob.md:46/TypeRegistry 说“计算 next_due_at 跳过 DST gap 时必须同事务写入 scheduled_job.skipped”。但 ScheduleStep 的后备追赶循环（runtime_schedule.go:288-298）会在 downtime 跨越多日时把 latest 直接迭代推进过 gap 日（NextOccurrence 静默跳过该日，:288-296），而 gapDates 只在最后以 (latest, next) 计算（:330-339）。若进程在“上一次 occurrence 之后、gap 日之前”没有任何一次扫描（停机跨越 gap 窗口），则 gap 日落在 (due, latest) 区间内，不会被审计——与文档的“跳过即审计”语句在文字上不一致。正常路径（gap 前后有扫描、至多漏一次 occurrence，含验收所测场景）审计正确。
- 最小修正（推荐改实现，不弱化文档）：在追赶循环内逐跳累积 gap 日期（每跳 prev→next 调用 calendarGapDates 并 append），或把范围改为 (due, next)；稳定性与幂等已具备——相同事件 ID 的既有事件走 :350-358 的核对/continue 分支，覆盖已审日不会重复。补一条针对性测试：停机跨过 gap 日恢复后仍有恰一条 gap 审计（并验证 next_due 推进）。
- 若不修实现，则必须修订文档措辞并标注该边界（不推荐：属于同一缺陷族的静默跳过）。
其他：事件 origin/created_at/扩展形状/防自触发均已核；无 DDL 变化。证据分层：DOC+CODE；未执行测试。

————————————————————
D07｜Context 去自引用；身份/快照/sections/output_contract 一次；外部 Manifest 精确 wire hash —— 裁决：认可（附 2 项低危注记）
————————————————————
具体证据
- Context 不再含 manifest：docs/DataStructure/Context.md 字段表无 manifest；docs/contracts.schema.json Context（4363-4510）无 manifest 属性、additionalProperties:false，required 含 context_id/as_of/snapshot_seq/sections（4488-4507）；全文件检索 `"manifest"`=0 命中（ContextManifest 为独立 $defs）；DTO（dto_generated.go:356-377）无 Manifest 字段。ContextManifest（schema:4129-4362）保留 id/intent_id/read_set/request_hash/output_schema_*/input_bytes/token_count_mode 等，外部存储（context_manifest 表）。
- sections 严格 + retrieved_evidence：ContextSection（schema:6708-6747）name enum 含 retrieved_evidence、5 字段、additionalProperties:false；builder（src/context/context.go:64-70、138-140）逐段生成；裁剪循环后统一重算 bytes（:181-190）= 最终字段 JSON 的 UTF-8 字节数（items/facts 特判取 s.Items/s.Facts 的真实内容），与 MemoryPolicy.md:72、Acceptance.md:81 的语义一致。
- output_contract 仅一次：Context 内嵌完整 DecisionEnvelope closure（schema:4437-4440；context.go:96-98）；adapter（src/model/model.go:80-82）对 DecisionEnvelope 只写“Follow the complete output_contract embedded in the Context user message”，不再复贴 Schema；测试断言 system 段无 "$defs"、output_contract 含 DecisionEnvelope 且不含无关 ExecutionPermit（core_integration_test.go:170-208，未执行）。
- 精确 wire hash 不伪 hash：context.go:191-195 以最终 Encode(req) 计算 manifest.RequestHash/InputBytes；测试在“实际发送编码”边界断言等价（core_integration_test.go:186；memory_manifest_test.go:57 在 Generate 内捕获同一 Encode 结果比对），身份一致性 Context.context_id==Manifest.id==Decision.contextID 由 Builder 与简报校验（work.go:149）维护。
- 检索计数语义：selected=实际 EvidenceRef 数；omitted=有界检索已发现而省略候选（context.go:103-111、138、147-155），每次 READ_MEMORY 重建（MemoryPolicy.md:72）。
注记（低危，建议而非阻塞）
a) “两次 bytes 字节级一致”目前由“同一确定性 Encode 两次 + 测试断言”保证，运行时没有冻结 wire 透传或发送前等价断言；若未来 Input 在 Build 与 Generate 之间被改动，manifest hash 将静默失真。建议加固：Generate 复用已冻结 bytes（或发送前用 manifest 哈希断言），属防御性改进。
b) 当前轮对话事件自身的 evidence 被排除出 retrieved_evidence 并计入 omitted_count（context.go:105-110）；排除理由（避免与 current_input 自重复）成立，但“omitted 只计有界检索已发现而省略的候选”的口径未覆盖这种“排除理由”，建议 MemoryPolicy/Context.md 补一句限定，避免计数语义歧义。
未复验项：memory 路径 manifest 的输入裁剪（>100 delta）由 memory_manifest_test.go 断言覆盖，但未执行；非 DecisionEnvelope 的后台输出仍在 system instruction 内嵌各自 Schema（其输入不是 Context，我认为不在 D07 约束范围，此判断如需固化为设计文字，可加注）。

【2. 原则未弱化确认（逐原则，含证据）】

- 固定 criteria：绑定后冻结 criterion_hash（runtime.go:263）；验证时 CRITERION_TAMPERED（runtime_local.go:192-193）；REPLAN 不改 criteria（runtime_controls.go:35 + 测试断言）；D02 明文“不得更改已存在 Task 完成条件”。未弱化。
- 授权：计划登记/即时登记均先读 authorization_grant 并 grantCheck（runtime_schedule.go:98-104、runtime.go:176-182）；world.update 提交在同 tx 内消费一次性許可（core/work.go:74-79 policy.ConsumePermitTx）；D05 新增双守卫（只允许 SlotController）是加强。未弱化。
- 幂等：command_ledger payload_hash（IDEMPOTENCY_CONFLICT）、occurrence_key 去重、D06 稳定 ID+重放核对、D05 ledger 复用+MAX(slot)、request_receipt 全部在源码中保留/加固。未弱化。
- 快照/hash：snapshot_seq/as_of 入 Context 与 Manifest、read_set 仅本地、request_hash=最终 wire（含测试断言）、output_schema_hash 取同一 closure、tombstone 用 null 而非伪造值、gap 事件用真实扫描时间、不造假 UTC JobRun。未弱化。

【3. 汇总】

| 项 | 裁决 | 需行动 |
|---|---|---|
| D01 | 认可 | 低危注记：文档写明 scheduled_job.updated 的 origin=runner（可选） |
| D02 | 认可 | 证据边界：事件/手动 occurrence 调用点建议补一次定向只读核查（非阻塞） |
| D03 | 认可 | 无（测试复跑在授权范围内另行安排） |
| D04 | 认可 | 无 |
| D05 | 认可 | 注记 live replay 工具属诊断路径；SaveConsciousness 旧槽拒绝未重读（如需背书请定向核） |
| D06 | 认可 + 低危边界修订 | 追赶路径 gap 审计空洞：建议实现侧逐跳累积 gap 日期并补停机跨 gap 测试；否则须改文档措辞 |
| D07 | 认可 | 两项低危建议：冻结 wire/发送前断言；omitted 口径补一句限定 |

说明：以上是“设计修订有效性 + 源码可实施性”层面的裁决，不等于任何模块验收背书；README 缺陷表当前状态文字（D03/D04“修复中”等）不应因本回复被提升为“已完成复验”。测试证据（PASS/FAIL）本轮一律 NOT_RUN——按 Master 指令未执行；如需运行时证据，请在冻结工作树后按既有协议安排（且工作树当前处于活动编辑状态，保存本回复前应先固定上述哈希）。

【参考资料（含链接）】
- review/D01-D07-deepseek-revalidation.request.md（本轮范围与指定模型）
- review/D01-scheduled-job-event.response.md、D02-notification-occurrence.response.md、D05-consciousness-criterion.response.md、D06-calendar-skip.response.md、D07-context-manifest.response.md、D07-retrieval-section.response.md（历史条件清单，仅作核对项）
- review/M1-foundation-ayanami.md（D03/D04 原文与建议；README 缺陷表 60-65 行登记）
- docs/contracts.schema.json、docs/DataStructure/{TypeRegistry,Context,ContextManifest,SourceRecord,ChangeEvent,ScheduledJob,JobRun}.md、docs/{Acceptance,ExecutionProtocol,MemoryPolicy,DataFlow}.md
- src/{store/runtime.go, store/runtime_schedule.go, store/runtime_memory.go, store/runtime_local.go, store/runtime_controls.go, store/runtime_execution.go, store/epoch.go, store/sources.go, ingest/fixture.go, contract/contract.go, contract/dto_generated.go, context/context.go, model/model.go, cmd/secretaryd/main.go}
- 测试指向（未执行）：src/tests/{runtime_test.go, memory_scheduler_test.go, memory_manifest_test.go, runtime_acceptance_test.go, calendar_oracle_test.go, core_integration_test.go}、src/ingest/{fixture_test.go, acceptance_test.go}
- 传输故障记录：上一轮 Hermes 强制摘要被 provider 拒绝（400 MissingSessionID, opencode-go 路由需 x-opencode-session）；本轮为同一模型直出结论。

【思路】
先按“文档修改段→必要契约/实现片段”做只读定点取证，以固定哈希为基准；随后对每项建立“要求条件清单→源码/契约证据→反例搜索”的核对闭环，重点关注绑定/冻结时序（D02）、准入唯一性（D05）、事务与幂等（D06）、以及哈希与自引用边界（D07）；对无法本轮执行的部分（测试、个别调用点、SaveConsciousness）明确标注未复验而不冒充通过；对发现的唯一实质边界（D06 追赶路径）给出最小修正路径。全程未修改任何文件，未沿用 Luna 结论。
