# Issue #2 当前交付与验收入口

范围：[GitHub Issue #2](https://github.com/ExplodeCode6324/Secretary_Simplified/issues/2)。本轮更新单一权威会话与非阻塞 TUI；AUTH/TUI 本轮定向验收与实际 Ayanami DeepSeek 模块复核已通过；DOC 冻结静态检查、BUILD 与公开同步记录在本目录。

| 问题 | 修改与实现 | 当前规范 | 证据 |
|---|---|---|---|
| 客户端可创建独立主会话，摘要/问题不共同连续 | Store 唯一登记、002 追加迁移、公共准入；core 顺序消费者；context/memory 冻结前缀 | docs/SingleConversationTUI.md、Storage、DataFlow、MemoryPolicy、相关 DataStructure | AUTH-01—08；src/store/issue2_authority_test.go、src/tests/issue2_authority_test.go；结果见 ../issue2-auth.json |
| 命令式等待阻塞交互，问题和控制需要手填 ID | src/cli/tui 异步客户端、src/cmd/secretary 默认 TUI 与保留 CLI，结构化面板/确认/稳定请求恢复 | docs/Interfaces.md、EngineeringArchitecture、Operations、release/README.md、release/API.md | TUI-01—13；新增单元测试、issue2-tui-pty-*、issue2-tui-daemon-*；最终正式二进制证据见 ../issue2-tui-pty-release/report.json 与 ../issue2-tui-daemon-release/report.json |
| 当前说明仍混用初版设计与最新交付 | 当前正文/图/样例/DDL镜像与导出同步，原始设计归档 | 根 README、docs/README、Design2、ImplementationHandoff、Acceptance、References | DOC-01—05；docs-impact.json / docs-impact.md |
| 构建脚本隐式重跑旧测试 | scripts/build.sh 只构建；测试显式选择本轮名称，变更包 vet | docs/Acceptance.md、release/README.md | BUILD-01；build.json |

`docs-impact.json` 提供每个文件的分类、章节、处理理由、实现关联和哈希，Markdown 清单便于浏览。当前 v1.1 设计导出见 docs/deliverables；v1.0 zip、原报告、失败与构建哈希保留其历史范围，不作为本轮新二进制通过证据。

不支持真实来源同步、任意 shell、新模型业务能力、手机联网、语音或撤销已受理 turn。TUI 退出只断开前端；真实资料仍按 docs/RealDataTrial.md 在独立目录、明确 PERSONAL 披露授权后接入。当前合成/fake 模型测试不证明真实语义质量。

## 最终证据与边界

- AUTH：[作者报告](../issue2-auth.md)、[实际Ayanami终审](../../../review/issue2-auth-final.response.md)。
- TUI：[作者报告](../issue2-tui.md)、[实际Ayanami终审](../../../review/issue2-tui-final.response.md)、[正式release PTY](../issue2-tui-pty-release/report.json)、[正式release daemon/重启](../issue2-tui-daemon-release/report.json)。
- DOC：[逐文件清单](docs-impact.md)、[机器检查](docs-impact.json)、[文档初审](../../../review/issue2-docs-review.response.md)。每次导出核对来源与zip逐字节；原稿/001校验和不改。
- BUILD：[二进制、空库与源码指纹](build.json)；变更包 vet 在 AUTH/TUI 作者报告和独立审中记录。测试只按本轮名称选择。

macOS 终端的恢复实证为正常退出、Ctrl+C、SIGTERM；PENDIN 只掩内核瞬态位，其余 termios 字段严格比较。独立测试进程的 panic 注入补证见 [补充报告](../issue2-tui-panic-supplement.md)，它包装实际 Model/相同 Run 选项，不声称正式 release 含注入入口。其他信号未作本轮运行保证，Linux 未作运行验收。纯文本使用 `chat --plain`，不承诺裸 --plain。旧源码工具/历史回放若直接构造任意会话或未冻结的主轮，需要按新接口调整后方可再运行；本轮没有执行它们，也不把它们列为当前交付验证命令。

测试进程清理确认见 [issue2-tui-cleanup.json](../issue2-tui-cleanup.json)，本轮 owned Core/Runner/PTY/helper 无残留。


## 逐项关闭依据

AUTH-01—08、TUI-01—13、DOC-01—05、BUILD-01 共 27 项均为 PASS，逐项证据路径见 [acceptance.json](acceptance.json)。[DOC/BUILD最终独立复核](../../../review/issue2-docs-final.response.md)确认正文/导出/二进制/来源指纹一致。

复核措辞澄清：TUI 终审中“store/tests 为旧套件”指其不在 TUI selector 内，实际是**本轮新增 AUTH 测试**，已由 AUTH 终审独立验证；不能据该句认为本轮未验 AUTH。完整源文件哈希以作者机器清单及最终 build.json 为准，不使用复核正文里的缩写哈希代替。
