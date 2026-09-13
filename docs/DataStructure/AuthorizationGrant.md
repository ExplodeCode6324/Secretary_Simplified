# AuthorizationGrant

程序管理的授权记录。

写入／生成：PolicyService。存储：authorization_grant。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/AuthorizationGrant`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| principal_id | string | 是 |  |
| revision | integer | 是 |  |
| capability_ids | array&lt;string&gt; | 是 |  |
| scope | object | 是 | 见类型定义 |
| policy_revision | integer | 是 |  |
| expires_at | string | 是 | UTC RFC3339 时间 |
| revoked | boolean | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

空范围表示不允许，不表示通配；测试授权绑定测试实体或明确的测试域注册规则。授权不由模型创建。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
