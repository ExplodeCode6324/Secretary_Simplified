# v1.0 历史文档附件校验

此检查器及原报告保留用于初版历史复现。它包含旧 SQLite/Schema 验证，不在本轮运行；Issue #3 使用 `python3 scripts/issue3_docs_check.py` 进行只读文件一致性与影响清单检查。issue2_docs_check.py 及其产物也保留原交付，不运行它覆盖 #2 的历史报告。


检查器只读文档，在临时目录创建 SQLite，再验证 Schema、样例、内部链接、字段覆盖和 SQL 约束。它不会启动 Secretary、使用真实资料、播放音频或调用模型。

需要 Python 3.9+ 与 [requirements-docs.txt](requirements-docs.txt) 中的文档工具依赖。可在隔离虚拟环境安装；这是文档工具，不是 Secretary 的运行依赖。

从项目根运行：

```sh
python3 docs/checks/validate_docs.py --report docs/checks/latest-report.json
```

检查器若没有 jsonschema 会明确失败，不会静默跳过 Schema 检查。报告里的 SQLite 版本是文档工具使用的版本，不能代替待实现 Go 驱动的集成验证。

输出包括原稿哈希、文档与契约数量、正反样例数、SQL 约束检查数和错误。任一错误返回非零退出码。完整运行验收另见 [Acceptance.md](../Acceptance.md)。
