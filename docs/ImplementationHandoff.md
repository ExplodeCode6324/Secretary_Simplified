# 后续实施交接

## 1. 开始时读取

按 [README](README.md) → [Architecture](Architecture.md) → [Decisions](Decisions.md) → [PiIntegration](PiIntegration.md) → [ImplementationMap](ImplementationMap.md) → [PLAN](PLAN.md) 顺序读取，随后按任务查 Execution/MemoryContext/Security/DataContracts。不要使用 Git 历史里的旧 Go 版文件填补新版接口。

本设计已经提供：目标代码文件、关键函数职责、状态/事务/权限/恢复语义、30 个 DTO、18 个工具参数契约、空库 DDL、默认配置、4 个系统模板、30 个应用验收场景、实际 Pi/Node 探针和类型蓝图。[RuntimeProtocol](RuntimeProtocol.md) 补齐初始化、直接聊天、工具入口、事件与具体执行算法。应用目录尚未创建，阶段状态不能继承研究 PASS。

## 2. 第一批实施任务

1. 确认 cwd 和 git status，保留 Master 其它改动；按研究步骤还原/验证 pi_src_origin 与依赖锁。
2. 运行附件检查、上游探针和蓝图 typecheck，记录机器与版本。出现漂移先修复锁和决策，不直接升级到 latest。
3. 按 ImplementationMap 初始化根 TypeScript 工程、测试入口与 fixture 模型。将 Schema/SQL 复制或生成到生产资源，校验其源 hash；docs 的运行示例不当作生产目录。
4. 实现 Storage/ObjectStore/TX1，以及 CLI init/submit/query；验证“接受后立即 kill 再查询”与相同 ID 冲突。
5. 实现任务/授权/预算/outbox、Pi runtime/journal、子只读工具、验收和回复，完成第一条纵向合成闭环。
6. 按 P2 用例补失效/并发/故障；然后依阶段接真实模型、隔离命令、事实记忆、增量整理与退出。

## 3. 文件/生成源关系

- `docs/contracts/*.json` 是 v1 设计接口来源。`contracts.schema.json` 定义 DTO；`tools.schema.json` 定义模型可传工具参数；registry 定义角色工具集合。
- `schema.sql` 是版本 001 的空库基础；实现迁移资源必须与其 checksum 对应。若设计改变先更新文档和用例，再生成迁移。
- `defaults.json` 是默认数值来源，启动配置允许覆写的字段需白名单和审计；模型不能覆写。
- `state-machines.json` 的边只声明允许候选，业务 guard 需实际编码并测试；不能把“边存在”视为授权。
- `acceptance.json` 给应用场景和目标测试文件。创建测试后以实际 run report 记录 PASS/FAIL；设计清单保留要求，不覆盖失败历史。
- `prompts/` 是系统模板源码，构建时记录 hash；动态状态通过字段组装，不把外部材料拼进系统权限段。

## 4. 自主工作与必须暂停的边界

可独立推进：合成数据、fixture、接口/存储/恢复实现、隔离临时目录测试、文档与确定性回归。不需要 Master 为表名、函数名或已经明确的工程细节逐项确认。

真实模型凭据、个人资料外发、超出既有授权的真实写操作、公开服务和设备控制需要相应已批准配置；缺少时把该项标为 PENDING，继续其它独立阶段。绝不把缺少真实授权解释为无法完成整个离线构建，也不虚报 live 验收。

停止条件：Schema/源码版本与设计不符、存储不能可靠确认、权限/隔离无法强制、未知写动作缺乏核查策略、必要 Context 无法容纳。保留证据，给出具体差异和推荐决策；不通过静默扩大权限、丢弃要求或改验收阈值绕过。

## 5. 每批完成后交付

报告具体行为变化、相关设计 ID、已运行命令和结果、证据类别、尚未覆盖边界、启动/停止/恢复方法。阶段通过需验收报告链接；提交源码前排除 node_modules、pi_src_origin、临时数据、私有配置和 token。是否推送、部署和真实数据接入遵守该轮 Master 指令，不从设计完成推断授权。
