> 模型限定：本报告来自 Ayanami 使用 `gpt-5.6-luna` 的历史复核。Master 最新指定必须由 `deepseek-v4.1-flash` 复核，因此本报告仅为 model-qualified 历史证据，不能充当最终复核裁决；原问题、结论与测试结果保留，待 DeepSeek 重新检视。

M1–M3 foundation fix 收口复核报告  
保存目标：`review/M1-M3-foundation-fix-ayanami.md`

一、总​​结结论

M1 foundation：有条件通过

- F1–F7 的原始失败场景已独立复核修复。
- D03 tombstone 与 D04 object_ref 去重行为已生效，Schema/DDL 未发生非预期漂移。
- F8 的生成器自动 gofmt 已实现；`go mod tidy` 可通过且无变更，但生成器本身未自动执行 `go mod tidy`，因此该项按“部分固定”处理。
- 不等于整个 M1（包含 root/core receipt、A02 全链路幂等等）已完整验收。

M3 diagnostics：有条件通过，不能宣称完整通过

- Manifest metadata、必需字段、未知字段、null objects、非法日期拒绝已复核。
- checkpoint 错误检查、objects 目录 fsync、末次 VerifyBackup 已在源码中确认。
- doctor 已返回 `DEGRADED`，`last_backup.state=UNKNOWN` 已复核。
- 原 M3 F2 仍未完全收口：budget/overdue/history 仍缺少明确 UNKNOWN 语义。
- F4 root CLI/Runner 恢复闭环按 Master 指示延期，本轮不称为完成。
- 发现 manifest 顶层 `created_at` 接受非 UTC offset；若遵循 Storage 的“UTC 固定毫秒文本”要求，该项仍需修订。

二、实际执行的检查

隔离副本：

`/tmp/m1m3review_20260914_033308/src`

执行结果：

- `go test ./tests ./store ./diagnostics ./ingest -count=1`：通过
- `go test -race ./store ./diagnostics ./ingest ./tests -count=1`：通过
- `go vet ./store ./diagnostics ./ingest ./tests`：通过
- `go build ./cmd/secretary ./cmd/secretaryd`：通过
- `go test ./contract ./platform ./store ./ingest ./diagnostics ./tests -count=1`：通过
- M1/M3 范围 Go 文件 `gofmt -l`：无输出
- `python3 contract/generate.py`：通过
- 生成后 `dto_generated.go` gofmt：通过
- `go mod tidy`：通过，go.mod/go.sum 无变化
- `go mod verify`：通过

独立探针覆盖：

- RevisionConflict.CurrentRevision
- SourceSynced RowsAffected 与事件回滚
- INITIALIZATION_INCOMPLETE
- ObjectStore 内容寻址
- fixture 重复页、冲突、非法记录与 quarantine 内容去重
- tombstone 写入
- UpdatedAt normalize error
- manifest 缺字段、未知字段、null objects、非法日期
- media_type/data_class/created_at metadata 篡改
- doctor DEGRADED 与 last_backup UNKNOWN

本轮未读取 `resources` 凭据，未连接 ELIZA，未响铃，未终止 Master 进程，未修改系统设置或项目源码。

三、M1 foundation F 项状态

F1｜固定，已验证｜A03

代码证据：

- `src/store/items.go:13-18`：定义 `RevisionConflict.CurrentRevision`
- `src/store/items.go:24-27`：revision 前置冲突返回当前 revision
- `src/store/items.go:96-100`：UPDATE 0 行时查询当前 revision 并返回结构化错误

独立场景：

两个 Store 句柄先后提交同一 revision，失败方通过 `errors.As` 得到 `*RevisionConflict`，`CurrentRevision=2`。

结论：旧裸 `REVISION_CONFLICT` 已修复。

F2｜固定，已验证｜A05/ObjectStore

代码证据：

- `src/ingest/fixture.go:42-66`：先计算 hash、查版本、判定重复/冲突，再创建对象
- `src/ingest/fixture.go:67-80`：校验失败直接 quarantine，不创建 ObjectRef
- `src/store/objects.go:17-29`：内容寻址 ID 与已有 ObjectRef 复用
- `src/store/objects.go:32-79`：对象文件与 ObjectRef 持久化

独立场景：

- 重复页二次同步：source_record 与 object_ref 均不增加
- 冲突/非法记录：不产生新的 object_ref
- 同一原文两次 PutObject：返回相同 ID/路径，仅保留一条 object_ref

结论：原 F2 重复对象与孤儿对象问题已修复。

F4｜固定，已验证｜quarantine 去重

代码证据：

- `src/ingest/fixture.go:101-124`
- quarantine 文件名由 source、version、raw 内容 hash 构成，并使用 `O_EXCL`
- 已存在同一内容时直接返回，不生成重复文件

独立场景：

同一冲突记录重复同步，quarantine 文件数量保持不变；不同冲突内容生成不同 quarantine 文件。

结论：已修复原“每次尝试新增 quarantine 文件”问题。

F5｜固定，已验证｜SourceSynced CAS

代码证据：

- `src/store/sources.go:65-76`
- 检查 `RowsAffected`
- 影响行数不是 1 时返回错误，不追加事件

独立场景：

人为制造 SQL revision 与 payload revision 不一致，`SourceSynced` 返回错误，事件数量不增加。

结论：已修复静默状态发散问题。

F6｜固定，已验证｜timestamp normalize

代码证据：

- `src/store/items.go:32-39`
- `UpdatedAt` normalize error 不再被忽略，返回 `INVALID_UPDATED_AT`

独立场景：

- 合法 `+08:00` 时间被规范化为 UTC 毫秒格式
- 小写 `z` 等 schema 可接受但 Go normalize 不接受的值，明确返回 `INVALID_UPDATED_AT`

结论：已修复原错误被延迟到事件校验、诊断不准确的问题。

F7｜固定，已验证｜INITIALIZATION_INCOMPLETE

代码证据：

- `src/store/store.go:91-99`

空数据库文件初始化时，明确返回：

`INITIALIZATION_INCOMPLETE: no schema_migration...`

结论：已修复原先统一返回 `MIGRATION_MISMATCH` 的问题。

F8｜部分固定，部分已验证｜生成器与依赖

代码证据：

- `src/contract/generate.py:27-28`：生成 DTO 后自动执行 gofmt
- `src/go.mod:5-8`：核心依赖已列为直接依赖

实际结果：

- 运行生成器成功
- 生成后的 DTO gofmt 通过
- `go mod tidy` 通过且 go.mod/go.sum 无变化
- `go mod verify` 通过

残留问题：

生成器只自动执行 gofmt，没有自动执行 `go mod tidy`。因此：

- “仓库依赖已 tidy”：固定
- “生成器自动 gofmt + go mod tidy”：未完全实现

建议：将 `go mod tidy` 明确放入构建/生成入口，或在脚本中显式执行；避免仅依靠人工运行。

D03/D04 文档同步：

- `docs/contracts.schema.json:487-540`：deleted tombstone 的 normalized=null 条件已写入
- `docs/DataStructure/SourceRecord.md:36-38`：已注明 D03
- `docs/Acceptance.md:62-64`：已注明 D04 与 object_ref 去重口径
- `README.md:58`：已列出 D03/D04 及修改文档路径
- `docs/contracts.schema.json` 与 `src/contract/contracts.schema.json` SHA-256 一致
- `docs/schema.sql` 与 `src/store/001_baseline.sql` SHA-256 一致

备注：README 当前仍写“修复中”，尚未记录本次独立收口结论。

四、M3 diagnostics F 项状态

F1｜固定，已验证｜Manifest ObjectRef metadata

代码证据：

- `src/store/backup.go:284-287`

当前同时比较：

- relative_path
- sha256
- byte_size
- media_type
- data_class
- created_at
- extensions 必须为空

独立篡改 media_type、data_class、created_at 后，`VerifyBackup` 均拒绝。

结论：原 F1 已修复。

F2｜部分固定，未完全收口｜Doctor UNKNOWN 语义

已固定部分：

- `src/diagnostics/diagnostics.go:25-35`：默认 `LastBackup.state=UNKNOWN`
- `src/diagnostics/diagnostics.go:61-65`：数据库可用但 telemetry 不完整时，整体状态为 `DEGRADED`
- 独立 doctor 探针实际得到 `status=DEGRADED`
- `last_backup.state=UNKNOWN`

仍未固定部分：

- `src/store/health.go:17`：`BudgetExhausted` 仍是 nullable pointer
- `src/store/health.go:20-21`：没有显式 budget UNKNOWN reason
- `src/store/health.go:71-77`：consciousness.overdue 仍固定为 nil
- 当前没有 backup history 查询，`last_backup` 只有默认 UNKNOWN，没有实际最近备份记录来源

结论：实现者声明的“DEGRADED + last_backup UNKNOWN”已通过；但原报告 F2 的完整范围仍未完全修复。状态不能记为完整固定。

F3｜部分固定，已验证主要场景；存在日期规范残留

代码证据：

- `src/store/backup.go:206-224`：JSON 解析、必需字段、未知字段拒绝
- `src/store/backup.go:226-230`：null objects 与非法 created_at 拒绝
- `src/store/backup.go:233-234`：schema_version/status 严格检查
- `src/store/backup.go:276-287`：ObjectRef Schema 与 DB metadata 校验

已验证拒绝：

- 缺少 created_at
- 未知顶层字段
- objects=null
- 非法日期字符串
- media_type/data_class/created_at metadata 不匹配

残留：

独立探针将 manifest 顶层 created_at 改为：

`2026-09-14T03:00:00.000+08:00`

当前 `VerifyBackup` 接受该值。代码使用 `time.Parse(time.RFC3339Nano)`，只验证 RFC3339 语法，没有要求 canonical UTC `Z` 或固定毫秒精度。

依据：

- `docs/Storage.md:5` 要求持久化时间为 UTC RFC3339 固定毫秒文本
- `src/store/backup.go:229-230` 当前只做 Parse

结论：旧 F3 的缺字段/未知字段/null/非法日期问题已修复；若按 Storage UTC canonical 要求，F3 仍需修订。建议比较 `NormalizeTimestamp(m.CreatedAt)` 与原值，或明确 manifest metadata 的 UTC canonical 规则。

F4｜未验证，按授权延期｜root CLI/Runner 恢复闭环

当前静态证据：

- `src/cmd/secretary/main.go:87-102`：restore 后执行 provisionRestored，并保持 Frozen
- `src/cmd/secretary/main.go:104-107`：带 `--backup` 时调用 VerifyBackup
- `src/cmd/secretaryd/main.go:93-99`：检查 execution_frozen/config frozen
- `src/cmd/secretaryd/main.go:168-196`：Runner 在 frozen 状态不执行调度和 Step
- `src/executor/executor.go:51-59`：Runner Step 入口检查 Frozen

本轮未宣称通过：

- 未执行 restored target 的完整 secretaryd core/runner 子进程闭环
- 未验证恢复后的历史 run reconciliation
- 未验证 frozen 状态下 claim/dispatch/effect 全路径抑制
- CLI 恢复 F4 按 Master 指示交由 root agent 继续

结论：F4 保持“未验证/延期”，不能纳入 M3 diagnostics 完整通过。

F5｜固定，正常路径已验证；故障注入未验证

代码证据：

- `src/store/backup.go:148-150`：检查 wal_checkpoint 返回错误
- `src/store/backup.go:152-161`：objects 目录执行 fsync
- `src/store/backup.go:179-185`：manifest 落盘、目录同步后执行末次 VerifyBackup
- `src/store/backup.go:187-188`：末次验证失败重新写入 INCOMPLETE marker

实际结果：

- 正常 backup/verify/restore 流程通过
- 对象篡改、数据库篡改、manifest 篡改均被拒绝

未覆盖：

- 人工注入 checkpoint 失败
- 磁盘满、I/O error、进程崩溃、断电
- 末次 VerifyBackup 失败后的真实恢复行为

结论：实现代码已固定；故障注入证据仍为未验证，不影响本次“修复存在性”结论，但不能扩展为完整 A20 通过。

五、未覆盖项

本轮没有扩大到其他业务模块，以下不作结论：

- root core receipt 与完整请求幂等
- M2 World/Memory/Context
- M3 全部 A11–A25
- 完整 CLI/Runner 恢复集成
- 真实模型、真实来源、REAL_USE
- 断电、磁盘满、真实进程崩溃恢复
- 文档检查器完整 DOC 通过状态

最终判断：

- M1 foundation 修复：有条件通过。F1–F7 行为修复已独立验证；F8 的 go mod tidy 自动化仍需明确。
- M3 diagnostics 修复：有条件通过。F1/F5 已固定，F2/F3 部分固定，F4 按授权延期；因此不得称 M3 diagnostics/A20 完整通过。

参考路径：

- `src/store/items.go`
- `src/store/sources.go`
- `src/store/objects.go`
- `src/store/store.go`
- `src/ingest/fixture.go`
- `src/store/backup.go`
- `src/store/health.go`
- `src/diagnostics/diagnostics.go`
- `docs/contracts.schema.json`
- `docs/DataStructure/SourceRecord.md`
- `docs/Acceptance.md`
- `docs/Storage.md`

复核思路：冻结源码到隔离副本，先复跑实现者声明的测试与 race/vet/build，再使用独立探针覆盖原报告中的负向场景；对通过项只报告实际证据，对 root 集成、故障注入和 telemetry 缺失项保留 `未验证/延期`，不将调用成功或普通测试通过泛化为完整验收。
