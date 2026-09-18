# 数据契约与存储映射

## 1. 规范与责任

[contracts.schema.json](contracts/contracts.schema.json) 定义 30 个主 DTO，顶层以 `record_type` 判别，`schema_version=1.0`。对象禁止额外字段；格式检查显式启用 UUID/RFC3339。未知 major 拒绝，未知 minor 不擅自忽略字段；升级通过版本化 adapter。

身份/权威：客户端生成 request_id、client_id；其余权威 ID 由程序生成。principal_id、epoch、许可与生效状态由服务端填写，不能从模型 JSON 透传。TaskDraft.requested_access 是请求范围，不是已授予权限；FactProposal.epistemic 是待校验声称，不使模型有资格标为 VERIFIED。

ID 为 UUID；seq/revision 为 JSON 安全整数（0 到 2^53−1，revision 从 1 开始）；Schema/程序检查全部计数范围，SQLite 额外检查 seq 上界及列级非负/关系约束。时间统一 UTC RFC3339 `Z`，到期/有效期比较在程序按时刻处理，不能只依赖任意文本字典序。金额用整数微单位及明确币种配置，不用浮点。

文本长度的 JSON Schema maxLength 是字符限制；接入层还检查 UTF-8 字节上限。入站拒绝重复 JSON key、NaN/Infinity、非整数计数、无效 Unicode 和未知字段。payload_hash 使用 `canonical-json-v1`：校验后对对象键按 Unicode code point 排序、数组保序、无空白 UTF-8 JSON；禁止浮点，字符串按原值保留，不进行语义/空白改写。稳定 ID 相同但有效载荷 hash 不同返回 IDEMPOTENCY_CONFLICT。

## 2. 对象目录

| DTO | 读写责任与主要存储 | 额外业务校验 |
| --- | --- | --- |
| InputRequest / Acceptance | API → requests/events | 认证、字节上限、接收提交后才确认 |
| ObjectRef | ObjectStore → objects/object_links | hash、实际文件同步、分类不能降低 |
| TaskDraft / TaskPatch / Task | 主提案/控制面 → tasks/task_versions | 当前输入依据、旧 revision、验收不自改、资源绑定 |
| GrantCommand / Grant | 可信控制面 → grants/events | 授权来源、scope、到期、分类和主体；模型不能直接建 grant |
| Attempt | Dispatcher → attempts | 绑定不可变任务版本、单活跃尝试、根额度 |
| OperationIntent / Permit | ExecutionGateway → operations/permits | 参数 hash、当前 grant/epoch/取消代际、一次性消费 |
| Receipt | 认证 worker → receipts/objects | 操作与尝试绑定、重复 hash、旧代际隔离 |
| VerificationProposal | 主提案 + 程序检查 → verifications | 每个必要 criterion 恰好一个 PASS；当前任务版本且无未知副作用 |
| ObligationDraft / Obligation | 主提案/确定性控制 → obligations | scope 与来源真实存在，解决依据明确 |
| FactProposal / Fact | cognition → proposals/entities/facts/fact_versions | 来源、有效时间、纠正链、world revision、认识等级 |
| EpisodeDraft / WorkingMemory | MemoryWorker → memory_jobs/working_memory/memory_cursor | 连续覆盖、语义检查、引用、原子水位与版本 |
| ContextManifest | ContextBuilder → contexts/objects | 一致切点、参数版本、最终请求预算/分类 |
| RootBudget | BudgetLedger → root_budgets | 预留与实耗同事务；REQUEST 或 MAINTENANCE 独立配额 |
| ModelCall | ModelGateway → model_calls | 唯一真实请求记录、保守预留、usage 未知不能计零 |
| Reply | CommitService → replies/outbox | 正式回复绑定输入代际；送达状态独立 |
| Event | 所有可信服务 → events | 唯一 ID、单调 seq、因果关系，不含大正文 |
| WorkerEnvelope | 内部传输 + outbox | 通道认证，不信模型给出的 worker 身份 |
| CancelCommand / DecisionCommand / LifecycleCommand / DeleteCommand | Master API → requests/events 及目标对象 | 版本、已有授权范围、控制状态 |
| Error | API/工具响应 | retryable 不授权重做未知副作用 |

存储中的 payload_ref 指向已通过对应 DTO 校验的对象；objects 存元数据与相对路径，不把任意 JSON 当作已验证契约。事件原文 payload 与审计元数据分开，便于删除正文而保留非敏感时间线。

`Task.current_attempt_id` 是按有效任务版本和尝试状态计算的读投影；不是另一个可独立写入的指针。`TaskDraft` 本身不含信任主体；可信上下文由调用链注入。实体名称/别名、经历索引初版可通过对象 payload 和程序查询保存，不急于把全部领域词条拆表。

实体创建、关系 object_entity_id、义务解决与内部候选 ID 的规则见 [RuntimeProtocol](RuntimeProtocol.md)。Dependencies 明确携带 obligation_revisions；Permit 包含 executing_attempt_id 与 revoked_at，不能只凭 originating attempt 判断一次恢复重试的执行身份。

## 3. 程序必须补的约束

Schema/DDL 能检查结构，不能证明以下条件：

1. 所有 input_refs/evidence_ids/obligation_ids 属于可访问安全域、对象仍可用，且与任务相关。
2. 请求和回复的身份、任务版本、授权 scope、epoch、取消代际在执行瞬间仍有效。
3. `valid_to > valid_from`；scope.ref_id 的实际对象类型与 kind 匹配；纠正关系不形成环。
4. 验收结果与当前 criteria ID 集合精确对应，无重复、无缺项、无靠模型伪造的确定性检查结果。
5. WorkingMemory.history_exit_seq <= watermark <= cut_seq；coverage 是连续前缀，不越过 pending；cursor 指向同版本 payload。
6. 模型输入实际 payload 与 ContextManifest 哈希一致，允许分类覆盖全部传入片段。
7. 对象删除/备份/检索失效共同遵守策略；引用 JSON 中的 ID 需由程序验证并写 object_links。

这些检查分别放在 `contracts/business.ts` 与各提交服务，不堆进一个“万能 Schema 校验函数”。验证顺序：大小/解析 → 版本 → Schema/format → 认证上下文 → 引用/版本 → 资源与披露 → 业务状态/预算 → 幂等 → 短事务重新校验并提交。

## 4. 预算结算

调度每次真实模型请求前，短事务增加 calls_used 并预留该次最坏允许 token（受请求窗口/输出上限限制）。网关拒绝且未发送可释放 token 预留，但拒绝请求仍有审计；模型已收到而 usage 丢失则保留保守消耗，不能按零成本释放。后续查到使用量再以稳定 model_call_id 一次性结算。

总额度同时约束次数、token、deadline、可选金额。缓存命中不免除请求次数；费用以实际 provider profile 的计价规则和记录版本计算。并发预留必须 CAS/事务防止双花。自动重试、新 attempt 和语义验收仍用原根预算。后台记忆有每小时 60 次/500,000 token 独立额度，并记录源请求归因；普通任务不能通过触发后台整理绕过总资源观测。

金额预留/实耗在 root_budgets 与 model_calls 分别保存；币种/价格版本来自固定 profile。usage 未知时把最坏预留转保守实耗，同 model_call_id 只结算一次；具体字段与修正规则见 RuntimeProtocol 第 6 节。

如果 provider usage 超出已验证预留范围，记录账务异常并停止该 profile 新请求，不用截小实际数值让约束看似通过。应把异常原始 usage 放对象，调整经审核的上限/账本后恢复。

## 5. DDL 边界与迁移

[schema.sql](contracts/schema.sql) 为新库起点，表内承担唯一键、引用、闭合状态枚举、非负预算、不可变版本和回执等约束。events、task_versions、fact_versions、receipts 不允许普通 UPDATE/DELETE；正文删除通过 ObjectStore tombstone。投影表可重建，但权威业务表不可随意清空。

task 与 task_versions、fact 与 fact_versions 的当前指针使用 deferred FK，必须同事务插入。触发器不负责调用模型或外部动作。object_links 的多态 owner、授权旧版快照及状态变更依据由 events/payload 和应用验证提供，不宣称全部跨字段约束已由 SQL 实现。

当前库不复用 Pi SessionRepo；Pi 自带的 SQLite 会话后端解决的是执行历史存储，不等价于本项目任务、授权、许可和原子 outbox。把它另建一套会话权威会重新引入双写。主/子运行历史由 Journal 和 ContextBuilder 重建即可。
