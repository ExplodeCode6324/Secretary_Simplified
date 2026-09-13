> 发布副本：仅对本机路径作规范化；原始报告本地保留于 `review/private/`。问题、结论和测试结果未改动。

# M2 World 修复复查与 Restore/Frozen 集成复核

reviewer_model: gpt-5.6-luna
provider: opencode-go
范围：只读源码 + `/tmp` 隔离副本安全测试；未读取 resources 凭据、未调用模型、未连接外部服务、未响铃、未修改业务源码或系统设置。

## 结论

F1-F4 的主要外部可达路径已修复，F5 在当前 public/API 调用链上足够；不能据此称完整 M2 验收通过。

当前判断：

- F1 permit/run 混用：已固定。
- F2 stale fence 写 receipt：已固定。
- F3 model 生成 WORLD_PROPOSAL 伪造 MASTER_EXPLICIT：对当前可达 Core/API 路径已固定；repository primitive 仍保留一个内部防御边界，见 R1。
- F4 CORRECT/RETRACT expected_revision=0：已固定；CLI restore/frozen runner 集成通过。
- F5：当前没有发现经 public `/v1/actions` 或 Core model path 可达的重复 World effect；`PutProposalTx` 自身仍不是独立幂等 API，但当前调用点是 Core 内部事务路径，不应单凭该内部方法报告外部漏洞。

## F1：ExecutionPermit 与 JobRun 绑定

结论：已固定。

固定证据：

- `src/core/work.go:35-45` 先验证 JobRun 和 ExecutionPermit schema，并强制：
  - `permit.RunID == run.ID`
  - `permit.FencingToken == run.FencingToken`
  - `permit.Capability == run.Command.Capability`
  - `Store.ValidateIncomingWork(...)` 通过后才允许 BeginWork。
- `src/store/work_repo.go:99-145` 重新从 durable `job_run`、`execution_permit` 读取并 Decode，比较 durable command 与入参 command、durable permit 与入参 permit，并检查：
  - 当前 run state 为 RUNNING
  - attempt/fence 一致
  - permit 未过期
  - grant 当前有效且 capability 允许
  - grant revision 与 permit 一致
  - cancel_generation 与 task 当前值一致。
- `src/store/runtime_execution.go:31-74` 的 World commit 事务仍再次以 durable permit 校验 proposal hash、grant、fence、cancel_generation、scope，并在同一事务消费 permit。

独立隔离 probe：

```text
TestFollowupForgedCommandAndPermitRejected/command
status=403 core_work=0

TestFollowupForgedCommandAndPermitRejected/permit
status=403 core_work=0

TestIncomingPermitBoundToExactRun
PASS，run ID 不匹配时 403，core_work=0
```

因此上一轮的 F1 复现：用 run B 搭配 permit A 成功提交 World fact，当前已不能复现。

## F2：旧 fence 写入 receipt

结论：已固定。

固定证据：

- `src/store/work_repo.go:56-66` 在 FinishWorkTx 内重新查询 `job_run WHERE state='RUNNING'`，比较实际 attempt/fence，并验证 receipt 的 RunID/AttemptNo/FencingToken。
- `src/store/work_repo.go:84-95` UPDATE 后检查 RowsAffected 必须为 1，否则返回 STALE_FENCE。

独立隔离测试：

```text
TestStaleWorkerCannotCommitReceipt
PASS
```

该测试先推进 job_run fencing_token，再用旧 receipt 调用 FinishWork；调用被拒绝，core_work 保持 RUNNING。

## F3：伪造 MASTER_EXPLICIT / PolicyRevision

结论：当前外部可达路径已固定；保留内部 primitive 防御边界 R1。

固定证据：

- `src/core/core.go:104-118` 的模型 Process 调用 `s.apply(..., false)`。
- `src/core/core.go:258-269` 的 WORLD_PROPOSAL 路径固定 `RequestID`、`PolicyRevision=1`，且 `trusted=false` 时强制 `Basis=INFERENCE`。
- `src/core/core.go:294-325` 的 Typed 路径才调用 `s.apply(..., true)`，并构造认证为 `PrincipalID=master`、`Origin=MASTER_CLI` 的输入。
- `src/store/world_repo.go:11-20` 拒绝 `PolicyRevision != 1` 和零版本 CORRECT/RETRACT。
- public `/v1/actions` 在 `src/core/http.go:68-80` 进入 Typed；外层 public socket 由 transport token 认证。

R1（低/中：内部防御边界，当前不可达外部漏洞）：

- `src/store/world_repo.go:91-98` 的 repository 仍根据 `p.Basis` 直接决定 ACTIVE/CANDIDATE；它没有在 repository 层再次禁止 `MASTER_EXPLICIT`。
- 目前 `PutProposalTx` 的非测试调用点只有 `src/core/core.go:269`，其 model path 已强制 INFERENCE；Typed path 是明确的 Master 入口。
- 因而一个直接运行同一 Go 进程、绕过 Core 入口调用 `PutProposalTx` 的内部 caller 仍可构造 `Basis=MASTER_EXPLICIT`。这属于内部 primitive 的 defense-in-depth 缺口，不是当前 public/API 可达 World 权限绕过。

建议（非本轮外部阻断项）：让 repository 接受明确 admission mode/source 参数，或拆分 `PutInferenceProposalTx` 与 `PutMasterProposalTx`，避免未来新增调用点重新信任 Basis 字段。

## F4：CORRECT/RETRACT expected_revision=0

结论：repository 规则已固定；Restore/frozen 集成也通过。

固定证据：

- `src/store/world_repo.go:15-20`：PolicyRevision 必须为 1；非 ASSERT 且 expected_revision=0 返回 CONFLICT。

独立隔离 probe：

```text
policy: POLICY_REVISION_MISMATCH
correct-zero: CONFLICT
retract-zero: CONFLICT
```

### Restore 集成实际结果

使用只含 synthetic 数据的隔离 backup，实际执行：

```text
go run ./cmd/secretary restore --config <source-config> --backup <backup> --target <restored>
```

结果：

```text
restore=PASS
```

恢复后状态查询：

```text
frozen_file=true
config_frozen=true
fresh_client_token=true
client_internal_distinct=true
grant_capabilities=[]
queued_state=QUEUED
attempt=0
core_work=0
```

随后真实启动隔离 Runner：

- `runner.sock` 成功 ready。
- 保持 `execution_frozen` 文件和 `config.execution_frozen=true`。
- 等待 3 秒后仍为：`QUEUED / attempt=0 / core_work=0`。
- 未发生 claim、dispatch 或 effect。

这确认了“恢复后可启动，但默认冻结；新 token；新 grant 无 capability；不会自动执行历史队列”的集成行为。

补充：第一次使用过长临时路径启动 Runner 时命中程序自身的 Unix socket <104 bytes 检查；改用短 `/tmp` 路径后按预期启动。该结果是路径保护，不是 Restore 失败。

### verify --backup 正负测试

正向：

```text
verify_positive=PASS
status=COMPLETE
```

负向：复制 backup 后篡改 `secretary.sqlite`，再执行 verify：

```text
BACKUP_DATABASE_HASH_MISMATCH
verify_negative_exit=1
```

对应实现：

- `src/cmd/secretary/main.go:91-112` restore/verify 分支。
- `src/cmd/secretary/main.go:360-380` fresh token 与空 capability grant。
- `src/store/backup.go:204-309` VerifyBackup 的 manifest、DB hash、SQLite integrity、foreign key、migration、object 校验。
- `src/store/backup.go:311-355` Restore 先 VerifyBackup，再创建 execution_frozen 并复制数据。
- `src/cmd/secretaryd/main.go:92-98,181-185` Runner 以文件/config 双重冻结判断，冻结时不进入 scheduler/executor step。

## F5：request-level 幂等边界最终意见

结论：当前外部可达路径足够；没有发现可达的重复 World effect。

调用链证据：

- `src/core/http.go:68-80` public `/v1/actions` 只进入 `Service.Typed`。
- `src/core/core.go:294-325` Typed 使用固定的 Master 输入，并将全部 action 处理放入 `AcceptTyped`。
- `src/store/core_repo.go:232-269`：
  - 对 `(principal_id, request_id)` 计算 canonical semantic hash。
  - 同 request + 不同内容返回 IDEMPOTENCY_CONFLICT。
  - 同 request + 同内容进入既有 receipt/turn 路径。
  - `finishTurnTx` 与 action apply 位于同一事务。
  - 已 COMMITTED 的 turn 不会再次 apply。
- `PutProposalTx` 的调用搜索只有：
  - 定义：`src/store/world_repo.go:11`
  - Core 内部：`src/core/core.go:269`
  - 测试直接调用：`src/tests/world_test.go:32`

独立隔离 probe：

```text
TestFollowupTypedSameRequestOneEffect
same typed request: items=1 receipts=1
```

因此不能把 `PutProposalTx` 自身没有 request-id 幂等自动等同为外部漏洞。当前它是 Core 的事务内 primitive：

- model retry 未完成事务会回滚 proposal 和 command ledger。
- 同一 Typed request 会在 request_receipt/turn 层截断重复 apply。
- public `/v1/actions` 没有独立直接写 world_proposal 的公网入口。

仍保留的未来边界：

- 若以后新增直接调用 `PutProposalTx` 的入口，或允许跨 request 重放同一 proposal，应增加 request_id 唯一/同 hash 复用规则。
- 该建议属于 defense-in-depth，不是当前 F5 外部可达漏洞。

## 当前仍未关闭或未充分覆盖的边界

### R2：入站 JobRun 不是完整 DTO 等值绑定（低/中，非当前 World effect bypass）

`src/store/work_repo.go:116-124` 比较 durable command 和 permit，但没有比较 durable JobRun 的全部字段，例如 TaskID、JobID、OccurrenceKey 与入参 run 的一致性；`src/store/work_repo.go:137-142` 还使用入参 `run.TaskID` 读取 task。

可构造的源级路径：拿合法 run/permit，替换入站 JobRun.TaskID 为另一个 cancel_generation 相同的 task。ValidateIncomingWork 的 command/fence/permit 检查仍可能通过。

对 World 路径而言，后续 `ConsumePermitTx` 在 `src/store/runtime_execution.go:47-66` 从 durable permit 的 run/task 重新取值，因此未发现可借此越权写 World fact 的路径。它仍是 receipt/audit 与非 World internal capability 的 DTO 完整性边界。建议未来比较完整 durable JobRun 身份字段；不把它混入当前 F1 失败结论。

### R3：证据校验仍是 object_ref metadata 级别

`src/store/world_repo.go:24-30` 校验 object_ref 的 sha256/data_class，但没有在 proposal admission 时调用 `ReadObject` 验证 blob 实体，也不验证 OriginID/Locator lineage。

当前对象存储自身有 `ReadObject` hash 校验，且 repository 没有独立公网入口；若数据库 metadata 与 blob 在外部篡改/损坏后直接进入 World proposal，仍需更强的 evidence provenance probe。属于证据完整性边界，未被本轮 F1-F4 修复覆盖。

### R4：冲突完整行为尚未被新增测试覆盖

`TestWorldPermitCorrectionAndHistory` 验证 ASSERT/CORRECT/RETRACT 三版本，但没有验证不同 fact_id、同 entity/predicate/key 的 conflict_group、CONTESTED head 与回滚行为。现有 conflict 实现位于 `src/store/world_repo.go:99-151`，本轮没有修改、也没有把它宣称为完整验收通过。

## 实际测试清单

在 `/tmp/secretary-world-fix-review.ePUKWk/src` 隔离副本：

1. 目标修复测试：

```text
go test ./tests -run '^(TestIncomingPermitBoundToExactRun|TestStaleWorkerCannotCommitReceipt|TestWorldPermitCorrectionAndHistory)$' -count=1 -v
PASS
```

2. Store backup negative/restore tests：

```text
go test ./store -run '^(TestBackupConcurrentRestoreFrozenAndTamper|TestBackupCorruptDatabaseRejected|TestManifestClassificationAndRequiredFields)$' -count=1 -v
PASS
```

3. 独立 follow-up probes：

```text
TestFollowupForgedCommandAndPermitRejected
PASS；伪造 command/permit 均 403，core_work=0

TestFollowupTypedSameRequestOneEffect
PASS；items=1，receipts=1

TestFollowupWorldRepositoryRejectsPolicyAndZeroCorrection
PASS；policy mismatch、CORRECT zero、RETRACT zero 均拒绝
```

4. Restore/runner/verify：

```text
restore=PASS
runner_socket=1
frozen_file=true config_frozen=true fresh_client_token=true
client_internal_distinct=true grant_capabilities=[]
queued_state=QUEUED attempt=0 core_work=0
verify_positive=PASS
verify_negative_exit=1
BACKUP_DATABASE_HASH_MISMATCH
```

本轮未重复全量 go test/vet/race；上一轮同一修复快照已通过全量 `go test ./...`、`go vet ./...`、`go test -race ./...`，本轮重点只复核修复点和集成边界。

## 参考资料

- Core internal work handler：
  file://<PROJECT>/src/core/work.go#L21-L170
- Work validation / receipt CAS：
  file://<PROJECT>/src/store/work_repo.go#L53-L145
- Permit transaction authority：
  file://<PROJECT>/src/store/runtime_execution.go#L31-L74
- World repository：
  file://<PROJECT>/src/store/world_repo.go#L11-L183
- Core admission / Typed path：
  file://<PROJECT>/src/core/core.go#L104-L120
  file://<PROJECT>/src/core/core.go#L258-L325
- Public actions route：
  file://<PROJECT>/src/core/http.go#L68-L80
- Typed transactional idempotency：
  file://<PROJECT>/src/store/core_repo.go#L232-L269
- Restore/VerifyBackup：
  file://<PROJECT>/src/store/backup.go#L204-L355
- CLI restore/provision：
  file://<PROJECT>/src/cmd/secretary/main.go#L91-L112
  file://<PROJECT>/src/cmd/secretary/main.go#L360-L380
- Frozen Runner：
  file://<PROJECT>/src/cmd/secretaryd/main.go#L92-L98
  file://<PROJECT>/src/cmd/secretaryd/main.go#L181-L185

## 思路

本轮没有依赖异步审查。采用：

1. 直接读取修复后的 Core、Work、World、Typed、CLI restore 和 backup 源码。
2. 在隔离副本运行 F1/F2/World 目标测试。
3. 写入临时 probe 验证 command/permit 伪造、Typed 同 request 重放、policy/zero revision 负向。
4. 用 synthetic queued DB 实际执行 restore，验证 fresh credentials、empty grant、frozen config/file、Runner socket 与无 claim。
5. 对 backup DB 做临时副本篡改，验证 `verify --backup` 正向和 hash mismatch 负向。
6. 将当前外部可达结论与内部 primitive 的 defense-in-depth 边界分开，不把未审的模型/context/memory 模块混入。

最终建议：F1-F4 可标记为修复验证通过；F5 当前外部路径可接受，但保留 R1/R2/R3/R4 作为后续硬化和覆盖边界。不能称完整 M2 World 验收通过。
