( _ _ ) 复核完成。以下为本轮完整报告内容，请 Codex 保存至 `review/foundation-world-diagnostics-deepseek-final.response.md`（我方不另写同名文件，避免双录）。

---

# 精简合并终审：M1 foundation / World 权限修复 / Diagnostics 备份 / artifact v2 探针 / A19·A23 契约口径 — DeepSeek v4.1 Flash

reviewer_model=deepseek-v4.1-flash / provider=opencode-go；未委派、未换模型；未运行全仓 test/race/vet；本轮未出现 429。旧 Luna 结论仅作线索，未采信；以下全部结论基于本会话内真实执行的命令输出与我本人的代码阅读。业务代码只读，全部写入仅在 /tmp 副本与 /tmp 日志；项目目录零写入（git status 未因本轮回合变化）。

## 0. 裁决摘要

**三个模块：定向范围内 CONDITIONAL PASS → 无 must-fix 发现，收口缺口仅剩我明确列出的未检项。** 原 F1–F8（收口曾 Luna）经我独立复核：修复全部在位且有直接测试证据；World 授权链（跨 run 身份绑定 / durable fence / receipt CAS / INFERENCE vs MASTER_EXPLICIT / 零版本拒绝）实测通过；Diagnostics 差异项（此前 transport 轮已跑）经哈希比对确认对当前代码仍有效，另补边界探针 PASS；artifact v2 探针原样执行两次，**outside 零逃逸、正常控制写成功**（组件级全拒绝按约定不计为逃逸）。**A19**：可在“仅 baseline v1”范围记 PASS，“旧数据迁移”当前无对象、不得宣称通过。**A23**：§3 字面**无“同一 DB 30 天内”要求**（不加集成门槛）；组合固定集的“预冻结 oracle/范围”要件当前需补一份冻结映射记录，或做最小整合运行——二选一由协调方裁决。

## 1. 快照与证据基础

- 环境：macOS 15.7.4 arm64；go1.25.6 darwin/arm64；评审时点 2026-09-14 08:04–08:12 HKT。
- 受检快照：`/tmp/ffinal_ayanami_20260914_080420/src`（自项目树复制；**30 个受检文件逐一与当前项目逐字节一致，无漂移**）。日志：同目录 `logs/{m1_contract_platform,m1_store,m1_ingest,world,diag_supp}.log`。
- Artifact 探针目录：`/tmp/artifact_v2_20260914_080444`（探针 sha256 前缀 `bf96628bd3e6e321…`，`executor/artifact.go` = `5f4a93a66517d77d…`，与 `/tmp/m3_deepseek_final_20260913_235347` 快照逐字节一致）；日志 `artifact_probe.log`。
- 关键文件 SHA-256 前缀（16 hex）：foundation_test `4c231a2a80`，items.go `739bbacc97`，store.go `30db6a254b`，sources.go `7a6462413b`，ingest/fixture.go `032967bd43`，fix ture_test `d51b11231a`，world_repo.go `1b3f3ea51c`，work_repo.go `e0d14d3cb7`，core/work.go `6c67b716cb`，core/core.go `c65706344a`，backup.go `f4740df17d`，health.go `a63d83a97b`，diagnostics.go `75246f2711`，verify.go `ee14393821`，artifact.go `5f4a93a665`，generate.py `4278aff8ec`，dto_generated.go `2ad6edbb53`。
- schema/policy 版本：1 / 1；入口构建（cmd/secretary + cmd/secretaryd）BUILD_OK（快照内）；未使用任何凭据，报告不含秘密字段。

## 2. M1 foundation（contract / platform / store / ingest）

执行（快照内、-count=1、定向）：`go test ./contract ./platform -run 'TestStrictContract|TestSchemaClosure|TestDuplicateKeysAndCanonicalGolden|TestLockAndClock'` → **ok**；`go test ./store -run 'TestItemCASCycleAndAtomicEvent|TestObjectTamperAndConnectionPragmas|TestConflictCarriesRevisionAndSourceCASRollback|TestInputHashMatchesStoredSemanticEnvelopeAndRejectsNoObject|TestAcceptanceA03…|TestAcceptanceA04…|TestAcceptanceA19…'` → **ok**；`go test ./ingest -run 'TestFixtureDedupConflictAndQuarantine|TestTombstoneAndNoDuplicateObjects|TestAcceptanceA05EmptyPageAndFailurePreserve'` → **ok**。

原 F1–F8 逐项独立复核：

- **F1｜CAS CurrentRevision — 修复确认（PASS）**。`items.go:13-19` 结构化 `RevisionConflict{CurrentRevision}`，`:96-101` 0 行更新在 tx 内回读当前版本；`TestConflictCarriesRevisionAndSourceCASRollback` 实测通过（current=2）。
- **F2｜重复/被拒记录不再建对象 — 修复确认（PASS）**。`fixture.go:52-66` 先 LookupSourceVersion 去重、再校验、**仅新且合格**才 PutObject（`:81`）；测试断言 object_ref 计数不因重复/冲突/quarantine 增长（`fixture_test.go:59-72`）。
- **F3/D1｜tombstone — 设计修订已落地（PASS）**。`contracts.schema.json` SourceRecord.allOf 现为 VALID⇒validation_error=null；deleted=true⇒normalized 允许 null，否则必须 FixtureItemValue；`fixture.go:71-88` 对 deleted 跳过解码并置 VALID；`TestTombstoneAndNoDuplicateObjects` 实测通过。
- **F4｜quarantine 去重 — 修复确认（PASS）**。内容寻址文件名 + `O_CREATE|O_EXCL`（`fixture.go:101-125`）；测试：两次隔离后文件恰 1 个。
- **F5｜SourceSynced RowsAffected — 修复确认（PASS）**。`sources.go:84-90` affected!=1 → `STORAGE_CORRUPTION`，事件不再泄漏；测试（SQL revision 篡改为 5）实测拒绝且事件数不变。
- **F6｜UpdatedAt 归一化 — 修复确认（PASS，代码级）**。`items.go:36-40` 显式 `INVALID_UPDATED_AT` 包装。
- **F7｜半初始化库 — 修复确认（PASS，代码级）**。`store.go:96-99` `INITIALIZATION_INCOMPLETE` 明确指引。
- **F8｜生成物/依赖一致性 — 修复确认（PASS，实测）**。在隔离副本以 `generate.py` 重生成：`dto_generated.go` 与提交版**逐字节一致**（DTO_IDENTICAL），schema 副本一致；`go mod tidy`（GOPROXY=off）后 go.mod/go.sum **零漂移**。
- **事务/事件 RowsAffected、timestamp/初始化、生成与依赖**：item UPDATE 行数校验（`items.go:96-101`）+ AppendEvent 同事务；A03 单胜者、A04 全有或全无、A19 baseline 回滚/版本拒绝均实测通过。**M1 无待商议项。**

## 3. World 权限修复（store/world_repo·work_repo + core/work）

执行：`go test ./tests -run 'TestIncomingPermitBoundToExactRun|TestStaleWorkerCannotCommitReceipt|TestWorldPermitCorrectionAndHistory|TestWorldCandidateDoesNotOverrideAndConflictGroupUsesPreferenceKey|TestAcceptanceA06WorldPermitDenialMatrix'` → **ok**。

- **跨 run permit/命令身份绑定 — 确认（PASS）**。`work_repo.go:111-158` 全 DTO 对照（command JSON、fence、attempt、state=RUNNING、task、external key、occurrence key）+ permit 全 DTO + RunID 绑定 + 过期 + grant 复检 + grant revision/cancel generation；handler 层三绑（`core/work.go:43-46`）。伪造 run → 403 且 core_work 零残留（实测）。
- **durable fence / attempt receipt CAS — 确认（PASS）**。`BeginWork`（幂等短路/重试重置/IDEMPOTENCY_CONFLICT）与 `FinishWorkTx`（fence+attempt+receipt 身份 + **RowsAffected==1**）实测：fence+1 后提交被拒、状态保持 RUNNING。
- **Model INFERENCE vs 认证 Typed MASTER_EXPLICIT — 确认（PASS）**。`core.go:174` 模型路径 trusted=false ⇒ `:381-382` 强制 Basis=INFERENCE（CANDIDATE，不覆盖 ACTIVE）；`:438` Typed 路径 trusted=true 且 Origin=MASTER_CLI，`:384-386` MASTER_EXPLICIT 非 MASTER_CLI 一律 PERMISSION_DENIED。冲突组按 preference key 归组、撤回/RETRACTED 历史保留（3 版本）实测通过。
- **policy version 与零版本 CORRECT/RETRACT 拒绝 — 确认（PASS）**。`world_repo.go:15-17` PolicyRevision!=1 → `POLICY_REVISION_MISMATCH`；`:18-20` ExpectedRevision==0 且非 ASSERT → `CONFLICT`。
- **事务 finalize 回滚 — 代码级确认（CODE-LEVEL，无注入式测试）**。world.update 的 permit 消费与 receipt finalize 均在 `CommitWorldAtomic` 同一事务内（`core/work.go:74-79`；`CommitWorldAtomic` 经 `s.Write` 传播 finalize 错误 ⇒ 整事务回滚：事实/提案状态/事件/回执/permit 全有或全无）。消费后重复提交被拒已实测；**finalize 注入失败路径无单测，如实记为未动态验证**。
- **F5 幂等口径（按指示）**：幂等性成立于 **Typed 整体事务**（注册同事务）+ 执行侧 BeginWork receipt 短路 + 提案 PENDING CAS，不把 `CommitWorldAtomic`/`ConsumePermitTx` 等内部 primitive 当作公网入口；未从内部方法虚构任何公共面。

## 4. Diagnostics / 备份（backup·health + diagnostics）

- **不重跑已跑项**（transport 轮 3 backup + 2 doctor PASS）：已核 `/tmp/m3ds/snap/src` 中 `backup.go/health.go/diagnostics.go/verify.go` 与当前**逐字节一致** ⇒ 其 PASS 对当前树仍有效。
- **我补的边界探针 PASS**（`zz_ayanami_supp_test.go`，隔离副本，见 diag_supp.log）：①manifest `created_at` 带 `+08:00` 偏移 → **VerifyBackup 拒绝**（UTC 规范化强制）；②未知顶层字段 → **拒绝**（字段数+DisallowUnknownFields 双重）；③成功备份后 `backup.latest.json` 与**数据库哈希绑定**；④无记录时 health 的 budget/overdue **不发明状态**（budget=UNKNOWN 文案 + nil、consciousness=UNKNOWN/overdue=nil）。
- **metadata 完整绑定 / strict manifest / 末次 verify**：`VerifyBackup` 逐对象与 object_ref 行 7 字段对照 + 文件哈希/大小复算 + FK/migration/计数（backup.go:204-310）；`Backup` 结束即调用 `VerifyBackup`，失败写 `INCOMPLETE` 且不更新最近备份记录（**失败分支为代码级，未注入**）。真实 epoch 由 config 驱动 `DoctorWithEpoch`（epoch 缺省 → “UNKNOWN: configured epoch was not supplied”；overdue=期望槽>实际槽），由已跑的 `TestDoctorEpochAndBackupHistory` 覆盖（未重跑）。

## 5. artifact v2 探针（原样执行，未改 production / 未改探针）

- 结果（两次连续运行，count=1）：**整体 PASS；outside 目录零逃逸（两阶段断言均过）；正常控制写与相位(1)-(3)全过**。
- **组件级（sub swap race，相位4）：0/250 写成功、0 逃逸**（两次均 0/250）——按既定口径，“竞态期间全部拒绝”**不计为逃逸或失败**；该子树在攻击持续期表现为保守拒绝。
- **根级（root swap race，相位5）：237/250、243/250 写成功、0 逃逸**（描述符锚定 + SameFile 复核下，根被换链期间写入要么被拒、要么落在原 inode，绝不落 outside）。
- 说明：探针自设的最严断言原样保留执行，本轮无 FAIL 输出；两次原始输出见 `/tmp/artifact_v2_20260914_080444/artifact_probe.log` 与 `ffinal…/logs`；生产 `artifact.go` 与探测目标哈希一致。

## 6. A19 契约口径（设计解释，与测试通过分开）

- **范围裁决**：A19 四子项中“旧数据迁移”在当前唯一 schema=1 的新项目**无适用对象（N/A）**——不得宣称“旧数据迁移通过”，也不得因无对象而记 FAIL。**适用范围内的 PASS**：新文件库 0→1 初始化、注入 DDL 失败回滚（零表残留）、不支持版本/校验和拒绝、JSON↔SQL 重复字段一致。证据：本轮 `TestAcceptanceA19BaselineRollbackAndVersionRefusal` PASS（含 version=2 拒开、DDL 注入回滚子测试）+ 镜像一致性断言。未来任何 v2 迁移须自带迁移/回滚测试，不能沿用本结论。

## 7. A23 契约口径（设计解释 / 真实测试状态 / 缺口与最小修订条件）

**(A) 设计解释 — 据 docs/Acceptance 实际条款**：§3 全文**没有**“每个变化/冲突必须同一 DB 30 天内”的字样，也不存在单库集成门槛——**不得凭空新增**。字面可绑定要求共 5 条：①冻结数据集（input/oracle 分离、作者撰写 oracle、冻结 manifest/种子/版本/哈希）；②30 天模拟回放、**每日至少 3 个变化点**；③每日意识由生成路径实际运行并保留输入/原始模型输出/校验/提交/最终 Context；④关键字段按 ID/枚举/时间/null/**冲突集合**严格比较、不做事后归一化；⑤M3 真实模型门槛=**固定集全部通过**、保留失败与重试。自由文本摘要按 §3 **允许抽样**。组合固定集以多个隔离序列执行，在字面上不违反条款；真正不可动的要件是“**预冻结的 oracle/范围**”——组合集若要充当评测单元，其组成与覆盖映射必须先被冻结为文档化单元，且不得选择性拼装（只取通过、丢弃失败）。

**(B) 真实测试通过（截至本次审计，仅结构/计数核验，未重跑模型调用）**：month-run5 90/90 item checkpoints + 30 快照，failures=0，独立身份审计 PASS(90)、意识引用审计 PASS(30)；month-run4 同规格（原报告保留、独立修正审计）；scenarios-run4 7/7、failures=0；world-read-run2 2/2（固定 801/802 冲突组 + 撤回读取）；失败史完整保留（run2 6/7、run3 中断等）。**这组数字已在附加报告中按序列明确分隔**——符合“不得事后归一化”的呈现要求。

**(C) 确切缺口**：①组合集在数量与类别上覆盖 §3 语义清单（四领域×5、增/完/改/取消、未知时间、冲突、撤回、过期、依赖、先前会话），但其多序列结构使其**不能宣称“整合 30 天世界史”**（附加报告已自我声明）；②“组合评测单元”的冻结映射（每 case 的 oracle 哈希、覆盖语义、全通过状态、失败保留清单）尚未以单一验收记录冻结——这是当前唯一实质缺口；③world 变更本身为冻结 set-up（读语义已由真实模型评估，授权语义属 A06 范畴）——这一分工需在解释记录中写明。

**(D) 最小修订商议条件（二选一，由协调方裁决；我不单方面扩缩验收）**：**选项1（零新运行）**：裁定“组合固定集”为评测单元，补齐冻结映射记录（映射 + 哈希 + 全通过 + 失败保留），A23 按组合范围定稿——属解释记录，不改 §3 文字。**选项2（最小新运行）**：若一方坚持“回放=单一序列”的整合式阅读，则在 month 回放内追加世界冲突/撤回/过期/依赖若干天并重跑受影响固定集（新 LIVE_MODEL 运行 + 新冻结回放哈希）。**两选项之外的“事后拼集替代预冻结范围”均不成立**；我建议选项1 并明示范围限制（含抽样限制），设计解释（A）与测试通过（B）保持分列。

## 8. 未覆盖 / 不宣称

- 未重跑 transport 轮已跑测试（按要求）；未做全仓 test/race/vet（禁止）；未运行任何 LIVE_MODEL 调用（A23 运行属存档核验，未重演）。
- finalize 注入故障（World）、F7 安全重试行为、backup 末次 verify 失败分支：代码级，未动态注入。
- F6 端到端错误来源与 F8 工具链仅验证到重生成一致，未做 gofmt 全仓归零扫描。
- A23 各 live 运行的逐 checkpoint 原始数据未逐条重放（本轮仅核结构/计数/审计状态）。
- D08/D09 及其他增量不在本请求范围；平台锁仅同进程 flock 用例（历史已核）。

## 9. 合规与原始错误

未读取 resource/ 密钥；未连接 ELIZA；未触碰受测 API、响铃或系统设置；唯一写入在 /tmp 副本与日志；项目树零改动。本轮无 429、无 provider 报错。本回复经 Codex 保存至 `review/foundation-world-diagnostics-deepseek-final.response.md`（我方未写同名文件）。

---

参考资料：docs/Acceptance.md（A19 行 :29、A23 行 :33、§3 :44-48、§5 :58）；review/M1-foundation-ayanami.md（上轮，线索）；review/M3-transport-cli-deepseek-final-resume1.response.md（已跑项）；reports/implementation/A23-supplemental-evidence.md 与 acceptance-matrix.md（A23 行）；证据：`/tmp/ffinal_ayanami_20260914_080420/{logs,src}`、`/tmp/artifact_v2_20260914_080444/artifact_probe.log`。

思路：先冻结哈希与隔离快照，再逐修复点“代码+断言”双重对照，凡可动态验证的一律在副本真实执行（F1–F8、World 授权链、diagnostics 边界、artifact 探针）；对解释类问题只做条款比对与范围裁决并区分“设计解释 vs 测试通过”；未检项全部如实列出，不耗工具上限、不越授权。

( _ _ ) 一句话：**M1/World/Diagnostics 修复全部坐实、artifact 零逃逸且控制写正常；A19 可限缩记 PASS，A23 只差一份“组合固定集冻结映射”（或最小整合运行），除此之外无 must-fix。**
