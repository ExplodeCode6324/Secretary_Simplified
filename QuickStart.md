# QuickStart：首次真实文字测试与完整命令

从仓库根目录执行下列命令。当前 release 是 **macOS arm64** 二进制，Core 和 Runner 各占一个终端，第三个终端打开 TUI。已有发布包可直接使用，无需先构建。

首次真实测试按下方第 1—5 步执行；后面的命令参考保留离线演示、脚本、备份恢复、模型切换和旧库升级。本文只给操作步骤，不会替你启动服务、修改现有配置或发送真实资料。

## 1. 更新代码并初始化专用目录

先在终端进入你克隆的 Secretary_Simplified 文件夹，然后执行：

```sh
git pull --ff-only
(cd release && shasum -a 256 -c SHA256SUMS)
export SEC_DATA="$HOME/.secretary-real"
./release/secretary init --data-dir "$SEC_DATA"
```

`init` **只执行一次**；已有 config.json 时会拒绝重复初始化。再次启动直接跳到第 3 步。目录用短路径，避免 macOS Unix socket 路径长度限制；不要用 `/tmp` 存放需要保留的真实资料。这里不复用仓库内合成验收库，也不复制公开样例数据库。

## 2. 选择模型并明确允许 PERSONAL 文字

初始化默认是离线 fixture，只允许 SYNTHETIC；它不能理解自然语言。下面将**新建专用目录**配置为 OpenCode Go 的 `gpt-5.6-luna`，并允许 PERSONAL 文字交给该服务商。仅当你同意把本次输入及该专用库内相关上下文发送给 OpenCode Go 时执行。将只愿留在本机的资料放在另一个独立目录，勿输入本次试用库。

命令会隐藏读取 API key，不把 key 写进 shell 命令历史。提示出现时粘贴 OpenCode Go key 本身，不能粘贴包含说明文字的 Markdown 文件内容。

```sh
python3 - <<'PY'
import getpass
import json
import os
from pathlib import Path

data = Path.home() / '.secretary-real'
path = data / 'config.json'
config = json.loads(path.read_text())
key = getpass.getpass('OpenCode Go API key（隐藏输入）: ').strip()
if not key or any(c.isspace() for c in key):
    raise SystemExit('API key 为空或含空白，未修改配置')
os.umask(0o077)
secrets = data / 'secrets'
secrets.mkdir(mode=0o700, exist_ok=True)
secret = secrets / 'opencode-go.key'
secret.write_text(key + '\n')
secret.chmod(0o600)
config['model_profile'] = {
    'profile': 'opencode-go',
    'endpoint': 'https://opencode.ai/zen/go/v1/responses',
    'model': 'gpt-5.6-luna',
    'secret_ref': 'secrets/opencode-go.key',
    'timeout_seconds': 60,
}
config['provider_policy'] = {
    'allowed_data_classes': ['SYNTHETIC', 'PERSONAL'],
    'source_ids': [],
}
config['source_configs'] = []
path.write_text(json.dumps(config, ensure_ascii=False, indent=2) + '\n')
path.chmod(0o600)
print('专用目录配置已保存；尚未调用模型。')
PY
```

本轮只接入 PERSONAL 文字，不开放真实邮件、日历或文件同步；`source_configs` 和 `source_ids` 保持为空。不要把真实输入标成 SYNTHETIC，也不要输入 SECRET 或 API key。具体边界见 [真实文字试用说明](docs/RealDataTrial.md)。此步骤不修改仓库内现有配置。

## 3. 启动 Core 与 Runner

终端 A（仓库根目录）：

```sh
./release/secretaryd core --config "$HOME/.secretary-real/config.json"
```

终端 B（另开窗口，同样进入仓库根目录）：

```sh
./release/secretaryd runner --config "$HOME/.secretary-real/config.json"
```

保持两个窗口运行。不要对同一目录重复启动多个 Core 或 Runner。服务启动后会持续处理队列与计划；启动并非单纯查看配置。

## 4. 诊断并打开 TUI

终端 C（仓库根目录）：

```sh
./release/secretary doctor --config "$HOME/.secretary-real/config.json"
./release/secretary --config "$HOME/.secretary-real/config.json"
```

doctor 是本地诊断，不是模型凭据连通测试。在 TUI 中先输入一小段已授权文字，Ctrl+S 或 Alt+Enter 发送，等待最终回复。Enter 只换行。F1 帮助，F2 问题，F3 委托，F4 计划，F5 事项，F6 通知；macOS 若 F 键被系统占用，可尝试 Fn+F 键。

所有使用同一配置的客户端共享唯一权威会话，不需要 session UUID。`ACCEPTED` 只表示受理；GET 查询失败时保留原请求，恢复连接/认证后继续查询，不另起新键重发。事项页过期时 n 暂停，r 明确重载第一页；已打开的确认仍绑定原对象/版本。

## 5. 停止与下次继续

在 TUI 窗口按 Ctrl+C 只退出前端，后台任务仍运行。要完整停止，在终端 A、B **分别按 Ctrl+C**，等待各自回到 shell。数据保存在 `$HOME/.secretary-real`，不要删除它。

下次从第 3 步启动相同目录，再打开 TUI；不再 init，不再覆盖配置。没有安装自启，不监听 TCP，也不自动调整系统睡眠。离开 Mac 或系统休眠会影响持续执行，实际音频叫醒尚未验收。

## 日常命令参考

每个新终端先进入仓库根目录并定义：

```sh
export SEC_CONFIG="$HOME/.secretary-real/config.json"
./release/secretary --help
```

以下含 `<...>` 的位置必须替换为实际 ID、游标或文件路径，不能原样粘贴执行。每条写命令都是独立操作，不要整块批量执行。

### 文字、请求与问题

```sh
./release/secretary chat --plain --config "$SEC_CONFIG"
./release/secretary input --text '替换为获授权的真实文字' --request-id "$(uuidgen)" --config "$SEC_CONFIG" --json
./release/secretary turns <turn-id> --config "$SEC_CONFIG" --json
./release/secretary requests <request-id> --config "$SEC_CONFIG" --json
./release/secretary input --answer-to <question-id> --text '明确回答' --request-id "$(uuidgen)" --config "$SEC_CONFIG" --json
```

plain chat 每行提交，`/quit` 或 EOF 退出；它返回受理信息，不是等待最终答案的 TUI。请保存输入返回的 request_id/turn_id；如需在调用前固定 ID，可先执行 `export SEC_REQUEST="$(uuidgen)"`，输入命令改用 `--request-id "$SEC_REQUEST"`。超时/结果未知时只用该 ID 查询，不重新运行生成 UUID 的提交命令。

### 事项、计划、委托与运行

```sh
./release/secretary items create --title '替换为真实事项标题' --domain project --config "$SEC_CONFIG"
./release/secretary items list --limit 50 --config "$SEC_CONFIG"
./release/secretary items list --limit 50 --cursor '<next_cursor>' --config "$SEC_CONFIG"
./release/secretary items show <item-id> --config "$SEC_CONFIG"
./release/secretary items update --file <ActionProposal.json> --config "$SEC_CONFIG"
./release/secretary jobs create --file <ActionProposal.json> --config "$SEC_CONFIG"
./release/secretary jobs list --config "$SEC_CONFIG"
./release/secretary jobs pause <job-id> --config "$SEC_CONFIG"
./release/secretary jobs resume <job-id> --config "$SEC_CONFIG"
./release/secretary jobs trigger <job-id> --config "$SEC_CONFIG"
./release/secretary tasks show <task-id> --config "$SEC_CONFIG"
./release/secretary tasks cancel <task-id> --config "$SEC_CONFIG"
./release/secretary runs show <run-id> --config "$SEC_CONFIG"
./release/secretary runs cancel <run-id> --config "$SEC_CONFIG"
```

事项创建/ActionProposal 受理后仍须查 turn。委托列表在 TUI F3 查看，CLI 的 tasks/runs 入口是 show/cancel，不能使用不存在的 `tasks list`。取消、暂停、恢复和触发可显式传 `--expected-revision <revision>`；省略时 CLI 会先读当前版本。`jobs trigger` 会实际触发执行。

### 通知、闹钟、世界事实与记忆

```sh
./release/secretary notifications --limit 50 --config "$SEC_CONFIG"
./release/secretary notifications ack <notification-id> --config "$SEC_CONFIG"
./release/secretary alarm stop <alarm-id> --config "$SEC_CONFIG"
./release/secretary alarm snooze <alarm-id> --seconds 300 --config "$SEC_CONFIG"
./release/secretary world list --config "$SEC_CONFIG"
./release/secretary world propose --file <ActionProposal.json> --config "$SEC_CONFIG"
./release/secretary world correct --file <ActionProposal.json> --config "$SEC_CONFIG"
./release/secretary memory --config "$SEC_CONFIG"
./release/secretary memory search --query '查询词' --config "$SEC_CONFIG"
./release/secretary events --after_seq 0 --limit 50 --config "$SEC_CONFIG"
./release/secretary context show <manifest-id> --config "$SEC_CONFIG"
./release/secretary actions --file <ActionProposal.json> --config "$SEC_CONFIG"
```

`--file` 读取**单个 ActionProposal 对象**，不是任意 JSON 或原始文字；operation kind 必须与意图相符，格式见 [API](release/API.md) 与 [Schema](docs/contracts.schema.json)。世界提议须经专用执行器与授权，不因 CLI 受理就成为权威事实。通知查询可能标为 DELIVERED，但只有显式 ack 才确认已读。

### 真实目录的备份与恢复

```sh
export SEC_BACKUP="$HOME/secretary-backup-$(date +%Y%m%d-%H%M%S)"
./release/secretary backup --output "$SEC_BACKUP" --config "$SEC_CONFIG"
./release/secretary verify --backup "$SEC_BACKUP" --config "$SEC_CONFIG"
./release/secretary restore --backup "$SEC_BACKUP" --target "$HOME/.secretary-restored" --config "$SEC_CONFIG"
```

恢复目标必须为空；恢复后新凭据与配置处于执行冻结状态，不直接恢复 live 运行，不移除冻结标记来跳过对账。备份包含私人资料，应留在本地，不提交 GitHub。

## 可选：构建及离线演示

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

真实接入本轮仅支持 PERSONAL 文字，需 Master 明确模型服务商和外发范围后使用新独立 data_dir；`source_configs` 和 `provider_policy.source_ids` 保持为空。真实来源文件同步尚未开放，不能把 fixture_path 改成真实文件路径。仅本地/未知分类资料放在另一独立目录，不共享 SQLite 或 objects；同库混放仍可能保守拒绝。完整边界与验证见 [RealDataTrial.md](docs/RealDataTrial.md)。

```sh
./release/secretary doctor --config /tmp/secretary-demo/config.json
./release/secretary items create --title '合成事项' --domain project --config /tmp/secretary-demo/config.json
./release/secretary items list --config /tmp/secretary-demo/config.json
./release/secretary input --text '合成测试文本' --config /tmp/secretary-demo/config.json
./release/secretary chat --config /tmp/secretary-demo/config.json
```

自然语言输入受理是异步状态，不代表动作已登记或完成。使用返回的 turn_id 查询 `secretary turns <turn-id>`；`secretary requests <request-id>` 查询持久回执。Issue #3 修正受理后查询失败：界面保留原请求，提示“已受理，暂时无法读取结果”；认证错误需恢复认证，不能把原文作为新键再次提交。POST 响应丢失也只查原 request-id；仅提交阶段确定未受理才在空草稿中恢复原文。

面板定时刷新保留已加载页及选中 ID。事项游标失效会标明过期并暂禁 n，继续有界查询当前对象；按 r 明确从第一页重载。目标消失/离开范围会提示并取消选择，已打开的控制确认仍用原目标/版本，不自动 ack。局部修复仅更新 CLI，daemon、Schema、数据库和最小恢复元数据不变。

复杂类型化操作使用 `--file <ActionProposal.json>`：`items update`、`jobs create`、`world propose` / `world correct`。格式严格遵循 docs/contracts.schema.json 的 ActionProposal；示例见 release/examples/reminder.json 与 release/examples/artifact.json，可用 `secretary actions --file <path>` 提交。CLI 默认显示状态说明和详情；`--json` 输出稳定 JSON，脚本应显式使用。

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

配置模板见 `release/config.deepseek.example.json`：profile 为 `deepseek`，endpoint 为 `https://api.deepseek.com/responses`，官方模型 ID 为 `deepseek-flash`。据 [DeepSeek 2026-09-10 官方公告](https://www.deepseek.com/en/news/deepseek-v4-1-flash/)，该 ID 对应 V4.1 Flash；OpenCode 使用的 ID 是 `deepseek-v4.1-flash`，两者不可混填。[官方 Responses 文档](https://api-docs.deepseek.com/api/create-response/)说明此接口无服务端会话状态，本地权威数据和会话继续保存在原 SQLite 中。

切换时保留 data_dir、授权和业务库，只显式替换 model_profile 与对应 secret_ref，重启自己管理的 Core；Runner 不依赖模型切换。已提交请求不重放；失败请求保留原记录，通过新 request_id 明确重新提交未执行的意图。未知外部结果仍须对账，不能因换模型而重做。公开模板不含密钥，实际官方配置和密钥仅存本机。

换模型机械验证使用真实 Core/SQLite 与模拟 HTTP：原模型提交事项、429 不登记新动作、官方协议接续更新同一 Item ID、重复处理不重复效果。真实官方 API 的切换结果单独报告，不能由这项机械测试冒充。

## 内置离线检查

```sh
./release/secretary verify --suite smoke --report /tmp/secretary-smoke-report --config /tmp/secretary-demo/config.json --json
```

`smoke`（同义名 `persistence`）在新的临时库中核验 SQLite 完整性、不可变对象、备份及恢复冻结，并生成 report.json / report.md。它不代替完整 A01—A25 验收。这些属于初版历史验证；Issue #2 只运行 reports/implementation/issue2 中列出的新增定向测试与变更包 vet，不执行旧全仓或长时套件。

公共接口的类型化请求格式与分页示例见 [API.md](release/API.md)。通知列表使用 `secretary notifications`，确认已读使用 `secretary notifications ack <id>`。列表可传 `--limit`、`--cursor` 及对应过滤参数。

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

当前架构见 [SingleConversationTUI](docs/SingleConversationTUI.md)，数据边界见 [RealDataTrial](docs/RealDataTrial.md)。既有两小时及 Issue #1 报告属于各自旧构建，本次 TUI 构建不继承其运行时长结论。
