Ayanami，请复查M2 world原F1-F4修复，并对F5边界给最终意见：
- core.InternalHandler Validate ExecutionPermit+run_id/fence/cap match+Store.ValidateIncomingWork比durable完整command/permit+过期grantcancel。
- FinishWorkTx再查job_run actual state/attempt/fence+receipt绑定+RowsAffected。
- Core model WORLD_PROPOSAL一律INFERENCE，只有Typed认证Master接口可MASTER_EXPLICIT，PolicyRevision程序固定1；repository拒policy!=1与CORRECT/RETRACT expected0。
- 新增tests/work_security_test.go两探针+world测试3通过。
- F5直接同request幂等已在Typed整个AcceptTyped事务有receipt/intent幂等；PutProposalTx仅Core内部路径，没有独立公网入口。请验证是否足够或明确还存在何种可达重复效果路径，不把单个内部方法无幂等自动等同外部漏洞。
另外root CLI restore已fresh独立token/new无权限grant+frozen配置，verify --backup使用VerifyBackup；请做隔离实际恢复后runner启动/冻结无claim效果和verify-backup负向测试，给F4集成状态。默认保持冻结，不读resources，不联系外部服务、不响铃、不改系统。
只读源码，隔离安全测试，旧失败保留，当前所有发现给行号/复现/固定或未固定证据；受测模型真实调用由root控制不要自行使用。reviewer_model=gpt-5.6-luna/provider=opencode-go单次覆盖。输出保存review/M2-world-and-restore-fix-ayanami.md。

重试说明：上一轮你委派子线程后该线程API中断，只留下等待说明，没有有效结论。请本轮你直接读取和运行最小独立探针，禁止再委派子agent/异步复核；优先核实原F1-F4与CLI恢复冻结，两者分别输出当前证据。不需重复全量工具链检查超过范围，最多20次工具调用后收口真实结论。
