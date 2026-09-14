# M2 core/context/model/memory + D09/D10 独立复核裁决（DeepSeek v4.1 Flash / opencode-go）

reviewer_model=deepseek-v4.1-flash, provider=opencode-go。本轮未委派、未跑全仓、未连真实模型/通知/响铃、未读 resources 凭据；业务仓库只读（我全部写入都在 /tmp 下的独立隔离副本内）。基线：HEAD 2b753f2，工作区 135 项未提交（评审针对当前工作树）。旧 Luna 报告仅作线索，以下每一项都是我自己读源码/跑测试/写探针得出的结论。

═══════════════════════════════
一、首要裁定（紧急候选）：pending_questions 生命周期 —— 成立，属"设计准入契约缺失 + 实现漏项"双缺
═══════════════════════════════

证据链（全部只读核实）：
1. schema 侧无通道：DecisionEnvelope 的 ActionProposal.kind 枚举仅 6 种（CREATE_ITEM/UPDATE_ITEM/SUBMIT_TASK/CREATE_JOB/WORLD_PROPOSAL/CANCEL_TASK，src/contract/contracts.schema.json:3486-3496），Control.kind 仅 5 种（WAIT/REPLAN/SUBMIT_ARTIFACT/REQUEST_COMPLETION/READ_MEMORY，:3908-3916）。没有任何 question 生命周期 Action/Control。
2. 生产代码零写路径：全仓搜索 PendingQuestions 仅命中 4 处——core_repo.go:13（初始化为空）、dto_generated.go（DTO 字段）、memory.go:188-197（只读校验）、测试文件（手工直写）。core.go 的 Process/apply/Typed 无一处引用 pending_questions。
3. memory.Summarize 只验不写：knownQuestions 取自旧状态（memory.go:188-193），draft 的 pending_question_ids 必须 ⊆ 旧集合，否则 UNKNOWN_PENDING_QUESTION（:194-198）；成功路径 v:=old 后 PendingQuestions 原样保留——既不能新增、也不能消解（resolved 标志全仓无任何程序可置位）。且生产环境中 knownQuestions 永远为空（无写入者），任何非空 IDs 必失败——F5 的"改为成员校验"在无创建路径的前提下实际不可达。
4. 唯一"通过"的 A09 是夹具自证：m2_acceptance_continuity_test.go:85-94 由测试自己 SaveConversation 手写一条 question 再只验重开持久化/CAS。按 Master 指示，不能作为 NL 完整实现。作者实现报告（reports/implementation/M2-core-memory.md:13）也自认 "pending-question resolution warrant independent re-review"。
5. 契约要求（MemoryPolicy.md L33/L37）：结构槽位要保存 pending_questions；待答问题须含 question_id、关联事项、生成序号、解决状态；回答经"明确 ID 或程序验证的指代关联"消解。Schema 里槽位形状齐（id/text/item_id/created_sequence/resolved，additionalProperties:false，contracts.schema.json:1995-2042），但"谁产生第一条 question"与"如何程序验证消解"在设计与实现两层都没有定义——是设计准入契约缺失（没有通道），因此实现自然漏项。A09 条款明列"待答问题"（docs/Acceptance.md:19），此缺口直接阻断 A09。

裁决：真实缺口，成立。不构陷：不是"忘了补一个字段"，而是缺少创建/消解两段契约与代码。

给 Codex 的最小安全方案（供商议）：
- 结构式创建（推荐唯一可行最小项）：新增一个 Action kind（如 ASK_QUESTION，payload=text+可选item_id）→ Core 在提交事务内：ID 由程序派生（DeriveID(intent_id+":question:"+序号)，拒绝模型提供 ID）；item_id 必须能解析到已知 Item 且 revision 匹配，否则整个决策拒绝；created_sequence=本回合已提交 sequence；resolved=false。随同一次 FinishTurn 与 read_set(CAS conversation revision) 原子写入。
- 程序验证消解：下一条 MASTER 回合提交时，① 输入文本显式含某个 open question 的 UUID（明确 ID），或 ② 恰好一个 open question 绑定于本回合明确引用的 Item（唯一指代）→ CAS 置 resolved=true（记录保留）；歧义不得猜测，保持 open 并回必要业务问题。禁止任何 substring/模型自声明锚点（沿用 D10 v2 已撤销 substring 的先例）；禁止模型自填 question ID。
- 配套：A09 测试必须走"NL 创建→重开持久化→答复→resolved"全链，不许夹具直写；摘要 draft 的 pending_question_ids 语义收窄为"保留声明"（当前代码已默认保留全部 open 项，可文档化）；schema 变更按 D05/D07 惯例走缺陷登记+文档同步。
- 若 Master 选择暂不实现：必须把 A09 标为未通过/推迟并书面登记，不得以现状宣称"待答问题已处理"。

═══════════════════════════════
二、live-cli-run1 诊断裁定：失败隔离正确；语义失败记录缺失
═══════════════════════════════

只读核对其私有合成数据目录（路径不复述）与 runtime-state.json：
- 事实核对：模型调用恰 2 次、均 SUCCEEDED、无 429（一次为 CLI 提醒决策 root=afc3…，一次为 bootstrap memory.refresh slot root=dbf0…）；两者原始输出 blob 均已归档（SYNTHETIC）。memory.refresh attempt1 回执 CORE_WORK_FAILED、effect_observed=false；execution_attempt/payload 显示 +5min 重排（runtime_execution.go:343-347 的 memory.refresh 专用延迟），测试约 3 分钟结束，重试尚未执行。
- 隔离判定：正确。consciousness_snapshot 行数=0（无半成品快照）；无任何 side effect 越过失败点；重试在该能力的既有 3 次/5min+30min 协议内。report.json 的 FAIL 项是 CLI 通知场景（模型静默选 SKIP0，即 D10 事故本体，另行保留），与 refresh 失败无关，两者不可混读。
- 诊断充足性判定：不足。失败原因（memory.go:122-138 的可解析引用校验，draft 引用了大小写/类型都不合法的 "task:…"/"run:…" 实体，必为 INVALID_REFERENCE）未在任何持久记录中落盘：executor_receipt 仅 CORE_WORK_FAILED、core_work 仅内嵌同一回执、execution_attempt.last_error=null、memory 角色无 decision_attempts 记录、两日志为空。只能靠"未归档的模型原始 blob + 代码反推"。即：不能只凭 SUCCEEDED 忽略——provider 层状态准确但不是语义结论；memory/consciousness 角色的语义失败记录是真实缺口（与 F4 残余同一类），需在 RefreshSlot 失败路径写入等价 decision_attempts（call_id/context_id/intent/attempt/status/reason）并填充 receipt/execution_attempt 明细。

═══════════════════════════════
三、D10（reminder defaults v1 边界）裁定：PASS（离线范围）
═══════════════════════════════

对最终澄清逐条核实（review/reminder-defaults-v1-boundary-deepseek.response.md + reminder-defaults-scope-deepseek.response.md：K2 已撤回 ≤2，改为沿用既有预算；旧 substring 规则撤回）：
1. NL Decision CREATE_JOB 且 capability∈{notify.local,alarm.play}：必须 misfire=FIRE_ONCE_WITHIN_GRACE 且 grace=300，否则 REMINDER_POLICY_UNSUPPORTED——core.go:209-232 实现精确；guard 仅由 Process（NL 路径）调用，Typed 不经此函数（core.go:412-448）。✓
2. 非默认整决策拒绝、无副作用、沿用既有 3 次预算：拒绝发生在任何写事务之前；失败回执由程序生成（core.go:202-206）；重试上限即既有 for attempt<3，未新增常量。✓
3. Typed 显式 SKIP0 保留：apply/RegisterJobTx 不读取策略 guard。✓
4. 其他 capability 不受限。✓（作者用例 briefing.build SKIP 接受）
5. 系统说明只提示 Typed、不以默认替代：model.go:72 角色说明文本与 scope 文档一致。✓

我的独立探针（与作者测试不同写法，隔离副本内自写 m2ds 探针）：
- 混合决策（CREATE_ITEM + 非默认 notify job）→ 整决策原子拒绝：调用恰 3 次、第 2 次起上下文含 REMINDER_POLICY_UNSUPPORTED 反馈、item/scheduled_job/task/job_run/notification 五表全 0、已提交回执重跑不再触发模型。PASS
- 默认 NL 首次即接受，落库 misfire/grace 恰为 FIRE_ONCE_WITHIN_GRACE/300。PASS
- Typed 显式 SKIP0 原样保留（落库恰一条 SKIP/0，未被默认替换、未被拒绝）。PASS
- 作者测试 TestNaturalReminderPolicyDefaultsAndAtomicRejection（6 用例）+ TestTypedReminderExplicitSkipRemainsUnchanged 在我的隔离副本实跑 PASS。
边界：T4（同原话真实 CLI 重跑）我未执行——run2 是否真实通过由 parent/Codex 另验；我确认的仅是"代码+夹具模型"层面的通道语义。

═══════════════════════════════
四、D09（检索预算）E1–E6 裁定：PASS（离线范围，附边界）
═══════════════════════════════

- E1 成功零命中也 RetrievalServed=true 并抑制 fallback：core.go:125-134 在 SearchMemoryPage 成功后无条件置位（与命中数无关）；context.go:46 fallback 仅在未服务检索时启用；context.go:141-143 已服务即写 extensions.context.retrieval（零命中 events:[]）。三份作者测试实跑 PASS。边界：core.go 的赋值本身无端到端测试覆盖（测试均直接构造 Builder 状态），我是代码级核实。
- E2 自引用不计省略：builder 先排除 TurnID==t.ID（context.go:106-111），且不向 omitted 累加；grouped 测试断言 omitted=1（仅来自被淘汰的旧证据）PASS。✓
- E3 净唯一 ObjectID 覆盖损失：store/retrieval.go:113-118 按 ObjectID 集合差计数；11 号候选与超 2KiB 整条候选的未覆盖引用计入省略；重复引用不计。TestRetrievalPageOmissionCountsUniqueReferenceLoss PASS。✓
- E4 分组淘汰+锚点：有 Evidence 组新→旧在后为无 Evidence 组；从尾部淘汰；无证据时退化为最新一条；锚点保留（len>1 门槛）。测试断言 E/N 顺序与锚点 PASS。✓
- E5 文本澄清与 overflow：MemoryPolicy.md L74-84 的 D09 段落与最终裁决一致（零命中信封、自引用、净引用口径、锚点退化、必要依赖+锚点超限明确失败）；TestRetrievalBudget… 第二阶段断言 CONTEXT_REQUIRED_OVERFLOW PASS。✓
- E6 Manifest 准确且每次 refresh 新 ID：Builder 每次 Build 取新 context_id；READ_MEMORY 后 attempt-- 重建→新 ID/新 manifest；manifest.RequestHash=最终 wire hash（context.go:191-195）。memory 角色由 refreshManifest 提供同构 manifest（memory/manifest.go），TestMemoryMoreThanHundredDeltasPersistsExactManifest 实跑 PASS。边界：没有专门断言"跨多轮检索 manifest.ID 各不相同"的测试——代码级确认。
（判据核查已完成；D10/D09 结论均为离线证据，不等于真实模型语义验收。）

═══════════════════════════════
五、F1–F10 逐项：已修 / 仍缺 / 未检查
═══════════════════════════════

F1 嵌套 SECRET —— 已修（离线）。src/model/input_policy.go:12-99 先强制契约校验（Context/ConsciousnessStateInput/摘要三键闭包），再对全树递归扫描 data_class：未知类/未允许类/高于声明类一律 DISCLOSURE_DENIED；RecordingModel 在归档与发送前走同一防线（model_records.go:27）。我的探针：在真实 Builder 产出的 Context 中注入合法嵌套 SECRET/SENSITIVE EvidenceRef（声明 SYNTHETIC）→ 精确 DISCLOSURE_DENIED；未知类被 schema enum 先拒（INVALID_MODEL_INPUT）同样不出网；合法 SYNTHETIC 通过。原"顶级标签伪装"反例闭合。边界：无 data_class 字段的自由文本仍依赖调用方分类（固有边界，已由 builder 拥有分类+闭包缓解）。

F2 grant+模型身份权限 —— 已修（核心路径）。NL 动作提交在 FinishTurn 事务内要求 turn.Input.Origin==MASTER_CLI 且 CheckCoreAuthorityTx(grant, principal, now)（core.go:155-163）；context 的 capability_ids 按当前 grant+principal 过滤，无 grant 为空（context.go:82-85, core_authority.go:26-41）；模型输出中的 authorized 等字段不参与授权。A06 permit 矩阵 10/10 在隔离副本实跑 PASS。残余：登记层对"有效格式但不存在/撤回/越 scope"的细粒度检查仍在 permit/dispatch 层（D05/D07 已批准的分层），未在 M2 全部 action 上逐项重打反例（与作者 D08 边界一致）。

F3 持久 epoch —— 已修。PinEpoch 以派生 ID 对象持久化，变更即 EPOCH_MISMATCH、存储损坏有校验（store/epoch.go:16-65）；RefreshSlot/ScheduleMemorySlot 均先 PinEpoch（memory.go:39, runtime_memory.go:15）。我的探针：漂移被拒、同 epoch 重针通过；作者 TestEpochDriftRejectedByDurableObject 实跑 PASS。

F4 原始输出/语义 retry 记录 —— 部分。Core 路径已闭环：RecordingModel 为必经包装（core.go:63, memory.go:114/168），记录 request hash+原始 output ObjectRef+状态（model_records.go:26-94）；语义结果逐次写入 decision_attempts（PROVIDER_FAILED/SEMANTIC_REJECTED/CONTEXT_ID_MISMATCH/READ_MEMORY/COMMITTED/COMMIT_REJECTED，core.go 对应行）。live-cli-run1 亦证实真实产生 model_calls 与 decision_attempts 记录。仍缺：memory/consciousness 角色无语义失败记录（见第二节诊断，作者实现报告亦自认）；"原始输出校验错误/retry reason"在 memory 角色不可查。

F5 pending questions —— 部分（校验改好、生命周期仍缺，阻断 A09）。详见第一节。

F6 合法 Context/唯一 output_contract —— 已修（离线）。Builder 产出满足 Context 契约的请求并接受适配器契约校验（input_policy.go:24-26）、output_contract 与 Schema 全等校验（:57-65）、DecisionEnvelope 的系统说明不再二次嵌 Schema（model.go:80-82"Follow the complete output_contract embedded in the Context user message"）；manifest 留在本地外层（D07 已批准，MemoryPolicy L70）。我的探针以 builder 原始 Context 通过 Encode 验证为证。边界：sections 7 种名称唯一性等由 schema 在 Validate 中兜底，未逐条专测。

F7 read_set —— 大部分已修。read_set 现含 ConversationState（带 revision）+ 选中 Items/Facts/Tasks/Sources/Consciousness（context.go:48-63），提交时逐项 CAS（core_repo.go:207-226）。我的探针：并发追加一轮后旧 conversation revision 提交 → CONFLICT；刷新 revision → 通过。残留：change 游标只做过 snapshot_seq 记录、未做对象级 CAS；observations 未进入 read_set（该表是否有 revision 语义未见定义）。属边界项。

F8 原话归档/幂等 —— 大部分已修。inputSemanticHash 排除自动派生副本，使"提交载荷 hash == 最终持久 envelope 语义"成立（core_repo.go:287-298）；顺序冲突在归档前先拒（:24-31 与 :239-245）。残留：并发同 request_id 不同文本的理论竞态窗口内，败者仍可能已归档出孤立 object（未复现，类属既有）；如要求彻底，可把归档纳入同一受控事务或 quarantine 登记。

F9 >100 deltas 裁剪与 omitted/统一 manifest —— 已修（离线）。RefreshSlot 先按 schema 上限裁剪到 100 再校验，omitted=已加载+快照省略的净差，并写入 OMITTED reason（memory.go:70-81）；每次尝试重建并持久化标准 ContextManifest（精确 wire hash、sections selected/omitted/bytes）。作者 TestMemoryMoreThanHundredDeltasPersistsExactManifest（125 条真实变更事件）在我隔离副本实跑 PASS。边界：离线夹具，未做 live 预算压力。

F10 Summary CAS/持久预算 —— 已修。ReserveSummaryAttempt：持久 root_budget、3 次上限、5/30 分钟 cooldown、revision 递增（memory_repo.go:407-449）；SaveConversation：revision+through CAS+无缺口校验（:247-273）。作者摘要 CAS 测试实跑 PASS。仍缺（与 F5 相关）：摘要对 pending questions 的"消解"语义不存在——那不是 F10 而是生命周期问题。

═══════════════════════════════
六、未检查/未覆盖（不得当作通过）
═══════════════════════════════
- LIVE_MODEL / REAL_USE：NOT_RUN（我全程零真实模型/零真实数据）。A23 多日回放不在本轮判据内；夹具测试不等于真实语义验收。
- 真实进程重启/备份恢复、World 模块深审、跨 10 次请求幂等、并发同槽 single-flight 等边界：未跑。
- D09 的 live-retrieval 复跑与 D10 的 T4 CLI 重跑：未验（保留原失败与 run2 由 parent 流程处理）。

═══════════════════════════════
七、总结论
═══════════════════════════════
CONDITIONAL。
- 已闭合（有本轮实跑/探针证据）：F1、F2(核心)、F3、F6、F9、F10；D10 全部固定判据（离线）；D09 E1–E6（离线）。
- 阻断项（必须闭合后才可宣称 M2/A09 通过）：① pending_questions 创建/消解全生命周期（设计+实现双缺，A09 直接相关）；② memory/consciousness 语义失败记录（F4/F10 残余，live-cli-run1 已现场证实）。
- 次要残留：F7 游标/observation 边界、F8 并发竞态窗口、F4 的 memory 角色记录、D09 core 接线无端到端测试。
执行记录：作者测试两批（检索×3+manifest+epoch；reminder 6 用例+typed+A06 十格+A17 两例+A07/A08/A09）在隔离副本全部 PASS；我自写 4 个独立探针（pending 生命周期、D10 原子性/Typed、嵌套 SECRET、epoch+read_set CAS）全部 PASS（其中一处初版探针断言过严——对 schema 先拒的未知分类强求 DISCLOSURE_DENIED——已修正为接受两级拒绝后复跑）。业务仓库未被我改动。

—— Ayanami ( _ _ )　本轮为 DeepSeek v4.1 Flash/opencode-go 独立裁决；下一步建议：把第一节的最小方案与第二节的记录补齐提交 Codex 商议后再冻结。
