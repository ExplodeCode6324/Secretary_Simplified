# Command

待登记的类型化执行命令。

写入／生成：P1 校验，P2 执行。存储：command_ledger / job_run 命令快照。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/Command`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| operation_key | string | 是 | 意图内稳定操作名 |
| capability | enum | 是 | 已登记能力 ID；notify.local, alarm.play, alarm.stop, alarm.snooze, artifact.write, source.sync, briefing.build, memory.refresh, world.update, memory.search |
| capability_version | 1 | 是 | 见类型定义 |
| arguments | object | 是 | 按 capability 的参数 Schema 校验 |
| expected_revisions | array&lt;ReadRef&gt; | 是 |  |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

capability 名本身不授予权限。执行参数必须二次 Schema 校验，命令不能携带任意 shell。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。


### D12 分类传播

模型参数按请求class；运行复制和替换参数join已有class。 使用 [Common.md](Common.md) 的 security.classification 契约；缺失旧派生标记返回 OUTPUT_CLASS_UNKNOWN，不自动回填。
