# 数据结构索引

版本：1.0；所有主结构在 [contracts.schema.json](../contracts.schema.json) 的同名 `$defs` 中定义。表中所列为写入归属，不代表额外进程。

| 结构 | 职责 | 存储 |
|---|---|---|
| [ObjectRef](ObjectRef.md) | 原文或产物的不可变引用 | object_ref |
| [InputEnvelope](InputEnvelope.md) | 已认证的输入信封 | input_turn / request_receipt |
| [SourceState](SourceState.md) | 来源同步游标和新鲜度 | source_state |
| [SourceRecord](SourceRecord.md) | 接入后的来源结构化记录 | source_record |
| [Observation](Observation.md) | 带时效和证据的观测 | observation |
| [Item](Item.md) | 业务事项与承诺 | item / item_dependency |
| [WorldUpdateProposal](WorldUpdateProposal.md) | 提交给专用执行器的长期事实修改提案 | world_proposal |
| [WorldFact](WorldFact.md) | 版本化长期事实 | world_fact_version / world_fact_head |
| [WorldModelInput](WorldModelInput.md) | 送给模型的长期事实只读投影 | 嵌入 context_manifest |
| [LiveWorldStateInput](LiveWorldStateInput.md) | 送给模型的实时状态只读投影 | 读取现有表，不单独落世界状态表 |
| [ConsciousnessStateInput](ConsciousnessStateInput.md) | 每 24 小时认知整理的完整输入 | 随快照产物保存引用 |
| [ConsciousnessState](ConsciousnessState.md) | 24 小时认知快照 | consciousness_snapshot |
| [ConversationEvent](ConversationEvent.md) | 会话原话和投递记录 | conversation_event |
| [ConversationState](ConversationState.md) | 有界会话状态 | conversation_session |
| [Task](Task.md) | 带完成标准的任务实例 | task |
| [Command](Command.md) | 待登记的类型化执行命令 | command_ledger / job_run 命令快照 |
| [ScheduledJob](ScheduledJob.md) | 持久触发计划与任务模板 | scheduled_job |
| [DecisionEnvelope](DecisionEnvelope.md) | 模型完整输出，回复与调度提案分离 | decision_record |
| [ContextManifest](ContextManifest.md) | 一次模型请求的可复核清单 | context_manifest |
| [Context](Context.md) | 实际送入模型的有界结构化上下文 | 文件对象 + context_manifest |
| [AuthorizationGrant](AuthorizationGrant.md) | 程序管理的授权记录 | authorization_grant |
| [ExecutionPermit](ExecutionPermit.md) | 绑定执行尝试的短期程序许可 | execution_permit |
| [JobRun](JobRun.md) | 一次触发的持久运行状态 | job_run / execution_attempt |
| [ExecutorReceipt](ExecutorReceipt.md) | 执行器返回的结构化状态与证据 | executor_receipt |
| [Verification](Verification.md) | 针对固定标准的验收记录 | verification |
| [WaitSubscription](WaitSubscription.md) | 持久等待和原子唤醒条件 | wait_subscription |
| [ChangeEvent](ChangeEvent.md) | 状态变更与恢复消费事件 | change_event / consumer_cursor / rule_state |
| [RootBudget](RootBudget.md) | 跨子任务共享的持久预算 | root_budget |
| [ConsciousnessDraft](ConsciousnessDraft.md) | 意识整理模型输出内容 | 合入 consciousness_snapshot |
| [ConversationSummaryDraft](ConversationSummaryDraft.md) | 会话摘要模型输出内容 | 合入 conversation_session |
| [RequestReceipt](RequestReceipt.md) | 幂等受理与提交回执 | request_receipt |
| [InputTurn](InputTurn.md) | 可恢复输入轮次 | input_turn |
| [ExecutionAttempt](ExecutionAttempt.md) | 副作用派发前后的执行尝试 | execution_attempt |
| [ConsumerCursor](ConsumerCursor.md) | 事件消费者水位 | consumer_cursor |
| [RuleState](RuleState.md) | 事件规则去重与冷却状态 | rule_state |
| [AlarmSession](AlarmSession.md) | 本地播放和停止会话 | alarm_session |
| [Notification](Notification.md) | 可恢复的 CLI 通知 | notification |
| [CapabilityDescriptor](CapabilityDescriptor.md) | 静态能力登记描述 | 配置及执行快照 |
| [ModelCallRecord](ModelCallRecord.md) | 模型调用诊断记录 | 诊断对象 |
| [CoreWork](CoreWork.md) | P1 内置执行器的持久收件与恢复记录 | core_work |

## 补充规范

- [Common.md](Common.md)：字段格式、错误与嵌套类型。
- [TypeRegistry.md](TypeRegistry.md)：来源、事实、观测、能力和验收条件的二次校验。
- [RuntimeRecords.md](RuntimeRecords.md)：数据库内部队列、投递与尝试记录。

每个 DTO 不是一张表；读模型和传输结构独立定义，避免直接把数据库行当作模型输入。完整 JSON 对象必须先通过 Schema，再通过权限、引用、时区、状态机和版本校验。


## 实施设计修订 D12：派生输出分类

统一程序独占 `extensions["security.classification"]={"data_class":<enum>}`；enum 为 SYNTHETIC/PERSONAL/SENSITIVE/SECRET，严格单字段、禁止额外成员。分类是披露上界，与事实真假、授权或证据质量独立。程序使用实际冻结成功模型请求的有效 class，按旧对象／本次请求／逐字复制来源取最高分类；更新和复制不降级。无 Evidence 或只有低分类 Evidence 均不能证明派生文本为低分类。

模型／客户端在任何结构层注入此键，整请求／整 Decision 拒绝；不读取其标签决定业务，原始拒绝证据保持原字节。持久写入仅使用程序值，其他合法 extension 保留。缺失／非法的旧派生标签保留 unknown，在披露／重推导入口返回 OUTPUT_CLASS_UNKNOWN，不回填 SYNTHETIC、不伪写 SECRET、不删除数据。空内容程序脚手架可无标，固定且不含用户／模型内容的字面量可显式 SYNTHETIC；真实输入原文仍用其原始 data_class。

裁决：`review/D12-output-class-final-contract.response.md`（整体替代初稿），反注入补充：`review/D12-injection-oracle-clarification.response.md`。无 DDL 或顶层 class 字段扩张。

## Issue #2 契约索引

公共入口省略会话的适配语义见 InputEnvelope；持久排序见 InputTurn；物理事件/认知前缀区分见 ConversationEvent；唯一认知状态见 ConversationState；生成与审计边界见 Context、ContextManifest、ConversationSummaryDraft。既有内部 JSON DTO 保留 schema_version=1 与严格字段；新增权威登记与轮次表属于 002 数据库迁移，HTTP 快照投影见 Interfaces。没有把客户端会话状态变成权威写入 DTO。
