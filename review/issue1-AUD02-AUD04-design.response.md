# Issue1 · AUD-02/AUD-04 设计裁决（本轮仅设计；静态复核，未运行任何测试/进程）

## 0. 复核基线声明
- 依据：`ac281ac` 提交 + 当前工作树（`src/core/work.go` 已含 AUD04 改动；`src/core/source_file.go`、两个新测试文件存在、未提交）。
- 本轮工具仅用于只读：未跑 go test/build/race，未做 HTTP e2e，未触碰 soak、其 data/binary，未调用模型/密钥/ELIZA。下述全部结论为静态代码/文档阅读（file:line 已注明），**不构成实测通过**；实测证据留待实施后的独立定向/race/build/短 HTTP e2e 轮次。

## 1. 总裁决
- **AUD-02：同意（附条件）**。runtime 提案方向与既有存储不变量相容，**无新 DTO/DDL 可行**；条件与精确边界见 §2。
- **AUD-04：同意（附条件）**。现有 `source_file.go` 草稿与 Master 给定方向一致；补两项测试条件见 §3。

## 2. AUD-02：结算协议修订——同意 + 必要校验/边界

### 2.1 必要状态/身份/取消验证（FinishWorkTx 修订的完整接受条件）
1. **状态**：`job_run` 须为 `RUNNING` 或 `RESULT_UNKNOWN`（`work_repo.go:70` 现仅收 RUNNING，是悬挂根因）。**明确拒绝** `CANCELLED` 的 run 进入 core_work 结算（当前 `CancelTaskTx` 只取消 QUEUED/CLAIMED，活 handler 的 run 不会是 CANCELLED；将来若改这一点须重审）。
2. **身份**：run_id、task_id、attempt_no、fencing_token、command_hash（`ValueHash(run.Command)` == `core_work.command_hash`，`work_repo.go:68-97` 已有部分）；旧 attempt/fence 一律 `STALE_FENCE` 拒绝，**不得**以旧代覆盖 core_work（core_work 是 run 单行）。
3. **取消代次**：取对应 durable permit `execution_permit WHERE run_id=? AND fencing_token=?`（该代次最近一次 dispatch 行，`runtime_execution.go:240` 写入 cancel_generation）；与当前 `task.cancel_generation` 比较：
   - 相等 ⇒ 正常结算（写 core_work 的 state+receipt）；
   - 已推进（取消/撤销发生在 dispatch 之后）⇒ **迟到结算**：允许持久真实结果，但须附 `extensions.runtime.cancellation = {late_settlement, cancel_generation_permit, cancel_generation_current}`（沿用 `RecordReceipt` 已有注解键 `runtime_execution.go:323-327`），status 不得拔高。
   - **不做** permit.expires_at 复核（工作发生在租约内，墙钟过期不应拒迟到收口）；授权撤销在 finish 阶段不再查 grant（admission 已查，`work_repo.go:142-148`）。
4. **只写 CoreWork**：FinishWorkTx 继续保持只 UPDATE `core_work`、不碰 `job_run`；run/task 状态只由 Runner 侧 `RecordReceipt` 控制。`world.update` 保持既有同事务收据（`work.go:74-79` 在 `CommitWorldAtomic` 回调内调用 FinishWorkTx），修订实现须在该 tx 内同样可调用。
5. **收尾 context**：业务全程用 `r.Context()`（HTTP 取消语义不变）；handler 退出后**仅** `WithTimeout(WithoutCancel(r.Context()), 5s)` 包一次 FinishWork 持久化，禁止继续模型/副作用/后台延长。条件：确认 Go 工具链 ≥1.21（本轮未读 go.mod）；不支持则用等价的独立 5s ctx。
6. **BeginWork 同代 RUNNING 拒重复**：现 `work_repo.go:50-53` 同 fence 无回执返回 nil → handler 继续 Execute，可重复模型调用。改为返回稳定错误码（如 `WORK_IN_PROGRESS`，409），**不进 Execute**；Runner 侧落 RESULT_UNKNOWN，后续对账。需测试：并发双 POST 恰一次模型调用。

### 2.2 迟到审计边界（回答"取消/撤销后可否记录已发生结果"）
**允许记录，禁止复活**。精确边界：
- 允许：① core_work 真实 receipt（§2.1-3 注解）；② `executor_receipt` 行——`RecordReceipt` 现有行为即"先插入、再按 fence/attempt 门控状态更新"（`runtime_execution.go:329-349`），旧代 receipt 落库但不改状态；③ 同 fence + 已取消 task：receipt 保存、run 可记事实状态，**task 保持 CANCELLED**（`runtime_execution.go:463-465`；验证器对 CANCELLED 不翻状态，`runtime_local.go:275-283`）；④ 审计行不可改写：`UNIQUE(run_id,receipt_key)` + hash 冲突返回 `IDEMPOTENCY_CONFLICT`，不得覆写。
- 禁止：任何新业务/模型/副作用；取消任务重试（既有守卫 `runtime_execution.go:359`）；重派发（`DispatchRun` 拒 CANCELLED，`runtime_execution.go:187`）；把取消任务标 SUCCEEDED；旧代改写 core_work/run 状态；把迟到 receipt 当新代执行回执（旧 core 结果只能经 `Query` 以新 id/key 的"对账回执"进入，`remote_query.go:52-56`，语义即 §6 协议）。
- 备注（已裁决，非开放项）：取消任务下 run 记真实终态属"记录事实与取消竞争"（ExecutionProtocol §4 line 40），保留；其不触发任何成功级联。

### 2.3 各能力结算规则（同意，边界收紧）
- **memory.refresh**：仅当 run 命令 slot==S、`consciousness_snapshot` 精确 slot S 存在、完整 DTO 校验且 DTO.slot==S（对齐 D05）方可收尾 SUCCEEDED，并标 `late_settlement basis=exact_slot_committed`。绑定合法性依据：`ScheduleMemorySlot` 仅在 MAX(slot)<S 时建 run（`runtime_memory.go:30-31`），槽行只经该确定性 root 谱系写入 ⇒ 该 run 谱系观测到的槽提交可归因于本次目标。**不得用 MAX(slot)/未来槽/任意历史**。槽未提交且模型取消（提交是原子写、`memory.go:164`，错误即未提交）⇒ FAILED/effect=false，按既有 5min/30min 预算重试。当前槽经既有 run 收口，不需新建任务；崩溃无证据则 UNKNOWN 可诊断。
- **briefing**：仅以**本次 attempt 内存中产出的 ObjectRef**（`work.go:150-154`）结算 SUCCEEDED，对象行 sha/class 一致性由 `rtObjectClassTx` 兜底。**反对**任何"按内容/存量对象搜索"式成功推断——object_ref 无 run 溯源列，去重对象可属他代，属"拿任意旧结果算成功"。模型取消且未 PutObject ⇒ FAILED/effect=false；对象可能已落但不确认 ⇒ UNKNOWN。
- **source.sync**：进入 `IngestService.Sync` 后失败 ⇒ **UNKNOWN**（可能分批写），禁止 FAILED/effect=false（会触发既有重试线 `runtime_execution.go:359-393` 造成重复部分效果）；进入前错误（未配置/读限/解析）⇒ FAILED/effect=false。拒绝/读失败不调用 Sync ⇒ 不推进 cursor、不产生成功事件/回执、不删旧记录。
- **world.update**：同 tx 收据既有，无需新机制；修订版 FinishWorkTx 在 tx 内复用即可。
- **Core 崩溃**：无证据 RUNNING 经租约清扫落 RESULT_UNKNOWN/NEEDS_ATTENTION（`runtime_execution.go:99-121`）；对账 Query 无回执即维持 UNKNOWN，**不伪 FAIL、不盲重发**。
- **Runner 侧**：`GET Query 取 durable receipt → RecordReceipt 控状态` 同意，`remote_query.go` **无需改动**（SUCCEEDED/FAILED/CANCELLED 过滤、命令哈希绑定、对账 id/key 语义均已正确；RESULT_UNKNOWN 结算不做 Query 状态机化，避免振荡）。

### 2.4 6 场景与无新 DTO/DDL
6 场景（排队未调模型取消 / 模型调用中取消 / 产物已落回执丢失 / Runner 先 UNKNOWN 同代收尾 / 旧代与迟到 / 槽收口）在本设计下**全部可实现**，且**无新 DTO/DDL**：`core_work.state` 含 RESULT_UNKNOWN（001_baseline.sql:144）；`ExecutorReceipt.status` 枚举含 RESULT_UNKNOWN/CANCELLED（contracts.schema.json:4749+）；`task.cancel_generation`（:123）、`execution_permit.cancel_generation`（:155）已存在；审计靠既有 `executor_receipt`。实施边界仅：`work.go` handler、`work_repo.go`（BeginWork/FinishWorkTx）、复用 RecordReceipt 注解；不迁移 schema。

### 2.5 最小 docs 路径
沿用 D02/D05/D06/D08 惯例：① `docs/ExecutionProtocol.md` 新增一节"实施中发现缺陷（AUD-02/04）"（结算窗口、身份/取消校验、迟到审计边界、各能力结算、Query↔RecordReceipt 分工；AUD04 读取规则）；② `docs/Acceptance.md` 增 AUD-02 六场景定向回归与 AUD-04 边界用例条目，**明确不重跑 A25、不新增任何持续时长门槛**；③ `docs/README.md` 修订索引列出上述两路径。不改 DataStructure/schema/DDL；本轮不改业务代码、不写同名 report。

## 3. AUD-04：读取限额——同意 + 条件
草稿符合方向：`O_RDONLY|O_NONBLOCK` open 解决 FIFO 打开阶段阻塞（open 前无可行预检，非阻塞打开是正解）；`f.Stat()` 对描述符（fstat 语义，读的就是该 inode，无 check-read TOCTOU）；非 regular（目录/FIFO/设备）在读取前拒绝；`LimitReader(max+1)` 保实际读限、>1MiB 拒 INPUT_TOO_LARGE；defer close；错误映射 FIXTURE_UNAVAILABLE。`work.go:112-116` 直接透传错误 ⇒ 拒绝不进入 Sync。
条件：① 补一条 gate 测试：拒绝（超限/非 regular）后 source_state cursor 不变、无成功 receipt/事件（现仅单元测试 reader 层，`source_file_test.go` 未覆盖此集成断言）；② 是否加"stat 大小快速拒绝"为可选，若加必须仍以实际读限为准（现实现省略可接受）。

## 4. 实施前置条件（必须满足）
1. 确认 Go ≥1.21（WithoutCancel）；否则等价替代仍保 5s 上限。
2. FinishWorkTx 修订版在 `Write` 与 world 同 tx 两种上下文均可调用。
3. permit 查找按 (run_id, fencing_token) 取该代 dispatch 行。
4. 任何 SUCCEEDED 判定必须有本次目标证据（exact slot 校验 / 本次 receipt 产物），无证据一律 UNKNOWN。
5. 保持 soak、其 data/binary、原证据原 hash 不动；不重跑两小时；不新增时长门槛；证据分层如实。

## 5. 确切反对点（不采纳项）
- 反对旧 attempt/fence 覆盖 core_work 或触发任何状态推进（只允许 executor_receipt 审计）。
- 反对以对象存在（briefing）或 MAX(slot)（memory）推断成功。
- 反对 source.sync 进入后记 FAILED/effect=false；反对任何 UNKNOWN 自动重试。
- 反对 5s 窗口承载任何业务/后台延长执行；反对同代重复 POST 再进 Execute。
- 反对改写/重跑既有持续测试证据，反对本轮动业务代码与同名 report。

## 6. 结论与下一步
静态结论：**同意（附 §4 条件）**。通过条件落实后，进入实施；实施完成后由独立轮次执行定向回归、race、build 与短 HTTP 假模型 e2e（正是 §2.4 六场景），届时才形成实测证据并可区分于本轮静态裁决。soak 与旧证据保持原样归 Codex 维护。
