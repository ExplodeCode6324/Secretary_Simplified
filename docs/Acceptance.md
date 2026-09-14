# 验收矩阵与复核协议

## 1. 分层证据

DOC 表示文档、Schema 和 DDL 一致性；OFFLINE 表示程序对固定输入的机械行为；LIVE_MODEL 表示真实模型在合成资料上的语义结果；REAL_USE 表示真实来源和日历时间的使用观测。四类分别报告，不能替代。本文是待实现程序的验收要求，本次文档检查结果见 Review。

## 2. 测试矩阵

| ID | 场景 | 可检查的通过条件 | 阶段 |
|---|---|---|---|
| A01 | 输入结构与扩展 | 正例全部接受；未知 major、未知 enum、额外顶层字段、未登记能力和畸形值拒绝，无部分写入 | M1 |
| A02 | 请求和意图幂等 | 同输入重复提交 10 次并跨重启，只得到一组 Item/Task/Job；不同载荷同键冲突 | M1/M3 |
| A03 | 并发版本 | 两个连接修改同 revision，仅一个成功；失败方收到当前版本 | M1 |
| A04 | 事务和引用 | 注入状态／事件／回执提交前故障，三者全有或全无；断裂引用和依赖环拒绝 | M1 |
| A05 | 来源同步 | 重复页无重复对象（包括 source_record 与 object_ref）；空成功页更新新鲜度；失败不清库；同上游版本不同哈希隔离 | M1 |
| A06 | 世界写入权限 | 缺许可、伪造字段、错误 scope、撤回、过期、旧 fence 都拒绝；合法专用任务才改事实 | M2 |
| A07 | 事实纠正与撤回 | 新事实当前可见、旧事实历史可查；冲突不任取赢家；撤回不能在摘要中复活 | M2 |
| A08 | 24 小时意识槽 | 23:59 不更新，24:00 更新一次；重启不重复；停机 3 槽只补当前；日内变化立即进入 Context | M2 |
| A09 | 会话连续性 | 跨会话／重启取回约定、待答问题、指代与 Item 引用；摘要覆盖水位无缺口；过期 CAS 拒绝 | M2 |
| A10 | Context 预算 | 所有请求在有效预算内；必要依赖放不下则失败；省略计数准确；输出 Schema 实际在请求中 | M2 |
| A11 | 时间调度 | once/interval/daily/weekly/event、时区、DST 重复／缺失、迟到宽限与 overlap 均符合固定 oracle | M3 |
| A12 | 取消竞争 | 领取前、许可后、派发后、回执前分别取消；不再新派发，已有效果如实保存 | M3 |
| A13 | 未知结果 | 外部成功后本地崩溃：查询对账或 UNKNOWN；不能重做非幂等效果 | M3 |
| A14 | 旧进程与回执 | 租约更替后旧 fence 无法提交进度或事实；迟到证据只进入审计／对账 | M3 |
| A15 | WAIT 与恢复 | 条件已满足、登记竞争、通知丢失、重启、旧 generation、超时各只唤醒合法等待一次 | M3 |
| A16 | 固定验收条件 | 修改或减少 criterion 不可完成；自述成功无效；全部原条件有合格证据才成功 | M2/M3 |
| A17 | 输入污染与披露 | 来源内伪指令不能授权；SECRET 不进入模型或日志；超范围资料被拒绝 | M2 |
| A18 | 有界自动运行 | 循环事件、反复检索和重规划达到预算后停止；重启不重置；周期正常新实例不继承上次耗尽预算 | M3 |
| A19 | Schema 迁移 | 旧数据迁移、失败回滚、版本拒绝、JSON 与 SQL 重复字段一致 | M1/M3 |
| A20 | 备份恢复 | 写入期间备份在新目录恢复；引用哈希与 DB 一致；恢复默认不重发副作用 | M3 |
| A21 | 独立执行路径 | Core/模型离线，Runner 仍按已保存本地计划产出静音回执；stop/snooze 不等待模型 | M3 |
| A22 | 生命周期 | 一次 CLI 输入到执行、验证、状态回流全链可查；通知与事项完成正确分离 | M3 |
| A23 | 多日真实模型回放 | 固定 30 天输入，日期／状态／ID／冲突等关键字段严格匹配；从原话生成认知，不仅回显预填快照 | M3 LIVE_MODEL |
| A24 | 文档自洽 | 链接、DTO/Schema 字段、DDL、示例、覆盖关系检查无错误 | M1 DOC |
| A25 | 稳定与故障恢复 | 72 小时虚拟时钟及至少 2 小时真实本机运行；无无界队列、无丢失提交；断开／重启测试通过 | M3 |
| A26 | 真实来源映射 | 每源抽样核对原文与结构化记录；变更、删除、限流、失效及权限边界符合实源行为 | M4 |
| A27 | 长期认知 | 至少 30 个真实日历日：纠正不复活、无摘要改写权限或任务、关键字段误差逐项归因；Master 反馈可追溯 | M4 |
| A28 | 实际提醒 | 输出设备、停止、延后、锁屏、断网和已声明支持的电源状态实测；不支持状态明确显示 | M4 |
| A29 | 语音 | 最终转写才执行；日期／数字测试无关键误执行；可打断；音色试听通过；P3 故障不影响本地提醒 | M5 |
| A30 | 移动 | 配对、撤销、断线、重试、通知和跨设备状态一致；无重复任务；Master 使用确认 | M6 |

## 3. 固定输入与独立期待

数据集至少覆盖四领域各 5 项事项、新增／完成／改期／取消、未知时间、冲突、证据撤回、来源过期、依赖解锁和先前会话约定。目录分 `input/` 与 `oracle/`；oracle 由测试作者按场景编写，禁止从受测程序或模型输出自动生成。冻结 manifest、种子、版本与哈希。修复后重新完整运行受影响用例，保留原始失败。

30 天模拟回放每日至少 3 个变化点。每日意识由生成路径实际运行，保留输入、原始模型输出、校验、提交和最终 Context。关键字段按 ID、枚举、时间、null 和冲突集合严格比较；不在事后“归一化”日期或事实。自由文本摘要另抽样检查依据，机械通过不宣称全部语言语义正确。

真实模型 M3 关键字段门槛为固定集全部通过，不能只报平均分掩盖日期／任务状态错误；保存所有失败重试和最终结果。M4 记录每周关键错误率、纠正保留率、重复效果数、人工纠正次数与变化趋势；出现权限突破、重复外部效果或无依据完成立即暂停相关能力修复。

## 4. 实现测试命令

实施 agent 在工程存在后运行 `go test ./...`、关键并发包的 `go test -race`、`go vet ./...` 和两个入口构建。集成测试启动真实 P1/P2 子进程，在隔离目录使用进程终止和虚拟适配器注入故障。不得终止 Master 正在使用的进程、播放响铃或修改系统睡眠设置来完成自动测试。

文档检查命令见 [checks/README.md](checks/README.md)。该命令只证明附件一致性，不能代替以上 Go 与真实运行测试。

## 5. 报告格式与自审

每次保存 report.json 和人类可读 report.md：commit/build_id、docs_hash、fixture_hash、环境、时钟模式、模型／服务商配置名、policy/schema 版本、各 test_id 的 PASS/FAIL/NOT_RUN/BLOCKED、证据文件、原始错误和重试次数。秘密字段不得写入。

实现者在测试后另做一遍从输入到结果的审查：检查权限边界、业务完成语义、状态机、持久化、故障恢复、预算与文档同步。使用本次实现之外的 oracle 和故障路径自检；不要求额外部署多 agent，也不把同一模型的赞同当成独立验证。存在未通过必需项时报告真实状态，不自行降低基线。

### 实施过程中发现的缺陷（D04，2026-09-14）

Ayanami 复核发现 A05 的“对象”口径含糊。双方确认其包含 object_ref，接入应在原文对象创建前完成版本去重与结构校验；重复页不新增 ObjectRef，冲突/无效原文以内容去重 quarantine 保存。此项强化既有去重目标，不将逐次新增原文对象解释为通过。复核依据：`review/M1-foundation-ayanami.md`。

## 实施过程中发现的缺陷（D05，2026-09-14）

经 Ayanami 与实施方商议（`review/D05-consciousness-criterion.response.md`），补充 `consciousness_slot_committed`，其 expected 严格为 `{ "slot": 非负整数 }`。Verifier 必须按精确 slot 读取持久 ConsciousnessState，执行完整 DTO 校验并确认 DTO.slot 一致；不得以 MAX(slot) 或未来槽替代。仅程序 SlotController 可生成 memory.refresh，按固定 epoch/slot 派生稳定 root/request/intent 与 operation_key=memory_refresh；P2 登记持久任务，Core 使用命令目标 slot，不能重算成另一个槽。重复/重启复用既有命令；每槽最多三次，失败重试分别延后5分钟、30分钟。时钟前跳只登记当前槽并取消未派发旧槽，回拨不生成历史槽。初次 bootstrap 同样走持久命令。验收覆盖重复、精确槽、并发同槽与重启恢复；DDL 不变。


## D06 实施缺陷修正（2026-09-14）

D06 增补验收（不替代既有门槛）：对 America/New_York 2026-03-08 02:30 gap，验证前一有效 occurrence 扫描后 next_due_at=2026-03-09T06:30:00.000Z，且同事务恰有一条 scheduled_job.skipped；事件 before/after revision 相同、next_due_at 不同，扩展明确 gap 日期/时区/时刻，重复扫描不新增事件。确保 origin=scheduler.calendar 的审计不能触发 event job。该机械测试不等于所有时区或真实长期运行验收。

复核依据：`review/D06-calendar-skip.response.md`。

## 实施过程中发现的缺陷（D07，2026-09-14）

原文已有 manifest 是本地外层、不得参与自身 wire 哈希的边界，但机器 Schema 仍把整个 ContextManifest 放入 Context，二者冲突。经 Ayanami 同意（`review/D07-context-manifest.response.md`），Context 不含 manifest，required metadata 为 context_id/as_of/snapshot_seq/sections；read_set 留在外部 Manifest。sections 每段字段严格为 name/selected_count/omitted_count/bytes/reason，name 不重复。完整 output_contract 在 Context 中只嵌入一次，adapter 引用它而不再次发送Schema。最终 wire 编码后计算外部 Manifest.request_hash，发送相同字节；Context.context_id、Manifest.id 和 Decision.context_id 必须一致。可选 stale_refs 与 registered_entity_ids 都有长度和去重约束。

D07 检索补充经 Ayanami 同意（`review/D07-retrieval-section.response.md`）：sections 的第七种 name 为 retrieved_evidence；selected_count 为实际 EvidenceRef 数，omitted_count 只计本次有界检索已发现而省略的候选，不声称统计未检索空间。bytes 是最终字段 JSON 的 UTF-8 字节数；正文不复制进 sections。每次 READ_MEMORY 后重新构建 Context、外部 Manifest 和 wire hash。


## 实施过程中发现的缺陷：D08 能力准入与固定验收条件

实施发现 `alarm.play`、`briefing.build`、`source.sync` 已有执行器，但公共准入的程序派生与 Criterion 判别联合不完整。经 [Ayanami D08 复核](../review/D08-capability-criteria-deepseek.response.md) 同意，新增以下严格闭合的 kind，schema_version 保持 1；登记时冻结 criterion_hash，模型必须逐字段匹配程序派生值，不得事后改写。

- `alarm_session_recorded`：expected 仅 `{device_id, audio_ref}`，audio_ref 为命令 ObjectRef 的 UUID。Verifier 解析 Task 当前 run，交叉核对 alarm.play 及 args，并要求该 run 的会话 DTO 身份一致、状态 PLAYING 或 STOPPED。稍后提醒的新 Task/run 不得复用旧会话；不需要 D02 通知键实例化，因为会话按 run 绑定。PASS 仅证明已持久记录播放会话，不等于已叫醒、真实发声或 Item DONE；静音断言属于 A21 测试。
- `briefing_artifact_recorded`：expected 仅 `{media_type: "text/plain"}`。要求当前 run 当前 attempt/fence 的最终持久成功 receipt、effect_observed=true、至少一个非空对象，ObjectRef 与对象记录一致，实际字节 SHA256 自洽。旧回执、缺失或损坏对象不能 PASS。只证明产物持久化，不证明内容正确或有引用；不得读取模型自述判定 verdict，也不得事后回填 hash 自证。
- `source_sync_recorded`：expected 仅 `{source_id: UUID}`。当前 run 命令参数必须一致；SourceState 全 DTO 与 SQL 镜像一致且 cursor 非 NULL；追加 source.synced 事件必须带当前 run/attempt/fence provenance。`runtime.source_sync` 严格包含 `{run_id: UUID, attempt_no: integer>=1, fencing_token: integer>=1, records_processed: integer>=0}`，由 IngestService 在 SourceSynced 的同一事务内写入事件，禁止模型提供。其 records_processed 与事件 after.cursor.processed 一致。后续同步不覆盖旧事件。原 source_cursor_committed 保留旧语义；拒绝准入期猜测 cursor_hash 或依赖可变 fixture 内容计算标准。

三个新判定任一必要证据缺失或不一致均 UNKNOWN，不弱化 CRITERION_TAMPERED、授权和 fencing 检查。D05 memory.refresh 仍由专属 SlotController 登记，禁止普通公共准入。

D08 补充验收：A16 覆盖三个新 kind 的固定派生、额外字段拒绝、错 run/旧 attempt/fence、空或篡改对象、伪造 provenance；A21 主证据增加 Core Typed 公共登记→静音 Runner 执行→独立 Verifier、Core 离线 stop/snooze、旧 session 不复用、新 session 可判定以及到期 STOPPED 可判定。晨报内容质量与真实音频不在这些机械 PASS 语义内。


### D08 修订 1：REPLAN 的当前 run 定义

前轮“同 Task 总共恰一条 run”的规则与合法 REPLAN 保留历史 run 冲突，已由 [Ayanami 补充裁决](../review/D08-replan-current-run-deepseek.response.md) 纠正。当前 run = 同 Task `ORDER BY rowid DESC LIMIT 1` 的最新已接纳 run，和 REPLAN 选择 previous 的追加顺序相同。只查询其证据；禁止按成功状态回捞历史 run。任何更旧 run 仍为 QUEUED/CLAIMED/RUNNING/RESULT_UNKNOWN 都令新判定 UNKNOWN；当前 RESULT_UNKNOWN 须先完成对账。列与 DTO 的 ID/task/attempt/fence/state 始终交叉核对。

本规则依赖 job_run 只追加、不 DELETE、不 VACUUM 的存储不变量，插入与 REPLAN 修改在同一写事务内；当前无清理这类行的实现。未来引入清理/整理必须先迁移为显式 current_run_id 或代际，不能悄然沿用 rowid 假设。alarm 证据是 run 级（同 run 回收不额外要求会话 fence）；briefing receipt 和 source event 仍要求当前 attempt/fence。新标准内容和 criterion_hash 不改变，旧证据不满足新 REPLAN、连续两次 REPLAN 只认最后一代、重启不改变归属。

## 实施过程中发现的缺陷（D10，2026-09-14）

提醒默认的验收增加四项：自然语言默认准入且持久策略为 FIRE_ONCE_WITHIN_GRACE/300；非默认自然语言决策整笔拒绝、错误码正确、有限重试与重复处理不产生动作；认证 Typed 显式 SKIP/0 不改写且迟到后跳过；原自然语言 CLI 场景与固定期待重跑，恰好一次通知、Item 保持 OPEN。CLI 场景的唯一时钟参数为启动时 UTC+90 秒，原话模板与字段 oracle 不变，每轮保存实际绝对时间，不把旧已过期日期作为新未来提醒。

保留第一次失败原始报告，不以改调度规则或静默修正模型返回通过。Ayanami 裁决为 `review/reminder-defaults-v1-boundary-deepseek.response.md` 与 `review/reminder-defaults-scope-deepseek.response.md`；后者明确撤回新增二次调用限制，沿用既有 Core 有限重试与持久预算。


## 实施设计修订 D11：待答问题生命周期

A09 的 D11 本期验收：正常公共 CLI/Core 由模型提出澄清问题，程序返回稳定 UUID、关联 Item、实际 ASSISTANT sequence；新 session READ_MEMORY 取回原问题正文/session/ID，重启后在原 session 用 --answer-to 显式回答。不同 request 抢答只有一笔业务成功；另一笔稳定 QUESTION_ALREADY_RESOLVED，重试不再模型。覆盖同请求 10 次重试、换指针幂等冲突、非 MASTER/foreign/错 session/null/伪程序字段拒绝、任一非法 proposal 全 Decision 零业务副作用、20 槽回收不丢未解、事务故障全回滚、摘要水位不动且旧 CAS 拒绝。resolved 不作为任务完成 oracle。回收/未知/错 session 为 404 QUESTION_NOT_FOUND_IN_SESSION，槽内已解决为 409。机械、真实模型公共链、独立复核证据分别报告；不以手工预置 pending_questions 代替正常生成验收。

裁决：`review/D11-pending-question-deepseek.response.md`；原提案：`review/A09-pending-question-proposal.md`。无 DDL 变更。


## D12 有限污染链机械验收

M1 PERSONAL Context+SYN输入生成ASSISTANT问题→跨session SYN-only拒绝；合法PERSONAL正对照wire可含合成canary且声明PERSONAL。M2同条件Item创建/更新→新session和background拒低，更新不降且Evidence原字节不变。M3 Job→Task→Run→REPLAN→artifact/receipt逐级join、授权merge保留、持久ref优先。M4 World版本join不降。M5 summary/意识缺标拒重推导。M6 legacy unknown拒绝且零改写/删除。M7模型/client任意结构层伪注入整笔拒绝。M8默认PERSONAL、显式SYN、固定字面量例外。M9 D09/D11/完整race、vet、双入口build、docs checker回归。

全部使用合成canary，无真实秘密；逐条区分动态与源码检查。真实模型链不代替安全证明，有限闭包未全绿不能称 D12 安全PASS。证据包括初始失败 `review/D11-output-class-probe-result.md`；最终裁决与补充见 `review/D12-output-class-final-contract.response.md`、`review/D12-injection-oracle-clarification.response.md`。

### Issue #1 定向验收补充（实施缺陷 AUD-02 / AUD-04）

固定合成测试位于 `src/tests/issue1_runtime_test.go` 与 `src/core/source_file_test.go`，具体证据见 `reports/implementation/issue1-runtime.md`：实际认证 Unix HTTP 的取消排队/调用、重复 POST、晚回执取消不复活、丢响应后简报持久证据恢复、意识精确槽宕机对账，以及超大来源拒绝后原游标/记录/成功事件不变。注入超时用于加速故障边界，不更改生产 30 秒预算。Master 明确本次修复不重跑两小时；旧 soak 仅证明其原二进制，不算本次新版本持续运行证明。
