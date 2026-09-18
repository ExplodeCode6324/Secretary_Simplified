# 本地运行与数据运维

以下是实现目标和操作契约。除附件验证命令外，CLI 子命令尚待构建；不能把本文的目标命令当作当前已安装程序。

## 1. 目录与启停

运行 state root 与 Git 仓库分开，原型使用明确指定的临时/合成目录。布局：`state.sqlite`、`objects/<sha256-prefix>/<object-id>`、`run/`、`config.json`、`secrets/`、`backups/`、`diagnostics/`。对象文件名使用稳定 ID，内容哈希是完整性元数据，不将不同安全域的相同正文自动共享。

目标 CLI：`secretary init --state-root ... --mode fixture` 初始化合成配置；`secretaryd --state-root ...` 前台运行；`secretary chat/status/pause/resume/stop` 控制同一个实例。正常停止：停止新派发 → 限时中止主/子模型 → 终止/核查执行器 → 写明未知操作 → 结算/保留预算预留 → checkpoint → 关闭连接与锁。Ctrl+C 走相同流程；第二次强制停止可能产生 UNKNOWN，不能声称零副作用。

默认不注册 launchd、不自启动、不启用远端同步。P1 应实现 `native/state-lock.c` 小型启动包装器：`open` 私有 lock file → `flock(LOCK_EX|LOCK_NB)` → 清除 FD_CLOEXEC → `exec` Node 主进程；锁描述符不得传给命令子进程。SIGKILL 后 OS 释放锁；数据库 epoch 提供旧回执校验。包装器失败拒绝启动。跨平台锁实现属于新平台验收。

## 2. SQLite 与迁移

每个连接启用 foreign_keys；写连接设置 WAL、synchronous=FULL、busy_timeout=5000，读取连接 query_only。事务尽量小；协调提交用 BEGIN IMMEDIATE。数据目录必须在可靠本机文件系统，初版不放网络盘。断电持久性依赖 OS/硬件正确实现同步，不作超出该假设的保证。

Schema 版本、应用版本、Pi 源码/分发物指纹和配置 hash 记在诊断记录。DDL 为从空库创建 v1 的基线，不是对旧版 Go 数据库的迁移；检测到旧库拒绝自动接管。升级先停止派发、备份、在副本测试 migration，单事务可迁移部分失败则回滚；不可事务步骤带迁移日志。只有全部完成才提升 schema_version。旧二进制拒绝打开更新版本库。

SQLite 库事务只保护数据库，不保护对象文件。对象写入顺序：同一卷临时文件 → 流式 hash/上限检查 → 文件同步 → rename 到最终名 → 目录同步 → DB objects/ref 事务。DB 事务失败产生无引用孤儿，可经宽限期清理；引用存在但对象丢失是完整性故障。

## 3. 备份与恢复

目标命令：`secretary backup create/verify`、`secretary restore --from ... --new-state-root ...`。初版维护窗口暂停修改与对象 GC，以 SQLite Backup API 生成一致快照，遍历该快照引用的对象并复制/校验，写 manifest（schema、cut_seq、配置 hash、对象 hash、删除策略版本）。全部 fsync 后标记 COMPLETE；不完整包不可恢复。不能在活跃库上只复制主 db 文件。

备份默认每日最多一份、保留 7 天、本地，后台调度在该能力实现前由明确手工命令触发，不谎称已有定时备份。凭据不明文随包备份；恢复后由本机密钥配置重新关联。提供方令牌不是业务证据。

恢复必须到新目录：验证 manifest/哈希/Schema → integrity_check/foreign_key_check → 验证所有证据对象 → 重建索引 → 创建新实例 epoch → `RESTORE_RECONCILIATION_REQUIRED`。备份时间之后的外部副作用不可从旧库推断；禁止自动重放全部旧待执行/在途操作。由只读目标核查或 Master 决定解除相应任务阻塞。

原型恢复目标：200 个任务/1 GiB 合成证据的校验与恢复在 5 分钟内、过程可中断重试；正式值由 P2/P3 在目标机测量后冻结。备份只保证回到其 cut_seq，不能声称备份后请求也可恢复。

## 4. 资料保留与删除

业务原文、结果和事实证据默认保留到主动删除；额外完整 Context 调试副本默认关闭，开启时最长 24 小时并继承最高分类。普通诊断元数据默认 30 天，不能携带正文/密钥；数据库状态与任务历史不因诊断清理消失。

删除流程：认证并明确范围 → 找原始对象与所有派生引用 → 生成影响清单 → 对受影响运行任务停止新操作 → 删除内容/索引/摘要并标记事实或证据 RETRACTED → 留最小非敏感 tombstone 与删除序号。若当前明确指令已授权该范围，不增加重复审批；范围含不相关资料才需澄清。

不可变审计与删除的协调：events 存非敏感元数据及对象引用，正文可删除，对象留下无正文 tombstone。保留 hash 也可能泄露可猜内容，按删除策略清除或加受控摘要标记。不能声称 SQLite 普通 DELETE 等于介质物理擦除；需要强删除时做停机重建/密钥销毁等经验证流程。

旧备份中的内容按 7 天到期或本次明确删除选择删除整个受影响备份；不能就地编辑后仍保留旧 COMPLETE 校验。维护恢复用 deletion ledger，恢复前应用比备份更新的删除记录，防资料复活；无法取得 ledger 时该备份不得直接恢复为可服务实例。

预览版本定义、安装级独立 deletion ledger 与中断恢复顺序见 [RuntimeProtocol](RuntimeProtocol.md) 第 8 节。初始化/进程内 worker 拓扑和普通聊天提交亦在该文给出，避免实施时靠猜补接口。

## 5. 可观测性与故障手册

指标：请求接受与控制延迟、任务状态计数、outbox 重试、UNKNOWN 数、主/子/整理调用数与 token、预留/实耗预算、记忆水位/积压、Context 各部分大小、对象增长、数据库忙/写错。trace 关联 request/task/attempt/operation/model_call/reply；不拿统一 trace_id 替代幂等 ID。

| 现象 | 程序动作与人工入口 |
| --- | --- |
| 模型不可用 | 限次退避；暂停受影响请求，status/cancel/revoke 可用 |
| 磁盘满/写失败 | 停止新派发，不发虚假 ACCEPTED；释放非关键临时文件后 integrity 检查与核查 |
| 外部工具断连 | UNKNOWN，提供证据与查询动作；不自动重复写 |
| 记忆水位不前进 | 显示阻塞事件和失败原因，重试/修复；不越过缺口 |
| Context 超限 | 显示必要/可选组成，收缩可选材料；必要信息仍过大则暂停 |
| 旧版本/配置不兼容 | 拒绝启动业务循环，提供诊断和迁移入口 |
| 隔离不可用 | 禁用受影响工具，标记原因；不回退宿主执行 |

诊断导出默认仅元数据和哈希，导出真实正文需已有对应授权。所有修复先保留必要证据，明确回滚点，不自动删除任务或授权库。
