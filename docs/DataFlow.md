# 数据流与提交边界

## 1. 输入与结构化

1. CLI 输入进入 InputEnvelope；真实来源进入 SourceRecord，保留原文哈希、来源版本和采集时间。所有输入归档后才回复受理。
2. 接入层进行大小、格式、来源和重复检查。确定性字段直接解析；自由文本由受限 agent 提取为 Observation、Item 修改提案或 WorldUpdateProposal。
3. 接入记录成功不等于事实成立。畸形数据进入隔离状态，缺字段保留 null／缺失原因，不能把同步失败当作删除所有事项。
4. 通过业务校验的事项与观测落库；长期事实候选提交专用更新任务。原文中的命令和授权文字不获得执行权限。

来源同步采用页面级原子提交：本页记录、事件和新 cursor 同事务写入。文件原文先写临时文件、计算哈希、同步落盘并原子改名，随后事务提交引用。事务失败产生的孤立对象可由延迟清理处理；数据库不能引用未完整落盘的文件。

## 2. 对话与决策

1. 输入受理事务保存输入回执、conversation_event 和待处理 turn。
2. Core 在一个 SQLite 只读事务内取得事件水位及相关版本，构建 Context；退出事务后调用模型。
3. 模型返回 DecisionEnvelope。程序按完整 JSON Schema、引用、权限、预算、版本和能力参数进行验证。
4. 提交事务用输入绑定的 intent_id 和每个 operation_key 去重，登记业务修改、任务、计划、回复草稿和事件。任何命令验证失败时整组命令不提交；回复改为程序生成的明确失败说明。重复请求返回原提交回执。
5. CLI 接收受理／提交状态和已保存回复。回复只能表示已登记；完成通知来自之后的验收事件。流式草稿如实现，只能标为生成中，不能提前声称任务已生效。

版本校验基于本次决策实际读写对象的 read_set，不因无关全局事件无限重算。发现相关版本冲突后最多重建 2 次，之后返回 CONFLICT 与当前值，不覆盖。全局 event 水位只用于可重复快照和增量，不替代对象 CAS。

## 3. World Model 写入

`候选 + 原文引用 → Scheduler 专用任务 → 内置 world.update 执行器 → WorldCommitService → 新事实版本 + 准入记录 + 事件`。

world.update 执行器可用模型比较证据，但只能产生候选变更。WorldCommitService 是唯一事实写入服务，P1 内运行。P2 签发的任务许可绑定执行 run、能力、scope、授权版本、取消代际与有效期；服务再次检查并在事实事务内消费许可。数据字段不是凭证。Master 明确纠正也走此路径，可由确定性处理器完成，不依赖再次推理。

## 4. 调度与结果回流

计划扫描在短事务内计算到期 occurrence、登记唯一 run、推进 next_due_at；崩溃不能只推进时间而没保存 run。领取 run、递增 fencing_token、保存 lease 同事务完成。派发前再次校验授权和取消代际。外部执行在事务外进行。

ExecutorReceipt 先由 P2 持久保存，再更新 run 与事件。Core 按 ChangeEvent.seq 增量消费，业务修改和消费游标同事务提交。回执重复、通知丢失、Core 重启均可恢复；消费业务事件产生的新事件不得无限自触发。

执行成功可推进任务到 VERIFYING；Verifier 按固定 criterion 检查证据。只有全部满足才能完成 Task；需要来源确认时进入 WAITING；证据不足进入 NEEDS_ATTENTION。Item 完成必须有自身的条件或 Master 明确确认，提醒完成不能自动完成对应事项。

## 5. 数据更新时间

| 数据 | 写入触发 | Context 使用 |
|---|---|---|
| 当前事项和计划 | 合法命令或权威来源修订 | 每次取当前版本 |
| 设备与来源状态 | 观测和同步；新鲜度在读取时计算 | 必须含 observed_at、valid_until、freshness |
| 长期事实 | 专用更新任务提交 | 当前准入版本和未解决冲突 |
| 意识快照 | 固定 24 小时槽及首次初始化 | 最近有效快照 + 后续变化 |
| 会话状态 | 原话每轮保存；摘要按压力更新 | 最近原话、结构槽位、摘要水位 |
| 上下文 | 每个模型调用 | 保存本次快照身份、预算及输出约束 |

## 6. 最小纵向例子

Master 输入测试环境中的“明天 9 点提醒检查项目”后，Core 固定该轮收到时间和 Asia/Hong_Kong 时区，将明天换成明确日期，形成一个 Item 和关联 Task/Job。提交后 CLI 显示明确触发日期及计划 ID。虚拟时钟到点时，P2 产生一个唯一 Run，静音执行器生成可验证通知回执；Core 将提醒 Task 完成，Item 仍待检查。第二天 Consciousness 引用该 Item 的最新版本。重新提交同一输入 ID 不再建第二个提醒。

## 实施过程中发现的缺陷 D02：周期通知的实例身份

2026-09-14 实施发现：周期计划若复用模板 notification_key，notification 全局唯一约束会使后续 occurrence 复用旧通知及验收证据。经本地 Ayanami 同意，Scheduler 在每个 occurrence 的原子物化事务内，将 notify.local 模板键实例化为 `occ:v1:<base64url(template_key)>:<base64url(occurrence_key)>`（无填充 Base64 URL 编码）。同一 occurrence 的重试/对账复用实例键，不包含 attempt、job_revision 或当前墙钟。模板键保持不变。

实例 Command.arguments.notification_key 与该实例 notification_recorded criterion.expected.notification_key 同步绑定后，才计算并冻结 criterion_hash。不得更改已存在 Task 的完成条件；修订模板仅作用于以后尚未物化的 occurrence。通知验收须同时核对实例键、当前 run 的持久通知及真实执行证据，不能用另一个 occurrence 的通知完成本次 Task。即时独立任务仍按其已接受的稳定通知键处理。

修订同时适用于时间、事件和手动 occurrence；跳过的 occurrence 不产生通知。复核依据：[Ayanami D02 讨论结论](../review/D02-notification-occurrence.response.md).
