> 模型限定：本报告来自 Ayanami 使用 `gpt-5.6-luna` 的历史复核。Master 最新指定必须由 `deepseek-v4.1-flash` 复核，因此本报告仅为 model-qualified 历史证据，不能充当最终复核裁决；原问题、结论与测试结果保留，待 DeepSeek 重新检视。

M3 diagnostics/backup 独立复核报告

一、结论

总体结论：有条件通过；不得宣称 A20 正式通过。

当前模块的基础路径已真实可运行：

- SQLite online backup 使用 `modernc.org/sqlite` 的 `NewBackup`/`Step`/`Finish`，不是直接复制运行中的 SQLite 主文件。
- Backup 会写入 `INCOMPLETE` marker，完成 manifest 后再删除 marker。
- Restore 要求目标目录为空，执行 manifest、DB hash、integrity、FK、migration 和 object hash 校验。
- Restore 会创建 `execution_frozen`。
- `secretaryd` 当前确实检查 `execution_frozen`；Runner 也在 `Step` 开始处检查 Frozen 状态。
- CLI 在隔离临时目录中已真实完成 init → backup → restore → doctor。

但存在以下需修复项：

1. 中：manifest 与 SQLite `object_ref` 的 metadata 未完整绑定。
2. 中：doctor 的 UNKNOWN 语义不完整，并存在“未知 telemetry 但总体 status=OK”的 false-green 风险。
3. 中低：manifest 缺字段、未知字段未严格拒绝。
4. 中：root CLI 的 restore/runner/verify 集成仍未完成闭环。
5. 低至中：backup checkpoint 和对象目录 durability 错误未完整处理。

因此：

- Backup 基础机械行为：通过。
- Restore 基础机械行为：通过。
- Doctor 数据库基础健康检查：通过。
- Doctor 完整诊断契约：有条件通过。
- CLI/Runner 恢复集成：有条件通过，尚未完成。
- A20 正式验收：未通过，不能以编译、普通测试或 race 通过替代。

二、需修复项

F1｜中｜Manifest ObjectRef metadata 与 DB 未完整一致性校验

位置：

- `src/store/backup.go:242-245`
- `src/store/backup.go:234-245`

当前查询只读取并比较：

- `relative_path`
- `sha256`
- `byte_size`

没有比较：

- `media_type`
- `data_class`
- `created_at`

独立探针：

- 创建含有一个 `SYNTHETIC` object 的 backup。
- 将 manifest 中该对象的 `data_class` 改成 `PERSONAL`。
- `VerifyBackup` 仍返回成功。

触发场景：

任何 manifest 被部分篡改、错误生成或手工修复时，文件内容 hash 仍然正确，但对象分类和媒体类型可能与数据库中的 immutable reference 不一致。`data_class` 涉及披露策略，不能仅视为展示字段。

修复建议：

将校验 SQL 扩展为：

```sql
SELECT relative_path, sha256, byte_size, media_type, data_class, created_at
FROM object_ref
WHERE id=?
```

并逐字段比较。补充 manifest metadata 篡改回归测试，至少覆盖 `media_type`、`data_class`、`created_at`。

结论：需修复。

F2｜中｜Doctor 对 UNKNOWN 的表达与总体状态矛盾

位置：

- `src/diagnostics/diagnostics.go:25-26`
- `src/diagnostics/diagnostics.go:33-35`
- `src/store/health.go:17`
- `src/store/health.go:20-21`
- `src/store/health.go:71-76`

当前无 heartbeat、无 backup history、无 budget telemetry 时，独立 probe 输出：

```json
{
  "heartbeats": {
    "core": {"state":"UNKNOWN"},
    "runner": {"state":"UNKNOWN"}
  },
  "database": {
    "consciousness": {
      "state":"UNKNOWN",
      "slot":null,
      "created_at":null,
      "overdue":null
    },
    "budget_exhausted":null
  },
  "last_backup":null,
  "status":"OK"
}
```

虽然 `limitations` 文本写了 UNKNOWN，但：

- `last_backup` 是 JSON `null`，没有显式 state。
- `budget_exhausted` 是 JSON `null`，没有显式 state。
- `consciousness.overdue` 永远是 `null`。
- `Status` 仍为 `OK`。
- `Health` 没有查询 `root_budget`。
- `Doctor` 没有读取或持久化 backup history。
- 有意识快照时，`overdue` 仍固定为 `nil`。

触发场景：

新初始化目录、进程尚未运行、从未执行过 backup，或 budget/scan telemetry 缺失时，doctor 会同时报告 UNKNOWN 字段和整体 OK。调用方如果只读取 `status`，容易误判为健康完整。

修复建议：

建议拆分：

```text
database_status: OK|ERROR
telemetry_status: OK|UNKNOWN|ERROR
```

或者在每个非必有指标中明确：

```json
"last_backup": {
  "state": "UNKNOWN",
  "reason": "NO_BACKUP_RECORD"
}
```

至少应：

- 缺少 backup record → `state=UNKNOWN`
- 缺少 budget record → `state=UNKNOWN`
- 缺少 scan/heartbeat → `state=UNKNOWN`
- 无法计算 consciousness overdue → 显式 UNKNOWN，而不是无语义 null
- 不要在关键 telemetry 全部未知时返回整体 `status=OK`

结论：需修复后才能称完整 doctor。

F3｜中低｜Manifest 未执行严格结构校验

位置：

- `src/store/backup.go:185-192`

当前流程是：

1. `contract.ParseJSON` 只检查 JSON 语法。
2. `json.Unmarshal` 到 `BackupManifest`。
3. 只手工检查 `schema_version` 和 `status`。

因此：

- 缺少 `created_at` 的 manifest 可以通过。
- 未知字段会被静默忽略。
- `objects:null` 等结构问题没有严格契约拒绝。
- `database_sha256`、`created_at` 等字段没有完整格式校验。

独立 probe：

删除 `manifest.json` 中的 `created_at`，`VerifyBackup` 仍返回成功。

修复建议：

为 `BackupManifest` 增加严格 Schema 或显式 strict decoder：

- required fields
- `additionalProperties:false`
- SHA-256 格式
- UTC RFC3339 timestamp
- `objects` 必须是非 null array
- `status`、`schema_version` 严格枚举/版本校验

结论：需修复。

F4｜中｜Root CLI 的恢复闭环尚未完成

模块本身已经创建冻结 marker：

- `src/store/backup.go:257-301`
- `src/store/backup.go:265`
- `src/cmd/secretary/main.go:91-99`

Runner marker 检查当前存在：

- `src/cmd/secretaryd/main.go:92-98`
- `src/executor/executor.go:48-51`

但是 CLI restore 只生成：

- `state/`
- `objects/`
- `execution_frozen`
- `config.json`

`secretaryd` 启动仍要求：

- `run/client.token`
- `run/internal.token`
- Core 使用的 `run/grant.id`

位置：

- `src/cmd/secretaryd/main.go:63-85`
- `src/cmd/secretaryd/main.go:118-124`

隔离 CLI 实测：

- init → backup → restore → doctor：成功。
- restore 后 doctor 正确显示 `execution_frozen:true`。
- 直接启动 restored target 的 runner：

```text
open /tmp/.../restored/run/client.token: no such file or directory
```

这属于 root 集成边界，不是 `Store.Restore` 本身的 hash/FK/integrity 缺陷。

修复建议：

提供明确的恢复后 provisioning 流程：

1. 保持 frozen。
2. 对账历史 run。
3. 重新生成或安全配置本地运行认证材料。
4. 校验 grant 与数据库中的授权记录。
5. 只有显式 enable 后才移除 marker/解除 config frozen。
6. 增加真实 Runner 启动测试，证明 frozen 状态下不会 claim、dispatch 或产生副作用。

另外，当前：

- `src/cmd/secretary/main.go:100-103` 的 `verify` 分支调用的是 `db.Health()`。
- 它不是 `store.VerifyBackup()`。
- 因此 `secretary verify` 当前验证的是 live DB health，不是 backup directory manifest/DB/object integrity。

建议增加明确命令，例如：

```text
secretary verify-backup --backup <directory>
```

或者让 `verify --backup <directory>` 进入 `store.VerifyBackup`。

结论：模块部分通过，root integration 仍需完成。

F5｜低至中｜Backup checkpoint 错误被忽略，durability 证据不完整

位置：

- `src/store/backup.go:141-147`
- `src/store/backup.go:164-168`

问题：

```go
snap.DB.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
```

返回值没有检查。

同时，复制对象后只对每个对象文件执行 `Sync`，没有显式同步 `dest/objects` 目录；Backup 返回前也没有调用一次完整的 `VerifyBackup` 作为最终自检。

正常隔离探针观察到 backup 根目录为：

```text
manifest.json
objects/
secretary.sqlite
```

且没有遗留 `secretary.sqlite-wal`，但这只能证明正常路径，不能证明 checkpoint 失败路径。

触发场景：

I/O error、busy、磁盘压力、异常关闭或 checkpoint 失败时，主 DB hash 可能不代表完整的 WAL 状态，或对象目录 entry 的持久性不足。

修复建议：

- 检查 checkpoint 返回值。
- 确认 checkpoint 完成后没有未处理的 WAL sidecar。
- 对 `objects/` 目录执行 fsync。
- manifest 完成前执行最终 VerifyBackup。
- 增加 checkpoint failure、disk-full、取消、异常关闭测试。

结论：建议修复；在正式 A20 证据中不能忽略。

三、已通过的部分

1. SQLite online backup

`src/store/backup.go:68-95` 使用：

```go
NewBackup
Step
Finish
```

未发现直接复制 live SQLite 主文件的实现。

2. INCOMPLETE 保护

`src/store/backup.go:52-61` 先写入 `INCOMPLETE`，完成 manifest 后在 `:161-162` 删除。

Verify 在 `:171-175` 拒绝仍带有 `INCOMPLETE` 的目录。

3. DB integrity/FK/migration

`src/store/backup.go:207-224` 实际执行：

- `PRAGMA integrity_check`
- `PRAGMA foreign_key_check`
- schema migration version/checksum 校验

4. Object hash 和引用数量

`src/store/backup.go:226-254` 检查：

- DB 中 object_ref 数量
- manifest object 数量
- object ID 唯一性
- path/hash/size 与 DB 对应关系
- 目标 object 文件实际 hash/size

5. Restore 冻结行为

`src/store/backup.go:257-301`：

- 要求目标目录为空
- 先生成 `execution_frozen`
- 复制 DB 和 immutable objects
- 重新通过 `Open` 校验 migration

已有测试真实验证：

- concurrent write during backup
- restore 到新目录
- 非空 target 拒绝
- object tamper 拒绝
- corrupted DB 拒绝
- frozen marker 存在

6. Runner marker 检查

当前 root 代码已经具备基本冻结检查：

- daemon 层：`src/cmd/secretaryd/main.go:92-98`
- Core loop：`:135-139`、`:153-158`
- Runner scheduler：`:173-181`
- Runner executor：`:193-196`
- Executor 自身：`src/executor/executor.go:48-51`

这部分不能再归类为“完全未集成”；应归类为“已有基本集成，但恢复后的 runtime provisioning 和 end-to-end proof 尚未完成”。

四、真实执行命令与结果

环境：

- macOS 15.7.4
- darwin/arm64
- Go 1.25.6
- module：`secretarysimplified`
- 使用真实 wall clock
- 全部数据为 synthetic fixture
- 未使用真实模型、真实来源或系统凭据
- 项目根目录无 `.git`，commit/build commit 不可提供

通过：

```text
go test ./store ./diagnostics -count=1
PASS

go test -race ./store ./diagnostics -count=1
PASS

go test ./...
PASS

go test -race ./...
PASS

go vet ./...
PASS

go build ./cmd/secretary
PASS

go build ./cmd/secretaryd
PASS

gofmt -l store/backup.go store/health.go \
  diagnostics/diagnostics.go store/backup_test.go \
  diagnostics/diagnostics_test.go
PASS，未输出未格式化文件
```

当前构建产物 SHA-256：

```text
secretary:
64df53e6d1cd356005582676ba27e2ea751422f7effad9454df4f83192fb3a5c

secretaryd:
77308a7cec7e765ba11af46aacee1837bfdf820f08ad3c71e308ecb40976b7e8
```

隔离 CLI 流程：

```text
secretary init
secretary backup
secretary restore
secretary doctor
```

结果：

- backup status：`COMPLETE`
- restore 成功
- `execution_frozen` 存在
- doctor `execution_frozen:true`
- doctor 数据库 integrity：`ok`
- doctor FK violations：`0`

独立临时副本探针真实失败：

1. 缺少 `created_at` 的 manifest 被接受。
2. manifest 中 ObjectRef `data_class` 与 DB 不一致仍被接受。
3. 缺少 backup/budget telemetry 时 doctor 输出 `status:"OK"`，同时相关字段为 `null`。

文档检查：

```text
python3 docs/checks/validate_docs.py
BLOCKED
原因：缺少 docs/checks/requirements-docs.txt 对应的已安装依赖。
```

这不是本模块 Go 测试失败，但 DOC 检查本轮不能记为通过。

说明：

复核早期某次 `go test ./...` 曾因 `tests/world_test.go:7` 未使用 `secretarysimplified/store` import 失败；随后该文件在复核期间被同步修改，当前文件已移除该 import，完整普通测试和 race 测试均重新通过。该原始失败保留为 moving working tree 证据，不归因于本 M3 diagnostics/backup 模块。

五、未覆盖项

以下内容本轮没有足够证据，不能宣称通过：

- backup/restore 期间进程崩溃、断电、磁盘满、I/O error。
- online backup `Step`/checkpoint 失败路径。
- restore 过程中 backup directory 被并发替换或修改的完整 TOCTOU 测试。
- Object GC 暂停协议；当前实现没有自动 GC，因此只证明“现有代码没有 GC 竞争”，没有证明未来 GC 协调机制。
- concurrent object creation 与 backup 的专项测试；已有并发测试主要写入 Item。
- FK 损坏和 migration checksum 篡改的独立负向测试。
- 完整 manifest Schema、未知字段、metadata 篡改测试。
- backup history 持久化和 doctor 最近 backup 查询。
- root budget exhausted 实际记录与 doctor 输出。
- heartbeat stale/future/malformed、scan failure、clock jump。
- 恢复后实际 Runner claim/dispatch/effect suppression。
- restore 后历史 run reconciliation 和显式 unfreeze 流程。
- 真实 P1/P2 子进程故障恢复。
- LIVE_MODEL、REAL_USE、真实来源、真实设备、声音/响铃。
- 未读取 `resource/` 凭据，未连接 ELIZA，未发响铃，未终止 Master 进程，未修改系统设置。

六、参考资料

设计约束：

- `docs/Operations.md:11-17`：doctor 必须显示 heartbeat、scan、budget、backup、disk 等诊断信息。
- `docs/Operations.md:33-39`：online backup、INCOMPLETE、对象 hash、integrity/FK/migration、restore frozen。
- `docs/Acceptance.md:30`：A20 备份恢复验收条件。
- `docs/Acceptance.md:50-60`：测试命令、证据分层、report 要求。
- `docs/Storage.md:9-11`：SQLite WAL、foreign_keys、busy_timeout、synchronous。
- `docs/Storage.md:45-52`：schema migration 和备份恢复兼容要求。

官方资料：

- SQLite Online Backup API  
  https://www.sqlite.org/backup.html

- SQLite PRAGMA integrity_check  
  https://www.sqlite.org/pragma.html#pragma_integrity_check

- SQLite Foreign Key Support  
  https://www.sqlite.org/foreignkeys.html

七、复核思路

先读取 Operations、Storage、Acceptance 的备份恢复和 doctor 契约；再逐行检查 `backup.go`、`health.go`、`diagnostics.go` 及 CLI/Runner 边界。随后重新执行实现者声明的普通测试和 race 测试，再运行全量测试、vet、双入口 build。最后将源码复制到 `/tmp` 临时副本，加入不进入项目的独立探针，专门篡改 manifest metadata、删除 manifest 字段并观察 Verify/Doctor 行为；再用 `/tmp` CLI 完成 init、backup、restore、doctor 流程。所有业务源码保持只读。

请由 Codex 将本回复原样保存为：

`review/M3-diagnostics-ayanami.md`
