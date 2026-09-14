> 发布副本：仅对本机路径作规范化；原始报告本地保留于 `review/private/`。问题、结论和测试结果未改动。

Ayanami DeepSeek 本轮裁决完毕。读了 review 两份文件，并按"有限相关"原则只读了 Store/Core/HTTP/Context/CLI 的实现点。代码未改，未运行任何构建或测试，因此本轮不宣称任何测试通过、无真实模型调用、无 key、无生产修改。

════════════════════════════
一、判决（回答）
════════════════════════════

总体：有条件同意（CONDITIONAL APPROVAL）。提案方向与实现点基本对齐，可实现；但有 6 条阻断项必须先修正，否则 AUTH-04/05/06/07/08 无法诚实通过。

先给"明确同意"（不需要改动的部分）：
1. additive migration 002，不动 001 字节 —— 同意。001 以 embed 常量校验（src/store/store.go:19-20、73），checksum = contract.Hash(baseline)，任何 001 字节改动都会让既有库与备份校验失配。
2. 公共入口 authority 检查 + 原 request 精确重放先于拒绝 —— 同意为契约，插入点见 C2。
3. durable accepted_seq + 独立固定 flock 单消费者、不跨 model 持 DB tx —— 同意。现状确实"无 DB tx 跨模型等待"（WriteObjects 只在提交段持 tx，src/store/objects.go:74-104；Process 的模型调用在 Read tx 之间发生）。
4. frozen terminal prefix 排除 future queued 的原文/分类/READ_MEMORY/summary，保留业务 readset 复核 —— 同意为契约，落实条件见 C5/C6/C7。
5. MASTER admission 不改认知 revision —— 同意，这是让"前轮不被持续打回"成立的根条件（见 C5）。
6. summary 覆盖不得跨被排除物理 seq —— 同意为契约，落实条件见 C8。
7. Typed 非空队列 AUTHORITY_BUSY(409) 零受理副作用；空队列在 consumer lock 内同步执行、不持久任意 callback —— 同意，且这是本轮唯一可行的最小方案，理由见 C9。
8. snapshot/history 只读 shape、分页与水位区分（session_id/revision/history_sequence/summary_through_sequence）且 legacy_sessions 一律 READ_ONLY —— 同意；分页可复用现有升序取数，见 C11。
9. 本轮仅 AUTH01-08/TUI/DOC/BUILD，结构新增只作迁移样例，不扩大历史恢复矩阵 —— 同意。

特别裁决一：frozen readset 与实际并发是否自洽？
自洽，但只在下述条件下自洽，否则不成立：认知 revision 的推进者必须收敛为"持同一 consumer lock 的 terminal ASSISTANT 提交"与"持同一锁的摘要"（C6），且 MASTER 受理不再推 revision（C5）。若沿用现值逻辑（appendConversationWithQuestionsTx 对 MASTER 也无条件 revision++ 并 CAS，src/store/core_repo.go:104-170、158-168），每次并发受理都会让已冻结 readset 立刻 CONFLICT，早先轮次被反复打回 —— 正是原 issue 2.2 禁止的行为。业务 readset 复核保留、CONFLICT 走 attempt<3 有限重试（src/core/core.go:66、203-205），因此不会无限打回。冻结 readset 的快照语义是"冻结认知 revision + 业务 readset 实时复核"，两者不冲突。

特别裁决二：HTTP 省 session 绑定后旧 request 重放的原会话 hash 兼容 —— 当前实现会破坏，必须修（C3）。
证据：身份哈希含 SessionID（inputSemanticHash 只剔除 ReceivedAt 与自动归档的同文本副本，src/store/core_repo.go:383-394）；hash 不等即 IDEMPOTENCY_CONFLICT（:28-31 与 :57-61）。旧客户端与旧 Typed 都硬编码默认会话 "00000000-0000-4000-8000-000000000001"（src/cmd/secretary/main.go:195-197、src/core/typed_http.go:37-38）。因此"省略 session → 服务端绑定新权威会话"再重放旧 request_id 时，重算 hash 与原回执不符 → 误判冲突而不是重放原回执。这正是"不能先强改旧信封破坏重放"的具体落点。

════════════════════════════
二、必要条件（必须修正，未满足不得开工/不得宣称完成）
════════════════════════════

C1（阻断）版本校验必须随 002 一起改，且不得改 001 字节
  src/store/store.go:103 现为 version!=1 即 MIGRATION_MISMATCH；src/store/backup.go:275-277 同样硬编码 ver!=1。Open 必须改为"校验已安装版本集合 + 001 checksum 仍等于 contract.Hash(baseline) + 002 已登记"，对未知版本仍拒绝、不静默迁移。既有用例 src/store/runtime_acceptance_test.go:69-72 把"version=2"当未知库，需按新语义移到 3（旧测试代码保留、不复写结论，但不得因此把本轮验收扩大成全仓重跑）。

C2（必做）authority 检查的插入点与顺序
  必须插在事务内重放分支之后、任何计数/归档/新记录之前，即 src/store/core_repo.go:57-71 之后、:72 之前；函数入口的预检查分支（:23-42）不得先行拒绝。这样保证"原 request 精确重放先于拒绝"且 409/拒绝零受理副作用。

C3（阻断）重放 hash 兼容规则（三入口统一）
  规则必须按 (principal_id, request_id) 命中后：若 hash 不匹配，则用存储 turn 的原 session 重算一次 hash；等值 → 判定精确重放，返回原回执/原 turn、原 session、原 hash，不开分支、不分配新 seq、不新建对象；仍不等 → IDEMPOTENCY_CONFLICT。推荐同时把权威会话选为现存历史 MASTER 会话（或默认常量本身），让旧信封零改动即天然 hash 兼容。绑定必须在 Validate/Decode 之前完成（src/core/http.go:34-48 的解析顺序），且"省略"与"显式 null/外来 ID"必须区分（保留 :28-33 的 raw 字段存在性检查语义）。显式外来 ID 仅在"不存在同 request_id 回执"时才是拒绝项。

C4（阻断，若有旧 pending 行）队列计数与取数必须收敛到权威会话
  src/store/core_repo.go:76 的 BACKPRESSURE 用全局 count(PENDING|PROCESSING)；:180-199 PendingTurns 按 updated_at 且跨会话；src/core/core.go:28-39 的 Step 会遍历并处理所有这些 turn。保留的旧非权威 pending 会既永久占容量、又被消费成第二认知分支。必须：计数限定权威会话；PendingTurns 仅取权威会话并按 accepted_seq 排序；Step/Process 不得消费非权威 pending（只读上报）。

C5（阻断）admission 事件不得推进认知 revision
  现 appendConversationWithQuestionsTx 对 MASTER/ASSISTANT 一律 revision++ 并 CAS。需拆出 admission 专用写入：插入 conversation_event（仅取得物理 sequence）而 UPDATE conversation_session 的 revision 不变；保留 :88 的空状态 INSERT OR IGNORE（新会话首条受理仍要建行）。ASSISTANT 与摘要路径维持现有 CAS 不动。

C6（必做）冻结 readset 自洽的锁不变量
  只有 (a) Process 全程（src/core/core.go:54-212）、(b) 提交后摘要（:194-198 → src/memory/memory.go:170）、(c) 空队列 AcceptTyped（src/store/core_repo.go:346-353）三处都持同一 consumer flock 时才自洽。契约需写明：除"持锁的 terminal ASSISTANT 提交"与"持锁摘要"外，任何路径不得推进 conversation_session.revision；consumer lock 的粒度是"整轮处理 + 摘要"（含模型等待，但不含 DB tx），否则 AUTH-04"一次一轮"没有强制力。

C7（阻断）单一 eligibility 函数必须同时作用于三处
  (i) prompt 最近事件（src/store/memory_repo.go:171-203）；(ii) 分类采集 —— 注意 :246-256 现在把该会话全部 input_turn 分类全量并入，future 未处理轮次会在此泄漏，必须按 terminal 前缀过滤；(iii) READ_MEMORY 上界 —— src/store/retrieval.go:55-60 现在用活值 MAX(rowid)，必须换成冻结边界。冻结边界（rowid/sequence/state/revision）持久化在 authority_turn。判据必须同一个函数，否则出现"内容已排除、分类/检索仍含 future"或反向泄漏。

C8（阻断）摘要窗口与水位的落点
  src/memory/memory.go:183 用 ConversationHistory(old.ThroughSequence, 40) 取窗，:230 直接把窗口末 seq 当 ThroughSequence。必须改为仅 terminal 前缀、遇首个非 terminal MASTER 事件即停；保留 src/store/memory_repo.go:339-346 的 count+CAS，但要额外断言 claimed 区间内不含被排除事件（现有 count 发现不了这种假覆盖）。摘要运行必须在 consumer lock 内，且保持"不跨模型等待持 DB tx"（现状 Summarize 未持 tx，保持）。

C9（必做）AUTHORITY_BUSY 的边界与备选否决理由
  AcceptTyped 的 apply 是进程内闭包（src/store/core_repo.go:327、352），不可序列化，因此"durable typed queue + 注册 action payload"在本轮不可实现（需另立命令 DSL，属超范围）；409 是本轮正确最小方案。条件：409 必须在重放分支后、归档/计数/回执之前返回；不写 request_receipt、不分配 accepted_seq、不产对象（WriteObjects 回滚会清理已建对象，src/store/objects.go:105-140，但检查应前置，清理只作兜底）；错误不得消耗 request_id（恢复后同 id 重发应可受理）；HTTP 等待超时必须映射为 409/503 而非 5xx 假失败；409 body 带 head turn_id 与 accepted_seq（只读），供客户端轮询。空队列但锁被摘要持有时：可取消等待 + 有界 deadline 后 409，不得伪造成功。

C10（必做）锁序与锁实现
  统一顺序：consumer flock → publish flock（src/store/objects.go:74-88）→ store.mu → BEGIN IMMEDIATE，不得引入反向路径；consumer lock 用固定 inode、O_NOFOLLOW、0600、可取消等待（复用 :35-60 的 EWOULDBLOCK 循环），文件永不删除（同 :281 不变量）。当前状态释放 flock、不自持（:108-109 注释语义）必须保持。

C11（必做）新只读端点的最小实现面
  /v1/conversation 与 /v1/conversation/history 走 transport.Reply + status() 错误映射；升序取数复用 src/store/memory_repo.go:452-456，但"exclusive before 边界、limit 1..100、has_more/next_sequence"是新增 SQL 条件，不是现有函数参数；pending_turns 上限对齐 Config.Limits.Queue；未知 request 404 沿用现有 /v1/requests/{id} 行为（src/core/http.go:60-63）；分类读取沿用 ReadClassification/ErrOutputClassUnknown 模式（memory_repo.go:154-166），legacy_sessions 一律 mode READ_ONLY，不得成为会话选择器。

C12（必做）三入口一致覆盖，且清除静默默认
  HTTP /v1/inputs（src/core/http.go:22-55）、/v1/actions 与 typed_http（src/core/typed_http.go:14-58）、直接 Store AcceptInput/AcceptTyped（src/store/core_repo.go:15、327）必须共用同一 authority 判定。typed_http.go:37-38 与 cmd/secretary/main.go:195-197 的硬编码默认必须改为"向服务端解析权威会话"或"配置显式选择"，否则硬编码 ID 本身就是绕过唯一性。--session 保留但只接受权威值，外来值按 C3 处理。

边界确认（本轮不做）：不重开 issue #1、不重跑旧 Context/Store 全套、不做全仓 race、不做月回放/A25/长期测试、不做 durable typed queue、不重写历史主键或改写旧分类、不做 SSE/WS/远程配对、不加 Issue2 之外的新功能门槛。结构新增只允许 authority_registry / authority_turn 这一组迁移样例。

════════════════════════════
三、参考资料（含链接）
════════════════════════════

输入文档（工作树路径）：
  <PROJECT>/review/issue2-original.md
  <PROJECT>/review/issue2-auth-proposal.md

被读实现（工作树路径；行号为工作树内容，非仓库以外的其它版本）：
  src/store/store.go:19-20,49-108,110-122 —— 001 embed/checksum、Init/Open 版本校验、Write
  src/store/backup.go:275-277 —— 第二个硬编码 version==1 点
  src/store/objects.go:18-61,71-140,281 —— publish flock 模式与锁序注释
  src/store/core_repo.go:15-52,53-103,104-170,171-199,200-208,209-295,296-323,325-358,360-372,374-378,383-394 —— AcceptInput/AcceptTyped、重放分支、admission 事件与 revision CAS、PendingTurns、Finish*/CheckReadSetTx、hash 组成
  src/store/memory_repo.go:150-272,339-346,452-456 —— Snapshot 采集面（含分类全量采集）、摘要 CAS、ConversationHistory
  src/store/retrieval.go:55-60 —— READ_MEMORY 活 rowid 上界
  src/store/core_authority.go:11-25 —— CheckCoreAuthorityTx（授权语义，与新增 authority 判定不同名不同义）
  src/memory/memory.go:168-240 —— Summarize 窗口与水位写入
  src/core/core.go:28-60,60-212,214-246,468-492 —— Step/Process 主消费者、readset 复核与有限重试、提交后摘要、错误码表、TypedClass
  src/core/http.go:15-63,96-130 —— /v1/inputs 解析顺序、/v1/turns、/v1/requests、/v1/actions
  src/core/typed_http.go:14-58 —— Typed 静默默认会话
  src/cmd/secretary/main.go:195-197,239,247,256 —— CLI 默认会话常量与 chat 循环
  src/store/runtime_acceptance_test.go:69-72 —— 现有"未知版本"用例语义
  src/platform/platform.go:30-36 —— 现有非阻塞 AcquireLock（非可等待版，仅作参考）

GitHub（issue 基线 commit cc48d753f65747df35d77664c2d6e3198b704469，仓库 ExplodeCode6324/Secretary_Simplified）：
  https://github.com/ExplodeCode6324/Secretary_Simplified/blob/cc48d753f65747df35d77664c2d6e3198b704469/src/store/core_repo.go
  https://github.com/ExplodeCode6324/Secretary_Simplified/blob/cc48d753f65747df35d77664c2d6e3198b704469/src/store/store.go
  https://github.com/ExplodeCode6324/Secretary_Simplified/blob/cc48d753f65747df35d77664c2d6e3198b704469/src/memory/memory.go
  https://github.com/ExplodeCode6324/Secretary_Simplified/blob/cc48d753f65747df35d77664c2d6e3198b704469/src/core/core.go

声明与限制：工作树会话快照的最近提交为 cc48d75，与 issue 基线前缀一致；本轮未再执行 git status/diff，故上述行号以工作树为准，若存在未提交改动需在实现前重取一次基线。本轮无构建、无测试、无模型调用、无密钥、无生产改动。

════════════════════════════
四、思路
════════════════════════════
1. 先判"提案能否落在现有代码形状上"，所以只挑四个必读面：迁移/版本校验、受理与重放、事件与 revision 写入、快照与摘要取数；其余（TUI/PTY、文档清单）本轮无代码前置，不读。
2. 每条同意项都找一个现有代码锚点，确认不是空中楼阁：hash 语义、flock 模式、重放分支、CAS 更新、retrieval 上界。
3. 把"看起来没问题"的两处单独算账：一是 frozen readset 与 revision 推进者的自洽（结论：靠 C5+C6 收敛才算自洽）；二是省 session 绑定后的重放 hash（结论：现有实现会误判冲突，必须做 stored-session 重算回退，并优先把权威会话选成旧默认/历史会话以零改动兼容）。
4. 对 Typed 备选方案给出否决证据（apply 是进程内闭包 → 不可持久），从而把 409 判定为"正确的最小方案"而不是"将就"。
5. 最后按"阻断 / 必做"分级，只列必须修正项，不新增 Issue2 之外的门槛；同时明示本轮未执行验证，避免把设计裁决冒充为已验收。
