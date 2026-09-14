# 文档导出版本

- `Secretary_Simplified-design-v1.1-2026-09-14.zip`：Issue #2 当前规范、根说明、release 使用/API与公共样例的逐字副本，校验文件为同名前缀 `.sha256`。以仓库根为解压路径，引用的 src/review/reports 在对应仓库交付中查看。
- `Secretary_Simplified-design-v1.0-2026-09-14.zip` 及其 `.sha256`：初版历史设计快照，保留原字节；其中实施/多会话/验收说明已由 v1.1 现行规范替代，不是当前操作入口。

由 `python3 scripts/issue2_docs_check.py --export` 生成当前导出并核对每个成员与来源字节。此命令不执行旧 Schema/SQLite运行套件、模型调用或长期测试。源码、二进制与运行状态不打入文档 zip；完整产品交付在仓库根与 release。
