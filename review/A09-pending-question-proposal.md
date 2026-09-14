# A09 待答问题正常准入：最小设计提案

状态：**PROPOSED，等待 root 与 Ayanami／Master 指定 DeepSeek 商议**。本文件不是已通过设计，也不构成实现或验收结论。本轮不修改正式文档、Schema、DDL 或生产源码。

## 1. 已核对的缺口与现有契约

- [Acceptance.md A09](../docs/Acceptance.md) 要求跨会话／重启取回约定、待答问题、指代及 Item 引用，摘要覆盖水位无缺口，过期 CAS 拒绝。
- [MemoryPolicy.md §4](../docs/MemoryPolicy.md) 要求待答问题包含问题标识、关联事项、生成序号和解决状态；回答使用明确 ID **或**程序验证的指代关联；无法消歧时澄清，不能凭摘要猜测。
- 现行机器 Schema 的 `ConversationState.pending_questions` 是最多 20 个严格对象，字段恰为 `id: UUID`、`text: string`、`item_id: UUID|null`、`created_sequence: integer>=1`、`resolved: boolean`，五字段全部 required，additionalProperties=false。正式字段是 **id**，不是 MemoryPolicy 文字中的 question_id；建议只明确该映射，不重命名已有字段。
- `ConversationSummaryDraft.pending_question_ids` 仅为最多 20 个 UUID。现行 Summarize 只接受已登记问题 ID、保留未解决的问题，不允许模型凭新 UUID 建问题。此边界应保持。
- `DecisionEnvelope.reply` 当前只允许 required `text` 与 `evidence`；`InputEnvelope` 没有问题回答指针，两者均 additionalProperties=false。因此不能把新状态准入藏进自由文本或未登记 extension。
- [Interfaces.md](../docs/Interfaces.md) 已有异步 `POST /v1/inputs`、回执查询及 `GET /v1/turns/{id}`；CLI 已有 `input --request-id --text` 与 session 参数。身份由本机认证入口写入，request_id 在重试间保持，202 不代表提交完成。无需另建写接口。
- 现有 Store 可持久保存 pending_questions，已有 tests 证明保存／重开／CAS 与水位保护。但该测试手工放入已存在问题，并不证明正常 Core 能创建或解决新问题。这是本提案要补的确切缺口。

## 2. 推荐最小范围

允许模型**提出待答问题内容**，只有程序能分配 ID、序号、解决状态并原子提交。回答采用**显式问题 ID + 原问题 session**；不实现任意语言指代自动解引用，不让模型声明“问题已解决”。

跨 session 的原话与问题仍可由现有 READ_MEMORY 取回；取回后的回复应提供原 session 和 question ID，客户端用 `--session <原session> --answer-to <question-id>` 恢复原会话。直接在其他 session 回答该 ID 按当前 session 范围返回 `QUESTION_NOT_FOUND_IN_SESSION`（404），不猜测迁移关系，也不泄露其他 session 是否存在此问题。此限制避免为本次最小修复新增全局问题索引、跨 session 权威拼装及分类传播逻辑。

**需商议明确**：这一“跨 session 可取回，切回原 session 后显式回答”的范围是否满足 A09 的本期解释。如果要求在任意新 session 直接回答旧问题，应另补来源 session 定位、同 principal 校验、旧问题文本的披露分类、双 session ReadSet/CAS；不可默默放宽，也不可宣称本最小方案已经覆盖。

## 3. 两个可选字段，既有对象继续合法

### 3.1 模型输出新增 `DecisionEnvelope.reply.questions?`

```json
{
  "text": "需要确认一个信息。",
  "evidence": [],
  "questions": [
    {"text": "这项工作的截止日期是什么？", "item_id": "<已有 Item UUID>"}
  ]
}
```

- questions 可省略或为 []；最多 3 个 proposal。
- 每个 proposal 仅 required `text`（1—512 字符）和 `item_id`（UUID 或 null）；additionalProperties=false。不允许 id、created_sequence、resolved、权限字段或模型选择的 question_key。
- text/item_id 完全相同的重复 proposal 拒绝；不尝试自然语言语义去重。
- item_id 非 null 时必须是当前 Context 已呈现的既有 Item，且对应 revision 在 ReadSet 中有效。最小方案不引用本 Decision 尚未创建的 Item，不新增 item_operation_key 联接。
- 只在经认证 MASTER_CLI 的正常决策成功路径登记问题；SOURCE/SYSTEM 或伪 principal 不能靠 reply.questions 获得元数据写入权。问题登记本身不等同于 Item／Task／World 变更，不赋予外部执行权限。
- `ConversationSummaryDraft` 不变：它仍只能引用已登记问题，不能创建、解决或复活问题。

### 3.2 输入新增 `InputEnvelope.answer_to_question_id?`

```json
{
  "schema_version": 1,
  "request_id": "<本次固定请求 UUID>",
  "session_id": "<原问题 session UUID>",
  "principal_id": "<由认证入口覆盖>",
  "origin": "MASTER_CLI",
  "received_at": "<程序 UTC 时间>",
  "text": "周五下午完成。",
  "attachment_refs": [],
  "data_class": "SYNTHETIC",
  "extensions": {},
  "answer_to_question_id": "<程序返回的问题 UUID>"
}
```

- 可省略；存在时必须是 UUID，显式 null 拒绝，避免空值的双重语义。
- 只接受同 principal、同 session、确实存在且 unresolved 的问题。问题所属 principal 从该 session 的 created_sequence 对应 assistant event → originating InputTurn 核对，不信客户端 supplied principal。
- 在受理前验证来源、session、ID 和状态，非法引用不先归档或落 PENDING。提交时再次检验，防受理后被其他回答先解决。
- 将此字段加入 InputEnvelope semantic hash。相同 request_id 改变 answer_to_question_id 必须幂等冲突；无字段的旧输入语义保持不变。不得在模型失败重试时移除字段并把回答重新解释为独立指令。
- “resolved=true”在本最小方案仅表示该问题获得了成功提交的显式回答，不证明答案真实性、业务事项完成或外部任务成功。这些仍受各自 action、grant、CAS 和 criterion 约束。

这些是 schema_version=1 的可选增量：**新读取器继续接受旧的无字段 payload**；旧严格读取器会拒绝新字段，不能宣称双向兼容。升级须同时部署 CLI、Core 和嵌入 Schema。新增 output Schema 字节需纳入实际预算测试，不靠提高预算掩盖超限。

## 4. 程序登记、渲染和事务

1. 在现有一次 Decision 提交事务内，先校验 ReadSet、显式回答目标和问题容量，再应用业务动作／问题元数据；整个事务仍全有或全无。
2. 新 question ID 使用 `DeriveID(namespace + principal_id + session_id + request_id + proposal_index)`，只对最终获准提交的 proposal 数组分配。重复请求返回原回执，不重新生成或按不同顺序重登记。
3. created_sequence 必须为**实际承载该问题的 assistant ConversationEvent.sequence**。由同一事务中的 append 函数分配／返回真实序号，不使用摘要 through_sequence、模型时间或未来占位序号。
4. 持久 `PendingQuestion` 使用既有完整五字段对象，初始 resolved=false。模型 proposal 与最终已登记回复是不同层：DecisionRecord 保留原 proposal；InputTurn／回执的最终 `reply.questions` 返回完整 PendingQuestion 对象。必须在接口文档明确两种形状；不对已盖程序字段的最终 reply 再套模型 proposal Schema。
5. 最终可见回复须展示每个问题的真实 text、question ID 和 session ID。若原 reply.text 未包含问题正文，程序按固定格式附加问题块；ConversationEvent.text 与实际交付文本一致，避免生成了隐藏问题、用户却只看到“需要确认”。这样现有关键词／ID检索可取回真实问题及定位信息。
6. 合法 answer 的 resolved 翻转与该轮成功提交的 InputTurn、回执、assistant event 及业务动作同事务。模型超时、校验失败、权限拒绝或普通失败回执不能顺带把问题解决。须显式区分“合法 Decision 成功提交”与用于告知失败的普通 reply fallback。
7. 问题回答的最终回复另提供程序生成 `answered_question_id`，并在 assistant 原话中固定记录“该问题已回答”及原问题 ID／正文，供后续跨 session 检索看到解决事实。这个标签只由服务器写入；模型不能自行提交它。无需新增顶层事件类型或修改 DDL。
8. 两个不同 request 同时回答同一问题：只允许一个完成 unresolved→resolved。另一个业务动作全部回滚，并进入明确失败结果 `QUESTION_ALREADY_RESOLVED`；不能在重试中绕过回答目标继续执行其他动作。同一 request 重试由既有回执返回已完成结果。
9. 修改 pending_questions 时增加 ConversationState.revision，但**不移动 through_sequence 或 summary_from_sequence**。并发 Summarize 的旧 CAS 必须失败重建；摘要的 omission 不能删除 unresolved 问题或把 resolved 改回 false。

## 5. 有界容量和历史

- 继续维持 pending_questions 最多 20 个、每轮新增最多 3 个。
- 新增前可按 created_sequence 从旧到新移除已 resolved 的槽位，原始问题／回答仍保留在 ConversationEvent、InputTurn 和请求回执中。
- 不得为了容纳新问题移除 unresolved 记录；20 个均 unresolved 时拒绝本次新增并反馈明确容量错误，不能静默丢弃或伪称已登记。
- 已被移出有界槽位的历史问题不可重新打开。相同旧 request 的回执仍可查询；新的 answer request 对不再 OPEN 的目标明确拒绝。
- 总是保留原始问题、明确回答和解析结果证据；不把摘要作为新问题授权或解决的唯一依据。

## 6. 公共 API／CLI 最小交付

- 保持 `POST /v1/inputs` 与202轮询协议，新增可选输入字段。
- CLI：`secretary input --session <S> --request-id <R> --answer-to <Q> --text <回答> [--json]`。不新增独立 resolve 命令，避免“标已解决”绕过回答原话和正常 Core 提交。
- `chat` 显示已登记问题及其 ID／session；最小实现可提示上述 input 命令回答，不新增自由文本猜ID快捷语法。
- `GET /v1/turns/{id}` 和 `GET /v1/requests/{request_id}` 返回同一稳定的已登记 question／answer 结果；重复查询不得改变状态。
- `GET /v1/items` 等其他接口、Runner 协议、能力注册、ExecutionPermit、Control 枚举不变。
- 错误分类建议沿用既有框架：无权限403；不存在或不属于当前 session 的问题404；already resolved／stale revision409；proposal形状或空问题400；容量使用明确的有界状态错误。正文不回显敏感回答。

## 7. 验收标准（批准后实现；本轮均 NOT_RUN）

1. **公共 CLI 完整路径**：真实 Core 进程与正常 input，不调用私有 typed helper 预置问题。先创建确定 Item；下一输入要求澄清其缺失日期，由真实模型输出 question proposal；202后查询得到程序 UUID、正确 Item ID、resolved=false、与实际 assistant event 一致的 created_sequence。原话和最终显示问题均保存。
2. **跨 session 取回／重启回答**：在新 session READ_MEMORY 取回问题原话、原 session 和 ID；重启 Core 后使用原 session + `--answer-to` 提交。该轮成功后 resolved=true；摘要和原话中 Item 引用正确。明确区分这条显式 ID 路径与未实现的自由指代解析。
3. **幂等和并发**：同 request 创建／回答各重试10次，跨进程重启后 ID、序号和状态不重复；同键不同回答目标冲突；两个新请求抢答同一问题仅一个成功，失败者没有 Item／Task／World 副作用。
4. **无权限与未知指针**：SOURCE／伪主体、未知ID、错session、已resolved目标以及模型伪造id/sequence/resolved均拒绝。非法公开输入不改变任何业务表／事件／回执；使用现有全表快照检查方法。
5. **事务故障**：在问题登记、回答状态翻转、assistant event、InputTurn／回执更新边界注入失败，状态与可见回复不可分裂；模型错误普通失败回复不解决目标问题。
6. **水位与容量**：问题状态变化不推进摘要水位；旧摘要CAS失败；20个unresolved不可被新问题挤掉；resolved槽位回收保留历史与旧回执，旧ID不能复活。
7. **披露／预算**：PERSONAL或SECRET会话的问题／回答遵守现有分类；SECRET不进入模型和日志；新字段确实进入最终 Schema／wire，并纳入实际字节预算与严格字段测试。
8. **证据分层**：程序身份／序号／事务等用确定性模型夹具验证；至少一条正常公共CLI问题→重启→显式回答使用真实已授权模型完成并保存原话、oracle、Manifest、raw output。两者分别报告，不用手工 PendingQuestion 夹具替代这条真实链。

## 8. 批准后拟同步文件

`docs/contracts.schema.json`、`docs/DataStructure/DecisionEnvelope.md`、`docs/DataStructure/InputEnvelope.md`、`docs/DataStructure/ConversationState.md`、`docs/MemoryPolicy.md`、`docs/Interfaces.md`、`docs/DataFlow.md`、`docs/Acceptance.md`，及 README 修改路径登记。嵌入 Schema／生成 DTO 随正式 Schema 同步，DDL 无需变化。全局设计变更编号由 root 分配，本提案不擅自占用 D 编号。

建议裁决聚焦三点：① 可选 reply.questions + answer_to_question_id 是否同意；② 原 session 显式回答的范围是否足够，或必须另补跨session直接回答；③ resolved 只表示已提交显式回答、与任务完成分离的语义是否同意。批准前不实施上述变更。
