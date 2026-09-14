AUD-03.2 补充证据复核（限定范围，未重开生产审核）——限定 PASS，无 mustfix

一、实读与实跑（无全仓/长测/模型/keys/源码修改；生产树未动）
- 实读新增函数：src/store/issue1_storage_test.go:452–586（注释 + TestAdmissionTypedBusinessWriteRollbackFullBaseline 全函数）。
- 实读报告补充：reports/implementation/issue1-storage.md §"AUD-03.2 evidence supplement"（L50–54）。
- 实读清单：reports/implementation/issue1-storage-checklist.json（新增记录 "typed-business-rollback-race" PASS 2.321s；AUD-03.2 现列两测试 + limits；原 "intermediate-whole-repo" FAIL 历史记录保留未改写）。
- 亲跑：`cd src && go test -race ./store -run '^TestAdmissionTypedBusinessWriteRollbackFullBaseline$' -count=1 -v` → EXIT 0；`--- PASS: TestAdmissionTypedBusinessWriteRollbackFullBaseline (0.23s)`，包 ok 2.127s（作者记录 2.321s，同一量级）。
- 追加性绑定：与我在上一轮终审的受审快照（issue1_storage_test.go = 02514c20…）逐字节 diff，仅两处：`12a13 > "sort"`（import）与 `449a451,586 >`（追加新函数）——既有断言/辅助函数零改动，无弱化。

二、声称覆盖逐项核对（全部成立）
1. "真实 PutItemTx 写 item/change_event 并 tx 内确认"：callback 内 `PutItemTx(ctx, tx, item, 0)` 后，同 tx 内 `SELECT count(*) FROM item WHERE id=?` 与 `change_event WHERE entity_id=?` 均断言 ==1，并校验 turn.ID 非空，然后返回 injected error（L548–564）——确证"先业务写、后注入错误"。
2. "已有正常 input/object/item 基线"：先 `AcceptInput(original,100)`（落 object_ref + blob + 受理行）再 `PutItem(existing,0)`；基线在注入前冻结（L457–464, 542）。
3. "逐表全行内容含 sqlite_sequence"：snapshot() 动态取 `sqlite_master WHERE type='table'` 全表名，逐表 `SELECT *` 全列扫描、逐行 json.Marshal、行内排序后整体比对（L465–521）；embedded DDL `src/store/001_baseline.sql` 含 AUTOINCREMENT（grep 计数 1）→ 运行库存在 sqlite_sequence，被该动态枚举纳入，sequence 漂移会被比对捕获。
4. "blob 名字节比较"：blobs() 对 ObjectsDir `*.blob` 取全名→全字节 map，注入前后 JSON 等值比较（L522–541, 571–573）——拒绝了"仅断言 ref/文件计数"的旧短板。
5. "同 request 重试成功 + 重放 callback 恰一次"：同一 `in` 连续两次 AcceptTyped 均返回 nil，`calls` 计数必须 ==1（L574–585）——既证拒绝后原请求可受理，又证幂等重放不重复执行业务。
6. 附加证：被拒绝尝试本身确实发布过对象（acceptInputTx 内先归档），因此 snapshot/blob 等值同时证明"本次发布文件被回滚清理"（AUD-03.2 的"对象引用无不受控残留"）。

三、边界与残留限制（不升级门槛）
- 该测试 blob 比较只 glob `*.blob`，不覆盖 `.object-tmp-*` 残留；临时文件零残留由我上轮独立探针（严格目录枚举：只允许 {.publish.lock, 1 blob}）在准入路径上覆盖并 PASS——合并证据完整，非缺陷。
- 注入为 callback 返回错误（真实 tx 内业务写后回滚），非掉电/磁盘故障；不对 WAL/journal 侧做字节比较（未声称）。
- 报告 §Source snapshot 仍列旧测试 hash 02514c20…（按"原终审保持原 hash 快照"要求保留）；当前测试文件实际 hash 见下。仅提示（非 mustfix）：若希望报告自证新测试，可在补充段内注明新 hash，或沿用调用方外部绑定方式。

四、最终绑定 hash（sha256）
- 新增/当前测试文件：src/store/issue1_storage_test.go = e92d20b65bb4ae62d047bafe79d22508ec3d7439632968c2944ebd28a6392fef（原终审快照 02514c20572da1ae11c53c2c0ae4de80c56879350bc4a2322e47b848cf11b840 保留为历史）
- 报告：reports/implementation/issue1-storage.md = b0032d1a756317201aa4c61feb6ae3fcb9bcd5593ea2fa984235c935d8b2310a
- 清单：reports/implementation/issue1-storage-checklist.json = 981155f33f7740d01ef8f713daa8c4f7ba991948087e9692f6a8ee739293a3cd
- 生产源码未变：objects.go = 05bc43a9997916d9058636d01be7b0a89765ad8e54f1dbd3d120ae377f79fb39；core_repo.go = 8633054d086201a119c3e282e03722f2ff8a82fb9bb0d0041be58b90d5a2ac96（与存储终审一致）
- 证据文件：/tmp/issue1-typed.log = 9f44d0d8b0b2bd87dd3c2a06340aca445fd2765f78a84f0697251444d428d149；/tmp/issue1-typed.diff（追加性证明）

五、裁决
AUD-03.2 补充证据有效：新测试真实覆盖"Typed 回调内先写业务（item+change_event）后回滚、全表行内容与 blob 名字节相对基线零残留、同请求重试成功且业务回调恰执行一次"，实测 PASS；原存储终审 hash 快照不受影响，本次复核仅绑定上述新增测试与报告/清单 hash。范围内判 PASS，未发现 mustfix。stdout 供调用方归档（ _ _ ）
