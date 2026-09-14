> 发布副本：仅对本机路径作规范化；原始报告本地保留于 `review/private/`。问题、结论和测试结果未改动。

# M3 restore/query-only 复验响应（DeepSeek 侧执行轮）

- 会话模型：deepseek-v4.1-flash / opencode-go（本轮无 429，无重试告警）
- 仓库快照：`<PROJECT>`，HEAD `2b753f2cb1e85c14e24fd79c8136b1354d2afc75`
- 复用二进制（未重新全 build）：`/tmp/m3e/bin/secretary` sha256 `97d0311bd7ecbf70fcccafb98694339861cf6301647f8d2326aa84f5a0c223f3`；`/tmp/m3e/bin/secretaryd` sha256 `d1d6ed47c580401fe27f8579c1d2c85a2512ae3496c5850208628eed5805e8b9`
- 隔离副本（仅用于临时 seed 程序与 probe，未污染主仓库）：`/tmp/m3qlab_080319/src`
- 合成运行区：`/tmp/m3r`（`meta.txt`、`pre_runner_checks.json`、`post_runner_checks.json`、`backup/`、`restored/`、`report/`）
- 约束遵守：未触碰 `resources/`、真实 API、ELIZA、响铃、系统修改；未触碰旧 `/tmp/m3ds` 或 `release/soak` 在用进程；无子 agent；无全仓测试；未改主仓库任何文件

## 结论（仅针对本轮 3 项，不构成整体验收）

| 项 | 结果 | 关键证据 |
| --- | --- | --- |
| 1 CLI backup/restore（含可领取 QUEUED seed、fresh token、空 cap、双冻结、Runner 2s 不变量） | PASS | 见下 §1 |
| 2 verify --suite smoke / verify --backup 正负向 | PASS | 见下 §2 |
| 3 仓库测试 `TestRuntimeRemoteQueryRebindsEvidence` 计数断言核查 | PASS（仓库测试本身无计数断言；已按指令加临时独立 probe 且 PASS） | 见下 §3 |

## §1 临时合成目录实测（CLI backup → restore → frozen Runner 2s）

步骤与退出码：

1. `secretary init --data-dir /tmp/m3r/data --json` → exit 0，`grant_id=958714ae-786c-46f9-be48-70dcbb40491b`
2. 合法 QUEUED seed（不篡改 DTO、不伪造队列）：在隔离副本内用与 `src/tests/runtime_test.go` fixture 相同的 store API（`store.DeriveCriteria` + `store.RegisterImmediate`）写入已 init 的数据目录 → `SEEDED run=2beeada8-ec1c-4aaa-acf8-3000ad0e8a75 attempt=0`，capability=`notify.local`
3. `secretary backup --config /tmp/m3r/data/config.json --output /tmp/m3r/backup --json` → exit 0，`status=COMPLETE`，`database_sha256=da8dc2d0b8974cc47c7b16f911f02493348c1b384807635ac3fea4e65350db0d`
4. `secretary restore --config /tmp/m3r/data/config.json --backup /tmp/m3r/backup --target /tmp/m3r/restored --json` → exit 0

恢复产物校验（`/tmp/m3r/checks.py`，只输出布尔，不输出 token 值）：

- `fresh_client_differs_from_origin_client=true`，`fresh_internal_differs_from_origin_internal=true`
- `fresh_tokens_mutually_distinct=true`（client≠internal），交叉比对 `fresh_client_differs_from_origin_internal=true`、`fresh_internal_differs_from_origin_client=true`
- 两者均为 64 hex 字符（32 字节）→ `token_hex_len_64_both=true`
- 空 cap grant：`authorization_grant` 共 2 行，最新一行 `capability_ids` 长度 0 → `restored_newest_grant_empty_caps=true`（历史 revision 1 保留，新增空能力 grant）
- 双冻结：`config.json.execution_frozen=true` 且文件 `restored/execution_frozen` 存在 → 两项均为 true
- 队列确实是“可领取”而非空队列：`job_run` 行数 1（state=QUEUED，`scheduled_for=2026-09-14T00:04:06.891Z` ≤ 当前 UTC），复刻领取谓词（QUEUED 且到期且 capability ∈ 本地可执行集合）计数 `claimable_local_now=1`

Runner 2 秒窗口（仅自建进程）：

- 实际启动恢复目录 Runner：`/tmp/m3e/bin/secretaryd runner --config /tmp/m3r/restored/config.json`（pid 44363，Hermes 后台会话 `proc_cfbef024d983`）
- 2 秒后 `kill -0` 存活 = true；`ps -o pid,etime,command` 显示 `44363 00:09`（该轮共运行约 9s，覆盖 2s 断言窗口）；`run/` 下新增 `runner.lock`、`runner.sock`（预期文件产物，非 DB 计数）
- 前后不变量比对（`diff.py`，比较 `row_counts` 全表、`job_run_states`、`execution_attempt_rows`、`claimable_local_now`、冻结与 token 布尔）：`changed_keys=[]`，`unchanged=true`
  - 队列：`job_run_states={"QUEUED":1}` 前后一致；attempt：`execution_attempt=0` 前后一致；效果/回执：`notification=0`、`executor_receipt=0`、`execution_permit=0` 等全表计数前后一致
- 结束：`kill 44363` 后 `runner_still_alive=false`，`pgrep -fl "secretaryd runner --config /tmp/m3r"` 无残留（仅结束自建进程，未影响其他进程）

## §2 verify 实测

- `secretary verify --config /tmp/m3r/data/config.json --suite smoke --report /tmp/m3r/report --json` → exit 0，`status=PASS`，4 项检查全 PASS（`sqlite_integrity`、`immutable_object_roundtrip`、`backup_manifest_and_objects`、`restore_integrity_and_freeze`），`real_model=false`、`real_data=false`，`build_id=97d0311b…` 与所用二进制 sha256 一致（说明测的就是该二进制）；写出 `report/report.json`、`report/report.md`
- 正向：`secretary verify --config /tmp/m3r/data/config.json --backup /tmp/m3r/backup --json` → exit 0，返回 manifest `status=COMPLETE`，`database_sha256` 与 `backup/manifest.json` 一致（`da8dc2d0…`）
- 负向：`cp -R backup backup_tampered` 后篡改副本 DB（偏移 188416/376832 翻转 1 字节）：篡改后 sha256 `6e4c740d988cbfe420fbdf9c8beee5013a596cb695763b2473de821241b000f2` ≠ 原 `da8dc2d0…`；`secretary verify --backup /tmp/m3r/backup_tampered --json` → **exit 2**，hash 拒绝：`BACKUP_DATABASE_HASH_MISMATCH`（manifest.json 未改，证明拒绝由 DB 哈希校验触发，而非 manifest 缺失）

## §3 RemoteCore.Query 计数断言核查

- 仓库内实测：`cd src && go test ./tests -run '^TestRuntimeRemoteQueryRebindsEvidence$' -count=1 -v` → PASS（0.04s，`ok secretarysimplified/tests 0.945s`）
- 代码阅读结论：`src/tests/runtime_remote_query_test.go` 使用静态 `http.HandlerFunc`（每次请求都回同一 `CoreWork`），**未按 HTTP 方法或 Execute/POST 计数**；`grep -n "count|POST|Method|Execute"` 在该文件无任何命中。它断言的是：新 receipt 的 ID 与旧 receipt 不同、attempt/fence 重绑定到当前 run、tampered command 被拒（`CORE_QUERY_IDENTITY_MISMATCH`）——**没有**“Query 后不产生 Execute/POST 计数增长”的断言
- 按指令未改仓库，仅在隔离副本 `/tmp/m3qlab_080319/src/tests/probe_remote_query_nopost_test.go` 增加临时独立 probe：按方法计数请求，执行两次 `RemoteCore.Query`（一次正常对账、一次篡改命令拒绝路径）→ `go test ./tests -run '^TestProbeRemoteQueryIssuesNoPOST$' -count=1 -v` PASS，计数器 `GET=2 POST=0 OTHER=0`，断言 POST/其他方法为 0、GET 恰为 2。实现侧对照：`src/executor/remote_query.go:19` 仅 `GET /internal/v1/work/<id>`，无 POST

## 未覆盖 / 边界（不得据此泛称“全部验收通过”）

- 未跑 A01–A25 或任何整体验收；未跑全仓测试；未做新静态审计；未跑 CLI 其它子命令
- 未接真实模型/真实数据（smoke 报告自证 `real_model=false`、`real_data=false`），未验证 live 语义
- §1 中 Runner 按设计处于 frozen，故“可领取 QUEUED run 保持 QUEUED”是被断言的不变量；本轮未演示真实领取/派发/效果落地流程
- §3 的计数断言只存在于临时副本，未提交为仓库回归测试（按指令“不改仓库”）；若要长期防回归，需要一次受控的仓库测试补充
- 过程瑕疵（如实记录）：首次 `checks.py`（URI `mode=ro` 打开）报 `unable to open database file`；紧接着的独立最小复测显示 plain 与 URI 两种打开方式均正常（86 张表元数据），判定为瞬时问题；最终校验改用 plain connect + `PRAGMA query_only=1` 重跑通过
