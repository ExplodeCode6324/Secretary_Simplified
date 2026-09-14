D12 最终契约收口裁决（Ayanami，只读依据已读源码与 root/foundation/runtime 落点；无新工具调用、无新取证。本回复整体替代初稿裁决中与本篇不一致的表述。）

一、两点修正的接受与定稿
1. 接受修正：初稿“extensions 不能承载体分类”不成立。memory_repo.go:44-58 的 collect 与 input_policy.go:72-98 的 scan 都是对 map/list 的递归遍历，命名空间点号限制只作用于 extension 键本身；因此 extensions["security.classification"] 的内层 data_class 同样被 collect/scan 命中。定稿统一采用“单一严格登记、程序独占 extension”，不做任何契约顶层 DTO 扩张。
2. 接受修正：Item/Task/Job/World 等不是“邻近问题”，是同一 verified request 输出 class 丢失、同一再输入/出站降级路径。本裁决给出有限必须闭合清单（见 §4），只修 event 视为未完成。
3. 初稿替代关系（明确）：
   替代 F1-F4：不再新增任何顶层字段；改为 extension（§3）。
   替代 R4/legacy 图章：不再以 SECRET 伪写/伪标，改为“保留 unknown + 明确 fail”（§6）。
   替代 C1/C2 部分：摘要/意识同样走 extension；旧摘要/旧快照缺标 → OUTPUT_CLASS_UNKNOWN 拒绝重推导。
   替代范围限定：闭合范围扩为 §4 有限清单。
   保留不变：双层闸门（Build Allowed + wire 嵌套≤declared）、DataClass 四值 enum 与 AllowedClasses 不扩权、SECRET 恒不可出网、原子边界=既有 finish 单写事务、无 DDL、不自动清库/不删旧数据、无自动全文分类与猜迁移、真实模型腿不作为 D12 安全证明、完整闭包未全绿不得称安全 PASS。
   补充新增：拒绝码增补唯一 fixed code OUTPUT_CLASS_UNKNOWN（仅 legacy 未知/非法标签用）；CLI/API 默认改 PERSONAL（§7）。

二、程序独占 extension 契约（mustfix）
- 名：security.classification（精确；匹配现有 propertyNames 模式，命名空间 security）。
- 严格形状：{"data_class": "<ENUM>"}；ENUM∈{SYNTHETIC,PERSONAL,SENSITIVE,SECRET}；additionalProperties=false，值必须为 string 且枚举内；出现其他成员或非法值即视为非法标签（读=unknown，写=拒绝）。
- 登记：在 contract 层建立唯一 registry helper（读写各一 + join + 模型/客户端输入剥离），并作为 contract.Validate 的严格子 schema（$defs 共享引用）挂到各载体现有 extensions 下；docs/checks 覆盖同步。凡契约已含 extensions 的载体一律用本 extension；核对后确无 extensions 的持久契约（实施时逐一核对，含 WorldUpdateProposal/Run/Receipt/Notification/ModelCallRecord 类）按同形态补 extensions 字段（schema 增量，仍非顶层 class 字段）。
- pending_question：不扩字段、不加 extension。其文本由所属 ConversationState 的 security.classification 保守覆盖（= max(summary、全部 question 文本生成 class)，仅升不降）；回答回显时 event 的 class = max(本轮 req.DataClass, 令状态标记)，等价保证 ≥ 被答问题父 event 原 class。闭包不丢。
- 模型/客户端注入禁止：程序在准入模型输出与客户端 payload 时，剥离/忽略其中任何 security.classification 出现；持久化时一律以程序值重写；持有非法/未知值的写入被拒绝；任何决策逻辑不读取模型/client 提供的 class。程序派生字段不可由客户端填充。

三、程序 class 参数来源与 join 规则（mustfix）
- 唯一来源：被冻结的实际模型请求有效 class（context.go:154 的 req.DataClass，Build 计算、成功尝试那一代），在事务前捕获，经 core.Process → FinishDecisionTurn/finishTurnWithQuestionsTx/apply/问题生命周期/回执传递；事务内不得重算。DecisionRecord 原始模型内容不被程序字段污染（mark 放对象/event/receipt 的 extension，不放原始决策）。
- 统一写入公式：mark = max(该对象既有可信 class、本次冻结请求 class、本次逐字复制内容的来源 class)；join 只升不降；UPDATE/REPLAN/版本推进均为 max(旧, 新)，保留 extension 不丢失。
- 复制/归档：PutObject/派生复制继承源对象持久 class，复制不得低于源；SUBMIT_ARTIFACT 从 DB 持久 ObjectRef 的 data_class 继承，不信任请求声明的更低值；createRunTx 的 authorization 写 extensions 改为 merge（保留程序 class）；WorldFact 新版本 join 旧版本 class 且不得丢 extensions。
- ModelCallRecord 与原始对象归档：ModelCallRecord 增加程序持久化的请求 class（当前缺此字段，实施审计与未来重建所需）；输入原文归档保持 input class（真实原值），请求 wire/briefing 等归档用冻结请求 class；所有这类写入程序独占。
- 非模型（typed/source）路径不伪造 model provenance：typed 用已认证 typed 输入 class；source 用 source 自身原始 class（独立原始分类，不与模型路径混用）。

四、必须与 D12 同批闭合的有限清单（mustfix）
1) 会话：ConversationEvent（ASSISTANT=有效输出 class；MASTER=生成 turn input class，因为文本=输入原文，属真实来源而非回填；非 MASTER 缺标见 §6）；ConversationState（summary+question 聚合标记）；问题生命周期（admit/answer 回显 join）；最终回复 fallback（见 §5 字面量例外）。
2) 记忆派生：ConsciousnessState（Refresh 保存前打标）；Summarize 产物（ConversationState 标记，max）；两者入口校验共用同一分类规则，旧缺标拒绝重推导。
3) 实体：Item CREATE（max(req class, 本轮 ref 分类)）与 UPDATE（max(旧, 新)，不改原 EvidenceRef 字节）；Task（SUBMIT_TASK 的 command/标题/criteria 模板）；ScheduledJob 与内嵌 Command；REPLAN 替换参数（join 升级）。
4) 运行：Run/Attempt/ExecutionReceipt 及其复制路径（createRunTx merge、产物/回执继承持久 ref class）。
5) World：WorldUpdateProposal（max(req class, 引用)）→ WorldFact 版本构建 join 旧版；来源同步事实按其 source class。
6) 出站：notification、briefing artifact、以及一切出网模型请求——在既有 Allowed/嵌套闸门下继承 join 后的标记；不新增任何披露权限。briefing 现有 PutObject(req.DataClass) 保留，但其上游 Item/Task 标记是本项前提。
7) 审计：ModelCallRecord、context manifest 与决策尝试报告的程序 class 标记（支撑审计与未来重建）。

五、非模型初始化场景枚举（mustfix；可显式 SYN 者仅限下列）
S1 输入原文归档 = input class（原值）。
S2 MASTER event = turn input class（原值）。
S3 纯程序固定字面量且不含任何用户/模型/旧问题内容：core.go:207 失败回退回复（固定模板+固定码白名单）、core_repo.go:300 AcceptTyped 固定回执 → 显式 SYNTHETIC；模板一旦未来嵌入任何内容即必须 join，回显/拼接（questions_repo.go:147 式）永不按 literal。
S4 typed/CLI 信封缺省 = PERSONAL（§7），显式 enum 可选。
S5 typed 创建的实体 = typed 输入 class（缺省即 PERSONAL），不是 SYN。
S6 source 同步产物 = source 声明 class。
S7 内容为空的程序状态（budget/epoch/cursor/调度脚手架等）无派生文本，不打标；若确需持久字面量文本，按 S3 条件。
S8 离线显式升级：仅限可证明全合成、来源完整的隔离验收库（可信数据集标识/来源证明+离线审计记录）；禁止从“当前剩余输入皆 SYN”推断历史输出为 SYN。
S9 问答回显：join（§2），永不 literal。

六、Legacy 未知语义与失败码（mustfix）
- 读：缺失或非法标签的派生对象保持 unknown（不写回、不伪写 SECRET、不落 input class、不落 SYNTHETIC），原记录字节不变、不删除、不改事实真假。
- 披露入口（模型请求嵌入、出站、摘要/意识重推导）遇 unknown → 以唯一新增 fixed code OUTPUT_CLASS_UNKNOWN 明确拒绝；DISCLOSURE_DENIED 仅保留给策略不允许与嵌套>declared。新码入 errorCode()/recordMemoryResult 白名单与 transport 固定码映射（不泄露细节）；本裁决不为旧对象生成任何自动标记。
- 可重建路径：仅当存在完整不可变调用上下文（wire/manifest/ModelCallRecord 含有效 class）时可离线重建，重建须 join 历史依赖、留审计、保留原始字节；当前旧记录普遍不可重建，按可知否面处之。

七、CLI/API 默认与显式入口（mustfix）
- 普通 CLI/input/chat 与 core.Typed 缺省从 SYNTHETIC 改为 PERSONAL；显式 --data-class 支持合法 enum（含 SECRET，但其内容永不出网模型路径）；API 缺省同 PERSONAL；不做内容级自动分类。
- 合成验收脚本必须显式 --data-class SYNTHETIC；仅改脚本标签，不改自然语言/ oracle；旧测试史与既有证据原样保存，新运行记录该参数。Interfaces.md/Operations.md 同步。


九、机械验收（mustfix，合成 canary；标注动态/只读）
M1 ASSISTANT 链（已动态复现）：修复后 SYN-only 拒绝、含 PERSONAL 时成功且 wire 含 data_class=PERSONAL、任何成功 wire 无 canary。动态必做。
M2 Item：PERSONAL Context+SYN 输入生成/更新 Item（标题含 canary）→ 新 session 与 background 低策略拒绝/无 canary；UPDATE 不降级且 EvidenceRef 字节不变。动态必做。
M3 Job→Task→Run→REPLAN→artifact/receipt：标记逐级 join；authorization merge 保留；SUBMIT_ARTIFACT 伪造低 class 被忽略（用持久 ref 值）。动态必做。
M4 WorldProposal→WorldFact：v2=max(v1, proposal)，extensions 不丢。动态必做。
M5 Summary/Consciousness：无标记来源 → 拒绝重推导（OUTPUT_CLASS_UNKNOWN）；有标记时宿主会话判定正确。动态必做。
M6 Legacy：旧缺标对象披露拒绝=OUTPUT_CLASS_UNKNOWN，记录零改写、零删除。动态必做。
M7 注入：模型在 reply/action payload 伪造 security.classification 被剥离/重写；typed/客户端伪造被忽略；非法值写入被拒。动态必做。
M8 非模型初始化：typed/CLI 缺省 PERSONAL、显式 SYN 生效、S3 字面量保持 SYN。动态必做。
M9 回归：D09 双锚、D11 生命周期、检索预算、MASTER 保护等既有测试全绿；go test -race ./...、go vet、双入口构建；改 schema/docs 后跑 docs/checks/validate_docs.py 留 report。动态必做。真实模型腿在修复后另跑，明确不构成 D12 安全证明。

八、闸门统一与原子边界（mustfix）
- 统一规则组件：registry + join + 递归 scan 供 input_policy 嵌套检查、Build 有效 class、Refresh/Summarize/briefing 入口共用；Build 对 page 事件按 registry 校验（非 MASTER 缺标/非法 → OUTPUT_CLASS_UNKNOWN；MASTER → turn input class）。
- 原子边界：event+questions+InputTurn 状态+receipt 同事务（core_repo.go:186-242 既有单写事务）；实体/任务/世界对象在各自 apply/注册事务内打标；事务内无网络/模型调用；失败全回滚，无部分打标。

十、正式 docs 路径（mustfix，同批）
docs/README.md（D12 索引行+修改文档清单+无 DDL）；docs/Security.md（class=披露上界，与真假/授权/证据质量独立；OUTPUT_CLASS_UNKNOWN；默认 PERSONAL；反注入）；docs/MemoryPolicy.md（检索输出标记与 legacy 口径）；docs/DataFlow.md（finish 事务与 join/复制不降级链）；docs/DataStructure/Common.md+INDEX.md（extension registry：名/形/独占/校验）；受影响契约文档逐项补 extension 语义（ConversationEvent/ConversationState/ConsciousnessState/Item/Task/ScheduledJob/Command/WorldUpdateProposal/WorldFact/ModelCallRecord/Notification/RuntimeRecords/ObjectRef/RequestReceipt 对应 md）；docs/Acceptance.md（D12 节：M1-M9+证据分层）；docs/Interfaces.md 与 docs/Operations.md（默认变更、legacy 操作、不自动清库）。裁决记录 review/D12-*.response.md 由 Codex 保存。

十一、明确非目标
不新增/泛化分类引擎；不做内容级自动分类；不猜迁移、不自动回填、不自动清库、不删改旧数据与 EvidenceRef；不扩 AllowedClasses/外发权限；不改 rank 顺序；无 DDL；SECRET 恒不出网；不把 unknown 写成任何具体 class。

十二、结论
设计同意（含上述修正）：以 security.classification 单一程序独占 extension 承载披露上界，按有限清单全链 join 不可降级、复制/更新保留、legacy 保留 unknown 并 OUTPUT_CLASS_UNKNOWN 明确 fail、默认 PERSONAL 与显式 SYN 的入口契约、旧测试史保存；M1-M9 全绿且 docs 同步前不得宣称 D12 安全 PASS。无新增审批需求。

参考资料（均为已读，无新取证）
src/store/memory_repo.go:42-64/:138-166；src/store/retrieval.go:26-120；src/store/core_repo.go:97-147/:186-242/:274-306；src/store/questions_repo.go:87-179；src/context/context.go:381-394/:40-45/:154；src/memory/memory.go:92/:144/:151-222；src/model/input_policy.go:67-99；src/model/model.go:55-62；src/config/config.go:91-113；src/contract/contracts.schema.json:1856-2083；src/contract/generate.py；src/core/core.go:54-208/:250-398；src/core/http.go:75-88；src/core/work.go:140-165；src/store/items.go:64-74；src/tests/core_retrieval_test.go；review/D11-output-class-probe-result.md 及隔离副本复现输出（/tmp/d12-verify.AqJtlQ）。

思路
两点修正均属实质更正：extension 递归可识别使单点承载成立，避免了全部顶层字段扩张；范围修正把“同类降级路径”收敛为有限清单+统一 join/merge 规则，并把 legacy 从“伪 SECRET”改为“保留 unknown+明确拒绝”，与“不猜迁移、不重写证据、不自动清库”的授权边界一致。
