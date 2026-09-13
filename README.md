# Secretary_Simplified

面向 Master 的持久化文字助手。依据 `docs/` 中的设计基线，以 Go 实现 Core、Scheduler、Executor、四层记忆与本地 CLI，首先在 Mac 上使用合成数据完成验证，再由 Master 接入真实资料。

> 当前状态：**代码与本机部署已落地，尚未通过 M1—M3 完整验收**。真实模型额度耗尽，最终复核也被阻塞；当前不能宣布可接入真实数据。

公开仓库：[ExplodeCode6324/Secretary_Simplified](https://github.com/ExplodeCode6324/Secretary_Simplified)。本机可运行版本位于 `release/`；最新有效离线证据见 [final-offline.json](reports/implementation/final-offline.json)。

## 本轮交付范围

1. 在 `src/` 完成一个 Go module，构建 `secretaryd`（Core/Runner 两种常驻模式）及 `secretary` CLI。
2. 在 `release/` 部署本机二进制、配置、初始化数据库和运行目录，提供启动与诊断说明。
3. 每个模块完成后交给本地 Hermes **Ayanami** 复核，真实复核结果保存在 `review/`，修复后记录复查结果。
4. 在 GitHub 创建公开的 `Secretary_Simplified` 仓库，发布可公开的项目文件。
5. 完成已授权的合成数据验证后通知 Master，说明真实数据接入所需范围与披露授权。

**发布边界：** API key、认证凭据、本地私有配置、运行数据库与私人原文不进入公开仓库。交付配置模板、数据库初始化方式及可公开的空库/部署材料；本地已部署的运行状态单独保留。资源目录中 Master 提供的密钥只用于明确配置的模型调用，不写入日志或复核报告。

## 设计与验收入口

- [设计目录与阅读顺序](docs/README.md)
- [整体设计](docs/design.md)、[工程架构](docs/EngineeringArchitecture.md)
- [实施交接](docs/ImplementationHandoff.md)、[阶段路线](docs/Milestone.md)
- [验收矩阵 A01—A30](docs/Acceptance.md)
- [安全与披露](docs/Security.md)、[运行与恢复](docs/Operations.md)
- [Ayanami 复核记录](review/README.md)

规范冲突按设计包要求修订并复核，不通过弱化验收标准消除失败。M4 真实资料、M5 语音、M6 移动端属于后续阶段。

## 实施计划

| 步骤 | 工作 | 退出证据 | 当前状态 |
|---|---|---|---|
| 0 | 阅读设计、确认依赖与 OpenCode Go 协议、建立 Ayanami 复核会话 | 设计范围、官方协议来源、真实握手记录 | 已完成初步核对 |
| M1 | 严格 DTO/Schema、迁移、ObjectStore、事项 CAS/依赖、来源同步、请求幂等与事件事务 | A01—A05、A19、A24；对应 Go 测试与复核 | 实施中 |
| M2 | WorldCommit 权限、事实历史/纠正/冲突、四层记忆、24 小时意识槽、有界 Context、模型适配器 | A06—A10、A16、A17；离线与真实模型分开报告 | 实施中 |
| M3 | Core/Runner、Scheduler/Executor、固定验收、取消/等待/重试/恢复、CLI/诊断/备份 | 适用 A01—A25；进程集成、竞态、故障恢复测试 | 实施中 |
| 验证 | 72 小时虚拟时钟、30 天模拟回放、至少 2 小时真实本机运行、合成资料真实模型评估 | 带构建与输入哈希的报告；所有失败及重试保留 | 离线通过；真实模型额度阻塞，最终构建时长采样中 |
| 部署 | 构建两个二进制，初始化本地 release，核验新目录使用流程 | release 使用说明、校验和、doctor 与完整链路 | macOS arm64 已构建并初始化，fixture 双进程链路通过 |
| 发布 | 凭据与私有路径检查、公开仓库创建、上传与远端同步核验 | 仓库链接、提交 ID、发布清单 | 公开仓库已创建并推送；凭据与私人运行资料排除 |
| 交接 | 汇总通过、失败、未运行及限制，通知 Master 接入真实数据 | 可复核交付报告与接入事项 | 待前序完成 |

## 模块分工与复核流程

- 主 agent：集成协调、模型适配、Core、World/Memory/Context、传输与 CLI、部署及发布。
- 基础模块子 agent：`contract`、`store` 基础、`ingest`、`platform` 和依赖锁定。
- 运行模块子 agent：`scheduler`、`executor`、`policy` 及对应持久化与测试。
- 复核协调子 agent：与本地 Hermes Ayanami 保持独立复核会话；接收模块路径、测试结果和关注点，保存实际回复。

模块流程：实现 → 针对性测试 → 通知 Ayanami → 保存复核 → 修复问题 → 重测/复查 → 更新本页。握手和审查意见不等同于测试通过；尚未审查的模块不能标为已通过复核。

## 实施过程中发现的设计缺陷

Master 已授权：实现中发现设计缺陷时，先与本地 Ayanami 商议；双方达成一致后可以修改。修改必须同步相关设计文档、Schema/DDL（如涉及）、实现和测试，并注明是“实施过程中发现的缺陷”。本节逐项链接修改文档和复核依据，不把实现偏差默认为设计已变更。

| 缺陷编号 | 实施发现与修改理由 | 修改的设计文档路径 | Ayanami 讨论/结论 | 状态 |
|---|---|---|---|---|
| D01 | 计划登记要求原子事件，但事件注册表缺 ScheduledJob 类型；新增 scheduled_job.updated，明确业务修订与扫描推进边界 | [docs/DataStructure/TypeRegistry.md](docs/DataStructure/TypeRegistry.md) | [Ayanami 同意及附带条件](review/D01-scheduled-job-event.response.md) | 设计已同步，实现与重验中 |
| D02 | 周期计划静态通知键导致后续运行复用旧通知；按 occurrence 无歧义编码并先绑定实例验收条件 | [ExecutionProtocol](docs/ExecutionProtocol.md)、[DataFlow](docs/DataFlow.md)、[ScheduledJob](docs/DataStructure/ScheduledJob.md)、[Task](docs/DataStructure/Task.md)、[Notification](docs/DataStructure/Notification.md)、[TypeRegistry](docs/DataStructure/TypeRegistry.md) | [Ayanami 同意及条件](review/D02-notification-occurrence.response.md) | 设计已同步，实现与重验中 |
| D03 / D04 | 有效删除页无法使用 normalized=null；明确重复页不新增对象引用的验收口径 | [contracts.schema.json](docs/contracts.schema.json)、[SourceRecord](docs/DataStructure/SourceRecord.md)、[Acceptance A05](docs/Acceptance.md) | [Ayanami M1 复核](review/M1-foundation-ayanami.md) | 基础模块同步修订与修复中 |
| D05 | memory.refresh 缺少可独立验证的完成条件；新增由槽控制器生成的精确槽验收，执行器必须消费 command.slot | [contracts.schema.json](docs/contracts.schema.json)、[MemoryPolicy](docs/MemoryPolicy.md)、[ExecutionProtocol](docs/ExecutionProtocol.md)、[TypeRegistry](docs/DataStructure/TypeRegistry.md)、[Verification](docs/DataStructure/Verification.md)、[ConsciousnessState](docs/DataStructure/ConsciousnessState.md)、[Acceptance](docs/Acceptance.md) | [Ayanami 同意及条件](review/D05-consciousness-criterion.response.md) | 设计与实现已同步，集成复查中 |
| D06 | 日历跳过缺少可审计事件；新增 scheduled_job.skipped，原子推进且禁止自触发 | [ExecutionProtocol](docs/ExecutionProtocol.md)、[TypeRegistry](docs/DataStructure/TypeRegistry.md)、[ChangeEvent](docs/DataStructure/ChangeEvent.md)、[ScheduledJob](docs/DataStructure/ScheduledJob.md)、[JobRun](docs/DataStructure/JobRun.md)、[Acceptance](docs/Acceptance.md) | [Ayanami 同意及条件](review/D06-calendar-skip.response.md) | 设计与实现已同步，针对性测试通过 |
| D07 | Context Schema 将本地 Manifest 放入模型请求，造成自哈希冲突和重复 Schema；明确 wire 与外部 Manifest 边界 | [contracts.schema.json](docs/contracts.schema.json)、[Context](docs/DataStructure/Context.md)、[ContextManifest](docs/DataStructure/ContextManifest.md)、[MemoryPolicy](docs/MemoryPolicy.md)、[Acceptance](docs/Acceptance.md) | [Ayanami 同意及条件](review/D07-context-manifest.response.md) | 设计与实现已同步，真实回放复验中 |

## 模型接入

本轮按 Master 指定使用 OpenCode **Go** 订阅的 `gpt-5.6-luna`。

- 当前核实的 Responses endpoint：`https://opencode.ai/zen/go/v1/responses`。
- 客户端使用自身 User-Agent，并为同一会话发送稳定的 `x-opencode-session`。
- 明确引用本地 secret 文件，不扫描其它账户凭据；阶段 M1—M3 只允许合成资料外发。
- 模型输出先执行完整本地 Schema 和业务校验，再进入业务事务。
- 协议来源：[OpenCode Go 官方文档](https://opencode.ai/docs/go/)，本轮核对日期为 2026-09-14。真实调用尚待适配器完成后验证。

## 工作记录

以下时间使用 Asia/Hong_Kong；仅记录实际完成或已开始的工作。

### 2026-09-14 · 启动与设计核对

- 确认项目根目录已有设计包、`resource/` 和 `review/`，尚无现成服务工程。
- 阅读整体设计、数据流、工程架构、实施路线、接口、验收、存储、执行、记忆、安全和运行规范。
- 明确本轮按 M1→M2→M3 完成；30 天模拟时间不能称为 30 天真实运行，至少 2 小时本机测试也不能用虚拟时钟替代。
- 查询 OpenCode Go 最新官方文档，确认 Luna 使用 Go 的 Responses 路由及会话 header 要求。
- 确认本机 Go 工具链和 GitHub CLI 可用，GitHub 当前登录账户为 `ExplodeCode6324`；尚未创建或发布仓库。
- 启动基础模块和运行模块的并行实现，约定共同 module 名 `secretarysimplified` 及 Store/DTO/时钟接口。
- 本地 Hermes Ayanami 已完成真实握手，见 [握手原始回复](review/00-ayanami-handshake.md)。此记录只是复核连接证据，尚无代码模块复核通过结论。
- 建立 `src/` 包目录及初步显式配置实现，开始生成严格 DTO 与固定依赖；这些改动仍在实施中，未宣布通过验收。
- 建立 `.gitignore`，排除资源密钥、私有配置、运行状态和凭据文件。
- 按 Master 追加要求创建本 README，集中呈现实施计划、进展与复核入口。
- 收到 Master 关于实施设计缺陷的处理授权，增加本页缺陷登记表；已同步给实现与复核子 agent。

### 2026-09-14 · 模块实现与首轮测试

- 基础模块测试与 race 检查通过，证据：[M1 基础报告](reports/implementation/M1-foundation.md)。独立复核仍在进行，尚未宣布 M1 整体通过。
- 完成 SQLite 一致性在线备份、对象 manifest 校验、恢复冻结和 doctor 基础；并发备份及损坏恢复拒绝测试通过，证据：[诊断报告](reports/implementation/M3-diagnostics.md)。
- 调度运行模块已进行通知/验收、租约、取消、预算、DST、路径与 WAIT 等针对性测试，Ayanami 复核中；尚不等于 A11—A25 完整通过。
- Core、模型适配、Context/记忆与 CLI 已进入集成修复阶段。测试发现的问题在修复，不将“可编译”当成“可交付”。
- D01 经 Ayanami 商议通过，已同步设计文档，具体路径和结论见上表。

## 当前验收状态

| 证据类别 | 状态 | 说明 |
|---|---|---|
| DOC | PASS | 最终修订后检查器通过，见 docs/checks/latest-report.json |
| OFFLINE | 全仓 race/vet 与双进程测试通过 | 逐项验收映射仍在收口 |
| LIVE_MODEL | BLOCKED | 第三轮前 26 点通过后触发五小时额度上限；无完整月回放 PASS |
| REAL_USE | 未开始 | 等完成交付后由 Master 接入真实数据 |
| Ayanami 代码复核 | 最终复查 BLOCKED | 既有结论保留；三项终审未形成裁决，见 review/final-pending.md |
| GitHub 发布 | 已完成首次发布 | 公开仓库包含源码、二进制、空库、设计和合成测试证据 |

## 目录

```text
Secretary_Simplified/
├── README.md       实施计划、工作记录、交付与复核入口
├── docs/           设计基线、Schema、DDL 和文档检查器
├── src/            Go module、实现与测试（实施中）
├── release/        二进制、CLI、配置、空库及本地部署状态（集成中）
├── resource/       本地接入资源；密钥文件不公开
├── review/         本地 Ayanami 的真实复核与修复记录
└── scripts/        构建、验证和部署辅助脚本
```

构建、启动、类型化操作及备份命令见 [release/README.md](release/README.md)，接口格式见 [release/API.md](release/API.md)。真实数据接入仍待 M1—M3 验收收口。

### 2026-09-14 · 真实模型与复核修复

- OpenCode Go Luna 真实合成探针成功；首次自然语言创建事项也已经实际 Core/Runner 落库，日期从 +08:00 正确保存为 UTC。
- 第一轮月回放在 18 个事项检查均通过后停止：6 次意识生成失败，原因包括返回 Schema 而非数据实例、输出超限。失败证据保留，第二轮从新库重跑，不覆盖原报告。
- Ayanami 最初使用本地默认 DeepSeek；后端连续连接失败后，保留同一 Ayanami 身份与会话，单次改用 gpt-5.6-luna 恢复复核。身份和持久配置未替换，失败记录保留。
- 根据独立复核补强执行许可/命令/run 绑定、旧 fence 回执拒绝、取消后对账、恢复凭据初始化、严格备份 manifest 和诊断未知状态。已修项仍须按模块复查。
- DOC 检查器已实际运行 PASS；它不代替运行时验收。两小时真实进程采样正在隔离 fixture 环境执行，未完成前不记为通过。
- 构建与启动说明见 [release/README.md](release/README.md)，当前仍是实施版本。

### 2026-09-14 · 最终契约复验

- D06、D07 均为实施过程中发现的缺陷，已与 Ayanami 商议并同步修改上述设计文档。
- 最新全仓 `go test -race ./...`、`go vet ./...` 与实际 Core/Runner 进程测试通过，源码指纹见 [runtime 最终检查](review/M3-runtime-final-checks.json)。
- 第二轮真实月回放在第 53 个索引之前出现意识输出截断，随后无效证据哈希导致更新被严格拒绝；已增加路径级重试反馈和精简输出约束。第三轮使用新数据库执行，所有历史失败保留在本地报告。
- 两小时进程采样仍属于较早的中间构建，不能替代最终二进制的稳定性验收。

### 2026-09-14 · 本机部署与公开材料收口

- 最终源码的 `go test -race ./...`、`go vet ./...` 和 DOC 检查通过。两个 release 二进制已重新构建，内置 smoke、双进程/Core 离线提醒测试重新通过。证据见 [最终离线检查](reports/implementation/final-offline.json) 与 [逐项验收矩阵](reports/implementation/acceptance-matrix.md)，不表示全部验收项通过。
- 补齐公共 typed API、列表过滤/分页、任务取消、计划 PATCH、CLI trigger 版本控制、通知确认、稳定 JSON 输出及内置 smoke。类型化调用深拷贝请求，重试保持 ID 与原 Item 响应版本；错误响应至少为 HTTP 400。
- 第三轮真实模型前 26 个检查点通过，随后遇到 `GoUsageLimitError`。服务商明确返回五小时额度用尽、Retry-After=10378 秒（约 2 小时 53 分钟）。未启用余额付费；前三轮失败记录与合成输入/输出保留在 [reports/live-model](reports/live-model)。
- Ayanami 三项终审受同一额度限制，没有最终裁决。续审会话和增量清单见 [review/final-pending.md](review/final-pending.md)。D01—D07 已形成的有效设计商议结论仍保留。
- 新构建已重新开始两小时本机采样，每轮用两个独立 CLI 重复提交同一请求，核对唯一 Item/提交、队列和心跳；旧构建采样已停止保留，不累计时间冒充最终构建通过。
- 当前尚未到通知 Master 接入真实数据的验收节点。额度恢复后仍需完整月回放、补充语义场景、Ayanami 终审及最终构建稳定性检查。
- 公开仓库首次发布完成，初始提交 `4b82b36`。发布前检查 1,145 个文件，未发现提供的实际 key、本机用户目录路径或私有运行目录；使用 GitHub noreply 提交身份。服务商提示的额度恢复时间约为 2026-09-14 07:00（Asia/Hong_Kong），实际以届时响应为准。
