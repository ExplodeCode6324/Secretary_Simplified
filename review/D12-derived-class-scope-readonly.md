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
