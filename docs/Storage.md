# SQLite 存储契约

## 1. 物理结构

完整初版 DDL 在 [schema.sql](schema.sql)。所有业务时间以 UTC RFC3339 固定毫秒精度文本保存，例如 `2026-09-14T01:00:00.000Z`，比较前必须正规化；日历规则另存 IANA timezone。布尔列使用 0/1；JSON 中保留真实 boolean。ID 为 UUID 文本，顺序由 ChangeEvent.seq 和会话 sequence 确定，不依赖 UUID 字典顺序。

SQL 列负责身份、关联、版本、检索和状态约束；有 payload_json 的表保存完整的版本化 DTO，包含扩展字段。两者代表同一对象，由专用 repository 在同事务生成，写入前必须核对重复字段一致；读取时检测不一致并报 STORAGE_CORRUPTION，不能择一覆盖。没有 payload_json 的规范化表由列重建 DTO，schema_version 由迁移版本映射，extensions 首版固定为空；新增持久扩展需迁移。request_receipt.response_json 对应 RequestReceipt.response。JSON 有效性检查仅验证语法，应用还必须执行 JSON Schema 和业务约束。

初版不按模块分库，不使用网络共享盘。WAL 支持读写并发但仍需串行化写事务；本项目使用短事务、唯一约束与有界重试。[SQLite WAL](https://www.sqlite.org/wal.html)

连接配置：foreign_keys=ON、busy_timeout=5000、synchronous=FULL；journal_mode=WAL 在初始化设置。每个连接读取并验证配置，不依赖驱动默认值。FULL 是本项目对提交持久性的选择，不等于对所有硬件故障作保证。[SQLite PRAGMA](https://www.sqlite.org/pragma.html)

## 2. 表归属

| 组 | 表 | 唯一写入入口 |
|---|---|---|
| 迁移与文件 | schema_migration、object_ref | 迁移器／ObjectStore |
| 接入 | source_state、source_record、observation | P1 IngestService |
| 事项 | item、item_dependency | P1 ItemService |
| 世界模型 | world_proposal、world_fact_version、world_fact_head | 提案由 P1；事实只经 WorldCommitService |
| 对话 | request_receipt、conversation_session、conversation_event、input_turn | P1 ConversationService |
| 认知 | consciousness_snapshot、context_manifest、decision_record | P1 对应业务服务 |
| 授权与预算 | authorization_grant、root_budget | PolicyService；P1/P2 使用同一事务接口 |
| 委托与命令 | task、command_ledger、scheduled_job | 共享 CommandService；允许 Runner 类型化取消／延后 |
| 执行 | job_run、execution_attempt、execution_permit、executor_receipt | P2 RunService；P1 仅通过许可服务消费许可 |
| Core 内置工作 | core_work | P1 CoreWorkService；按 run_id 去重和恢复 |
| 验收和等待 | verification、wait_subscription | P1 Verifier／TaskService |
| 事件恢复 | change_event、consumer_cursor、rule_state | 各受控服务同事务追加／推进自己的游标 |
| 本地效果 | alarm_session、notification | P2 本地适配器；客户端收取确认走服务接口 |

## 3. 应用层必须补充的约束

SQL 已实现主要唯一键、外键和状态枚举，但以下规则必须在共享 repository 的事务内实现并测试：

- 更新 `WHERE id=? AND revision=?`，成功后 revision+1；零行即冲突。
- Item 依赖为无环图；插入依赖时检查可达路径，不能仅拒绝自环。
- world_fact_version 只追加；world_fact_head 指向最新准入结果；纠正和撤回不覆写历史。单值事实冲突由 policy 处理。
- 源 `(source_id, external_id, source_version)` 若出现不同哈希，隔离为 SOURCE_VERSION_CONFLICT；不能悄悄覆盖。
- 事实证据引用、Context read_set、criterion 与 artifact 引用均验证存在性和版本。嵌套 JSON 引用不由 SQLite 外键自动保护。
- 同一 intent 的命令账本、任务／计划与事件必须原子；task occurrence 与 run occurrence 必须匹配。
- 日历推进、run 创建、重复触发抑制同事务；状态迁移、租约与 fencing 按 ExecutionProtocol 校验。
- 追加事件与 consumer cursor、等待唤醒在处理事务里更新，避免仅推进游标却未完成业务处理。
- immutable criterion_hash 不得被普通 Task 更新改写；准入的证据原文不能被对象 GC 删除。

## 4. 扩展与迁移

schema_version 使用整数 major，初版为 1。未知 major 拒绝执行；未知业务 enum 或 capability 拒绝执行；只读展示可保留原始结构并标不支持。新增可选字段必须提供默认／缺失语义并更新 Schema、DTO、例子和兼容测试；必需字段或语义变化递增 major 并提供迁移。

每个主要 DTO 带 extensions，键使用 `namespace.name`，值必须为 JSON object。扩展不得改变身份、权限、完成条件、状态迁移或调度；被程序消费前须登记扩展 Schema。首版内置 Schema 不接受任意额外顶层字段。

SQL 迁移与 JSON payload 迁移同步编号，迁移记录保存迁移文件哈希。附件 DDL 是空库基线，不得对运行库反复执行。升级规则与备份恢复见 [Operations.md](Operations.md)。
