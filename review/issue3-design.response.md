静态复核完成（6 个工具轮已用尽，未运行任何测试、未改源码/文档、未调用模型或读取密钥）。以下为 Ayanami（DeepSeek）对本轮设计商议的回复，stdout 即归档件。

════════════════════════════════════════
一、对提案主体的同意
════════════════════════════════════════

同意方向，理由与源码一致：

- RES-01 病灶确认：`post()` 两阶段（model.go:307-321）把 POST 与 GET 混成一个 `sentMsg{id,turn,err}`（model.go:60-64），`Update` 只按 `*APIError` + status 4xx 判"拒绝"（model.go:440-452），与 get-turn 阶段无区分。POST 202 后 GET 的 4xx 会删 `pending`/`envelopes`/恢复文件并回填草稿 —— 与 issue 描述一致，静态成立。
- "分离提交结果与观察结果 + accepted/turnID + 终态集 + 迟到消息不得退回 + 恢复元数据不加正文 + 重建 request→turn"，全部接受，且与现有事实吻合：恢复文件确为仅 `{instance_id,request_id}`（client.go:157-203），且有测试断言恰好 2 个键（model_test.go:107-110），加字段会直接打破它 —— 不得扩文件，正确做法就是重启后用 receipt 重建（model.go:133-153 已有雏形）。
- RES-02 病灶确认：tick 刷新以空游标重取第一页并整表替换（model.go:375-377 → 470-476），`selected` 按长度夹取（476），翻页成果被覆盖 —— 静态成立。
- "不重开 #1/#2、不动 schema/backend、不新增 metadata 迁移、不强制全库同步" 接受；接受 issue 自身的边界（仅 R1-01—03、R2-01—03、DOC-R、BUILD-R）。

════════════════════════════════════════
二、必要修正（C1—C9）
════════════════════════════════════════

C1. POST 阶段不能只按 status 判"确定未受理"，必须按 envelope code 白名单判别；且"未收到响应"与"收到 4xx"是两类事实。
证据：core/http.go:64-107（所有 4xx 均发生在 AcceptInput 之前或作为其 error 返回，成功唯一出口是 202）；transport/transport.go:27-61（错误 code→status 映射：401/403/409/413/422/429 均由 code 决定）；transport.go:123-153（Call 无重试；无响应的传输错误只返回 `DEPENDENCY_UNAVAILABLE`，status=0）。故 accepted 判定 = (i) 是否收到 HTTP response？(ii) envelope code 是否在 pre-persistence 白名单？(iii) 该 request 是否已有 accepted 事实？三者缺一即 unknown。

C2. 白名单之外的 code（尤其 IDEMPOTENCY_CONFLICT）不得进"确定拒绝"分支。
理由：`IDEMPOTENCY_CONFLICT` 表示同一 request_id 曾以不同内容提交，旧键可能已被受理（store 侧 receipt 语义），属 unknown/需核对，既不能清追踪也不能回填草稿。属 issue 原文"并非所有 POST 4xx 都必然未受理"的直接实例；`REQUEST_REJECTED` 等 api() 兜底 code 同样归 unknown。

C3. client.go:79-81 的 `404 → NOT_FOUND` 覆盖会抹掉 envelope code，破坏 C1。
证据：transport 会把 `QUESTION_NOT_FOUND_IN_SESSION` 映射为 404（transport.go:40-41），而 api() 在 status==404 时无条件改写 code。修正：保留 envelope code（或 status/code 分开记录），展示层继续按 NOT_FOUND 措辞；否则 POST 阶段无法按契约分类。

C4. `accepted==true` 但 POST 响应无 `turn_id`（或为空）时当前两分支都不成立，pending 永久停在"提交中"（model.go:455-459）。契约必须要求：accepted 事实独立于 turnID 记录，无 turnID 时状态="已受理，等待后端登记轮次"，由 receipt 查询继续。

C5. sync 的 unresolved 文案会倒退已受理事实：model.go:401-403 对任意未解析 id 写"受理未确认"，若该 request 已 accepted（内存或 receipt 已确认），这是 issue 所禁的"退回未受理"。修正：`accepted[id]` 为真时写"已受理；暂时无法查询（保留原 ID，不重发）"。

C6. 终态"只显示一次"需要显式的 notified-once 标记。
现状：终态清理发生在 sync 路径（model.go:406-412）但不发任何通知；sentMsg 路径则会写 `pending=state`，可把已清理项重新塞回 pending。契约：`done[request]`（终态事实）+ `notified[request]`（只提示一次）；`done` 后任何迟到消息只允许合并 turn 数据，不得改 pending/notice、不得重挂恢复标识。

C7. 两个面板族的游标语义不同，必须分别处理（这是 RES-02 最关键的修正）。
- items（Core `/v1/items` → PublicPage）：游标绑定 query hash + change_event 快照序号，任一写操作后旧游标再取即 409 CONFLICT（store/public_api.go:39-52），排序键 (created_at,id)（:59）。即 items 链是"单快照链"，随时会整体失效。
- tasks/jobs/notifications（Runner，RuntimePage）：游标为 {table,limit,last,through,seq}（store/runtime_control.go:301-307），首个游标把 `Through=MAX(rowid)`、`Seq=MAX(seq)` 钉住（:333-338），后续页只在 `(last, through]` 内取 `limit+1`（:341）。含义：窗口内既有对象顺序不可变（rowid 单调）；新对象永不进入已有链，只会出现在"重新取的第一页"；删除会造成窗口内回填、与下一页重叠（→ 去重），不会产生空洞。
- 结论：C7 要求"刷新锚点页"在 items 族上可能直接失败（CONFLICT/INVALID_CURSOR），必须定义回退，不能当作异常吞掉。
- 待复核项（本轮未直读）：`/v1/tasks`、`/v1/notifications` 的实际路由在 `registerTypedRoutes`（core/http.go:194），须由修复者确认其属哪一族；游标族按路由实测归类后写进 DOC-R。

C8. RuntimePage 对 notification 有服务端副作用：列页即把 PENDING 改 DELIVERED 并落库（runtime_control.go:367-378）。  
影响：(a) R2-03 "刷新没有副作用请求或隐式 ack" 只能解释为"客户端不额外发写请求"；列页本身写库是既有后端语义，须写入 DOC-R/Operations，不得在测试里断言"刷新零写入"。(b) 刷新范围从"第一页"改为"选中页"会改变哪些通知被标记 DELIVERED —— 这是行为变化，必须在 SingleConversationTUI.md 与 Operations.md 注明。

C9. `post()` 的 sentMsg 需要一个可判定的阶段+事实三元组，靠 Update 端重新猜阶段是不行的。
建议形状（示意，未改源码）：
    type stage int // stageSubmit / stageObserve
    type sentMsg struct { id string; stage stage; accepted bool; turnID string; turn contract.InputTurn; err error }
`accepted` 与 `stage` 同时存在才能覆盖 C4 与"POST 成功但观察失败"；`turnID` 独立于 `turn.ID` 才能在 GET 失败时保留"已知轮次身份"（R1-01 的明确要求）。

════════════════════════════════════════
三、RES-01 可实施契约
════════════════════════════════════════

状态与字段（内存，不落盘正文）
- Model 新增：`accepted map[string]bool`、`knownTurns map[string]string`、`done map[string]bool`、`notified map[string]bool`；沿用 `pending/envelopes/turns`。
- 恢复文件不变：仍只写 instance_id + request_id（client.go:162-203），禁止正文/阶段/终态入文件。

post() 输出
1. POST 收到 2xx：accepted=true；有 turn_id → knownTurns+turn 查询（观察阶段）。
2. POST 收到 4xx：stage=submit；分类交由 Update（C1/C2）。
3. POST 无响应（超时/断线，api() 折成 DEPENDENCY_UNAVAILABLE，client.go:69-71）：stage=submit，unknown 类。
4. 观察 GET 失败（4xx/5xx/超时/404）：stage=observe，err=err（turn 可能为零值）。

Update(sentMsg) 判定顺序
- 先并入事实：accepted / knownTurns / turns（不做任何清理）。
- done[id] 为真 → 只合并 turn 数据，不改 pending/notice；结束。
- stage=observe 且 err：pending[id]="已受理；查询失败：CODE"；401/403 追加"需恢复认证；不重发、不更换 request_id"；保留 envelopes + 恢复文件；不恢复草稿；绝不发新 POST。
- stage=submit 且 err：
    · accepted[id] 已为真 → 视作重复/异常响应，保持追踪不变。
    · 是 APIError 且 code ∈ 白名单（INVALID_SCHEMA、UNAUTHENTICATED、PERMISSION_DENIED、DISCLOSURE_DENIED、AUTHORITY_SESSION_MISMATCH、OUTPUT_CLASS_UNKNOWN、INPUT_TOO_LARGE、CONTEXT_REQUIRED_OVERFLOW、UNKNOWN_CAPABILITY、UNSUPPORTED_VERSION、BACKPRESSURE、BUDGET_EXHAUSTED、AUTHORITY_BUSY、AUTHORITY_TURN_NOT_HEAD、CONFLICT、STALE_FENCE、QUESTION_ALREADY_RESOLVED、QUESTION_CAPACITY_EXCEEDED、QUESTION_ANSWER_EMPTY、NOT_FOUND(含 QUESTION_NOT_FOUND_IN_SESSION)，且 status/code 未被 C3 破坏）→ 确定拒绝分支：notice="输入被后端拒绝：CODE"（401/403 追加"需恢复认证"）；仅当 `m.input.Value()==""` 才回填 `envelopes[id].Text`；删 pending/envelopes/forgetPending。
    · 其余（含 IDEMPOTENCY_CONFLICT、REQUEST_REJECTED、5xx、无 code）→ unknown：pending[id]="提交结果未知；按原 request_id 核对，不重发"；保留文件；不回填草稿。
- stage=observe 且 err=nil：turn 为终态 → done+notified（只提示一次："请求已完成：COMMITTED/FAILED（决策提交不代表业务执行成功）" 措辞沿用 client.go:236-250），删 pending/envelopes/file；非终态 → pending[id]=state，notice 沿用"已受理…"。
- 终态集合以 contract DTO 为准（本轮只确证代码判定 COMMITTED/FAILED，client.go:238-249 与 model.go:406；其余状态码修复者须对 dto_generated.go 核对后补全，不得凭猜测扩集）。

同步/重建路径
- 已有 receipt→turn 路径保留（model.go:133-153）。accepted 事实在重启后由 receipt 的 turn_id 重建；收到 receipt 404（NOT_FOUND）不改写为"未受理"措辞也不删文件（issue 明确禁止自动变回待重发草稿），仅可追加可辨识文案（如"后端无该 request 记录"），是否清理留给用户显式操作。
- 全程：任何 tick 都不得对 pending 中任一 id 发起 POST。

════════════════════════════════════════
四、RES-02 可实施契约
════════════════════════════════════════

缓存与字段
- `type panelPage struct { parent *string; items []map[string]any; next *string }`（parent = 取该页所用 cursor，首页 nil）；Model：`pages []panelPage`、`selectedID string`、`panelEpoch int`、`panelGen int`、`lastRefreshGen int`。
- rows 永不直接赋值：rows = 依序 concat(pages) 后按 id 去重（先出现的保留，天然让"刷新过的页"胜过尾部重复项）。selectedID 有值时按 ID 回绑；消失 → selected=-1 + 明确提示（"所选对象已不在已加载范围；请重新选择"）；rows 存在时 selectedID 始终跟随可见高亮。

请求发出（openPanel 三态 first/next/refresh）
- 创建闭包时捕获：epoch、gen（递增）、kind、对 refresh 捕获锚点页 parent、对 next 捕获 tail.next。panelMsg 携带 {name, epoch, gen, kind, pageIndex, page}。
- Update 丢弃规则（R2-02 的核心）：name 不符或 epoch 不符 → 丢；append 的 parent ≠ 当前 tail.next，或 gen 非最新 → 丢；refresh 的 pageIndex 已不存在或 parent ≠ pages[i].parent，或 gen ≤ lastRefreshGen → 丢。
- panelBusy 期间的键盘导航（n/r）忽略，消除同一 epoch 内自相交。

刷新替换范围（回答你的第二问）
- 主路径：只替换"锚点页"一页的 items 与其 next，parent 不变，其余页一个字节不动。
- 锚点 = 含 selectedID 的页；selectedID 缺失时取上次出现的页；再退化为第 0 页。
- 回退路径（仅当锚点页请求以 CONFLICT/INVALID_CURSOR/CURSOR_QUERY_MISMATCH 失败，即 C7 的 items 族）：以一条命令从第 0 页连续重取到锚点页号（≤ 原页数，逐页串行，同 gen 语义），完成后整段替换并按 ID 回绑；若页数超过自设上限（建议 ≤20）或重取再失败 → 保留现有行 + 显式 stale 提示（"列表快照已变化，当前视图可能过时；按 r 重新加载"），不得静默跳回第一页。
- 每日刷新开销有界：主路径每 tick ≤1 次 50 条读取；回退仅在游标失效时发生，且以用户自身翻页深度为上限，符合"不改成每秒全库扫描"。

游标推进与"不倒退、不重复"（回答你的第一、第三问）
- 排序不可变：items 键 (created_at,id)、runtime 键 rowid 均为不可变键（public_api.go:59、runtime_control.go:341）→ 现有对象不会重排；差异只来自"插入"（items 会在链中间插入，runtime 新对象不进旧链）与"删除/状态变化"。
- 追加锚点永远只用 tail.next；刷新中间页返回的新 next 只记在 pages[i].next，不改追加锚点（允许链出现"弯折"：pages[i].next ≠ pages[i+1].parent），弯折不造成空洞（runtime 删改只会回填重叠；items 失效走回退路径）。
- 因重叠会重复：去重按 ID（先出现优先）是显示不变式；追加时若该页带来 0 个新 ID → 停止翻页并提示，避免重复拉取循环。
- 游标值视为 opaque：只做相等性比较（parent 对链），禁止解码或比较大小 —— 避免耦合服务端游标格式；"不倒退"由上述顺序规则保证，而非比较游标值。
- 不在已加载范围全量同步；不新增 backend/schema/metadata 迁移。

确认与控制（R2-03）
- `m.confirm` 的 id/revision 快照在 prepareControl 时固化（model.go:329-352），panel 刷新不触碰 confirm —— 保留为契约并加测试断言（刷新改名 revision 后 confirm 仍为原值）。
- 目标消失 → selected=-1 + 提示 + 不自动改选；prepareControl 已有越界防护（model.go:330-332）继续生效。
- 客户端刷新只发 GET；不隐含 ack/action（现有 TestIssue2TUIStructuredQuestionAndExplicitControl:161-178 已覆盖"浏览不 ack"），但按 C8 不得断言服务端零写入。

════════════════════════════════════════
五、验收映射与证据要求（未实测，全部按 NOT_RUN 计）
════════════════════════════════════════

- R1-01：假服务 POST 202 → GET 400/401/403/404。断言：POST 次数=1；pending 保留且文案含"已受理"；knownTurns 保留；envelopes 与恢复文件仍在；input 未回填；无新 POST；401/403 文案含"需恢复认证"。仅断言 notice 字符串不合格（issue 原话）。
- R1-02：GET 超时/5xx → 复连后 receipt→turn 完成；终态提示恰一次；确认终态后才删追踪与文件；含"客户端重建"变体（新 Model + loadPending）。断言全程 POST 次数=1。
- R1-03：三对照 —— (a) 确定拒绝（假服务须返回契约 code，如 DISCLOSURE_DENIED/403 或 BACKPRESSURE/429；测试侧同时断言 receipt=404，把"4xx 后未受理"固化成契约事实）；(b) POST 响应丢失 → 保留原 ID、查 receipt、不自动重发、不回填草稿；(c) 观察阶段 4xx 不得进拒绝分支。注意不要把"任意 403=拒绝"写进测试（C1/C2）。
- R2-01：两页 + 第二页选中 → 自动刷新后：所选 ID 仍在、仍选中、页数不缩、无重复、不回第一页。
- R2-02：三种交错——延迟 refresh 到达时已切面板（epoch 丢）；延迟 append 到达时其 parent 已被替换（parent 丢）；同 epoch 双 refresh 乱序（gen 丢）。断言无重复行、tail.next 只由最新响应更新。
- R2-03：刷新后 revision 变化可见；confirm 保持原 ID/rev；目标消失 → -1 + 提示 + 不自动执行。表驱动覆盖 tasks/jobs/items/notifications，并至少各模拟一种游标族错误（CONFLICT vs CURSOR_QUERY_MISMATCH）验证回退路径。
- 基线纪律：新测试先在 c84d924 上运行、保留真实 FAIL 输出，再修复后转 PASS；报告区分"本轮实际执行"与历史 PASS（issue 已要求，此处只是重申）。

════════════════════════════════════════
六、DOC-R / BUILD-R 增量
════════════════════════════════════════

DOC-R（在 issue 清单之上必加）
- SingleConversationTUI.md：提交/观察两阶段状态机；accepted/knownTurns/done 语义；unknown≠rejected；终态一次提示；恢复文件字段不变；面板游标双族（items=快照链可整链失效；runtime=rowid 窗口 + Through 钉住 + 新对象只进新首页）；锚点页刷新与回退路径；按 ID 去重与选中消失语义；链弯折；C8 通知 DELIVERED 副作用（或指向 Operations.md）。
- Interfaces.md / Operations.md：显式记录"HTTP 结构与恢复元数据无变更"；通知列页写库语义；刷新范围变化对 DELIVERED 的影响。
- 交付索引与验收矩阵：关联本单、说明 #2 原验收未覆盖这两个组合，原报告/日志原样保留。
- 逐文件影响清单（含"无影响+理由"）按 issue 提交；docs/checks 只证明文档附件一致，不替代测试证据。

BUILD-R
- `cd src && go test -race ./cli/tui -run '^TestIssue2Residual' -count=1 -timeout=30s`、`go vet ./cli/tui`；若改动落到 cli/tui 之外，按实际改动包扩范围。
- 受影响入口构建哈希（TUI 二进制）；仅客户端改动时不重建未变化 daemon（issue 原文允许）。
- 报告附实际命令、修复 commit、构建哈希；未执行项如实标注。

════════════════════════════════════════
七、结论与未验证项
════════════════════════════════════════

结论：同意提案主体；需按 C1—C9 修订后冻结，其中 C1/C2/C3/C7/C8 是"不改就会把 bug 换个形状保留"的必要项。本回复是静态设计复核：未运行任何测试，不称 PASS，不称任何验收项已通过；issue 的两项发现仍是静态结论，待修复者的短时定向用例产生真实前后结果。

本轮未直读、需修复者核对的开放项：
1. `registerTypedRoutes` 中 /v1/tasks、/v1/notifications 的实际处理函数与游标族（C7 的应用范围）。
2. `AcceptInput` 的事务边界（A03 全有或全无是否覆盖"错误前已落 receipt"这一情形；R1-03 的 receipt=404 断言即为其固化手段）。
3. contract DTO 中 InputTurn 的完整状态枚举与终态集合。
4. docs/ 各现行文件的实际措辞（DOC-R 逐文件清单以修复者核对为准）。

读取范围（供归档）：review/issue3-original.md；src/cli/tui/model.go、client.go、model_test.go；src/core/http.go；src/store/runtime_control.go(部分)、public_api.go(部分)；src/transport/transport.go。未改源码/文档，未读密钥，未调用 Secretary 模型。 ( _ _ )
