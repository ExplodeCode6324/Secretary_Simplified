# SourceRecord

接入后的来源结构化记录。

写入／生成：P1 IngestService。存储：source_record。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/SourceRecord`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| source_id | string | 是 | UUID 标识 |
| external_id | string | 是 |  |
| source_version | string | 是 | 上游版本或适配器生成的内容版本 |
| content_hash | string | 是 | SHA-256 十六进制摘要 |
| raw_ref | ObjectRef | 是 | 见类型定义 |
| observed_at | string | 是 | UTC RFC3339 时间 |
| source_updated_at | string / null | 是 | 见类型定义 |
| deleted | boolean | 是 |  |
| normalized_type | enum | 是 | 已登记的业务类型；fixture.item |
| normalized | FixtureItemValue / null | 是 | 见类型定义 |
| validation_status | enum | 是 | ；VALID, QUARANTINED |
| data_class | enum | 是 | ；SYNTHETIC, PERSONAL, SENSITIVE, SECRET |
| extensions | object | 是 | 有命名空间的可选扩展 |
| validation_error | string / null | 是 | 见类型定义 |

## 约束与使用

normalized 按 normalized_type 的注册 Schema 二次校验。无上游版本时使用内容哈希形成版本；删除必须有 tombstone 或完整同步对账证据。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。

## 实施过程中发现的缺陷（D03，2026-09-14）

经 Ayanami 复核与实施方商议，原 VALID 分支强制 normalized 为对象，无法表达合法删除 tombstone。修订为 deleted=true 且 VALID 时 normalized=null、validation_error=null；非删除 VALID 仍要求 FixtureItemValue。QUARANTINED 规则不变。复核依据：`review/M1-foundation-ayanami.md`。
