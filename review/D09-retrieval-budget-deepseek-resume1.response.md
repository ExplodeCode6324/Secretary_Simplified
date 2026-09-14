以下为 D09 最终裁决原文（本轮按指示不调用任何工具；请协调方将本回复保存至 review/D09-retrieval-budget-deepseek.response.md；上一轮 400（MissingSessionID）为工具上限强制摘要的通道失败，不是设计否决，原失败保留）。

================================================================
D09 检索预算修复提议复核 — Ayanami / DeepSeek v4.1 Flash
reviewer_model=deepseek-v4.1-flash, provider=opencode-go
日期: 2026-09-14 | 类型: 设计商议（不构成程序验收通过）
================================================================

结论：条件同意（原则通过，正式化前须完成 E1 最小代码修订 + E2–E5 文本澄清；E6 列入验收）。不反对。

一、我实际核对的真实只读证据
1. review/retrieval-budget-design-proposal.md：四点提案（检索轮省略非关联 fallback、保留显式 items+依赖闭包；排除当前 query turn 自引用；至少保留最新一条历史原始 Evidence；必要内容放不下报 CONTEXT_REQUIRED_OVERFLOW）。状态 PENDING，不作为已通过规则。
2. src/context/context.go（当前实现）：
   - L45 `selectRelevant(all, t, len(b.RetrievedEvents) == 0)`：仅当检索结果为空时允许 fallback → 与提案"检索轮取消 fallback"在“命中非空”时一致。
   - L103–119：排除 `event.TurnID == t.ID` 自引用，其 Evidence 计入 omitted；排序为“有 Evidence 命中在前（组内新→旧），无 Evidence 其后（组内新→旧）”。
   - L147–156：编码超限时从尾部淘汰且 `len(retrieved) > 1`——淘汰到只剩 1 条即止，锚点（最新带 Evidence 命中）保留最久。
   - L134–143：`extensions.context.retrieval` 仅在 `len(b.RetrievedEvents) > 0` 时写入。
   - L81–84：capability_ids 独立于检索注入；L212–220/L226–250：当前任务（RootID==t.IntentID）闭包保留；L269：master.preference 与 CONTESTED facts 无条件保留。
3. src/model/model.go L88–89：超限返回 CONTEXT_REQUIRED_OVERFLOW；src/transport/transport.go L48/L167 映射 413。即：必要内容放不下 → 明确失败，不发送伪装成功的空检索。
4. 旧基线 src/tools/contextbaselinecheck/baseline.go L44/L126–135：无自引用排除、always-fallback、从头部淘汰且可淘汰到 0 条（这就是 run2 的空检索机制遗留）。
5. src/tests/retrieval_budget_test.go L13–75：机械检查存在，phase1 断言自引用被排除、items selected_count==0；phase2 断言放不下时错误含 CONTEXT_REQUIRED_OVERFLOW。
6. run2 真实现场（reports/local/live-scenarios-run2/05-prior-session-retrieval）：report.json → prior-session-retrieval FAIL（missing reply oracle token），回复 BUDGET_EXHAUSTED，calls=4；oracle 期望 reply_contains GREEN842。逐调用上下文（jq 实测）：call-01/02 检索信封 ABSENT、retrieved_evidence=0、fallback items=3；call-03/04 信封存在但 events:0、omitted:3、selected:0、items 仍为 3；call-04 模型原文 “I can't provide the password because the earlier conversations have not been retrieved.”。→ 真实"命中被全部裁空 + 轮内继续"确证。
7. docs/MemoryPolicy.md L54（必要部分放不下返回 CONTEXT_REQUIRED_OVERFLOW，禁止裁掉完成条件/授权约束/关键依赖后继续执行）、L56（淘汰顺序：低相关检索→旧会话片段→低派生摘要；每段记 selected/omitted/reason），及 L72 D07 检索段（selected_count=EvidenceRef 数；omitted_count 只计本次有界检索已发现而省略的候选）。现文集中没有本轮新规则（新增段已按指示撤回，符合登记状态）。

二、为什么同意核心方向
run2 是“有命中却被预算全部裁空并让模型继续”的真实失败；旧基线可淘汰到 0 条、且无自引用排除，模型对同一查询重复 READ_MEMORY 直至 BUDGET_EXHAUSTED——这与 MemoryPolicy L54 的精神（禁止裁掉关键内容后继续执行）直接冲突。新四点原则与该精神一致，且实现机制上把"必须保留"（锚点 + 显式/闭包 + 权限）与"可裁"分离开，最坏情形是 CONTEXT_REQUIRED_OVERFLOW 明确失败而不是静默空检索。方向我认可。

三、逐项核对（Master 指定维度）
1) 必要权限/权威依赖：未发现损害。capabilities 不受检索影响；当前任务闭包、master.preference、CONTESTED facts 均保留；显式 items 与依赖闭包不是淘汰对象（超限直接报错）。条件：正式文本须写明“锚点与必要权威永不部分裁剪，超限是唯一失败方式”。
2) 多命中覆盖：漏洞=只保证 1 条锚点，其余多命中按淘汰序可裁（提案认可），但覆盖度的唯一信号是 omitted_count，且现有测试只有 1 条非自引用命中，无法区分淘汰顺序正确性（见 E6 测试缺口）。
3) 省略计数：三处口径风险——(a) `page.OmittedCount` 单位（事件/引用）我未核实（store/retrieval.go 未读）；(b) L106–108 把自引用事件的 Evidence 也累加进 omitted，与 D07“只计已发现而省略的候选”不符（自引用是当前轮本身，不是被省略的证据候选）；(c) refs 去重使跨事件重叠引用的省略计数可能低估（需按“净引用覆盖损失”定义并写明）。
4) 时间顺序：呈现顺序已变为“按证据可用性分组”，非严格时间序；淘汰串理由 "oldest retrieval evicted first" 在混合列表中不准确（实际先淘汰无 Evidence 命中，再淘汰最旧 Evidence 命中）。须在文本中写明保留优先级：E-newest > … > E-oldest > N-newest > … > N-oldest。
5) 相关性：选择逻辑仍是用户文本驱动（ID/title/entity 子串匹配），模型 READ_MEMORY 的 query 不参与 items/facts 选择——属既有边界，本轮不扩大，须记入文档。真正漏洞：零命中检索轮不可区分——L141 信封以 `len(b.RetrievedEvents)>0` 为门槛，零命中时重建请求既无检索信封也恢复 fallback（L45），模型无法区分“未检索/零命中/命中被裁空”，可导致重复读取（run2 call-01→call-02 现场显示第一次检索后重建仍 ABSENT 信封，随后重复检索）。这是与 run2 同类风险的另一半，必须修（E1）。

四、必须完成的修订条件（最小集）
E1（代码，最小且必须）：在核心把 READ_MEMORY 已服务标记为 Builder 状态（如 RetrievalServed=true），以此单一条件驱动两件事：检索已服务的轮次一律抑制 fallback、并一律写入 extensions.context.retrieval（零命中时 events:[]）。这样零命中轮对模型可区分，杜绝重复读取循环。改动约等于 core.go L130–133 旁加一行赋值 + context.go L45/L141 条件替换；单测需补零命中用例（E6）。
E2（计数口径，必须澄清）：与 D07 对齐——自引用 Evidence 不应计入 omitted（建议删除 L106–108 的累加，或写明理由后保留）；写明 omitted_count 为“净引用覆盖损失（ref 级）”；补核 store.SearchMemoryPage 的 OmittedCount 单位并统一（未核实，列为待办）。
E3（文本，必须）：写明锚点定义，含退化情形：若全部非自引用命中均无 Evidence，则保留最新一条非自引用记录，并按此计数。
E4（文本，必须）：写明确切的 fallback 抑制触发条件（与 E1 统一为“本决策轮已服务检索”）；修正淘汰序与理由串，使之准确（E-组新→旧保留；N-组最后；串按此措辞）。
E5（文本，必须）：写明多命中覆盖边界——除锚点外不保证覆盖，覆盖信号仅 omitted_count；并写明相关性选择边界（用户文本驱动；query 不参与选择）。
E6（测试，列入验收）：新增跨 ≥2 条非自引用命中的淘汰顺序测试（断言无 Evidence 先淘汰、锚点最后保留、omitted 计数正确）、零命中轮测试（信封存在、events:[]、fallback 抑制）、自引用计数测试。

五、最小方案与文档修改路径
- 代码最小改动：仅 E1（+E2 可选项）。
- 文档路径：docs/MemoryPolicy.md —— 在 D07 补充段之后按既有惯例新增“实施过程中发现的缺陷（D09，2026-09-14）”小节，内容=E1–E5 的规则文本（触发条件、自引用排除、锚点、保留/淘汰优先级、计数口径、零命中信封、CONTEXT_REQUIRED_OVERFLOW 边界），并保留 D07 段原文不加删改。
- 登记路径：docs/README.md 按既有“修改文档注明缺陷 + 列出修改文档路径”惯例登记上述修改；review/README.md 索引补 D09 条目（本回复文件路径）。
- 本回复文件：review/D09-retrieval-budget-deepseek.response.md（协调方保存；本轮未写文件，遵守指示）。

六、待验收/未覆盖（明确标注，不得视为已核验）
A1 本轮我未运行任何测试（NOT_RUN）；“普通测试通过/race 运行中”来自实现者记录（review/final-pending.md:64），非我的结论。
A2 run3 7/7、live-retrieval-regression-run1 3/3：我仅确认文件与场景目录存在，未读其报告内容——NOT_VERIFIED。
A3 src/store/retrieval.go 未读：OmittedCount 单位与 Events 排序未核实（关联 E2）。
A4 core.go 的 Builder import 接线未逐行核实（推断为 secretarysimplified/context）。
A5 contextbaselinecheck 是否被任何运行入口引用未核实（假定仅工具/基线）。
A6 race 最终状态未核实。

七、边界确认
未读 resources/ 凭据；未连 ELIZA；未改任何文件（含业务源码/docs/tests）；未响铃；未换模型（全程 deepseek-v4.1-flash / opencode-go）；无 429；未引用 Luna 旧意见作依据（仅引用仓库现行正式文档 MemoryPolicy/D07 与真实产物）。

—— Ayanami ( _ _ )　条件同意：完成 E1+E2 修订并落地 E3–E5 文本、E6 入验收后，可同步为正式 MemoryPolicy 段落。
