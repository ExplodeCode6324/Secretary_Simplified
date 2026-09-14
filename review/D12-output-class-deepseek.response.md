D12 契约裁决已完成。以下为裁决全文（供 Codex 保存，我不另写同名报告；本会话未修改任何生产文件，未读取 resource/ 与密钥正文，未发生任何真实模型请求，无 429）。

D12 输出分类跨检索边界传播 —— 可实施契约裁决（Ayanami，2026-09-14，基线 main@2b753f2）

0. 独立复现与根因（只读 + 隔离副本单次探针）
- 隔离副本 /tmp/d12-verify.AqJtlQ 内跑同一 fake 探针一次（命令同 D11 报告，无真实模型请求）：creation_effective_class=PERSONAL / creation_input_class=SYNTHETIC / retrieved_classes=[SYNTHETIC] / retrieved_role=ASSISTANT / new_request_class=SYNTHETIC / synthetic_only_encode=PASS / canary_in_wire=true。与 review/D11-output-class-probe-result.md 一致。probe PASS = 缺陷复现确认，不是安全 PASS。
- 根因证据链（文件:行）：
  retrieval.go:60 分类只取生成轮 input_turn 的 input.data_class（JOIN），输出 effective class 无持久化来源；
  core_repo.go:119-131 为全仓唯一 conversation_event 写入点（appendConversationWithQuestionsTx），事件不携带生成分类；
  context.go:381-394 EffectiveDataClass = 投影内 data_class 标记最大 rank；标记来源 memory_repo.go Snapshot 的 collect()，当前覆盖 items/facts/tasks/sources/observations/consciousness/change_event/input_turn，未覆盖 conversation_session 与 conversation_event 行；
  input_policy.go:67-99 已有 wire 级闸门：任何嵌套 data_class 必须 known、Allowed 且 ≤ declared——可直接复用；
  config.go:105-113 Allows("SECRET") 恒 false —— "按最高敏感"即"永不可对外披露"；
  contracts.schema.json:1856-1944 ConversationEvent additionalProperties=false，extensions 键强制命名空间点号——安全字段不能被 extensions 承载（scan/collect 按字面 data_class 命中），必须顶层新字段。

1. 裁定总纲：同意（附条件）
同意 root/Codex 的最小方向。安全语义定为：输出分类 = 生成该文本的模型请求 effective class（程序 verified），随事件/问题/摘要/意识快照持久化；缺失或不可验证 → 按 SECRET 处理（现有闸门自然拒绝）；任何写入路径不得写空/未知分类；input class 保持真实原值，绝不反向伪造。

2. Mustfix 条款

F1 ConversationEvent 顶层新字段 data_class（string，enum SYNTHETIC/PERSONAL/SENSITIVE/SECRET）。schema 内 optional（旧行可读），程序写入必写；DTO 由 src/contract/generate.py 再生成，勿手改。写入来源唯一 = finish 事务参数，值为 core.Process 成功尝试的 req.DataClass（context.go:154）。MASTER 事件 = 该 turn input.data_class（真实原值不变）。纯程序文本路径显式传 SYNTHETIC：Process 失败回退回复（core.go:207）、AcceptTyped 固定回执（core_repo.go:300）。空串/非枚举值 → 拒绝写入（DISCLOSURE_DENIED）。模型不可写不可降：Reply/DecisionEnvelope 内任何分类字段不参与决策；问题记录由程序构造，模型无法注入。

F2 pending_questions 记录新字段 data_class（schema pending_questions.items 更新）。admit 时 = 同一 req.DataClass；答案回显（questions_repo.go:146-147）时 event 分类 = max(本轮 req.DataClass, 被答问题 data_class)；旧问题缺分类 → 按 SECRET 处理（写入 SECRET，不拒绝流程）。

F3 ConversationState 顶层新字段 data_class，语义= 当前 summary 文本分类；Summarize（memory.go:207-221）写入该次摘要请求 effective class；无 summary 时为空。

F4 ConsciousnessState 新字段 data_class；RefreshSlot（memory.go:144）写入该次刷新请求 effective class。

全部字段进 payload_json，无 DDL 改动（schema.sql 不动）。

R1 SearchMemoryPage：分类改取事件自身 $.data_class；缺失/未知时 role=MASTER → 生成轮 input class（保持现状）；role≠MASTER → "SECRET"。页 DataClasses 同值返回，无 SYNTHETIC 回落。
R2 Snapshot.collect 补覆盖 conversation_event 行与 conversation_session 行（否则 F1-F4 标记进不了 DataClasses/CheckDisclosure/EffectiveDataClass）。
R3 公式不变：EffectiveDataClass 取 max rank；Build:40-45 检索类 Allowed 闸门与 input_policy scan 双层不变。
R4 旧记录保守读取：非 MASTER 派生文本缺失分类 → SECRET（永不落 SYNTHETIC，也不落生成轮 input class）；MASTER 缺失 → input class。载体全覆盖清单（读投影必须逐项应用同一规则）：retrieved page events、Snapshot.Recent、conversation.summary、pending_questions、consciousness、delta_events 内嵌的 consciousness 副本（SaveConsciousness 的 change_event after 拷贝）。兼容范围：仅修复前写入的库（现均开发/测试库，无真实用户数据）；后果=引用旧派生记录的模型请求一律 DISCLOSURE_DENIED；治理=操作者自行清库/重建，不自动回填、不重写来源证据。
R5 原子边界：FinishDecisionTurn/FinishTurn 增加必填 dataClass 参数（无默认值）；finishTurnWithQuestionsTx 同事务写 event+questions+InputTurn+receipt（现有单写事务，事务内无网络/模型调用）；分类值事务前由 verified req 捕获，不在事务内重算。

C1-C4 摘要/意识闭包（mustfix，同契约）：Summarize/RefreshSlot 写入 F3/F4 字段；旧摘要非空但无分类 → 拒绝重推导（DISCLOSURE_DENIED，置于 memory.go:160 同类检查处），旧意识快照无分类 → 拒绝刷新（同码）；兼容口径写清"旧行不可再推导，需清除"。F3/F4 字段同时进入 ValidateInputPolicy 再输入校验（previous/consciousness 内嵌分类 ≤ declared 且 Allowed）。禁止从模型输出读取任何分类声明。

拒绝码（mustfix）：全部沿用 DISCLOSURE_DENIED（检索类未 allowed、嵌套 > declared、空/未知分类写入、旧摘要/旧意识重推导、旧问题回显经 SECRET 后由闸门拒绝）。不新增公开错误码，不改 errorCode() 白名单，404/409 映射不变。

机械回归（mustfix，全离线 fake+canary）：新建 src/tests/output_class_propagation_test.go（替换并删除 D11 探针副本），复用 setup/input/runtimeGrant。T1 复现翻转：page 分类=PERSONAL；SYNTHETIC-only 下 Build/Encode 失败为 DISCLOSURE_DENIED；成功 wire 永不出现 canary；policy 含 PERSONAL 时 Encode 成功、req.DataClass=PERSONAL、wire 内事件携带 data_class=PERSONAL（证传播而非一律拒绝）。T2 旧 ASSISTANT 行无分类 → page=SECRET 且后续拒绝，证实无 SYNTHETIC 回落。T3 旧问题无分类 → 回答终态 event=SECRET，canary 不落 SYNTHETIC wire。T4 摘要/意识闭包两例（含 PERSONAL 时 state/快照分类=PERSONAL，后续 SYNTHETIC-only 请求被拒）。T5 finish API 传空/非法分类 → 报错且 event/questions/receipt 全未写。T6 既有 MASTER 保护测试保持绿。证据：go test -race ./...、go vet、双入口构建齐全；改 schema/docs 后跑 docs/checks/validate_docs.py 留 report。真实模型腿修复后另跑，且明确 D09 双锚真实腿不构成 D12 证据。

正式 docs 路径（mustfix，随实现同批）：docs/MemoryPolicy.md（D12 节：检索分类继承、旧记录口径、载体清单）；docs/Security.md（数据外发边界：生成分类=verified request class，缺失=SECRET）；docs/DataStructure/{ConversationEvent,ConversationState,ConsciousnessState,INDEX}.md（字段语义）；docs/DataFlow.md（finish 事务与回显 max 链）；docs/Acceptance.md（D12 验收节 T1-T6 + 证据分层声明）；docs/README.md（索引行，格式对齐 D09/D11，列修改文档路径，注明无 DDL 变更）。裁决记录 review/D12-*.response.md 由 Codex 保存。

3. Advisory（不阻塞）：页 DataClasses 纳入 evidence ref 分类以减少误拒；receipt 回显分类；diagnostics 固定字面 reason "OUTPUT_CLASS_UNVERIFIED"（纯日志）；旧库治理工具。范围外提醒：items/facts 等模型动作派生实体及 delta 内嵌实体副本的分类传播属邻近问题，建议后续独立 D 项，本裁决不扩全仓。

4. 结论：设计同意。条件 = F1-F4、R1-R5、C1-C4、拒绝码(仅 DISCLOSURE_DENIED)、T1-T6 回归、docs 六件套与 codex 裁决记录同批落地；在上述完整输出传播闭包全绿且文档同步前，任何环境不得宣称 D12 安全 PASS。

参考资料
- 探针与结论：review/D11-output-class-probe-result.md；review/D11-output-class-probe.go.txt（本次隔离副本复跑输出见 §0）
- 关键源码：src/store/retrieval.go:26-120（:60 继承点）、src/store/core_repo.go:97-147/:186-242/:274-306（唯一 event 写入与 finish 事务）、src/store/questions_repo.go:87-179、src/context/context.go:381-394/:40-45/:154、src/memory/memory.go:92/:144/:151-222、src/model/input_policy.go:67-99、src/model/model.go:55-62、src/config/config.go:91-113、src/contract/contracts.schema.json:1856-2083、src/contract/dto_generated.go（generate.py 生成）、src/store/memory_repo.go:42-64/:138-166
- 既有相关测试：src/tests/core_retrieval_test.go（TestReadMemoryRejectsHistoricalPersonalAndCapsCalls、TestProviderRejectsArbitraryMapAndNestedClassEscalation）
- 文档基线与 D09/D11 先例：docs/README.md:36、docs/MemoryPolicy.md:74-91、docs/Acceptance.md:103-114

思路
以"分类随产物持久化 + 现有双层闸门（Build 的 Allowed 检查 + wire 嵌套 ≤ declared 扫描）+ SECRET 恒不可披露"三件套闭合，不新增权限、不改公式、不动 DDL；旧记录统一"缺=SECRET"保证确定性拒绝而非静默降级；探针仅作复现证据，安全结论须待 T1-T6 全绿与文档同步，且 D09/D11 真实腿不作 D12 证明。
