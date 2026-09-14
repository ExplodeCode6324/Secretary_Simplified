# Secretary_Simplified

面向 Master 的持久化文字助手。依据 `docs/` 中的设计基线，以 Go 实现 Core、Scheduler、Executor、四层记忆与本地 CLI，首先在 Mac 上使用合成数据完成验证，再由 Master 接入真实资料。

> 当前状态：**代码、release、公开仓库、审计修复与指定 DeepSeek 独立复核已完成，可以接入受控真实文字进行下一阶段测试**。原 final2 构建实际运行 7,200.010 秒、240 次采样、零失败；新修复版另有全仓竞态/静态检查和短时端到端验证，依 Master 验收口径未重跑两小时。真实来源文件同步尚未开放，接入范围见 [RealDataTrial.md](docs/RealDataTrial.md)。

公开仓库：[ExplodeCode6324/Secretary_Simplified](https://github.com/ExplodeCode6324/Secretary_Simplified)。本机可运行版本位于 `release/`；最新离线与构建证据见 [final2-offline.json](reports/implementation/final2-offline.json)，逐项结论见 [验收矩阵](reports/implementation/acceptance-matrix.md)。

## 本轮交付范围

1. 在 `src/` 完成一个 Go module，构建 `secretaryd`（Core/Runner 两种常驻模式）及 `secretary` CLI。
2. 在 `release/` 部署本机二进制、配置、初始化数据库和运行目录，提供启动与诊断说明。
3. 每个模块完成后交给本地 Hermes **Ayanami** 复核，真实复核结果保存在 `review/`，修复后记录复查结果。
4. 在 GitHub 创建公开的 `Secretary_Simplified` 仓库，发布可公开的项目文件。
5. 完成已授权的合成数据验证后通知 Master，说明真实数据接入所需范围与披露授权。

**发布边界：** API key、认证凭据、本地私有配置、运行数据库与私人原文不进入公开仓库。交付配置模板、数据库初始化方式及可公开的空库/部署材料；本地已部署的运行状态单独保留。资源目录中 Master 提供的密钥只用于明确配置的模型调用，不写入日志或复核报告。

**复核模型要求（Master 于 2026-09-14 追加）：** Hermes Ayanami 必须使用 DeepSeek v4.1 Flash。此前因连接失败改用 Luna 的报告保留为历史证据，不能代替指定模型的最终复核；不再自动回退到 Luna。Secretary 自身实现测试仍使用 GPT 5.6 Luna。

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
| M1 | 严格 DTO/Schema、迁移、ObjectStore、事项 CAS/依赖、来源同步、请求幂等与事件事务 | A01—A05、A19、A24；对应 Go 测试与复核 | 实现、定向验证及独立复核完成 |
| M2 | WorldCommit 权限、事实历史/纠正/冲突、四层记忆、24 小时意识槽、有界 Context、模型适配器 | A06—A10、A16、A17；离线与真实模型分开报告 | 实现、真实合成模型链及独立复核完成 |
| M3 | Core/Runner、Scheduler/Executor、固定验收、取消/等待/重试/恢复、CLI/诊断/备份 | 适用 A01—A25；进程集成、竞态、故障恢复测试 | 实现、审计修复、独立复核和原 final2 的 A25 已完成 |
| 验证 | 72 小时虚拟时钟、30 天模拟回放、至少 2 小时真实本机运行、合成资料真实模型评估 | 带构建与输入哈希的报告；所有失败及重试保留 | 新修复 race/vet、定向回归、发布包短冒烟通过；原 final2 两小时 240 次采样零失败，两组构建证据分开 |
| 部署 | 构建两个二进制，初始化本地 release，核验新目录使用流程 | release 使用说明、校验和、doctor 与完整链路 | macOS arm64 已构建并初始化，fixture 双进程链路通过 |
| 发布 | 凭据与私有路径检查、公开仓库创建、上传与远端同步核验 | 仓库链接、提交 ID、发布清单 | 公开仓库已创建并推送；凭据与私人运行资料排除 |
| 交接 | 汇总通过、失败、未运行及限制，通知 Master 接入真实数据 | 可复核交付报告与接入事项 | M1—M3 交付完成；等待 Master 提供受控文字及允许外发的服务商/资料范围 |

新增 [上线前审计 issue #1](https://github.com/ExplodeCode6324/Secretary_Simplified/issues/1) 的对象原子发布/孤儿恢复、取消后持久结算、准入拒绝归档和来源读取上限均已修复并定向核证。issue 的静态发现与实际故障注入结果分开记录。真实接入选用仅 PERSONAL 文字和独立数据目录方案，边界说明在 [docs/RealDataTrial.md](docs/RealDataTrial.md)，实施修订同步 [docs/Operations.md](docs/Operations.md)。原 A25 报告保持原构建哈希，新修复回归另行记录于 [issue1-regression.json](reports/implementation/issue1-regression.json)。

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
| D01 | 计划登记要求原子事件，但事件注册表缺 ScheduledJob 类型；新增 scheduled_job.updated，明确业务修订与扫描推进边界 | [docs/DataStructure/TypeRegistry.md](docs/DataStructure/TypeRegistry.md) | [Ayanami 同意及附带条件](review/D01-scheduled-job-event.response.md) | 实现、设计同步及 DeepSeek 复核完成 |
| D02 | 周期计划静态通知键导致后续运行复用旧通知；按 occurrence 无歧义编码并先绑定实例验收条件 | [ExecutionProtocol](docs/ExecutionProtocol.md)、[DataFlow](docs/DataFlow.md)、[ScheduledJob](docs/DataStructure/ScheduledJob.md)、[Task](docs/DataStructure/Task.md)、[Notification](docs/DataStructure/Notification.md)、[TypeRegistry](docs/DataStructure/TypeRegistry.md) | [Ayanami 同意及条件](review/D02-notification-occurrence.response.md) | 实现、设计同步及 DeepSeek 复核完成 |
| D03 / D04 | 有效删除页无法使用 normalized=null；明确重复页不新增对象引用的验收口径 | [contracts.schema.json](docs/contracts.schema.json)、[SourceRecord](docs/DataStructure/SourceRecord.md)、[Acceptance A05](docs/Acceptance.md) | [Ayanami M1 复核](review/M1-foundation-ayanami.md) | 实现、设计同步及 DeepSeek 复核完成 |
| D05 | memory.refresh 缺少可独立验证的完成条件；新增由槽控制器生成的精确槽验收，执行器必须消费 command.slot | [contracts.schema.json](docs/contracts.schema.json)、[MemoryPolicy](docs/MemoryPolicy.md)、[ExecutionProtocol](docs/ExecutionProtocol.md)、[TypeRegistry](docs/DataStructure/TypeRegistry.md)、[Verification](docs/DataStructure/Verification.md)、[ConsciousnessState](docs/DataStructure/ConsciousnessState.md)、[Acceptance](docs/Acceptance.md) | [Ayanami 同意及条件](review/D05-consciousness-criterion.response.md) | 实现、设计同步及 DeepSeek 复核完成 |
| D06 | 日历跳过缺少可审计事件；新增 scheduled_job.skipped，原子推进且禁止自触发 | [ExecutionProtocol](docs/ExecutionProtocol.md)、[TypeRegistry](docs/DataStructure/TypeRegistry.md)、[ChangeEvent](docs/DataStructure/ChangeEvent.md)、[ScheduledJob](docs/DataStructure/ScheduledJob.md)、[JobRun](docs/DataStructure/JobRun.md)、[Acceptance](docs/Acceptance.md) | [Ayanami 同意及条件](review/D06-calendar-skip.response.md) | 实现、设计同步及 DeepSeek 复核完成 |
| D07 | Context Schema 将本地 Manifest 放入模型请求，造成自哈希冲突和重复 Schema；明确 wire 与外部 Manifest 边界 | [contracts.schema.json](docs/contracts.schema.json)、[Context](docs/DataStructure/Context.md)、[ContextManifest](docs/DataStructure/ContextManifest.md)、[MemoryPolicy](docs/MemoryPolicy.md)、[Acceptance](docs/Acceptance.md) | [Ayanami 同意及条件](review/D07-context-manifest.response.md) | 实现、设计同步及 DeepSeek 复核完成 |
| D08 | 闹钟、晨报与来源同步缺少可独立验证的准入条件；补充当前运行绑定的会话、产物及来源同步证据 | [contracts.schema.json](docs/contracts.schema.json)、[ExecutionProtocol](docs/ExecutionProtocol.md)、[Verification](docs/DataStructure/Verification.md)、[TypeRegistry](docs/DataStructure/TypeRegistry.md)、[Acceptance](docs/Acceptance.md) | [DeepSeek Ayanami 同意及条件](review/D08-capability-criteria-deepseek.response.md) | 实现、设计同步及 DeepSeek 复核完成 |
| D09 | 检索已命中却被预算全部裁空，零命中与未检索混淆；含问题身份的助手回复被裁掉。明确检索状态、两类记录各一条必要原文及省略口径 | [MemoryPolicy](docs/MemoryPolicy.md) | [初次裁决](review/D09-retrieval-budget-deepseek-resume1.response.md)、[双锚点补充裁决](review/D09-dual-anchor-deepseek.response.md) | 实现、设计同步及 DeepSeek 复核完成 |
| D10 | 实际提醒模型遗漏既有五分钟宽限默认，SKIP/0 导致正常首扫即跳过；限制自然语言提醒策略并拒绝越界，Typed 明示策略保留 | [ExecutionProtocol](docs/ExecutionProtocol.md)、[Interfaces](docs/Interfaces.md)、[Acceptance](docs/Acceptance.md) | [DeepSeek 通道边界同意](review/reminder-defaults-v1-boundary-deepseek.response.md)、[最终范围与重试澄清](review/reminder-defaults-scope-deepseek.response.md) | 实现、设计同步及 DeepSeek 复核完成 |
| D11 | 会话已有待答问题槽位却缺少正常创建/回答入口；补充模型内容提议、程序身份及原会话显式回答的原子生命周期 | [contracts.schema.json](docs/contracts.schema.json)、[DecisionEnvelope](docs/DataStructure/DecisionEnvelope.md)、[InputEnvelope](docs/DataStructure/InputEnvelope.md)、[ConversationState](docs/DataStructure/ConversationState.md)、[InputTurn](docs/DataStructure/InputTurn.md)、[MemoryPolicy](docs/MemoryPolicy.md)、[Interfaces](docs/Interfaces.md)、[DataFlow](docs/DataFlow.md)、[Acceptance](docs/Acceptance.md) | [DeepSeek 同意及条件](review/D11-pending-question-deepseek.response.md) | 实现、设计同步及 DeepSeek 复核完成 |
| D12 | 派生输出分类跨会话丢失，及普通输入硬编码合成；统一程序标记、单调传播、旧未知拒绝、输入默认 PERSONAL | [完整设计修改路径索引](docs/README.md#d12-实施缺陷修订索引)、[Security](docs/Security.md)、[Interfaces](docs/Interfaces.md)、[Operations](docs/Operations.md) | [最终裁决](review/D12-output-class-final-contract.response.md)、[注入澄清](review/D12-injection-oracle-clarification.response.md) | 实现、全仓 race/vet 与两组 DeepSeek 独立复核完成 |

## 模型接入

本轮按 Master 指定使用 OpenCode **Go** 订阅的 `gpt-5.6-luna`。

- 当前核实的 Responses endpoint：`https://opencode.ai/zen/go/v1/responses`。
- 客户端使用自身 User-Agent，并为同一会话发送稳定的 `x-opencode-session`。
- 明确引用本地 secret 文件，不扫描其它账户凭据；阶段 M1—M3 只允许合成资料外发。
- 模型输出先执行完整本地 Schema 和业务校验，再进入业务事务。
- 协议来源：[OpenCode Go 官方文档](https://opencode.ai/docs/go/)，本轮核对日期为 2026-09-14。已通过真实合成调用及完整月回放。

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
| LIVE_MODEL | 部分场景已通过，最终复验中 | 第四轮 90 个事项检查点与 30 次意识生成全部通过；第五轮按最新设计重跑。完整语义范围见 A23 补充证据 |
| REAL_USE | 未开始 | 等完成交付后由 Master 接入真实数据 |
| Ayanami 代码复核 | DeepSeek 终审中 | DeepSeek v4.1 Flash 已恢复；历史 Luna 报告不作为指定模型终审，见 review/final-pending.md |
| GitHub 发布 | 已完成首次发布 | 公开仓库包含源码、二进制、空库、设计和合成测试证据 |

## 目录

```text
Secretary_Simplified/
├── README.md       实施计划、工作记录、交付与复核入口
├── docs/           设计基线、Schema、DDL 和文档检查器
├── src/            Go module、实现与测试
├── release/        二进制、CLI、配置、空库及本地部署状态
├── resource/       本地接入资源；密钥文件不公开
├── review/         本地 Ayanami 的真实复核与修复记录
└── scripts/        构建、验证和部署辅助脚本
```

构建、启动、类型化操作及备份命令见 [release/README.md](release/README.md)，接口格式见 [release/API.md](release/API.md)。真实数据接入仍待最终构建的运行验证收口。

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

D09 检索预算修复经 Ayanami 使用 DeepSeek 条件同意，E1—E6 实施条件已补充代码与测试，正式修改文档为 [docs/MemoryPolicy.md](docs/MemoryPolicy.md)。设计裁决见 [D09 DeepSeek 回复](review/D09-retrieval-budget-deepseek-resume1.response.md)，程序最终独立复核仍另行记录；不得将设计同意等同于代码验收。原失败与修复回放保留在本地证据目录，提案原文继续留档。


### 2026-09-14 · 额度恢复、指定复核模型与官方备用接入

- 按 Master 指定恢复 Hermes Ayanami 的 DeepSeek v4.1 Flash 复核，保存实际握手及独立设计复验。此前 Luna 报告标明历史模型，不作为 DeepSeek 终审结论。
- 第四轮真实模型月回放通过 90 个事项检查点及 30 次每日意识生成。原报告的历史 ID 映射引用问题单独形成更正审计，使用不可变事件与各提交时点重建，不覆盖原始报告；第五轮使用最新 Schema 和检索规则重新执行。
- D08 补充闹钟、晨报、来源同步的固定验收条件，并经 DeepSeek 同意修订合法 REPLAN 后的当前运行选择规则；修改文档路径见上表及 [REPLAN 补充裁决](review/D08-replan-current-run-deepseek.response.md)。D09 修复检索命中被全部裁空的问题，保留已检索但零命中的明确状态；修改 [docs/MemoryPolicy.md](docs/MemoryPolicy.md)。
- 修复 DST 离线跨缺失时刻的跳过审计遗漏，以及远端 FAILED 回执的 effect_observed 被 Runner 覆盖的问题；对应正反例、全仓 race/vet 与 DOC 检查通过，独立代码终审继续进行。
- Master 提供官方 DeepSeek key，并明确仅在再次出现 OpenCode 429 时启用。已建立独立官方配置与凭据文件，公共模板为 [release/config.deepseek.example.json](release/config.deepseek.example.json)。截至本记录未出现新的真实 429，没有切换生产测试服务商或 Ayanami provider。
- 官方配置使用 Responses API 的 `deepseek-flash` 模型 ID；模拟 HTTP 的跨服务商持久状态、同键重试和 429 无副作用测试已通过。这只是机械切换证据，不能称为官方 API 实际调用通过。若条件触发，Secretary 与 Ayanami 同步切换，保留原失败并记录真实切换结果。
- 旧 release 构建完成 7,200 秒实际进程采样、240 次检查、零失败；本次生产修改改变二进制，因此将重新构建并重新计时，不把旧证据冒充新构建通过。

### 2026-09-14 · 提醒全链与待答问题补全

- 实际 CLI 第一轮暴露默认提醒规则未传入模型，SKIP/0 在首扫迟到约 0.9 秒后跳过。原失败保留为 [CLI run1](reports/live-model/cli-run1/report.json)。D10 经 DeepSeek 商议后增加自然语言通道默认策略约束，Typed 显式策略原样保留；同场景 [CLI run2](reports/live-model/cli-run2/report.json) 通过，恰好一次通知、Item OPEN、同请求不重复，Core 关闭后仍可确认通知。
- D11 经 DeepSeek 同意补全待答问题生命周期：模型仅提问题文本与已有 Item 引用，程序分配身份和生成序号；原 session 中的明确 ID 回答与成功决策同事务提交，不凭摘要猜目标，不将问题解决等同事项 DONE。正式修改路径见缺陷表。
- 意识与会话摘要新增独立的 `reports/memory_attempts` 业务校验结果：供应商 SUCCEEDED 与程序 INVALID_REFERENCE/UNKNOWN_PENDING_QUESTION 分开记录，关联原模型 call ID，原因使用固定码，不输出原文或私有路径。针对性测试通过；历史真实失败没有被覆盖。
- DeepSeek 对其他模块的定向复核未发现新的必须修复项，实际模型来源见 [model-provenance.json](review/model-provenance.json)。D11 与记忆诊断增量仍需复核后冻结最终构建。
- 第五轮月回放同样通过 90 个检查点及 30 次意识生成，公开证据和独立身份审计已归档。组合场景的逐项固定输入、oracle 哈希、覆盖条款和完整失败史见 [A23 统一索引](reports/implementation/A23-coverage-ledger.md)，不把新建立的索引伪称为历史冻结文件。

### 2026-09-14 · 跨会话链路与派生数据分级补查

- 第六轮真实月回放通过 90 个事项检查点及 30 次意识生成；[公开原始报告](reports/live-model/month-run6/report.json)与独立审计保留。此轮尚不包含之后的 D09 双锚点和派生分级修复。
- [待答问题真实 CLI run1](reports/live-model/question-run1-failed/report.json)成功创建问题，但跨会话检索时裁掉含程序问题 ID 的助手回复，最终耗尽检索预算；完整失败未覆盖。D09 补充裁决要求分别保留带引用和不带引用记录的最新一条，原 32,000 字节预算不变；修改路径为 [docs/MemoryPolicy.md](docs/MemoryPolicy.md)，新构建重跑待执行。
- D11 空白文字校验及独立记忆业务结果记录已获 DeepSeek 补充同意；HTTP 受理前拒绝与已受理输入的异步失败边界补充于 [docs/Interfaces.md](docs/Interfaces.md)。
- D12 发现记录：隔离 fake 模型测试确认 PERSONAL 上下文生成的助手回复可被当前 SYNTHETIC 输入标签降级，并经跨会话检索送入仅允许 SYNTHETIC 的请求。只使用合成 canary、无网络、无真实数据；[复现证据](review/D11-output-class-probe-result.md)。后续最终裁决与修复见缺陷表及下文。

D12 已按 Ayanami 最终裁决实施，初稿的扩展字段限制及排除 Item/任务的范围判断已被纠正并明确替代。问题/Item/摘要/意识、运行链/事实版本及注入/默认输入分类定向测试、全仓 race/vet 均通过。旧派生记录缺标保持未知并拒绝披露，无自动回填或清库。合成脚本显式声明 SYNTHETIC，原自然语言与 oracle 不变；真实 CLI 的分类冒烟测试 5/5 通过，证据见 [D12-cli-class-smoke.json](reports/implementation/D12-cli-class-smoke.json)。

待答问题 run2 已取回正确程序问题 ID，但模型持续附带重复 READ_MEMORY 导致预算耗尽，失败保留。仅补充模型提示明确检索足够时最终 controls=[]，不自动接受带 control 的最终答案、不改预算；[Ayanami 确认](review/READ_MEMORY-final-answer-clarification.response.md)。最终真实链仍待重跑。

### 2026-09-14 · 最终构建与真实合成回放

- D12 元数据与提示增长后，问答 run3 和月回放 run7 出现 CONTEXT_REQUIRED_OVERFLOW；不是 429。全部失败公开保留。精简重复提示语，并将 40 处相同 Schema 扩展定义提取共享引用，展开后完整结构等价；32,000 字节上限、必需原文与 oracle 不变。正式说明在 [Common.md](docs/DataStructure/Common.md)，[独立等价复核](review/final-schema-prompt-deepseek.response.md) 与 [可复算数字基准](reports/implementation/registered-extensions-equivalence.json) 分开记录。
- [待答问题 run4](reports/live-model/question-run4/report.json) 完整通过，包含程序问题创建、跨会话原 ID 检索、重启后原会话显式回答、同请求幂等、新请求已解决冲突；Item 内容不变。此前 run1–3 失败原样保留。
- [月回放 run8](reports/live-model/month-run8/report.json) 90 个事项检查点全部通过，实际持久化 slot 0–29 共 30 个意识快照，独立历史身份与引用审计通过，公共材料可以复现。组合语义场景使用整套 month8 + scenarios4 + world2，129/129；最新 [A23 索引 v2](reports/implementation/A23-coverage-ledger-v2.md) 保留全部失败历史及原 v1，不声称事后索引曾在调用前冻结。
- D12 [Core/Memory 终审](review/D12-core-memory-deepseek-final.response.md) 与 [Runtime/World 终审](review/D12-runtime-world-deepseek-final.response.md) 均未留下必须修复项。最终源码的 race/vet、DOC、构建和真实 CLI 分类测试通过。
- 当前 release 构建校验和为 CLI `e60c1616…` / daemon `6682c739…`。最终两小时隔离 fixture 采样从 2026-09-14 09:33:59（Asia/Hong_Kong）重新计时；先前因生产修改停止的短采样都保留为 SUPERSEDED，不累计时间。

### 2026-09-14 · Master 上线前审计 issue #1

- AUD-01/AUD-03：跨进程固定发布锁、完整临时写与同步、hard-link no-replace 原子发布，正式引用和准入业务共事务；损坏孤儿隔离重建，已提交损坏保持拒绝并保留证据。队列拒绝及 Typed 实际写业务后失败均验证全量回滚；[存储报告](reports/implementation/issue1-storage.md)、[逐项清单](reports/implementation/issue1-storage-checklist.json)、[独立终审](review/issue1-storage-final.response.md)和 [Typed 回滚补审](review/issue1-storage-typed-final.response.md)。实施发现与协议修改路径：[Storage.md](docs/Storage.md)、[DataFlow.md](docs/DataFlow.md)、[ObjectRef.md](docs/DataStructure/ObjectRef.md)、[Operations.md](docs/Operations.md)。
- AUD-02/AUD-04：实际复现取消后 CoreWork 无回执残留；修复有限收尾、同代未知结果对账和不重复预算结算，简报对象/回执原子提交、意识精确槽恢复。来源在读取阶段限为 1 MiB+1，超限回执明确且不推进旧游标。实际 socket/HTTP 故障注入与重开数据库验证通过；[运行时报告](reports/implementation/issue1-runtime.md)、[独立终审](review/issue1-runtime-final.response.md)。同步设计路径：[ExecutionProtocol.md](docs/ExecutionProtocol.md)、[Acceptance.md](docs/Acceptance.md)、[设计索引](docs/README.md)。
- 交叉补查修复诊断输出归档的无期限发布锁等待，5 秒独立持久化上下文保留错误传播；不承诺抢占底层 fsync。[定向与独立复核](reports/implementation/issue1-diagnostic-bound.md)；修订路径 [Operations.md](docs/Operations.md)。
- GATE-01/02 选择受控 PERSONAL 文字与独立数据目录；真实来源同步未开放，fixture 不能改指真实文件，未给本机现有 ProviderPolicy 扩权。默认分类、未授权零外发、授权正向、目录隔离和同库保守拒绝均通过实际程序路径验证与指定模型复核；[接入说明](docs/RealDataTrial.md)、[证据](reports/implementation/issue1-gates.md)。
- 全仓 `go test -race ./...`、`go vet ./...`、新二进制构建及 [2.807 秒短时发布包冒烟](reports/implementation/issue1-release-smoke/report.json)通过。新增测试断言另有定向 race，未冒称先前全仓运行已包含后加断言。冒烟脚本初版误按统一 result 包装解析 doctor，已修脚本；失败和纠正后的旧构建基线均在报告历史中保留。
- 新 release 校验和为 CLI `84e24025…` / daemon `1d62faad…`，完整源码与回归证据见 [issue1-regression.json](reports/implementation/issue1-regression.json)。原两小时测试仍使用 `e60c1616…` / `6682c739…` 的固定副本，未动其状态或二进制。按 Master 明确要求，本轮不重跑两小时，不把旧持续运行结论扩大为新构建实测。

### 2026-09-14 · 原持续测试完成与交付

- 原 final2 测试自然完成：实际 7,200.009923696518 秒、240 次采样、零失败；[原始报告](reports/implementation/final2-release-soak/report.json)逐字节归档，[核验记录](reports/implementation/final2-release-soak/verification.json)包含报告、固定二进制及采样器哈希。固定二进制也与公开提交 `2ba8547` 中的文件一致。确认进程身份后，仅停止本次隔离 Core/Runner；采样器自然退出。
- Issue #1 已于修复提交 `88c7818` 完成并回贴证据关闭；新 release 的回归与原持续运行保持各自构建标识。代码、二进制、CLI、空数据库、设计修订、复核及工作记录均已交付。
- 下一步由 Master 提供一组受控真实文字，并明确允许交给哪个模型服务商及外发范围。使用新独立数据目录，真实来源文件同步不开放，既有 ProviderPolicy 不自动扩权；不能用 SYNTHETIC 标签绕过权限。当前交付不声称已经完成真实资料或实际音频/叫醒验证。
