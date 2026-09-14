# 本地部署

当前二进制目标为 macOS arm64。Issue #3 在 Issue #2 唯一权威会话与 TUI 上修复受理状态误报及分页刷新，新增定向测试与 vet 通过；仅更新 CLI，daemon 和数据库保持原字节。本单最终场景数、复核与交付状态见 [验收索引](../reports/implementation/issue3/README.md)，完整构建哈希见 [BUILD-R](../reports/implementation/issue3/build.json)。真实试用使用独立目录和明确模型授权范围，仅接入受控 PERSONAL 文字；真实来源文件同步尚未开放。完整计划与工作记录见根目录 README。

历史 Issue #1 修复构建为 CLI `84e24025…` / daemon `1d62faad…`；其完整哈希在原报告中，当前构建以 SHA256SUMS 为准。全仓 race/vet、定向故障回归与短时发布包测试已通过；原 final2 两小时测试保留旧构建标识，不将其结果写成此修复版的持续运行证明。Master 明确本轮修复无需重跑该时长测试。

公开的 `db/secretary.sqlite` 是不含授权、凭据或用户数据的空库检查材料；正常部署仍执行 init 创建自己的运行库。

## 构建与初始化

从项目根执行 `./scripts/build.sh`。此脚本只构建并生成 `release/secretary`、`release/secretaryd` 及 SHA256SUMS。

首次使用独立目录：

```sh
./release/secretary init --data-dir /tmp/secretary-demo
./release/secretaryd core --config /tmp/secretary-demo/config.json
# 在另一个终端启动：
./release/secretaryd runner --config /tmp/secretary-demo/config.json
```

Mac Unix socket 有长度限制；选择较短的 data-dir。初始化生成 `state/secretary.sqlite`、objects、run 中彼此独立的客户端/内部凭据和 fixture 配置。fixture 是确定性离线测试模式，不进行自然语言理解；使用类型化命令测试业务或显式配置 live 模型。

仓库自带的本地 `release/config.local.json` 与 secrets/state/run 不公开。Master 提供的 OpenCode Go key 已仅放在本机 secrets。测试期间模型仅允许 SYNTHETIC 数据；尚未授权真实资料外发。

真实接入本轮仅支持 PERSONAL 文字，需 Master 明确模型服务商和外发范围后使用新独立 data_dir；`source_configs` 和 `provider_policy.source_ids` 保持为空。真实来源文件同步尚未开放，不能把 fixture_path 改成真实文件路径。仅本地/未知分类资料放在另一独立目录，不共享 SQLite 或 objects；同库混放仍可能保守拒绝。完整边界与验证见 [RealDataTrial.md](../docs/RealDataTrial.md)。

```sh
./release/secretary doctor --config /tmp/secretary-demo/config.json
./release/secretary items create --title '合成事项' --domain project --config /tmp/secretary-demo/config.json
./release/secretary items list --config /tmp/secretary-demo/config.json
./release/secretary input --text '合成测试文本' --config /tmp/secretary-demo/config.json
./release/secretary chat --config /tmp/secretary-demo/config.json
```

自然语言输入受理是异步状态，不代表动作已登记或完成。使用返回的 turn_id 查询 `secretary turns <turn-id>`；`secretary requests <request-id>` 查询持久回执。Issue #3 修正受理后查询失败：界面保留原请求，提示“已受理，暂时无法读取结果”；认证错误需恢复认证，不能把原文作为新键再次提交。POST 响应丢失也只查原 request-id；仅提交阶段确定未受理才在空草稿中恢复原文。

面板定时刷新保留已加载页及选中 ID。事项游标失效会标明过期并暂禁 n，继续有界查询当前对象；按 r 明确从第一页重载。目标消失/离开范围会提示并取消选择，已打开的控制确认仍用原目标/版本，不自动 ack。局部修复仅更新 CLI，daemon、Schema、数据库和最小恢复元数据不变。

复杂类型化操作使用 `--file <ActionProposal.json>`：`items update`、`jobs create`、`world propose` / `world correct`。格式严格遵循 docs/contracts.schema.json 的 ActionProposal；示例见 examples/reminder.json 与 examples/artifact.json，可用 `secretary actions --file <path>` 提交。CLI 默认显示状态说明和详情；`--json` 输出稳定 JSON，脚本应显式使用。

## 诊断、备份与恢复

```sh
./release/secretary backup --output /tmp/secretary-backup --config /tmp/secretary-demo/config.json
./release/secretary verify --backup /tmp/secretary-backup --config /tmp/secretary-demo/config.json
./release/secretary restore --backup /tmp/secretary-backup --target /tmp/secretary-restored --config /tmp/secretary-demo/config.json
```

恢复目标必须为空。恢复校验 SQLite 快照、manifest 与不可变对象哈希；生成新的本地客户端和内部凭据，并保持配置与 execution_frozen 标记双重冻结。恢复后的服务可启动诊断，但不会重发历史效果。尚未完成对账和显式重新配置前，不移除冻结标记。

两个进程在前台运行，Ctrl-C 退出。本项目不安装自启、不调整睡眠或音频设置、不监听 TCP。静音 alarm 只验证本地控制流程，实际叫醒需要 Master 后续配合实测。

## 显式切换模型服务商

Master 已授权：再次遇到 OpenCode HTTP 429 时，切换测试到 DeepSeek 官方 API，Ayanami 同步切换官方 provider。当前正常调用不自动改服务商，不启用 OpenCode 余额付费。

配置模板见 `config.deepseek.example.json`：profile 为 `deepseek`，endpoint 为 `https://api.deepseek.com/responses`，官方模型 ID 为 `deepseek-flash`。据 [DeepSeek 2026-09-10 官方公告](https://www.deepseek.com/en/news/deepseek-v4-1-flash/)，该 ID 对应 V4.1 Flash；OpenCode 使用的 ID 是 `deepseek-v4.1-flash`，两者不可混填。[官方 Responses 文档](https://api-docs.deepseek.com/api/create-response/)说明此接口无服务端会话状态，本地权威数据和会话继续保存在原 SQLite 中。

切换时保留 data_dir、授权和业务库，只显式替换 model_profile 与对应 secret_ref，重启自己管理的 Core；Runner 不依赖模型切换。已提交请求不重放；失败请求保留原记录，通过新 request_id 明确重新提交未执行的意图。未知外部结果仍须对账，不能因换模型而重做。公开模板不含密钥，实际官方配置和密钥仅存本机。

换模型机械验证使用真实 Core/SQLite 与模拟 HTTP：原模型提交事项、429 不登记新动作、官方协议接续更新同一 Item ID、重复处理不重复效果。真实官方 API 的切换结果单独报告，不能由这项机械测试冒充。

## 内置离线检查

```sh
./release/secretary verify --suite smoke --report /tmp/secretary-smoke-report --config /tmp/secretary-demo/config.json --json
```

`smoke`（同义名 `persistence`）在新的临时库中核验 SQLite 完整性、不可变对象、备份及恢复冻结，并生成 report.json / report.md。它不代替完整 A01—A25 验收。这些属于初版历史验证；Issue #2 只运行 reports/implementation/issue2 中列出的新增定向测试与变更包 vet，不执行旧全仓或长时套件。

公共接口的类型化请求格式与分页示例见 [API.md](API.md)。通知列表使用 `secretary notifications`，确认已读使用 `secretary notifications ack <id>`。列表可传 `--limit`、`--cursor` 及对应过滤参数。

## 回答已登记的问题

回复中的 `[question_id=… session_id=…]` 块表示程序已保存的待答问题。使用当前权威会话中的明确 ID 回答（TUI 可在 F2 中选择）：

```sh
./release/secretary input --answer-to <question-id> --text '明确回答' --request-id <本轮request-id> --config <config.json> --json
```

先查询 turn 的最终提交结果。一次明确回答只更新该问题的解决状态，不自动完成关联事项；模型失败时问题仍未解决。不同客户端共享同一权威问题；其他旧会话保留只读历史，不恢复为可写分支，不猜测指代或把摘要当作授权。

## 输入分类（D12）

普通 input/chat/typed 写入默认 PERSONAL。现有合成测试配置不会向模型披露 PERSONAL；测试时须显式附 `--data-class SYNTHETIC`，例如：

```sh
./release/secretary input --text '这是一条合成测试输入' --data-class SYNTHETIC --config release/config.local.json --json
```

不要把真实内容标成 SYNTHETIC 来绕过限制；真实测试的 ProviderPolicy 与来源范围由 Master 接入时明确。旧派生记录缺少程序分类时返回 OUTPUT_CLASS_UNKNOWN，程序保留记录且不会自动清库或猜测迁移。完整规则见 docs/Operations.md 与 docs/Interfaces.md。

## Issue #2：默认 TUI 与旧库升级

已有配置和服务运行时，执行 `./release/secretary --config <config.json>`（或 chat/tui）打开终端界面。Enter 换行，Ctrl+S / Alt+Enter 发送，F1 帮助，F2 问题，F3 委托，F4 计划，F5 事项，F6 通知；Ctrl+C 仅退出前端。PgUp/PgDn 滚动，Ctrl+P/N 输入历史。`chat --plain`、命令式 input 与 --json 继续用于脚本/管道；非 TTY 不进入全屏。

配置缺失或 Core 离线时不会自动初始化或启动服务。新库 init 建立唯一实例/会话；旧库需先停止对应 Core/Runner，再执行 `./release/secretary migrate --authority-session <旧MASTER会话ID> --config <config.json>`。其他旧会话只读保留，不能自动合并；多个终端不创建新会话。

当前架构见 [SingleConversationTUI](../docs/SingleConversationTUI.md)，数据边界见 [RealDataTrial](../docs/RealDataTrial.md)。既有两小时及 Issue #1 报告属于各自旧构建，本次 TUI 构建不继承其运行时长结论。

当前 release 哈希以 SHA256SUMS 为准。历史 Issue #2 实际测试绑定 CLI `8e1f4220…` / daemon `e3756a1a…`，证据见 `reports/implementation/issue2/build.json` 和 `issue2-tui-*-release/report.json`；Issue #3 的新 CLI 证据单列于 `reports/implementation/issue3/`，未变化的 daemon 保持原字节。样例库仍为 001+002，只有空权威登记，不含业务资料或凭据。
