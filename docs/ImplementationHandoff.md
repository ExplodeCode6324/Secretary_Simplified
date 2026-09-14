# 当前实施交接：Issue #3

项目已交付单一权威会话和非阻塞 TUI；当前只修复受理后查询误报拒绝与面板分页被刷新重置，不重做原架构或原验收。

1. 阅读 GitHub #3、SingleConversationTUI、Interfaces、Operations 与当前 reports/implementation/issue3 索引。c84d924 为原生产基线，原审计仅静态发现；修前实际测试结果单独保留。
2. RES-01 分离提交与观察阶段，保持稳定 request/已知 turn、恢复元数据及终态；RES-02 保持页缓存、选中 ID、响应代次与确认目标，按既有 items 快照游标处理过期。
3. 每个模块交本地 Ayanami DeepSeek 复核；局部设计修改按实际商议同步正文。公共 API、数据库、Schema、恢复文件 instance_id/request_id 与依赖不变，勿人为新增迁移。
4. 只执行 TestIssue2Residual 定向 race、改动包 vet 和受影响 CLI 构建。不要运行旧 Context/Store/摘要/记忆/恢复、原 13/27 项、全仓、月回放、付费模型或 A25。daemon 和样例 DB 未变化则保持原字节。
5. python3 scripts/issue3_docs_check.py --export 生成新的 v1.1.1 文档副本和本单逐文件影响清单；不覆盖 #2 的原清单/zip/测试报告。公开扫描也必须指定 issue3 新报告路径。
6. 冻结实际修复结果、复核与构建哈希，提交 GitHub 并在 #3 回贴新证据。完成后通知 Master；真实文字测试仍在独立目录和明确披露范围内进行。
