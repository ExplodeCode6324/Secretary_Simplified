复核完成。已读范围：review/A09-pending-question-proposal.md 全文 + MemoryPolicy §4 + Acceptance A09（M2）。未复审源码、未运行测试、未委派、未写入任何文件。以下为三点裁决与须修正条件。

结果 ( _ _ )

① 可选 reply.questions ＋ InputEnvelope.answer_to_question_id ＋ 程序独占 ID/序号/状态 ＋ 原子登记 —— 同意。
依据：模型只提交内容（text/item_id），id、created_sequence、resolved 全部程序独占，与「程序控制权威写入、模型不能自写状态」的既有边界一致；DeriveID 以 namespace+principal+session+request+proposal_index 决定性派生，重放稳定；created_sequence 锚定真实 assistant ConversationEvent.sequence（不用摘要水位、模型时间、占位序号），是唯一正确锚点；五字段沿用既有 PendingQuestion、resolved 初始 false、同事务全有或全无，与 MemoryPolicy §4「问题标识、关联事项、生成序号、解决状态」逐项对齐。两阶段校验（受理前拒绝非法引用、不落 PENDING；提交时复检防竞态）与「登记不等于 Item/Task/World 变更、不赋予执行权限」的收窄均正确。正式字段名保持 id、仅补 question_id 映射说明、不重命名——同意。

② 跨 session 取回原问题、必须以原 session＋显式 ID 回答、不实现任意自由指代 —— 满足 A09 本期，同意。
依据：A09 原文义务是「跨会话／重启取回约定、待答问题、指代及 Item 引用」，是取回义务而非任意会话直接作答义务；MemoryPolicy §4 回答路径为「明确 ID 或程序验证的指代关联」二选一，本期只实现「明确 ID」一支即满足，未实现一支明确宣示不覆盖即可，与「无法消歧时澄清、不能凭摘要猜测」不冲突。三项收口确认（均为提案原文收束，不算新条件）：
 (a) 正式文档、README 与验收报告一律不得出现「自由指代自动解引用」「任意 session 直接回答旧问题」类表述；
 (b) 错 session 一律 QUESTION_NOT_FOUND_IN_SESSION（404），不猜测迁移、不泄露其他 session 是否存在该问题；
 (c) 该同意的活锚点是验收 2 的检索腿——新 session 中 READ_MEMORY 必须真实取回原问题原话、原 session 与 ID，且重启后可用原 session＋--answer-to 完成解除；若该腿在实施中失败，按验收缺陷回补，本范围裁决不变。

③ resolved 仅表示成功提交显式回答、不证明答案真实性、不证明任务 DONE —— 同意。
依据：与证据分层（自述成功无效、A16 完成条件独立）及 MemoryPolicy「解决状态」的会话事实语义一致；模型不能自写 resolved、失败 fallback 不得顺带翻转、翻转与 InputTurn／回执／assistant event／业务动作同事务（提案 §4.6）——三处都把 resolved 锁死为会话状态事实而非业务真值。收口一句：任何投影或界面不得把 resolved 呈现为任务或外部动作完成。

须修正条件（文档级钉死，属已授权设计同步范围，无需再请示、不追加任何重跑门槛）：

一、proposal 校验失败的「拒绝范围」未钉死（提案 §3.1 行39；§4.2 行73）。
问题：「重复 proposal 拒绝」未写拒绝对象是单条还是整个 Decision；「只对最终获准提交的 proposal 数组分配」在误读下可被实现成「部分接受＋静默过滤」。
要求：D11 文本逐字写明——questions 数组任一 proposal 校验失败（重复、item_id 引用/revision 无效、超 512 字符、超 3 条）⇒ 整个 Decision 校验失败：零问题登记、零 Item/Task/World 副作用、不静默丢弃、不语义合并；同 request_id 重试可重跑；proposal_index 固定为提交数组内下标。
理由：与 A04「无部分写入」及提案 §5「不能静默丢弃」同一原则，防止两个实现者分叉。

二、非 MASTER_CLI（SOURCE／SYSTEM／伪主体）决策携带 reply.questions 的确定性处理未写明（提案 §3.1 行41 与 §7-4「SOURCE／伪主体…均拒绝」之间留白）。
要求：二选一钉死并写入验收 oracle；建议取「拒绝」，与验收 4 用词一致。若取「忽略」，须同步改写验收 4 对应措辞，并保证可见回复不出现登记声明与问题块。不得留白由实现者任选。

三、「不再 OPEN」回答目标的错误码未钉死（提案 §5 行87 与 §6 行97 之间）。
要求：钉死一个映射并写入 Interfaces.md 与验收 6 oracle。推荐：仍在槽位且 resolved ⇒ 409 QUESTION_ALREADY_RESOLVED；不在槽位（含已回收）或错 session／未知 ⇒ QUESTION_NOT_FOUND_IN_SESSION 404，文案按「当前会话范围内不存在待答的该问题」表述，不表述「从未存在」。
理由：回收仅针对已 resolved 槽位，若「曾存在」与「未知」自由区分，会泄露历史或产生跨实现不一致。

补充小项（建议一并钉死，均不影响三点裁决）：
 – 可见文本附加规则确定性化：建议「凡成功登记/回答，程序一律按固定格式追加问题块/answered 标记，不依赖对模型文本的子串判别」，固定格式写入 Interfaces.md，验收 1/2 以此匹配。
 – 竞态失败 request 的重试重放语义：建议已记录 QUESTION_ALREADY_RESOLVED 的 request 重试直接返回该记录结果，不重跑模型、不绕过回答目标。

其余复核点已核对、无需修改：事务全有或全无与失败 fallback 隔离（§4.1/4.6）、幂等重放与同键冲突（§4.2/4.8）、水位不移动＋revision 使旧 CAS 失败（§4.9）、20/3 容量与回收不挤占 unresolved（§5）、证据分层验收（§7.8）——与 MemoryPolicy §4 及 A09「水位无缺口、过期 CAS 拒绝」一致。

总评：三点均同意；上述 3 条须修正并入 D11 正式文本（接口文档与验收 oracle 同步），不改变方案结构与范围。无需 Master 二次批准，不追加重跑次数要求——按现有授权双方商议后同步执行。我不写任何文件，本结论全文由 Codex 保存至 review。
