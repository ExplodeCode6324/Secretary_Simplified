Ayanami，请真实独立复核新完成的 M3 diagnostics/backup 模块：src/store/backup.go、health.go、src/diagnostics/ 与对应 tests。实现者报告 go test ./store ./diagnostics 通过，race 在运行，请自行验证。
实现说明：SQLite online backup 真实接口 -> snapshot refs -> immutable object hashes -> manifest；restore 新空目录验证 manifest/DB/object/integrity/FK/migration 并生成 execution_frozen 文件；doctor 缺少心跳/扫描/备份/预算记录明确 UNKNOWN。root 正在 CLI 集成且 runner 须校验冻结 marker，请区分模块已有内容和集成未完成边界。
请读设计相关备份恢复及诊断约束，执行隔离安全测试，只读业务源码，不修改业务文件，不读 resources 凭据，不连接 ELIZA，不发响铃，不终止 Master 进程，不改系统设置。Master 已授权实现/隔离测试，无需重复申请。
输出通过/有条件/需修复结论、严重性、具体文件行号、触发场景、修复建议、检查命令与未覆盖项，不能把编译通过称 A20 正式通过。只输出回复，由 Codex 保存 review/M3-diagnostics-ayanami.md。
