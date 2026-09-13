D02 复核结论（只读；未修改任何文件，未读取 resources，未连接 ELIZA）

1. 具体问题：周期 occurrence 复用通知身份  
证据：
- `src/store/runtime.go:201-221`：`createRunTx` 将模板 `criteria` 与 `command` 原样复制到每个 occurrence。
- `src/store/runtime_schedule.go:169-173、238-242`、`runtime_control.go:51-76`：不同 occurrence 虽有不同 `occurrence_key`，未派生新的 `notification_key`。
- `src/store/runtime_local.go:56-64`：同 key 且同文本直接返回，不创建新通知；不同文本返回 `IDEMPOTENCY_CONFLICT`。
- `src/store/runtime_local.go:187-190`：Verifier 仅按 `notification_key` 查询。
- `docs/schema.sql:194-197`：`notification.notification_key` 为全局 `UNIQUE`。
结论：同意，D02 缺陷成立。第二次 occurrence 会复用首次通知记录，或因文本变化冲突，均不能正确产生独立提醒。

2. 实例化占位绑定方案  
方案：Scheduler 物化 occurrence 时，基于模板 key 与该 occurrence 的稳定 `occurrence_key` 派生实例 key；同步修改：
- `JobRun.command.arguments.notification_key`
- 实例 `Task.criteria[].expected.notification_key`
随后以最终 criteria 计算并固定 `criterion_hash`。模板 `ScheduledJob.command` 与 `task_template` 不变；同一 occurrence 的重试必须复用同一实例 key。
结论：同意，但必须在 Task/Run 落库前完成，并保证两处 key 一致。不能通过 REPLAN 修改 criteria。当前 `runtime.go:267-279` 的 `IMMUTABLE_CRITERION` 检查和 `runtime_wait.go:142-156` 的 REPLAN 语义应保持不变。

3. 命名方案  
对 `template_key + ":" + occurrence_key` 的裸拼接：反对无条件采用。当前 Schema 对 key 无禁止冒号约束，可能产生歧义或碰撞。
明确替代方案：使用无歧义的版本化编码，例如：

`occ:v1:<base64url(template_key)>:<base64url(occurrence_key)>`

该 key 必须由程序确定性生成，不能包含 attempt、job revision 或当前时间；不建议取消全局 `UNIQUE`。

4. Schema 与文档条件  
证据：
- `docs/contracts.schema.json:2162-2171、2516-2525` 目前只验证两个 key 是字符串，未验证模板 command 与 criterion 一致，也未表达 occurrence 绑定。
- `docs/ExecutionProtocol.md:5-7、15、76` 要求每 occurrence 独立 Task、criterion 固定且 REPLAN 不改目标。
- `docs/DataStructure/TypeRegistry.md:20` 目前将 `notification_key` 描述为任务意图字段，需区分「模板意图 key」与「程序派生的 occurrence key」。
结论：同意修订语义文档；采用上述编码方案时无需新增字段或放宽 DDL UNIQUE，但必须增加服务层校验和测试：
- 模板 key 与模板 criterion 初始一致；
- 每个 occurrence 两处实例 key 一致且唯一；
- 模板 payload 未被物化过程改写；
- 同一 occurrence 重试不产生第二条通知；
- 第二个 occurrence 产生独立通知；
- REPLAN 保持 `criterion_hash` 不变。

5. 需登记的修改路径  
`docs/ExecutionProtocol.md`、`docs/DataFlow.md`、`docs/DataStructure/ScheduledJob.md`、`Task.md`、`Notification.md`、`TypeRegistry.md`；README 增加 D02「实施过程中发现的缺陷」及上述路径。若更新机器契约描述，再同步 `docs/contracts.schema.json`；DDL 不需结构修改。

总裁决：D02 缺陷成立；实例化绑定核心方案同意；裸字符串拼接反对，改用无歧义编码；以上条件满足后可修订设计。
