> 发布副本：仅对本机路径作规范化；原始报告本地保留于 `review/private/`。问题、结论和测试结果未改动。

复核报告（reviewer: Ayanami 指定 DeepSeek 实例；工具轮次 4/4，其后为本自然 final）

【结论】限定 PASS — 在本次迁移声明的核对范围内未发现错误：链接目标全部存在、命令与 cwd 适配、旧段无重复命令残留、命令参数与首启配置同 src/cmd/secretary/main.go 及 src/config/config.go 相符。附 1 条既有内容观察项（非本次迁移缺陷，见下）。

【核对要点与实际证据】

1) 迁移结构与只读性
   - git diff --stat：README.md +6 行；release/README.md 净 -91 行（删 92、留 1）。QuickStart.md 为新增（untracked）。其余 tracked 变更无。
   - release/secretary、release/secretaryd、release/SHA256SUMS 为 tracked 且工作区干净 → 二进制与校验单未被本迁移改动，与"只改文档"一致。
   - README.md 的 diff 纯新增：[QuickStart：首次真实文字测试与完整使用命令](QuickStart.md) 链接行 + "使用文档拆分（2026-09-14）"说明段；未删除既有内容。release/README.md 的 diff 纯删除旧命令段 + 新增一行链接。

2) 旧段残留（去重核对）
   - release/README.md 现全文：0 个 ``` 代码块、0 处 ./release/secretary 命令（grep 双重确认）。原命令区（旧行 15-90 的全部 sh 块）已删除，原处仅保留 [QuickStart.md](../QuickStart.md) 链接（现第 9 行）。
   - 旧命令内容确认已迁入 QuickStart.md，且参数保持：init --data-dir、secretaryd core/runner --config、doctor/items create/items list/input/chat、backup --output/verify --backup/restore --backup --target、verify --suite smoke --report、input --answer-to、input --data-class SYNTHETIC。
   - 路径已按新 cwd 适配：examples/reminder.json → release/examples/…、API.md → release/API.md、config.deepseek.example.json → release/config.deepseek.example.json，均指向实际存在文件。

3) 命令参数对照 main.go（src/cmd/secretary/main.go，488 行）
   - items create（--title/--domain，默认 project）、items list/show/update --file、jobs create/list/pause/resume/trigger、tasks/runs 仅 show/cancel（文档"不能使用不存在的 tasks list"与代码一致）、notifications/notifications ack、alarm stop/snooze（--seconds 必填）、world list/propose/correct、memory/memory search --query、events（--after_seq/--limit）、context show、actions --file、turns/requests、input（--text/--request-id/--answer-to）、chat --plain（/quit 或 EOF 退出）、doctor、backup/verify/restore、migrate --authority-session、--help：均在代码中找到对应分支与参数。
   - 取消/暂停/恢复/触发支持 --expected-revision，省略时先读当前版本：与 mutation() 逻辑一致。
   - init 二次执行拒绝（"已有 config.json 会拒绝重复初始化"）：与 initData 的 already initialized 一致。
   - 默认 --data-class=PERSONAL、--json/--plain 语义：与 flags()/output() 一致。

4) 首启配置 profile=opencode-go 与 PERSONAL 显式授权（src/config/config.go）
   - Default()：profile=fixture，allowed_data_classes=[SYNTHETIC] → 第 2 步文档"初始化默认是离线 fixture，只允许 SYNTHETIC"成立。
   - Validate()：profile 仅接受 fixture/opencode-go/deepseek；opencode-go 强制 endpoint=https://opencode.ai/zen/go/v1/responses 且 model=gpt-5.6-luna，与 QuickStart 脚本写入值逐字一致；SECRET 在 Validate 与 Allows() 双重拒绝。
   - PERSONAL 无任何默认入口，只能由用户显式把 PERSONAL 写入 allowed_data_classes（QuickStart 以明确同意语表述后才给出该脚本）→ "PERSONAL 需显式授权"在代码常量与文档两侧成立。

5) 相对链接目标存在（存在性检查，未读内容）
   全部 OK：docs/RealDataTrial.md、release/API.md、docs/contracts.schema.json、docs/SingleConversationTUI.md、release/examples/reminder.json、release/examples/artifact.json、release/config.deepseek.example.json、release/config.local.json、docs/Operations.md、docs/Interfaces.md、scripts/build.sh、reports/implementation/issue3/README.md、reports/implementation/issue3/build.json、docs/README.md、docs/design.md、docs/EngineeringArchitecture.md、docs/ImplementationHandoff.md、docs/Milestone.md。

6) cwd 适配
   QuickStart 明示"从仓库根目录执行"，全部命令用 ./release/…（已确认 release/secretary 存在）；(cd release && shasum -c SHA256SUMS) 为自含子 shell；离线演示用绝对路径 /tmp/..。无按 release/ 目录 cwd 执行而未适配的残留。

【观察项（非本次迁移缺陷，不扩大验收）】
README.md:249（HEAD 原 247 行，既有内容）："正常启动：./release/secretary --config <配置>；…" 一行内联提示。该行不在本迁移 diff 内（本迁移对 README.md 纯新增），不属"旧段残留"；如 Master 要求 QuickStart 之外零命令提示，可另行处理，本次未计为错误。

【未覆盖（超出允许只读范围，未验证）】
- secretaryd core/runner 子命令参数（daemon 源码不在允许读文件内）
- diagnostics/contract/store 内部（smoke 套件同义名、ActionProposal 解码、restore 空目标约束、策略强制点）
- release/SHA256SUMS 行内容与 scripts/build.sh 行为（仅确认存在）
- 外部 URL（DeepSeek 公告/文档）未抓取；TUI 运行期行为未运行验证

【实际所读路径】
- <PROJECT>/QuickStart.md（全文 265 行）
- <PROJECT>/README.md（相关段：21-29、249、252-256 及 git diff/HEAD 对照）
- <PROJECT>/release/README.md（全文 11 行）
- <PROJECT>/src/cmd/secretary/main.go（全文 488 行）
- <PROJECT>/src/config/config.go（全文 115 行）
- 存在性检查（不读内容）与 git 只读查询；未读上述清单之外的任何文件内容。

【三文档 SHA256（复核时）】
- QuickStart.md     5607abe50735acb9da01805fff157df7df4145219f26560b1290ba312b2042f7
- README.md         6fb75ded7cf58c32bfa5b601f75202e9b2d6c2b9f2eeaef6b38a0823892065d7
- release/README.md d42354b0e7c2ec2fa40ac31db84c457010d7cbc74d51bb98867873712a3a69b1

【reviewer 声明】
本次复核由真实 DeepSeek 模型执行（provider=opencode-go，model=deepseek-v4.1-flash）——复核过程本身即一次模型调用，不能声称"全程无模型调用"。未调用 Secretary 产品模型/资料；未启动任何服务、未运行测试、未读取 secrets、未运行旧 checkers/export；全程只读（shasum/stat/grep/git 查询），未创建或修改任何文件（工作区显示的 review/ 与 quickstart-publication-scan.json 非本次复核产生）。除上述三项核对与首启配置对照外，未扩大旧验收。
