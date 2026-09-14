复审完成（只读审计；未改任何生产/文档，未跑旧测试，未触真实模型/密钥）。

裁决：限定 PASS（TUI-01..13 + AUTH 共享面 + BUILD，锚定下列 hash 快照），mustfix = 无。5 条观察项、5 项未测已明示。

一、实跑命令与结果（本机 go1.25.6 darwin/arm64，src/ 内）
  1) 选择器发现（先 rg 后跑，未跑整包旧 tests）
     rg -l "issue2|Issue2" --glob '*.go' .  → 仅 src/cli/tui/model_test.go、src/cmd/secretary/issue2_cli_test.go 属本轮（store/tests 下 issue2_authority_test.go 为旧套件，按范围排除）
     go test -count=1 -race -run 'Issue2' ./cli/tui/ ./cmd/secretary/  → ok（13 个用例全 PASS；跑了两遍：一次在 model_test.go=eb5ccaa…，一次在 a18db40… 含新增 TestIssue2TUIInjectedShortTimeoutNeverEndsObservationOrChangesRequest）
  2) 构建/vet：go vet ./cli/tui/ ./cmd/secretary/ 干净；GOOS=linux GOARCH=amd64 go build+vet 通过（仅编译级验证）
  3) 二进制可复现性（关键交叉印证）
     go build -o /tmp/…/secretary ./cmd/secretary → f90a992e27d05a65146b9ff06b1b36b91b851be2aacbd3810a915d3029ed69c9，与 run5 报告 binary_sha256 逐位一致
     go build -trimpath -ldflags='-s -w' → secretary 8e1f42206a9438e4018afbeb60421a568f6aea6349342daa2e497a5d17a17e0f、secretaryd e3756a1a4219149dd0228e0252be95b05c9530b377ca61a0e28ee27e600bbb75，与 issue2-tui-pty-release、issue2-tui-daemon-release 记录的 cli/daemon hash 完全一致 → 两份收口证据可回溯到当前源码树
  4) PTY 原始数据 + 脚本 oracle（读了 scripts/issue2_tui_pty.py 全文：断言基于服务端侧 posts/controls/history_queries 与 termios 前后快照，非仅界面关键词）
     reports/implementation/issue2-tui-pty-release/pty-{1..4}.txt 全文 grep：无任何原始 \x1b]52 OSC（合成通知文本里注入的 OSC52 未上屏）；pty-4.txt 含 NONINTERACTIVE_INPUT_REQUIRED、87B 无 ESC
  5) 语言语义疑点亲验（判断 return m, m.showPanel(...) 是否丢改动）：/tmp/issue2-semantics 最小复现 → panel=questions, n=1，改动生效；与 PTY 实跑（F2→Enter 绑定真实 question id）互证

二、逐项核对（全部基于亲读+实跑）
  多行中文/粘贴/resize：model.go 粘贴路径先于发送键，^S 被过滤入草稿；window resize 不丢草稿（单测+PTY TUI-03）
  异步草稿/延迟控制：pending 不阻塞编辑/滚动/面板（单测 + PTY drop/delay 场景）
  JSON-TTY / 非 TTY 无键等待：main.go:285-290 显式报错不读键盘；--json 无 ANSI；管道 EOF 退出（PTY check 10/11 + TestIssue2CLINonTTYDefaultAndHelpDoNotReadInput）
  question/request 恢复：仅 GET /v1/requests/{id} 复查原 id，不重投；pending 文件重连后清零（PTY check 4 + 单测断言零 POST）
  task/job CAS：expected_revision 来自行快照并冻结于确认框，409/冲突如实显示、零自动重试；Core 离线时 Runner 控制仍可达（PTY TUI-09，core_offline=True 下 cancel RV=2 成功）
  通知非自动 ack：浏览只 GET；ack 需 y 确认（controls 仅 2 条：cancel + ack）
  终端恢复/清洗/metadata：Ctrl+C 与 SIGTERM 后 termios 全字段严格相等（仅掩 PENDIN，符 pty-pendin 裁决）；SafeText 覆盖 C0/C1/DEL/CSI/OSC/DCS，应用于历史/摘要/行/notice/title/status；恢复文件仅 {instance_id, request_id}、0600、temp+rename+目录 fsync
  x/ansi：仅 src/cli/tui/model.go 的 Wrap/Truncate（全仓 rg 无其他引用）；go.mod 三 TUI 依赖均 direct
  AUTH 契约面（无错位可上报）：core 路由 /v1/conversation、/v1/conversation/history、/v1/turns/{id}、/v1/requests/{id}、/v1/inputs 齐备；store/authority_repo.go:22-28、353-358 的 json 标签与 TUI Authority/History 逐字段一致；store/authority_repo.go:170 强制 AUTHORITY_SESSION_MISMATCH。AUTH-05（真实 Core 重启后 authority/历史/共享态保持）已出现在 issue2-tui-daemon-release（6 checks, PASS, hash 与本机重建一致）——run1 报告仅 5 checks，属脚本早期版本，已被 -release 取代，建议最终报告以 -release 为收口引用，run1 留作历史。

三、观察项（非阻断，建议记入最终报告/文档）
  O1 main.go:86 裸 `secretary --plain` 在 TTY 下仍进全屏 TUI（仅 chat/tui 分支查 plain）；help 文本未承诺该形式，属措辞/行为缝，建议补一行判断或文档化
  O2 bubbletea v1.3.10 默认信号处理仅 SIGINT/SIGTERM（tea.go:276-300）+ SIGWINCH/SIGCONT，无 SIGHUP；设计 6(c) 提过 SIGHUP → 记为已知限制（TUI-12 证据域为 Ctrl+C/SIGTERM）
  O3 scripts/issue2_tui_{pty,daemon_bridge}.py 为 git untracked，建议最终报告记录脚本 hash（pty=afc7d83…、bridge=84d5bb7…）
  O4 `return m, m.showPanel(...)/m.send()` 依赖 gc 求值行为（规范对非调用操作数复制时点未严格保证），本机实测且 PTY 互证正确；换编译器需留意（不改代码）
  O5 审计期间源码在动：model_test.go 曾 eb5ccaa… → a18db40…（新增超时观察用例，重跑 PASS）；我的结论锚定第四节快照

四、未测项（明示）
  Linux 运行期 PTY（terminal_linux.go 仅交叉编译+vet）；SIGHUP/SIGQUIT/panic 恢复路径；LIVE_MODEL/REAL_USE（PTY 后端为本地合成鉴权 HTTP，daemon 运行为真实本地 daemon + 合成提醒）；旧套件/全仓 race/月回放/长测（按范围未跑）；reports/implementation/issue2-tui.md 落盘时我最后确认仍不存在（issue2/ 目录仅有 docs-check、publication-prebuild-scan 等）

五、关键 hash（最终活快照，约 13:15 本地）
  src/cli/tui/model.go 96d74969242b1962c52463ca20ea6fcbd3f7cbaafa3010e37d3e1212f5aa3dae
  src/cli/tui/client.go e97b2f5bdc650be8cafd9ffb47bbbdacb41bd4e6aece04496b646adcd6d72d3d
  src/cli/tui/terminal_darwin.go 05b3d3c3d914f94d7d69fe1d4eb91010d73aa31c5cd6dfe125326147fb008f57
  src/cli/tui/terminal_linux.go 8cd94fc843fb5cfb2f2d53315e7d2e2e851d96030c8f2c09f43ea00e739f2cb0
  src/cli/tui/model_test.go a18db40884282a7ef7d671c7de22d83bc60d98d9277f23366600949ea4473f10
  src/cmd/secretary/main.go 3bdd96d78de8957ed6804ed2af36e250cf11cfb22a1ae8163c86576e6148525a
  src/cmd/secretary/issue2_cli_test.go 73a856510b311d4cc543798d0b012723318a9fcdb5f9a52366ddb963d17ef0c8
  src/go.mod 7a2e1ee66c0f5d617a1fee76a636d5c4ad9aa9f4c153511408ff05e3e4a1782d／src/go.sum f9a91f713aa9799fd3fa8eedc523327095ba04775ddb0095adc6ba043a661eeb
  git 状态：src/cli/tui/ 与 issue2_cli_test.go 为 untracked，main.go/go.mod/go.sum 为 M（未提交；本轮不动）

追最终快照说明：这是我在工具预算内能追到的最新点（运行方正在收口——pty-release/daemon-release 于 13:14 落盘）。若 issue2-tui.md 落盘后源码再有改动，请以该文件的源码 hash 为准并知会我复核差异；期间只要 model.go client.go terminal_*.go main.go 任一 hash 变化，本 PASS 即需重新锚定（重跑第 1 项选择器 race 即可，约 3 秒）。

思路：先用 rg 限定新用例选择器避免旧套件扩大面，再以「二进制可复现性」把 PTY/daemon 证据链钉到当前源码（这是本轮最强的交叉验证），同时亲读脚本 oracle 与原始 transcript 排除假 PASS；对语言层疑点（返回值求值顺序）用 /tmp 最小复现独立判定，不臆断；契约面只查 TUI 实际调用的路由与字段标签，发现一致即不上报错位，发现的证据版本差（run1 vs -release）如实记录。( _ _ )
