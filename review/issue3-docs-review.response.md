【Issue #3 DOC-R / BUILD-R 独立静态复核 — 有限范围结论（2026-09-14 14:0x HKT）】

**结论：清单机制与边界设计成立；但当前快照不可宣布完成，且冻结时的 PASS 2293/111 在现时点不可复现（原因明确、可解释）。不作为终审 PASS，需 root 收口后重跑。**

一、独立实跑（亲跑，先 --help）
  - `python3 scripts/issue3_docs_check.py --help` → 正常，仅一个开关 `--export`，描述 "DOC-R only: ... no runtime suites"。未加 --export 实施。
  - 冻结产物（13:59:16 生成）核对：status=PASS、记录 2293、checks 111、0 项失败——与 Master 提供的 "PASS 2293 files 111 checks" 一致（在 13:59 快照口径内自洽）。
  - 我 14:03:27 重跑（未 --export）结果：`{"status": "FAIL", "files": 2300, "checks": 111, "errors": ["Export mirror README.md", "Export mirror docs/Design2.md", "Export mirror docs/RealDataTrial.md", "Export mirror docs/References.md"]}`，rc=1。

二、FAIL 根因（实测锁定，非 checker 缺陷）
  - 四个失败文件的 mtime 均为 14:02:39，晚于 v1.1.1 导出时间 13:59:16：导出后正文又被编辑，导出未同步 → mirror 校验如实报错。cheker 行为正确（发现失配即 FAIL，不归一化）。
  - 我 14:02:07 的首轮只读比对中 Design2/RealDataTrial/References 尚未改动；复核期间受跟踪改动 21→26、porcelain 37→42——快照仍在移动（root 正文初稿仍在编辑中）。
  - 记录增量精确可复现：2293 → 2300 = +7（v1.1.1.zip/.sha256、issue3/after.json、review/issue3-docs-review.*、issue3-runtime-final.*）+ 3 项重分类（Design2/RealDataTrial/References 由 unaffected 变 modified）。无凭空/丢失记录。

三、清单与边界核对（已实证部分）
  - 机制合理：清单由 git 事实派生（diff BASE ∪ untracked）、逐文件 sha256、current/historical 分层——不会漏项，非人工列举。
  - "API/metadata 结构无变化" 成立：10 项 Unchanged（contracts.schema.json 双副本、schema.sql、migration、go.mod/go.sum、daemon/db 二进制逐字节==BASE）+ Schema mirror 全过；CHANGED 列表不含任何 DataStructure/schema 文件。
  - 发布副本合理：v1.1.1 导出成员集 98 项，与 v1.1 完全一致（无增删）；release/README.md、release/API.md 在导出内；release/SHA256SUMS 未改动（git 事实）。
  - 旧证据原字节保留：v1.0/v1.1/v1.1.1 三 zip 哈希在我跑前跑后完全一致（未跑 --export）；v1.1.1 哈希 ea09cf36…e6a88 与其 .sha256 记录相符；旧报告全部 historical/unaffected 且不在 changed。
  - 历史不当新规范：issue2/README.md 仅追加一行指向 issue3（声明原 acceptance/PASS-FAIL/哈希原字节保留）；docs/Review.md、checks 原报告、archive、旧 zip 均判历史；README/docs-README/checks-README/release-README 均已明文化该边界。
  - 语义对标有效设计（correction response C1–C8）：已读部分（Design2、release/README、RealDataTrial、References、各索引）与 C1/C2（unknown 不重发、不换新键）、C3（观察失败不退回、404 保留 envelope 语义）、C7（保留页缓存与选中 ID、游标过期暂禁 n、有界查询、r 从第一页重载）、C8（不自动 ack）一致；版本 1.1.1、验收范围仅 R1/R2/DOC-R/BUILD-R 表述正确。
  - 未逐行读（本次静态结论的边界，diff 已存档待读）：docs/SingleConversationTUI.md（57 diff 行，含状态图）、docs/Interfaces.md、docs/Operations.md、release/API.md（各约 11 行）四处小改动的逐字确认；viewed 摘要层面与之无冲突。

四、冻结前必要动作（root；我未代做）
  1) 停止正文编辑后重跑 `--export` 生成与定稿一致的 zip/新校验（zip 哈希必变，.sha256 与 deliverables/README 记录需同步）——或若坚持保留现 v1.1.1 哈希，则 14:02:39 批次四处编辑不得进入本版；
  2) 随后再跑一次不带 --export 的检查，以取得自洽的最终清单（记录数 ≥2300）——那才是可冻结/可提交的 "最后一对"；当前 v1.1.1（ea09cf36…）不是最终交付物；
  3) API 键位/错误码/构建边界与 BUILD-R 定向证据仍留待最后阶段对齐（本轮不涉）。

五、边界声明
  - 未修改源码/文档；被检查器覆写的 `docs-impact.json/md` 已按原字节还原并复原时间戳（cmp 双确认）；未跑 --export、未跑旧 checker/tests/Go 套件；未读 key/真实模型；#1/#2 旧 27/13 项未触碰。
  - stdout 与全部运行证据归档：`/tmp/issue3-ayanami-20260914-140327/`（checker.stdout/stderr.log、frozen 与 run 双版 docs-impact、zips before/after 哈希、22 个逐文件 diff、双 zip 成员表、git 快照）。若需持久位置请指定路径，我下轮可复制。

参考资料
  - 有效设计：`review/issue3-design-correction.response.md`；本单记录：`reports/implementation/issue3/README.md`、`docs-impact.json/md`（冻结版 + 本轮重跑版存归档）
  - 脚本：`scripts/issue3_docs_check.py`；相关正文：`docs/SingleConversationTUI.md`、`docs/Interfaces.md`、`docs/Operations.md`、`release/API.md`、`release/README.md`、`README.md`、`docs/deliverables/README.md`

思路：以"独立复现 + 时点证据链"审查——先 --help 与冻结产物核对，再在备份后原样重跑并做前后哈希/diff 对比（跑前备份、跑后还原，保证零遗留改动），用 mtime + git 双轨锁定"导出后编辑"的因果，最后以增量算术（+7/3 重分类）验证清单机制无漏项；边界内能签的签，不能签的（未冻结快照、四处未逐行核的小 diff）如实留在结论外。( _ _ )
