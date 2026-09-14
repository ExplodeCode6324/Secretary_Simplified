# Secretary_Simplified 设计文档包

版本：1.0；日期：2026-09-14；状态：可交付实施的设计基线，程序尚未实现和验收。

本目录中的 Secretary 均指 Secretary_Simplified。本包依据 Master 本轮要求重写，保留 [Design2.md](Design2.md) 原稿供溯源。旧 Secretary、Secretary_Simple 的文档及实验只提供经验，不属于本项目的实现依赖或通过证据。

## 阅读顺序

| 文档 | 内容 |
|---|---|
| [design.md](design.md) | 目标、业务范围、抽象架构、设计原则 |
| [DataFlow.md](DataFlow.md) | 从输入到事实、上下文、任务和回执的数据流与事务边界 |
| [DataStructure/INDEX.md](DataStructure/INDEX.md) | 模块数据契约、模型输入输出、字段与扩展规则 |
| [MemoryPolicy.md](MemoryPolicy.md) | World Model、24 小时意识更新、会话与 Context 组装 |
| [EngineeringArchitecture.md](EngineeringArchitecture.md) | 进程、Go 包、依赖、部署和并发 |
| [Storage.md](Storage.md) | SQLite 表结构、归属、迁移与一致性 |
| [ExecutionProtocol.md](ExecutionProtocol.md) | 调度、执行、授权、取消、重试、等待、验收 |
| [Interfaces.md](Interfaces.md) | CLI、本机 API、错误及示例 |
| [Security.md](Security.md) | 输入信任、权限和数据外发边界 |
| [Operations.md](Operations.md) | 配置、诊断、恢复、备份和运行限制 |
| [Acceptance.md](Acceptance.md) | 可复现验收矩阵与证据格式 |
| [Milestone.md](Milestone.md) | 对齐 Master 六步路线及阶段退出条件 |
| [ImplementationHandoff.md](ImplementationHandoff.md) | 交给实施 agent 的工作顺序与完成标准 |
| [Decisions.md](Decisions.md) | 已给定约束、工程默认和后期决策 |
| [Review.md](Review.md) | 本次文档自审结果及尚未验证的边界 |
| [References.md](References.md) | 设计来源和技术依据 |

机器可读附件：[SQLite DDL](schema.sql)、[JSON Schema](contracts.schema.json)、[示例目录](examples/README.md)、[文档检查器](checks/validate_docs.py)。它们是设计附件，不是已完成的服务代码。

## 规范解释

必须表示实施和验收约束；默认表示可由配置调整、需记录变更的工程选择；后置表示当前阶段不得作为缺失项阻塞交付。数据字段以 DataStructure 和 Schema 为准，物理约束以 schema.sql 为准，行为以对应专题为准；发现冲突必须修订并复核，不能自行选取较宽松版本。

阶段 1—3 在隔离测试目录、合成输入和受控本机能力范围内自主完成。阶段 4 才接入 Master 的真实资料与使用习惯。真实模型测试若缺少合法可用的凭据，记录为未执行；离线完成不能改称真实模型通过。

D09（实施过程中发现的检索命中被预算裁空问题）：经 Ayanami 的 DeepSeek 条件同意，修改 [MemoryPolicy.md](MemoryPolicy.md)，明确已服务标记、零命中信封、历史锚点、权威依赖、ref 级省略口径与淘汰边界。设计商议不等于程序验收，裁决记录为 `review/D09-retrieval-budget-deepseek-resume1.response.md`。


## D12 实施缺陷修订索引

派生输出分类丢失缺陷按 Ayanami D12 最终裁决及注入补充闭合；无 DDL。修改：contracts.schema.json、Security.md、MemoryPolicy.md、DataFlow.md、Acceptance.md、Interfaces.md、Operations.md；DataStructure/Common.md、INDEX.md、ConversationEvent/ConversationState/ConsciousnessState/Item/Task/ScheduledJob/Command/WorldUpdateProposal/WorldFact/ModelCallRecord/Notification/RuntimeRecords/JobRun/ExecutionAttempt/ExecutorReceipt/ObjectRef/RequestReceipt/InputTurn/ContextManifest/ChangeEvent.md。裁决位于 review/D12-output-class-final-contract.response.md 与 review/D12-injection-oracle-clarification.response.md；实现和验证证据分层，索引不表示安全验收已通过。
