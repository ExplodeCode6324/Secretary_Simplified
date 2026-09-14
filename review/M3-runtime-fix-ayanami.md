> 模型限定：本报告来自 Ayanami 使用 `gpt-5.6-luna` 的历史复核。Master 最新指定必须由 `deepseek-v4.1-flash` 复核，因此本报告仅为 model-qualified 历史证据，不能充当最终复核裁决；原问题、结论与测试结果保留，待 DeepSeek 重新检视。

> 发布副本：仅对本机路径作规范化；原始报告本地保留于 `review/private/`。问题、结论和测试结果未改动。

M3 Runtime 修复后独立复核

复核结论：有条件通过，仍需修复 1 个 P1 完整性缺陷，并保留 1 个高危文件安全边界未覆盖项。

本报告对应当前冻结快照：
/tmp/m3_review_ayanami_current2_194635

快照时间：2026-09-13T19:46:26Z
源码 hash：/tmp/m3_review_ayanami_current2_194635/hashes.txt
业务源码未修改；探针只写入隔离快照。

一、工程检查

以下命令在加入临时 residual probe 前执行，均通过：

1. cd /tmp/m3_review_ayanami_current2_194635/src && go test ./... -count=1
   PASS；tests、store、contract、memory 等全部通过。

2. go test -race ./tests -run 'Runtime|AyanamiProbe' -count=1
   PASS。

3. go vet ./...
   PASS。

4. go build ./cmd/secretary ./cmd/secretaryd
   PASS。

5. go test ./tests -run 'AyanamiProbe|RuntimeCalendar|RuntimeEvent|RuntimeRecurring|RuntimeKnown' -count=1 -v
   PASS。
   五项独立 probe 均通过：artifact cancel、expired permit、cancelled UNKNOWN reconciliation、muted alarm expiry、rejected dispatch active-budget rollback。

6. python3 docs/checks/validate_docs.py
   BLOCKED：隔离环境缺少 checks/requirements-docs.txt 依赖；未将其报为通过。

process integration JSON 已读取：
review/M3-runtime-process-integration.json
其中 real_data=false、real_model=false。该文件是实现者报告，没有随源码提供可独立重放的进程 fixture 命令，因此只能作为 OFFLINE 自述证据，不能替代本次复核或 REAL_USE。

二、M3-01 至 M3-07 状态

M3-01 授权、permit、fence、local expiry、artifact scope
结论：核心修复通过；文件 scope 仍有高危 TOCTOU 未覆盖。

证据：
- src/store/runtime_execution.go:470-527：artifact 检查持久 permit、hash、consumed_at、grant revision、expiry、run fence、task cancel_generation，并在同一事务消费 permit。
- src/store/runtime_execution.go:553-600：local notification/alarm 检查 command hash、expiry、capability、attempt、grant revision、cancel_generation，并在同一事务消费 permit。
- src/store/runtime_local.go:47-80、120-174：副作用写入与 permit 检查处于同一 DB transaction。
- src/tests/runtime_ayanami_probe_test.go：原独立过期 permit 与取消 artifact 反例均 PASS。

残余问题：SafeArtifactPath 仍是 Lstat 检查后由 MkdirAll/CreateTemp/Rename 使用路径。见 src/store/runtime_local.go:16-45、src/executor/executor.go:102-129。若本机存在恶意并发进程，可在检查与 rename 之间替换父目录 symlink，造成 scope 绕过。该竞态未以可靠时序 probe 证明，但属于真实文件安全边界。

修复建议：使用 dirfd/openat/O_NOFOLLOW 或等价的不可替换目录句柄；不要将路径字符串预检查作为最终 containment 保证。

M3-02 cancel context / RemoteCore context
结论：实现修复通过，动态覆盖有限。

证据：
- src/executor/executor.go:31-33、75-80、163-191：Runner 保存 task_id 到 context.CancelFunc，Cancel 后取消 active context。
- src/executor/remote_core.go:13-26：Execute 接收 context。
- src/transport/transport.go:104-131：HTTP request 使用 NewRequestWithContext，Unix dial 继承 context。

未覆盖：没有独立 fake Core 阻塞调用后并发 Cancel、验证 HTTP handler 收到 context cancellation 的专门测试。进程 integration 报告也未包含该场景。

M3-03 UNKNOWN reconciliation 与 Task CANCELLED
结论：修复通过。

证据：
- src/store/runtime_execution.go:282-294：effect_observed 且 Task 已取消时写入 runtime.cancellation evidence。
- src/store/runtime_execution.go:411-427：收到 receipt 后若 currentTask 已 CANCELLED，保持 Task=CANCELLED。
- src/store/runtime_local.go:530-551：ReconcileLocal 的前后 VerifyTask 不再将取消任务恢复为 SUCCEEDED。
- 独立 TestAyanamiProbeCancelledUnknownReconcileMustPreserveCancellation PASS。

M3-04 canonical command hash / durable binding
结论：command binding 修复通过；criteria hash 持久完整性仍需修复，见 M3-R01。

证据：
- src/store/runtime.go:25-30：hash 先 JSON normalize 后计算。
- src/store/runtime_execution.go:553-599：local effect 对比 durable command hash，并检查 capability/attempt/permit。
- src/store/runtime.go:173-181：immediate command ledger 的 criterion semantic hash 排除程序生成的 criterion ID，重试不会因 ID 改变而冲突。

M3-05 muted alarm expiry / effect receipt / budget / verifier
结论：主流程已实现，有条件通过。

证据：
- src/store/runtime_local.go:289-349：session STOPPED、run RESULT_UNKNOWN、Task NEEDS_ATTENTION 在同一 transaction 完成。
- src/store/runtime_local.go:353-360：事务提交后补 effect receipt，并触发预算 settlement 与 VerifyTask；若补 receipt 中途失败，下一扫描仍会匹配 STOPPED + RUNNING/RESULT_UNKNOWN。
- 独立 muted alarm persistence/expiry probe PASS。

未覆盖：独立 probe 只断言 session STOPPED，未严格断言 run、receipt、active_ms budget 和下一扫描幂等；真实音频设备未触碰。

M3-06 DST gap audit
结论：修复通过。

证据：
- src/store/runtime_schedule.go:335-360：next_due 推进与 scheduled_job.skipped 同 transaction。
- src/store/runtime_schedule.go:340-356：稳定 DeriveID、重复事件 payload 校验、runtime.calendar_skip 扩展。
- src/contract/contract.go:143、202-218：scheduled_job.skipped 与严格扩展注册。
- src/tests/runtime_test.go:544-588：DSTGapHasAtomicAudit 验证 gap 日期、origin、before/after next_due、重复扫描不新增。
- docs/ExecutionProtocol.md:99、docs/DataStructure/TypeRegistry.md:65 等已同步 D06 规则。

未覆盖：多 IANA 时区、weekly gap、跨年/极端 offset、长时间停机与 gap 同时 misfire。

M3-07 event loop / consumer cursor / effect hash / causation
结论：实现静态修复通过，独立动态覆盖不足。

证据：
- src/store/runtime_schedule.go:186-253：忽略 scheduler.calendar 自身审计、忽略同 job runner/scheduler 事件、按实际 effect hash 维护 no-progress、consumer_cursor 与调度推进同 transaction。
- src/store/runtime.go:81-95：event job task created 记录 causation_id。
- src/tests/runtime_test.go:348-372：已有 cooldown 持久化测试，但未完全覆盖自触发与三次相同 effect hash 的停止。

未覆盖：显式 self-generated event、相同 effect hash 三次、重启后 consumer_cursor 补扫的独立 oracle。

三、当前新增 P1 问题

M3-R01 固定 criterion hash 的完整性检查不完整
严重性：P1；需修复。

具体问题：rtSaveTask 只验证 criteria 内容 hash 等于持久 criterion_hash，但没有验证传入的 t["criterion_hash"] 仍等于 before["criterion_hash"]；SQL UPDATE 也没有检查 RowsAffected。内部路径若只篡改 criterion_hash：

1. rtHash(t["criteria"]) 仍等于旧 criterion_hash，检查通过；
2. UPDATE ... WHERE id=? AND criterion_hash=? 影响 0 行；
3. 函数忽略影响行数并继续写 task.updated ChangeEvent；
4. 返回 nil，造成持久 Task 与事件载荷不一致。

证据：src/store/runtime.go:315-332。

独立反例：隔离文件
/tmp/m3_review_ayanami_current2_194635/src/store/ayanami_residual_probe_test.go

命令：
cd /tmp/m3_review_ayanami_current2_194635/src && go test ./store -run AyanamiResidualCriterion -count=1 -v

结果：FAIL，原始错误：
criterion hash mutation was accepted

修复建议：
- 在 SQL 前显式要求 rtStr(t["criterion_hash"]) == rtStr(before["criterion_hash"])；
- 检查 RowsAffected，必须恰为 1，否则返回 STALE_TASK 或 CRITERION_CONFLICT；
- 不得在 DB 更新未生效时追加 after 事件；
- 同样审查 rtSaveRun 的 RowsAffected 与 stale snapshot 保护。

商议结论：需修复。当前普通 Replan 与 recurring criteria 测试通过，不足以证明 hash 字段本身不可篡改。

四、固定、未固定、未覆盖汇总

已固定：
- M3-01 核心 permit expiry/capability/attempt/hash/fence/cancel 与同 tx consumption；
- M3-02 Runner active cancel context、RemoteCore context propagation；
- M3-03 cancelled Task 保留取消并记录 effect evidence；
- M3-04 durable command canonical hash；
- M3-05 alarm STOPPED/UNKNOWN/receipt/budget/verifier recovery 主路径；
- M3-06 D06 scheduled_job.skipped 原子 gap audit；
- M3-07 self-event/filter、effect hash、consumer cursor、causation 主路径；
- D02 recurring notification occurrence binding；
- known no-effect retry bounded/backoff/identity。

仍未固定：
- M3-R01 criterion_hash field mutation + zero-row update acceptance；
- artifact path Lstat-to-Rename symlink TOCTOU。

未覆盖：
- RemoteCore 真正阻塞后 HTTP context cancellation；
- alarm expiry 后 receipt/budget/Verifier 的严格断言与第二次扫描幂等；
- M3-07 self-trigger、same-effect 三次停止、consumer cursor restart replay；
- world policy permit 的完整 scope/predicate/operation/consumed 矩阵；
- 多时区/weekly DST、真实两小时运行、真实模型、LIVE_MODEL、REAL_USE；
- process integration JSON 的独立重放命令和完整原始日志。

五、参考资料

- file://<PROJECT>/docs/ExecutionProtocol.md
- file://<PROJECT>/docs/Acceptance.md
- file://<PROJECT>/review/D06-calendar-skip.response.md
- file://<PROJECT>/review/M3-runtime-process-integration.json
- file:///tmp/m3_review_ayanami_current2_194635

最终门槛：有条件通过；在 M3-R01 修复并补跑完整受影响矩阵前，不应宣称 M3 Runtime 完整验收通过。
