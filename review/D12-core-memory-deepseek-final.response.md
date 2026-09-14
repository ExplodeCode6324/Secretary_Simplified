独立增量复核完成。以下是最终报告（第一轮，自然收口）。

==================================================
一、结论（回答）

范围：Core / contract / store(core_repo, questions, retrieval, memory_repo, items) / context / memory / model(input_policy, Provider, RecordingModel) / diagnostics / CLI 入口；Runtime/World 深链未审（另会话）。

裁决：本组 M1/M2/M5/M6/M7/M8 与审计传播（D12 §4.7）全部闭合，未发现 mustfix 级缺陷。独立 probe 全绿；实现者定向测试在本会话隔离副本内实跑全绿；race 子集通过仅作线索；最终 live 链 NOT_RUN。

1) 定向测试实跑（隔离副本 /tmp/d12_review/iso1/src，count=1）
   PASS TestOutputClassPropagationQuestionAndItem（question/item）
   PASS TestOutputClassLegacyUnknownAndStrictHelper（M6）
   PASS TestOutputClassMemoryAndLegacySnapshots（M5）
   PASS TestClassificationInjectionRejectsWholeDecisionAndClient（M7）
   PASS TestClassificationTypedAndHTTPDefaults（M8）
   PASS（线索）go test -race ./store ./tests -run 'TestOutputClass|TestClassification|TestQuestion|TestRetrieval|TestAyanami*' → ok

2) 我的独立 canary probe（自写，非重跑实现者用例）：/tmp/d12_review/iso1/src/tests/d12_ayanami_probe_test.go，6 项全 PASS
   P1 SYN-only 不成功携 PERSONAL：canary 先入 PERSONAL 上下文并经 Process 留存（前置条件实证）→ SYN-only 下 Build 在出 wire 前拒绝（DISCLOSURE_DENIED，错误串无 canary），Process 回退固定字面量、无模型调用、落库事件无 canary；放开 PERSONAL 后正对照必须能携 canary（事件 class=PERSONAL）；纯 SYN 会话仍可提交（非一律拒绝）。
   P2 问答回显 join：PERSONAL 提问入 state（mark=PERSONAL）→ SYN 回答轮事件携问题原文且 class=PERSONAL（≥父事件），resolved=true；随后 SYN-only 拒绝。
   P3 审计闭合：PERSONAL 轮 ModelCallRecord/decision_attempts/manifest 均实际 reqclass=PERSONAL；decision_record 原始模型内容零污染；该库零 application/json 档案（PERSONAL 档案不写 SYN），输入原文归档 text/plain=PERSONAL；SYN 轮 record 与 wire/output ObjectRef（含 output_ref）均 SYNTHETIC。
   P4 注入：模型在 actions payload 深嵌套伪造 security.classification 键 → 整 Decision 拒绝（CLASSIFICATION_INJECTION）、0 item、0 decision_record、attempt class 正确；纯文本提及（非键）不被误杀（键级语义实证）。
   P5 入口：typed 默认 PERSONAL（input/item/MASTER 事件三处）；AcceptTyped 固定回执事件 SYN（S3）；receipt/turn/assistant 三方 mark 互相一致（本次实测三者皆 SYNTHETIC，属 S3 固定字面量语义）；显式 SYN 生效；/v1/actions 缺省→200+PERSONAL item，null/""/非法值/数字→400 INVALID_SCHEMA 且零副作用；/v1/inputs 缺省→服务端补 PERSONAL，null/空→400；两端嵌套 client 注入→CLASSIFICATION_INJECTION、零 turn 入账；SECRET 不可入 AllowedClasses 且 Allows('SECRET')=false。
   P6 legacy 单条缺标 Item → Snapshot 连续两次 OUTPUT_CLASS_UNKNOWN，双 item 字节零改写、无隐式修复（保留原始字节）。

3) 源码审计要点（要点级）
   contract：SecurityClassification 严格单字段 enum、additionalProperties=false，40 载体经 extensions 引用；无顶层 class 扩张（初稿未复开）；SECRET 无伪标写入（全仓仅 rank/拒绝表出现）；ReadClassification 缺/非法一律 OUTPUT_CLASS_UNKNOWN，无默认 SYN；invalid 旧标经 ClassifyExtensions 拒绝。
   Core：req.DataClass 冻结自 Build，经 FinishDecisionTurn 传入 state 聚合/事件/回执/apply/controls（ApplyControlTx 的 outputClasses=req class；REPLAN/SUBMIT_ARTIFACT 用 DB 持久 class、join 不降）；MASTER 事件=输入原 class、ASSISTANT=有效输出 class；fallback 为固定模板+固定码白名单（S3）。
   Context/model：Build+CheckDisclosure 双保险；input_policy 按 schema 路径核查 derived carriers 必 mark（world.facts/live.items/tasks/conversation/consciousness/recent_events/retrieval.events/delta before-after），递归 scan 全量 data_class 键且嵌套≤declared、跳过 output_contract；Provider/RecordingModel/core 三层拒模型输出注入。
   memory/diagnostics：RefreshSlot 既存 slot 重读 Class+Allows，Summarize/Refresh 保存前打标、旧缺标拒绝重推导；recordMemoryResult/errorCode/transport 固定码均含 OUTPUT_CLASS_UNKNOWN（403，无细节外泄）。
   CLI/HTTP：默认 PERSONAL、--data-class 校验；RawMessage 区分缺省（PERSONAL）与 null/空（非法）。
   READ_MEMORY：system roleInstruction 含"最终答案 controls=[]、已有足够信息不得 cursor=null 重查"；控制互斥/预算/oracle 逻辑代码未动（无静默移除、无预算放宽）。

4) 自身 probe 错误区分（原始失败保留，均为我方错误、非产品缺陷，已修正后复跑全绿）
   a. 我误按 201 断言 /v1/actions 成功码——该路由用 status(e)=200（201 属 typed 路由）。
   b. P3 首跑用 setup 返回的旧 Provider（policy 快照早于我改 AllowedClasses）→ 探针自身的 DISCLOSURE_DENIED 假失败。
   c. P3 次跑误设 SYN 轮 model_calls 目录仅 1 个文件——实际另有 <call>.request.json（ObjectRef 顶层 data_class 形态）。

5) 范围边界与遗留
   - M3/M4（Job→Task→Run→REPLAN/artifact、WorldProposal→WorldFact 版本链）深链未审（Runtime/World 另会话）；本组仅核到 Core apply/controls 的 reqclass 注入点。
   - 观察项（非 mustfix，不升级门槛）：typed 路径 InputTurn/回执的 extension mark=SYNTHETIC（信封/实体=PERSONAL），与 S3 固定字面量及 docs"receipt 与同事务 InputTurn/ASSISTANT 一致"自洽；经全量 ReadClassification 调用点核查，当前无披露载体读取 input_turn.extensions，属惰性审计痕迹。
   - 最终 live 链（含 M1 真实模型腿、READ_MEMORY run2 重跑）本会话未运行：NOT_RUN；真实 run2 失败原样保留；未用夹具替代。
   - 未跑全仓 test/race/vet；未动业务源码；未写任何 review/ 报告文件；无委派；未触凭据/ELIZA/真实数据/真实模型/响铃/其他进程；无 429，未换 model/provider。

参考资料（均实际读取/实跑，非引述）
- 最终契约：review/D12-output-class-final-contract.response.md；注入裁决：review/D12-injection-oracle-clarification.response.md；READ_MEMORY 语义：review/READ_MEMORY-final-answer-clarification.response.md（唯一 prompt 语义来源）
- 源码：src/contract/classification.go、contract.go、contracts.schema.json($defs.SecurityClassification 等 40 载体)；src/store/{runtime_classification,core_repo,questions_repo,retrieval,memory_repo,items,runtime_controls,objects}.go；src/context/context.go；src/model/{input_policy,model,fixture}.go；src/memory/memory.go；src/diagnostics/model_records.go；src/core/{core,http,typed_http}.go；src/transport/transport.go；src/config/config.go；src/cmd/secretary/main.go
- 机械报告（参考后均实际复核）：reports/implementation/{M2-core-memory.md,D12-runtime-checks.json,D12-cli-class-smoke.json,d12-foundation-mechanical.json}
- 实跑证据（临时隔离副本，含我独立 probe）：/tmp/d12_review/iso1/src（tests/d12_ayanami_probe_test.go）

思路
先以两份最终契约+READ_MEMORY 语义固定验收口径（拒初稿顶层 class/SECRET 伪标），再按"实现者测试只作线索、独立复验为准"的原则：将 src 整体复制到临时隔离目录实跑指定定向测试 → 自写带独立 canary 的对抗 probe 覆盖两向控制（SYN-only 拒绝携 PERSONAL、PERSONAL 放行必须能携 canary）、问答回显 join、审计档案标记、深嵌套注入与文本提及边界、入口缺省/null 语义、legacy 零改写 → 对失败逐一归因，区分自身 probe 错误与产品缺陷后再判定。因系统预算通知已至强制收口，本轮即自然最终报告；如需第二轮，可将 probe 文件与副本保留在同一路径下直接复跑。
