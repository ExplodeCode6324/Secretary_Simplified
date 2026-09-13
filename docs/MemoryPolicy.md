# 记忆组织与 Context 策略

## 1. World Model

实体按稳定 entity_id 关联，事实按 `(entity_id, predicate, fact_id)` 保存版本。不同事实可以表达不同关系；同一单值 predicate 存在不兼容的有效候选时建立 conflict_group，当前读模型展示 CONTESTED，不按最后写入时间选赢家。多值 predicate 必须在类型注册表声明。

事实准入依据分三种：MASTER_EXPLICIT、SOURCE_RULE、INFERENCE。Master 的明确偏好或指令性约定可按注册规则准入；来源规则只能修改其负责领域，例如学校接口的截止时间不能改变 Master 的个人偏好。INFERENCE 默认保持 CANDIDATE；需要证据和显式规则才能成为 ACTIVE。置信度高不是授权，也不能替代来源。

每个版本包含证据引用、有效时间、准入规则版本和替代关系。纠正生成新版本，旧版本保留；撤回使依赖事实被标记待复核，相关摘要不再作为有效依据。自生成摘要或同一来源的复制件不算独立佐证。初版不实现复杂概率图，确定性准入规则必须先覆盖测试数据。

## 2. Live World State

由 Item、Task、JobRun、SourceState 和设备 Observation 组成读取投影。它不重复存一份事项状态，避免双写分歧。快照记录 snapshot_seq、as_of 与对象版本。过期观测标为 STALE；没有观测标为 UNKNOWN；没有装入上下文标为 OMITTED，三者不可混用。

实时优先级用到期时间、Master priority、阻塞依赖和执行异常确定排序。模型可建议重点，但不能把已经完成的事项重新变成待办。日期被明确纠正后，当前投影使用新日期；旧日期仍可在历史检索中找到，不能与当前值无标记混排。

## 3. Consciousness 每 24 小时更新

首次初始化创建 bootstrap 快照。保存固定 epoch_at，槽号 `floor((now_utc - epoch_at) / 24h)`；正常情况下每个槽生成一次，UNIQUE(slot) 防重。24 小时是经过时长，不等同于每个本地日历日零点；可用展示时区标注生成时间。

P2 的内置槽控制器按持久 epoch 和当前 UTC 计算所需 slot，以固定命名空间和 epoch/slot 派生 UUID request_id/intent_id，使用 operation_key=memory_refresh 登记一次性 `memory.refresh` 命令。slot 由程序计算，不能在普通周期命令里永久保存 slot=0 后反复执行。已有任务重启后继续对账，耗尽尝试次数的同槽任务不再重新登记。配置 epoch 初始化后固定，改 epoch 需迁移既有槽标识；普通重启不能重置。

每次从权威现状、最近 24 小时变化、上次有效快照正文和未闭环任务正文构建 ConsciousnessStateInput。previous_snapshot_id 必须与 previous_snapshot.id 对应，bootstrap 二者均为空；task_refs 必须能在 tasks 中解析，不能只给模型不可解读的 ID。停机跨过多个槽时只补当前槽，并记录 missed_slots 和实际 changes_from_seq；不重放每个旧槽触发一串模型调用。时钟回拨不生成旧槽，前跳按漏槽规则处理。

产出包括 focal_goals、priority_items、open_loops、important_changes、uncertainties、brief_summary。引用保存 id/revision；每项必须有来源，身份与长期规则只能来自已准入事实。不能仅在旧摘要上反复压缩，不读取新权威数据。

快照构建在固定水位进行。提交时检查前一快照版本、槽号和输入引用仍可追溯；快照允许保存“截至该水位”的历史结果，不强求构建期间世界停止变化。后续变化在 Context 中覆盖旧状态；撤回证据必须在装配时过滤。失败保留旧快照并标记 overdue，同槽最多 3 次尝试（含首次），两次重试分别延后 5 分钟和 30 分钟；耗尽后保留告警，下一槽可再生成。结构错误不能用原始文本代替合法快照。

当天有新任务、完成、取消、日期纠正或授权撤回时不等待下一次日更：即时投影更新，并记录 snapshot_seq 之后的 delta。紧急变化进入当前 Context 和通知评估；不会强行刷新整个 Consciousness。

## 4. Conversation State

原话追加保存，session 内 sequence 连续且唯一。结构槽位保存 focus_entity_ids、pending_questions 和 commitment_item_ids。承诺内容在 Item 中有独立记录；会话摘要只引用它。助手说出承诺不会绕开任务登记。

近期窗口默认最多 20 个完整轮次，同时服从 12 KiB 字节上限。必须保留当前输入；超大输入先归档并明确返回 INPUT_TOO_LARGE 或按附件方式选择区段，不能静默截断当前指令。摘要保存覆盖起止序号及原文引用。提交摘要须 CAS 前一版本和 through_sequence，不能跳过中间原话。摘要失败保留可检索原文，Context 标注未压缩范围。

重启通过持久事件恢复待处理轮次。待答问题需包含 question_id、关联事项、生成序号与解决状态；回答通过明确 ID 或程序验证的指代关联，无法消歧时提出必要业务问题，不能凭摘要猜测。

## 5. Context 组装算法

本文借鉴 MemGPT 的分层存储、有限工作上下文和按需检索思路；本项目另行规定程序控制预算与权威写入，不直接照搬其自修改记忆机制。[MemGPT 原论文](https://arxiv.org/html/2310.08560v2)

组装顺序：

1. 读取策略、当前时间、任务完成条件、能力目录、输出 Schema；这些属于必要部分。
2. 在一个只读事务取得权威快照和会话状态。装入当前输入、相关 Item/Task 及父子和依赖闭包、冲突和未知状态。
3. 装入相关 WorldModelInput、LiveWorldStateInput、最近有效意识快照；按当前版本校正快照中的过期引用，并标记 stale_refs。
4. 装入意识水位之后与本次事项相关的变化、未读重要变化计数、近期原话和有覆盖水位的摘要。
5. 用实体引用、日期和关键词检索原文；最多 3 次检索、每页 10 条、每条 2 KiB。跨页使用固定快照游标。检索结果属于不可信资料区。
6. 计算最终序列化请求的字节与 token 预算，保存 ContextManifest，再发给模型。模型要求补充信息时通过结构化 READ_MEMORY 请求重新组装，复用根预算。

初始上限：最终请求 64 KiB；模型输入预算 32,000 token、输出预留 2,000 token、工具/协议余量 1,000 token。实际 token 上限取配置与服务商窗口约束的较小值。使用对应 tokenizer；无可靠 tokenizer 时记录 conservative_estimate 并以 UTF-8 字节数作为保守 token 上界，不能把旧实验的字节数当作真实 token 数。

建议 section 字节预算：固定规则和 Schema 24 KiB，当前输入及任务 8 KiB，当前世界 12 KiB，意识和增量 8 KiB，会话 8 KiB，检索 4 KiB；可在总上限内借用剩余额度。仅嵌入本次输出类型的传递引用闭包，不发送整个 contracts.schema.json；保留完整本地校验。必要部分放不下时返回 CONTEXT_REQUIRED_OVERFLOW，缩小本次任务范围或分页读取，禁止裁掉完成条件、授权约束和关键依赖后继续执行。

淘汰顺序：低相关检索、旧会话片段、低优先级派生摘要。每段记录 selected_count、omitted_count、reason 和数据截止水位；省略不能被解释为对象不存在。模型输出 Schema 必须实际嵌入请求或协议的结构化输出设置，并计入预算。

## 6. 遗忘与长期检验

首版不引入遗忘曲线。可调整检索排名和缓存淘汰，不能自动删除未完成任务、Master 明确偏好、冲突、授权或事实证据。后期引入遗忘策略需要独立配置版本、回放对照和误删恢复机制。

阶段 3 的多日回放验证日期纠正、冲突、摘要水位、重启和预算；必须包含从原始输入自主生成事实候选与意识快照的链路，不能全部预填正确 Consciousness。阶段 4 另行执行至少 30 个真实日历日的观测，区分机械约束、模型语义正确性与 Master 的实际使用反馈。

## 实施过程中发现的缺陷（D05，2026-09-14）

经 Ayanami 与实施方商议（`review/D05-consciousness-criterion.response.md`），补充 `consciousness_slot_committed`，其 expected 严格为 `{ "slot": 非负整数 }`。Verifier 必须按精确 slot 读取持久 ConsciousnessState，执行完整 DTO 校验并确认 DTO.slot 一致；不得以 MAX(slot) 或未来槽替代。仅程序 SlotController 可生成 memory.refresh，按固定 epoch/slot 派生稳定 root/request/intent 与 operation_key=memory_refresh；P2 登记持久任务，Core 使用命令目标 slot，不能重算成另一个槽。重复/重启复用既有命令；每槽最多三次，失败重试分别延后5分钟、30分钟。时钟前跳只登记当前槽并取消未派发旧槽，回拨不生成历史槽。初次 bootstrap 同样走持久命令。验收覆盖重复、精确槽、并发同槽与重启恢复；DDL 不变。

## 实施过程中发现的缺陷（D07，2026-09-14）

原文已有 manifest 是本地外层、不得参与自身 wire 哈希的边界，但机器 Schema 仍把整个 ContextManifest 放入 Context，二者冲突。经 Ayanami 同意（`review/D07-context-manifest.response.md`），Context 不含 manifest，required metadata 为 context_id/as_of/snapshot_seq/sections；read_set 留在外部 Manifest。sections 每段字段严格为 name/selected_count/omitted_count/bytes/reason，name 不重复。完整 output_contract 在 Context 中只嵌入一次，adapter 引用它而不再次发送Schema。最终 wire 编码后计算外部 Manifest.request_hash，发送相同字节；Context.context_id、Manifest.id 和 Decision.context_id 必须一致。可选 stale_refs 与 registered_entity_ids 都有长度和去重约束。

D07 检索补充经 Ayanami 同意（`review/D07-retrieval-section.response.md`）：sections 的第七种 name 为 retrieved_evidence；selected_count 为实际 EvidenceRef 数，omitted_count 只计本次有界检索已发现而省略的候选，不声称统计未检索空间。bytes 是最终字段 JSON 的 UTF-8 字节数；正文不复制进 sections。每次 READ_MEMORY 后重新构建 Context、外部 Manifest 和 wire hash。
