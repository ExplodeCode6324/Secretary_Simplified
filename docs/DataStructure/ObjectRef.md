# ObjectRef

原文或产物的不可变引用。

写入／生成：ObjectStore。存储：object_ref。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/ObjectRef`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| relative_path | string | 是 | 受控目录内相对路径 |
| sha256 | string | 是 | SHA-256 十六进制摘要 |
| media_type | string | 是 |  |
| byte_size | integer | 是 |  |
| data_class | enum | 是 | ；SYNTHETIC, PERSONAL, SENSITIVE, SECRET |
| created_at | string | 是 | UTC RFC3339 时间 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

路径必须在受控根目录，拒绝 ..、绝对路径和符号链接逃逸；内容哈希不匹配则不可用。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。


### D12 分类传播

既有顶层data_class仍是对象实际原始/派生字节分类；原文用inputclass，模型wire/产物用reqclass，复制不降级。 使用 [Common.md](Common.md) 的 security.classification 契约；缺失旧派生标记返回 OUTPUT_CLASS_UNKNOWN，不自动回填。


Issue #1 AUD-01/03：对象采用同目录完整临时写+同步+hard-link no-replace原子发布，object_ref 与受理/业务联合事务；完整无引用孤儿需重新同步后采纳，坏孤儿隔离，已提交损坏不可覆盖。取消后的清理须持固定跨进程锁并确认引用不存在。协议与恢复接口见 [Storage.md](../Storage.md)。
