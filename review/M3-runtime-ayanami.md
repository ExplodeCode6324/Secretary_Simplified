> 发布副本：仅对本机路径作规范化；原始报告本地保留于 `review/private/`。问题、结论和测试结果未改动。

结论

最终结论：需修复。

当前冻结复核快照中：

1. 编译、go vet、完整单元测试和 Runtime race 测试通过。
2. D01 scheduled_job.updated 已完成，不再将旧的 scheduled_job 事件登记问题列为当前阻塞项。
3. 独立安全行为探针发现 3 个真实失败：
   - 取消后 artifact scope 仍可通过。
   - 过期 permit 仍可执行 local notification。
   - 已发生未知效果的任务在取消后 ReconcileLocal 会被改写为 SUCCEEDED。
4. 因此不能以“14 项 Runtime 测试通过”作为 M3 完整验收通过。
5. 当前工作区在冻结快照后仍被并行 agent 修改过：runtime_control.go、executor/http.go 的 hash 已变化。以下结论严格对应冻结快照 `/tmp/m3_review_ayanami_latest_192908`；Codex 保存前应以最终工作区重新执行同样命令。

一、当前检查结果

冻结快照：

`/tmp/m3_review_ayanami_latest_192908`

静态文件 hash 已保存于：

`/tmp/m3_review_ayanami_latest_192908/hashes.txt`

检查结果：

1. `cd /tmp/m3_review_ayanami_latest_192908/src && go test ./... -count=1`
   - PASS
   - 当前 `src/tests/runtime_test.go` 中共有 14 个 `TestRuntime*` 测试，不是实现报告中的 10 个。

2. `go test -race ./tests -run Runtime -count=1`
   - PASS

3. `go vet ./...`
   - PASS

4. `go build ./cmd/secretary ./cmd/secretaryd`
   - PASS

5. 独立临时探针，未写入业务源码：
   `go test ./tests -run AyanamiProbe -count=1 -v`
   - FAIL：取消后的 artifact scope 未拒绝
   - FAIL：过期 permit 未拒绝 local notification
   - FAIL：取消后的 UNKNOWN reconciliation 改写成 SUCCEEDED
   - PASS：MutedAlarm 持久记录与过期停止
   - PASS：被撤销授权导致 DispatchRun 拒绝时，active budget 未泄漏

6. 文档检查：
   `python3 docs/checks/validate_docs.py`
   - BLOCKED：隔离快照缺少 docs checker 依赖 `checks/requirements-docs.txt`
   - 未将该结果伪报为通过。

未读取 resource 凭据，未连接 ELIZA，未发出真实响铃，未终止 Master 进程，未修改系统设置。业务源码未修改；仅创建了 `/tmp` 隔离快照和临时探针。

二、需修复问题

问题 M3-01：local permit 的 expiration/consumption/fence 检查不完整

严重性：高

证据：

`src/store/runtime_execution.go:490-520`

`checkLocalDispatchTx` 检查了：

- durable JobRun 是否为 RUNNING；
- fencing token；
- 当前 grant；
- grant revision；
- task cancel_generation。

但没有检查：

- `execution_permit.expires_at`；
- `execution_permit.consumed_at`；
- permit 是否绑定到当前 attempt；
- permit capability 是否与传入的 JobRun command 完全一致。

`RecordNotification` 和 `MutedAlarm` 分别在：

- `src/store/runtime_local.go:47-75`
- `src/store/runtime_local.go:117-168`

调用该检查，但没有使用 `ConsumePermitTx`。

`CheckArtifactScope` 位于：

`src/store/runtime_execution.go:438-464`

该函数只检查 grant、grant revision 和 path root，没有检查当前 run state、fence、cancel_generation 或 permit expiry。Executor 的 artifact 路径在：

`src/executor/executor.go:91-98`

直接调用该函数。

触发场景：

1. 领取并 Dispatch 一个 run。
2. 将 permit 等待到期，或将持久 permit 的 `expires_at` 改为过去时间。
3. 继续调用 `RecordNotification` 或 local artifact 路径。
4. 当前实现仍可能允许副作用。

独立探针实际结果：

`TestAyanamiProbeExpiredPermitMustDenyLocalEffect`
失败：过期 permit 仍允许 local notification。

另一个场景：

1. Dispatch 后取消 Task，使 `cancel_generation` 增加。
2. 调用 `CheckArtifactScope`。
3. artifact scope 仍返回成功。

独立探针实际结果：

`TestAyanamiProbeArtifactCancelMustDeny`
失败：取消后的 artifact scope 仍可通过。

修复建议：

- local side effect 必须接收明确的 permit ID，不应通过 `(run_id, fencing_token)` 查询“最新 permit”。
- 在同一个事务内检查：
  - permit 未消费；
  - permit 未过期；
  - permit.run_id、attempt_no、fence 完全匹配；
  - grant revision、task cancel_generation 完全匹配；
  - permit capability 与 durable command capability、command hash 一致；
  - artifact path、entity、operation 均在 permit scope 内。
- 将当前仅支持 world.update 的 `ConsumePermitTx` 扩展为通用 local permit consumption，或实现等价的 transaction-bound local commit。
- `CheckArtifactScope` 不应继续作为脱离 run/permit 的独立授权 API。

商议结论：需修复。当前实现的 fence 检查不能覆盖 local artifact 路径，permit 检查也没有覆盖 expiration 和 single-use 语义。

问题 M3-02：已派发任务的取消没有真正发送 cancel 请求

严重性：高

证据：

`src/store/runtime_execution.go:415`

`CancelTaskTx` 只查询并直接取消：

- QUEUED；
- CLAIMED。

RUNNING 不在查询范围内。

`src/executor/executor.go:152-153`

`Runner.Cancel` 仅调用 `Store.CancelTask`，没有：

- 保存或查找正在执行的 context；
- 向 CoreBridge 发送取消；
- 向 RemoteCore 发送 cancel 请求；
- 调用 capability 的 `supports_cancel` 适配器；
- 等待并收集最终效果证据。

触发场景：

1. Run 已进入 RUNNING。
2. Core 或外部能力正在执行。
3. 调用 cancel endpoint。
4. 数据库中的 Task 可能被标记为 CANCELLED，但实际执行 context 没有收到取消信号。

这违反设计中“已派发发出 cancel 请求，并继续收集最终证据”的要求。当前 cancel_ack 只能代表本地状态变更，不能代表取消请求已发出。

修复建议：

- Runner 维护 run_id/attempt_no/fence 到执行 context 的受控映射。
- `CancelTask` 对 RUNNING run 生成持久 cancel request，并向支持取消的执行器发出请求。
- cancel request 和 cancel_generation 必须持久化。
- 外部执行无法取消时，保留 CANCELLED 与实际效果之间的竞争记录，不得将其静默当作未执行。

商议结论：需修复。当前只实现了 queued/claimed cancel，不是完整的 dispatched cancel。

问题 M3-03：未知效果 reconciliation 会覆盖取消结果

严重性：高

证据：

`src/store/runtime_execution.go:467-488`

`ReconcileLocal` 流程为：

1. 检查 run 为 RESULT_UNKNOWN；
2. 调用 `VerifyTask`；
3. 生成 SUCCEEDED receipt；
4. 调用 `RecordReceipt`；
5. 再次调用 `VerifyTask`。

`RecordReceipt` 在：

`src/store/runtime_execution.go:382-395`

收到 SUCCEEDED 后会把 Task 改为 VERIFYING，最终 `VerifyTask` 会把 Task 改为 SUCCEEDED。

虽然 `VerifyTask` 在：

`src/store/runtime_local.go:238-245`

对已经 CANCELLED 的 Task 会避免立即覆盖，但 reconciliation 之后 `RecordReceipt` 又会重新进入正常成功状态路径。

触发场景：

1. local notification 已经写入持久状态。
2. Runner 在 receipt 持久化前崩溃。
3. recovery 将 JobRun 标为 RESULT_UNKNOWN。
4. Master 取消 Task。
5. ReconcileLocal 证明通知已存在。
6. 当前实现把 Task 最终改为 SUCCEEDED，丢失取消竞争语义。

独立探针实际结果：

`TestAyanamiProbeCancelledUnknownReconcileMustPreserveCancellation`
失败：reconciliation 将已取消 Task 改写为 SUCCEEDED。

修复建议：

- 若 reconciliation 发现 Task 已 CANCELLED：
  - 可以将 JobRun 标记为实际效果已观察；
  - 必须保留 Task=CANCELLED；
  - 写入独立的 cancel/effect race audit；
  - 返回类似 `CANCELLED_EFFECT_OBSERVED`；
  - 不得把 Task 改成 SUCCEEDED。
- Receipt 状态、Task 终态和“效果已发生”必须允许表达：
  - JobRun=SUCCEEDED；
  - Task=CANCELLED；
  - EffectObserved=true；
  - CancellationRace=true。

商议结论：需修复。设计明确要求取消竞争如实保存，不能把已经取消的任务恢复为成功。

问题 M3-04：local effect 使用调用者传入的 command，而不是严格使用 durable command

严重性：高

证据：

`checkLocalDispatchTx` 在：

`src/store/runtime_execution.go:490-520`

读取了 durable run，但只比较 state/fence/cancel generation，没有比较 command hash。

随后：

`src/store/runtime_local.go:47-75`

使用调用者传入的 `run` 参数中的 command arguments 创建 notification。

`src/store/runtime_local.go:117-168`

同样使用调用者传入的 command 控制 alarm 行为。

触发场景：

1. 调用者持有合法 run ID、attempt 和 fence。
2. 在内存中篡改传入 `contract.JobRun.Command`。
3. 将 notify.local 改成不同的 notification_key，或将 command capability 改为 alarm.play。
4. 当前 local store 方法仍可能基于篡改后的 command 执行。

修复建议：

- 对传入的 JobRun 与数据库 durable command 做 canonical command hash 比较。
- permit 中保存 command hash，并在 local effect transaction 中再次比较。
- capability 必须同时匹配：
  - durable run command；
  - permit capability；
  - local adapter capability。
- 不允许以“run_id + fence”作为 command authenticity 的完整证明。

商议结论：需修复。该问题与 permit/fence 语义直接相关。

问题 M3-05：ExpireMutedAlarms 只停止 alarm_session，没有完成 run/reconciliation 收尾

严重性：中高

证据：

`src/store/runtime_local.go:275-309`

`ExpireMutedAlarms` 只更新：

- alarm_session.state；
- revision；
- stopped_at；
- stop_reason。

没有更新：

- JobRun；
- ExecutionAttempt；
- ExecutorReceipt；
- Task；
- active_ms budget；
- UNKNOWN reconciliation evidence。

触发场景：

1. local alarm 已创建并进入 PLAYING。
2. Runner 在 receipt 保存前崩溃。
3. daemon 执行 ExpireMutedAlarms。
4. alarm_session 变成 STOPPED，但 JobRun 仍可能保持 RUNNING，Task 和 attempt 没有最终结果。

修复建议：

- 过期停止必须形成持久的 local effect receipt，或明确将 run 置为 RESULT_UNKNOWN。
- 通过同一个 reconciliation path settle active budget。
- 保存 max-duration 作为 cancellation/effect evidence。
- 对已经有最终 receipt 的 run 保持幂等，不重复写入。

商议结论：需修复或至少补充明确的恢复状态机。

问题 M3-06：DST gap 被跳过，但没有为被跳过 occurrence 建立显式 SKIPPED 记录

严重性：中

证据：

`src/store/runtime_schedule.go:71-84`

`NextOccurrence` 对不存在的本地时间直接找下一个有效日期。

`src/store/runtime_schedule.go:232-283`

ScheduleStep 只 materialize 当前 latest occurrence；missed 信息最多写入 `runtime.misfire.omitted_occurrences`，没有为 DST gap 生成对应 occurrence 的 SKIPPED JobRun 或独立 skip event。

设计要求见：

`docs/ExecutionProtocol.md:28`

“日历时间不存在时跳过该次并记事件”。

当前 DST 单元测试只验证：

- spring gap 后得到下一天；
- fall fold 选择第一次；
- 不重复 fold。

没有验证被跳过 occurrence 的持久审计记录。

修复建议：

- 为每个本地日期生成稳定 occurrence key。
- DST gap 生成 SKIPPED 记录或明确的 calendar-skip event。
- fold 只生成第一次 UTC occurrence，并持久化该选择。
- 为 daily/weekly/interval misfire 使用独立 oracle，不把 gap 与普通停机遗漏混为一类。

商议结论：需补齐持久审计语义和测试。

问题 M3-07：事件规则没有完整落实 root/origin/no-progress 防循环规则

严重性：中

证据：

`src/store/runtime_schedule.go:149-201`

事件调度按 seq 和 event_type 查询，但没有明确过滤：

- 当前 consumer 已处理的 causation；
- 同一 root 自己产生的事件；
- 无状态变化事件；
- last_effect_hash。

`rule_state` 写入时：

`last_effect_hash` 始终为 nil，`no_progress_count` 直接加一。

触发场景：

1. event job 监听 task.updated。
2. 自身创建 Task 后产生 task.updated。
3. 该事件被自身再次消费。
4. 当前实现依赖三次计数停止，而不是按 root/causation/no-progress 规则主动忽略。

修复建议：

- 持久化 consumer cursor，并使用 `(consumer_id, seq)` 幂等推进。
- 忽略同一 root 的无状态自触发事件。
- 用实际 effect hash 判断是否有进展。
- `no_progress_count` 只有在 effect hash 未变化时增加。
- 对已消费 event 保留 causation_id 证据。

商议结论：需补强。当前有冷却和三次上限，但不等价于设计要求的防循环协议。

三、重点验收项状态

授权 / permit / fence / cancel

状态：需修复。

已通过：

- 旧 fence dispatch 被拒绝；
- expired dispatched run 进入 RESULT_UNKNOWN；
- queued cancel 不再派发；
- grant revoke 导致 DispatchRun 拒绝；
- active budget 在 DispatchRun 事务失败时未泄漏。

未通过或未覆盖：

- permit expiration；
- permit single-use；
- local artifact path 的 fence/cancel；
- dispatched cancel 请求；
- cancel 后 UNKNOWN reconciliation；
- command hash 与 permit 绑定；
- world.update 的完整 policy scope 矩阵。

持久恢复未知效果

状态：有基础实现，但需修复。

已通过：

- local notification 产生后，Runner 崩溃模拟可进入 RESULT_UNKNOWN；
- ReconcileLocal 能依据持久 notification 避免重复创建。

需修复：

- 取消后的 reconciliation 会改写 Task；
- alarm expiration 没有完成 run/receipt 收尾；
- RemoteCore/CoreWork 路径未在本次 Runtime 行为集中完整故障注入验证。

DST fold/gap

状态：有条件通过。

已通过：

- America/New_York spring gap 跳到下一有效日；
- fall fold 选择第一次 UTC；
- 不重复 fold；
- 14 个 Runtime 测试和 race 测试通过。

未覆盖：

- DST gap 的 SKIPPED 持久事件；
- 多个 IANA 时区；
- weekly schedule；
- misfire 与 DST 同时发生；
- leap day、跨年、极端 UTC offset。

WAIT / 预算 / 固定 criteria

状态：有条件通过。

已通过：

- WAIT timeout 持久化；
- Replan 不修改 criterion hash；
- root budget 跨调用不重置；
- recurring notification occurrence 会绑定独立 notification key；
- artifact/notification criterion 具备 deterministic verifier。

未覆盖：

- condition already satisfied；
- competing registration；
- notification lost；
- restart rescan；
- old generation；
- wait event cursor 原子推进；
- world/source/master_confirmed 等 criterion 的完整执行路径；
- active budget 在所有失败、取消、超时、UNKNOWN 分支的账本一致性。

local muted alarm / notification / artifact scope

状态：需修复。

已通过：

- muted alarm 不触碰真实音频设备；
- alarm session 持久化；
- max duration 会停止 PLAYING session；
- notification delivery 与 acknowledgement 状态分离；
- artifact traversal 和既有 symlink component 被拒绝；
- artifact 写入使用临时文件、fsync、rename。

需修复：

- artifact scope 不重查 permit expiration/fence/cancel；
- local command 可被传入对象篡改；
- alarm expiration 未完成 run/receipt/budget 收尾；
- SafeArtifactPath 检查与最终 rename 之间仍存在 symlink TOCTOU 风险，尚未通过 openat/O_NOFOLLOW 类机制证明。

四、未覆盖和验收边界

以下不能由本次结果宣称通过：

1. LIVE_MODEL。
2. REAL_USE。
3. 真实 Master 资料、真实 source adapter。
4. 真实设备响铃、锁屏、电源状态和音频恢复。
5. 72 小时真实运行；当前只是虚拟时钟测试。
6. 30 个真实日历日模型回放。
7. 多进程真实崩溃恢复和外部 Core cancel。
8. 完整 A11-A16、A18、A20-A25 验收矩阵。
9. 文档检查器，因为隔离环境缺少 requirements-docs.txt 依赖。
10. 冻结快照之后 runtime_control.go、executor/http.go 的最新并行修改。

五、参考资料

1. 调度、取消、permit、WAIT、预算、未知效果协议：
file://<PROJECT>/docs/ExecutionProtocol.md

2. M3 验收矩阵：
file://<PROJECT>/docs/Acceptance.md

3. Runtime records 语义：
file://<PROJECT>/docs/DataStructure/RuntimeRecords.md

4. 已完成 D01 设计复核：
file://<PROJECT>/review/D01-scheduled-job-event.response.md

5. 实现 agent evidence：
file://<PROJECT>/review/M3-runtime-implementation-evidence.md

6. 本次隔离复核快照：
`/tmp/m3_review_ayanami_latest_192908`

六、复核思路

先读取 ExecutionProtocol、Acceptance 和 RuntimeRecords，按 A11-A16、A18、A21 的要求建立行为检查表；随后冻结源码快照并记录 hash，避免并行 agent 修改造成行号漂移。

动态验证分三层：

1. 基础工程层：go test、race、vet、两个入口构建。
2. 实现者测试层：验证现有 Runtime 测试确实可重复通过。
3. 独立 oracle 层：不复用实现者断言，分别构造 permit expiration、取消竞争、UNKNOWN reconciliation、muted alarm expiry 和 revoked dispatch 场景。

因此，本次不是“单元测试通过即验收”，而是：

工程检查：通过。

已实现行为：部分通过。

安全状态机：存在高严重性缺陷。

最终门槛：需修复。
