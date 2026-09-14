本次复核已完成，工具调用 12 轮用满（含 2 次被沙箱拦截的等价尝试），以下为最终结论，分三段。不再调用工具。

—— Part 1/3｜结论 ——

回答：
1) 范围：Issue #2 的 DOC-01—05 限定静态独立复核。仅运行本轮 scripts/issue2_docs_check.py（先 --help，确认只有 --export 一个参数），其余全部为只读核验（git、shasum、go mod tidy -diff、grep、只读 Python 分析）。未跑旧测试、旧 SQL 检查器（docs/checks/validate_docs.py 未执行）、未启动服务、未读 key/产品真实 model、未写生产/文档。
2) root "--export 已 PASS" 我独立复现了 PASS，但过程中出现关键漂移，必须分开陈述：
   - 13:08:30 我复跑：PASS（2237 文件 / 102 检查 / 0 错误）
   - 13:10:23 我复跑：FAIL（Export mirror docs/Interfaces.md）
   - 13:10:37 实现方重新 --export；13:10:38 我复跑：PASS（2239 文件 / 102 检查 / 0 错误）
   原因：复核期间仓库仍在活跃编辑（docs/Interfaces.md、release/API.md、src/go.mod、src/cli/tui/model.go 在 13:09—13:10 被继续修改），一度导致 v1.1 导出包与正文失配。
3) 判定：在 13:10:38 快照上，DOC-01—05 的静态条款成立（清单覆盖完整、镜像逐字节一致、历史/现行分离、未扩张验收门槛）；但这是"冻结瞬时结论"，不是本轮最终 PASS。交付包须在编辑全部停止后重跑一次 --export + checker，并复核 .sha256↔zip，方可引用为 DOC 证据。
4) 复核窗口内实际发生的修正：x/ansi 直依赖矛盾（发现时真实存在）已被实现侧修正，收口时点核对通过。未冻结的 AUTH/TUI API 细节不宣称最终 PASS；未要求旧测试验证新文档。

—— Part 2/3｜证据与发现 ——

独立核验（我本人执行）：
- 覆盖：以 git ls-files 重建候选文档集合与 reports/implementation/issue2/docs-impact.json 比对，缺失 0；记录含路径/分类/变更/requirements/章节/实现关联/sha256。快照分类（13:08:30）：现行 248 未改 + 69 改；历史 1919 未改 + 1 改；13:10:38 总数 2239。
- 镜像：92 个导出成员集合相等、逐字节一致（13:07:44 版 zip）；v1.1 .sha256 == zip 实际字节（3325139a…，180351 B，当时版本）。13:10:37 重建后的 zip 及对应 .sha256，留给冻结后再核。
- 基线：Design2 原稿 == BASE 字节（81c528c1…）；docs/schema.sql 与 src/store/001_baseline.sql == BASE（75668022…）；docs/migrations/002_authority.sql == src/store/002_authority.sql（122b72f6…）。即"原 Design2 归档、001 原 bytes 不改"成立。
- 依赖：13:08 时 go.mod 中 x/ansi 为 `// indirect` 且 src 无 import，而 docs/EngineeringArchitecture.md:78、docs/References.md:30（另 Decisions.md:48）声称"传递依赖提升为直接依赖、按 cell 宽度处理中文"→ 实质矛盾；13:10 已改为直接 require（go.mod:8）+ src/cli/tui/model.go:15 import，GOPROXY=off go mod tidy -diff 干净，矛盾消除。
- 一致性：design.md / Design2.md / EngineeringArchitecture 流程图、SingleConversationTUI 的 sequenceDiagram + stateDiagram-v2 与 Interfaces（会话 API、键位）、DataFlow（物理序 vs 认知前缀）、MemoryPolicy（共享状态/摘要水位）、Storage（002 双表、受理序号同事务）、Operations（migrate 流程）、Acceptance（本轮唯一范围表）相互一致，均表达：唯一权威会话、服务端排序、冻结前缀、客户端仅视图、任务局部上下文不可被选为 MASTER、legacy 只读、无 /new。
- 更正确认：auth 更正集的 C3 回退边界、C9 的 409 语义、F2 registry 自动新生条件、F3 backward/before_sequence 分页（13:09 刚写入 Interfaces，release/API 已含 direction）；TUI 更正集的 Ctrl+S/Alt+Enter、粘贴抑制 ^S 均可核对。
- 历史/现行分离：docs/README 的 Issue #2 历史边界段、checks/README 自标 "v1.0 历史"并指向本轮脚本、清单对 review/reports 标历史（DOC-04）→"不把历史原报告当现行规范"成立；现行正文中"新建/切换会话"只出现在拒绝/兼容/历史语境（逐处核对）。门槛未扩张：Acceptance §4 只跑本轮定向；scripts/build.sh 仅 build + SHA256SUMS（无 test/vet）。

需要处理的发现：
- B1（已修复，收口复核）x/ansi 声明-实现矛盾，见上；"仅用于中文 cell 宽度"的代码用途我未逐行核验。
- B2（过程中）交付漂移：13:07:44 封包后仍有正文/代码修改；且我 13:08:30 重生成的 docs-impact.json 与 root 交付版 sha 不同（.md 逐字节相同），已定位为 review/issue2-pty-pendin.response.md 等文件在 13:07:45—13:08:29 窗口被编辑（该文件 sha 记录在清单中，属解释性说明，非缺陷）。
- B3（小项）root README "当前验收状态"表 DOC 行仍指 docs/checks/latest-report.json（已自标历史）；建议收口时一行措辞注明适用构建或改指本轮脚本。
- B4（小项）review/issue2-docs-review.response.md 为空文件（0 字节）却被清单列为 current/modified；关闭前补齐或注明原因（我按指示未写任何文档）。
- B5（观察）清单理由为分类模板 + 章节/实现列，符合 DOC-01 字段要求；建议关闭说明指引审阅 docs-impact.md 表格。

—— Part 3/3｜思路、待核与边界 ——

思路：把 DOC 结论锚定在"冻结快照"上——先消除交付漂移（编辑停止→重导出→重检查），再收口 AUTH/TUI 未冻结面；清单与 zip 每次正文变更都必须同步重导出。

实施待核（不新造门槛）：
1) 冻结后重跑 `python3 scripts/issue2_docs_check.py --export`，并独立复核新 .sha256↔zip（我仅对 13:07:44 版核验过）。
2) x/ansi 直接依赖已在代码/模块一致，收口时确认文档用途描述与 wrap/truncate 实现一致并随最终快照复核。
3) AUTH/TUI 源码仍收口（API 细节最终再对齐）；在快照冻结前不要再动文档，否则清单/pass 需重跑。
4) B3、B4 两小项在关闭前处理。

复核边界与声明：写入仅限 checker 自身产物（docs-impact.json/.md 三次重生成：PASS→FAIL→PASS，现盘即 13:10:38 版）与 stdout 归档 reports/implementation/issue2/docs-check-stdout.txt（3 条运行摘要，另备份 /tmp/issue2review/）；未归档我自己的审阅脚本之外的任何内容。本轮复核会话按 Master 指定由 DeepSeek 承担，未另调 Secretary 模型；是否涉及网络模型调用不作保证性声明。未宣称：未冻结 API 的最终 PASS、旧测试对新文档的验证、PTY 实机交互结论（属执行侧）。DataStructure 采用清单理由核对 + 抽查，未逐文件逐行复核。
