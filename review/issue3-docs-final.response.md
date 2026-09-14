════════════════════════════════════════
【Issue #3 收口复核 — 最终静态结果】2026-09-14 14:1x HKT
════════════════════════════════════════

一、最终清单复跑（亲跑，未加 --export）
- 结果：{"status": "PASS", "files": 2316, "checks": 111, "errors": []}，rc=0。
- 与冻结产物逐字节一致：md diff 为空、记录数一致（2316=2316）→ 2316/111/PASS 为可复现状态，非单次产物。
- 最终镜像：98/98 成员逐字节一致（含初审 4 个 FAIL 文件），zip.sha256 5cfbc3ff… 校验 OK；导出成员集与 v1.1 相同（仍 98，无增删）。
- 计数口径：引述 "2309" 为其消息写作时点；其后落盘的新增记录（publication-scan.json、docs-final request/response、runtime-delta.response、evidence/manifest.json 等共 +7 行）已被清单收编（只收编清单、不改导出正文）→ 引用请以 2316 或推送前最后一次运行为准。
- 备注：复跑仅重写 docs-impact.*（字节相同）；mtime 因复跑刷新（两次含时间戳复原的复合命令被沙箱拦截，未做 touch 复原，以 sha256 为准）。脚本无 --export 时不写其它文件；zip/旧包/CLI 全文未动。

二、初审 mirror FAIL 保留 + 新 freeze 修复（不抹去）
- 初审冻结 ea09cf36… 在 14:03 实跑为 FAIL（4 mirror：README.md、docs/Design2.md、docs/RealDataTrial.md、docs/References.md——导出后 14:02:39 又被编辑）。完整记录留存：
  /tmp/issue3-ayanami-20260914-140327/（checker stdout、frozen/run 双版清单、22 份 diff、zip 前后哈希、双 zip 成员表）。
- 重导出为 5cfbc3ff…（已含该批编辑），本轮复跑 0 错误、98/98 通过、与 .sha256 文件一致 → 新 freeze 已修复，两段记录并存、可追溯。

三、四处小 diff 补读（逐行）＋ 三语义确认
- docs/SingleConversationTUI.md：状态图重写——Drafting→Submitting→{Drafting(提交阶段确定未受理) / Unknown(POST 未确认) / Accepted(受理+已知 turn_id)}；Unknown 只查原 request 不自动重发；Accepted 遇 GET 4xx/5xx/超时"保留受理事实"、不退回；终态不倒退、不生成新键重发。文字含"已受理，暂时无法读取结果"、401/403 恢复认证不绕过、仅提交阶段确定未受理才恢复原文且不覆盖新草稿；恢复文件仅 instance_id/request_id。
- docs/Interfaces.md / docs/Operations.md / release/API.md：提交/观察分离语义镜像；items 过期处理（标明过期、禁用后续翻页 / 暂停 n、仅单对象 GET、r 自第一页重载、不静默跳页、不回扫）；不隐式 ack；"HTTP/DTO 与恢复结构不变、无新增 API/迁移"。
- 三语义：①"位置取首次出现、值取后出现页的版本" ✓（逐字）；②"追加页没有新 ID 时停止继续翻页并提示" ✓（"后 tick 不重开"为语义覆盖、无逐字表述——不构成 mustfix，可选补写）；③"读取失败保留待回绑 ID，直到成功响应才能判定原对象是否在已加载范围" ✓（逐字）。
- ROOT 顶层：L3"当前交付范围为 Issue #3"/ L7"历史交付：Issue #2…" ✓；TUI 内集合已改题 "#2 历史验收集合" ✓。

四、BUILD-R 静态核对
- build.json 绑定 after-final 源：model.go 8933b156…、panels d3306373…、client 098bb7…；new_test 5488cc51…；production binary 59d93dd7… == 实际 release/secretary == release/SHA256SUMS 首行。
- after-final.json：race 选择器 exit 0、vet exit 0、11 parent/35 cases；production_files 与 build.json 源逐一致、test_sha256==5488cc51…；scope 明示原 after.* 保留。
- build-initial.json：旧构建证据保留（7fcdf368…、旧 test a03b98ea…）。
- 不变性：daemon e3756a1a…、db 540797f3…、go.mod/go.sum、schema 双副本 + mirror——复跑 111/111 全过（含 10 项 Unchanged 与 Schema mirror）；旧包 v1.0=5cb0ba15… / v1.1=15f6236b… 与初审逐位一致（原 bytes 不变）。
- RES delta 6 新 cases 归另一 DeepSeek 亲跑，未重复；本单测试证据为 root 侧记录，未代跑。

五、结论（精准限定）
DOC-R 静态 PASS；BUILD-R 静态 PASS；无 mustfix。两条轻微观察：a) 可选补"后 tick 不重开"逐字句；b) 计数引用用 2316（最后运行为准）。边界：未跑 --export、未读 keys/产品模型、未跑旧 checker/tests、未改源码/正文/旧包；新增 review/provenance 仅收编清单、不改导出规范正文、不循环复审——本报告即终。

参考资料
- 有效设计：review/issue3-design-correction.response.md
- 脚本与清单：scripts/issue3_docs_check.py；reports/implementation/issue3/docs-impact.json（2316）
- 正文：docs/SingleConversationTUI.md、docs/Interfaces.md、docs/Operations.md、release/API.md、README.md（顶层）、release/README.md、release/SHA256SUMS
- BUILD 证据：reports/implementation/issue3/build.json、after-final.json、build-initial.json、evidence-index.json
- 归档：/tmp/rei-out.log（复跑 stdout 归档）、/tmp/rei-err.log、/tmp/rei-f.json、/tmp/rei-f.md（冻结版备份）；/tmp/issue3-ayanami-20260914-140327/（初审全套）

思路：先补读四处 diff 对齐有效设计，再以"备份→复跑→逐字节对比"的零信任流程验证冻结清单可复现性，并用哈希链（zip/CLI/daemon/旧包/构建绑定）分别锚定 DOC 与 BUILD 两侧；最后把 2309 与 2316 的口径差归因到"收编"机制，显式保留初审 FAIL 记录、不回头抹平。( _ _ )
