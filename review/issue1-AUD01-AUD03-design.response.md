Ayanami 本地复核输出（静态设计商议，未实施、未跑测试、未改动任何文件）

0. 本轮事实基线
- 模型/执行：deepseek-v4.1-flash / opencode-go（与持久配置一致），本回复全程无网络模型调用 → 未出现任何 429，无需切换。
- 只读范围：review/issue1-original.md 的 AUD-01（L28–53）、AUD-03（L85–107）；src/store/objects.go 全文（107 行）；core_repo.go L1–110、L320–394；store.go L28–122；另加两批只读事实核查（PutObject 调用点清单、工具链能力）。未扩全仓、未委派、未做动态复现。
- 工具链事实（可复核）：go1.25.6（go.mod `go 1.25.6`）；`os.Root.Link` / `os.Root.Rename` / `os.Root.Remove` 均存在；DSN 已是 `_txlock=immediate` + `busy_timeout(5000)` + `synchronous(FULL)` + `SetMaxOpenConns(4)`（store.go:36,40）。
- 静态裁决：AUD-01、AUD-03 在代码层成立。证据：objects.go:42 `O_CREATE|O_EXCL` 直写最终名；写入/Sync 失败即返回且无回收路径；objects.go:43–49 已存在文件哈希不符 → `STORAGE_CORRUPTION: orphan object hash`（无 ref/有 ref 不区分，且同内容永久卡死）；core_repo.go:46 与 342 的 `archiveInput()` 在 `s.Write` 之前（L50/L346），PutObject 的 ref 插入走 `s.DB.ExecContext` 独立 autocommit（objects.go:78）→ 队列满（core_repo.go:79–81）或 Typed apply 失败回滚后文件与 ref 仍持久。
- 未写入任何文件；`review/issue1-AUD01-AUD03-design.response.md` 由 Codex 保存（本回复即其内容），我方不另写同名报告。

1. Foundation 提案裁决：方向成立，条件式通过（6 条必须修订项见 §9，均为事实级修正，非风格意见）
提案骨架（同库写事务 + 同目录 temp + hard-link no-replace + ref 同 tx + 区分孤儿/损坏 + 无 DDL）与 AUD-01/03 验收项对齐，可实施。但提案有 3 处未定死的顺序/边界会被实现成不安全或不必要的形式（temp 写入是否在锁内、link ENOENT 与清扫竞争、filesystem 不支持 hardlink 时的行为），必须显式固化。

2. 需要落成文字的 6 条不变式（协议核心）
I1 最终名只在写锁内出现：`<id>.blob` 的创建只允许通过 `link(temp, final)` 且发生在 BEGIN IMMEDIATE 事务持锁期间；禁止任何路径直接 `O_CREATE` 最终名。
I2 数据先于名字：temp 完整写入（校验 write 计数与 err，处理 short write）→ `f.Sync()` 成功 → `f.Close()` 成功，三者任一失败即中止且不 link（Sync 失败一律视为致命，不得降级）。
I3 名字先于引用：link 成功 → `ObjectsDir` 目录 fsync 成功 → 同 tx `INSERT INTO object_ref` → 之后才允许 COMMIT。
I4 引用先于业务：object_ref 与 input_turn/request_receipt/conversation 写入、Typed 业务行必须同一 tx（AUD-03）。
I5 已提交即不可触碰：ref 存在时，任何路径（含持锁内）不得覆盖、删除、重命名该文件；发现哈希/字节不符只能拒绝并告警。
I6 反向蕴含（崩溃可恢复性的全部依据）：提交成功的 ref ⇒ 文件完整且 durable；无 ref 的 final ⇒ 只可能是"完整对象（可采纳）"或"旧版遗留坏文件（隔离重建）"，不存在第三种状态。

3. 发布协议（可实施条件，含锁顺序）
阶段 P0（锁外，纯 I/O，不持 DB 锁）：算 id → 建议性快路径（ref 存在且 ReadObject 校验通过 → 直接返回；任何不一致一律穿透到 P1，不做任何修复动作）→ 写同目录 temp `tmp-<pid>-<rand>`（`O_CREATE|O_EXCL|O_WRONLY`,0600）→ 完整写 + Sync + Close。失败：尽力 `Remove(temp)` 并返回错误（残留 temp 由恢复清扫兜底）。
阶段 P1（`s.Write`，BEGIN IMMEDIATE，跨进程唯一写锁）：按提案顺序——(1) tx 内幂等复查（receipt → payload_hash 一致则返回既有 turn）、(2) `CheckAnswerTarget` 权威检查、(3) 队列容量 count 检查（core_repo.go:76–81 原地保留）、(4) 以上任一拒绝 → 直接返回错误（此时尚未 link，无锁内回滚负担），(5) ref 复查：
  5a ref 存在 → tx 内 `ReadObject` 校验（长度+sha256）；通过 → 返回既有 ref（temp 于锁外清理）；不符 → 返回 `STORAGE_CORRUPTION: committed object mismatch`，保持文件原样。
  5b ref 不存在、final 存在 → `Lstat` 必须为普通文件（symlink/特殊文件一律拒绝并告警，不得跟链）；哈希匹配 → 采纳（不 link，直接进 5c 的 ref 插入）；不匹配 → 隔离（见 §4）后 link。
  5c ref 不存在、final 不存在 → link(temp, final)；EEXIST → 落回 5b 复核（不得盲信）；ENOENT → temp 被清扫（见 §7），限次重写 temp 重试 ≤2 次，仍失败则报错。
(6) → ObjectsDir fsync → `INSERT object_ref`（普通 INSERT；违反唯一键=协议违约，映射为显式内部错误并告警，绝不让原始 UNIQUE 冲突外泄）。
提交：`Commit()` 错误=结果不确定 → 见 §5，不删文件。
锁持有时间界：锁内只允许 link + dir fsync + 1 条 INSERT；字节写入与 temp fsync 一律在 P0。理由（事实）：busy_timeout=5000ms，若把 MB 级 fsync 放进锁内，对端 BEGIN IMMEDIATE 会被推到 SQLITE_BUSY——这不是正确性问题但会变成假失败，且与"两进程并发不暴露未处理错误"的验收项冲突。

4. 坏孤儿 vs 已提交损坏：必须双轨（错误码拆分）
- 判据是 ref 存在性（在写锁内判定，锁保证判定与动作之间无合规写者介入），不是文件内容。
- ref 存在 + 文件不符 = 已提交损坏：拒绝读写、保留证据、不隔离不重建；`ReadObject`（objects.go:103–105）行为保留；`PutObject` 同内容必须失败（不得静默重建）。现 objects.go:49 的错误文案把两类混为一谈，必须拆成两个可区分错误码（committed corruption / orphan recovered）。
- ref 不存在 + 文件不符 = 坏孤儿（仅可能来自旧版半写入或外部篡改）：写锁内隔离到受控唯一名 `<id>.blob.orphan-<utc>-<rand>`（`Root.Rename`），再 link 本次 temp 并插 ref。隔离名不得匹配 `.blob` 规范形态，避免被 ReadObject/去重路径当作正式对象。`RelativePath` 仍为 `<id>.blob`，无 DDL。
- ref 不存在 + 文件完整 = 采纳复用（提案"完整无ref验证复用"确认可行：id 是内容派生，任何后续同内容请求都可采纳）。
- 篡改场景（ref 存在但文件被改）必须与旧版半写文件得到相反处理，这是 AUD-01 验收第 4 项与第 2 项的分界线，测试须分别断言（§10 t2/t3）。

5. rollback / commit-unknown cleanup 边界（三条互斥规则）
R-a 普通回滚（闭包返回错误，锁仍持有）：只删除"本 tx 自己 link 出来的路径"，在 `s.Write` 闭包内以 defer 执行（unlink + dir fsync），此时 ref 插入随 tx 回滚、且写锁排除其他合规写者 → 删除安全。禁止在 `Write` 返回后再删（锁已释放，对端可能已 link/采纳 → 会删掉对端刚提交的 ref 所指文件）。
R-b P1 前置拒绝（容量/幂等/answer）：从未 link，唯一动作是锁外 temp 清理，正式 ref/file 零残留（直接满足 AUD-03 验收 1/2 "不新增正式业务对象引用"，且比验收更严：也不新增文件）。
R-c commit-unknown：不删、不猜。记录事件并走恢复：重新取写锁查 ref——ref 存在且校验通过 → 保留；ref 缺失 → 文件为"完整无 ref"孤儿 → 留作可采纳（不强制回收，避免丢 I/O 与误删）；temp 照常可清。AUD-01 验收第 3 项（两进程并发不暴露未处理冲突）由此路径保障。
边界外的禁令：不得为"看起来更干净"而在锁外删除任何 `<id>.blob`；不得回滚隔离动作（隔离是单向的，坏证据不恢复）。

6. AUD-03：容量检查与受理顺序（预查不可替代最终事务）
- 最终准入 count 必须留在 `acceptInputTx` 同 tx 内（现状已正确，改动中不得把它挪到 tx 外或"归档前查一次"）。
- 顺序固定为：tx 内幂等 → answer → 最终 queue → PutObjectTx + Input/Typed 业务同 tx。core_repo.go:46 / 342 的 `archiveInput` 迁入 tx（改为 `archiveInputTx(ctx, tx, ...)`，内部用 PutObjectTx）；必须删除原锁外调用，不能保留双份（残留旧调用=漏洞复现）。
- tx 前已有的 receipt 预查（L27–42）与 `CheckAnswerTarget(s.DB)`（L43）降级为纯优化，注释明确"advisory only"，权威判断一律在 tx 内。
- Typed：apply 失败 → 走 R-a，业务行、request_receipt、input_turn、conversation、object_ref、file 全无残留；成功后同请求重试仍只产生一次业务效果（依赖既有 CommittedOperationKeys 语义，不在本 issue 扩改）。
- 并发最后一个名额：两个进程各自 BEGIN IMMEDIATE 串行化，后者读到已提交的 count → BACKPRESSURE；这是"不可用预查替代"的机械保证，须有测试（§10 t9）。

7. 孤儿/暂存 bounded + 有据回收（无 DDL 前提下的全部手段）
- 命名即账本：`tmp-<pid>-<rand>`（在途）、`<id>.blob.orphan-<utc>-<rand>`（隔离证据）。无新表、无新列。
- 在途 temp 有界：每次发布至多 1 个，所有退出路径（成功/拒绝/回滚/取消）都清；崩溃路径每次崩溃至多遗留 1 个 → 上界=崩溃次数，由清扫收敛。
- 清扫（启动/显式 recover，必须在写锁内）：只删 `tmp-*`，且满足"年龄 > GRACE 或 持有 pid 已不存活"（pid 编码在名内）；单次运行设上限并记日志；绝不触碰 `<id>.blob` 与 `orphan-*`。
- 死锁式竞争修正（提案缺口）：清扫与对端"锁外写 temp"可并发 → 对方 link 得 ENOENT → 必须限次重写 temp 重试（§3 5c），否则恢复动作会变成假发布失败。GRACE/上限/重试次数是固定卫生常量（非测试时长门槛），随代码注释与文档登记。
- 隔离物不算暂存、不自动删：它是损坏证据，只在 recover 报告中列出（名/大小/sha256/原 id），删除须显式操作者动作（`--purge-quarantine`）。这与"有配额、可追踪、可回收暂存"不冲突：AUD-03 的增长源已被 §6 的零残留受理消灭，隔离物只在旧版遗留损坏出现时产生（一次性、可枚举），无需配额拒绝路径去牺牲可用性。
- 启动/显式恢复不碰 committed refs（I5）；若确实发现"ref 存在但文件缺失/损坏"，只报告 + 拒绝，不自动重建。

8. 兼容性陷阱（必须显式保留，否则修复本身会造回归）
- `inputSemanticHash` 会剥离自动归档的 text/plain ref（core_repo.go:383–393）：迁 tx 后，存储的 envelope 仍须追加同一归档 ref（保持字段形态与既有库一致），而哈希计算保持剥离 → 修前已入库的 `request_receipt` 在修后同请求重试时不会误判 `IDEMPOTENCY_CONFLICT`。这是硬条件，需专项断言。
- tx 内校验读取必须走 tx 句柄：`GetObjectRef`/`ReadObject` 现在用 `s.DB`（objects.go:81/86），在写 tx 内用另一连接会阻塞或读旧快照 → 需 tx 版变体，锁内一律用 tx 版。
- `s.mu` 非可重入（store.go:110–112）：`PutObject`（将自身包 `s.Write`）绝不可在已持有写 tx 时再调用，否则自死锁。已核查现存 6 处外部调用点（core/work.go:150、executor/executor.go:118、ingest/fixture.go:81、diagnostics/model_records.go:50/91、diagnostics/verify.go:51、tools/livescenarios/world.go:27）均不在任何 `Write` 闭包内 → 本设计当下不引入重入死锁；但这是新增不变量，须登记 + 代码注释 + 复核项。
- 发布 tx 用 durable/persist 上下文（参照 model_records.go:91 的 `context.WithoutCancel`），避免取消把发布事务打成半途（executor.go:118 用 `child` ctx，接入时注意）。
- `objectMu`：不是正确性元素且与 DB 锁构成潜在顺序反转（持 objectMu 等 DB 锁 vs 持 DB 锁等 objectMu）→ 建议直接删除；若保留，只允许作锁内短临界区且绝不跨 DB 获取。
- 非 hardlink 文件系统：不得回退到"copy + rename"（POSIX rename 会替换，违反 I5）。启动探测（temp→link→unlink）失败即以 `STORAGE_UNSUPPORTED_FS` fail-closed 拒绝启动对象写入。

9. 必须修订项（四段式，R=revision）
R1 hardlink 前提未封口 — 证据：提案只写"hard-link no-replace"，未处理不支持 hardlink 的目录（同目录 temp 只解决 EXDEV，不解决 link 能力）— 方案：启动探测 + fail-closed + 文档登记 — 结论：需修订（小幅）。
R2 锁内/锁外边界未定 — 证据：store.go:36 busy_timeout=5000；objects.go 现有 dir fsync 已是锁外行为 — 方案：§3 锁界（锁内仅 link+dir fsync+INSERT）— 结论：需修订（关键）。
R3 清扫与在途 temp 竞争未定义 — 证据：§7 情景 — 方案：年龄/pid 门 + link ENOENT 限次重试 — 结论：需修订（关键）。
R4 tx 内校验读取用了 `s.DB` — 证据：objects.go:81/86、store.go:40 — 方案：tx 版只读变体 — 结论：需修订（关键）。
R5 错误分类文案未拆 — 证据：objects.go:49 单一 `orphan object hash` — 方案：committed corruption / orphan recovered 双码 — 结论：需修订（必需，直接对应 AUD-01 验收 2/4 的可判定性）。
R6 objectMu 去留 — 证据：objects.go:15–19、store.go:110 — 方案：删除（或严格限域，不跨 DB 获取）— 结论：需修订（低风险但须在评审记录中留字）。
（§8 的 semantic hash 兼容与 s.mu 重入属于"确认保留的硬条件"，不是修订项，但须进实现 checklist。）

10. 测试清单（按 issue，不新增任何持续时长门槛）
AUD-01 定向：
 t1 真实退出点注入（子进程 helper 模式，4 个阶段）：S1 temp 半写入中、S2 temp Sync+Close 后未 BEGIN、S3 link 后未插 ref、S4 插 ref 后未 Commit。断言：重启后同内容重试成功、ref 恰 1 条、字节/哈希正确、遗留物类别符合预期（S1/S2 仅 temp；S3 完整无 ref final；S4 已提交或完整孤儿）。
 t2 预置无 ref 的损坏半文件（规范名）→ 同内容提交成功；隔离物存在且记录了名/大小/sha；再重试为去重 no-op。
 t3 已提交损坏：篡改已提交 ref 对应文件（改字节、截断两种）；`ReadObject` 拒绝；`PutObject` 同内容返回 committed corruption；文件字节与位置未被改动（无删除、无重命名、无覆盖）。
 t4 两进程（各自独立 `Store.Open` 同一库/同一目录）× 相同内容 N 轮小负载：并发期间第三方取样观察器不得看到"规范名下哈希/长度不符"的文件；结束后 ref 唯一、两进程返回同一 id、无未处理唯一键冲突。
 t5 回滚路径：Typed apply 返回错误 → ref/文件/业务行全无残留、目录中除 temp 外无新增。
 t6 恢复/清扫矩阵：预置 旧+死 pid temp、新鲜+活 pid temp、`orphan-*`、committed ref 文件 → 清扫结果逐项断言（只删前者，后者零触碰）。
 t7 质量门：`go vet ./...`、定向 `go test -race`（src/store + 新增用例）、两个入口构建、issue 既有短 e2e（复用现有短用例，不新增长时流程）。
AUD-03 定向：
 t8 满队列：limit=k 灌满后提交 m 个不同内容 → 全部拒绝、object_ref 零新增、objects 目录零新增文件；排空一个后原请求可受理。
 t9 并发抢最后名额（2 进程 × n 次）→ 受理数 ≤ 剩余名额；失败方零 ref/零文件（证明最终容量检查在 tx 内且权威）。
 t10 Typed 回调注入错误 → 业务/请求状态/ref/file 零不受控残留；修复回调后重试只产生一次业务效果。
 t11 重启/清理回归：t8/t9 后重开不删除任何有效引用；合法同请求重试仍幂等（receipt/semantic hash 兼容断言并入）。
 t12 无 DDL 断言：`schema_migration` 版本/校验和不变。
NOT_RUN（本 issue 明确不重跑，原证据与 hash 保留不动）：两小时持续测试、soak/binary/data 全量、任何新时长门槛。

11. docs 最小登记：需要，且只需最小集
- 必要性：本修复改动的是"对象发布"这一跨模块契约（store 语义 + Core/Runner 调用规约 + 恢复行为），不登记会让后续模块继续按 objects.go 旧行为写代码。
- 最小集（在既有 2026-09-14 授权内，无需再次申请）：(a) 受影响设计文档补一段"对象发布协议 + 锁顺序 + 错误分类"（含 §2 六条不变式与 §9 的 fail-closed 条件）；(b) 在该文档与 README 按授权标注"实施发现的缺陷"并列出被改文档路径；(c) 结论只落在新编号 review 文件（建议 `review/issue1-AUD01-AUD03-resolution.md`），原报告文件与 hash 一律不动，复跑标记为"本轮不重跑，保留原始证据"。(a) 的具体归位（EngineeringArchitecture 还是 DataStructure）以文档现状为准——本轮未读 docs，不作臆断，由复核阶段核对后再定。

12. 开放确认点（留给 Codex / Master，非阻塞实现）
- 隔离物是否需要配额拒绝路径（我倾向"不需要"：增长源已被零残留受理消灭）。
- objectMu 删除 vs 限域保留（我倾向删除）。
- 清扫 GRACE/单次上限/ENOENT 重试次数的具体取值（卫生常量，写进代码注释与文档即可）。
- §11(a) 的文档归位。

最终裁决（自然收束）：foundation 提案方向正确、与 AUD-01/03 验收项可对齐，判定为"条件式通过"；落地前必须吸收 §9 的 R1–R6（其中 R2/R3/R4 属关键，R5 属判据必需），并保留 §8 的兼容性硬条件。AUD-01/03 的静态结论成立，无需动态复现即可进入实现；本轮不实施、不改生产、不写报告文件，未触碰任何密钥/真实模型/其他进程 ( _ _ )
