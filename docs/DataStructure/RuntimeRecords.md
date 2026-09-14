# 内部运行记录

以下结构服务于程序协调，不直接作为模型命令输入。同名对象均已在 contracts.schema.json 定义，已有对应 Go 类型；本文补充其协作语义，不能自行改成任意字典。

| 记录 | 关键字段与行为 |
|---|---|
| RequestReceipt | principal_id、request_id、payload_hash、intent_id、state、response_json、created_at；唯一主体／请求键；先 ACCEPTED 后 COMMITTED 或 REJECTED |
| InputTurn | id、session_id、request_id、intent_id、state、input:InputEnvelope、reply:Decision.reply/null、committed_operation_keys:string[]、processing_owner:string/null、lease_until:timestamp/null、updated_at；PROCESSING 崩溃后按租约恢复 |
| ExecutionAttempt | run_id、attempt_no、fencing_token、dispatch_state、prepared_at、dispatched_at/null、finished_at/null、executor_id、command_hash、last_error/null；PREPARED 到 DISPATCHED 在副作用调用前持久化 |
| ConsumerCursor | consumer_id、last_seq、revision；处理业务与游标在同一事务，按事件 seq 补扫 |
| RuleState | rule_id、root_id、cursor_seq、next_allowed_at/null、no_progress_count、last_effect_hash/null；供事件去重、冷却和无进展检查 |
| AlarmSession | id、run_id、revision、state、device_id、audio_ref:ObjectRef、playback_handle/null、saved_settings:object/null、started_at/null、stopped_at/null、stop_reason/null；播放句柄不可由模型伪造 |
| Notification | id、run_id/null、notification_key、state、text、created_at、delivered_at/null、acknowledged_at/null；CLI 收取和明确确认分开；重复读取同一通知不能重复创建通知 |
| CapabilityDescriptor | id、version、arguments_schema、result_schema、side_effect_class、supports_idempotency、supports_query、supports_cancel、timeout_ms；静态注册并验证实现 |
| ModelCallRecord | call_id、root_id、context_id、provider_profile、model_id、request_hash、output_ref/null、started_at、finished_at/null、input_tokens、output_tokens、count_mode、status、error_code/null；嵌入诊断对象并关联 Context |

这些记录的 schema_version、extensions、时间、ID 和错误规则继承 Common。状态枚举以 DDL 和 ExecutionProtocol 为准。AlarmSession.saved_settings 首版至少包括 output_volume:int(0..100)、muted:boolean、device_id:string；恢复音量前确认设备及本次会话仍持有控制权，不能覆盖用户在播放期间主动调整的设置。

## 配置记录

主配置文件另含 ProviderPolicy 和 ServiceConfig：ProviderPolicy 指定 profile_id、允许 data_class、source_ids、local_only、secret_ref；ServiceConfig 保存 profile、路径、时区、认知 epoch、队列和预算默认值。schema_version 和配置哈希记入诊断，SECRET 内容不写配置文件。SourceState 中不存真实 token。

初版输入、来源与事实的服务层不得绕过严格 DTO；运行记录不得成为给模型访问数据库的通用通道。


### D12 分类传播

Task/JobRun/ExecutionAttempt/ExecutorReceipt及复制、REPLAN、产物回执全链join，授权扩展不得丢失。 使用 [Common.md](Common.md) 的 security.classification 契约；缺失旧派生标记返回 OUTPUT_CLASS_UNKNOWN，不自动回填。

## Issue #2 规范化协调记录

002 新增 authority_registry（singleton=1、instance_id、session_id、registered_at、mapping_reason）与 authority_turn（turn_id、accepted_seq、prefix_sequence、prefix_rowid、frozen_state_json）。前者登记唯一 MASTER 会话；后者是受理队列及冻结视图，不作为模型可写 DTO。冻结状态引用完整 ConversationState，原 InputTurn schema_version=1 不变。

主消费者按 accepted_seq 串行恢复；物理 history sequence 与前缀 rowid/已处理界限分开。旧非权威 pending 保留但不自动执行；客户端重连只查询后端投影。具体物理约束见 Storage 与 migrations/002_authority.sql。
