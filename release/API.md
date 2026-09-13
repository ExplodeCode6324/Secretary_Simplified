# 已实现的本地接口

Core 和 Runner 使用 data-dir/run 中的私有 Unix socket 与 client.token；内部凭据独立。没有 TCP 服务。接口表见 docs/Interfaces.md。

## 类型化变更

POST /v1/items、POST /v1/jobs、POST /v1/world/proposals、PATCH /v1/items/{id}、PATCH /v1/jobs/{id} 使用以下信封：

```json
{"schema_version":1,"request_id":"00000000-0000-4000-8000-000000000200","session_id":"00000000-0000-4000-8000-000000000001","expected_revision":0,"payload":{}}
```

创建时 expected_revision 为 0；更新必须使用当前业务 revision。创建 payload 分别对应 ActionProposal 的 CREATE_ITEM、CREATE_JOB、WORLD_PROPOSAL payload。事项更新 payload 对应 UPDATE_ITEM.changes，支持 title/status/due_at/priority。计划 PATCH 只允许 schedule/enabled/misfire/grace_seconds/overlap/max_attempts，不能修改已冻结 command、criterion 或 root。

POST /v1/actions 支持原子提交 ActionProposal 数组，信封是 schema_version/request_id/session_id/actions。它返回持久 InputTurn；执行仍异步。创建和更新事项的专用路由返回该次提交的原始 Item 版本，重试不会重复创建。World proposal 返回专用 task_id，尚不是事实写入成功。

## 查询与控制

GET /v1/items 支持 domain/status/cursor/limit；GET /v1/world 支持 entity/predicate/cursor/limit。返回 items/next_cursor/snapshot_seq。默认 50、最多 200；游标绑定过滤参数、页大小与事件水位，水位改变时返回冲突，客户端从第一页重新读，避免拼接不同版本。世界列表只列当前 head，历史版本仍在不可变事实表。

POST /v1/memory/search 输入 schema_version=1、query，可选 entity_ids/cursor；返回授权资料的 events 与下一页 cursor。GET /v1/events 支持 after_seq/limit，按递增序号恢复。

任务 cancel、计划 trigger/pause/resume 使用 schema_version/request_id/expected_revision。run cancel 的版本属于关联 Task，CLI 会先读取。alarm stop/snooze 不等待模型，snooze 必须给 delay_seconds。notifications ack 使用 schema_version/request_id。

输入返回 202 表示受理而非完成；按 turn_id 查 /v1/turns/{id}。同一次重试保持 request_id，不要重新生成。所有失败响应使用非成功 HTTP 状态，公开错误不包含私有原文。
