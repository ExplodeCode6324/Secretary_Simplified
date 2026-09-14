# Issue2 TUI 最小实现方案 — 设计裁决（Ayanami）

范围确认
- 本轮只对本 issue 的 AUTH-01—08 / TUI-01—13 / DOC-01—05 / BUILD-01 裁决；不重开 #1，不重跑旧 context/storage、全仓、月回放、长期测试；两小时测试 A25 不重跑、不设替代长时门槛。
- 不新增远程/真实付费模型依赖；race 只限定本轮选定并发用例；本轮为设计裁决，代码未实现（工作区中的 src/cli/、go.mod 脚手架不算实现）。
- 唯一会话 / 非多会话：以下 7 项逐条给出「同意」或「必要修正」。

1. 唯一会话非多会话 — 同意
- 与 issue §0.1/§0.2 一致：客户端只是输入/显示终端，无 `/new`、无选择器、无客户端自建分支；恢复仅指重连同一权威会话并按水位补齐。
- 实现补充（必要）：唯一性必须在服务端覆盖全部主对话写入口（inputs、actions/Typed、chat、单次 input、未来客户端），只卡 TUI 不满足 AUTH-06。现有 CLI 客户端仍硬编码默认 session（main.go:195-198，默认 "…0001"），须改为「省略由服务端绑定」或「提供时校验与权威一致，否则拒绝」，迁移与兼容说明写入 docs/Interfaces.md（AUTH-06/07、DOC-03）。
- request_id/turn_id/task_id 继续作幂等键与任务身份，不得被「唯一会话」吸收（§0.1）。
- 客户端草稿/滚动/已读位是本地视图，不上传、不覆盖服务端（AUTH-08 以服务端水位为真源）；客户端不得保存自己的主摘要。

2. 0600 元数据 / 实例隔离 — 同意，附 3 点必要修正
- 内容白名单：仅 {instance_id, request_id,（可选 turn_id, created_at）}；0700 目录 + 0600 文件；原子写（temp+rename）；同机多客户端共用目录时加 flock。无正文/答案/摘要/token。
- 隔离判定：读取时 instance_id 不匹配即忽略（当作无记录，不猜测）；同实例多客户端可加一个本地标签仅用于视图过滤，不参与服务端权限或认知。恢复真源=服务端：GET /v1/requests/{id} 已存在（core/http.go:60-63），返回 request_receipt.response_json（store/core_repo.go:200-203）；未完成 turn 查询由 foundation 新增（§2.3）。
- 重试语义：同 ID 重发必须原信封逐字一致——服务端按 payload_hash 去重（core_repo.go:28/57），同 ID 改正文会冲突；改过字就只能是新 request_id 的新提交，UI 必须明示「新提交」。取不到回执（未接受/404）→ 显示「未确认」交用户处理；零自动换 ID、零静默丢弃、零自动重投。

3. TUI 非 TTY / --json / plain 兼容 — 同意，明确降级规则
- 全屏 TUI 仅当 stdin 与 stdout 都为 TTY 且未指定 --json；任一不满足即走非交互路径：`chat` 保持现行逐行行为（bufio.Scanner，main.go:244-280 的兼容形态），裸 `secretary` 保持现行用法输出（main.go:81-84）；都不进入键盘等待、不接管屏幕、EOF 正常退出。
- 任何模式下 `--json` 纯机器输出、绝无 ANSI；现有 output() 的「状态行+JSON」保持脚本兼容（main.go:62-77）。TUI 是新增入口，不改旧命令输出格式（TUI-11）。
- `--plain` 强制逐行（即使 TTY），供 SSH/无全屏场景。
- 验收断言：非 TTY 运行 stdout 无 0x1B、无挂起、退出码合理。

4. request ambiguity 恢复 — 同意（细节并入第 2 条）
- 状态机：先落 0600 元数据 → 发送（同一 request_id）→ 回执记 turn_id 并清理 pending → 跟踪；丢响应只用原请求查询，不换 ID 自动重发（§4）。
- 未确认态：用户可「同 ID 手动重试（原信封）」或「放弃后以新 ID 重新提交」，二者均需显式选择；活进程保留原信封手动 retry 同 ID 允许（同 payload）。
- 重开/换端：以服务端未完成 turn 与历史为准；本地元数据缺失时不得凭文字相同合并两条提交（§4 末段），不得猜测。
- 显示按 request_id/turn_id 稳定 ID 去重（TUI-07）。

5. 安全控制序列 — 同意，修正为可执行清单
- 删除集合：C0（除 \n \t）、C1（0x80–0x9F）、DEL（0x7F）、ESC 序列（CSI/OSC/DCS/SOS/PM/APC 及两字节 ESC）、行缓冲边界处的截断残片；\r\n 归一为 \n；保留换行与 tab。
- 覆盖全部外部来源文本（历史、模型回复、问题、事项、通知、错误、状态栏），只在渲染层清洗，不改服务端权威原文。
- secret/token 永不进入输出/日志/界面标题，诊断只打 ID/状态码（TUI-12/13）。
- CJK 宽度：bubbles textarea 已用 go-runewidth + uniseg（textarea.go:21-22），在 pty 用例里验证中文/宽字符重绘（TUI-03）。

6. Ctrl+S / IXON / 通用发送键 — 带修正同意
- 结论：Ctrl+S 可作默认——raw mode 会清除 IXON，运行期 ^S 可达应用；bubbletea v1.3.10 具备 bracketed paste 检测（key_sequences.go:77-109，Key.Paste）与 RestoreTerminal（exec.go:118-129，tty_unix.go 使用 os/signal）。
- 必要修正：(a) 发送键可配置（默认 Ctrl+S）；(b) 机会性支持 Alt+Enter（ESC CR，收到即发送，收不到不报错）；(c) 信号与 panic 全走恢复路径（SIGINT/SIGTERM/SIGHUP + defer RestoreTerminal），pty 用例断言「退出后 stty -g 与启动前一致」纳入 TUI-12；(d) 文档附 stty -ixon 兜底说明；(e) 发送键丢失时草稿保留、不自动提交。
- 粘贴不提交：Enter=换行保证多行粘贴（含终端无 bracketed paste 的 macOS Terminal.app 场景）只插入文本不误提交；粘贴文本夹带 0x13 记为已知边界即可。
- Ctrl+P/N 输入历史仅内存，不落盘（与「不写 shell history/不新增正文日志」一致）。

7. 依赖范围 — 同意为「最小、成文的例外」，附流程修正
- 允许集合与锁定：仅新增两个直接依赖 bubbletea v1.3.10 + bubbles v0.21.0（v1 稳定 tags）；go.mod 已含 pin、go.sum 已含 h1 摘要（当前 worktree 为 // indirect，实现引入 import 后必须 `go mod tidy` 转为 direct，否则依赖记录失真）。传递依赖约 19 个模块（含 2 个伪版本 colorprofile / x/cellbuf），全部随 go.sum 锁定并列清单。
- 文档修订（必要）：docs/EngineeringArchitecture.md:51「首批第三方运行库限两项」须改为「两项 + 终端交互组件（bubbletea/bubbles，仅 UI 层）」并写明理由与边界；:53 的「查官方仓库、固定版本、保存 go.mod/go.sum 和依赖清单」照办；docs/References.md 依赖表同步；全部进入 DOC-01 逐文件清单。
- 边界：不引入前端运行时/框架平台；标准库职责（HTTP/JSON/日志/进程/UUID/配置）不变；TUI 不 import store、不执行 CLI/shell 子进程、不搬模型调用或权限判定（§5.1）。
- 依据：issue §3.1/§5.1 明确允许「成熟且兼容现有 Go/目标系统的终端组件，锁定依赖」；用标准库自研 raw-mode/宽度/粘贴/重绘等同自研 TUI 引擎，风险更高，例外合理。

其余核对（闭环项，不属 7 项但需一致）
- foundation 接口：现有 core 路由（core/http.go:17-135）无权威会话查询、分页原话历史、结构化问题、未完成 turn；需按 §2.3 最小新增（会话+水位、history(cursor/limit)、questions(open)+答案绑定、turns?state=open）；GET requestID 恢复已存在（core/http.go:60）。
- Runner 控制已齐备：executor/http.go:68-132 有 ack/cancel/pause/resume/trigger，控制体要求 request_id + expected_revision（:10-24）；CLI 已直连 runner.sock（main.go:160-162），TUI 同样直连即可「Core 离线不阻塞 Runner」；Frozen 时 resume 返回 EXECUTION_FROZEN 必须如实显示；409/STALE_FENCE 刷新后重确认（TUI-09/10）。
- 轮询有界：每次调用带 ctx 超时（3–5s）、并发上限、250ms→2s 退避加抖动、终态即停、增量用 events after_seq（core/http.go:120-125），禁止全库扫描（§5.1）；与 UI 事件循环分离。
- 单 UI goroutine：同意（tea 的 Cmd 在独立 goroutine 仅做有界 HTTP；model 只由 Update 写；无 DB/子进程/模型调用）。
- Tab=面板切换需在帮助/文档明示（输入区与 \t 不冲突），非阻断。

结论
- 7 项中 5 项直接同意（1—5），2 项带修正同意（6 发送键配置+恢复断言+边界记录；7 例外成文+tidy+清单）；无阻断项，总方案可执行。
- 实现完成前硬性 4 条：(i) 全入口服务端准入；(ii) 元数据最小化+实例隔离+零自动重投；(iii) 全量外部文本清洗与终端恢复断言入 pty 证据；(iv) go mod tidy + 依赖文档例外成文。
- 测试与证据仅限本轮编号用例；race 仅选定并发用例；报告把「本轮实际执行」与「历史证据未重跑」分开（§6.4）。

参考资料
- review/issue2-original.md（§0 不变量、§2.1 准入、§3.1 启动兼容、§4 断线与恢复、§5.1 实现边界、§6 验收）
- src/cmd/secretary/main.go:62-77, 155-162, 195-198, 203-233, 244-280
- src/core/http.go:17-135（requests:60、events:120）
- src/store/core_repo.go:28, 57, 92-93, 200-203, 291
- src/executor/http.go:10-24, 68-132
- docs/EngineeringArchitecture.md:49-53
- src/go.mod / src/go.sum（bubbletea v1.3.10、bubbles v0.21.0 pins 与 h1）
- 模块缓存佐证：bubbletea@v1.3.10 key_sequences.go:77-109 / exec.go:118-129；bubbles@v0.21.0 textarea.go:21-22

思路
先以 issue §0/§6 的硬约束为尺，逐项对照现有 CLI/Core/Runner 实现面（routes、receipt、控制体、依赖规则）确认「提案可行性」与「越界点」；凡提案与 §5.1/§4 语义有缝的地方（准入范围、同 ID 重试的 payload 一致性、发送键边界、依赖记录 tidy）给出最小可落地修正，不新增任何远程/付费/旧回归门槛。

—— 裁决完毕；如需，我可随即把第 7 条的文档替换措辞按 §5.2 清单逐条落到 EngineeringArchitecture/References，等实现提交后连同 pty 证据一起复核。( _ _ )
