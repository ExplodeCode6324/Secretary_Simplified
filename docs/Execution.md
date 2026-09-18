# 执行、事务与恢复协议

## 1. 排序、版本与控制优先级

每次启动取得 OS 独占锁，短事务增加 `owner_epoch`。内部事件用数据库分配的 `seq` 排序；发生时间用于语义，不用于去重。每个逻辑主会话有 `input_generation`：新 Master 输入、控制修改和必须使当前回答失效的纠正递增该值。

优先级：停止/撤权/取消 → 用户输入持久接收 → 在途结果接纳 → 主交互/验收 → 子任务派发 → 后台记忆。入站持久化不等待任何模型。SQL 事务不得包含模型、网络、进程等待；数据读写用有界队列，拒绝队列过载时返回未接受。

普通自然语言输入到达时，先接收并递增输入代际，使旧轮回复不能直接成为正式最新答复；向主 Agent 请求中止，在安全工具边界结束并重组。只有已解析并经可信接口接纳的任务修改才改变任务版本；不能靠关键词把普通引述当作撤权。显式 `/cancel`、`/revoke`、`/pause` 不经过模型。

新输入在语义解释前，不自动改变所有子任务权限；若用户意图尚不明确，可显示“已接收，当前任务仍在运行”。明示暂停命令立即限制新派发。主轮中止不自动取消子任务；子任务取消必须是单独的权威操作。

提案携带 `expected_task_revision`（若有关任务）、`expected_world_revision`、`input_generation` 和相关要求/授权版本。提交时按读集合校验；变化与提案无关可确定性重验，无法证明则拒绝为 `STALE_PROPOSAL` 并重新读取。不得用整个数据库全局 revision 的任何变化无差别否决所有任务，以免后台事件使主会话饥饿。

## 2. 任务与尝试

权威状态为 `QUEUED / RUNNING / VERIFYING / PAUSED / SUCCEEDED / FAILED / CANCEL_REQUESTED / CANCELLED / RESULT_UNKNOWN`。允许的边及守卫见 [状态机](contracts/state-machines.json)。枚举不是授权；每条边还必须检查身份、前置版本、证据和相应业务守卫。

- `QUEUED`：任务已保存，尚不保证有授权；缺授权进入 `PAUSED`，原因 `AUTH_REQUIRED`。
- `RUNNING`：有效尝试启动。`VERIFYING`：结果已持久接纳但尚未证明完成。
- `SUCCEEDED`：当前任务 revision 的所有必要验收项满足；未验证项不能用“部分成功”掩盖。
- `FAILED`：有确定失败依据，且没有未核查外部动作。重新尝试由明确控制事件创建新 attempt；终态任务不原地改回执行中，可创建关联新任务。
- `PAUSED`：没有未经核查的在途副作用，因授权、额度、输入或依赖等待；保存恢复条件。
- `CANCEL_REQUESTED`：立即提升 `cancel_generation`，阻止新许可，向进程发送中止；尚不保证已停止。
- `RESULT_UNKNOWN`：至少一个动作是否产生副作用尚不确定；只允许查询/核查操作，无普通写重试。
- `CANCELLED`：已无可继续的执行者，在途结果已核查；允许存在已记录的既成副作用。

任务定义使用不可变 `task_versions` 保存目标、输入、要求引用、允许资源请求、验收标准。变更由 Master 或其明确授权的程序控制接口产生新 revision；旧 attempt 绑定旧 revision。模型可提议放宽条件，但不能自行批准。修改中的运行任务先限制新动作；兼容性不能证明则取消旧尝试后重派。旧证据可按新标准重新验收，不可直接复用旧“成功”结论。

尝试状态为 `PREPARED / RUNNING / REPORTED / ABORTING / ENDED / UNKNOWN`。每次新尝试消耗根预算；目标系统的同一操作重试保持 `operation_id`，不因新 attempt 重新制造一次副作用。子任务递归委派初版禁用，模型返回的委派请求只是未支持操作。

## 3. 事务边界

| 编号 | 事务内必须一起写 | 事务外动作 | 崩溃后的处理 |
| --- | --- | --- | --- |
| TX1 接收 | request、USER_INPUT/CONTROL_RECEIVED 事件、输入代际、待处理状态 | 已保存的原文对象；随后发送 ACCEPTED | 同 request_id/hash 返回同接收记录；hash 不同返回冲突 |
| TX2 派发 | task/version、attempt PREPARED、DISPATCH outbox、关联事件 | 发送已提交派发消息 | outbox 重发，worker 按 attempt_id 去重；没有提交不得启动 |
| TX3 许可 | 校验任务/授权/预算/代际；operation INTENDED、预算预留、一次性 permit | 网关取得许可并开始实际操作 | 无开始证据且无法证明未执行，进入 UNKNOWN；不能猜测安全重试 |
| TX4 起动 | permit 标记已消费、operation DISPATCHED、OPERATION_STARTED 事件 | 紧接着调用目标工具 | 起动与外部动作有不可消除空窗；恢复需查询 |
| TX5 回执 | 不可变 receipt、operation 结果、消耗结算、必要 task 状态、事件 | 证据对象已保存并校验哈希 | 重复 receipt 相同 hash 无变化；不同 hash 冲突并核查 |
| TX6 验收 | verification（条件逐项）、任务终态、正式 reply、REPLY outbox、事件 | 发送正式回复 | 重发/查询同 reply_id；不重跑任务 |
| TX7 记忆 | working_memory 版本、连续水位、源范围与覆盖清单、事件 | 先得到可验证 draft | CAS 失败重新整理，不发布水位超前快照 |
| TX8 事实 | fact version、当前指针、world revision、纠正关系、投影失效事件 | 提案和证据已验证 | 事实与版本事件一致；缓存不得压过权威版本 |

数据库写失败立即进入 `STORAGE_DEGRADED`：停止新的模型/工具派发，不发接受确认；当前动作请求停止并保留能保留的运行信息。磁盘恢复后对可能已执行的动作核查。日志本身写不了时，不能伪造一个“已持久记录错误”。

## 4. 授权到操作的绑定

`grant` 只由可信 Master 控制面或已批准静态策略创建。操作许可由网关生成，绑定 grant revision、task revision、owner epoch、cancel generation、能力、规范化资源、参数 hash、到期时刻和 operation_id。模型不得提供有效性判断或自报主体。

调度器、许可消费和实际资源网关检查最新值。撤权事务与许可消费按同一顺序提交：撤权先发生则消费失败；消费先发生的动作按在途副作用处理。不能宣称撤权可以原子撤销外部服务器已经接收的请求。并行工具初版关闭。

命令和写入资源在执行边界验证规范路径、授权根、符号链接/硬链接策略，敏感路径与网关凭据绝不进入子进程。单纯在模型 tool_call 前过滤参数不足以提供 OS 隔离。

操作分为 `READ_ONLY / IDEMPOTENT_WRITE / NON_IDEMPOTENT_WRITE`。只读也必须防外发，并不意味自动允许。幂等写必须记录目标幂等键、有效期与查询手段；窗口过期的未知结果不得重试。非幂等写在断连后默认 UNKNOWN，需目标查询或 Master 根据证据决定；新的明确操作须新 operation_id 并关联原未知操作。

## 5. 回执、验收与回复

回执经认证的 worker 通道接收，task/attempt/operation 绑定由网关核对。旧代际 worker 不得修改权威状态，但其报告可保存为 `QUARANTINED_RECEIPT` 供协调器查询目标核查，防止丢失既成副作用证据。

验收条件在任务版本中固定为 `FILE_HASH / FILE_EXISTS / EXIT_CODE / TARGET_STATE / SEMANTIC`，每项说明目标和期望。`EXIT_CODE=0` 仅能证明进程返回，不能替代其它业务目标。语义验收保存判断模型/策略版本、引用与未验证部分；不使用子模型自报成功作为唯一标准。

一条正式回复有稳定 reply_id、关联输入代际、任务版本及证据引用。模型产生的流式文字仅为 `DRAFT_DELTA`，UI 明示尚未正式提交；中止后不得当作完成记录。初版可只显示简短程序进度，将模型正文缓冲到 TX6 后发送，以降低语义混淆。

交付状态 `PENDING / SENT / ACKED / UNKNOWN` 独立于任务状态。`SENT` 表示写入传输，不证明用户读到；客户端保存 reply_id 去重，并明确 ACK。断连后重发不会再次执行任务。

TX6 同时支持无 task 的普通聊天、异步派发确认和终态结果；旧任务完成不能因新输入而丢失权威终态，但旧轮草稿必须失效。具体提交与回复去重键见 [RuntimeProtocol](RuntimeProtocol.md)。

## 6. 启动恢复顺序

1. 核查数据目录、独占锁、Schema 版本、数据库 integrity/foreign keys 和关键对象；故障进入仅诊断模式。
2. 增加 owner_epoch；使旧 permits 不可再消费。查找遗留执行者并按执行记录恢复/停止；进程 PID 不单独作为身份，需启动随机标识和进程起始时间。
3. 已提交 outbox 恢复发送；没有许可消费的派发可去重重投；已消费而无确定回执的外部操作先 UNKNOWN。
4. 已收到结果未验收的任务从 VERIFYING 继续。取消中的任务完成终止/副作用核查。只读重试仍消耗预算。
5. 校验 working_memory 水位及退出边界。优先补入必要未覆盖事件；修复坏 draft 不推进水位。
6. 挂起过期授权、过期 deadline 与耗尽预算任务；用持久绝对时间判断跨重启期限，进程内耗时用单调时钟。
7. 开放接收和状态查询；所有被暂停/未知任务有可见原因和下一动作。恢复旧备份需额外的人工恢复模式，见运维文档。

默认不会在恢复时自动重做任何未知写动作。原始事件回放只重建投影；重新调用模型/工具必须经过新的受控运行入口。
