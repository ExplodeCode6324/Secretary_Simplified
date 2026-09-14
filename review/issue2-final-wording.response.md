—— 措辞升级核证完成（只读两段 + 一条 shell 校验；未重跑任何 tests/check）——

【回答｜核证结果：PASS，升级表述精确、无超证】

1) 两段正文（Interfaces.md:136 / Operations.md:72，均 13:22:52 更新）逐字为：
"本轮 release 终端恢复实证覆盖正常退出、Ctrl+C 与可捕获 SIGTERM；另有实际 Model 包装、相同 Run 选项的独立测试进程 panic 注入通过，证据见 reports/implementation/issue2-tui-panic-supplement.md。生产二进制未加入故障注入钩子；SIGHUP/SIGQUIT 未作本轮运行验证，不扩大为所有信号保证。纯文本入口写作 `chat --plain`；裸 `secretary --plain` 不是已承诺的模式切换。目标为 macOS arm64，Linux 仅编译验证，未声明其终端运行通过。"
→ 相较上份报告"panic 尚无本轮运行证明"的保守表述，现为精确补证范围：panic 补证（child-only、实际 Model、相同 Run 选项、独立测试进程）＋生产无注入钩子＋SIGHUP/SIGQUIT 仍未验＋不扩大为所有信号保证。既不冒 PASS 也不隐藏补证，升级判定：确认。

2) README.md:237 与 reports/implementation/issue2/README.md:23 均已带补证路径（issue2-tui-panic-supplement.md），且写明：首轮等待/drain 失败与修正后 PASS 分别保留、owned 测试进程全部退出无遗留常驻、不声称正式 release 含注入入口、Linux 未作运行验收。

3) 最终包核证（shell 单条完成）：zip = docs/deliverables/Secretary_Simplified-design-v1.1-2026-09-14.zip（205777 B，13:22:52），实际字节 sha256 == .sha256 文件：ZIP_SHA_MATCH。

**最终 zip hash（请以此为准）：15f6236bc4187cffd77167315aee9cc9a66542314b26372038c3030dce8b4f64**
（替代我 13:20:57 导出时的 956c4e350a78…，差异来源为 Interfaces/Operations/README/索引的措辞升级字节；release/secretary、secretaryd 13:13:35 后未重建，生产 8e1f4220…/e3756a1a… 不变，与此前核证一致。）

【后续处置确认】
- review/provenance 新增文件按证据收编：root 仅重建清单，不改规范正文，不因此循环再审 —— 认可，我方不再就此发起复核。
- AUTH 新 tests 误称（"旧套件"）由协调 README 加注纠正、不改旧原文 —— 认可该处置，与我上轮注释项一致，无需我动作。

【边界】
未跑任何测试/检查器（含 checker 本身）；仅 grep 两段 + 一条 shasum 比对；工具轮 1/2。最终 hash 已在上文留档（stdout 由调用方归档）。DOC01–05/BUILD 静态 scope 维持 PASS，无 mustfix。（ _ _ ）
