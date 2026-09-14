> 模型限定：本报告来自 Ayanami 使用 `gpt-5.6-luna` 的历史复核。Master 最新指定必须由 `deepseek-v4.1-flash` 复核，因此本报告仅为 model-qualified 历史证据，不能充当最终复核裁决；原问题、结论与测试结果保留，待 DeepSeek 重新检视。

D07 复核结论（只读；未修改文件，未读取 resources，未连接 ELIZA）

1. 自引用缺陷成立  
证据：
- `docs/DataStructure/Context.md:14`：Context 必须包含完整 `ContextManifest`。
- `docs/contracts.schema.json:4370-4385`：`manifest` 是 Context required 字段。
- `docs/DataStructure/ContextManifest.md:19、32`：`request_hash` 定义为最终请求哈希，且当前说明“不把 hash 字段本身递归计入”，与“最终 wire bytes 的完整 SHA-256”不一致。
- `src/context/context.go:73-111`：实际先编码 wire，再生成并保存 manifest；manifest 并未进入实际 input。
- `src/tests/core_integration_test.go:170-196`：现有测试已要求 `manifest.RequestHash == SHA256(actual wire)`。
结论：同意，完整 Manifest 放入被 hash 的 wire 会形成自引用；不得使用零值、占位值或事后伪造 hash。将 Manifest 保留为本地外部记录是正确修复。

2. Context 与外部 ContextManifest 分离  
方案：同意。正式 Context 移除 `manifest`，新增并要求：

```text
schema_version
context_id: UUID
as_of: UTC RFC3339
snapshot_seq: integer >= 0
system_rules
current_input
world
live
consciousness
conversation
recent_events
tasks
delta_events
retrieved_evidence
capability_ids
sections
output_contract
extensions
```

外部 `ContextManifest` 保留完整审计字段，包括：

```text
id (= context_id)
intent_id
snapshot_seq
as_of
read_set
request_hash
policy_revision
output_schema_id
output_schema_hash
model_profile
input_bytes
input_tokens
token_count_mode
sections
extensions
```

必要条件：
- `context_id == ContextManifest.id == model.Request.ContextID`；
- `snapshot_seq/as_of` 与同一次只读快照一致；
- `read_set` 只留本地 Manifest，不进入模型 Context；
- `request_hash = SHA256(实际发送的最终 wire bytes)`，不是 Context 内字段。

3. `output_contract` 方案  
结论：同意，但必须只出现一次。

- `Context.output_contract` 放完整且严格的实际输出 Schema closure，不能只有 schema 名称；
- `ContextManifest.output_schema_hash` 计算同一份 canonical schema bytes；
- adapter/system instruction 只能说明“遵守 `input.output_contract`”，不得再次嵌入完整 Schema；
- `src/model/model.go:61-74` 当前 adapter 会把完整 Schema放入 system instruction，需改为不重复；
- 本地仍须对模型输出执行 `contract.Decode/Validate`，不能因 Schema 已送入 Context 而降低校验。

4. sections / selection 最小结构  
不建议同时保留同义的 `sections` 与 `selection`。建议正式字段统一为 required `sections`，移除 `selection`。

建议定义严格 `ContextSection`：

```text
name: enum(items, facts, tasks, delta_events, recent_events, consciousness)
selected_count: integer >= 0
omitted_count: integer >= 0
bytes: integer >= 0
reason: string
```

`additionalProperties:false`，section name 不重复。`bytes` 必须对应最终 Context 中相应字段的 UTF-8 JSON 字节数；`input_bytes` 则是包含 adapter transport wrapper 的最终 wire 字节数，两者不可混淆。  
依据：当前 `src/context/context.go:44-60、97-107` 已实际生成上述元数据，但使用了未登记的 `selection` 字段。

5. 可选字段  
结论：同意保留为 optional，但结构必须明确：

- `stale_refs?: ReadRef[]`，最大数量沿用 read_set 上限；若存在 stale ref，必须按当前实现语义将受影响 Consciousness 内容置空或标明 omitted。
- `registered_entity_ids?: string[]`，限制数量、去重；仅作为只读 allow-list，不是授权凭证。因配置类型为通用 string，不应强制 UUID。
- 若当前所有 Builder 都能确定这些值，建议实际 wire 稳定地发送空数组，而不是让“缺失”与“确实为空”含义混淆。

6. 精确 hash 的生成顺序  
必须单独写入设计：

1. 组装并完整校验无 Manifest 的 Context；
2. 将完整 `output_contract` 放入 Context；
3. adapter 生成最终 wire bytes；
4. 对这份即将发送的 bytes 计算 `request_hash`；
5. 保存外部 ContextManifest；
6. 发送完全相同的 bytes。

当前 `Builder` 先 Encode，随后 `Generate` 又重新 Encode（`src/context/context.go:73-111`、`src/model/model.go:118-124`）。修订后必须保证两次 bytes 字节级一致，或直接传递已冻结 wire；不能仅依赖“理论上 deterministic”。

总裁决：D07 自引用缺陷成立；“正式 Context 移除 Manifest、外部 Manifest 精确记录 hash、output_contract 只出现一次”方案同意。必要条件是新增 Context 身份字段、统一 `sections`、严格 sectionSchema、adapter 不重复 Schema、Manifest 与实际 wire 建立字节级 hash 关系。建议修改路径：`docs/DataStructure/Context.md`、`ContextManifest.md`、`MemoryPolicy.md`、`docs/contracts.schema.json`、`docs/Acceptance.md`，并在 README 登记 D07 实施缺陷及路径。由 Codex 保存至 `review/D07-context-manifest.response.md`。
