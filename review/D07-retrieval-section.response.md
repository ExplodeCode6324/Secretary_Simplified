D07 补充结论：同意。

约束：

- `ContextSection.name` 增加 `retrieved_evidence`，与 Context 正文字段一一对应；不再使用 `selection` 作为另一套同义结构。
- section 只记录：
  `name / selected_count / omitted_count / bytes / reason`；
  检索正文只出现于 `Context.retrieved_evidence`，不得在 section 中复制证据内容，也不得伪装成 `recent_events`。
- `selected_count` 统计实际纳入的 EvidenceRef 数量；`omitted_count` 统计本次 bounded retrieval 中被省略的候选数量，不把“未检索”伪装成 omitted。
- `bytes` 必须是最终 Context 中 `retrieved_evidence` JSON 的 UTF-8 字节数；`reason` 说明低相关淘汰、页/字节预算或无结果。
- 无 READ_MEMORY 检索时也建议保留 `retrieved_evidence` section，计数为 `0/0`；有检索时必须记录真实 selected/omitted。
- 每次 READ_MEMORY 重新组装 Context，重新生成 sections、ContextManifest 和最终 wire `request_hash`；不得修改或复用旧 ContextManifest 的 hash。
- `retrieved_evidence` section 不改变既有 `read_set`、`stale_refs`、`output_contract` 和精确 wire hash 条件。

总裁决：通过；这是对既有 READ_MEMORY 路径的必要最小补充。只需同步 `ContextSection` Schema、Context 文档、MemoryPolicy/READ_MEMORY 说明及相关验收，不涉及 DDL。
