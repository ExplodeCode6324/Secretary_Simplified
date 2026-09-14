# Context

实际送入模型的有界结构化上下文。

写入／生成：P1 ContextBuilder。存储：文件对象 + context_manifest。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/Context`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| context_id | UUID | 是 | 与外部 Manifest.id、Decision.context_id 一致 |
| as_of | UTC timestamp | 是 | 投影时间 |
| snapshot_seq | integer | 是 | 数据库快照事件序号 |
| sections | array&lt;ContextSection&gt; | 是 | 每段选择、省略和最终 UTF-8 字节统计，name 唯一 |
| stale_refs | array&lt;ReadRef&gt; | 否 | 已失效的派生引用，最多 100 且去重 |
| registered_entity_ids | array&lt;UUID&gt; | 否 | 已登记实体，最多 100 且去重 |
| system_rules | string | 是 |  |
| current_input | InputEnvelope | 是 | 见类型定义 |
| world | WorldModelInput | 是 | 见类型定义 |
| live | LiveWorldStateInput | 是 | 见类型定义 |
| consciousness | ConsciousnessState / null | 是 | 见类型定义 |
| conversation | ConversationState | 是 | 见类型定义 |
| recent_events | array&lt;ConversationEvent&gt; | 是 |  |
| tasks | array&lt;Task&gt; | 是 |  |
| delta_events | array&lt;ChangeEvent&gt; | 是 |  |
| retrieved_evidence | array&lt;EvidenceRef&gt; | 是 |  |
| capability_ids | array&lt;string&gt; | 是 |  |
| output_contract | object | 是 | 实际嵌入对应 JSON Schema，不只放名字 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

manifest 哈希及计量字段属于本地外层，不参与最终模型请求序列化；只发送其余字段和必要只读元信息。公开 capability_ids 不包含令牌。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。

## 实施过程中发现的缺陷（D07，2026-09-14）

原文已有 manifest 是本地外层、不得参与自身 wire 哈希的边界，但机器 Schema 仍把整个 ContextManifest 放入 Context，二者冲突。经 Ayanami 同意（`review/D07-context-manifest.response.md`），Context 不含 manifest，required metadata 为 context_id/as_of/snapshot_seq/sections；read_set 留在外部 Manifest。sections 每段字段严格为 name/selected_count/omitted_count/bytes/reason，name 不重复。完整 output_contract 在 Context 中只嵌入一次，adapter 引用它而不再次发送Schema。最终 wire 编码后计算外部 Manifest.request_hash，发送相同字节；Context.context_id、Manifest.id 和 Decision.context_id 必须一致。可选 stale_refs 与 registered_entity_ids 都有长度和去重约束。

D07 检索补充经 Ayanami 同意（`review/D07-retrieval-section.response.md`）：sections 的第七种 name 为 retrieved_evidence；selected_count 为实际 EvidenceRef 数，omitted_count 只计本次有界检索已发现而省略的候选，不声称统计未检索空间。bytes 是最终字段 JSON 的 UTF-8 字节数；正文不复制进 sections。每次 READ_MEMORY 后重新构建 Context、外部 Manifest 和 wire hash。

## Issue #2 当前绑定与可见性

主轮读取持久队头冻结的 ConversationState 与可见事件前缀；后到未处理输入同时从近期原话和 READ_MEMORY 排除。客户端历史不参与构建。世界与业务对象仍执行原 read_set/授权检查。
