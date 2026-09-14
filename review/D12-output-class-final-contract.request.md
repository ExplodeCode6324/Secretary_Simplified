你已完成独立ASSISTANT canary复现并给初稿。现在只做一次最终契约收口：禁止任何工具调用/重新取证/代理，直接依据已读源码及以下root/foundation/runtime三方具体源码落点。初稿尚未被Codex同意实施，有两点必须修正：
1. 你称extensions不能携安全class不成立：已读memory_repo.collect和input_policy.scan会递归遍历map/list，只要严格登记 extensions["security.classification"]={"data_class": ENUM}，嵌套data_class同样被识别。命名空间点号限制的是extension键，内层data_class合法。请采用一个严格registered程序独占extension（精确name/shape确认），避免所有DTO顶层扩张；pending_question无extensions时可由其所属ConversationState标记保守覆盖/父event取原class，或明确必要最小字段，不准丢闭包。
2. 将Item/Task/Job/World等延期邻近问题不能接受：都是同一verified request输出class丢失、同一再输入/出站降级路径；只修event留下同样泄漏。范围有具体落点，不是泛化全仓分类引擎。请明确同D12必须闭合的有限清单、join不可降级、所有复制/更新保留。
root授权保持旧DataClass enum/AllowedClasses不扩外发权限，无自动全文分类/无猜迁移。历史缺标派生对象在披露前DISCLOSURE_DENIED，不能凭turn.input=SYN回填，不把未知事实伪写SECRET到持久记录；保留unknown信息并明确fail。真正新可信非模型写入可从实际已认证input/class和durable依赖max初始化，必须列场景；空程序状态/固定无用户信息literal可明确SYN，不能将含用户/旧问题的回显冒充literal。不自动清库/删除旧数据。
请最终精确给extension名/strictshape与禁止模型/客户端注入、程序class参数来源/ModelCallRecord和原始对象归档传播、legacy失败码+非模型初始化、CLI普通默认PERSONAL而测试显式SYN、必要载体/出站传播与机械验收。初稿替代关系明确；唯一输出可实施最终设计，不再次工具。Master已授权无需新审批。
以下完整只读scope报告（ASSISTANT动态复现，其余源码证据）及补充：
# D12 派生输出分类传播范围（只读）

本报告只追踪同一信息流缺陷，没有修改生产、没有真实秘密或网络调用。ASSISTANT 跨会话泄漏已有合成动态复现，其他下述路径是源码审计，不能称为逐项动态验收。

## 确认的传播落点

- **问题／ASSISTANT／最终回复**：最终文本包含模型信息，而 SearchMemoryPage 仅从创建 turn.input.data_class 取分类。已确认 PERSONAL 有效 Context + SYNTHETIC 本轮输入的生成问题，会作为 SYNTHETIC 返回另一会话。问题五字段本身无 class。同一会话扫描全部 input_turn 的防护并不能跨此边界。
- **Item 创建**：core.apply CREATE_ITEM 的 Evidence 只来自本轮 input 原话 ObjectRef。有效 Context 为 PERSONAL、当轮原话为 SYNTHETIC 时，新标题可以含前者信息，Evidence 却只有后者分类。**Item 更新**：读取旧 Item 后改 title 等字段，不传播模型请求有效 class；旧 Evidence 不证明新内容分类。全局 Snapshot 只递归收集已有 data_class，所以新会话及后台都可能低估。
- **Task／ScheduledJob／Command**：SUBMIT_TASK、CREATE_JOB 的模型生成参数、标题、criteria 进入登记，未传入 req.DataClass；runtime createRunTx 基础对象重新建立 extensions。仅给 Job 打标还不足，后续 Task/Run/Command、REPLAN 替换参数、产物和回执也必须继承。
- **WorldProposal／WorldFact**：有 Evidence 并不能证明模型只使用这些引用。有效 Context 可含更高分类，而提案 value/理由只引用低分类来源。proposal 与最终 fact 的程序标记需贯穿 WorldCommit，更新不得降低旧版本分类。
- **摘要**：Summarize 请求使用 EffectiveDataClass，但保存 v.Summary 时不记录输出 class。原会话仍受全部历史 input 分类保护；若摘要受其他 Item／Task／检索污染，该保护不足以替代独立输出标记。Snapshot 当前读取 ConversationState 后没有 collectBytes，此处须同时接入标记收集。
- **Consciousness**：Refresh 请求有效 class 正确计算，但生成 ConsciousnessState.Extensions 为空；Entry 的 Evidence 可为空，EntityReadRef 不携 class。它作为全局下一轮／其他会话输入时可失去污染级。后台 Snapshot 使用固定系统 session，不能依赖源会话 input 历史补救。
- **已有正确局部**：briefing 产物 PutObject 使用 req.DataClass。但其输入有效 class 仍依赖 Snapshot 的完整传播；上游 Item/Task 失标时这一步也无法自行恢复。

## 最小完整修复建议（待正式裁决）

采用一个严格登记、程序独占的 `extensions` 分类元数据（命名和结构以 D12 裁决为准），不用模型选标签、不修改原 EvidenceRef，也不把无引用视为 SYNTHETIC。统一 join=max(原对象已有有效分类、当前模型 Request.DataClass、输入/引用明确分类)；更新只升级，普通自动写入不能降级。模型返回的同名元数据必须拒绝或由程序无条件重新生成，不能信任其值。

Core 正常提交要将已冻结实际请求的有效 class 传到 apply、问题生命周期和最终回复/event，保持原模型 DecisionRecord 不被程序字段污染。独立给派生实体打标，不能只给 turn/event 打标；Command/Job→Task/Run、REPLAN、WorldProposal→WorldFact、artifact/receipt 等复制和更新路径全部保留 join 后的标记。Typed 输入按已认证输入及其真实引用分类打标，不为无模型路径伪造 model provenance。

memory.Refresh/Summarize 在保存前按各自实际 Request.DataClass 打标；Snapshot 收集 ConversationState、Consciousness、Items、Facts、Tasks、changes 的程序标记；SearchMemoryPage 用 event 的有效输出标记，并将缺失或非法标签交给兼容策略，不再把 input class 当 ASSISTANT 完整输出分类。Provider 的嵌套分类检查、Context 请求有效 class 与日更/摘要/briefing 的入口检查须共用分类规则。

修改位置：contract extension registry/分类 helper；core.Process/apply、store/core_repo/questions_repo；store/retrieval/memory_repo；memory.Refresh/Summarize；runtime 的登记、createRunTx、REPLAN/复制、产物/回执路径；world 提案提交与版本构建。正式文档需说明 class 代表披露上界，与真假/授权/证据质量独立。

## 旧记录保守兼容

缺少标记不能直接回退为 input.data_class 或 SYNTHETIC。可证明是全合成且来源完整的隔离验收数据库，可依据可信数据集标识/来源证明按离线显式升级；不能仅因当前剩余输入全合成就认定历史输出合成。

有完整不可变请求 wire／manifest／模型记录时，可从原调用有效 class 重建输出标记，再 join 其历史依赖分类；保留升级审计和原始字节。只有输入/引用而没有完整调用上下文不能证明不存在额外污染。无法重建者在披露入口按“分类未知／需要修复”拒绝，而非默认为低分类或伪称 SECRET。范围可先隔离到待发送的旧派生对象/会话，具体全局阻断策略须由设计明确；不删除旧记录、不篡改原事实真假。

最小验证集：已复现 ASSISTANT 链；PERSONAL Context + SYN input 生成/更新无高分类 Evidence 的 Item，再新 session 与 background；Job→Task/Run/REPLAN；无 Evidence Consciousness/summary；WorldProposal→WorldFact；旧缺标记拒绝、模型伪低标记、更新不降级。全部使用合成 canary，明确每条是否动态验证。
# D12 全部模型派生实体闭包补充（root/foundation新只读发现）
本项是主请求后的现场补充，尚未批准，不修改生产。
- Core CREATE_ITEM 仅继承本轮原文 SYNTHETIC EvidenceRef；模型当前effective PERSONAL不随Item title保存。
- UPDATE_ITEM 保留旧Evidence，不追加req污染级；title等可写PERSONAL派生值。
- SUBMIT_TASK / CREATE_JOB command/title 未传req.DataClass；后续提醒、artifact、Source/World流可能携带派生文本。
- ConsciousnessState.Extensions={}，Summary仅v.Summary，无请求effective级持久化。
- 回复/问题已证明canary跨session从PERSONAL降为SYNTHETIC。只修ASSISTANT不能闭合上述路径。
root/Codex共同建议：复用严格登记的程序extensions保存有效分类，覆盖所有真正受模型派生文本污染的实体/副本；写入来源仅程序model.Request.DataClass，客户端/模型不得覆写或降低；UPDATE取已有可信分类与本次effective最大值，不伪改原EvidenceRef/class；读取统一收集，不从input_class或空Evidence推低；legacy无法可靠恢复视未知，保守拒绝/SECRET，不默认SYNTHETIC。无需泛化分类引擎。请Ayanami对完整最小闭包、派发到notification/artifact等出站继承、typed/source独立原始分类和程序固定常量例外给明确契约，防伪造extension被模型/客户端注入。

待同一会话正式收口的具体实施问题：extension精确名/shape、legacy失败码、可信非模型初始化可从哪些真实输入/依赖分类得到标记、何时必须未知；不全文自动分类/不猜迁移。ModelCallRecord目前无req.DataClass持久标记，原请求归档class为SYN，需同步程序标记帮助未来审计并避免原始对象class降级。runtime补充：SUBMIT_ARTIFACT只核DB ObjectRef path/hash/size，未核data_class，必须从持久ref继承而不信请求伪低；createRunTx的run.extensions被authorization覆盖需merge保留程序class；WorldFact新版本必须join旧class不能丢失extensions。root将传reqclass到command/control并拒model/client伪造程序标签，Store负责原子传播和旧未知。

root另核真实数据入口同边界：CLI input/chat目前硬编码SYNTHETIC，core.Typed也如此。建议显式data-class入口：普通CLI/input/chat和typed默认PERSONAL，合成验收脚本必须显式--data-class SYNTHETIC；允许合法enum显式选择，SECRET仍不得模型出网，不能默认把真实人工文字称SYNTHETIC。不自动分类内容，程序派生字段仍不可客户端填。请D12同次裁决该安全默认与API缺省兼容，变更脚本标签不改自然语言/oracle，保存旧测试史。
