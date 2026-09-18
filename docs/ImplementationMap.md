# 从空仓库构建：文件与改动清单

所有下列 `src/`、`tests/`、`native/` 路径是 **待新建目标**，不是已存在实现。机器契约在 docs/contracts，研究代码在 docs/research；应用构建不得把 docs/research 当作生产运行目录。

## 1. 工程初始化

1. 创建根 package.json（private、ESM、engines Node 24.21.0），锁定 Pi Core/AI 0.85.1、TypeBox 1.3.27、TypeScript 5.9.3；需要完整类型检查时显式加入研究中验证的 MCP optional peer，详见 research/package.json。
2. `tsconfig.json` 采用 strict、module/moduleResolution NodeNext、target ES2023、noUncheckedIndexedAccess、outDir dist。所有相对 import 使用 `.js`，源码为 `.ts`。业务代码不使用 any 躲过契约。
3. 添加 scripts：build（tsc）、typecheck（tsc --noEmit）、test:offline（编译后 node --test）、test:integration、test:live（显式配置才可运行）、check:docs。fixture 单测不得联网。
4. `src/contracts/` 从 docs/contracts 派生类型和校验入口；如果手写类型必须有 DTO fixture 相容测试。运行时固定 `ajv=8.17.1` + `ajv-formats=3.0.1`，使用 Ajv2020、strict=true、strictTypes=false 和 addFormats，与研究探针相同；不能只做 TypeScript 编译检查。
5. 引入空库 DDL 为 `src/storage/migrations/001_initial.sql`，记录原文件 hash。迁移脚本作为 build 资源复制到 dist，禁止依赖 cwd 读 docs。
6. `.gitignore` 保留 pi_src_origin、node_modules、运行数据、构建与凭据排除；根仓库不提交真实状态目录。固定 source/package 锁而非 Git 子仓库副本。

## 2. 模块逐项实现

| 目标文件 | 必须提供的接口/算法 | 直接依赖 | 对应测试 |
| --- | --- | --- | --- |
| src/config/load.ts | loadConfig；闭合 schema、模式、profile、私有目录、绝不读 Pi 全局配置 | contracts/defaults | config-mode、private-paths |
| src/contracts/validate.ts | parseBoundedJson、validateRecord、UUID/UTC/schema 版本/重复键检查 | Ajv/格式检查 | fixtures、duplicate-json-key |
| src/contracts/business.ts | validateScope、validateReferences、compareVersions、verifyCriteriaSet | storage read view | invalid-reference、stale-proposal |
| src/storage/database.ts | DatabaseSync 生命周期、PRAGMA、withImmediateTransaction；禁止异步等待进入事务 | node:sqlite | pragma、rollback、reopen |
| src/storage/migrate.ts | 空库初始化与版本拒绝、迁移 checksum、升级副本试跑 | database | migration-failure |
| src/storage/object-store.ts | putStream/readRange/tombstone；流式上限、hash、fsync/rename/目录 fsync | fs、crypto | truncated-object、orphan-object、deleted-object |
| src/storage/journal.ts | append 原始记录/事件；稳定映射 Pi message/toolCall；有界写队列 | database、objects | event-order、journal-failure |
| src/storage/repositories.ts | request/task/grant/receipt/outbox 的参数化 SQL；CAS 和去重冲突 | database | duplicate-id-hash、FK、atomic-outbox |
| src/core/accept.ts | TX1 acceptInput/control；相同 ID/hash 返回同接收记录 | repositories、objects | T02、T15 |
| src/core/coordinator.ts | 优先级队列、单主推理、输入代际、中止与重组、短事务提交 | accept、runtime、commit | T04、T17 |
| src/core/commit.ts | TX2/T5/T6/T8；提案版本与当前策略重验 | business、repositories | T03、T13、T22 |
| src/core/replies.ts | 普通聊天、派发确认、晚任务通知、kind/basis 去重 | commit、outbox | T22；RuntimeProtocol |
| src/storage/event-types.ts | 闭合事件 payload、记忆白名单、source/role 分类 | contracts | unknown-event、no-memory-loop |
| src/core/recovery.ts | epoch 提升、在途核查、outbox、cursor、过期预算恢复 | repositories、executors | T03、T18、T23 |
| src/core/budget.ts | 根与维护预算预留/结算/未知 usage；幂等 model_call_id | database | concurrent-reserve、unknown-usage |
| src/tasks/service.ts | create/get/list/patch/cancel；不可变 task version；状态图守卫 | commit、budget | state-transitions、task-patch |
| src/tasks/dispatcher.ts | outbox claim/send/ack、attempt 去重、单子任务槽 | runtime、repositories | crash-dispatch、late-result |
| src/tasks/verifier.ts | 确定性 criterion checker + 有证据语义 proposal | object-store、model-gateway | T13、criteria-not-weakened |
| src/policy/grants.ts | grant/revoke/expire、认证主体、scope/class、revision | repositories | T05、grant-expiry |
| src/pi/runtime.ts | Agent 构造、固定工具、explicit sequential、idle 轮换 | Pi Agent Core | research P01—P04/P07/P08 + 集成 |
| src/pi/journal.ts | Pi 事件到业务原始记录 adapter；失败 abort/DEGRADED | storage/journal | persist-before-tool、storage-error |
| src/pi/tools-main.ts | Interfaces 列出的内部工具；同输入根预算 | tasks/cognition/retrieval | no-host-tools、async-dispatch |
| src/pi/tools-child.ts | file/command/web/report 参数定义；execute 再校验 | execution/gateway | mutated-args、child-role |
| src/model/gateway.ts | 受信模型 Context、预算预留、失败错误流、完成结算 | budget、provider adapter | zero-fetch、stream-interrupt |
| src/model/responses.ts | 直接 Pi AI stream、final onPayload、maxRetries=0、受控 fetch | Pi AI Responses | research P05/P06 + 请求快照 |
| src/model/fixture.ts | 可脚本化正常/错误/工具/中止流，无真实传输 | EventStream | 所有离线场景 |
| src/execution/gateway.ts | TX3/TX4、许可消费、最终参数/资源/撤权校验 | policy、budget、repositories | T18/T19、TOCTOU-cancel |
| src/execution/files.ts | 受控打开、有界读、原子写、对象版本证据 | fs、object-store | path-escape、version-changed |
| src/execution/artifacts.ts | attempt 内有界文本对象、分类继承、内部 command 去重 | object-store | create-then-write、classification |
| src/execution/command.ts | 固定 shell、sanitized env、进程组、输出上限、超时 | sandbox | child-process-cancel、disk-output |
| src/execution/sandbox-macos.ts | 生成 deny-default SBPL、自检、不可用即拒绝 | sandbox-exec | denied-path、sandbox-unavailable |
| src/execution/http.ts | GET/HEAD、域名/IP/重定向逐跳限制、有界结果 | URL/DNS、受控 transport | redirect-denied、SSRF、large-body |
| src/memory/world.ts | FactProposal 接纳政策、纠正链、world revision | commit、policy | T06、T24 |
| src/memory/entities.ts | 有来源的名称解析/创建、歧义候选、关系引用 | commit、object-store | T24、same-name-distinct |
| src/memory/obligations.ts | 独立权威要求/承诺/问题，来源与解决依据 | commit | T10、T20 |
| src/memory/worker.ts | 连续 batch、closed episode、coverage、CAS、水位/坏事件 | model、journal | T08/T09/T20/T21 |
| src/memory/retrieval.ts | scope/time/task/entity/keyword 查询、有界 cursor、证据回查 | repos、object-store | T07、cursor-stall |
| src/context/build.ts | 一致 cut_seq、必要工作集、版本/来源去重、预算组装 | memory、storage | T10/T11/T24 |
| src/context/exit.ts | 水位/覆盖/工具配对校验，提交退出位置，runtime 轮换 | build、runtime | crash-exit、tool-pair |
| src/transport/server.ts | 私有 UDS、认证、路由、限流和持久接收/查询 | core services | auth、disconnect、control-latency |
| src/transport/client.ts | request ID 本地持久、重发/查询、reply ACK/cursor | HTTP UDS | T02/T22 |
| src/ops/backup.ts | 暂停写/GC、Backup API、对象清单、hash 验证 | storage | T23 |
| src/ops/restore.ts | 新目录恢复、删除 ledger、未知动作 reconciliation | backup、recovery | old-backup-no-replay |
| src/ops/delete.ts | 引用反查、派生失效、tombstone 与 backup 策略 | storage、policy | T26 |
| src/ops/deletion-ledger.ts | 私有追加/fsync/hash 链、PREPARED/COMMITTED、恢复合并 | fs、delete、restore | delete-crash、stale-backup |
| src/diagnostics/events.ts | 结构化脱敏事件/指标；不记原文/令牌 | core trace | secret-redaction |
| src/bin/secretaryd.ts | 启动/信号/恢复/关机组装，默认 fixture | 全部服务依赖注入 | process-restart |
| src/bin/secretary.ts | init/chat/status/task/cancel/grant/pause/backup 等子命令 | client、ops | first-flow-cli |
| native/state-lock.c | flock 独占锁并 exec Node；FD 生命周期 | OS syscall | double-start、kill-restart |
| native/file-broker.c | 根 FD、openat 逐段 NOFOLLOW、普通文件/硬链接检查、renameat | OS syscall、sandbox | T05/T19、path-race |
| native/process-supervisor.c | 生命管道、进程组、nonce/起始时间、父进程消失终止 | OS process APIs、sandbox | T19、orphan-descendants |

模块之间通过构造器显式注入 Clock、IDFactory、Store、ProviderTransport、ExecutorTransport；测试不得 monkey-patch 全局生产网络。提供方 key 只到受控 provider adapter，不到工具参数或沙箱环境。

这些模块的连接点、事件政策和文件/网络算法以 [RuntimeProtocol](RuntimeProtocol.md) 为准。

## 3. 核心函数伪代码

```text
accept(input, trustedMaster):
  validate size/schema; body = ObjectStore.put(canonical payload)
  transaction:
    if request_id exists: require same principal/payload hash; return existing
    insert request; increment input_generation; append event
  return ACCEPTED(request_id, committed_seq)

execute(toolArgs, trustedAttempt):
  validate final args and canonical resource
  transaction: verify revisions/grant/epoch/cancel/budget; create intent+permit
  transaction: consume permit once; mark DISPATCHED
  try external executor without DB lock
  persist evidence; transaction append receipt and reconcile operation
  return bounded result; unresolved effect => UNKNOWN

commitSuccess(proposal):
  transaction:
    require current task version and relevant dependencies unchanged
    require every current criterion PASS with acceptable evidence
    require no UNKNOWN operation and no outstanding cancellation
    insert verification; update task; append event; insert reply+outbox

publishMemory(draft, oldCursor):
  verify schema/references/semantic coverage and contiguous processed prefix
  transaction CAS(oldCursor): insert memory version and matching watermark
  after commit evaluate history-exit; never exit ahead of watermark
```

## 4. 第一条纵向实现路径

先实现配置/合成目录 → SQLite/对象 → accept/query → fixture Agent + Journal → task/dispatcher → 受控 file.read → 程序验收 → reply/outbox → 进程重启查询。此时不实现丰富事实模型、语义检索或自动遗忘。

为 `fixtures/first-file.txt` 写固定文字与 SHA-256；fixture 模型发 task.create，子 fixture 发 file.read，回报片段/对象引用，验收检查 hash 与期望内容。外部程序杀掉协调器的检查点分别在 TX1 后、TX2 后、工具前、工具后回执前、TX6 后发送前；每次重启复用同 state root 和 request_id，断言外部状态和数据库，而非只看终端文案。

再接真实模型做相同只读任务；然后隔离写；最后 P4—P6 的记忆。这一顺序应能独立按本文推进，不依赖重新阅读所有 Pi 源码。
