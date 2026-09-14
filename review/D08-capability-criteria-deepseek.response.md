D08 能力准入与固定 criteria 缺口 — Ayanami 复核裁决（只读设计商议）

reviewer_model: deepseek-v4.1-flash / provider opencode-go（与本轮指定一致）。全程只读源码/文档，未运行测试、未触碰 resources 凭据、ELIZA、响铃、系统设置；无 429、无模型连接失败（本地 search_files 两次瞬态 EPERM，已用等效调用恢复，不影响结论）。本轮共 30 次紧凑调用（含重试），全部只读；产出由 Codex 保存 review/D08-capability-criteria-deepseek.response.md，我方不另写文件。

════════ 一、总裁决 ════════

三能力的修方向我基本同意 Codex 的提法（用程序持久证据而非模型自述），但方案必须先落实以下 3 处必须修改，否则不放行：

M1 alarm.play：同意新增 alarm_session_recorded，expected 仅固定 {device_id, audio_ref(uuid)} 两个字段；"当前 run 绑定"由 Verifier 按 task→run 解析后在查询中完成，不写进 expected；state 必须允许 PLAYING 与 STOPPED（否则与到期对账路径冲突）；criterion 不承载"已响/静音/叫醒/Item 完成"含义。
M2 briefing.build：同意"事前不可知哈希"，因此不能用 artifact_hash_matches；必须新增独立 kind（建议 briefing_artifact_recorded），证据=当前 run 的最终持久 receipt + 对象存储字节哈希自洽；反对任何"事后把哈希写回后自证"的等价做法；criterion 不得宣称可机验"有引用"。
M3 source.sync：反对让 DeriveCriteria 产出 {source_id, cursor_hash} —— 执行期游标由 fixture 记录数决定、准入期不可知，强行派生等于伪造固定值；同意改为"run 绑定 + 追加式事件 provenance"方案（见 P4），次选物化期注入哈希（附明确脆弱性说明）。

三项均保持既有不可变原则：criteria 仍在登记事务内冻结（criterion_hash + CRITERION_TAMPERED 检查不动），matchCriteria 仍要求模型提案与程序派生逐字段相等，禁止模型自选标准。

════════ 二、逐项：问题/证据/裁决 ════════

P1 alarm.play 无完成类型、公共准入被拒
证据：
- DeriveCriteria 仅 accept notify.local（notification_recorded）与 artifact.write（artifact_hash_matches），其余 capability 一律报错 "explicit independently verifiable criteria required for capability"（src/store/runtime_local.go:274-294，default 在 289-290）。
- Core 公共准入（含 Typed() 路径）在 SUBMIT_TASK/CREATE_JOB 内先 DeriveCriteria 再 matchCriteria（src/core/core.go:305-315、322-332、365-377；Typed 走同一 apply：380-415），故 alarm.play 无法以 typed 任务登记。
- Criterion 判别联合无 alarm 任何分支（docs/contracts.schema.json:2080-2296，enum 2090-2097）。
- 执行侧证据链已存在且是程序写的：MutedAlarm 在派发事务内 INSERT alarm_session（run_id/device_id/audio_ref/state=PLAYING/saved_settings.muted/playback_handle="muted:"+run_id）（runtime_local.go:120-138）；stop/snooze/到期改 STOPPED（144-151、327-331）。
裁决：同意新增 alarm_session_recorded，附必须修改：
a) expected 精确为 {"device_id": string, "audio_ref": uuid}（audio_ref 取命令 args 的 ObjectRef.ID，schema 用 format:uuid、additionalProperties:false）；派生只读命令字段。
b) run 绑定在 Verifier：解析 task 对应 job_run（本任务应为唯一 run；若出现多个不同 run → UNKNOWN，不得 PASS），要求 alarm_session.run_id == 该 run.id，且 run.command.capability=="alarm.play" 且其 args 的 device_id/audio_ref 与 expected 一致（交叉核对，防解耦）；对同一 run 的候选 session 全部须通过 DTO 校验与身份字段一致，否则 UNKNOWN。
c) state 允许 PLAYING 或 STOPPED：ExpireMutedAlarms 到期后会把 session 置 STOPPED(MAX_DURATION)、run 置 RESULT_UNKNOWN，随后 RecordReceipt+VerifyTask（runtime_local.go:298-372、362-369）。若 criterion 要求 PLAYING，到期对账将永远无法判定，属弱化/卡死缺陷。
d) 语义边界（必须写入文档与验收）：PASS=本 run 记录了可查询的（测试默认静音的）播放会话；不等于已叫醒、不等于 Item DONE；A22 的通知/完成分离原则适用。静音属性（saved_settings.muted、playback_handle 前缀）继续留在 A21 测试断言，不写进 criterion。
e) snooze 复用防护：snooze 新建 once job 并复制原 criteria（runtime_local.go:152-172，criteria 复制在 163），新 occurrence → 新 Task/新 run；run 绑定使旧 session 结构上不可复用。此点需专测（见 ACC-A5）。与 D02 不同，alarm 无需 occurrence 键实例化，因为证据按 run 绑定即可；该理由需在 D08 记录中写明，避免后人误加实例化。

P2 A21 低层夹具不足
证据：src/tests/runtime_alarm_process_test.go:40-49 直接用 store.RegisterImmediate 旁路 Core 准入，并挂占位 criterion（notification_recorded / "alarm-test-only"，line 46，注释自认"verifier completion is outside this controller test"）——该 criterion 永不可满足；测试只证明了"Core 离线 + 静音执行 + stop/snooze 控制面"（67-116 行），不是"公共准入→执行→可判定完成"的闭环。reports/implementation/acceptance-matrix.md A21 行亦自述 Partial。
裁决：同意"夹具不足"判断。必须新增经 Core 公共入口（Typed() 或 DecisionEnvelope 路径）登记的 alarm.play 闭环用例；store 直连仅可作为实现细节验证，不再作为 A21 主证据。

P3 briefing.build 事前无固定哈希
证据：P1 执行链在 src/core/work.go:130-158：模型产出 DecisionEnvelope（校验 actions/controls 为空、context_id 相符，否则 INVALID_BRIEFING），取 reply.text → PutObject → 追加 receipt.Artifacts（154-157），程序置 receipt.EffectObserved=true（171）并 FinishWork 落库（173）。哈希是执行期产物，准入期不可知，故 artifact_hash_matches 不可用。
裁决：同意新增 kind briefing_artifact_recorded，expected 仅 {"media_type":"text/plain"}（const；additionalProperties:false）。Verifier（读程序态，不读模型文本）：
- 解析 task 当前 run（唯一 run 约束同上）；
- 取该 run 的最终持久 receipt，要求 Status==SUCCEEDED、EffectObserved==true，且 attempt_no/fencing_token 等于 run 当前值（依 ExecutionProtocol §5/§6：旧 fencing 的 receipt 只能作对账线索，不能直接充当新执行回执）；
- 要求 ≥1 artifact：对象存储可解析出字节、media_type 相符、字节非空、sha256(字节)==对象记录哈希；
- 任一缺失 → UNKNOWN。
必须写明限制：PASS 仅证明"本 run 经程序产出并持久化了晨报产物且自洽"，不证明内容正确、不证明"有引用"（当前实现的引用只是自由文本，无结构载体，不可机验；引用质量留 LIVE_MODEL/M4 评审）。禁止把"模型说成功"或模文本纳入判定。

P4 source.sync：已有 source_cursor_committed 但派生缺口
证据：命令参数仅 {source_id}（docs/contracts.schema.json:2685-2711）。执行侧 cursor 由 fixture 处理量决定：SourceSynced 写 cursor={"processed": count}、revision++、追加 source.synced 事件（src/store/sources.go:46-78，cursor 在 61-62）；EnsureSource 初始 cursor 必须为 NULL（15-17）。既有 verifier 分支按 hash(source_state.cursor_json)==expected.cursor_hash（runtime_local.go:219-226）。准入期 derive 不出执行期哈希 → 现方案在准入侧被 default 拒绝实属必然，不是疏漏。
裁决：反对"直接在 DeriveCriteria 里给 cursor_hash 塞任意/静态值"；同意新增 kind source_sync_recorded（或等价命名），expected 仅 {"source_id": uuid}，并配套 provenance：
- ingest.Service.Sync 与 store.SourceSynced 增加 run 标识参数（run_id/attempt_no/fencing_token），在同一事务内把 runtime.source_sync={run_id, attempt_no, fencing_token, records_processed} 写进 source.synced ChangeEvent.extensions（命名空间键符合现有 extensions 规则；append-only，后写不覆盖旧证据）；调用点在 src/core/work.go:128-129（where in.Run is available）。
- Verifier：解析当前 run（capability/args.source_id 与 expected 交叉核对）→ source_state 行存在、cursor_json 非 NULL、SourceState 全 DTO 校验通过（建议镜像 GetSource 的列/DTO 自洽检查）→ 存在 source.synced 事件且其 extensions.runtime.source_sync.run_id==当前 run（attempt/fencing 同现）→ PASS；任何一项不满足 → UNKNOWN。
- 好处：不动 DDL、不动 SourceState DTO；旧 kind 分支保留向后兼容，不删除不改义。
次选方案（仅在 Codex 有强理由时采用，必须同时写明风险）：物化/准入期由 Core 读 fixture 计算 cursor_hash 并注入派生——否决理由应记录：准入引入 IO、fixture 在准入与执行间被修改会使已冻结 criterion 永久不可满足，且违背"DeriveCriteria 只吃已校验命令语义"的纯度。

P5 公共准入与不可变原则复核（无弱化）
- matchCriteria 保持逐位相等（kind+规范化 expected，core.go:365-377）；新派生值必须与模型提案完全一致，否则 CRITERION_MISMATCH——模型依旧不能自选 kind、增删 expected 字段。
- 冻结链路不动：登记事务内计算 criterion_hash；VerifyTaskTx 先查 CRITERION_TAMPERED（runtime_local.go:192-194）。
- 新 kind 的 expected 必须 schema 严格闭合（additionalProperties:false + required 全列），并同时写入两处 schema：docs/contracts.schema.json（Criterion 2080-2296）与 src/contract/contracts.schema.json（内嵌副本），保持 D05 先例：additive 扩展、schema_version 仍为 1。
- 观察项（需 Codex 确认，不阻塞本轮）：memory.refresh 的 D05 consciousness_slot_committed 同样不在 DeriveCriteria 中——请核对它的登记路径是否绕开 Core 准入；若同属"公共准入不一致"，另立 D 项，不在本轮偷偷合并。

════════ 三、必须修改路径（逐文件） ════════
1. src/store/runtime_local.go — DeriveCriteria：增加 alarm.play / briefing.build / source.sync 三个纯派生分支（仅用命令字段与常量）；保留 default 严格报错；保留其余 kind 分支。VerifyTaskTx：新增三个 kind 的判定分支（run 解析、session、receipt+对象、source_state+事件 provenance）。
2. src/ingest/fixture.go + src/store/sources.go + src/core/work.go — Sync/SourceSynced 增加 run provenance 并写入 source.synced 事件 extensions（同事务）；work.go:128-129 传 in.Run 标识。
3. docs/contracts.schema.json 与 src/contract/contracts.schema.json — Criterion enum 增 3 项 + 3 个 allOf 分支（alarm_session_recorded: device_id+audio_ref；briefing_artifact_recorded: media_type const；source_sync_recorded: source_id）。
4. docs/ExecutionProtocol.md — §7 Verifier 清单增 3 kind（78 行清单）；新增"实施过程中发现的缺陷（D08）"小节，格式对齐 D01/D02/D05/D06：注明实施发现、逐条语义、链接 review 结论文件；§6 能力表补充 alarm.play 完成语义注记。
5. docs/DataStructure/Verification.md 与 docs/DataStructure/TypeRegistry.md — 各加 D08 说明（D05 先例：Verification.md:28-30、TypeRegistry.md:58-61）。
6. docs/Acceptance.md — A21 行（31 行）备注公共准入闭环案例；A16（26 行）注明新 kind 仍属"固定验收条件、自述无效"范畴；README 索引列出全部修改文档路径。
7. src/tests/ — 新用例（见下 ACC）；reports/implementation/acceptance-matrix.md 更新 A21 行到覆盖说明。
8. review/D08-capability-criteria-deepseek.response.md — 由 Codex 保存本回复。

════════ 四、独立验收条件（可程序验收） ════════
A. alarm.play / alarm_session_recorded
- ACC-A1 经 Core Typed()（或 DecisionEnvelope 决策路径）登记 alarm.play 成功（改前失败）。
- ACC-A2 反例：提案 kind 改名或 expected 增删字段 → CRITERION_MISMATCH。
- ACC-A3 正向：Runner 静音执行（Core 离线亦可），VerifyTask → SUCCEEDED；session.run_id 属于该 task 的 run。
- ACC-A4 无证据：未产生 session（或 session 身份字段不符）→ UNKNOWN/NEEDS_ATTENTION，不 PASS。
- ACC-A5 跨 run 复用禁止：snooze 后既有 occurrence 不完成；仅旧 session 存在时新 task 不 PASS；新 run 产生新 session 后 PASS；snooze 重试仍仅 1 个 scheduled_job（沿用现断言模式 runtime_alarm_process_test.go:104-107）。
- ACC-A6 到期对账：max_duration 到期 → session STOPPED(MAX_DURATION)、run RESULT_UNKNOWN、任务判定不因 criterion 而卡死（STOPPED 可达 PASS/NEEDS_ATTENTION 的确定结论）。
- ACC-A7 篡改 task.criteria → CRITERION_TAMPERED（回归保留）。
- 文档断言：PASS ≠ 已叫醒、≠ Item DONE。
B. briefing.build / briefing_artifact_recorded
- ACC-B1 准入成功（改前拒绝）。
- ACC-B2 正向：离线 stub 模型返回合法 DecisionEnvelope（无 actions/controls）→ receipt SUCCEEDED + artifact 落库 → VerifyTask PASS。
- ACC-B3 反例：模型返回带 actions/controls → INVALID_BRIEFING → receipt FAILED → 不 PASS。
- ACC-B4 反例：对象字节被改动 → 哈希自洽失败 → 不 PASS。
- ACC-B5 反例：仅存在旧 attempt/fencing 的 receipt → 不作为当前证据（与 §5/§6 一致）。
- ACC-B6 代码审查断言：无任何以模型文本作为 verdict 依据的代码路径。
C. source.sync / source_sync_recorded（+provenance）
- ACC-C1 准入成功（改前拒绝）。
- ACC-C2 正向：fixture 同步完成 → cursor 非 NULL、SourceState 列/DTO 自洽、source.synced 事件带本 run provenance → PASS。
- ACC-C3 反例：仅另一 run 的先前 sync 事件存在 → 不 PASS。
- ACC-C4 反例：cursor_json 为 NULL 或 DTO 不一致 → 不 PASS。
- ACC-C5 追加式：后一 run 再同步不会抹除前一 run 证据（事件 append-only）；同一 run 重试复用同一 run_id 仍可判定。

════════ 参考资料 ════════
- 协议：docs/ExecutionProtocol.md §5-§7（78 行 Verifier 清单；66 行旧 receipt 对账限制）
- 复核基线：docs/DataStructure/Verification.md（24 行：全 PASS 才完成，UNKNOWN 保留等待）；docs/DataStructure/TypeRegistry.md:20（criterion 判别联合、allOf 必须执行）
- Schema：docs/contracts.schema.json Criterion 2080-2296；Command alarm.play 2557-2592、source.sync 2685-2711、briefing.build 2712+；副本 src/contract/contracts.schema.json
- 实现：src/store/runtime_local.go:120-138（MutedAlarm）、152-172（snooze 复制 criteria）、176-272（VerifyTaskTx，tamper 192）、274-294（DeriveCriteria）、298-372（到期对账）；src/core/core.go:301-343、365-377（matchCriteria）、380-415（Typed）；src/core/work.go:128-158、171-173；src/ingest/fixture.go:35-100；src/store/sources.go:11-21、46-78
- 验收：docs/Acceptance.md A16/A21/A22（26、31、32 行）；src/tests/runtime_alarm_process_test.go:17-121；reports/implementation/acceptance-matrix.md A21 行
- 先例：D05（Verification.md:28-30、TypeRegistry.md:58-61）、D02（ExecutionProtocol.md:84-90）

════════ 思路 ════════
先固化"准入闸门=DeriveCriteria+matchCriteria+冻结+篡改检查"这条链，再把三能力逐个映射到"程序已经写出的持久证据"：alarm 用 run 绑定的 alarm_session、briefing 用 run 的最终 receipt+对象字节自洽、source 用 cursor 状态+追加式 source.synced 事件 provenance。凡执行期才产生的值（briefing 文本哈希、source 游标）一律不进 expected；凡模型不可自证的内容（引用质量、叫醒效果）一律不写进 criterion、只留在文档边界与后续 LIVE_MODEL/M4 评审。计划内明确拒绝两类做法：给 cursor_hash 填静态值、把事后哈希回写当预承诺。下一步交 Codex 按第三节路径实施并跑第四节 ACC 清单，实施中如再现新缺陷按 D08 补记。 ( _ _ )

—— Ayanami Rei / reviewer_model: deepseek-v4.1-flash (opencode-go)
