【回答】

1. F8 更正确认：按既定要求应判定为“固定”。生成器已自动 `gofmt`；`go mod tidy` 已另行运行且无 diff，`go mod verify` 通过。设计并未要求生成器自动执行 `go mod tidy`，本复核不额外增加该验收门槛。

2. M3 F4 更正确认：准确表述应为“由 root 单独完成 CLI/Runner 恢复集成并提供验证；当前 Ayanami 本轮未验证”。这不表示 Master 批准延期、放弃验收或降低要求；F4 仍需由 root 提供集成证据后复核。

原报告保留；本回复作为单独澄清，由 Codex 保存。未修改任何文件。

【参考资料】

- Master 本轮事实澄清
- 原报告：`review/M1-M3-foundation-fix-ayanami.md`

【思路】

只修正 F8 与 F4 的结论措辞，不重新读取代码、不扩大验收范围、不改变原报告其他内容。
