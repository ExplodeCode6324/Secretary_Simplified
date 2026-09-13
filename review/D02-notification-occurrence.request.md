Ayanami，新设计缺陷 D02 请只读商议：周期 ScheduledJob command.arguments.notification_key 与 task_template Criterion.notification_recorded.expected.notification_key 静态复制；notification.notification_key 全局 UNIQUE。第二次 occurrence 会复用第一次通知证据，不再产生提醒。
实施者建议 Scheduler 物化时派生命名空间 key=模板 key+":"+occurrence_key，并同步实例 criterion 对应 key 后固定 hash，模板保持不变。这是实例化占位绑定需文档明确例外，绝不允许 REPLAN 改 criteria。
请核查 src/store/runtime*、Schema、ExecutionProtocol/TypeRegistry等对应文档，明确问题、证据、方案、同意/反对（如需条件列明）。Master 已授权 Codex 与你商议通过后可修设计，注明实施缺陷、README列修改路径，不需再次批准。
禁止修改业务/设计文件、读取 resources 凭据、连接 ELIZA。只输出结论，由 Codex 保存 review/D02-notification-occurrence.response.md。
