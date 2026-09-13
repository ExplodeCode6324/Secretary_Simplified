> 发布副本：仅对本机路径作规范化；原始报告本地保留于 `review/private/`。问题、结论和测试结果未改动。

M2 World 独立复核结论

结论：不通过“完整 M2 World 验收”。

实现者报告的 TestWorldPermitCorrectionAndHistory 确实通过，且基础路径中的缺许可、过期许可、错误 scope、许可复用拒绝、合法 ASSERT/CORRECT/RETRACT 和三版本历史均可复现。但我发现 3 个需要修复的高严重性问题，以及 2 个中严重性问题。Core/model/context/memory 未纳入本次结论。

一、具体发现

F-1 高危：ExecutionPermit 未绑定请求中的 JobRun

位置：

- src/core/work.go:32-39
- src/core/work.go:44-55
- src/store/runtime_execution.go:32-74

问题：

Core internal handler 接收 `in.Run` 和 `in.Permit` 后，仅使用 `in.Permit.ID` 消费许可，没有验证：

- `in.Permit.RunID == in.Run.ID`
- permit 的 attempt/fencing/cancel_generation 与提交的 run 一致
- 传入的 run 是否就是签发该 permit 时的权威 run

`ConsumePermitTx` 会检查 permit 自己关联的 `job_run`，但没有把该 job_run 与 HTTP 请求中提交的 `in.Run` 对比。

独立复现：

在隔离 DB 中建立：

- run A：许可 A 只允许 entity A
- run B：许可 B 只允许 entity B
- proposal：目标是 entity A

随后用：

- `run = run B`
- `permit = permit A`

调用 `/internal/v1/work`。

实际结果：

- HTTP 200
- world_fact_version 成功写入 1 条事实
- permit A 被消费
- permit B 未被消费
- B 的 scope 没有真正参与授权

这意味着一个拥有合法内部调用能力的 Runner/调用方，可以把一个 run 的许可套到另一个 run 上，绕过后者的 grant scope。

建议：

在 Core handler 和 `ConsumePermitTx` 中共同校验：

- permit.RunID 与提交 run.ID 完全一致
- permit 的 grant_revision、fencing_token、cancel_generation、attempt_no 与权威 JobRun 一致
- 从数据库重新读取权威 JobRun，并比较 command hash，而不是信任请求体中的 JobRun
- `FinishWorkTx` 也应使用权威 run，不应使用调用方任意提交的 run DTO

严重性：High。

F-2 高危：FinishWorkTx 未重新检查权威 JobRun fence/CAS

位置：

- src/store/work_repo.go:55-75

问题：

`FinishWorkTx` 只读取 `core_work`，检查：

```text
v.FencingToken == run.FencingToken
```

但没有读取 `job_run` 验证当前权威 fencing token，也没有检查 attempt_no。最终 UPDATE 虽然包含 `core_work.fencing_token` 条件，但没有检查 RowsAffected。

因此，旧 worker 在 job_run 的 fence 已经推进后，仍可能完成旧的 core_work receipt。

独立复现：

1. Claim/Dispatch 一个 run。
2. BeginWork。
3. 在隔离 DB 中推进该 job_run 的 fencing_token，模拟旧 lease 被新 worker 接管。
4. 使用旧 run 和旧 fence 调用 FinishWork。

实际结果：

```text
err=<nil>
core_work_state=SUCCEEDED
```

建议：

`FinishWorkTx` 应：

- 重新读取 job_run
- 要求 `job_run.attempt_no == run.AttemptNo`
- 要求 `job_run.fencing_token == run.FencingToken`
- 要求状态仍允许当前 attempt 完成
- UPDATE 使用 `run_id + attempt_no + fencing_token`
- 检查 RowsAffected 必须为 1
- 校验 receipt.RunID、receipt.AttemptNo、receipt.FencingToken 与权威 run 一致

World commit 路径中的 `ConsumePermitTx` 对旧 permit 有部分保护，但 Core receipt 状态本身仍存在旧 worker 覆盖风险。

严重性：High。

F-3 高危：WorldFact 的 ACTIVE 状态仍信任 proposal.Basis

位置：

- src/store/world_repo.go:85-92
- src/core/core.go:247-260
- src/world/world.go:16-20

问题：

World commit 根据 proposal 中的字符串字段直接决定事实状态：

```text
if p.Basis == "MASTER_EXPLICIT" {
    status = "ACTIVE"
}
```

`CommitWorld` 没有将 `Basis` 与当前授权主体、请求来源或 grant policy 重新绑定。Core 的 `WORLD_PROPOSAL` 路径确实在 `src/core/core.go:251-252` 检查了 `MASTER_EXPLICIT` 与 `MASTER_CLI`，但 WorldCommitService 本身没有这个防线。

因此，只要调用方能够构造一个合法 proposal 并取得 world.update permit，就可以把：

```text
basis = INFERENCE
```

伪造成：

```text
basis = MASTER_EXPLICIT
```

从而把本应是 CANDIDATE 的事实直接写成 ACTIVE。

同类问题还包括：

- `policy_revision` 没有与当前 grant 的 policy revision 比较
- `acceptance_rule` 由 proposal 内容直接填充
- WorldCommitService 没有重新判定该 proposal 是否确实来自 Master 明确输入

建议：

- `MASTER_EXPLICIT` 只能由经过认证的 Master 输入路径生成，不能由普通 proposal 字段决定
- WorldCommitService 应根据授权上下文重新计算 admission status
- 比较 proposal.PolicyRevision 与当前授权策略版本
- 不要把 proposal 中的 basis/authorized 类字段视为凭证
- 增加伪造 Basis、PolicyRevision、AcceptanceRule 的回归测试

严重性：High。

F-4 中危：revision=0 时允许 CORRECT/RETRACT 创建幽灵版本

位置：

- src/store/world_repo.go:61-92

问题：

当前逻辑允许：

```text
operation = CORRECT 或 RETRACT
expected_revision = 0
```

随后创建 revision 1：

- CORRECT：创建一个没有旧事实的“纠正”
- RETRACT：创建一个没有既存事实的 RETRACTED tombstone

独立复现：

使用合法证据、合法 permit，但不存在任何 fact head，提交：

```text
operation = RETRACT
expected_revision = 0
```

实际结果：

```text
err = nil
revision = 1
status = RETRACTED
```

这会破坏事实历史语义，也可能让读模型出现“从未存在但已撤回”的伪事实。

建议：

- ASSERT 才允许 `expected_revision=0`
- CORRECT/RETRACT 必须要求 `expected_revision>=1`
- CORRECT/RETRACT 必须确认对应 fact head 存在
- 增加空事实纠正、空事实撤回测试

严重性：Medium。

F-5 中危：World proposal 没有 request-level idempotency

位置：

- src/store/world_repo.go:11-29
- src/store/001_baseline.sql:51-55

问题：

`world_proposal` 的 `request_id` 没有 UNIQUE 约束，`PutProposalTx` 也没有做已有 request_id 的 hash 比对或返回既有 proposal。

当前幂等性主要依赖：

- proposal ID
- permit consumed_at
- command ledger

但相同 request_id 可以被写入多个不同 proposal ID。若同时改变 operation key/proposal ID，理论上可以绕过单个 command 的幂等保护并产生重复候选或重复事实尝试。

这不是本次已执行 probe 中的成功攻击，但属于 World idempotency 设计缺口。

建议：

- 对 `(principal_id, request_id)` 或至少 `request_id` 建立唯一语义
- 相同 request_id + 相同 canonical payload 返回原 proposal
- 相同 request_id + 不同 payload 返回 IDEMPOTENCY_CONFLICT
- proposal hash 应采用 canonical JSON，而不是依赖普通 Marshal 的当前表现

严重性：Medium。

二、已确认正确的部分

1. 证据对象基本绑定有效

位置：

- src/store/world_repo.go:15-25

`PutProposalTx` 会检查：

- evidence 非空
- object_ref 存在
- SHA256 与 object_ref 一致
- DataClass 与 object_ref 一致

因此基础 evidence object binding 是有效的。

未覆盖的证据边界：

- 未验证 OriginID 的真实 lineage
- 未验证 Locator 是否对应对象内容
- 未验证 proposal 的 evidence DataClass 是否符合 grant/provider policy
- 未执行对象文件被篡改后的 World proposal probe

2. 事实事务与 core_work receipt 的原子性结构正确

位置：

- src/store/world_repo.go:43-162
- src/store/world_repo.go:147-160
- src/core/work.go:51-56

成功路径是：

1. 消费 permit
2. 写入 fact version/head
3. 写入 world.updated event
4. 更新 proposal state
5. 在同一事务中写入 core_work receipt

我在隔离副本中注入 finalize failure，实际结果：

```text
world_fact_version: 0
proposal_state: PENDING
permit.consumed_at: empty
error: INJECTED_FINALIZE_FAILURE
```

说明 fact、proposal state、permit consumption、core_work receipt 确实会整体回滚。

结论：原子性设计本身通过该 probe，但仍受 F-1 的跨 run permit 绑定问题影响。

3. 基础 scope denial 有效

使用一个只允许 entity A 的 grant，提交 entity B 的 proposal：

```text
err = SCOPE_DENIED
facts = 0
```

说明正常使用匹配 permit 时，entity/predicate/operation scope 检查有效。问题在于 F-1 允许把其他 run 的 permit 搭配进来。

4. 独立 Core token/socket 结构存在

结构复核：

- src/cmd/secretaryd/main.go:75-84：client.token 与 internal.token 必须不同
- src/cmd/secretaryd/main.go:127-130：Core public socket 和 internal socket 使用不同 token
- src/cmd/secretaryd/main.go:168-169：Runner 使用 internal socket 和 internal token

因此“Core internal 独立 token/socket”在当前组合代码中已实现。

但 `core.Service.InternalHandler()` 本身不带认证，认证依赖外层 `transport.Serve`。建议补一个实际 Unix socket 集成测试，验证：

- client token 访问 internal socket 被拒绝
- internal token 访问 public socket 被拒绝
- 错误 token 被拒绝
- handler 不会被错误地直接挂载到无认证 HTTP server

三、实际测试

全部在隔离副本执行，未修改原始业务源码、设计文件、持久配置，也未读取 resources 凭据、连接 ELIZA 或触发外部副作用。

已执行：

```text
go test ./tests -run '^TestWorldPermitCorrectionAndHistory$' -count=1 -v
PASS
```

```text
go test ./... -count=1
PASS
```

```text
go vet ./...
PASS
```

```text
go test -race ./... -count=1
PASS
```

独立安全 probes：

```text
TestReviewWorldPermitRunBindingProbe
PASS，但实际日志确认 F-1：
HTTP 200，facts=1，permit A consumed，permit B 未消费
```

```text
TestReviewStaleFenceFinishWorkProbe
PASS，但实际日志确认 F-2：
FinishWork err=nil，core_work_state=SUCCEEDED
```

```text
TestReviewWorldRetractionAtRevisionZeroProbe
PASS，但实际日志确认 F-4：
revision=1，status=RETRACTED，err=nil
```

```text
TestReviewWorldAtomicFinalizeRollbackProbe
PASS：
facts=0，proposal_state=PENDING，permit_consumed=""
```

```text
TestReviewWorldTrueScopeDenialProbe
PASS：
err=SCOPE_DENIED，facts=0
```

四、尚未覆盖的边界

- 两个独立进程同时提交相同 fact 的真实并发测试
- 不同 fact_id、相同 predicate/key 的冲突组完整行为
- CORRECT 是否应产生冲突、旧版本状态是否符合最终设计
- evidence object 文件内容被篡改后的 World path
- OriginID/Locator provenance 校验
- 相同 request_id、不同 proposal_id 的实际重复提交
- grant revocation 或 grant revision 在 Dispatch 与 Commit 之间变化
- 不同 receipt_key、重复 Core 请求及结果未知恢复
- 通过真实 Unix socket 的 Core public/internal token 隔离
- Core/model/context/memory 模块；本次没有把它们的状态混入 World 结论
- 完整 M2 acceptance、真实模型调用、ELIZA 或外部 provider 行为

五、建议修复优先级

P0：

1. 将 permit 与提交 run 做强绑定，修复 F-1。
2. 在 FinishWorkTx 中重新校验权威 job_run attempt/fence，并检查 RowsAffected，修复 F-2。
3. WorldCommitService 不得信任 proposal.Basis/PolicyRevision 决定 ACTIVE，修复 F-3。

P1：

4. 禁止 revision=0 的 CORRECT/RETRACT，修复 F-4。
5. 为 world proposal 增加 request-level idempotency，修复 F-5。
6. 增加 Core internal/public Unix socket 集成测试。

参考资料

- World service：
  file://<PROJECT>/src/world/world.go#L16-L20

- World repository：
  file://<PROJECT>/src/store/world_repo.go#L11-L176

- Permit validation：
  file://<PROJECT>/src/store/runtime_execution.go#L31-L74

- Core work handler：
  file://<PROJECT>/src/core/work.go#L17-L88

- Core work persistence：
  file://<PROJECT>/src/store/work_repo.go#L11-L76

- Claimed World test：
  file://<PROJECT>/src/tests/world_test.go#L12-L95

- Token/socket composition：
  file://<PROJECT>/src/cmd/secretaryd/main.go#L75-L169

- World design constraints：
  file://<PROJECT>/docs/DataFlow.md#L22-L26
  file://<PROJECT>/docs/Security.md#L9-L15
  file://<PROJECT>/docs/DataStructure/WorldFact.md#L31-L35

思路

本次采用“声明路径复核 + 隔离副本运行 + 独立负向 probe”的方式：

1. 先确认实现者声称的 DB happy path 真实通过。
2. 阅读 World、permit、Core work、receipt、schema 和 socket composition 的数据流。
3. 针对 permit/run binding、旧 fence、伪造 admission 字段、revision=0 retract、事务回滚分别构造最小 probe。
4. 只把实际执行输出作为事实，不把其他 agent 的 Core/model/context/memory 状态混入。
5. 将“已证明安全”“已发现缺陷”“尚未覆盖”分开。

最终判断：World 基础事实历史和事务回滚路径已有可用实现，但当前不能称为完整 M2 验收通过。最先修复 F-1、F-2、F-3。( _ _ )
