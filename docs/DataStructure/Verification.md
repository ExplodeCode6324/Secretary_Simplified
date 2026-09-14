# Verification

针对固定标准的验收记录。

写入／生成：P1 Verifier。存储：verification。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/Verification`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| task_id | string | 是 | UUID 标识 |
| criterion_hash | string | 是 | SHA-256 十六进制摘要 |
| results | array&lt;object&gt; | 是 |  |
| verifier_version | string | 是 |  |
| created_at | string | 是 | UTC RFC3339 时间 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

每个 criterion_id 恰好一个结果，全部 PASS 才能完成；UNKNOWN 保留等待或需关注。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。

## 实施过程中发现的缺陷（D05，2026-09-14）

经 Ayanami 与实施方商议（`review/D05-consciousness-criterion.response.md`），补充 `consciousness_slot_committed`，其 expected 严格为 `{ "slot": 非负整数 }`。Verifier 必须按精确 slot 读取持久 ConsciousnessState，执行完整 DTO 校验并确认 DTO.slot 一致；不得以 MAX(slot) 或未来槽替代。仅程序 SlotController 可生成 memory.refresh，按固定 epoch/slot 派生稳定 root/request/intent 与 operation_key=memory_refresh；P2 登记持久任务，Core 使用命令目标 slot，不能重算成另一个槽。重复/重启复用既有命令；每槽最多三次，失败重试分别延后5分钟、30分钟。时钟前跳只登记当前槽并取消未派发旧槽，回拨不生成历史槽。初次 bootstrap 同样走持久命令。验收覆盖重复、精确槽、并发同槽与重启恢复；DDL 不变。


## 实施过程中发现的缺陷：D08 能力准入与固定验收条件

实施发现 `alarm.play`、`briefing.build`、`source.sync` 已有执行器，但公共准入的程序派生与 Criterion 判别联合不完整。经 [Ayanami D08 复核](../../review/D08-capability-criteria-deepseek.response.md) 同意，新增以下严格闭合的 kind，schema_version 保持 1；登记时冻结 criterion_hash，模型必须逐字段匹配程序派生值，不得事后改写。

- `alarm_session_recorded`：expected 仅 `{device_id, audio_ref}`，audio_ref 为命令 ObjectRef 的 UUID。Verifier 解析 Task 当前 run，交叉核对 alarm.play 及 args，并要求该 run 的会话 DTO 身份一致、状态 PLAYING 或 STOPPED。稍后提醒的新 Task/run 不得复用旧会话；不需要 D02 通知键实例化，因为会话按 run 绑定。PASS 仅证明已持久记录播放会话，不等于已叫醒、真实发声或 Item DONE；静音断言属于 A21 测试。
- `briefing_artifact_recorded`：expected 仅 `{media_type: "text/plain"}`。要求当前 run 当前 attempt/fence 的最终持久成功 receipt、effect_observed=true、至少一个非空对象，ObjectRef 与对象记录一致，实际字节 SHA256 自洽。旧回执、缺失或损坏对象不能 PASS。只证明产物持久化，不证明内容正确或有引用；不得读取模型自述判定 verdict，也不得事后回填 hash 自证。
- `source_sync_recorded`：expected 仅 `{source_id: UUID}`。当前 run 命令参数必须一致；SourceState 全 DTO 与 SQL 镜像一致且 cursor 非 NULL；追加 source.synced 事件必须带当前 run/attempt/fence provenance。`runtime.source_sync` 严格包含 `{run_id: UUID, attempt_no: integer>=1, fencing_token: integer>=1, records_processed: integer>=0}`，由 IngestService 在 SourceSynced 的同一事务内写入事件，禁止模型提供。其 records_processed 与事件 after.cursor.processed 一致。后续同步不覆盖旧事件。原 source_cursor_committed 保留旧语义；拒绝准入期猜测 cursor_hash 或依赖可变 fixture 内容计算标准。

三个新判定任一必要证据缺失或不一致均 UNKNOWN，不弱化 CRITERION_TAMPERED、授权和 fencing 检查。D05 memory.refresh 仍由专属 SlotController 登记，禁止普通公共准入。


### D08 修订 1：REPLAN 的当前 run 定义

前轮“同 Task 总共恰一条 run”的规则与合法 REPLAN 保留历史 run 冲突，已由 [Ayanami 补充裁决](../../review/D08-replan-current-run-deepseek.response.md) 纠正。当前 run = 同 Task `ORDER BY rowid DESC LIMIT 1` 的最新已接纳 run，和 REPLAN 选择 previous 的追加顺序相同。只查询其证据；禁止按成功状态回捞历史 run。任何更旧 run 仍为 QUEUED/CLAIMED/RUNNING/RESULT_UNKNOWN 都令新判定 UNKNOWN；当前 RESULT_UNKNOWN 须先完成对账。列与 DTO 的 ID/task/attempt/fence/state 始终交叉核对。

本规则依赖 job_run 只追加、不 DELETE、不 VACUUM 的存储不变量，插入与 REPLAN 修改在同一写事务内；当前无清理这类行的实现。未来引入清理/整理必须先迁移为显式 current_run_id 或代际，不能悄然沿用 rowid 假设。alarm 证据是 run 级（同 run 回收不额外要求会话 fence）；briefing receipt 和 source event 仍要求当前 attempt/fence。新标准内容和 criterion_hash 不改变，旧证据不满足新 REPLAN、连续两次 REPLAN 只认最后一代、重启不改变归属。
