Issue1 AUD02/AUD04 本地独立增量终审报告（Ayanami / DeepSeek 独立轮次，只读审计 + 亲跑实证）

════════════════════════════════════════
一、判定
════════════════════════════════════════
限定 PASS —— 在下列限定与未跑清单之外，未发现 mustfix；8 项授权范围行为全部经源码核对 + 亲跑定向 race 测试实证通过。

限定：
1. 依据协议 = review/issue1-AUD02-AUD04-design-correction.response.md (sha256 46134f38) + review/issue1-memory-reconcile-clarification.response.md (sha256 ba1c876c)。已核对：无首稿额外 extension 复活（仅沿用已注册 runtime.cancellation，shape 严格 {effect_observed, cancel_generation}，contract.go:274-279 强制 len==2 且 cancel_generation>=1）；GET 路径纯只读，无 GET 暗写 CoreWork / 无新 HTTP 接口 / 无 DDL 变更。
2. 本 PASS 覆盖的工件为下列 SHA 对应的最终版本；报告在审计期间 1e1d40c6→33e9ed93→a11779e1 收敛，最终版与源码哈希已对齐（issue1-runtime-checks.json 佐证，其记载的 11 条资产哈希我逐条复核一致，含 docs 三条）。
3. 未跑项见第五节，均不冒充 PASS。

════════════════════════════════════════
二、范围 8 项逐项判定（源码证据 → 实证测试）
════════════════════════════════════════
1. 30s HTTP 取消后 5s 纯持久收尾 —— PASS
   core/work.go:183 settleCtx=WithTimeout(WithoutCancel(r.Context()),5s)，仅包裹 ReconcileMemoryWork/FinishWork；模型、RefreshSlot、source adapter 全程挂在会取消的 r.Context()。实证：TestIssue1CancelledSocketWorkSettlesAfterRunnerUnknown（真实 Unix HTTP + fake model，取消后 CoreWork 落 FAILED no-effect）。

2. 同代 UNKNOWN 有证据 FAILED 进入原预算且结算一次 —— PASS
   runtime_execution.go:373（FINISHED 仅在收据到达前 run 为 RESULT_UNKNOWN 才可重试）、:406（dispatch_state!=FINISHED 一次性 active_ms 守卫）、:347-349（跨代回执只审计不覆盖）、:360+464（CANCELLED 不重试）。实证：同上测试断言 activeBefore==activeAfter（不重复结算）、QUEUED/attempt=1、memory 5min backoff、model calls==1。

3. exact slot 同代 root 恢复 + GET 只读 —— PASS
   work.go:28-31 GET→GetCoreWork 纯读；runtime_memory.go:98-187 单 Write tx 内校验 core_work 同 run/attempt/fence/hash + slot 合法整数 + consciousness_snapshot.slot 精确匹配 + epoch object（SHA 校验）+ task.root_id==DeriveID(slot root)，经 FinishWorkTx 落 SUCCEEDED，随后走既有 RecordReceipt+Verifier。实证：TestIssue1CommittedMemoryCrashGapReconcilesAfterReopen（SQLite trigger 注入结算失败→重启后恢复；future slot 拒收；wrong fence 返回 STALE_FENCE）。

4. briefing WriteObjects ref+receipt 原子 —— PASS
   work.go:158-174 objects.Put + FinishWorkTx 同一 WriteObjects tx（flock→mutex→BEGIN IMMEDIATE，objects.go:74-141）；失败回滚后清空 Artifacts/EffectObserved=false 保守未决。实证：TestIssue1LostBriefingResponseUsesAtomicArtifactWithoutModelReplay（HTTP 提交后掐断，重启后从原子工件解 UNKNOWN，model calls==1）。

5. duplicate POST 不重模型 —— PASS
   work_repo.go:46-52（已有收据同代幂等返回，不同结果 WORK_ALREADY_SETTLED 拒绝）、:36-45（仅 FAILED+no-effect+严格新 attempt 允许重绑）、:53-56（同代无收据 WORK_IN_PROGRESS，item 8）。实证：TestIssue1DuplicateSocketPostAndLateCancelledReceipt（duplicate Execute 被拒，model calls==1）。

6. 取消 Task 不复活 —— PASS
   work_repo.go:124/143-147（仅 Task CANCELLED 时才接受晚到回执并附 runtime.cancellation 审计）、runtime_execution.go:323-327、:360、:137-139、ClaimRunReady TASK_CANCELLED、DispatchRun TASK_NOT_DISPATCHABLE。实证：同测试 task 保持 CANCELLED、marker 存在、runner 后续 Step 不复活。

7. 旧 FAILED 跨代拒绝 —— PASS
   remote_query.go:51-53（跨代重绑定必须 SUCCEEDED+EffectObserved，其余 not-known）；runtime_memory.go:118（旧 fence STALE_FENCE）。实证：同测试 remote.Query(attempt+1/fence+1) known=false；crash-gap 测试 wrong fence 返回 error。

8. bounded source max+1 + FIFO nonblock 拒收不推进 cursor —— PASS
   source_file.go:15 O_RDONLY|O_NONBLOCK、:21 IsRegular 拒收、:26-33 LimitReader(limit+1) 实测读上限、超限 INPUT_TOO_LARGE；work.go:115-128 仅在受限读取成功后进入 adapter（进入后失败置 uncertainEffect）。实证：TestIssue1SourceReadActualLimit / TestIssue1SourceGrowthAfterOpenAndNonRegular / TestIssue1OversizedSourceSocketDoesNotAdvanceCursor（64MiB；source_state payload、record 计数、source.synced 事件数全部不变）。

未来 slot / wrong fence 反例：已核对 runtime 补齐到位 —— 测试 issue1_runtime_test.go:345-365（runtimeDB trigger 注入 + slot++ 拒收 + fence++ 拒收），报告 a11779e1 行 29 如实记载；两者均在我亲跑的 PASS 内执行。

════════════════════════════════════════
三、实际测试证据（亲跑，2026-09-14 11:09–11:12 HKT，go1.25.6 darwin/arm64）
════════════════════════════════════════
R1 go test -race ./tests -run '^TestIssue1' -count=1 -v → PASS (5.886s) EXIT=0
   7 tests：GatePersonalAuthorization、CancelledSocketWork、OversizedSource、QueuedBackgroundCancellation、CommittedMemoryCrashGap、LostBriefingResponse、DuplicateSocketPost
   注：触发注入路径的日志实际出现于 -v 输出：CORE_WORK_SETTLEMENT_FAILED run=b7928ca4-6f84-4656-9c27-d35931125002 attempt=1 fence=1（证明注入真实生效而非空跑）
R2 go test -race ./core -run '^TestIssue1Source' -count=1 -v → PASS (1.292s) EXIT=0（2 tests）
R3 复跑（针对最终测试版本 20d9dc70）：R1+R2 再 PASS（5.820s / 1.311s）+ TestStaleWorkerCannotCommitReceipt -v → PASS (0.19s)；运行前后工件 pre/post hash 全等 → HASH_STABLE
R4 报告 line 36 声明命令原样复跑：go test -race ./core ./store ./executor ./tests -run 'TestIssue1|TestStaleWorkerCannotCommitReceipt|TestRuntimeRemoteQuery' -count=1 -timeout=90s → FINAL_EXIT=0（core 2.181s；store [no tests to run]；executor [no test files]；tests 7.041s）——与报告/JSON 的 "store PASS (no matching tests)" 记载一致，耗时差异为机器波动。

════════════════════════════════════════
四、关键 SHA（最终核对，与 issue1-runtime-checks.json 逐条一致）
════════════════════════════════════════
src/core/work.go                 ee6f6a7f605cd4beabd1baf894a1b057f09356f099384684c7b2d592e78caa90
src/core/source_file.go          9f6467a42edab896beb39c278cbb8c0b77f7e367bd149a890b904f1c4cccf371
src/store/work_repo.go           0402e7b6ab27ebe817dd3c92480cd0dc3ee4c9e7b2dc476954c7cd3821653f41
src/store/runtime_memory.go      8755427a33197737bccf265454d0404a5939fe926ce61f751706d28f886e0252
src/store/runtime_execution.go   e3ce7e4a022920dcc6d137b0d6144a8788c9b76e4c75f41947b7d0ea52ad2d6d
src/executor/remote_query.go     110debc4c5a8e620c0935fb41777e4645cc46e90b5b63174f71513c8a1a3fe70
src/tests/issue1_runtime_test.go 20d9dc7046d378742fd77a96a785b260ccd97198a999456a6007801ead9d3fda
src/core/source_file_test.go     d125d8aaa858cb2320ffb6bbae411acac9433b630b8700a6c720f5b945bd539f
docs/ExecutionProtocol.md        8bbc6780...（与 checks.json 一致）docs/Acceptance.md dbc2e76f...、docs/README.md 6c8e27ee...
report issue1-runtime.md         a11779e1c1d546d6b1887315cbb1d4dc876c99ab4d75a256d81689df0dc26f4f
checks  issue1-runtime-checks.json 23a5d6709c0f650bf6048aa0aa77dec498c18c5372622aff8085f3e9f315fd0c
生产源码 hash 自审计开始至收口全程未变（ee6f6a7f 等自 T3 至收口恒定）；测试文件在最终版本 20d9dc70 上完成全部实证。

════════════════════════════════════════
五、未跑项（明示，不冒充 PASS）
════════════════════════════════════════
- 全仓 race/vet、release smoke、soak/final2 二小时 —— 按授权与协议禁止/由根代理另行协调（报告 line 36 亦如此声明）；旧 soak 与其二进制/原 hash 未触碰。
- 范围外套件：./store issue1_storage_test.go、./diagnostics issue1_cancel_test.go、runtime_ayanami_probe 系列、其余既有套件 —— 未跑（注意：报告命令中 ./store 实为 no tests to run，属命令设计如此，非漏跑）。
- docs/checks 验证器（报告自称 DOC_ONLY PASS）—— 我未复核。
- 字面 30s HTTP 超时未做单独 e2e：协议禁止新增时长门槛；5s 收尾路径已以 1s 超时 + 真实 Unix HTTP 取消等价触发实证。
- 无真机断电/OS power-loss 测试（报告自我限定为受控故障注入，我认可该限定）。
- 报告在审计期间由 runtime 侧持续补齐；若调用方存档版本与我核对的 a11779e1 不同，以新 hash 重对齐即可（无阻塞）。

════════════════════════════════════════
六、参考资料
════════════════════════════════════════
- 协议：review/issue1-AUD02-AUD04-design-correction.response.md、review/issue1-memory-reconcile-clarification.response.md
- 实现报告：reports/implementation/issue1-runtime.md（a11779e1）；哈希清单 reports/implementation/issue1-runtime-checks.json
- 源码：src/core/work.go、src/core/source_file.go、src/store/work_repo.go、src/store/runtime_memory.go、src/store/runtime_execution.go、src/executor/remote_query.go
- 测试：src/tests/issue1_runtime_test.go、src/core/source_file_test.go、src/tests/work_security_test.go:41（TestStaleWorkerCannotCommitReceipt）
- 注册校验：src/contract/contract.go:274-279；文档：docs/ExecutionProtocol.md:129

思路：先读两份裁决协议锁定收口版规则（排除首稿 extension/GET 改写）→ 静态逐项比对 8 项范围（含注册 shape 与 GET 只读）→ rg 定位定向测试名与反例 → 亲跑两条指定 race 命令 + 报告声明命令 → 以 pre/post 哈希锁定"测试工件未被中间修改"→ 最后对齐报告与 checks.json 的源码哈希，未跑项全部明示。无 mustfix，收口为限定 PASS。
