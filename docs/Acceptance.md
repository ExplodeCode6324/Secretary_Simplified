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
