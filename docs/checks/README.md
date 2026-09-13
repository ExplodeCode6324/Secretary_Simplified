# 文档附件校验

检查器只读文档，在临时目录创建 SQLite，再验证 Schema、样例、内部链接、字段覆盖和 SQL 约束。它不会启动 Secretary、使用真实资料、播放音频或调用模型。

需要 Python 3.9+ 与 [requirements-docs.txt](requirements-docs.txt) 中的文档工具依赖。可在隔离虚拟环境安装；这是文档工具，不是 Secretary 的运行依赖。

从项目根运行：

```sh
python3 docs/checks/validate_docs.py --report docs/checks/latest-report.json
```

检查器若没有 jsonschema 会明确失败，不会静默跳过 Schema 检查。报告里的 SQLite 版本是文档工具使用的版本，不能代替待实现 Go 驱动的集成验证。

输出包括原稿哈希、文档与契约数量、正反样例数、SQL 约束检查数和错误。任一错误返回非零退出码。完整运行验收另见 [Acceptance.md](../Acceptance.md)。
