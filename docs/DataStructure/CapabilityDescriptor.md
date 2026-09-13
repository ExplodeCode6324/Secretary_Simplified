# CapabilityDescriptor

静态能力登记描述。

写入／生成：程序能力注册表。存储：配置及执行快照。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/CapabilityDescriptor`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 |  |
| version | integer | 是 |  |
| arguments_schema | object | 是 | 类型注册表约束的 JSON 对象 |
| result_schema | object | 是 | 类型注册表约束的 JSON 对象 |
| side_effect_class | enum | 是 | ；READ_ONLY, LOCAL_MUTATION, EXTERNAL_MUTATION |
| supports_idempotency | boolean | 是 |  |
| supports_query | boolean | 是 |  |
| supports_cancel | boolean | 是 |  |
| timeout_ms | integer | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

服务启动检查能力实现与 Schema 对应，未知能力不派发。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
