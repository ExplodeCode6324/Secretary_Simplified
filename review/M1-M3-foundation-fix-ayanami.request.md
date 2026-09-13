Ayanami，请对当前源码做 M1 foundation 与 M3 diagnostics 初审缺陷修复收口，分别给结论。原始报告 review/M1-foundation-ayanami.md、M3-diagnostics-ayanami.md 不覆盖，独立核验以下实现者声明。
M1：F1结构化RevisionConflict.CurrentRevision；F2/F4先dedup/校验+quarantine内容去重，ObjectStore内容寻址；F5RowsAffected；F6normalize err；F7INITIALIZATION_INCOMPLETE；F8生成器自动gofmt+go mod tidy。已接受你的局部D1/D2方案，全局编号D03/D04，改docs/contracts.schema.json、DataStructure/SourceRecord.md、Acceptance.md及README。
M3：manifest media/data_class/created_at全部DB绑定；必需键/未知键/null objects/date严格拒绝；checkpoint错误检查+objects目录fsync+末次VerifyBackup；doctor状态DEGRADED且last_backup明确UNKNOWN。CLI恢复F4由root另修，本轮不要把未完成root集成称模块完全通过。
实现者运行 go test ./tests ./store ./diagnostics ./ingest 通过，race在跑。请隔离副本独立探针验证旧失败已经拒绝、旧有效行为未回归；给每F项固定/未固定/未验证状态和代码行号证据。不要扩大到所有模块。
本次单次 reviewer_model=gpt-5.6-luna/provider=opencode-go，身份记忆不变。只读业务/设计，不读resources、不连接ELIZA、不响铃不改系统。允许隔离安全测试，无需再申请。只输出回复，由Codex保存review/M1-M3-foundation-fix-ayanami.md。
