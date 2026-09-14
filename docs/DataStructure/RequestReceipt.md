# RequestReceipt

幂等受理与提交回执。

写入／生成：共享请求服务。存储：request_receipt。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/RequestReceipt`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| principal_id | string | 是 |  |
| request_id | string | 是 | UUID 标识 |
| payload_hash | string | 是 | SHA-256 十六进制摘要 |
| intent_id | string | 是 | UUID 标识 |
| state | enum | 是 | ；ACCEPTED, COMMITTED, REJECTED |
| response | object | 是 | 按接口返回类型验证 |
| created_at | string | 是 | UTC RFC3339 时间 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

同 principal/request 键只对应一个语义载荷，response 不包含密钥。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。


### D12 分类传播

最终回复extension标记与同事务InputTurn/ASSISTANT一致；固定无内容失败字面量可SYN。 使用 [Common.md](Common.md) 的 security.classification 契约；缺失旧派生标记返回 OUTPUT_CLASS_UNKNOWN，不自动回填。

## Issue #2 会话兼容

精确已受理 request 先按原主体与语义回放，省略会话的兼容请求可恢复原绑定。显式不同 session 不允许 hash 回退；不重写旧 receipt、语义载荷、分类或旧历史。响应丢失时客户端保留 request_id 查询并重试，不由前端生成另一笔请求替代未知结果。
