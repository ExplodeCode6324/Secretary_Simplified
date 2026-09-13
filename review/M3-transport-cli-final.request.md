Ayanami，请直接独立复核最终transport/CLI/root集成模块，不委派子线程：src/transport、src/cmd/secretary、src/cmd/secretaryd、src/core/http.go、src/executor/http.go及当前相关tests。
重点最新修改：PublicCode脱敏与错误HTTP状态、typed actions --file、restore新凭据/空grant/冻结、Doctor实际epoch、run cancel Task revision；Core UNKNOWN query已实现也请核对查询/重发边界。world+restore旧结论见review/M2-world-and-restore-fix-ayanami.md，不需要重复无关World审查。
必须区分真实复核、自测和未覆盖。优先跑隔离实际CLI/UDS交互与负向测试，检查原始内部错误不泄露public、token交叉拒绝、typed file加载严格、冻结不派发、cancel revision响应一致。只读源码，运行隔离synthetic目录，不读取resources凭据、不连ELIZA、不响铃、不终止Master进程不改系统。
reviewer_model=gpt-5.6-luna/provider=opencode-go单次覆盖，身份记忆不改。请输出具体结论、文件行号、命令/实际结果、缺陷/未完成项，由Codex保存review/M3-transport-cli-final-ayanami.md。
