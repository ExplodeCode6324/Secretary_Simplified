Ayanami，请针对 Secretary_Simplified 的单项实施设计缺陷商议：ExecutionProtocol 要求计划登记同事务 ChangeEvent，但 TypeRegistry 初始事件没有 scheduled_job 类型，仅 item/task/world/run/source/memory。实施者提议登记 scheduled_job.updated，对应 ScheduledJob 完整 before/after，并同步 TypeRegistry。当前 M3 在修正使用完整 task.updated/run.updated。
请只读 docs/ExecutionProtocol.md、相关 TypeRegistry、DataFlow、ChangeEvent/ScheduledJob Schema 验证是否确有缺陷，然后逐项给出「具体问题 / 证据 / 方案 / 同意或反对」结论。若命名或载荷方案有问题，请提出明确替代方案。不要修改设计或源码。
Master 已授权：实施发现设计缺陷，Codex 与 Ayanami 商议通过后可以修改，但必须在修改设计中注明实施缺陷，README 列修改文档路径。无须再次请 Master 批准。禁止读取 resources 凭据或连接 ELIZA。
请只输出短而具体的复核回复，由 Codex 保存 review/D01-scheduled-job-event.response.md。
