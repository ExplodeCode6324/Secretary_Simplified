# ConsciousnessState

24 小时认知快照。

写入／生成：P1 memory.refresh。存储：consciousness_snapshot。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/ConsciousnessState`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| revision | integer | 是 |  |
| slot | integer | 是 |  |
| snapshot_seq | integer | 是 |  |
| previous_snapshot_id | string / null | 是 | 见类型定义 |
| input_hash | string | 是 | SHA-256 十六进制摘要 |
| created_at | string | 是 | UTC RFC3339 时间 |
| focal_goals | array&lt;FocusEntry&gt; | 是 |  |
| priority_items | array&lt;FocusEntry&gt; | 是 |  |
| open_loops | array&lt;FocusEntry&gt; | 是 |  |
| important_changes | array&lt;FocusEntry&gt; | 是 |  |
| uncertainties | array&lt;string&gt; | 是 |  |
| brief_summary | string | 是 |  |
| missed_slots | integer | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

旧快照可以继续展示但 Context 校正过期引用；slot 唯一。首次 bootstrap 同样服从证据与引用校验。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。

## 实施过程中发现的缺陷（D05，2026-09-14）

经 Ayanami 与实施方商议（`review/D05-consciousness-criterion.response.md`），补充 `consciousness_slot_committed`，其 expected 严格为 `{ "slot": 非负整数 }`。Verifier 必须按精确 slot 读取持久 ConsciousnessState，执行完整 DTO 校验并确认 DTO.slot 一致；不得以 MAX(slot) 或未来槽替代。仅程序 SlotController 可生成 memory.refresh，按固定 epoch/slot 派生稳定 root/request/intent 与 operation_key=memory_refresh；P2 登记持久任务，Core 使用命令目标 slot，不能重算成另一个槽。重复/重启复用既有命令；每槽最多三次，失败重试分别延后5分钟、30分钟。时钟前跳只登记当前槽并取消未派发旧槽，回拨不生成历史槽。初次 bootstrap 同样走持久命令。验收覆盖重复、精确槽、并发同槽与重启恢复；DDL 不变。


### D12 分类传播

保存前用实际 Refresh 请求有效 class；Evidence 为空仍必须标记；旧缺标拒绝重推导。 使用 [Common.md](Common.md) 的 security.classification 契约；缺失旧派生标记返回 OUTPUT_CLASS_UNKNOWN，不自动回填。
