复核完成。以下为完整复核报告（含全部证据与商议意见），请 Codex 保存至 `review/M1-foundation-ayanami.md`。

---

# M1 foundation 模块复核报告 — Ayanami

复核对象：Secretary_Simplified M1 foundation（contract / platform / store 基础 / ingest fixture）
复核日期：2026-09-14 03:05–03:12 HKT；复核者：Ayanami（只读复核，未修改任何项目文件）

## 0. 结论

**有条件通过（Conditional PASS）** —— 以本次冻结快照为准：声明范围内的 A01/A03/A04/A05/A19 基础行为多数经我独立复现通过，模块自带测试全部通过且 vet/race 干净；但存在 **1 项需双方商议的设计缺陷（D1，阻断性）**、**1 项口径设计问题（D2）** 与 **若干需修复项**，在 D1/F1/F2 处理完成前不能宣称 M1 foundation 完整验收通过。四类证据中本轮仅覆盖 OFFLINE（隔离副本）+ 部分 DOC 对照；LIVE_MODEL / REAL_USE 未涉及。

## 1. 复核快照与证据基础（关键哈希，SHA-256 前 16 位）

快照冻结时间 2026-09-14 03:07:10，隔离副本 `/tmp/m1review2_20260914_030710/src`（项目目录只读未动）。复核期间发现范围内文件在 03:03–03:05 仍有改动（items.go 先后两版、contract 新增 json.go 等），故以下哈希为本次评审的事实基准；之后再发生的修改不在本报告范围。

```
contract.go        882c49c879fcc91e…   json.go        8877b720e16986e4…
dto_generated.go   87522b8abe3c5f3e…   contract_test  15ae6a1cac06e5c8…
contracts.schema   df45650505d5883e…  (= docs 副本一致)
platform.go        7f711f64f280d097…   platform_test  feb4316a1af61a5d…
store.go           65f0d6c0f260f7f7…   items.go       573b85b63c8d15e0…
objects.go         e0da0e26aa89024b…   sources.go     004f46d94a163514…
001_baseline.sql   75668022e95ba4c5…  (= docs/schema.sql 一致)
foundation_test    8a7f4dd7f184f1c6…   fixture.go     7aeaeca01da932ce…
fixture_test       e074d38dccfa4fdc…   go.mod         dfcc01ac72802497…
```

证据日志：`/tmp/m1review2_20260914_030710/{m1_tests.log, m1_tests2.log, m1_tests3.log, m1_race.log, MANIFEST.sha256, dto.diff}`（临时目录，建议 Codex 转存）。

## 2. 执行的检查（全部实际执行，不采信转述）

隔离副本内执行，均未触碰项目目录：

1. `go test ./contract ./platform ./store ./ingest -count=1` → 全过（含 agent 声明的 CAS/DAG 回滚/事件原子性/四连接 PRAGMA/对象篡改/fixture 重复与冲突 quarantine 测试）。
2. `go test ./store -race`（关键并发包）→ 通过；`go vet ./contract ./platform ./store ./ingest` → 干净；`go build` → 干净。
3. 我自写独立探针 17 个用例（新写于副本，未入项目；文件：`zz_review_a01_test.go`、`zz_review_m1_test.go`、`zz_review_a05_test.go`）：
   - A01：未知 major、额外顶层字段、未知 enum（status/time_state）、priority 越界、坏 UUID、坏时间戳、错类型、未登记扩展、runtime.misfire 负数/小数、runtime.authorization 坏 grant_id/多键 —— 全部被拒；已登记扩展被接受；Decode 失败不产生部分写入；重复 JSON 键与非整数“语义数字”被 json.go 拒绝。
   - A03：两个 Store 句柄（两个连接池）同 revision 竞争 → 恰一方成功。
   - A04：依赖环中途失败 → 无部分依赖行、无事件泄漏、revision 不变；悬空依赖与悬空 evidence 拒绝且无部分写入；4 连接 PRAGMA（fk=1/busy_timeout=5000/synchronous=2）与 journal_mode=wal。
   - A05：重复页去重计数正确；空成功页更新新鲜度；失败不清库且新鲜度不前进；冲突隔离不落库。
   - A19：version=2 拒绝、checksum 篡改拒绝、迁移中途失败回滚干净（无残留 object_ref/schema_migration）、JSON↔SQL 重复字段不一致可检出（item status/revision 注入篡改 → STORAGE_CORRUPTION）。
4. DOC 对照：src 内 `contracts.schema.json` 与 `docs/` 一致；`001_baseline.sql` 与 `docs/schema.sql` 一致；DTO 再生成漂移检查（见 F8）。

## 3. 通过项摘要

- **A01 严格结构**：通过。schema `additionalProperties:false` 全量生效、`schema_version` const=1、format(uuid/date-time) 显式断言（AssertFormat），扩展白名单仅 runtime.authorization / runtime.misfire 且形状校验正确；未知命名空间 → UNREGISTERED_EXTENSION。
- **A03 并发版本**：半通过 —— CAS 单胜者成立；**失败方未收到当前版本（F1）**。
- **A04 事务与引用**：通过（离线部分）。依赖环/断裂引用拒绝、状态与事件同事务全有或全无、四连接 PRAGMA 与 WAL 属实；receipt 部分按已知说明由 core_repo 实现，不在本报告结论内。另见 F5 的写路径加固建议。
- **A05 来源同步**：3/4 通过。空页新鲜度、失败不清库、同版本异哈希隔离均属实；**“重复页无重复对象”不满足（F2），且 tombstone 拒绝入库（D1）**。
- **A19 迁移与一致性**：通过（离线部分）。版本/校验和拒绝、迁移失败回滚干净、JSON↔SQL 一致性防护（item 含依赖行、source_state/source_record 读取校验）均属实；“旧数据迁移”当前无 v1 之外迁移可测（N/A）。

## 4. 发现清单

**F1｜A03 冲突错误不携当前版本（中）**
- 问题：`src/store/items.go:17-19` 与 `:84-86` 两类冲突均只返回裸 `"REVISION_CONFLICT"`；Acceptance A03 要求“失败方收到当前版本”。
- 证据/复现：两个 Store 句柄同 revision 各写一次；败者错误=`"REVISION_CONFLICT"`，实际当前 revision=2（日志 m1_tests.log:13）。
- 建议：0 行更新时在 tx 内查当前 revision，返回结构化错误（如 `REVISION_CONFLICT: current=2`）；在 Interfaces 错误表登记；补回归测试。

**F2｜重复/被拒记录仍创建对象（中）——关联 D2**
- 问题：`src/ingest/fixture.go:37` 在去重与校验**之前**无条件 PutObject；`:60-63` 冲突/失败后对象已落盘。
- 证据/复现：①同页二次同步：`source_record` 2→2 但 `object_ref` 2→4（新增 2 文件+2 行）；②冲突被拒记录：object_ref 1→2，孤儿对象=1；③tombstone 失败同样遗留对象。
- 建议：先查（source_id, external_id, source_version, content_hash）与校验，仅对“新且合格”记录 PutObject；为历史孤儿对象提供清理/GC；补测试。

**F3｜tombstone 无法入库（高）→ 归类设计缺陷 D1，见 §5。**

**F4｜quarantine 文件按尝试累加（低）**
- 问题：`fixture.go:83-92` 冲突/无效记录每次同步落新文件（文件名=每次新 v.ID）。
- 证据：同一冲突同步两次，quarantine 文件 1→2，DB 仍 1 行。
- 建议：以内容/来源身份命名或 hash 去重；或在 DB 记 quarantine 索引。

**F5｜SourceSynced 未校验 CAS 影响行数（中低）**
- 问题：`src/store/sources.go:65-68` UPDATE 结果被丢弃，0 行也照常追加 source.synced 事件。
- 证据/复现：SQL revision 置 5（payload 仍 1）后调用 SourceSynced → 返回 nil、事件 +1、SQL revision 仍 5，静默发散。
- 建议：检查 RowsAffected==1，否则返回 STORAGE_CORRUPTION/CAS 错误；补测试。

**F6｜UpdatedAt 归一化错误被忽略（低）**
- 问题：`items.go:27` 对 UpdatedAt 归一化忽略错误（CreatedAt 在 `:23-25` 有显式检查）。
- 证据：`updated_at="2026-09-14T03:00:00z"`（小写 z 过 jsonschema、不过 Go RFC3339）→ 报错来源错位为 `INVALID_CONTRACT ChangeEvent`（事务已回滚，无数据损坏，但诊断误导）；"garbage" 则正常在 Item 层拒绝。
- 建议：与 CreatedAt 同口径显式检查；统一两个校验器的时区/大小写口径。

**F7｜半初始化库文件不可自恢复（低）**
- 问题：`store.go:58-59` 文件存在即走 Open，`:93-96` 统一报 `MIGRATION_MISMATCH`。
- 证据：空文件 + Init → "MIGRATION_MISMATCH"（无“未完成初始化”提示，需人工删文件）。
- 建议：识别“无 schema_migration 表”→ 清晰指引或安全重试；可考虑先建临时库+原子改名以缩小崩溃窗口。（注：迁移失败回滚本身已验证干净。）

**F8｜生成物与生成器漂移 + 依赖卫生（低，工具链）**
- 问题：以 `contract/generate.py` 从权威 schema 再生成 `dto_generated.go`，产物哈希 ≠ 提交版本（0144490d… vs 87522b8a…）；抽样 diff（前 60 行）均为 gofmt 对齐差异（提交版为对齐格式、生成器输出未对齐）——未完成“gofmt 归零”验证，请修复后补做。`gofmt -l` 另示 store/backup.go、store/health.go（范围外文件）未格式化。`go.mod` 中直接依赖（modernc/sqlite、santhosh-tekuri/jsonschema）仍标 `// indirect`。
- 建议：generate.py 末尾执行 gofmt（或文档化 gofmt -w 步骤）+ 增加一致性检查；对 go.mod 跑 `go mod tidy` 并提交。M1 范围源码未发现两库之外的第三方直接 import（google/uuid 等为传递依赖）。

**低危观察（不单列编号）**：悬空依赖的 FK 错误为原始 SQLite 文案（`constraint failed: FOREIGN KEY constraint failed (787)`），建议包装为领域错误；`platform.Lock.Close` 忽略解锁错误、`ManualClock.T` 可无锁直写（供测试用，注意并发使用）。

## 5. 设计缺陷商议意见（四段式；按 Master 授权，双方确认后可直接修订设计并在 README 登记，无需再次请 Master 批准）

**D1｜SourceRecord 契约无法表达合法 tombstone**
1. 具体问题：`contracts.schema.json` 的 `SourceRecord.allOf` 规定 “VALID ⇒ normalized 必须为对象；QUARANTINED ⇒ normalized=null 且 validation_error 非空”。而删除 tombstone（deleted=true、无正文）天然 normalized=null——两个分支都无法容纳，与 `DataStructure/SourceRecord.md`“删除必须有 tombstone 或完整同步对账证据”矛盾。`fixture.go:49-54` 对 deleted 跳过解码并置 VALID，导致 Sync 报错、整批中止。
2. 证据：`TestReviewDeletedTombstone`——单独 tombstone：Sync err=`INVALID_CONTRACT SourceRecord … at '/normalized': got null, want object`，records=0；混合批（good,tombstone）：good 已落库 1 行，tombstone 处整批报错。
3. 方案（Ayanami 建议）：修订 SourceRecord 条件为—`if deleted==true：允许 normalized=null（VALID、validation_error=null）`；否则维持“VALID ⇒ normalized 对象”。同步改 `DataStructure/SourceRecord.md`，并在修改处注明“实施过程中发现的缺陷”，README 登记路径；fixture.go 补 tombstone 回归测试。
4. 结论：**需修订（设计+实现）**。待 Codex 确认方案后执行；我将在修订后复查。

**D2｜A05“重复页无重复对象”口径与对象去重**
1. 具体问题：A05 要求“重复页无重复对象”，但实现与设计均未定义“对象”口径（source_record 行 vs object_ref 对象）；现实现按记录去重、不按对象去重，且对象在去重/校验前创建（F2），产生重复与孤儿对象。
2. 证据：同页二同步 object_ref 2→4；冲突记录孤儿对象=1（§4 F2）。
3. 方案（建议）：明确 A05 口径=object_ref；实现“先判定新记录再存对象”，必要时内容寻址或孤儿清理；如设计确有意保留逐次对象，则修订 A05 措辞并注明依据。
4. 结论：**需修订（口径+实现二选一，倾向按“去重后仅存新对象”落地）**。

## 6. 未覆盖与边界

- A02 幂等（receipt/ledger，核心由 root core_repo 实现）与 A06+、A08/A09/A11+ 等不在本轮范围；A21–A30 阶段未到。
- A24 文档检查器（docs/checks）本轮未运行；DTO 生成一致性仅完成漂移检测，归零验证待 F8 修复后补。
- 多进程（真 P1/P2 二进程）级 CAS/锁竞争未测（建议纳入 M3 集成测试）；platform 锁仅验证同进程内 flock 互斥。
- 缺陷对象完整性抽查（ReadObject 哈希/路径防护、篡改检测）通过，但 evidence 引用对对象**文件**层的深度校验未逐项展开。
- 项目在本报告冻结后仍持续改动（其他模块并发进行），本轮结论仅对标 §1 快照。

## 7. 声明

本轮未运行任何项目目录内代码、未修改任何项目文件（全部动作在 /tmp 隔离副本）；未读取 resource/ 密钥内容、未联网外传、未连接 ELIZA。以上全部“证据/复现”均为我实际执行所得，未采信实施 agent 的转述结论。

---

参考资料：`docs/Acceptance.md`（A01/A03/A04/A05/A19 行与 §1/§5）、`docs/API_*`（略，见 §1 文件清单）、快照哈希见 §1；证据日志路径见 §1 末。
思路：以 Acceptance 条款逐条对照冻结快照，先复跑声称全过的测试、再用独立探针覆盖“测试未断言”的边界（冲突信息、对象去重、tombstone、CAS 影响行数、迁移回滚），凡结论均附可复现步骤；对超出实现层面、需契约/文档变更的问题，按授权走 D1/D2 商议流程。
