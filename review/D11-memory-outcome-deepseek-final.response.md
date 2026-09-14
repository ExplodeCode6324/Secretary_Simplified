═══════════════════════════════════════════════════
Ayanami 独立增量复核裁决（DeepSeek v4.1 Flash / opencode-go；未委派、零 429、零真实模型调用）
D11 待答问题生命周期 + memory/consciousness 语义失败持久诊断
═══════════════════════════════════════════════════

基准与执行记录（供 Codex 存档引用）
- 基线：HEAD 2b753f2 + 未提交工作树；业务仓库全程只读；我唯一的写入都在隔离副本 /tmp/ayanami-d11rev-1789345587（探针文件 src/tests/ayanami_d11_probe_test.go）与 /tmp 临时输出内。未修改仓库/报告/同名文件，未动其他进程，未跑全仓测试。
- 定向执行（隔离副本，go1.25.6）：`go build ./...` OK；`go test ./store -run Question`（2 测试）PASS；`go test ./tests -run 'TestD11|TestMemoryProviderSuccess…'`（D11 机械 4 + Core 3 + memory 2）PASS；**我自写 7 个独立反例探针 + 2 个 memory 子探针全部 PASS**（首版一处自伤：误用不存在的 conversation_event.turn_id 列，修正探针后 PASS，产品侧无涉）。
- 另注：副本内顺带命中并全绿了我上一轮遗留的 5 个 ayanami 探针（非本轮判据）。

一、生产 must-fix ①：D11 待答问题生命周期 —— CLOSED（机制/核心/HTTP/CLI 链）
对照旧裁决三条钉死条件逐项实证：
1. 拒绝范围：docs/DataStructure/DecisionEnvelope.md:29 已逐字钉死“任一 proposal 校验失败⇒整个 Decision 失败、零副作用、不静默丢弃、proposal_index 为数组下标”。实现 src/store/questions_repo.go:83-131 在任何 apply 之前校验，161-166 以 `pending-question:v1:principal:session:request:index` 决定性派生 ID。**我的反例**：有效+重复 proposal 混合且带 Item 回调 → QUESTION_DUPLICATE、回调未执行、item=0、turn 保持 PENDING、state 零登记；修正后重试成功且派生 ID 与预期逐字相等。伪造 id/未知 item_id/SOURCE origin 的整决策拒绝另有作者用例（实跑 PASS）。
2. 非 MASTER_CLI 携 questions（含空数组）拒绝：questions_repo.go:94-97；伪主体另有 Core 授权链（core.go:155-163 + CheckCoreAuthorityTx）与作者用例证实零登记。
3. 404/409 映射：answerQuestion（questions_repo.go:24-65）槽内已解决⇒QUESTION_ALREADY_RESOLVED；未知/错 session/主体不匹配/已回收⇒QUESTION_NOT_FOUND_IN_SESSION（不泄露“曾存在”）；transport.go:39-56 映射 404/409/403。**我的反例**：HTTP resolved→409 且零持久化；锚点伪造（指向 MASTER event 或不存在 sequence）→404 且预先于归档拒绝；已回收目标→404，回收前已受理的答复提交→404 且不部分提交。
4. 补充钉死项：固定文本块逐字核验通过（`\n[question_id=…]\n<text>` 与 `\n[answered question_id=…]\n<原标题>`，ASSISTANT event 文本==最终 reply 文本）；created_sequence 等于真实 ASSISTANT sequence；幂等先于 resolved（受理回执重放−resolution 先后顺序经探针确认：同键重放返回原 turn、新键⇒409、败者不再模型）；20 槽只回收最旧 resolved、未解永不裁、满员整决策拒绝；4 个 SQL 故障点全回滚+去触发重试恰好一条问题/一条动作；旧摘要 CAS 不能复活问题、水位不动。`reports/implementation/D11-question-mechanical-checks.json` 的 test_sha256 与当前测试文件实测哈希一致（450a9124…）。

二、生产 must-fix ②：memory/consciousness 语义失败持久诊断 —— CLOSED（含明确边界）
- 实现：memory.go:114-121/172-179 两个 defer + recordMemoryResult（:225-241）固定白名单 reason（INVALID_REFERENCE、UNKNOWN_PENDING_QUESTION、DISCLOSURE_DENIED、BUDGET_EXHAUSTED、MODEL_HTTP_4xx、MODEL_SECRET_UNAVAILABLE…；provider 失败⇒PROVIDER_FAILED/MODEL_CALL_FAILED；兜底 MEMORY_RESULT_REJECTED）；sink diagnostics/model_records.go:166-184 原子写 `<DataDir>/reports/memory_attempts/<call_id>.json`，字段 call_id/context_id/root_id/role/status/reason/recorded_at，调用侧 `_ =` best-effort。
- 实证：作者两角色语义失败测试实跑 PASS；**我的反例**：provider 直接失败时两角色均产生 1 条 PROVIDER_FAILED/MODEL_CALL_FAILED 记录、call_id 可回链 model_calls（FAILED），consciousness_snapshot=0、summary 水位=0；silent 成功路径不受影响。
- 对既有诊断条款的判定：旧 M2 §二 的核心缺口（语义失败只藏在未归档 blob、无持久记录）**闭合**。root_id 即 memory 角色的 intent 对等根；无 `attempt` 序号可接受（每次模型调用一条独立记录，运行级 attempt_no 在 job_run/execution_attempt，可按 call_id/时间还原），不构成新门槛。
- 边界（明示，不发明新义务）：(a) 仅“已发生模型调用”的失败会落 record；调用前失败（epoch/披露/校验/预算）无 CallID，证据仍在既有 receipt/budget 通道；(b) 诊断 IO 失败被忽略、不改变业务——与 Core decision_attempts 同范式，符合“不发明诊断IO失败改变业务”；(c) receipt/execution_attempt 内联明细仍未填充（receipt 恒 CORE_WORK_FAILED、execution_attempt.last_error 仍 null）——具体原因改由 memory_attempts 按 call_id 查得；若要内联属执行层回执契约变更，列为残余口径而非未闭合项；(d) 尚未有真实运行产生 memory_attempts（run2 构建早于该修复，其数据目录只有 model_calls/decision_attempts）。
- 契约冲突：未发现实质冲突。仅一处措辞余留：Interfaces.md:88 的 404“文案”句 vs transport 信封 message 字段实际携带错误码（行为上不泄露且更保守；若 oracle 逐字匹配文案需对齐）。

三、D10 live-cli-run2 只读核证（不重演真实模型）
- PASS 原件核对：notification 恰 1 条 DELIVERED（text=标题，00:06:47.374→47.822Z）；Item 保持 OPEN 且 due_at 严格等于 oracle；三表 counts=1/1/1；ack_with_core_offline=true（core 终止后 ack 成功）。
- 同 request 幂等：scripts/live_cli_acceptance.py:44-45 断言重复提交返回同一 turn_id；PASS 蕴含该断言通过（重复响应原文未归档，边界如实标注）。模型调用恰 2 次（1 CLI 决策+1 memory.refresh），无重复决策。
- 时间 startUTC+90s：due−received=89.99s（00:05:16.392→00:06:46.381Z）✓。run1 原 FAIL 保留未改写 ✓。
- 模板口径精确化：run2 wire 逐字含 model.go:72 现文本（“supports only misfire=FIRE_ONCE_WITHIN_GRACE and grace_seconds=300 … Once means one occurrence … authenticated Typed … never substitute”，泛化、非特调），run1 wire 无此段且无 questions 文案——该段是 run1→run2 之间随已评审 D10 修复引入（属 K4/不变项范围），run2 准备期未再加新提示。故“未额外加 grace 提示”在“未超已评审文本、未针对用例特调”口径成立；若被读作“零任何 grace 文字”则该字面解读不成立（如实指出）。
- 附注：run2 build_sha256 与现 release 不同（run2 后为 D11 重建），run2 与源码的绑定仅经 report 字段、无该构建源码清单——属证据范围边界。
- D11 真实腿现状：reports/local/live-question-run1 本轮出现但 **FAIL 于脚本侧**（“Expecting value: line 1 column 1”，发生在新 session READ_MEMORY 轮询期；此前真实登记已成功：question d305db51… resolved=false、created_sequence=2、item 绑定、marker 齐全）。按指示不以私 seed/夹具充当该腿：D11 真实模型腿=未达成，待 root 修脚本重跑；非生产 must-fix 缺陷。

四、总结论
- 两件生产 must-fix 均 CLOSED（机械+核心+HTTP/CLI+原子性有本轮实跑与独立反例证据）；未检项均为证据范围：D11 真实模型 CLI 链（脚本 FAIL 待重跑）、memory_attempts 无真实运行记录、run2 构建源码清单缺档。不升级为退出门槛，不需要二次批准。
- 残余口径（仅登记不强加）：receipt/execution_attempt 内联明细未填；Interfaces 文案措辞；memory 记录无 attempt 序号。
- 建议：Codex 保存本裁决至 review/D11-memory-outcome-deepseek-final.response.md（当前为空占位）；D11 真实脚本修复后重跑并保留 run1 失败；真实 CLI 腿通过前，D11 只按“机械+代码链已闭合、live 未达成”表述。

参考资料（本轮实际读过/跑过）
- review/D11-pending-question-deepseek.response.md、review/M2-core-memory-deepseek-final.response.md（旧有效裁决）；review/M2-core-memory-deepseek-final.followup-notes.md、review/final-evidence-followup-notes.md
- src/store/questions_repo.go、core_repo.go、questions_test.go；src/core/core.go、http.go、work.go；src/transport/transport.go；src/memory/memory.go；src/diagnostics/model_records.go；src/cmd/secretary/main.go；src/contract/contract.go、contracts.schema.json；docs/Acceptance.md:110-114、MemoryPolicy.md:87-91、Interfaces.md:84-90、DataFlow.md:60-64、DataStructure/{DecisionEnvelope,ConversationState,InputEnvelope,InputTurn}.md
- src/tests/runtime_d11_test.go、questions_core_test.go、memory_outcome_test.go；reports/implementation/D11-question-mechanical-checks.json
- reports/local/live-cli-run2/*（report/input-oracle/runtime-state/model_calls）、live-cli-run1/report.json、live-question-run1/*（report/cli-transcript）；scripts/live_cli_acceptance.py、live_question_acceptance.py（头部）
- 我的证据文件：/tmp/ayanami-d11rev-1789345587/repo/src/tests/ayanami_d11_probe_test.go（隔离副本）

思路简述（为什么这样裁）
优先按“生产可见风险”排序：先用隔离副本复跑作者定向测试建立基线，再以自写反例攻击最危险假设——整决策原子性、伪锚点、幂等与 resolved 的时序、回收边界、provider/语义失败两态分离；对 D10 不调真实模型，只做档案交叉校验（脚本断言+数据目录+wire 逐字比对），并把“run1→run2 之间模板变化”与“run2 准备期是否加料”分开判定；对 D11 真实腿，严格区分脚本侧故障与产品侧缺陷，按指示保留 NOT-PASS 而不拿夹具顶替。全部约束（只读、临时副本、不委派、无 429/无真实凭据）均在执行记录中如实标注。( _ _ )
