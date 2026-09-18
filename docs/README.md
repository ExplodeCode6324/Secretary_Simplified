# 设计 v1.0 阅读入口

日期：2026-09-18。状态：设计完成，待 Master 复核。范围：单机、一个 Master、一个文本入口、一个逻辑主会话、最多一个运行子任务。

本设计落实 [研究计划](archive/PLAN-2026-09-17.md) 与 [评审 R1—R8](InitialReview.md)。计划所提 9 月 16 日讨论原文未在仓库提供；这里明确记录根据现有计划重建的基线，不宣称恢复了未见原文。旧 Go 设计的固定 24 小时意识快照不是本版默认；本版 Consciousness 是持续增量更新的派生工作记忆。

## Master 建议复核顺序

1. [Architecture](Architecture.md)：目标、边界、部署、正常闭环和不变量。
2. [Decisions](Decisions.md)：工程选择与默认值；尤其 D02、D06、D10、D12 的使用政策。
3. [MemoryContext](MemoryContext.md)：要求/承诺如何保留，事实如何接纳，退出与遗忘的含义。
4. [Execution](Execution.md) 与 [Security](Security.md)：任务修改、授权、取消及外部副作用。
5. [PLAN](PLAN.md)：开发顺序和每一阶段的退出条件。

工程规格与参考：

| 文件 | 负责的契约 |
| --- | --- |
| [PiIntegration](PiIntegration.md) | 固定源码、Agent 适配、模型出口、P1 实验 |
| [DataContracts](DataContracts.md) | 标识、对象语义、校验顺序、数据库与 DTO 映射 |
| [Interfaces](Interfaces.md) | 本地客户端、主/子工具、事件与错误 |
| [Execution](Execution.md) | 状态机、版本、事务、操作许可、回执、恢复 |
| [MemoryContext](MemoryContext.md) | 认知提案、来源、增量整理、一致 Context、预算 |
| [Security](Security.md) | 信任边界、访问和外发、工具隔离、凭据 |
| [Operations](Operations.md) | 启停、备份恢复、迁移、删除、运维观测 |
| [Acceptance](Acceptance.md) | T01—T30、故障注入、语义评测和真实使用 |
| [ImplementationHandoff](ImplementationHandoff.md) | 建议代码边界、第一批工作、停止与升级条件 |
| [ImplementationMap](ImplementationMap.md) | 从零创建的逐文件改动、函数职责、依赖与测试 |
| [RuntimeProtocol](RuntimeProtocol.md) | 初始化、普通聊天/异步回复、内部命令、实体/产物入口、事件与原生资源算法 |
| [DesignReview](DesignReview.md) | R1—R8 闭合关系、自检、仍待运行证明的事项 |
| [References](References.md) | 一手来源与固定 Pi 版本 |
| [JSON Schema](contracts/contracts.schema.json) | 闭合 DTO 定义；不代替授权与业务检查 |
| [SQLite DDL](contracts/schema.sql) | 空库参考结构；不代替运行时提交协议 |
| [状态机](contracts/state-machines.json) | 允许转换与必须执行的业务守卫 |
| [配置默认](contracts/defaults.json) | 可机器读取的工程初值，默认 fixture 模式 |
| [用例清单](contracts/acceptance.json) | 阶段、证据类别和场景的映射 |
| [工具 Schema](contracts/tools.schema.json) / [角色工具表](contracts/tool-registry.json) | 最终工具参数及主/子可见范围 |
| [源码研究与实际探针](research/README.md) | 发布源码定位、依赖锁、可运行测试、类型蓝图 |
| [示例](examples/README.md) | 合法及必须拒绝的对象 |
| [附件检查](checks/README.md) | 可重复运行的 DOC_ONLY 验证 |

## 复核时的关键选择

- 默认允许自动接纳 Master 明确表达、范围清楚的普通事实/偏好；外部观测保留来源和不确定性，授权与敏感政策只由可信程序接口更改。
- 默认原型仅使用合成数据与专用测试目录，真实模型、个人资料、真实写入和网络目标需要明确运行配置与已有授权依据。
- 所有主/子/整理/验收模型调用经过同一出口；无允许的数据分类和提供方配置时不外发。
- 真实资料默认保留到 Master 删除；调试请求正文默认不额外长存。具体保留、删除与备份规则见运维文档。
- P1/P2 先离线，P3 才进入真实模型小闭环；阶段尚未通过，复选框保持未完成。

这些都是完整且可实施的默认设计，复核可逐项调整；无需为了暂未接入的语音、安卓或多客户端先确定全部参数。
