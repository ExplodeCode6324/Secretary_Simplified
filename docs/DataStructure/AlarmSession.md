# AlarmSession

本地播放和停止会话。

写入／生成：P2 Mac 适配器。存储：alarm_session。

完整机器契约：[contracts.schema.json](../contracts.schema.json)，`#/$defs/AlarmSession`。

## 字段

| 字段 | 类型 | 必需 | 含义 |
|---|---|---|---|
| schema_version | 1 | 是 | 契约 major 版本 |
| id | string | 是 | UUID 标识 |
| run_id | string | 是 | UUID 标识 |
| revision | integer | 是 |  |
| state | enum | 是 | ；STARTING, PLAYING, STOPPING, STOPPED, UNKNOWN, FAILED |
| device_id | string | 是 |  |
| audio_ref | ObjectRef | 是 | 见类型定义 |
| playback_handle | string / null | 是 | 见类型定义 |
| saved_settings | object / null | 是 | 见类型定义 |
| started_at | string / null | 是 | 见类型定义 |
| stopped_at | string / null | 是 | 见类型定义 |
| stop_reason | string / null | 是 | 见类型定义 |
| extensions | object | 是 | 有命名空间的可选扩展 |

## 约束与使用

旧会话不能覆盖新播放或用户主动调整的设备设置。

通用空值、版本、扩展、引用和错误规则见 [Common.md](Common.md)；事件与提交关系见 [DataFlow.md](../DataFlow.md)。
