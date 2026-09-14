最终短裁决（仅基于已读事实 + 本轮更正；未新增工具/扫描，无网络调用发生）

A. 更正与撤回（明确替代初稿内容）
1. 表述更正：初稿"全程无网络模型调用"不成立。准确表述为：未另调被测 Secretary 模型（fixture/diagnostic 路径未触发）；reviewer 自身经 opencode-go DeepSeek 运行，属 Hermes 侧模型调用。接受该更正并以此为准。
2. R-a 撤回并替换：database/sql 在 ctx 取消后自行 rollback 释放 SQLite writer → "defer 在闭包内删自有新建路径 = 仍持 writer"这一前提是错的，R-a 作废。替换为：flock 全程持有（flock 不随 ctx 取消释放）→ 取消/commit 错时仍持 flock，用 bounded uncancelled persistence ctx 查 object_ref：存在 → 保留+校验；确定无 row → 只删本次自有新建路径（temp + 本次 link 的 final）；查询不确定 → 保留完整孤儿。补一条安全理由：删除决策只能发生在持 flock 内，释放后一律不删（否则可能删掉对端刚提交 ref 所指文件）。
3. R2 降级（非 must-fix）：我不坚持"字节写入与 temp fsync 必须在 flock/DB 锁外"。把临时写移入 flock/tx 后，SQLite 5s busy 只会变成可诊断错误，不构成正确性缺陷；锁内大 fsync 属性能建议（吞吐/等待），不是安全硬条件。区分确认：性能建议 ≠ 安全硬条件，初稿把它写成 must-fix 属过度收紧。
4. R3 消解：临时文件只在 flock 内创建，恢复清扫也在同一 flock 内 → 不存在"清扫 vs 活 temp"竞态，年龄/pid 门与 link-ENOENT 限次重试均不需要；若 flock 内仍见 link ENOENT，视为不变量破坏，直接报错。
5. R1 降级：不要求每次启动强制探测文件系统。hardlink 失败 fail-closed（报错、绝不 copy+rename 覆盖）本身即安全边界；探测降为可选诊断。
6. 撤回 purge CLI 与新配额；隔离物由现有文件级运维枚举即可。
7. 撤回"发布事务用 persist ctx"对业务的适用：业务/model 一律不得脱离取消运行；唯一例外是取消/commit-unknown 时持 flock 的 bounded uncancelled object_ref 决策查询。

B. 同意的最小方案（确认，含两点实现级补钉）
同意：固定 objects/.publish.lock flock 跨进程锁；锁 inode 永不删除/改名；ctx 可取消等待（LOCK_EX|LOCK_NB 轮询 + ctx 检查，不用不可取消的阻塞 flock 等待）；所有 PutObject/Accept/恢复统一 flock→SQLite IMMEDIATE，持锁至 commit 结果确认或清理完成；`Store.WriteObjects(ctx, func(tx, *ObjectWriter) error)`，ObjectWriter.Put 仅在已持 flock+tx 时可用，不导出裸 PutObjectTx；受理顺序 tx 幂等/answer/queue → 完整 temp 写 + sync + close → hardlink no-replace → dirsync → ref 与业务同 tx；temp 写在 flock/tx 内（已有大小上限）；恢复同 flock 只清已知 temp，不碰 committed；坏无 ref final 隔离、committed 损坏保留拒绝；objectMu 移除；tx 内读取用 tx；semantic hash 兼容保留。
实现级补钉（属上述方案的必要细节，非新协议）：
- 锁序全局固定 flock → BEGIN IMMEDIATE，禁止任何反向获取；每临界区独立 open 的 fd（结束 close 释放），不要跨 goroutine 共享长存 fd 后显式 LOCK_UN——否则同进程另一 goroutine 的释放会解开仍在进行的临界区（flock 按 open file description 生效，跨 fd/跨进程都冲突，这正是机制可用性所在）。
- 混合版本边界：忽略 flock 的旧二进制不在互斥模型内；其遗留坏文件由"坏无 ref final 隔离"覆盖，该覆盖是设计的一部分而非例外。

C. 替代关系（一目了然的对照）
- R-a → flock 持有 + bounded uncancelled 决策查询（存在保留 / 确定无 row 才删自有新建 / 不确定留孤儿）；禁止锁外删除。
- R2 → 性能建议（锁内 I/O 延长 flock 持有、SQLite 等待可诊断），非 must-fix。
- R3 → 由"temp 只在 flock 内创建、恢复也在 flock 内"整体消解；无年龄/pid 门、无 ENOENT 重试协议。
- R1 → fail-closed（link 失败即错，禁 copy/rename）已足够；启动探测可选。
- 初稿 t6/t7 相应调整：清扫矩阵改为"flock 内仅清已知 temp 名、committed 与 orphan 零触碰"，去掉年龄/pid 用例；其余测试清单与"不新增时长门槛"不变。

D. 剩余真正 must-fix（仅列，附具体安全理由）
1. flock 锁序固定 + inode 永不删除 + 每临界区独立 fd — 否则互斥失效或自锁，直接回到并发覆盖/误删。
2. 不导出裸 PutObjectTx — 唯一暴露面必须强制"已持锁+tx"，否则旁路即跨进程竞争回归。
3. tx 内读取一律走 tx 句柄 — 第二连接会阻塞或读旧快照，导致误采纳或误隔离。
4. 取消/commit-unknown 的清理判定（持 flock + bounded uncancelled 查询；存在保留并校验、确定无 row 才删自有新建、不确定保留孤儿）— 误删等于丢已提交引用/证据。
5. committed 与 orphan 错误分类拆分、committed 永不移动/删除/覆盖，隔离名不匹配 `.blob` 规范形态 — 保证据、保 ReadObject/去重语义。
6. archiveInput 迁入 tx 且删除旧锁外调用（禁止双份）— 残留旧调用等于 AUD-03 未修。
7. object_ref 与业务行同 tx；runtime briefing 亦经 WriteObjects 将 ObjectRef 与 FinishWorkTx 原子绑定 — 消除半提交态；注意 src/core/work.go 已被 AUD-02 触及（工作树有改动），两处改动需在同一文件上协调顺序避免互相覆盖。
8. semantic hash 兼容（存储 envelope 仍追加归档 ref，哈希计算保持剥离自动归档 ref）— 否则既有 request_receipt 会被误判 IDEMPOTENCY_CONFLICT。

E. 非 must-fix（登记即可，不阻塞）
锁内 I/O 的长短（性能/可诊断性）、启动探测（可选）、非普通文件（symlink 等）拒绝（加固）、隔离物人工枚举方式、objectMu 删除方式（若因风险暂留，只允许为最外层短临界区且绝不跨 flock/DB 获取——按本轮决策仍推荐直接删除）。

F. 裁决
同意该最小方案；以 B 的方案 + D 的 must-fix 为本 issue 的可实施协议，初稿 R2/R3/R-a 由 C 的替代项作废。初稿首稿内容不改，本更正作为本轮最终裁决叠加其上（ _ _ ）
