# 设计决定与后置输入

## 1. Master 本轮给定的约束

| ID | 约束 | 落点 |
|---|---|---|
| U01 | Secretary 指 Secretary_Simplified；基于 Design2 重写 | design 与 README |
| U02 | 明确数据流、结构化接入、SQLite、上下文与调度输入输出 | DataFlow、DataStructure、Storage |
| U03 | 字段允许扩展，围绕实际功能测试迭代 | Common、TypeRegistry、Acceptance |
| U04 | World Model 由专用执行器经字段／授权控制修改 | world.update、ExecutionPermit、WorldCommitService |
| U05 | Consciousness 每 24 小时更新 | MemoryPolicy 固定经过时长的槽 |
| U06 | Context 借鉴 MemGPT，遗忘曲线按后续测试再定 | MemoryPolicy |
| U07 | 先 Core/Scheduler/Executor/CLI，再真实源、语音、移动 | Milestone M1—M6 |
| U08 | 第 4 步之前尽量无需 Master 介入，agent 自行复核 | ImplementationHandoff、Acceptance |

## 2. 本版工程默认

这些是为本次重设计选择的可实施默认，不冒称全部曾由 Master 逐项确认。

| ID | 选择 | 原因与代价 |
|---|---|---|
| E01 | 两个常驻 Go 进程、一个 SQLite | 延续草稿的独立执行路径；需短事务与跨进程测试 |
| E02 | 内置 agent 在 P1 运行，通过 P2 派发 | 满足初版专用执行器需求；避免额外平台 |
| E03 | CLI 首发，Mac App 不阻塞初版 | 与本轮阶段顺序一致 |
| E04 | 同一 DB 中事务性事件和持久扫描 | 无消息中间件依赖；轮询延迟需计量 |
| E05 | 专用程序服务完成最终事实提交 | 模型字段不能自行创造许可；增加窄接口和审计 |
| E06 | 24 小时快照 + 每次 Context 的当前状态／变化 | 保持日更节奏，又不使用一整天过期任务状态 |
| E07 | 结构化日历规则，暂不提供任意 cron 字符串 | 降低解析歧义与依赖，后续可适配已有规则 |
| E08 | 按对象拆分主要 DTO，与单一 Schema 文件对应 | 按索引查找，统一扩展与校验；不代表独立服务 |
| E09 | 初期检索不用向量数据库，遗忘默认关闭 | 先保证可追溯和可恢复，按真实资料再优化 |
| E10 | 测试阶段静音、合成源、独立目录 | 自动复核不影响 Master 的设备和私人资料 |
| E11 | 独立输入或时间 occurrence 有独立执行预算，子任务共享 | 周期长期可运行，自激任务仍有上限 |

## 3. 与草稿的具体变化

草稿将 App、audio-worker 和真实源列入同一物理图，本版按六阶段拆开。草稿把外部 Agent 标为后续扩展，本版将实现 World Model 更新所需的受限内置 agent 提前到 M2/M3，外部通用 Agent 仍后置。草稿将 Conversation State 标为 context，本版区分会话持久状态和单次模型工作上下文。

补齐了原稿缺少的物理表、严格字段、重复请求、相关版本 CAS、执行许可、未知效果、等待唤醒、周期实例、预算、备份、数据披露和测试证据。没有导入旧 Secretary 的分布式服务、PostgreSQL 或其他较重架构要求。

## 4. 后续输入的时点

M1—M3 无业务架构问题需要先等待 Master；真实模型凭据缺失只影响明确标注的 LIVE_MODEL 项。M4 决定真实来源范围、外发策略、提醒配置和实际使用反馈。M5 决定音色与试听；M6 决定设备、配对和连接范围。实际开发如发现必须改动业务、权限或正式验收语义，应形成具体决策包再处理，不能把本表当作自动授权扩大范围。
