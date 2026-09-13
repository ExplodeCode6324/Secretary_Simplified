# ContextManifest

一次模型请求的可复核清单。

写入／生成：P1 ContextBuilder。存储：context_manifest。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/ContextManifest`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| intent_id | string | 是 | UUID 标识 |
| snapshot_seq | integer | 是 |  |
| as_of | string | 是 | UTC RFC3339 时间 |
| read_set | array&lt;ReadRef&gt; | 是 |  |
| request_hash | string | 是 | SHA-256 十六进制摘要 |
| policy_revision | integer | 是 |  |
| output_schema_id | string | 是 |  |
| output_schema_hash | string | 是 | SHA-256 十六进制摘要 |
| model_profile | string | 是 |  |
| input_bytes | integer | 是 |  |
| input_tokens | integer | 是 |  |
| token_count_mode | enum | 是 | ；TOKENIZER, CONSERVATIVE_ESTIMATE |
| sections | array&lt;object&gt; | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

read_set 是提交时 CAS 的实际相关对象集合。request_hash 对最终请求计算，不把 hash 字段本身递归计入。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。

## 实施过程中发现的缺陷（D07，2026-09-14）

原文已有 manifest 是本地外层、不得参与自身 wire 哈希的边界，但机器 Schema 仍把整个 ContextManifest 放入 Context，二者冲突。经 Ayanami 同意（`review/D07-context-manifest.response.md`），Context 不含 manifest，required metadata 为 context_id/as_of/snapshot_seq/sections；read_set 留在外部 Manifest。sections 每段字段严格为 name/selected_count/omitted_count/bytes/reason，name 不重复。完整 output_contract 在 Context 中只嵌入一次，adapter 引用它而不再次发送Schema。最终 wire 编码后计算外部 Manifest.request_hash，发送相同字节；Context.context_id、Manifest.id 和 Decision.context_id 必须一致。可选 stale_refs 与 registered_entity_ids 都有长度和去重约束。
