Ayanami 复核完成，以下为最终报告（隔离副本实测，业务树零写入）。

━━━━━━━━━━━━━━━━━━━━━━━━
D12 Runtime/World 独立增量复核 — M3/M4 闭合裁决
━━━━━━━━━━━━━━━━━━━━━━━━

一、裁决结论（具体 M3/M4）
- M3 闭合：PASS（8/8 目标测试 + 7/7 独立反例探针）。Job/Command→Task/Run/Attempt/Permit/Receipt/notification/alarm/REPLAN/WAIT 全链 join 不降；authorization 扩展 merge 保留 grant 绑定；SUBMIT_ARTIFACT 伪低 class 被拒（ARTIFACT_REFERENCE_MISMATCH），实际值取 DB 持久 ObjectRef。
- M4 闭合：PASS。WorldProposal 入账时与证据 class join（只升不降）→ WorldFact 版本构建 join 旧版本；v2=max(v1,proposal) 实证；分类 extension 不丢；旧缺标版本 commit 明确 OUTPUT_CLASS_UNKNOWN 且零写入。
- mustfix：无（本增量范围内未发现实证缺陷，也未发现需改代码的现有契约缺口）。
- 证据分层声明：全部为 OFFLINE/合成 canary 动态证据；真实 CLI/模型腿 NOT_RUN，不作为也不代替 D12 安全证明（维持设计 §12：M1–M9 全绿+docs 同步前不得称 D12 安全 PASS）。

二、特别独立审两项（均实测，非采信）
1. rtSaveRun 不反写旧 Command / 固定 command hash 稳
   探针 TestAyanamiD12M3ReceiptJoinKeepsCommandHashStable 实测：PERSONAL run 收 SECRET 收据后，Run/Task/Receipt class 升至 SECRET，而 run.command 仍为 PERSONAL；attempt 表 command_hash == 收据前 == 收据后（rtSaveRun 内置 rtInherit(command, command, before.command) 只与自身旧值 join，不吸收收据 class）；command_ledger.payload_hash 前后不变。固定 hash 稳定成立。
2. PutProposalTx 证据升级 / 旧 caller 被拒 / 工作路径读 DB
   实测：证据对象 SENSITIVE + proposal 声明 SYNTHETIC → PutProposalTx 入库时已 join 为 SENSITIVE（存储 proposal_hash == ValueHash(GetProposal 回读值)）；旧 caller 内存副本 commit → PROPOSAL_CONFLICT 拒绝；GetProposal 回读副本 commit → 成功，v1= SENSITIVE。源码核对生产路径：src/core/work.go:65-75 确为 GetProposal→ValueHash→CommitWorldAtomic；permit 的 proposal_hash 亦取自 DB（src/store/runtime_execution.go:225）。root 陈述与实际一致。

三、实际命令/结果/源码 hash
环境：macOS 15.7.4，go1.25.6；隔离副本 /tmp/d12-review.64UMC6（业务树未改，src/** 哈希复核前后一致，17/17 与 reports/implementation/D12-runtime-checks.json 声明 MATCH）。
1. `go build ./...`（GOPROXY=off, -mod=readonly）→ exit 0
2. `go test -run 'TestD12' ./tests -count=1 -v` → 8/8 PASS（ok 1.111s）
   日志 sha256：8bed9d956b4fd087505d77075ac5c0c2885f26fdb92a689e720de6be8cadd527
3. 回归关键探针 `go test -run 'TestD08|TestWorldPermitCorrectionAndHistory|TestWorldCandidateDoesNotOverrideAndConflictGroupUsesPreferenceKey|TestAcceptanceA12CancellationFourStages|TestAcceptanceA13ProcessExitsAfterEffectBeforeReceipt|TestRemoteFailurePreservesEffectAndRetryCeiling' ./tests` → 10/10 PASS（日志 f9fe2d73c99df2c356a9c6d881e7a209ddeeb3d709f275df928e820dc323c4c5）
4. `go test -run 'TestAyanamiResidual|TestAcceptanceA04' ./store` → PASS（日志 e304ca7b886e6b2a009212ba0991538dc7bc2189c54784a3431c5ecc3e070e2f）
5. 自写反例探针 `go test -run 'TestAyanamiD12' ./tests -count=1 -v` → 7/7 PASS（日志 ce0d3f0396756f47bf3fdbe1575c676ac9d41484e8cc1f2ea4e5d0dde7c0886d；探针源 2136b386c458424338adca704cad67c2b18ed2052872eb90e7d76d6a0991075f）
   探针覆盖：req PERSONAL+低引用不降/高引用升/伪造低拒；Receipt 高 class 只升不反写命令；Attempt/Permit/WAIT join；REPLAN merge 保 grant+固定 criterion_hash；证据升级旧 caller 拒绝；world 版本不降+legacy 零写拒绝。
源码 hash（与 JSON 声明一致）：runtime_classification.go c29a9142…；runtime.go cb6129aa…；world_repo.go ef67a0cc…；executor.go 0a2575d3…；runtime_d12_test.go cc974499…（全 17 项 TOTAL_MISMATCH=0，前后两次校验）
范围声明：按令未跑全仓 test/race/vet；本会话跑法为非 -race（D12 报告中的 -race 子集为其自身证据，我未复现）。

四、观察项（非 mustfix，不设新门槛）
- 严格扩展登记实测生效：未登记 extension 键被 UNREGISTERED_EXTENSION 拒绝（contract.go:286 白名单：classification/authorization/artifacts/context.retrieval/source_sync/calendar_skip/cancellation/misfire）——本探针初稿因加自定义键被拒，反证“单一严格登记、程序独占”成立。
- legacy 缺标 Task：WAIT 与 SUBMIT_ARTIFACT（含携带 req class 时）一律在入口 rtInherit(t,t) 处 OUTPUT_CLASS_UNKNOWN 拒绝，零字节改动，不存在“伪写低 class”路径——比设计最低要求更保守。
- SUBMIT_ARTIFACT 对任何与 DB 不符的声明（低或高）均拒绝，严于“不信任更低值”，无安全洞。
- 出站/收据元数据继承持久 class（notation/receipt artifacts）与 metadata fault 下 effect=true+RESULT_UNKNOWN+无盲 retry 由 D12 目标测试再证 PASS。

参考资料
- review/D12-output-class-final-contract.response.md、review/D12-injection-oracle-clarification.response.md（有效设计基线）
- reports/implementation/D12-runtime-checks.json（哈希声明已 17/17 对账）
- src/store/runtime_classification.go:12-49；runtime.go:226-315,337-367；runtime_control.go:42-107,207-229；runtime_controls.go:13-196；runtime_execution.go:76-167,282-468,592-642；runtime_wait.go:33-98；world_repo.go:11-48,58-141；executor/executor.go:96-155；core/work.go:62-84；contract/classification.go:8-134；contract/go(registry):286
- 隔离副本证据：/tmp/d12-review.64UMC6/（d12_run1.log、regression_tests.log、regression_store.log、ayanami_probe_run2.log、src/tests/ayanami_d12_m3m4_probe_test.go）

思路
先对账声明哈希锁定冻结态，再在 rsync 隔离副本跑既有 D12 八测与限定回归（D08 currentRun/REPLAN 固定 criteria、A12/A13 取消+效果不确定、world permit），随后自写 7 个 M3/M4 反例探针直接测“只升不降/不反写/旧 caller 拒绝/legacy 零写”。两处初稿探针被产品更严行为拒绝（extension 白名单、legacy 控制入口拒绝），按“查实际不采信”修正探针后全绿——故无 mustfix，仅留上述观察项；真实 CLI 与 Core/contract/Memory 面维持另会话/另验。

未遇 429，未换 reviewer/provider，未触资源凭据/真实模型/响铃，未委派。后续两轮可留给 Master 质询或要求我将本报告落盘至 review/ 新编号文件。
