# 本地部署

当前二进制目标为 macOS arm64。代码、审计修复与合成验收已完成，可按下文独立目录和明确模型授权边界接入受控 PERSONAL 文字测试；真实来源文件同步尚未开放。完整证据与工作记录见根目录 README。

Issue #1 修复构建为 CLI `84e24025…` / daemon `1d62faad…`，完整校验见 SHA256SUMS。全仓 race/vet、定向故障回归与短时发布包测试已通过；原 final2 两小时测试保留旧构建标识，不将其结果写成此修复版的持续运行证明。Master 明确本轮修复无需重跑该时长测试。

公开的 `db/secretary.sqlite` 是不含授权、凭据或用户数据的空库检查材料；正常部署仍执行 init 创建自己的运行库。

## 构建与初始化

从项目根执行 `./scripts/build.sh`。此脚本运行 Go 测试和 vet，再生成 `release/secretary`、`release/secretaryd` 及 SHA256SUMS。

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

自然语言输入受理是异步状态，不代表动作已登记或完成。使用返回的 turn_id 查询 `secretary turns <turn-id>`；`secretary requests <request-id>` 查询持久回执。重试应复用 request-id；已有键的不同语义载荷会冲突。

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

`smoke`（同义名 `persistence`）在新的临时库中核验 SQLite 完整性、不可变对象、备份及恢复冻结，并生成 report.json / report.md。它不代替完整 A01—A25 验收。完整源码测试命令为 `cd src && go test -race ./... && go vet ./...`。

公共接口的类型化请求格式与分页示例见 [API.md](API.md)。通知列表使用 `secretary notifications`，确认已读使用 `secretary notifications ack <id>`。列表可传 `--limit`、`--cursor` 及对应过滤参数。

## 回答已登记的问题

回复中的 `[question_id=… session_id=…]` 块表示程序已保存的待答问题。使用原 session 和明确 ID 回答：

```sh
./release/secretary input --session <session-id> --answer-to <question-id> --text '明确回答' --request-id <本轮request-id> --config <config.json> --json
```

先查询 turn 的最终提交结果。一次明确回答只更新该问题的解决状态，不自动完成关联事项；模型失败时问题仍未解决。跨会话取回后须切回原 session，不猜测指代或把摘要当作授权。

## 输入分类（D12）

普通 input/chat/typed 写入默认 PERSONAL。现有合成测试配置不会向模型披露 PERSONAL；测试时须显式附 `--data-class SYNTHETIC`，例如：

```sh
./release/secretary input --text '这是一条合成测试输入' --data-class SYNTHETIC --config release/config.local.json --json
```

不要把真实内容标成 SYNTHETIC 来绕过限制；真实测试的 ProviderPolicy 与来源范围由 Master 接入时明确。旧派生记录缺少程序分类时返回 OUTPUT_CLASS_UNKNOWN，程序保留记录且不会自动清库或猜测迁移。完整规则见 docs/Operations.md 与 docs/Interfaces.md。
