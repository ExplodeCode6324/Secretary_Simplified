清理完成（仅我启动的 core=41664、runner=41932；/tmp/m3ds 与 release/soak 进程非我创建，未触碰）。第三 run 已核实为 memory_refresh 槽位（非异常）。以下为本轮终审报告，供 Codex 保存至 review/M3-cli-e2e-deepseek.response.md。

═══════════════════════════════

# M3 CLI 实机终审补测 — DeepSeek v4.1 Flash（本轮）
reviewer_model=deepseek-v4.1-flash / provider=opencode-go；未委派、未换模型、未出现 429（未触发停止条件）。按 review/M3-cli-e2e-deepseek.request.md 只补 §4.2–4.4 缺口。

## 0. 裁决摘要
**条件通过（维持）**。CLI 主干链实机证据充分，已测部分未发现阻断缺陷、无 must-fix；但 **restore 实机、verify smoke/verify-backup、UNKNOWN-query 定向测试三项本轮未执行**（会话预算耗尽），一律不计 PASS。补做后方可定稿最终通过。

## 1. 实际执行（真实输出）
环境（新隔离）：/tmp/m3e —— snapshot=/tmp/m3e/src（src rsync 快照+build），bin=/tmp/m3e/bin/{secretary,secretaryd}，env A=/tmp/m3e/env/A（fixture 默认配置，未读 resources/真实 API）。
构建：go1.25.6 darwin/arm64；`go build ./cmd/secretary`、`./cmd/secretaryd` → BUILD_OK；快照文件哈希链 b0ddab7d6db6a84a…；binary sha256 97d0311b…/d1d6ed47…；repo commit 2b753f2。
守护进程：secretaryd core(pid 41664)、runner(pid 41932)；收尾按 PID 结束（仅我创建者；他人 /tmp/m3ds、release/soak 进程未动）。

a) `actions --file` 严格性（对 core.sock）
- 合法 CREATE_ITEM → exit 0，committed_operation_keys=[cli-item-1]
- 顶层未知字段 → exit 2，`INVALID_CONTRACT ActionProposal: … additional properties 'unexpected' not allowed`
- payload 未知字段 → exit 2，`… at '/payload': additional properties 'unknown_field' not allowed`
- 缺文件 → exit 2，`open …/does_not_exist.json: no such file or directory`

b) jobs create / trigger CAS + 同 request 幂等（对 runner.sock）
- `jobs create --file jobA.json` → exit 0，committed cli-job-a
- trigger 错 revision(999) → exit 3 CONFLICT
- 正确 revision=1 → 建 run；同 request(TR1) 重放 → 同一 run 72981fd6…（两次解析一致）；TR2 → run 79261ea3…，重放同 id
- 同 request 不同 expected_revision → exit 3 **IDEMPOTENCY_CONFLICT**
- `runs show` → SUCCEEDED attempt=1 fence=1；未知 run → 404 NOT_FOUND exit 2

c) notifications ack
- ack → `{"acknowledged":true}` exit 0；同 request 重放 → true；已 ack 再发新 request → true；未知 id → 404 NOT_FOUND exit 2

d) runs cancel（Task revision CAS，按设计响应仅 cancel_ack）
- 显式错 revision(424242) → `cancel_ack:false` + exit 3 CONFLICT（CAS 生效）
- 无 --expected-revision（CLI 自动 run→task revision 路径）→ `{"cancel_ack":true}` exit 0；同 request 重放 → true
- 响应体仅 cancel_ack，与 docs/ExecutionProtocol.md 一致（未自加 revision 要求）

e) schema 搜索 / doctor / --json 与默认输出
- `memory search --query probe`：--json 与默认输出为同一 JSON（默认前缀“状态：OK”）
- `doctor`：--json 与默认输出同一 JSON（前缀“状态：DEGRADED”）；integrity ok、FK 0、runner 心跳 UNKNOWN、last_backup UNKNOWN、limitations 文案——未发明数据 ✓
- `items list --json` 200

f) 负向：runner 未启动时 `jobs list` → exit 5 DEPENDENCY_UNAVAILABLE ✓

## 2. 新观察（不阻断）
- O-CLI1 低：布尔 flag 会吞后随位置参数——`--config X --json doctor` 使 pos 为空（仅打印用法、exit 0）；`--json items list` 报 unsupported command。`--json` 置于命令末尾即正常。建议解析层特判布尔 flag。
- O-CLI2 低：runs/tasks 需三位置参数（`runs show <id>`）；`runs <id>` 报 “show|cancel id required”，帮助文本未列子命令形式。
- O-CLI3 信息：公共错误码折叠延续（REVISION_CONFLICT/IDEMPOTENCY_CONFLICT 均呈现 CONFLICT，退出码 3 正确）；错误体 result 为零值 JobRun（延续 O5）。
- O-CLI4 信息：CREATE_JOB 落库后 task.criteria 呈现服务端 DeriveCriteria 文本（evidence_policy="Authoritative persisted state or artifact bytes"），与提交文件中的 criterion 不同——疑似服务端重新派生，未做源码级确认，请 Codex 核 core.go:317 附近。
- O-CLI5 信息：trigger 后 run.command.arguments.notification_key 被改写为 occurrence 键（occ:v1:…）；每次 trigger 产生独立 notification（本轮 DB notification=2），与 occurrence 绑定一致。
- 备注：DB 中第 3 个 run（3fb249ef）已核实为 runner memory_refresh 槽位，非异常。

## 3. 未执行（严禁计 PASS）
1. restore 实机（§4.3）：CLI backup、restore、新独立 tokens（布尔/哈希级）、空 cap grant、双冻结、恢复 Runner 不 claim/dispatch/effect —— 未运行（/tmp/m3e/bk 为空）。
2. `verify --suite smoke`、`verify --backup` —— 未运行。
3. §4.4：TestRuntimeRemoteQueryRebindsEvidence 及 UNKNOWN query/不重发探针（POST 计数不增）—— 未运行；后台无遗留测试进程。
4. 显式“正确 revision”cancel 对 live run 的取消效果（QUEUED→CANCELLED）未验证：E7 实际走的是自动 revision 路径（脚本变量为空所导致，如实说明）。
5. 预算耗尽中断，未播种 QUEUED 队列快照、未做备份时点核对。

## 4. 合规
未触碰 resources/真实 API/ELIZA；无响铃、无系统设置变更；token 全程未读取未打印；仅 /tmp/m3e 与新拷贝只读查询（sqlite3 经拷贝副本查询）；仓库零写入；已结束全部由我创建的进程。

结论一句话：**CLI 主干链实机干净（actions 严格性、trigger CAS+幂等、ack、cancel CAS+仅 cancel_ack、--json/默认输出、负向码），无 must-fix；restore 实机、verify smoke/backup、UNKNOWN-query 定向测试三项仍未覆盖——维持条件通过，补做后即可定稿。** ( _ _ )
