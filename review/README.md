# Ayanami 模块复核

此目录保存本地 Hermes Ayanami 对实现模块的原始复核回复。复核调用由 Codex 协调，回复不等同于程序验收通过；正式验收仍须依据 `docs/Acceptance.md` 的独立测试证据。

- 入口：本机 `hermes chat`，default profile；身份通过本机 SOUL 的名称核实。
- 会话：`secretary-simplified-module-review`。
- `00-ayanami-handshake.md`：设计基线与复核准备确认。
- 后续每个模块使用独立编号文件保存请求和原始回复，并记录相关测试结果及修复复核。
- stderr 单独保留用于追踪调用失败；不包含密钥；发布前需统一扫描。
- 不读取 `resources` 中的凭据，不连接远程 ELIZA，不将 Codex 自审包装成 Hermes 的独立审查。

模块只有在真实回复到达后才标记已复核。API 失败、超时或空回复均记录为未完成，不视为赞同。

## 复核索引

- [设计修订授权确认](01-design-change-authorization.response.md)：Ayanami 已确认 Master 授权范围。
- [M1 foundation 初审](M1-foundation-ayanami.md)：有条件；独立探针揭示需修复项及 tombstone 契约缺陷。原始测试探针/日志位于 [M1-evidence](M1-evidence)。不得当成最终验收通过。
- [D01 计划事件登记](D01-scheduled-job-event.response.md)：同意，含映射/原子事务/事件边界/测试与文档同步条件。
- D02、M3 runtime、M3 diagnostics 初次调用遭遇 API Connection error，调用失败不代表复核否决或通过；重试与最终状态以对应 attempt 及 response 文件为准。

## Reviewer 模型切换记录

M1 初审与 D01 成功回复使用本地 Ayanami default 模型 `deepseek-v4.1-flash`（provider `opencode-go`）。其后 D02/runtime/diagnostics 遭遇连接失败，原始失败分别保留 attempt 文件。经当前任务协调方授权，复核仅对单次 `hermes chat` 使用 `--provider opencode-go --model gpt-5.6-luna`；身份、记忆、会话与持久配置未变。用户已明确选择该模型。

[切换握手](02-luna-reviewer-handshake.response.md) 已真实成功。此后 D02、M3 runtime、M3 diagnostics 的当前 response 文件来自 `gpt-5.6-luna`；各自原始 deepseek 失败均不构成审查结论。审查意见仍须源码与测试证据支持，不以模型赞同取代验收。

## 后续已到达结论

- [D02 周期通知身份](D02-notification-occurrence.response.md)：同意实例化绑定，要求版本化无歧义编码；禁止裸拼接、禁止 REPLAN 改 criterion。
- [M3 runtime 初审](M3-runtime-ayanami.md)：需修复；独立 probe 发现 permit/cancel/reconciliation 缺陷。证据位于 [M3-runtime-evidence](M3-runtime-evidence)。
- [M3 diagnostics 初审](M3-diagnostics-ayanami.md)：有条件；manifest、doctor 与恢复 CLI 集成需修复。
- [M2 world 初审](M2-world-ayanami.md)：需修复；跨 run permit、旧 fence receipt 与事实准入权限等问题已交实现者。
- [D05 意识槽验收条件](D05-consciousness-criterion.response.md)：同意，须实现唯一 SlotController、命令指定 slot、精确 DTO 验证与移除直接 Refresh 旁路。
- 以上 D02/M2/M3/D05 均使用单次 `gpt-5.6-luna`，原有 Ayanami 身份与记忆保留。

所有初审结论限定于报告中的源码快照。并行实施后的修复必须由后续报告明确核实；不能静默覆写原失败或把旧结论迁移为当前通过。
