终审完成 ( _ _ ) 判决与证据如下。

════════════════════════════
一、判决
════════════════════════════

PASS — Issue2 AUTH-01…08「后端定向证据」范围。无 mustfix。
前提边界（不越界）：本判决只覆盖本轮 AUTH 后端 + BUILD/vet + TUI 重叠面的代码对齐检查；不覆盖 TUI 运行时验收、DOC 全量、月回放、旧 Context/Store 全仓、长期门槛。作者报告的 8 项 map 与 source hash 经实跑与重算全部对上，未以作者 PASS 代审。

我实跑（唯一 selector，未跑整包旧 tests）:
  cd src && go test -race ./store ./tests -run '^TestIssue2AUTH' -count=1 -v
  → exit 0；store 2.730s / tests 5.713s，9 个测试函数全 PASS（含 TUI13 的 PERSONAL/SECRET 两个子测试）
    store: LegacyExplicitMappingAndStrictReplay / MigrationRequiresBothRolesStopped /
           SummaryStopsBeforeQueuedPhysicalGap / FrozenPrefixPersistsAndFiltersRetrieval
    tests: SharedAuthorityAndFrozenOrdering / SharedQuestionsRestartAndWatermarks /
           TUI13PublicEntryDisclosureAndOmittedSession / InternalTaskContextIsNotSecondConversation /
           HistoryQueryParametersStrict
  cd src && go vet ./store ./core ./context ./memory ./transport → exit 0（无输出）
  stdout 归档: /tmp/i2-test.log、/tmp/i2-vet.log、/tmp/i2-hashes.txt
  另有复核副本: review/issue2-auth-final.ayanami-test.stdout.log、review/issue2-auth-final.ayanami-hashes.txt

════════════════════════════
二、源码 hash（实算，与报告逐条一致）
════════════════════════════

002_authority.sql 122b72f6e67b2357…9150bcb
authority_repo.go 66b4716fa2706c9c…6a6f754
store.go 5ed8843396a0ad29…63a3d29（全值见归档文件，17 项 0 差异）
core_repo.go 9a19ad71b2c53634…3b544e8b6 / questions_repo.go dd2be8d52868b0d2…7c67f275
memory_repo.go 307fc494c1da3080…75bba274 / retrieval.go 573aa8e2258cc89a…4324fd9
backup.go e8dfe2a21faa0cd8…054e145b2bd / core.go 34c5befd946ad450…a200400c3
http.go f737238a38d945ba…9c1d5e94e / typed_http.go e4d3f254d4e0d3cf…f98a4f3afb92b7
work.go bc0414994fa0e0f9…b482de7c4b27 / context.go 6959f0b84c56725…c6d2551bb796
memory.go 75a4ef127320d3d1…a806945b14 / transport.go 36d6a7fffe694fbc…e1abaa9a74d19
store/issue2_authority_test.go 816764ab1e86d6f7…c0fd15a092782
tests/issue2_authority_test.go 43fc69723a4bede0…ffc7f9bbdb76

════════════════════════════
三、逐项核对（读了实现，不是只看报告）
════════════════════════════

1. 版本/迁移：Open 与 backup 共用同一 validateMigrations（store.go:104 / backup.go:275 / authority_repo.go:150）；未知 v3、断档、checksum 不符一律 MIGRATION_MISMATCH；仅 v1 库拒绝打开并要求 migrate。UpgradeAuthority 取 run/core.lock、run/runner.lock、migration.lock —— 与 secretaryd 实际角色锁路径（cmd/secretaryd/main.go:49,58, role∈{core,runner}）一致，取不到即 AUTHORITY_MIGRATION_BUSY，不 kill。
2. 选择语义：无任何 master/MASTER_CLI 历史才自动新生随机会话；有历史必须显式选择，internal-only 拒绝；旧 payload 逐字节不变（测试断言 before==after）；legacy 一律 READ_ONLY。
3. 受理与重放（C2/C3 更正版）：三入口共用 authoritySessionTx；精确重放先于 authority 检查；显式外来 session 只按实际提交 hash 判定（IDEMPOTENCY_CONFLICT / 403 AUTHORITY_SESSION_MISMATCH），不回退不改写；仅 HTTP raw 字段缺失才走 ResolveSession（存在性判定在 Decode 之前，http.go:85-93 / typed_http.go:38-42）；显式 null 在 /v1/actions 与 typed 路由直接 400，/v1/inputs 的 null 经 AssertFormat+uuid 格式判为 INVALID_SCHEMA。
4. 单消费者/冻结边界：consumer flock 覆盖整轮+摘要；FreezeTurn/FinishTurn 强制 head=min(accepted_seq)（AUTHORITY_TURN_NOT_HEAD，409）；MASTER 受理不再推 revision，只有持锁的终态 ASSISTANT 与摘要 CAS 推进。attempt 上限 3、仅在 CONFLICT 上重试，不会无限打回。
5. prefix eligibility：同一个 prefixPredicate 同时作用于 prompt 事件（memory_repo.go:175）、分类采集（:253-258，按 turn 状态排除 future）、READ_MEMORY（retrieval.go:59-70，rowid 上界钳到冻结 RowID）；冻结边界与 frozen_state 持久化在 authority_turn，重开后边界不动（测试实测）。前轮终结失败（含失败文本终态）纳入 eligible，predicate 同时保留 FAILED 兜底。
6. 队列与旧 pending：计数/PendingTurns 限定权威会话并按 accepted_seq 排序；限额 1 时旧库仍有 pending 也能受理新请求，PendingTurns 为 0 —— 旧 readonly pending 不占 queue、不被消费。
7. 摘要：SummaryContext 遇首个 queued 物理事件即设界；SaveConversation 额外断言 claimed 区间不含非终结 turn（AUTHORITY_SUMMARY_GAP），CAS + 连续覆盖保留；摘要不跨模型等待持 DB tx。
8. TaskLocal：Builder 仅在 Origin==SYSTEM 时可用，否则 AUTHORITY_TASK_CONTEXT_DENIED；走 AuthoritySession+SummaryContext+Snapshot 只读，不建 conversation_session、不推 revision（测试断言 revision/history 不变、legacy_sessions 空）；公开 MASTER 无法进入该模式。
9. 失败回执稳定性：AUTHORITY_BUSY(409，零受理副作用，不消耗 request_id)、AUTHORITY_TURN_NOT_HEAD(409)、AUTHORITY_SESSION_MISMATCH(403)、QUESTION_ALREADY_RESOLVED 稳定文本回执、BACKPRESSURE(429) 与 BUSY 代码可区分；重放不重跑模型（实测 replay 后请求数不变）。

════════════════════════════
四、TUI 状态（不冒称已验收）
════════════════════════════

TUI 侧仅做代码对齐核对：src/cli/tui 经 tui.Resolve 从服务端解析 authority（client.go:89）、信封使用 authority.SessionID（model.go:285）、有实例漂移检测；go build ./cli/... ./cmd/... exit 0。但本轮 selector '^TestIssue2AUTH' 不会执行 TestIssue2TUI*（8 个），vet 清单也不含 cli 包 —— 因此 TUI socket/PTY 生命周期与 TUI 测试结论本报告不宣称通过，仍由 runtime 侧单独记录。

════════════════════════════
五、非阻断观察（不构成 mustfix）
════════════════════════════

1. MASTER 历史判定过滤 principal='master' AND origin='MASTER_CLI'（authority_repo.go:87,96）。仓库内所有 master 受理路径都写这一组合（HTTP 强制、Typed 硬编码、CLI/tools），F2 成立；仅当有人手工造出其它 origin 的 master 行时该判定会视为"无 MASTER 历史"，属语义边界选择，不扩大本轮判负。
2. READ_MEMORY 的 `attempt--`（core.go:148）使检索轮不消耗 3 次尝试，边界由每次检索的 ChargeBudget 保证有界（BUDGET_EXHAUSTED），与设计"有界"一致，非无限循环。

参考资料（工作树路径）：reports/implementation/issue2-auth.md / .json；review/issue2-auth-design.response.md、review/issue2-auth-design-correction.response.md；src/store/authority_repo.go:32-189,240-350；src/store/core_repo.go:16-110,349-408；src/memory/memory.go:170-260；src/context/context.go:36-50；src/transport/transport.go:27-58；证据归档见上文 /tmp 与 review/ 两个路径。

思路：先重算 17 个 hash 并实跑唯一 selector 与 vet，确认报告基线自身可核；再按"证据能反驳什么"逐个读实现——重点放在最容易假通过的三处（显式/省略 session 的 hash 分支、prefix 三处同判据、CAS 与队列收敛），用测试断言反查（旧 pending 占 queue、future 分类/检索泄漏、BUSY 零副作用、replay 不重跑模型）；TUI 只做静态对齐与编译，明确划出未验收面。
