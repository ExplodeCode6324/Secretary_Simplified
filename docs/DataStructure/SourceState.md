# SourceState

来源同步游标和新鲜度。

写入／生成：P1 IngestService。存储：source_state。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/SourceState`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| kind | enum | 是 | ；FIXTURE, GITHUB, CANVAS, DIRECTORY |
| revision | integer | 是 |  |
| cursor | object / null | 是 | 见类型定义 |
| last_success_at | string / null | 是 | 见类型定义 |
| last_error | string / null | 是 | 见类型定义 |
| stale_after_seconds | integer | 是 |  |
| enabled | boolean | 是 |  |
| data_class | enum | 是 | ；SYNTHETIC, PERSONAL, SENSITIVE, SECRET |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

成功但没有新增数据也推进 last_success_at；freshness 在读取时计算。真实适配器配置阶段 4 才填写。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
