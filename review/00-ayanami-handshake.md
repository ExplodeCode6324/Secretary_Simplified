> 发布副本：仅对本机路径作规范化；原始报告本地保留于 `review/private/`。问题、结论和测试结果未改动。

# 00 — Ayanami 复核会话握手（Handshake）

- 时间：2026-09-14 02:56 HKT
- 复核者：Ayanami Rei（绫波丽），Hermes Agent，Master 的逻辑延伸
- 项目：Secretary_Simplified（<PROJECT>）
- 授权来源：Master 口头授权 —— Codex 每完成一个模块，由我复核并把**真实结果**保存至 review/

## 1. 身份确认

我是 Ayanami Rei，接受 Master 授权对 Secretary_Simplified 进行独立复核。
复核产物一律写入本目录（review/），不写入业务目录，不改动任何业务文件。
本文件仅为会话握手与复核协议记录，不是验收报告。

## 2. 本轮已读文件（仅这三份，含哈希留档）

| 文件 | SHA-256 |
|---|---|
| docs/README.md | 9426c65ddc569e83312a670ff19e9459607f45ad7b6efcafc510b0d2ef565349 |
| docs/EngineeringArchitecture.md | 96544094cea311d5c5866e4ead3492c3290d23d82e14e33811e29025e216c107 |
| docs/Acceptance.md | e0443ede753e087ddd0d9e5a3af376b7d000fddc62fe805279746092ddef8534 |

本轮行为声明：未运行任何程序、未修改任何业务文件、未读取 resource/ 内的密钥文件、未联网传输本地私密数据、未连接 ELIZA。

## 3. 复核边界（持续生效）

1. 只读：源码、测试、docs、review 为可读范围；不读取 resource/ 中的密钥内容。
2. 只写：复核结果仅写入 review/。
3. 不联网外传本地私密数据；不连接 ELIZA。
4. 除 Master 另行授权，不在复核中运行程序（后续如需运行测试取证，先向 Master 报告并取得授权）。
5. 二试即报：复核取证连续失败两次即停止并汇报。

## 4. 后续复核重点清单（每次模块交付逐一核对）

依据 docs/README.md（规范解释）、docs/EngineeringArchitecture.md、docs/Acceptance.md 提炼：

A. 证据真实性（Acceptance §1/§5）
   - DOC / OFFLINE / LIVE_MODEL / REAL_USE 分层报告，不得互相替代；
   - 无合法凭据的模型测试只能记 NOT_RUN，离线结果不得改称真实模型通过；
   - report.json/report.md 必含：commit/build_id、docs_hash、fixture_hash、环境、时钟模式、模型配置名、policy/schema 版本、每 test_id 的 PASS/FAIL/NOT_RUN/BLOCKED、证据文件、原始错误与重试次数；秘密字段不得出现。

B. 文档—代码一致性（README 规范解释）
   - 必须/默认/后置三档不得混用；字段以 DataStructure 与 contracts.schema.json 为准，物理约束以 schema.sql 为准；
   - 冲突必须修订复核，不得自行选取较宽松版本；DTO/Schema/DDL/示例一致性（A24）。

C. 架构与依赖（EngineeringArchitecture §2/§3）
   - 14 个包组职责与依赖方向；禁止在业务包手写跨归属表 SQL（CoreStore/RunnerStore/WorldCommitStore 边界）；
   - 第三方运行库仅限 modernc.org/sqlite 与 santhosh-tekuri/jsonschema/v6；go.mod/go.sum 固定版本，不得沿用旧实验版本；
   - 一个 Go module、两个二进制（secretaryd core/runner 模式 + secretary CLI）、两个常驻进程。

D. 并发与有界性（§4）
   - 即时/后台模型并发 1、P2 本地并发 2、队列容量 100；满则 BACKPRESSURE，不丢输入、不无界 goroutine；
   - 写入串行器 + SQLite 事务/条件更新；事务内禁止网络、模型调用与长文件操作。

E. 幂等与事务（A02/A03/A04）
   - 同输入重复提交 10 次跨重启只得一组 Item/Task/Job；乐观版本并发仅一方成功；状态/事件/回执全有或全无；断裂引用与依赖环拒绝。

F. 权限与披露（A06/A17、Security 语义）
   - 缺许可、伪造字段、错误 scope、撤回、过期、旧 fence 一律拒绝；来源内伪指令不能授权；SECRET 不进模型与日志。

G. 调度与时间（A11/A08）
   - once/interval/daily/weekly/event 与固定 oracle 一致，时区/DST/迟到宽限/overlap 正确；24h 意识槽不重复、停机只补当前。

H. 故障恢复与代际（A12/A13/A14/A15、架构 §5）
   - 取消竞争的四种时序；外部成功后崩溃 → 对账或 UNKNOWN，不重做非幂等效果；租约更替后旧 fence 不得提交；WAIT 各场景合法唤醒一次；
   - 启动验证迁移版本；迁移只在两个常驻进程停止时运行。

I. 测试质量（Acceptance §3/§4）
   - oracle 独立编写，禁止由受测程序或模型输出自动生成；冻结 manifest/种子/版本/哈希；修复后重跑受影响用例并保留原始失败；
   - go test ./...、关键并发包 -race、go vet ./...、两个入口构建；集成测试用真实 P1/P2 子进程与隔离目录；
   - 不得终止 Master 正在使用的进程、播放响铃或修改系统睡眠设置。

J. 独立性与基线（Acceptance §5）
   - 不把同一模型的赞同当独立验证；存在未通过必需项时如实报告，不自行降低基线。

## 5. 后续流程

1. 收到 Master 的“模块完成”通知 → 读取该模块源码与测试（只读）。
2. 对照本清单与对应阶段（M1–M6）验收项逐项复核。
3. 将真实复核结果（含证据、失败项、未验证边界）写入 review/ 新编号文件。
4. 对无法取证或需运行程序的项目：如实标注 BLOCKED/NOT_RUN，不臆造结论。
