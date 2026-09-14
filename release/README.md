# 本地部署

当前二进制目标为 macOS arm64。Issue #3 在 Issue #2 唯一权威会话与 TUI 上修复受理状态误报及分页刷新，新增定向测试与 vet 通过；仅更新 CLI，daemon 和数据库保持原字节。本单最终场景数、复核与交付状态见 [验收索引](../reports/implementation/issue3/README.md)，完整构建哈希见 [BUILD-R](../reports/implementation/issue3/build.json)。真实试用使用独立目录和明确模型授权范围，仅接入受控 PERSONAL 文字；真实来源文件同步尚未开放。完整计划与工作记录见根目录 README。

历史 Issue #1 修复构建为 CLI `84e24025…` / daemon `1d62faad…`；其完整哈希在原报告中，当前构建以 SHA256SUMS 为准。全仓 race/vet、定向故障回归与短时发布包测试已通过；原 final2 两小时测试保留旧构建标识，不将其结果写成此修复版的持续运行证明。Master 明确本轮修复无需重跑该时长测试。

公开的 `db/secretary.sqlite` 是不含授权、凭据或用户数据的空库检查材料；正常部署仍执行 init 创建自己的运行库。

完整使用命令已迁移至 [QuickStart.md](../QuickStart.md)。首次真实文字测试请从其中第 1 步开始。

当前 release 哈希以 SHA256SUMS 为准。历史 Issue #2 实际测试绑定 CLI `8e1f4220…` / daemon `e3756a1a…`，证据见 `reports/implementation/issue2/build.json` 和 `issue2-tui-*-release/report.json`；Issue #3 的新 CLI 证据单列于 `reports/implementation/issue3/`，未变化的 daemon 保持原字节。样例库仍为 001+002，只有空权威登记，不含业务资料或凭据。
