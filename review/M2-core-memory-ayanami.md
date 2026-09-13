# M2 core/context/model/memory 复核报告 — Ayanami

复核结论：不通过（阻断性缺陷未闭合）。

本报告不是实现者自述的复述。已实际读取源码、契约与设计，运行离线测试、race、vet、构建，并在 /tmp/m2reviewprobe 做了不修改业务仓库的独立 Go 探针。未读取 resources 凭据，未连接 ELIZA，未调用真实模型 API，未响铃，未终止 Master 进程，未修改系统设置。World 模块不在本报告结论内。

## 1. 复核快照与证据分层

- 时间：2026-09-14 03:40 HKT；环境：macOS darwin/arm64，Go 1.25.6。
- Git：`No commits yet on main`；工作区文件仍均为 untracked，因而不存在可核验的 fixture commit/build commit。
- docs_hash（本报告选取的六份设计文档 SHA-256 列表再哈希）：`53dae94af35dc2c642988366f6f8c8ababc47822c5fd790b1888c1789d44c538`。
- fixture/test 组合哈希：`5752989d9e5f3dc7d82613ec467e08cbbc7668a0e03984bfb31a140be1daa1d0`。
- 关键单文件哈希：`src/model/fixture.go`=`3480d914893d9d4fde3dee2e0bdbb9775b8c4996be132793994f566013aa7e6d`；`src/tests/core_integration_test.go`=`e447fd63e7ba2c4d3dfee8fbf6c9f15826346e9d3b07f34bce4bfee19f65e124`。
- 模型配置：源码默认 `fixture`，provider 代码支持 `opencode-go/gpt-5.6-luna`；本次没有启用真实模型。
- 证据等级：本报告中的 PASS 仅为 OFFLINE/DOC；LIVE_MODEL、REAL_USE 均 NOT_RUN。结构测试不能改称真实模型验收。

## 2. 验收矩阵状态

| ID | 状态 | 结论 |
|---|---|---|
| A02 | PARTIAL | 同进程重复 request_id 的离线幂等测试通过；未证明 10 次、跨重启、动作组全量幂等，且归档哈希存在缺陷（F8）。 |
| A06 | FAIL | 模型可直接形成业务 Item 写入，动作没有统一 capability/grant 绑定；world proposal 的 grant 检查被推迟到 Runner 路径（F2）。 |
| A07 | NOT_RUN | World 由独立会话复核。 |
| A08 | PARTIAL/FAIL | 顺序 slot、23h 不更新、72h 只补当前的 fixture 行为通过；epoch 未持久化/未与 Config.Epoch 绑定，真实重启语义不成立（F3）。 |
| A09 | PARTIAL/FAIL | 摘要 CAS/水位的正例通过；pending question 被直接拒绝，Context read_set 漏掉会话状态（F5、F7）。 |
| A10 | FAIL | 最终 wire hash/字节预算与 schema 字符串注入有正例；但送入模型的对象不是合法 Context，output_contract 缺失，memory.refresh 没有统一 ContextManifest/预算装配（F6、F9）。 |
| A16 | NOT_ACCEPTED | 固定条件的若干 OFFLINE 测试通过，但权限、语义 retry、真实记录和 live 条件没有合格证据。 |
| A17 | FAIL | Builder/Memory 的显式分类测试通过；Model Provider 可被 `DataClass=SYNTHETIC` 伪装而把嵌套秘密原文放进 wire（F1）。 |
| A18 | PARTIAL | memory.refresh 的 3 次持久预算在 fixture 失败 client 上通过；摘要没有同等的 cooldown/有限语义 retry 记录，且 epoch/调用记录边界未闭合（F3、F4、F10）。 |

## 3. 阻断/高严重性问题

### F1 — HIGH：Model Provider 只信任顶层 DataClass，SECRET 可进入真实 wire

1. 具体问题：`src/model/model.go:52-74` 仅执行 `p.Config.Allows(r.DataClass)`，随后无条件把 `r.Input` JSON 序列化进请求。调用者可以把 `DataClass` 标为 `SYNTHETIC`，但在嵌套对象中放置 SECRET 字段；Provider 和 RecordingModel 都没有第二道边界检查。Security 明确要求 SECRET 永不进入 Context、模型请求、工具参数转储或日志。
2. 证据/复现：隔离探针 `TestProviderCanWireSecretUnderSyntheticLabel` 调用 `Provider.Encode`，使用 `DataClass:"SYNTHETIC"` 和 `secret_marker:"DO_NOT_SEND"`；测试通过且返回 wire 含 `DO_NOT_SEND`。这没有网络调用。
3. 修法：不要把调用者传入的字符串当作分类证明。让 ContextBuilder 产生不可伪造的、已审查的 request DTO；Provider 只接受该 DTO/已验证的 Context，递归校验所有 Evidence/ObjectRef/source/data_class，发现 SECRET 或未知分类立即拒绝。RecordingModel 也必须在归档和发送前使用同一防线。
4. 结论：需修订；在修复前 A17、云端模型权限隔离不能通过。

### F2 — HIGH：Core 的模型动作没有统一的 capability/authorization 绑定

1. 具体问题：`src/core/core.go:151-262` 的 `apply` 直接对 `CREATE_ITEM`、`UPDATE_ITEM` 写 Item；没有检查 AuthorizationGrant、scope、policy revision、撤回或过期。`src/context/context.go:61` 对每次请求无条件公开 `notify.local`、`artifact.write`、`world.update`、`briefing.build`、`memory.search`，也没有按当前 grant 过滤。`src/store/runtime.go:136-190` 的 `RegisterImmediateTx` 只把 grant 字符串放入运行记录，不在登记事务内查 grant 的主体、能力、scope、有效期和撤回状态；相关检查被推迟到 Runner dispatch。
2. 证据/复现：隔离探针 `TestSourceModelCanCommitItem` 构造 `Origin:"SOURCE"`，fake model 返回合法 `DecisionEnvelope` 的 `CREATE_ITEM`；`core.Service.Process` 成功，`ListItems` 得到 1 个新 Item。该动作没有 capability 或 grant 参与。另一个“无 grant world proposal”探针未能把预期状态稳定复现为持久成功，因此不把它作为独立运行 PASS/FAIL；源码路径仍显示登记层缺少 grant read-back，需补针对有效格式但不存在/撤回/越 scope grant 的反例。
3. 修法：将所有 action 先规范化为已登记 capability 的 Command；在同一提交事务中由认证主体解析 grant，并检查 capability、scope、policy revision、expiry/revoked、实体/predicate/operation 和预算。模型输出的 `authorized`、capability_ids、origin 等字段只能是数据，不能授予权限。无授权应使整组动作回滚并返回结构化拒绝，不应留下可等待到 Runner 才失败的 proposal/task。
4. 结论：需修订；这是 A06 与 DecisionEnvelope “服务绑定身份与授权”要求的阻断项。

### F3 — HIGH：24 小时 slot 的 epoch 由调用者注入，未持久化也未使用 Config.Epoch

1. 具体问题：`src/memory/memory.go:23-55` 用 `s.Epoch` 计算 slot 和 root；`config.Config.Epoch` 虽在 `src/config/config.go:29-38` 存在，却没有被 Refresh 校验或加载。数据库也没有保存 epoch 指纹。重启时只要构造 Service 时使用了不同 epoch，就可能改变 slot/root 命名空间。
2. 证据/复现：隔离探针 `TestEpochIsNotPersistent` 首先以 epoch E 生成 slot 0，再用 epoch E-48h 模拟重启，在实际时间 E+24h 刷新；结果出现 2 条 `consciousness_snapshot`，说明同一实际时间线可因调用者改变 epoch 而生成错误的新 slot。实现者测试 `TestTypedItemsAnd24HourMemory` 只用同一个内存变量作为 replacement 的 Epoch，不能证明跨进程配置恢复。
3. 修法：初始化时把规范化 UTC epoch、config hash 和 schema/policy 版本写入持久配置表；Refresh 必须从持久值计算 slot，配置改变需显式迁移并拒绝静默启动。request/intent/root 统一由持久 epoch+slot 派生；补真实 reopen/重启探针和 epoch drift 拒绝测试。
4. 结论：需修订；顺序 fixture 的 slot 通过不能覆盖 A08 的持久语义。

### F4 — HIGH：实际 ModelCallRecord 没有接入 Core/Memory，语义 retry 也没有被记录

1. 具体问题：`src/diagnostics/model_records.go:15-81` 只是一个显式包装器；`core.Service` 和 `memory.Service` 的 `Model` 字段由调用者直接注入，当前集成测试 `src/tests/core_integration_test.go:20-29` 注入的是裸 `model.Provider`。`src/core/core.go:62-129` 在 provider 返回后才做 ContextID、controls、operation key 和事务语义校验；这些失败会继续 retry，但 RecordingModel 只把 provider 调用记录为 `SUCCEEDED`，没有 attempt_no、语义校验结果、原始输出校验错误或 retry reason。
2. 证据/复现：`src/diagnostics/model_records_test.go:15-56` 只直接调用 `RecordingModel.Generate`，证明包装器单独能保存一个 fixture 输出；它没有经过 Core/Memory。源码中没有 Core/Memory 到 RecordingModel 的默认构造或必经装饰器。因而“有实际 ModelCallRecord”只能对直接 wrapper 单测成立，不能对被测 M2 链路成立。
3. 修法：在唯一的 model adapter/service factory 中强制记录每次尝试；保存 request hash、原始 output ObjectRef（仅允许的 data class）、provider 结果、schema validation、semantic validation、retry reason、attempt index、最终提交关联。区分 provider call record 与 decision-attempt record；语义失败不能伪装成 SUCCEEDED。补一个 fake model 首次返回错误 ContextID、第二次合法的端到端测试，断言两次原始输出和失败原因均持久存在。
4. 结论：需修订；用户要求的“原始输出校验/语义重试记录、实际 ModelCallRecord”尚未达到。

### F5 — HIGH：ConversationSummary 的 pending questions 被无条件丢弃

1. 具体问题：`src/memory/memory.go:119-137` 先解码 `ConversationSummaryDraft`，但只要 `PendingQuestionIDs` 非空就返回 `UNKNOWN_PENDING_QUESTION`，从不验证并写入 `ConversationState.PendingQuestions`。
2. 证据/复现：隔离探针 `TestSummaryDropsPendingQuestions` 返回含 1 个合法 UUID 的 `ConversationSummaryDraft`；结果稳定为 `UNKNOWN_PENDING_QUESTION`，摘要水位不推进。契约 `ConversationSummaryDraft` 明确允许 `pending_question_ids`，MemoryPolicy A09 要求跨会话取回待答问题。
3. 修法：在摘要输入中提供当前待答问题的完整结构（question_id、关联事项、生成序号、解决状态）；验证输出 ID 必须属于输入范围，按 CAS 将保留的问题结构合入 `ConversationState.PendingQuestions`，已解决项由明确 ID/程序验证消解。不能用“任何非空都拒绝”替代处理。
4. 结论：需修订；A09 不通过。

### F6 — HIGH：ContextBuilder 没有构造真实 Context，Schema 注入测试只验证了字符串

1. 具体问题：`src/context/context.go:44-71` 以裸 `map[string]any` 组装请求，缺少 Context 契约要求的 `output_contract`、`extensions` 等字段；`manifest` 也没有成为模型侧 Context 对象。`src/model/model.go:60-70` 只把 `contract.Schema(r.OutputType)` 拼进 system prompt 的字符串，并设置 `json_object`，没有把 output contract 放入结构化 Context。
2. 证据/复现：隔离探针 `TestContextIsNotContextContract` 构建真实 Builder 请求；`contract.Validate("Context", req.Input)` 失败，且 JSON 中没有 `output_contract`。现有 `TestWireManifestHashAndSchemaBudget`（`core_integration_test.go:170-196`）只检查 system content 含 `DecisionEnvelope`、不含 `ExecutionPermit`，没有验证 Context DTO、output_contract、capability 过滤或 required 字段闭合。
3. 修法：定义最终 wire DTO，按 `contract.Context` 填齐 required 字段；output_contract 使用本次输出类型的实际引用闭包并计入预算。若 manifest 按设计只留在本地外层，则应修订 Context 契约/文档，不能让测试和实际请求各自采用不同形态。对最终序列化 payload 做独立 `contract.Validate`，再计算 hash/bytes/tokens。
4. 结论：需修订；A10 当前是“预算 hash 通过、Context 契约不通过”。

## 4. 中严重性问题

### F7 — MEDIUM：Core read_set 没有覆盖会话状态、摘要和所有被使用的权威依赖

1. 具体问题：`src/context/context.go:34-43` 只把选中的 Item/WorldFact/Task 放入 read_set；而请求还使用 ConversationState、recent events、delta events、consciousness 和 source/observation 投影。`src/store/core_repo.go:181-199` 虽支持 `ConversationState`，但 Core 从不填入该引用。
2. 证据：`FinishTurn` 只调用 `CheckReadSetTx(ctx, tx, manifest.ReadSet)`；全局 `SnapshotSeq` 没有替代对象 CAS 的能力。并发追加一轮会话原话或修改摘要时，已有模型请求仍可按旧 conversation 继续提交。
3. 修法：read_set 必须按实际使用内容加入 session revision/through_sequence、consciousness snapshot、source/observation 版本和必要 change cursor；提交时逐项 CAS，冲突按设计最多重建 2 次后返回 CONFLICT。不要以全局水位代替对象版本。
4. 结论：需修订；A09/A10 的连续性与一致性证据不完整。

### F8 — MEDIUM：输入归档发生在幂等判定前，canonical hash 与持久 envelope 不一致，冲突请求留下孤立对象

1. 具体问题：`src/store/core_repo.go:15-32` 先对未加入自动原话 ObjectRef 的 `InputEnvelope` 计算 hash，再在 `archiveInput`（`:234-245`）追加 ObjectRef。冲突/失败请求也会先执行 `PutObject`。
2. 证据：隔离探针 `TestConflictArchivesUncommittedText` 首次提交文本 `one` 后，以同 request_id 提交文本 `two`；返回 `IDEMPOTENCY_CONFLICT`，但 `object_ref` 计数从 1 变成 2。第一次持久化的 `InputTurn.Input` 已含自动附件引用，而 receipt hash 是附件加入前的语义载荷 hash。
3. 修法：先在受控事务/明确的 quarantine 路径完成原文对象登记，再对最终将持久化的 envelope 计算 canonical hash，并把 receipt、turn、event、object reference 的关系写成一致的提交记录。若冲突输入也必须保留，须登记为有出处的 rejected/quarantine record，不能留下无引用 object_ref。
4. 结论：需修订；当前只能说“原话正例被保存”，不能说输入持久化链完整。

### F9 — MEDIUM：memory.refresh 没有统一使用 Context 预算/manifest，变化截断没有被披露

1. 具体问题：`src/memory/memory.go:42-58` 直接把完整 `MemorySnapshot` 投影成 `ConsciousnessStateInput` 后调用模型；没有 ContextManifest、section omission 计数或 memory 专用的必要依赖裁剪。`src/store/memory_repo.go:167-210` 的 change event 读取上限为 1000，但 `DeltaOmitted` 没有进入 `ConsciousnessStateInput`；Schema 又把 `recent_changes` 限制为 100 项。
2. 证据：Core Builder 的 byte/hash 正例不能证明 memory.refresh；memory.Refresh 仅依赖 Provider.Encode 的总字节检查，超限时在已预留预算后直接失败。不存在对应的最终请求 manifest 可审计实际省略原因。
3. 修法：为所有模型角色复用统一预算器：先保留当前权威、依赖闭包、slot/水位和 schema，再按确定顺序裁剪；每段写 selected/omitted/reason/watermark；必要依赖放不下必须返回 CONTEXT_REQUIRED_OVERFLOW。将 changes omission 显式加入输入或报告，不得静默把 1000/100 的截断当成完整变化。
4. 结论：需修订；A10 不能只由 DecisionEnvelope Builder 的正例代表。

### F10 — MEDIUM：摘要 retry 只有通用预算扣减，没有 24h/语义 retry 的持久状态与记录

1. 具体问题：`src/memory/memory.go:108-117` 每次 Summarize 扣 `model_calls` 和 `output_tokens`，但没有 cooldown、retry reason、attempt record；`src/store/memory_repo.go:312-353` 的特殊 3 次 slot 预算只覆盖 memory.refresh。摘要遇到模型错误/语义错误时可被外部反复调用，直到通用 root budget 的 8 次上限，而不是显式有限 retry 协议。
2. 证据：已有测试 `TestMemoryRetryDurableLimit` 只覆盖 memory.refresh fixture failure，并验证 3 次；没有摘要错误重试、延迟、重启恢复或语义失败记录测试。
3. 修法：摘要也建立持久 attempt/cooldown 状态，定义首次+有限重试的明确间隔和终态；每次失败保留原话、raw output、校验错误和预算消耗，重启继续而不是重置。
4. 结论：需修订或明确摘要不属于 A18；在当前规范解释下不能宣称全模块有界自动运行通过。

## 5. 已实际通过的离线部分

- `src/tests/core_integration_test.go:198-209`：typed 失败后 `PendingTurns` 为 0；说明该反例没有进入模型扫描队列。未覆盖对象归档残留和所有 action 类型。
- `:211-242`：相同原话 ObjectRef、ConversationEvent evidence 和内容去重的正例通过。
- `:37-74`：fixture 输入受理、同 request_id 重试、空动作 DecisionEnvelope 的事务完成、语义重复 Process 不追加 assistant event，通过。它没有提交模型生成的业务动作。
- `:76-120`：同 slot 不重生成、72h 跳到 slot 3 并记录 missed_slots=2、当前 Item 进入意识内容，通过；epoch persistence 未覆盖。
- `:121-152`、`:153-168`：Builder 对 PERSONAL/SECRET 分类的拒绝正例通过；这证明受控 Builder 的路径，不证明 Provider 防绕过。
- `:170-196`：最终 wire 的 request hash、byte measurement 和输出 Schema 名称字符串正例通过；不证明完整 Context 契约或实际模型语义。
- `:286-301`：memory.refresh 的 3 次 durable budget/cooldown fixture 失败路径通过。
- `src/diagnostics/model_records_test.go:15-56`：直接包装 `model.Provider` 时，synthetic request/output 的 ObjectRef 和 ModelCallRecord 可读回且 output 为实际 fixture bytes，通过；不证明 Core/Memory 默认接入。

## 6. 实际执行命令与结果

1. `go test ./tests ./store ./diagnostics ./ingest`：PASS。
2. `go test ./...`：PASS。
3. `go vet ./...`：PASS。
4. `gofmt -l core context model memory store/core_repo.go store/memory_repo.go diagnostics/model_records.go tests/core_integration_test.go`：无输出，PASS。
5. `go test -race ./tests ./store ./diagnostics ./ingest`：PASS。
6. `go test -race ./...`：一次 FAIL，`src/tests/runtime_test.go:371 TestRuntimeEventRuleCooldownSurvivesDatabaseState` 报 `durable cooldown 0 <nil>`；随后单独 `go test -race ./tests -run '^TestRuntimeEventRuleCooldownSurvivesDatabaseState$' -count=1 -v`：PASS。故全量 race 不能报告为稳定 PASS，M2 定向 race 通过；该失败属于 runtime/World 侧，不将其归责给 M2，但也不隐藏。
7. `go build ./cmd/secretary ./cmd/secretaryd`：PASS。
8. `python3 docs/checks/validate_docs.py`：BLOCKED/NOT_RUN；工具输出要求先在隔离环境安装 `docs/checks/requirements-docs.txt`。本次没有安装依赖，不能把已有 `latest-report.json` 当本次执行证据。
9. 隔离 `/tmp/m2reviewprobe`：六个已完成的离线探针（source action、conflict archive、pending question、epoch、Context contract、Provider secret wire）均按各自预期通过；另一个无 grant world proposal 探针未稳定复现预期持久状态，故不作为 PASS/FAIL 证据，仅保留为待补反例。

## 7. 未覆盖边界

- LIVE_MODEL：NOT_RUN。没有真实模型请求；因此没有日期、语义事实、模型自主生成意识或自然语言理解通过证据。
- REAL_USE：NOT_RUN。没有真实来源、真实 Master 资料、24 小时运行或跨真实进程恢复。
- World：按 Master 指示由独立会话另审；本报告只指出 Core/Runner 授权交界的源码问题，不替代 World 结论。
- 没有 10 次跨重启请求幂等、双真实 P1/P2 子进程集成、授权撤回/过期/错误 scope/旧 fence 的 M2 端到端反例。
- 没有摘要 pending-question 正例、memory.refresh 并发同 slot single-flight、stale world read-set/CAS、semantic retry ModelCallRecord 端到端测试。

## 8. 建议退出条件

先修复 F1、F2、F3、F4、F5、F6；然后补 F7-F10 的独立反例。修复后必须重新冻结 fixture/fixture hash，重跑受影响 OFFLINE 测试、`go test ./...`、关键包 race、vet、两个入口构建，并把每次模型尝试的原始 output、校验错误、retry 次数和最终提交关联写进 report。真实模型验证由 root 协调，本次不替代。

参考资料（项目内）：
- [Acceptance.md](../docs/Acceptance.md)：A02、A06-A10、A16-A18 与证据分层。
- [Security.md](../docs/Security.md)：信任边界、SECRET 和 ProviderPolicy。
- [MemoryPolicy.md](../docs/MemoryPolicy.md)：24h slot、会话水位、Context budget。
- [DataFlow.md](../docs/DataFlow.md)：输入归档、动作组事务、read_set CAS。
- [ExecutionProtocol.md](../docs/ExecutionProtocol.md)：授权、permit、预算和 retry。
- [Context.md](../docs/DataStructure/Context.md)、[ContextManifest.md](../docs/DataStructure/ContextManifest.md)：实际 Context 和请求证据。
- [ModelCallRecord.md](../docs/DataStructure/ModelCallRecord.md)：调用记录字段和未结算计费。
- [ConversationSummaryDraft.md](../docs/DataStructure/ConversationSummaryDraft.md)、[ConsciousnessStateInput.md](../docs/DataStructure/ConsciousnessStateInput.md)、[RootBudget.md](../docs/DataStructure/RootBudget.md)：后台模型契约。

思路：先按 Acceptance 分层，分别复跑实现者命令与全量质量门；再从源码追踪“输入→快照→最终 wire→模型返回→语义校验→事务提交→诊断记录”的数据流，最后用独立 fake client/临时 SQLite 探针打穿 SECRET、权限、epoch、摘要和归档边界。只把可复现的动态结果写成 PASS；无法证明真实模型或真实使用的项目保持 NOT_RUN。
