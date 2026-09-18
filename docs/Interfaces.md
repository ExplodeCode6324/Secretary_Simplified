# 接口与错误协议

所有接口属于 v1 设计，尚未提供运行服务。模型参数、网络 DTO、存储行不是同一个类型。

## 1. 本地客户端

HTTP/1.1 over Unix socket，默认 `$STATE_ROOT/run/core.sock`；不监听 TCP。请求最大 64 KiB，UTF-8 JSON，拒绝重复键、非有限数与未知字段。初版大文件通过已授权子任务读取，不提供任意上传接口。认证令牌放 header，不能写入审计正文。写操作须 `Idempotency-Key: UUID`，与 body 的 request_id 一致。客户端身份、响应 envelope、普通聊天与晚任务通知见 [RuntimeProtocol](RuntimeProtocol.md)。

| 方法与路径 | 请求 | 响应/语义 |
| --- | --- | --- |
| POST /v1/inputs | InputRequest | 202 Acceptance；只表示持久接收 |
| GET /v1/requests/:id | 无 | 接收、处理、关联 task/reply、最后错误；身份限定 |
| POST /v1/tasks/:id/cancel | CancelCommand | 稳定接收记录；可随后查询取消结果 |
| POST /v1/tasks/:id/patch | TaskPatch | 更新任务定义的新 revision；不允许模型直接调用 |
| POST /v1/grants | GrantCommand | 可信 Master 控制面创建/收窄/撤销；记录依据 |
| POST /v1/proposals/:id/decision | DecisionCommand | 版本绑定的决定；不会隐式批准其它提案 |
| GET /v1/tasks | state、cursor、limit | 默认 20，最多 100，稳定 cursor 与 cut_seq |
| GET /v1/tasks/:id | 无 | 当前版本、尝试、要求、证据、未决副作用 |
| GET /v1/events | after_seq、limit | 可见事件；内部凭据/原始敏感正文不在 feed |
| GET /v1/replies | after_seq、limit | 已提交正式回复，客户端按 reply_id 去重 |
| POST /v1/replies/:id/ack | request_id、client_id | ACKED，重复 ACK 无副作用 |
| POST /v1/control/pause | LifecycleCommand(PAUSE) | 停止新派发，保留查询/接收/取消；执行者逐一核查 |
| POST /v1/control/resume | LifecycleCommand(RESUME) | 仅在恢复和门禁通过后恢复调度 |
| POST /v1/control/stop | LifecycleCommand(STOP) | 持久接受后执行有限时长的正常关机；结果可重启后查询 |
| GET /v1/health | 无 | READY/PAUSED/RECOVERING/DEGRADED、版本和积压计数 |
| GET /v1/data/delete-plan | object_id（可重复） | 影响清单、对象与备份 ID、当前 event seq 作为 expected_revision |
| POST /v1/data/delete | DeleteCommand | 对预览版本执行删除；遵循本次明确指令及恢复 ledger |

`InputRequest` 是文本输入；其它控制命令用相同持久 RequestEnvelope，但分别验证 action-specific payload。统一命令注册表见 [DataContracts](DataContracts.md)。客户端超时不能推断接收失败：以原 request_id 查询或重发，相同 ID 不生成新任务。

分页 cursor 是服务器生成的不透明值，绑定查询参数 hash、读取切点、最后排序键。客户端不把字段为空但 cursor 存在当作无限下一页；重复 cursor 视作协议错误。正式事件顺序以 seq 为准；显示时间可按客户端时区转换，存储使用 UTC。

## 2. 主会话工具

| 工具 | 参数要点 | 结果/限制 |
| --- | --- | --- |
| task.create | TaskDraft，关联当前 request | 返回持久 task_id；不等待子任务完成 |
| task.get/list | id 或有界查询 | 权威任务状态和证据引用 |
| task.propose_patch | task_id、expected_revision、变更建议 | 产生待决提案，不自行扩大范围/放宽验收 |
| task.cancel | task_id、expected_revision、当前输入依据 | 仅已有用户取消意图/策略覆盖时提交，否则待决 |
| memory.search | task/entity/time/keyword/cursor | 有界、带状态与来源结果 |
| evidence.read | object_id、offset、limit | 分类检查后返回片段；无任意路径读取 |
| cognition.propose | FactProposal | 校验并按政策自动接纳或排队，不直接更新表 |
| entity.resolve | name、scope、source_event_seq、selected_entity_id | 创建或返回有来源的实体 ID；歧义返回候选，不自动同名合并 |
| obligation.propose | ObligationDraft | 创建要求、承诺、待决建议；来源必须关联 |
| obligation.resolve | id、expected_revision、状态建议、依据与证据 | 解决或取消建议经程序/政策核查后生效 |
| task.verify | VerificationProposal | 按当前版本重新核查后提交或拒绝 |

主工具不能执行 shell/http 或加载扩展。内部记忆读取属于主会话职责，不违反“外部任务交给子会话”。

## 3. 子会话工具与 worker 通道

工具为 `file.read`、`file.write`、`command.run`、`web.fetch`、`artifact.create` 和 `task.report`；是否启用由本次 grant 和阶段决定。前四种外部工具走 ExecuteOperation：参数复验 → 权限/资源检查 → 预算/许可 → 实际执行 → 原始回执持久化 → 返回有界结果。artifact.create 仅写受控对象库，task.report 仅提交建议；二者仍校验角色/尝试、去重、额度和持久化，不要求虚构外部资源 grant。

worker 包包含 task_id、task_revision、attempt_id、owner_epoch、cancel_generation、输入对象引用、要求、验收条件、budget_ref 与 grant_ref；不含全量主会话、Master token 或 provider key。结果包含已做事项、产物/证据、未完成项、错误和不确定性。程序为消息附加认证主体和 receipt_id，模型不能证明消息身份。

工具参数 JSON Schema 在 Pi 参数检查后、实际 execute 入口再校验一次；上游 beforeToolCall 可修改参数而不自动再做同等校验，不能信任更早的检查。外部工具错误以结构化错误和 isError 返回，工具错误不等于任务必须立刻失败；重试仍遵守操作幂等和根预算。

## 4. 错误与可恢复性

统一响应：`code / message / retryable / request_id? / current_revision? / details_ref?`。details_ref 引用受控证据，不能泄露绝对私有路径或凭据。

| code | HTTP | 默认动作 |
| --- | --- | --- |
| INVALID_ARGUMENT / SCHEMA_VERSION | 400 | 修正输入；不产生业务动作 |
| UNAUTHENTICATED / FORBIDDEN | 401/403 | 不重试提升权限 |
| NOT_FOUND | 404 | 区分无对象与无权查看的暴露策略 |
| IDEMPOTENCY_CONFLICT / STALE_PROPOSAL | 409 | 查询原请求/当前版本；禁止换 ID 隐式重做 |
| AUTH_REQUIRED / CONTEXT_BLOCKED / RESULT_UNKNOWN | 409 | 保存暂停/未知状态，等待明确恢复 |
| INPUT_TOO_LARGE | 413 | 用受控对象导入或缩小范围 |
| BUDGET_EXHAUSTED / QUEUE_FULL | 429 | 不接受新派发；是否已接收看 request 查询 |
| STORAGE_DEGRADED / MODEL_UNAVAILABLE | 503 | 停止受影响链路；控制面尽可能可用 |

业务状态转换错误在事务前/内拒绝，不能通过改错误文案回避。所有重试由单一 supervisor 调度：瞬时模型错误最多 2 次额外尝试、指数退避 1s/2s 且总 deadline 生效；工具重试由操作类型决定，不能用同一策略笼统重试写入。
