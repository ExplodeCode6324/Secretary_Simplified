> 模型限定：本报告来自 Ayanami 使用 `gpt-5.6-luna` 的历史复核。Master 最新指定必须由 `deepseek-v4.1-flash` 复核，因此本报告仅为 model-qualified 历史证据，不能充当最终复核裁决；原问题、结论与测试结果保留，待 DeepSeek 重新检视。

D05 复核结论（只读；未修改源码/设计，未读取 resources，未连接 ELIZA）

1. 缺陷成立  
问题：`memory.refresh` 需要验证目标 Consciousness slot，但现有 Criterion 没有对应枚举；不能使用 `world_revision_matches` 冒充。  
证据：
- `docs/MemoryPolicy.md:19-23`：SlotController 必须按 epoch/slot 生成持久 `memory.refresh` 命令。
- `docs/ExecutionProtocol.md:60、78`：声明 `memory.refresh`，但 Verifier 清单没有 consciousness slot。
- `docs/contracts.schema.json:2088-2095`：Criterion 枚举仅 artifact、notification、world、source、master。
- `src/store/runtime_local.go:186-216`：Verifier 无 memory criterion 分支；即使加入 DTO，当前也无法 PASS。
结论：同意，D05 是真实设计缺口。

2. 提议的 Criterion  
方案：增加 `consciousness_slot_committed`，严格定义：

```json
{
  "kind": "consciousness_slot_committed",
  "expected": {
    "slot": 0
  }
}
```

`expected` 必须 `additionalProperties:false`，`slot` 为 integer、minimum=0。slot 只能由程序 SlotController 计算和写入，模型不能选择或修改。  
证据：
- `docs/contracts.schema.json:1737-1740` 已规定 `ConsciousnessState.slot >= 0`。
- `docs/schema.sql:87-90` 已有 `slot >= 0` 与 `UNIQUE(slot)`。
结论：同意；不需要修改 DDL，也不应复用 fact revision criterion。

3. Verifier 条件  
方案：Verifier 必须按精确目标执行：

```sql
SELECT payload_json
FROM consciousness_snapshot
WHERE slot = ?
```

读取后执行 `contract.Decode("ConsciousnessState", ...)`，确认 DTO 合法且 DTO.slot 等于 expected.slot。不能只查询 `MAX(slot)` 或 `slot >= expected`，否则可能用其他槽完成当前任务。  
结论：同意，但这是必要条件。slot 全局唯一使“该槽已提交”可作为幂等证据；SlotController 必须禁止非它生成的 `memory.refresh` 任务，否则仅凭 slot 可能被错误任务借用。

4. 现有实现缺口  
问题：当前实现尚未形成完整的“持久命令 → 执行 → Criterion 验证”闭环。
- `src/cmd/secretaryd/main.go:148-166`：Core 定时器直接调用 `memory.Service.Refresh`，绕过持久 JobRun。
- `src/core/work.go:69-76`：虽处理 `memory.refresh` run，但忽略 `run.command.arguments.slot`，重新按当前时间计算。
- `src/memory/memory.go:23-85`：Refresh 接口自行计算 slot 并直接 `SaveConsciousness`。
- `src/store/runtime_local.go:251-270`：`DeriveCriteria` 当前不支持 `memory.refresh`。
方案条件：
- SlotController 以持久 epoch+slot 派生稳定 root/request/intent，使用固定 `operation_key=memory_refresh`，唯一生成 command、Task 和 `consciousness_slot_committed` criterion。
- Executor 必须消费 command 中的目标 slot；不得静默改用当前时间重新决定 slot。
- 直接 ticker Refresh 必须改为登记/推进持久命令，不能保留旁路成功路径。
结论：提议本身同意，但仅增加枚举和 Verifier 不足；以上三项必须同时满足。

5. 重启与旧槽  
证据：
- `src/memory/memory.go:39-40`：已有快照 slot 不小于目标 slot 时不再生成。
- `src/store/memory_repo.go:223-240`：保存时拒绝旧/重复 slot，并在同一事务写入快照和 `memory.refreshed` 事件。
- `src/tests/core_integration_test.go:98-115、298-300`：已有同槽重入、时钟回拨、失败重启预算测试。
结论：同意现有“旧槽不重复”方向；新增验收必须覆盖持久命令重复提交、精确 slot 验证、并发同槽提交和重启恢复。

6. 需要登记的修改路径  
`docs/MemoryPolicy.md`、`docs/ExecutionProtocol.md`、`docs/DataStructure/TypeRegistry.md`、`docs/DataStructure/Verification.md`、`docs/DataStructure/ConsciousnessState.md`、`docs/contracts.schema.json`、`docs/Acceptance.md`；README 增加 D05 缺陷及实际修改路径。`docs/schema.sql` 不修改。

总裁决：D05 缺陷成立；增加 `consciousness_slot_committed` 方案同意，但必须绑定“SlotController 唯一生成、command slot 不得被忽略、Verifier 精确查询并验证完整 DTO、移除直接 Refresh 旁路”四项条件。由 Codex 按约定保存至 `review/D05-consciousness-criterion.response.md`。
