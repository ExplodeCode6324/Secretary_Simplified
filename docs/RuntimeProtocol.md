# 实施补充协议：从初始化到闭环

本文补充主规格之间的连接点，给出初版确定的实现选择。所有目标路径仍待新建；研究代码不等于应用。

## 1. 初始化与进程内拓扑

`secretary init` 只在不存在的 state root 或空目录操作：拒绝符号链接和已有数据库；设 umask 077；创建私有目录；生成 Master principal、client ID、逻辑 session ID、客户端令牌；写 fixture 配置；执行版本 001；插入 system_state(epoch=0)、memory_cursor(version=NULL,watermark=0,exit=0) 和 Master/Secretary 两个实体。所有初始化步骤成功才写 READY 初始化标记；中断遗留目录须通过 init --recover 检查后续建，不覆盖现有资料。

精确依赖从 research 锁派生。安装时固定可执行入口，不让模型选择 Node、shell、动态模块路径。根 `package.json` 的 bin 指向编译后的 CLI 及原生锁启动入口；`secretaryd` 不允许绕过锁包装器直接启动业务循环。

v1 主/子 Agent 与模型网关位于一个协调进程中的不同运行对象；子 Agent 本身不是恶意代码隔离边界。WorkerEnvelope 经类型化进程内通道传递，其身份由 Dispatcher 对象注入，不能从模型参数还原。只有通用命令和原生文件 broker 使用 OS 隔离进程。未来将 Agent 放到独立进程时才启用一次性 worker 通道令牌；协议与去重 ID 不变。所有模型凭据仅留在网关。

启动顺序：校验配置 → native 独占锁 → SQLite/迁移/对象检查 → 增 epoch → 恢复 → 启动 UDS 控制面 → dispatcher/main/memory。READY 之前仅开放 health/诊断，不接收可能被丢弃的业务输入。

## 2. 请求、普通聊天与异步任务回复

所有 POST 使用 `Authorization: Bearer <local-token>`、`X-Secretary-Client-ID: UUID`、`Idempotency-Key: UUID`。body 中 request_id 必须等于后者；InputRequest.client_id 还须匹配 header。所谓 RequestEnvelope 是服务端持久元数据 `(principal,client_id,request_id,route,payload_hash)`，不是嵌套 JSON。当前单 Master 的 principal 从固定配置推导；同一个 request_id 的 route 或 body 不同均冲突。

API 入站只等待 TX1，统一返回 202 Acceptance，控制动作也如此；请求查询返回它的确定处理结果。快速动作可在同一事件循环立即提交，但不把 ACCEPTED 改称执行成功。认证/结构失败不创建接收记录。control/pause、control/resume、control/stop 使用 LifecycleCommand，路径和 action 不一致拒绝。reply ACK body 使用 `{request_id,client_id}`；处理 ACK 不递增输入代际。

`GET /v1/requests/:id` 返回 `{request_id,state,accepted_seq,task_ids,reply_ids,error}`，其中 error 为 Error 或 null。列表统一 `{items,cut_seq,next_cursor}`，无下一页时 cursor=null。游标由服务端 HMAC 签名，绑定查询参数/切点/最后键；服务器重启不更换该签名密钥。状态列表切点不意味着保留全部旧状态快照：返回 ID 顺序稳定、状态为本次快照，客户端以 revision 检查变化；需要历史状态必须查版本/事件。

不需要子任务的普通聊天使用 `commitReply`，即 TX6 的无 task 分支：检查 input_generation 与读集合 → 保存最终正文对象 → 同事务 reply(task_id=NULL,task_revision=NULL)+REPLY_COMMITTED event+outbox，并把 request 标为 COMPLETED。不凭 agent_end 直接发送草稿。重启后查询/重发同 reply_id，不重新调用模型。

有任务时，创建任务后可以提交一条“任务已建立”的正式回复，request 仍 PROCESSING；终态验收/失败/已核查取消可各自触发结果回复，全部关联原 request。一个输入可产生多个任务；所有关联任务终态且无待决提案时 request 才 COMPLETED。PAUSED 是仍需输入/额度/核查的非终态；技术失败而无活动任务可 FAILED。

回复去重键固定为 `reply:<request_id>:<kind>:<basis>`：ACK 的 basis 为任务 ID，FINAL 为 task_id/revision/terminal event seq，普通 CHAT 为本轮持久 model_call_id；不使用正文 hash 作为回复身份。重新生成过期主轮只能给当前输入生成新答复，旧模型正文保留为未发布草稿。

旧输入的异步任务完成时仍须提交其权威终态。若主会话 input_generation 已更新，TX6 用确定性结果摘要生成该任务通知，并记录当前发布代际及原 request/task 归属；不能让旧轮模型草稿冒充最新用户答复。需要语义正文时重新排队当前主会话组合通知，任务终态不回滚等待模型。

task.verify 只提交验收提案，不直接输出模型草稿。确定性条件通过后 CommitService 可生成最小正式结果；丰富解释作为后续回复。这样模型工具执行期间不依赖本轮尚未完成的 assistant message 才能提交。

## 3. 内部命令与幂等

一次模型工具调用的内部 command key 为 `(model_call_id,toolCallId,tool_name)`，Journal 在执行前持久记录原参数 hash 和程序分配的 command ID。相同 key 相同参数返回原结果；参数变化冲突。恢复重放日志不重新运行工具。产生 task、proposal、artifact 的 ID 由该持久 command 映射分配，避免模型重试重复建档。

模型 DTO 中已有 ID 分为引用与候选：引用必须真实存在；FactProposal.id 只是候选相关标识，服务端在持久 command 中分配并返回权威 proposal_id。TaskDraft.source_request_id、TaskPatch.request_id、task.report.attempt_id 必须等于可信运行上下文，不能让模型借用其它请求/尝试。GrantCommand.grant_id 由可信 CLI 预生成以供重发，CREATE 要求不存在且 expected_revision=0；REVOKE 要求当前 revision，access/class 参数不能扩大原授权。收窄授权用同事务撤销旧 grant 并建立新 grant 的内部服务，不新增未定义的 PATCH 动作。

所有主/子工具调用都消耗原根预算 tool_calls；内部查询失败仍计次数以阻止无限循环。后台整理不暴露工具，输出单个结构化 draft。语义 verifier 同样无外部工具。

## 4. 实体、义务与产物的完整入口

`entity.resolve(name,scope,source_event_seq,selected_entity_id)`：必须引用本次可访问的来源；按 NFC 名称和 scope 查候选，不把归一化匹配当作身份证明。零候选时创建稳定 ID 与 names_ref；单候选仍须来源支持同一实体，否则返回 AMBIGUOUS；多候选返回最多 20 个候选，由后续明确来源选择。模型不得仅凭同名合并实体。创建与 ENTITY_REGISTERED 事件同事务；重复内部 command 返回同 ID。结果为 `{status:RESOLVED|AMBIGUOUS,entity_id:null|UUID,candidates:[{id,name,scope}]}`。

Fact/FactProposal 的 value 表示文字值，object_entity_id 表示关系另一端；两者不能同时非 null。两者均 null 表示未知，不等于否定。subject/object entity 必须已经 resolve；删除来源使依赖事实失效。实体 names_ref 保存原名、别名、scope、来源对象/事件和 revision，不能包含未获准的身份归并。

`obligation.propose` 只创建新要求/承诺/待决提案。`obligation.resolve` 提供 ID、expected_revision、RESOLVED/CANCELLED、basis_event_seq、evidence_ids、reason。确定性任务验收可解决其绑定承诺；取消/放宽禁止要求必须有对应 Master 控制依据，无法证明则存为 OBLIGATION 类待决提案。通过后同事务提升 revision、保存 resolution_event_seq、使依赖 Context 失效。单纯“没有再提到”不能解决义务。

`artifact.create(text,media_type)` 只创建本次 attempt 的受控文本对象，返回 `{object_id,sha256,bytes,classification}`。UTF-8 最多 16 KiB；分类由源材料和可信运行上下文向上合并，不接受模型设置更低分类。它不写用户工作目录、不需要 fs.write grant；消耗工具次数/对象配额并记来源。随后 `file.write(path,content_ref,expected_sha256)` 才是外部写入，必须有 fs.write 授权。这样写文件不依赖一个无法创建的 content_ref，也无需授予通用命令权限。

file.write 的 expected_sha256=null 表示仅新建，已存在则冲突；非 null 表示写前内容必须匹配。返回新目标版本/hash 与操作证据。文件变化无法原子对比替换时见第 7 节限制。工具生成大内容可使用已批准隔离命令，仍受 50 MiB 上限；初版没有不受限对象上传接口。

## 5. 事件分类与记忆触发

Event.kind 使用下表代码常量；领域 payload 的 DTO 或内部闭合类型在 `src/storage/event-types.ts` 定义。事件 source 是写入者，不是内容的可信度。派生整理不得把自己生成的摘要当作独立原始来源。

| 事件族 | 示例与原文 | 记忆策略 |
| --- | --- | --- |
| 用户/控制 | USER_INPUT、CONTROL_RECEIVED；Request body | 用户原文必须覆盖；控制结果由权威记录覆盖 |
| 任务/操作 | TASK_CREATED/PATCHED/STATE_CHANGED、OPERATION_STARTED、RECEIPT_ACCEPTED/QUARANTINED | 活跃任务保留指针；已闭合结果触发整理；未知状态必须保留 |
| 认知/义务 | ENTITY_REGISTERED、FACT_COMMITTED、OBLIGATION_CHANGED、PROPOSAL_DECIDED | 原始来源和新版本为必要输入；不重复制造事实 |
| 对话 | MODEL_MESSAGE_STORED、REPLY_COMMITTED | 只从主/子完整交互段整理；工具链配对后才可退出 |
| 派生与控制诊断 | MEMORY_COMMITTED、HISTORY_EXITED、MODEL_CALL_SETTLED、BUDGET_RESERVED/SETTLED、OUTBOX_DELIVERED、REPLY_ACKED、HEALTH_CHANGED | 程序白名单 no_memory_needed，不单独触发新的模型整理 |

MemoryWorker 自己的模型正文使用诊断/派生事件类别，不能落为新的用户经历。未知事件种类默认 pending，不能白名单忽略。维护事件的水位在下一次有真实内容的批次顺带越过；纯白名单尾部可确定性复制当前记忆为新版本、同事务推进匹配游标，不调用模型，不再生成 MEMORY_COMMITTED 自循环，审计采用已有批次完成记录。

固定 target_seq 后，每个 seq 恰好一条 coverage；no_memory_needed 的内部事件由程序填，模型不能新增白名单。尚未闭合的交互停住可退出前缀，活跃任务状态仍从权威表直接进入 Context。积压达到阈值时减速/暂停，不跳过这个缺口。不同任务的晚回执不会让已发布 memory 回退，必要纠正作为新事件处理。

## 6. 检查器与费用实现

criteria 的 subject/expected 字符串必须按 kind 编译，不能交给模型随意解释：FILE_HASH 为规范路径+64 位小写 SHA；FILE_EXISTS 为规范路径+字符串 `true`；EXIT_CODE 为本 attempt 的 operation UUID+十进制整数；TARGET_STATE 为可信 checker 注册名与闭合 JSON 期望值；SEMANTIC 为可检查的目标描述+评分规则。不存在的 TARGET_STATE checker 在 task.create 时拒绝。checker 读取目标也经同一授权网关，不能借验收访问任意路径。

VerificationProposal 的确定性 PASS 被忽略并重新计算；SEMANTIC 由无工具 verifier 按冻结规则生成、引用证据对象，并绑定模型/策略 hash。unverified 非空或任一必要项非 PASS 时不能 SUCCEEDED。

真实 profile 固定一个计价币种，不自动换汇；金额单位为该币种的百万分之一。每次预留按已验证最大输入/输出、计费类别及价格向上取整，原子增加 root 的 tokens_reserved/money_reserved。ModelCall 保存对应预留与结算金额，status 保证只结算一次；成本价格版本/原始 usage 在关联对象中留证。

确认零 fetch：释放本次预留；calls_used 仍计入尝试上限。已发送而 usage 未知：将完整预留转为保守实耗，usage_known=false，禁止第二次释放；后来确认 usage 时用相同 model_call_id 追加一次审计调整。provider 无法给可信费用上界而配置了金额上限，则拒绝启用；金额上限为 null 时仍计 token/次数/期限，费用显示 UNKNOWN 而非零。

## 7. 文件、网络与运行时隔离的具体实现

Node 字符串 realpath 检查与最终 open 之间存在竞态。v1 文件 broker 用新建 `native/file-broker.c`：初始化时打开已授权根目录 FD；拒绝绝对相对混用、空组件、`.`/`..`、NUL；逐段 `openat(parentfd,component,O_DIRECTORY|O_NOFOLLOW)`；最终 `openat` 同样 O_NOFOLLOW，fstat 确认普通文件、所有权/链接数策略及大小。禁止设备/FIFO/socket，拒绝 nlink>1 的输入；对象库/配置根永不列入授权根。根 FD 所代表的目录是该次授权能力，程序不重新跟随路径替换。

写入使用固定父目录 FD，临时文件 O_CREAT|O_EXCL|O_NOFOLLOW，写入/fsync 后 renameat，最后 fsync 父目录。新建采用平台支持的不覆盖 rename 语义或在独占目录锁内实现；不把普通 rename 覆盖误称新建。带 expected hash 的更新要求根目录仅由本协调器写入且所有操作共用资源锁；若目标可被不受控进程并发改写，v1 拒绝“比较并替换”，返回 RESOURCE_CONFLICT，不能保证任意宿主文件系统 CAS。外部多人编辑目录可只读，或后续提供目标系统原生版本 API。

原生 broker 也通过固定沙箱运行，只接收有限 stdin/pipe 帧（操作、规范相对路径、受控内容），不加载动态代码。对每个读取对象先记录权限、版本与分类；不能把原生 helper 的权限提升为模型可用宿主通道。

命令 profile 以 P12 为机制起点，额外对每个批准解释器验证系统依赖；P12 的 broad mach-lookup/metadata 许可不是生产模板。实际模板只列经测必要服务/路径，默认禁网络和访问 state root。路径使用 SBPL 参数绑定或专用字符串编码，不能将含引号的模型路径直接拼入策略。进程监督器为每次 attempt 建独立进程组，持久化随机 nonce、PID/起始时间；禁止脱离组/后台守护进程。运行器必须有父进程生命管道，协调器消失时终止整个组；单纯依赖主进程 SIGTERM 不够。P1/T19 必须测双重 fork、setsid、父进程 SIGKILL；无法限制/查清后代时禁用 process.exec，保留文件 broker。

HTTP broker 用可信 Node https/request transport，禁止自动代理环境与自动跳转；逐跳解析 URL、授权主机/端口与全部 A/AAAA，规范化 IPv4-mapped IPv6，拒绝不允许地址。选定允许 IP 后以固定 lookup/dial 连接，同时保留原 host 的 SNI/证书校验，并核对实际 remoteAddress；重定向最多 5 次，每跳重验。禁止用户凭据 URL、任意 headers、非 GET/HEAD 和超限压缩解包；读取上限针对解压后的正文。GET 可能被目标用来产生副作用，未经可信只读目标政策的 URL 不能仅因方法为 GET 就自动认定无副作用。

## 8. 删除与恢复台账

`DeleteCommand.expected_revision` 定义为预览时数据库最新 event seq，而非 world revision；增加 GET /v1/data/delete-plan?object_id=... 返回该 revision、引用影响、受影响备份与删除对象列表。提交时重新比较并构建影响集，变化则 409，避免确认后删到新引用的正文。

执行删除前暂停相关运行；先以私有 append/fsync 的 deletion ledger 写 PREPARED（对象 ID、删除控制 request、受影响 backup ID，不含正文），再在数据库提交删除意图/失效事件，再删除文件与缓存并提交 tombstone/deletions，最后 ledger 写 COMMITTED。任一中断恢复都继续幂等删除，不把 PREPARED 当作取消。备份恢复必须接入当前 ledger，原 state root 丢失时先使用另外保留的 ledger 副本；拿不到就保留恢复隔离状态，不能猜测无删除。

ledger 位于安装级私有 recovery 目录，与具体备份/恢复目标目录分离，并以 sequence/hash 链检查截断；备份/迁移流程一起保护它。这里不假设能从旧备份自动推导它没有包含的删除。删除自身请求正文时，剩下的只允许最小非敏感控制元数据，不能把敏感内容复制进 ledger 来规避删除。

## 9. 对应实现与回归

新增文件：`src/core/replies.ts`、`src/storage/event-types.ts`、`src/memory/entities.ts`、`src/execution/artifacts.ts`、`src/ops/deletion-ledger.ts`、`native/file-broker.c`、`native/process-supervisor.c`。已有文件依本协议补接口；初始化/无任务回复在 P2，实体/义务在 P4/P5，隔离基础在 P1/P3。

回归附属于 T02/T15/T22（无任务回复、重复内部命令、晚任务结果）、T06/T24（实体/关系）、T10/T20（解决义务、无自触发整理）、T05/T19（路径竞态与后代进程）、T13（checker 编译）、T14/T28（金额与未知 usage）、T26（删除崩溃及旧备份）。六个新工具参数正反例见 [tool-cases.json](examples/tool-cases.json)；仍只验证结构。
