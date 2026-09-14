# M3 transport/CLI 终审裁决 — DeepSeek v4.1 Flash（resume1）

reviewer_model=deepseek-v4.1-flash / provider=opencode-go；未委派、未换模型、未运行全仓 test/race/vet（另：模型侧未出现 429）。上轮收口失败的原始错误按指示保留在文末。

---

## 0. 裁决摘要

**条件通过（CONDITIONAL PASS）**。已实测范围内（UDS 传输层、认证、脱敏、错误状态码、typed routes、分页/cursor、派生 ItemID、重试/冲突、严格反序列化）未发现阻断级缺陷，无 must-fix；但 **CLI 端到端链条、restore 实机（双冻结/新凭据/不 claim）、UNKNOWN query 实机均未执行**，连同 2 项口径确认，须补做后方可定稿为最终通过。未查项一律不计入 PASS。

---

## 1. 实际执行的证据边界

**已执行（真实输出见本会话记录）：**

1. 快照与构建：`rsync src/ → /tmp/m3ds/snap/src`（评审时点快照，供并发编辑隔离）；`go version` = go1.25.6 darwin/arm64；`go build ./cmd/secretary` + `./cmd/secretaryd` → BUILD_OK。
2. 定向测试（快照内，`-count=1`，无 race/vet/全仓）：
   - `go test ./transport/ -run 'TestPublicErrorNeverLeaksValues|TestDuplicateJSONAndOversizeRejected'` → 2 PASS
   - `go test ./diagnostics/ -run 'TestDoctorDoesNotInventHeartbeats|TestDoctorEpochAndBackupHistory'` → 2 PASS
   - `go test ./store/ -run 'TestBackupConcurrentRestoreFrozenAndTamper|TestBackupCorruptDatabaseRejected|TestManifestClassificationAndRequiredFields'` → 3 PASS
3. 临时合成环境 `/tmp/m3ds/env/A`（secretary init + 实跑 `secretaryd core` 与 `secretaryd runner` 两守护进程），真实 UDS 负向探针 `bash probe1.sh`（P1–P21）、`bash probe2.sh`（T1–T13），全部输出已逐条记录。

**未收集/未执行（见 §4，不得当 PASS）：** `go test ./tests/` 8 项定向用例（后台进程 `proc_50de9f02f3d3` 启动后未轮询，结果未知）；全部 CLI 链条与 restore 实机探针；UNKNOWN query 实机探针。

---

## 2. 逐重点结论

**① PublicCode 脱敏 / 错误 HTTP ≥400 — 通过（实测）**
- `transport.Reply` 对非 nil 错误强制 ≥400（transport.go:33-36）；错误体仅含 public code（transport.go:57）。
- 实测 P1/P2/P16 无 token/错 token → 401 `UNAUTHENTICATED`；P3/P4 双向跨 token 拒绝 → 401；P5 内部 token → 200；P7/P17/P20/P21 未知对象 → 404 `NOT_FOUND`；P12/P13/P14/P15 严格拒绝 → 400。全部错误体无验证器文本、路径、SQL 泄漏（P7/P8 的 `result` 为零值 DTO，仅空字段）。
- 观察 O2/O3/O5（§3），均不构成泄漏或 <400。

**② typed routes / public 分页与 cursor/limit — 通过（实测）**
- `GET /v1/items?limit=2` → 200、`next_cursor`；续页 → 200、第二页收尾（T5/T6）。limit 越界（999/-1）→ 400（P8/P9）；cursor 非 base64 / 合法 base64 但非 JSON → 400（T8/T8b）；cursor 携带不同 filter（domain=life）→ 400（T7，query-hash 绑定）；新事件后重用旧 cursor → 409 `CONFLICT`（T9，快照序号绑定）。
- 见观察 O1（core 侧 `limit=abc` 静默默认，与 runner 侧不一致）。

**③ requestID 派生 Typed ItemID / 重试与冲突 — 通过（实测）**
- 同 request 重试 → 201、同一派生 ID `1bcfd571-… `、revision 1（T1/T2）；同 request 不同 payload → 409 `IDEMPOTENCY_CONFLICT`（T3）；后续修改后重放原 request → 仍返回原始 revision 1 的不可变响应（T13）。实现 core.go:418-421 + core_repo.go:230-262（含语义 hash 归档归一）。

**④ typed actions --file 严格 — 契约/代码级通过，实机未覆盖**
- `ActionProposal` 及 CREATE_JOB payload 等均为 `additionalProperties:false`（contracts.schema.json:3478-3852 已读）；CLI `actionFile` 经 `contract.Decode`（main.go:367-377）。实机 CLI 负向（未知字段/缺文件）**未执行** → 未覆盖。

**⑤ CLI trigger CAS/版本、notifications ack、schema 搜索、--json/默认输出、verify smoke/verify-backup — 代码级核对完成，CLI 实机 0 条执行**
- TriggerJob：request+revision 必填、CAS 不符 → `REVISION_CONFLICT`、同 request 重试经 occurrence_key 返回原 JobRun（runtime_control.go:42-107）；AckNotification 幂等回执（runtime_control.go:382-408）；memory search 强制 schema_version=1（core/http.go:62-75）；VerifySuite（verify.go:16-98）；VerifyBackup 严格清单/hash/FK/migration 校验（backup.go:204-310）。
- UDS 面已实测：ack 未知 id → 404（P17）、缺 schema_version → 400（P18）、runner limit=201 → 400（P19）、trigger/cancel 未知对象 → 404（P20/P21）。
- **CLI 实机链条（jobs trigger/ack/runs cancel/--json/verify smoke/verify --backup）全部未执行。**

**⑥ restore 新独立 tokens、空 cap grant、双冻结、Runner 启动但不 claim — 未覆盖（实机未执行）**
- 仅代码级：Restore 先验备份并写入目标 `execution_frozen` 文件（backup.go:311-356）；CLI restore → 新 32B 随机 client/internal token + 空 `CapabilityIDs` grant + `config.Frozen=true`（main.go:128-143, 417-437）；守护进程冻结门（secretaryd main.go:92-98）使 Step/claim 循环整体短路。
- 既有测试 `TestBackupConcurrentRestoreFrozenAndTamper` 本轮执行 PASS（其细部断言未逐行复核）。**实机“启动但不 claim”按要求不得写 PASS。**

**⑦ Doctor 真实 epoch / UNKNOWN — 通过（测试实测），CLI 输出未采集**
- 两个定向测试 PASS；DoctorWithEpoch 由 config epoch 驱动（diagnostics.go:28-85；cmd main.go:113-120；core/http.go:16-19），未记录遥测显式 `UNKNOWN`（diagnostics.go:29,55-71），缺 backup.latest.json 时 `LastBackup=UNKNOWN`。

**⑧ run cancel 返回 Task revision — 代码级：CAS 口径正确；实机未执行**
- `/v1/runs/{id}/cancel` → 解析 run.TaskID → 用 **Task revision** CAS（executor/http.go:89-102；runtime_control.go:204-226；rtSaveTask 递增 revision，runtime.go:335-360）；响应体仅 `{"cancel_ack":bool}`，与 docs/ExecutionProtocol.md:40 一致；CLI 在发出前读取 Task revision 做 CAS（main.go:209-223）。404 负向实测通过（P21）。
- **口径边界**：若“返回 Task revision”要求响应内携带 revision，当前实现未满足 → 需协调方确认。

**⑨ Core UNKNOWN query 持久结果 / 不得重发未知效果 — 代码级通过；实机未执行**
- reconcile 只查询 `/internal/v1/work/{run_id}` 并以新 ID/fence 生成对账 receipt，绝不重发（remote_query.go:14-96；本地能力已剔除）；`RESULT_UNKNOWN` 不可被 claim（ClaimRunReady 仅取 QUEUED；runtime_execution.go:79-164）；过期 lease 仅未派发（PREPARED）回 QUEUED，已派发 → RESULT_UNKNOWN（99-121）；自动重试仅限 FAILED+effect_observed=false+DISPATCHED（323-357）。相关测试在未收集的后台命令中 → 不作为证据。

---

## 3. 观察与轻微问题（不阻断，含位置与严重性）

| ID | 严重性 | 事实（实测/代码） | 位置 |
|---|---|---|---|
| O1 | 低 | core 侧 `limit=abc` → HTTP 200（Atoi 错误被忽略，回落默认 50）；runner 侧同样输入返回 400 `INVALID_LIMIT`。建议统一为 400 | core/http.go:45,54 vs executor/http.go:44-51 |
| O2 | 低 | 错误码保真度：`INVALID_LIMIT`/`INVALID_CURSOR`/`INVALID_ARGUMENT` 等被 PublicCode 折叠为 `INVALID_SCHEMA`/`REQUEST_FAILED`（实测 P8/P9/T7/T8/T8b/P14/T12）。状态码正确，仅分类损失 | transport.go:167-178 |
| O3 | 低 | 未声明路由返回 Go 默认纯文本 `404 page not found`（P6），非 JSON 信封；客户端解析需容错 | core/http.go:15 mux 默认行为 |
| O4 | 信息 | `RemoteCore.Ready` 仅做 socket dial（250ms），未调用 `/internal/v1/health`，注释与行为不完全一致 | executor/remote_ready.go:9-20 |
| O5 | 信息 | 错误响应的 `result` 含零值 DTO（P7 Item、P8 RuntimePage），无数据泄漏，消费方应以 `error.code` 为准 | transport.go:27-61 |

---

## 4. 未覆盖清单（如实列，均不计 PASS）

1. **后台测试结果未收集**：`proc_50de9f02f3d3` = `go test ./tests/ -run 'TestPublicTypedItemsPaginationAndNoPartialWrites|TestPublicSearchRequiresSchemaVersion|TestRuntimeControlCASAndPagination|TestRuntimeCancelBeforeDispatch|TestRuntimeDispatchedCancelSignalsExecutor|TestRuntimeUnknownLocalReconciliation|TestRuntimeKnownNoEffectRetryBounded|TestRuntimeRemoteQueryRebindsEvidence'` — 启动后未轮询，结果未知（其中分页/搜索断言已由本轮 UDS 实测 T1–T13 独立覆盖，其余两项不覆盖）。
2. **CLI 实机链条全部未执行**（本轮仅实机执行过 `secretary init`）：jobs create/trigger（CAS/版本/幂等/冲突）、notifications ack（CLI）、runs cancel（CLI+Task revision 链路）、doctor 输出与 `--json` 对比、`verify --suite smoke`、`verify --backup`、backup/restore CLI、`actions --file` 负向。
3. **restore 实机未执行**：新凭据独立性、空 cap grant、双冻结、恢复后 Runner 启动不 claim、EXECUTION_FROZEN 拒绝路径。
4. **Core UNKNOWN query 实机对照未执行**；其“不重发”结论目前为代码级。
5. P11 探针因 URL 编码错误作废（已由 T8/T8b 替代）。
6. 快照边界：测试/构建基于 `/tmp/m3ds/snap/src`（评审时点 07:44 的 src 快照）；live 工作树可能被 D08/D09 等其他 agent 继续修改；D08/D09 未纳入本裁决。

---

## 5. 必须修复与放行条件

- **must-fix：无**（已实测范围内未发现阻断缺陷）。
- **建议修（不阻断）**：O1、O2、O3。
- **放行条件（补做后方可定稿）**：①CLI 链条实机（§4.2）；②restore 实机（§4.3）；③UNKNOWN query 实机 + 后台测试结果补收；④两条口径确认：未声明路由是否须 JSON 信封；`runs cancel` 是否须在响应内携带 Task revision。

---

## 6. 合规与原始错误保留

- 全程未触碰 resources/key、ELIZA、受测真实模型；无响铃、无系统配置变更；业务库只读；仅在 `/tmp/m3ds` 临时 synthetic 环境操作；未向仓库写入任何文件（快照/环境均在 /tmp）；未输出任何 token 值。
- **原始错误（保留）**：上轮收口请求经 Hermes → provider 失败：`HTTP 400 (Console Go): MissingSessionID — Request is missing x-opencode-session and cannot be routed efficiently`（见 opencode.ai/docs/go）。本轮为 resume1；无 429，无 provider 切换。
- 本回复即 `review/M3-transport-cli-deepseek-final.response.md`（resume1）内容，由 Codex 负责另存。

( _ _ ) 结论一句话：**传输与 typed 面实测干净，CLI/恢复/UNKNOWN-query 三段实机缺口仍在——条件通过，补做后复核即可定稿。**
