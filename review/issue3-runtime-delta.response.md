Issue #3 终审 delta 复核完成（Ayanami；仅两处 delta，旧结果/原 after.* 未重跑未改写，未改源码/文档，未读 keys，未调模型/后端）。

一、结论：delta 限定 PASS（0 mustfix；1 项非阻断观察）

两处 delta 的实现与根因诉求一致，新增 6 用例先真实 FAIL、修后 PASS 已被我独立复现；11 parent / 35 case 精准 Residual race 与 vet 我亲跑通过；DOC-R 三处澄清到位；我方明确裁决：同意「位置首次，值后页优先」作为 R2 既有目标的实施细化（见第四节）。

二、三生产文件相对我上次 hash 的小 diff（精确等于两处 delta）

- client.go：零 diff（hash 与上次一致，未动）。
- model.go：唯一新增字段 `panelNoProgress bool`（model.go:43）。
- panels.go：
  1) openPanel：`more && m.panelNoProgress` → 提示「追加页没有新 ID；已停止继续翻页，重新打开面板可重置」并 return nil（不再发追加请求）；showPanel 复位 `panelNoProgress=false`。
  2) applyPanel 追加分支：以 m.rows 现 ID 集合判定 `added`，为 false → 置 panelNoProgress + 同文案；rebuildPanel 在 panelNoProgress 时把 `panelCursor` 置 nil（后 tick 刷新也无法重新启用；`n` 因 cursor 为 nil 被拒）。
  3) `m.panelSelection = ""` 从 applyPanel 开头移到**成功应用页之后的末尾**：任何 err/409-invalid 分支提前 return 时保留待回绑意图，成功才消费。
- residual_test.go：仅追加 `R202AppendWithoutNewIDsStopsNavigation`（tasks/jobs/items/notifications 4 例）与 `R203ResetFailureRetainsSelectionIntent`（selected=25/75 两例），假服务用 page3 复用 page1 且游标非空来构造 0-new；reset 例以 503 覆盖首个响应再重试。共 +6 case。

三、亲跑命令与结果（只新增两测试；整体仅 ^TestIssue2Residual）

- 定位：`grep -n` 得 residual_test.go:445 / :473。
- 只跑新两测试：`go test -race ./cli/tui -run '^TestIssue2ResidualR202AppendWithoutNewIDsStopsNavigation$|^TestIssue2ResidualR203ResetFailureRetainsSelectionIntent$' -count=1 -timeout=30s -v` → EXIT 0，6/6 PASS。
- 整体：`go test -race ./cli/tui -run '^TestIssue2Residual' -count=1 -timeout=60s -v` → EXIT 0，parents=11、leaves=33、fails=0 → 11 parent / 35 case（33 子例 + 2 个无子例父例），与 after-final.json 自述一致。
- `go vet ./cli/tui` → EXIT 0（空输出）。
- 独立 FAIL 复现（我自己做，不依赖 runtime 日志）：把当前测试文件盖到上次复核的旧代码快照（panels.go 仍为 bdc2a93c…、model.go 8ed44710…，即我上次记录的修订）→ 同 6 例失败且报错逐字与 before-review-delta.log 相同（4×「zero-progress append was not stopped 100」、2×「failed reset consumed target intent」），行号 454/485 与当前测试修订一致 → 该 before/after 对为**同一测试修订**的真实 FAIL→PASS 链（优于首轮 before.* 跨测试修订的那一对）。

四、DOC-R 核对与明确裁决

- docs/SingleConversationTUI.md:102 已写明「按 ID 去重，位置取首次出现、值取后出现页的版本」（即位置首次/值后页优先）、「追加页没有新 ID 时停止继续翻页并提示」；:104 已写明「按 r 明确从第一页重新加载，按原选中 ID 回绑；读取失败保留待回绑 ID，直到成功响应才能判定原对象是否在已加载范围……不悄然改选另一个对象」。三处澄清与实现逐句一致。
- 裁决：**同意**，作为 R2-01/R2-03 既有验收目标的实施细化（非新增验收项、不扩范围）。理由：锚点页取自「含选中 ID 的最后一页」，故刷新后的副本必然是该 ID 的最后一次出现——此时「值后页优先」才能让刷新值可见，初设计的 first-payload 规则在该位置会把更早页的旧副本留在屏上，反而破坏「刷新后版本变化可见」；同时 page 加载顺序即时间顺序、且 generation 门只允许最新响应落盘，故「后页值」= 更新值是可判定的，不含「服务端最新 revision」的全库承诺。「位置首次」保证行序/选中位置稳定（无重排跳动）。实现与文档已由此配对自洽，我方接受。

五、证据保全与自证一致性（逐字节核对）

- review/issue3-evidence/ 与我最初 /tmp 产物 hash 完全相同（原样存，未被改写）：ayanami-probe.go.txt 402c57d0…、probe.log 4c43a621…、residual-race.log 97f1c546…、residual-verbose.log 435a8b8c…、vet.log e3b0c442…（空）；manifest.json 声明值逐条与实文件相符。
- 报告自证：before-review-delta.log cc960407…、after-final.json f8ec3b06…、after-final.log c7b51573…、implementation.md 38a88180…、after.log 7f17b8bf…、after.json 594f1491…、vet.log/vet-final.log e3b0c442…，全部与 evidence-index.json 及我独立计算值一致；after-final.json 记录的三生产文件 hash 与我的实测值一致。旧 before.*/after.* 未被改写。

六、完整新 hash（本轮 delta 身份）

- src/cli/tui/client.go 098bb781363742fad17b9c9e99b78cca2e8a12dd1775613bb29e27b102c1970e（不变）
- src/cli/tui/model.go 8933b1563631e8372002defb560466af4f08daeba5d5774dfb9d9e869d997537（原 8ed44710…）
- src/cli/tui/panels.go d3306373baaedec899d8848e82cb9eb0370f8e5bb8f191476824c1f81a01fa68（原 bdc2a93c…）
- src/cli/tui/residual_test.go 5488cc51449a2aa51c7ed4bcbf8d96cab0008ddec8f836d34cd07c8765de3c01（原 a03b98ea…）
- before-review-delta.log cc960407082a0fb7375f9531146e407111213fc720bac666db185b9f5844e67e；after-final.json f8ec3b060d9c0d59a73e3de4cd1e507f71809a792d60c9dc4554884dd5e2a1d7；after-final.log c7b5157348130ff7855238f8d66ab5656af94a7774698023ea83bb4983504fe7；implementation.md 38a8818038c6dd5569227c847a71b01c0db6ce2f906620f7633b3ce5d358c964；evidence-index.json dd18811a284ef625ffb7f4e1434222276b22abdee2887888b83b57138f5b5ae7；reports/implementation/issue3/README.md bde9a6fcef2f397d0e25dfd88af6adc4d8e1ae4b42adca52ca01c4050f7428f2
- 我的本轮 stdout 归档：/tmp/i3_delta2tests.log 4cc0ffb2d8b4b9c5476ef60d6efdde0c5d47abf30a56345ea449428aad214730；/tmp/i3_test_v2.log 7287d210f1e9a7d192d8a5f5df9f53809a4a94f59a22bc39c8571fefc232df9d；/tmp/i3_replay_oldcode.log cb183ad81603369fb748783ccde9beead820cc88f3d47eafdbeb3658af13cf52；/tmp/i3_vet2.log e3b0c442…（空）

七、非阻断观察与未测限制（如实）

1. 静态观察（未实测追加用例）：连续两次按 r 且首次失败时，第二次 r 用 `selectedID()`（此时 -1 → 空）覆盖 `panelSelection`，会丢掉首次失败保留的待回绑意图；可选 1 行硬化：resetPanel 中 `if selected == "" { selected = m.panelSelection }`。当前不构成 mustfix——确认框始终显示目标 ID/revision 且需显式 [y]，不会静默对错误对象执行；新用例覆盖的「首次失败→成功重试」路径本身 PASS。
2. items 409（invalid 分支）也保留意图不消费，属保守且幂等重放，无副作用，记录不阻断。
3. 本轮遵从指示**未做终构建**、未跑旧 TUI suite/旧 13/27/Context/Store/后端/长测/真实模型；三生产文件 diff 仅客户端字段与面板逻辑（client.go 零 diff），可确认无 backend/schema/恢复文件结构变动；release CLI 与 root README 的最终冻结不在本 delta 结论内。

思路：以「只跑新增选择器」的最小实测 + 用我上次 hash 的旧快照做同修订反向复现（FAIL 报文与 runtime before 日志逐字一致）+ 报告/证据逐字节 hash 自证 + 文档三处澄清与实现逐句比对，构成 delta 的三级证据；去重整条则按锚点选择规则与 generation 门推导其确定性后作裁决，不把「同意」当测试通过、也不把运行结果归化为超出范围的新验收。( _ _ )
