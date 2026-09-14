Issue #3 RES01/02 独立终审完成（Ayanami，实测 + /tmp 独立 probe；未改生产、未跑旧 suite/backend/真实模型）。

一、结论：限定 PASS（0 mustfix）

- 亲跑命令全部通过：src 目录下 `go test -race ./cli/tui -run '^TestIssue2Residual' -count=1 -timeout=30s` → EXIT 0（`ok secretarysimplified/cli/tui 2.404s`）；加 `-v` 复核为 9 个父测试 / 27 个 table 子例 + 2 个独立父例 = 29 个案例全 PASS、0 FAIL；`go vet ./cli/tui` → EXIT 0、输出为空。
- 受影响 CLI 入口自行构建通过：`go build -o /tmp/secretary_cli_final ./cmd/secretary`（17,881,682 bytes）。未重建/未重启 daemon，未调用任何服务或真实模型，未读密钥。
- 两个状态机与冻结设计（issue3-design + correction，更正优先）逐条核对，未发现 mustfix；发现 4 项记录性偏离与 3 项限制，均在下文如实标注。

二、命令与关键哈希（完整）

- HEAD 基线：c84d92453a1d8338be728422e3164777ef822579（修复仍在工作树未提交，故以文件哈希为身份）
- client.go 098bb781363742fad17b9c9e99b78cca2e8a12dd1775613bb29e27b102c1970e
- model.go 8ed44710d1d3b7873463d37d809d11413f3617f82de10621b1ab16bbe65cc0d7
- panels.go bdc2a93cf1ebacdc44cea92d088e176a5409d8f92ccd5998c6494f37db94e208
- residual_test.go a03b98eaabcb8eb28bd4370409717ef5eeffcc5db913ffd89c46f50a0d0482cb
- 修前证据原样保留（未改写）：before.json ccaadb8e…、before.log 5bf56623…（与其 assets 记录一致）、before-items-cursor.json e10641f7…、.log f4a28d49…
- stdout 归档：测试 /tmp/i3_test.log 97f1c546…；-v 计数 /tmp/i3_test_v.log 435a8b8c…；vet /tmp/i3_vet.log e3b0c442…（空）；独立 probe /tmp/zz_issue3_probe_test.go 402c57d0… + 输出 /tmp/i3_probe_v.log 4c43a621…；CLI 二进制 9aa33bfe1aa8a8e4a52d245680b811a415e42a0cb6dd163b639e6098de025ef7
- DOC-R：reports/implementation/issue3/docs-impact.json 9a35b99a…（status=PASS、errors=[]、2,293 文件条目）；docs/deliverables/…v1.1.1-2026-09-14.zip ea09cf36…，`shasum -a 256 -c` → OK，.sha256 33413ecf…

三、两个状态机核对（实测证据）

1) 提交/观察（RES01）
- 代码：post() 只输出 {accepted,turnID,turn,err}（model.go:297-315）；Update 先并入 accepted/knownTurns，再按 done 早退 → 观察错误 → 提交错误三分支（model.go:457-493）。
- 已受理后任何 GET 错：R1-01（400/401/403/404）+ 我的 /tmp probe 用「白名单内 code 的 GET 错误」（401 UNAUTHENTICATED、403 QUESTION_AUTHORITY_DENIED、404 QUESTION_NOT_FOUND_IN_SESSION、400 BACKPRESSURE）全部 PASS：pending/envelopes/恢复文件/turn 身份保留、草稿不回填、POST 次数恒为 1、notice 含「已受理」且不含「被后端拒绝」。这是对「观察阶段永不进拒绝分支」的最坏组合覆盖（shipped 测试只用了非契约 code QUERY_DENIED）。
- 确定拒绝 vs unknown：白名单（client.go:96-106）经真实契约核验——POST /v1/inputs 的拒绝全部发生在持久化前（core/http.go:64-107；store/core_repo.go:16-46 为事务前校验，acceptInputTx:58-107 全部错误在 receipt/input_turn INSERT 之前 → WriteObjects 事务回滚）。R1-03 三对照 + IDEMPOTENCY_CONFLICT/REQUEST_REJECTED/BUDGET_EXHAUSTED/CONTEXT_REQUIRED_OVERFLOW unknown 对照 + unexpected 2xx/bad receipt → 均 PASS（保守：保留原 ID、不重发、不回草稿）。
- receipt→turn 重建 + 后置 403：R1-02 两模式 PASS——重建实例在 GET 403 下仍 accepted、身份保留；终态 COMMITTED 后迟到消息与重放 sync 不改 pending/notice、不重复显示、全程 POST=1。
- probe 补充：202 无合法 turn_id → 保持 submit-unknown（未记 accepted，符合更正「可记」的下限）、不重发、下一 tick 由 /v1/requests/{id} 恢复 accepted 身份（PASS）。

2) 面板页缓存（RES02）
- 范围/代次：applyPanel 仅接受 name 匹配且 generation==当前 的响应（panels.go:144-147），任何新请求都递增 generation → 延迟 refresh/append、切面板后迟到响应一律丢弃（R2-02 四面板 PASS，含重放同一响应）。
- 刷新范围：只替换锚点页（含选中 ID 的最后页；无选中则尾页），其余页不动；rows 按 ID 去重重建，选中按 ID 回绑（panels.go:112-143、185-198）。R2-01 四面板：两页→刷新后 100 行、选中不变、游标 page3、零 POST PASS。
- 原 ID/rev：confirm 在 prepareControl 固化，刷新改名后仍原 ID/revision（R2-03 PASS）；目标消失 → selected=-1 + 提示，不自动改选、不自动执行。
- items 409：只发所选对象 GET（每 tick ≤1，不再发死游标；R203ItemsStaleTicks 断言 itemCalls 恰 1 条）、404 → 清选择（R203ItemsStaleTicksAndMissingTarget PASS）；按 r 明确回第一页并按原 ID 回绑，未命中 → -1 + 提示（R203ItemsInvalidatedCursor PASS）；n 在失效态被拒。
- 通知：刷新路径零 POST/零 ack（测试断言）；DELIVERED 服务端语义已在 SingleConversationTUI.md:106 明示「不声称刷新完全没有数据库写入」，与 executor GET /v1/notifications 经 RuntimePage 写库一致（runtime_control.go:367-378）。
- 真实契约对齐：tasks/jobs/alarms/notifications 列表路由确实存在（executor/http.go:40-55 → RuntimePage 的 rowid 游标族）；items 走 core 快照游标（CONFLICT→409，transport.go:42）；扣款即 R1/R2 假调用器所用路径均对应真实端点，未发现「假调用器过宽掩盖端点错误」。
- probe 补充：跨页重复 ID（回填重叠）+ 锚点刷新 → 显示的是刷新后的新 revision（实现为「后出现的值优先、位置取首次」，与设计文本「先出现的保留」不同但行为等价或更优，且锚点是含该 ID 的最后一页，故刷新值必然可见；PASS，不构成 mustfix）。

四、记录性偏离与限制（不阻断 PASS，但应写入 runtime 报告）

1. 白名单仍有未直读链：UNAUTHENTICATED/PERMISSION_DENIED 在 /v1/inputs 上的具体发出点我未逐一读到；QUESTION_* 来自 CheckAnswerTarget（questions_repo.go:27/30/64，调用点 core_repo.go:44/76，均在持久化前）。白名单若缺少某真实 precommit code（如 DISCLOSURE_DENIED 类），只会落入保守 unknown（不回草稿、不重发），不会重现原 bug。
2. items 409 的前提链复核：CONFLICT→409 已由 transport.go:42 证实；public_api.go 的游标失效错误码我本轮未直读（沿用设计复核引用），故此链的一环为静态继承。
3. before.* 与当前测试文件不同修订：before.json 记录的 residual_test.go 为 a66bd7f9…（before-items-cursor 为 4dcc20ea…），当前为 a03b98ea…。FAIL→PASS 跨测试修订，我未重跑基线（遵指示不改写 before.*）。
4. 设计未落项（低危）：追加页「0 个新 ID → 停止翻页并提示」未实现（依赖服务端 next_cursor 终止）；r 重置时若首页 GET 失败，原 ID 回绑意图会被消费掉（下次成功加载会选首行）——均为静态观察，未实测。
5. 测试证据缺口（非缺陷）：R1-01 未断言 401/403 文案「请恢复认证」；R1-03(a) 未按设计 §五 断言 rejection 后 receipt=404（该事实目前只能由事务边界静态推出）。

五、未测/边界（如实）

未跑旧 TUI suite、旧 13/27 项、Context/Store/全仓/长期测试、后端 daemon 与真实模型；无网络、无密钥、无其他服务；未重建/重启 daemon（release/secretary 二进制在工作树中由 runtime 更新，非我重建）。另：reports/implementation/issue3/README.md 中 R1/R2 修后结果与 BUILD-R 仍标「待补/冻结后记录」，review/issue3-runtime-final.response.md 在我读取时为空（0 bytes）——即本轮 runtime 报告尚未冻结，本 stdout 可作为其归档输入。

思路：先按更正后的冻结契约建立「必须成立的断言集」，再用 shipped 9-parent/29-case 测试 → 我亲跑的 race/vet/build → 针对 shipped 未覆盖的最坏组合（白名单 code 的观察错误、202 无 turn_id 恢复、跨页重复+锚点刷新）写 /tmp 独立 probe（复制 src 副本内加测试，不动生产）三级证据交叉验证；白名单与 409/游标族则回读真实 HTTP 契约（core/http.go、store/core_repo.go、executor/http.go、transport.go）逐条比对，不能证实的链一律标注限制而非归化为 PASS。( _ _ )
