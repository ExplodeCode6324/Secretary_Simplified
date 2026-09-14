Ayanami 独立最终增量复核（Issue #1 AUD-01/AUD-03 冻结实施）——限定范围 PASS，无 mustfix

一、复核范围与实际执行（全部本地合成，无网络/模型/密钥/真实资料；未改生产树；探针仅 /tmp 副本）
1. 代码实读：src/store/objects.go（368 行全读）、core_repo.go（含 AcceptInput/AcceptTyped/acceptInputTx/archiveInputTx 必要段）、src/store/issue1_storage_test.go（449 行全读）、store.go 锁序追踪 + 全仓 WriteObjects/PutObject/.Write( 调用点清单（限定追踪，未全仓测试）。
2. 亲跑定向 race（非作者转述）：
   - `cd src && go test -race ./store -run '^TestObject|^TestAdmission' -count=1 -v` → EXIT 0，store 4.570s；含四退出点子测试逐项 PASS（temp_created/before_publish/after_publish/before_commit）。
   - `cd src && go test -race ./store ./ingest ./diagnostics -count=1` → EXIT 0（store 6.387s / ingest 1.900s / diagnostics 7.806s）；`go vet ./store ./ingest ./diagnostics` → EXIT 0。
3. 独立探针（/tmp/ayanami-issue1-probe-r2，rsync 源码，SHA 与仓库一致；新增 probe 测试，未进入生产树）：
   - `go test -race ./store -run '^TestIndependentIssue1' -count=1 -v` → EXIT 0（2.611s）。
   - 逐阶段磁盘遗留分类（真实 os/exec + os.Exit(41)）：temp_created → 恰 1 个不完整 `.object-tmp-*`、0 final、0 ref；before_publish → 恰 1 个完整 temp、0 final、0 ref；after_publish / before_commit → 0 temp、恰 1 个完整 final、0 ref（崩溃前未提交）；四阶段重试后恰 1 ref + 字节/hash 正确；RecoverObjectStaging(100) 对两 temp 阶段恰回收 1、对另两阶段 0；`.publish.lock` 为普通文件且 inode 跨崩溃+重开不变（os.SameFile）。
   - 准入零对象（独立于作者断言）：12 次满队列拒绝 + 12 次 Typed 回调失败后，objects 目录严格等于 {.publish.lock, 1 个 .blob}，无 temp/无隔离物；ref 数 1 且 relative_path 与文件名一致；存储 envelope 仍恰含 1 个派生 text/plain 归档 ref（SHA256==原文 hash）；receipt.payload_hash == inputSemanticHash（剥离派生 ref）→ 语义哈希兼容经独立验证；同请求重放同 turn ID、ref 数不变；同 request_id 改 payload → IDEMPOTENCY_CONFLICT。
4. 文档实读 + SHA 对齐：Storage.md / DataFlow.md / DataStructure/ObjectRef.md / Operations.md 四份均与报告快照 SHA 完全一致，文本与"更正版裁决"一致（flock 固定 inode 永不删除、flock→Store mutex→BEGIN IMMEDIATE、hardlink fail-closed 禁 copy/rename、committed 永不移动、隔离名 `.orphan-<uuid>`、取消后 5s 无取消查询清理、RecoverObjectStaging 为 1–1000 有界 Go 接口且明示"现有CLI没有凭空新增 purge 命令"、不重跑 2h、不加时长门槛）。
5. 无 DDL：git status 未见任何 schema/DDL 文件改动；Store.Write（store.go）未被修改（不在改动清单）。

二、8 项 must-fix（更正版 D 节）逐条核验结论
1. flock 锁序/inode/独立 fd：objects.go `lockObjects` O_CREATE|O_RDWR|O_NOFOLLOW 每临界区独立 fd，LOCK_EX|LOCK_NB + 10ms 轮询可 ctx 取消；顺序 flock→s.mu→BeginTx 已实现并注明；未发现任何反向路径（WriteObjects 调用点仅 core_repo:46/346、work.go:158、objects 内部；无"持 s.mu 再取 flock"）✓
2. 无裸 PutObjectTx：ObjectWriter 字段全部未导出，Put 有 active 守卫，外部仅经 WriteObjects/PutObject ✓
3. tx 内读取走 tx 句柄：`getObjectRef(ctx, w.tx, …)`；readObjectRef 为纯文件路径 ✓
4. 取消/commit-unknown 清理边界：持 flock → bounded（5s，WithoutCancel）查询 object_ref → 存在保留并校验、ErrNoRows 才删本次 w.created、其他错误保留完整孤儿；仅删自有新建路径，释放锁在清理之后 ✓
5. committed 与 orphan 分类：committed 走 metadata/hash 拒绝且不移动不覆盖；坏无 ref final 移 `.orphan-<uuid>`（不匹配 .blob 后缀）后重建；完整孤儿先重 file.Sync+目录 Sync 再采纳 ✓
6. archiveInputTx 进入 tx：单一调用点（core_repo.go:82，位于幂等→answer→queue count 之后）✓ 旧锁外调用已消失
7. ref 与业务同 tx：acceptInputTx 内 InputTurn/receipt/conversation 同 tx；runtime briefing 经同样的 WriteObjects 绑定（AUD-02 侧，运行时代理复核）✓
8. semantic hash 兼容：信封追加派生 ref、哈希剥离——由探针独立复验 ✓

三、关键 SHA（sha256）
- src/store/objects.go 05bc43a9997916d9058636d01be7b0a89765ad8e54f1dbd3d120ae377f79fb39（= 报告快照）
- src/store/core_repo.go 8633054d086201a119c3e282e03722f2ff8a82fb9bb0d0041be58b90d5a2ac96（=）
- src/store/issue1_storage_test.go 02514c20572da1ae11c53c2c0ae4de80c56879350bc4a2322e47b848cf11b840（=）
- src/store/store.go 30db6a254bc96d5dd9fefef19573e07fca044d0819f7dd2cbb442f9b84a25189（Store.Write 未改）
- reports/implementation/issue1-storage.md e9b122ad1a9020253a3dac22bf35284fead55003ddd9952f0268477342d0918c；docs 四份 SHA 与报告一致（e6f52d1c… / 61ee6dbc… / 507be153… / 84804796…）；HEAD ac281ac2e3504ef215f9b69ee1b314e2db16adea
- 证据日志：/tmp/issue1-directed.log 1c43e1f8…、/tmp/issue1-probe2.log 6e9076e1…；探针源 fca052d2…（仅 /tmp）

四、保留的未测限制（不升级为门槛）
1. runtime 侧 AUD-02 两项测试（QueuedBackgroundCancellation/NStaleWorker）未由我复跑；我的 PASS 仅覆盖 storage/ingest/diagnostics + issue1 定向 + 独立探针。
2. 最后槽竞争用"同进程两 Store"验证；跨进程互斥由两进程发布测试单独覆盖，未做跨进程准入次竞争组合。
3. commit-unknown 用"真实提交后注入 ack 丢失"模拟；清理决策中"查询不确定→保留"分支为代码实读，未被故障注入。
4. docs/checks/validate_docs.py 未由我重跑（作者声明 PASS）；文档一致性由我直读 + SHA 对齐替代验证。
5. 探针首轮 4 子项失败经定位为我的探针假设错误（`.publish.lock` 为首次发布时惰性创建，Init 后尚不存在），修正后全 PASS——非实现缺陷，仅记录（文档"inode 永不删除/改名"与惰性创建一致）。

五、裁决
AUD-01/AUD-03 冻结实施在本复核范围内判定 PASS：协议（flock→SQLite、hardlink no-replace fail-closed、ref 与业务同 tx、取消/commit-unknown 边界、坏孤儿隔离 vs committed 损坏保留、语义哈希兼容、无 DDL）与代码、文档、定向与独立证据一致；未发现 mustfix。两小时测试未重跑、旧证据 hash 未触碰、无新时长门槛；本轮无密钥/真实资料/产品模型调用（reviewer 自身经 opencode-go DeepSeek 运行，未另调被测 Secretary 模型）。stdout 供调用方归档（ _ _ ）
