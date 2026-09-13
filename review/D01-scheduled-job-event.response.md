复核完成。以下为复核回复全文（只读完成，未修改设计或源码；未读取 resource/ 凭据，未连接 ELIZA）：

========================
D01 复核回复 — scheduled_job 登记事件

复核范围（只读）：docs/ExecutionProtocol.md、docs/DataFlow.md、docs/DataStructure/TypeRegistry.md、ChangeEvent.md、ScheduledJob.md、docs/contracts.schema.json（ChangeEvent/ScheduledJob 定义）、docs/schema.sql（change_event、scheduled_job）；补充只读 src 实现取证（contract.go、runtime.go、runtime_schedule.go）。
========================

【D01-1】缺陷确认：计划登记没有合法 ChangeEvent 类型
具体问题：ExecutionProtocol §2 要求「计划登记」与 ChangeEvent、request_receipt 同事务；但 TypeRegistry 初始 event_type 清单不含 scheduled_job 任何类型，登记实际上以「静默无事件」收尾，违反同事务与审计意图。
证据：
- docs/ExecutionProtocol.md:15「发生业务修改、任务登记、计划登记时，所有修改与 ChangeEvent、request_receipt 同事务」
- docs/DataFlow.md §2.4「登记业务修改、任务、计划、回复草稿和事件」
- docs/DataStructure/TypeRegistry.md:18 事件清单仅 item.created、item.updated、task.updated、world.updated、run.updated、source.synced、memory.refreshed；同文件 :5 规定注册表版本化、「以后以新增类型迁移扩展」
- src/contract/contract.go:139-146 白名单外的 event_type 一律 UNKNOWN_EVENT 拒绝；:168-170 entity_type 以特例清单放行
- 实现侧工作区证据：src/store/runtime.go:66-68 对 entity=="scheduled_job" 直接 return nil（事件被吞）；src/store/runtime_schedule.go:105-107 RegisterJobTx 记计划后调用该函数
- docs/schema.sql:174-180 change_event 无枚举约束 → 缺口在设计与 Go 校验层，不在 DDL
方案：按 D01-2/D01-3 增加 scheduled_job 事件类型；不采用「豁免计划登记事件」的替代修法（削弱已接受要求，且以静默失败形态存在）。
结论：同意——缺陷确实存在，属设计缺陷而非实现偏差。

【D01-2】命名：单个 scheduled_job.updated（登记 before=null）
具体问题：登记语义用 .updated 是否合法，还是应加 .created 对偶。
证据：TypeRegistry.md:18「创建时 before=null」为通用语义；同域先例 task/run/world 均只有 .updated，任务登记即以 task.updated + before=null 发出（runtime.go:65-70、:219），且 M3 正统一为完整载荷；仅 item 采用 created/updated 对偶。
方案：采纳单一 scheduled_job.updated，语义写入 TypeRegistry：登记 = before:null、after 为完整 ScheduledJob（revision=1）；修订 = before/after 完整 DTO。未来若出现必须区分「新登记」的消费者，再增补 scheduled_job.created（注册表支持新增迁移）。
结论：同意提案，不反对；无需 created/updated 对偶（若 Codex 坚持与 item 对偶也可接受，但当前无消费者需要该区分，推荐最小修订）。

【D01-3】载荷方案与「仅同步 TypeRegistry 不够」的补齐项
具体问题：完整 before/after 是否合规；以及提案只提「同步 TypeRegistry」，单靠它该方案不能工作。
证据：TypeRegistry.md:18「before/after 为该 entity_type 的完整 DTO 或 null……至少一侧非空，after 的版本与 entity_revision 一致」；contract.go:158-165 强制 revision 一致；contract.go:168-170 为精确匹配特例（source↔SourceState、world_fact↔WorldFact、run↔JobRun、memory↔ConsciousnessState）——"scheduled_job" 与 "ScheduledJob" 过不了 EqualFold（下划线差异）。
方案（同意的前提，须一并落地）：
  a. 载荷：登记 before:null、after 完整 ScheduledJob（revision=1）；修订 before/after 完整；entity_revision=after.revision。与 run.updated 先例一致（含 extensions.runtime.authorization.grant_id，未超出现有披露边界）。
  b. contract.go:143 增 "scheduled_job.updated"→"ScheduledJob"；:168 增特例 scheduled_job↔ScheduledJob（否则 ValidateEvent 拒绝）。
  c. 删除 runtime.go:66-68 跳过分支，改走通用路径发出；事件与 scheduled_job 写入保持同一 tx（RegisterJobTx 已在调用方 tx 内）。
  d. 事件边界写入 TypeRegistry：登记与业务修订（enable/disable、时区/规则变更、取消/延后类修改）发 scheduled_job.updated；调度扫描推进 next_due_at 属运行推进（无 revision 变化），不单独发事件，避免扫描噪音。若实现已对推进发事件，二选一后统一写明，禁止两义。
  e. 本次不必扩展 WAIT 白名单（runtime_wait.go 维持 item/task/source 三型），除非出现等待计划变更的需求。
结论：同意载荷方案；b、c、d 为同意的前提条件——只改 TypeRegistry.md 不构成完整修订。

【D01-4】文档同步与重验（授权范围内可直接执行）
具体问题：修订须留痕并按协议重验。
方案：TypeRegistry.md 更新事件清单与登记语义，并在文内注明「实施过程中发现的缺陷」；README 缺陷表登记本项与修改文档路径；测试补充：登记事件同事务（before=null / after 完整 / revision 一致 / entity_type=scheduled_job 通过）、UNKNOWN_EVENT 仍拒绝未登记类型；重跑 go test ./...、go vet ./... 与 docs/checks（重生成 latest-report.json）。无需 DTO、contracts.schema.json 或 DDL 变更。
结论：同意按此执行。

汇总：D01-1 缺陷成立；D01-2 命名同意；D01-3 载荷同意（含 b/c/d 补齐条件）；D01-4 流程同意。双方达成一致，可按授权修订设计并注明实施缺陷、README 登记路径；无须再报 Master 批准。
========================

（回复结束。按约定由 Codex 保存为 review/D01-scheduled-job-event.response.md；我方不另写同名文件。）
