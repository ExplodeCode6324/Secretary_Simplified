# 运行、诊断和恢复

## 1. 配置

配置版本化，使用 JSON；秘密仅以 secret_ref 引用。至少包含 data_dir、timezone、model_profile、provider_policy、queue_limits、context_limits、execution_budgets、source_configs、retention、alarm_defaults 和 consciousness_epoch_at。初始化生成配置回执和指纹。

阶段 3 的默认 profile 为 fixture：独立临时目录、SYNTHETIC 数据、静音执行器、虚拟时钟可注入、禁止真实来源和外部写操作。真实模型 profile 可对同一合成数据启用明确配置的模型。模型与依赖版本写入报告，不能从模型自述判断型号。

实施审计 issue #1 发现真实接入承诺需要明确边界：本轮采用 [受控 PERSONAL 文字与独立数据目录](RealDataTrial.md)，真实来源文件同步未开放。source_configs 的 fixture_path 只供合成资料，不能改指真实文件；默认配置不扩宽模型披露策略。同库全量披露检查可能因无关禁止外发记录而拒绝请求，禁用来源不等于获得披露授权。

生产时钟为 UTC + IANA 展示时区；进程内超时用单调时钟，持久租约使用 UTC 截止时间并配合实例锁与 fencing。检测到异常时钟跳变先做 lease/计划对账；不依据倒退的时钟延长已失效许可。

## 2. 健康和诊断

doctor 至少显示：数据库版本、P1/P2 心跳、最近扫描时间、队列深度、来源最近成功时间、意识快照槽与 overdue、未知 run 数、未解决冲突、最近预算耗尽、备份时间和磁盘剩余量。

每次模型调用保存 input/context hash、schema/model/policy 版本、耗时、token 计数方式和返回状态。每个决策可追到输入、read_set、命令、许可、run、证据与验证。事实解释必须能显示来源原文引用和准入规则，不能只显示“模型认为”。

Issue #1 实施补查发现取消后模型诊断输出归档仍使用无截止时间的上下文。经 Ayanami 同意，该纯持久化路径使用独立 5 秒上下文，保留归档错误传播；模型业务执行仍随原上下文取消。该期限约束可取消的发布锁等待和数据库操作，不承诺中断任意底层文件系统 sync。模型返回成功不等于业务产物已提交；[定向证据](../reports/implementation/issue1-diagnostic-bound.md) 与后续独立复核分开记录。

默认保留全部业务事件、事实历史和未解决执行证据；普通运行日志滚动 14 天，完整诊断上下文默认关闭、启用时保留 7 天。引用仍被事实或验收使用的对象不能按缓存清理。删除真实资料是单独的可审计业务流程，第一版不自动遗忘。

## 3. 恢复规则

| 故障 | 恢复行为 |
|---|---|
| Core 模型调用中退出 | 扫描未完成 turn；复用 intent；命令以账本去重 |
| 通知丢失 | Runner 扫任务表，Core 扫事件表；不依赖内存消息 |
| Runner 派发后退出 | 有查询能力先对账，否则 RESULT_UNKNOWN |
| 旧回执迟到 | 保留审计，校验 fencing 和 cancel generation，不覆写新 run |
| SQLite busy | 最多 5 次带抖动重试，总等待默认 5 秒；超限返回可重试状态 |
| 磁盘满 | 不发布无法持久化的成功回执；暂停新副作用派发；控制停止路径尽力可用并暴露状态 |
| 源同步失败 | 保留旧数据，标记 STALE 和错误；不删除事项 |
| 意识生成失败 | 继续使用当前权威状态和旧快照，显示 overdue |
| Context 溢出 | 明确失败或有界检索，不删必要约束继续执行 |

## 4. 备份与迁移

用 SQLite 一致性备份接口建立数据库快照；随后从快照读取对象引用列表，复制不可变对象并核验哈希，保存 manifest。备份期间暂停对象 GC；manifest 完成前备份标为 INCOMPLETE。运行中直接复制主 db 文件不能视为有效备份。[SQLite 备份接口](https://www.sqlite.org/backup.html)

恢复在新空目录执行，校验 manifest、对象哈希、integrity_check、foreign_key_check 和迁移版本。恢复环境默认冻结执行，先对账历史 run，再明确启用调度；从备份恢复不能重发已发生的外部副作用。

每次迁移先创建可恢复快照，停服务、持锁、在事务内迁移、执行一致性检查。迁移失败回滚；破坏性版本升级不提供未经验证的逆向 SQL，回到原二进制与迁移前备份。应用版本和 Schema major 不兼容时拒绝启动。

## 5. 实际叫醒边界

P2 的本地播放不依赖模型或 Core，但依赖供电、操作系统、登录会话、音频设备和文件可用性。阶段 3 用静音设备适配器证明路径和控制；阶段 4 由 Master 配合检查实际输出、停止、延后、锁屏、断网、Core 故障以及支持的睡眠恢复状态。未经实测的关机、注销、合盖或硬件唤醒不列为可靠叫醒能力。

## D12 实施修订：分类与旧记录

输入默认 PERSONAL；仅合成测试显式使用 `--data-class SYNTHETIC`。默认模型策略仍只允许 SYNTHETIC，因此普通输入可以被本机受理保存，但不会在未授权 PERSONAL 外发时交给模型。SECRET 永不交给模型；改变输入标签不替代 ProviderPolicy 授权。

派生分类沿实体更新、执行复制、问题、摘要和意识生成单调合并。旧派生记录若没有可信 security.classification 标记，披露或再次生成时返回 OUTPUT_CLASS_UNKNOWN；保留原记录、事实历史和证据，不自动标成 SECRET、SYNTHETIC，不自动删除或清库。只有完整不可变上下文与来源证明支持的显式离线治理才可重建分类，并须留存原始字节和审计；当前没有自动迁移工具。新建隔离合成验收状态由其明确的合成 fixture 标记，不属于旧库自动回填。

该缺陷在实施过程中发现，裁决及路径索引见 [D12 最终裁决](../review/D12-output-class-final-contract.response.md) 与 [设计索引](README.md)。


### Issue #1 对象恢复边界

升级后应停止旧版本的同库发布进程再使用新二进制；旧进程不遵守 `.publish.lock`。锁 inode 不可手工删除。完整无引用对象或旧无引用半文件在同内容重试时自动采纳/隔离重建；已提交损坏保持错误，勿删文件强行“修复”。坏孤儿证据名为 `<id>.blob.orphan-<uuid>`，不进入自动清理。

受控 Go 运维接口 `Store.RecoverObjectStaging(ctx, limit)` 可每次清理最多 1–1000 个已知临时文件，保护所有持久引用、正式 blob、隔离证据及未知文件；这不是现有CLI子命令的新增承诺。正常队列/Typed拒绝不再保留完整输入归档。协议见 [Storage.md](Storage.md)，最终裁决 `review/issue1-AUD01-AUD03-design-correction.response.md`。本 issue 不重跑或新增两小时持续测试。

## Issue #2：客户端启动与离线迁移

`secretary` / `secretary chat` / `secretary tui` 的交互终端连接已有 Core；配置缺失、未初始化或 Core 离线时给出可操作提示，不自动 init、启动服务或扩展披露策略。`chat --plain` 与命令式 CLI 保留。TUI 退出只恢复终端并断开客户端，Core/Runner 与已登记任务继续运行。

旧库升级前停止该目录的 Core 与 Runner并保留迁移前快照，使用 `secretary migrate --authority-session <旧MASTER会话ID> --config <配置>`。迁移器验证进程锁，原子追加 002 并登记映射；正常打开不猜测候选，内部会话不得被选中。其他原会话只读保留。空旧库可显式迁移建立新权威，新库由 init 登记；不得复制或拼接其他目录的数据冒充同一实例。

断线或等待超时用原 request_id 查询受理回执及 turn，不自动创建新请求。客户端根据 instance_id 区分本地视图缓存；分页事件按稳定 ID 去重。任务控制与通知确认必须经认证接口，Runner 控制在 Core 离线时仍独立可用。

本轮 release 终端恢复实证覆盖正常退出、Ctrl+C 与可捕获 SIGTERM；另有实际 Model 包装、相同 Run 选项的独立测试进程 panic 注入通过，证据见 reports/implementation/issue2-tui-panic-supplement.md。生产二进制未加入故障注入钩子；SIGHUP/SIGQUIT 未作本轮运行验证，不扩大为所有信号保证。纯文本入口写作 `chat --plain`；裸 `secretary --plain` 不是已承诺的模式切换。目标为 macOS arm64，Linux 仅编译验证，未声明其终端运行通过。
