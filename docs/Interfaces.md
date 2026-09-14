# 接口与 CLI

## 1. 本机协议

阶段 3 使用 HTTP/1.1 over Unix domain socket：`core.sock` 和 `runner.sock`，均位于权限 0700 的运行目录，socket 权限 0600。客户端使用本机专用凭据，服务核验客户端身份和角色；身份凭据与模型密钥分离。当前版本不监听 TCP，也不开放浏览器跨源访问。

请求 JSON 均包含 schema_version，变更请求包含 request_id 和 expected_revision（创建为 0）。响应外层包含 request_id、status、result、error；error 为 code、message、retryable、details，不泄露凭据或完整私人原文。request_id 不能由一次 HTTP 重试重新生成。

模型等待型操作返回 202 和可查询 ID；提交事务完成的即时操作返回 200/201。超时不等于未受理：客户端用同 request_id 查询回执再重试。列表接口按 `(created_at,id)` 或 seq 分页，默认 50、最大 200；cursor 绑定查询条件和读取水位。

## 2. Core API

| 方法与路径 | 输入 | 输出 |
|---|---|---|
| POST `/v1/inputs` | InputEnvelope | accepted、turn_id、intent_id |
| GET `/v1/requests/{request_id}` | 客户端身份 | 原提交回执或 NOT_FOUND |
| GET `/v1/turns/{id}` | 无 | 状态、回复、命令登记结果 |
| GET `/v1/items` | domain/status/cursor | Item 页 |
| POST `/v1/items` | 类型化创建命令 | Item + revision |
| PATCH `/v1/items/{id}` | patch + expected_revision | 新版本 |
| GET `/v1/tasks/{id}` | 无 | 目标、完成条件、run 和验收 |
| POST `/v1/tasks/{id}/cancel` | request_id、expected_revision | cancellation generation |
| POST `/v1/jobs` | ScheduledJob 创建命令 | job_id、next_due_at |
| PATCH `/v1/jobs/{id}` | 规则/启停 + expected_revision | 新计划版本 |
| GET `/v1/world` | entity/predicate/cursor | 事实版本、状态、依据 |
| POST `/v1/world/proposals` | WorldUpdateProposal | 专用任务 ID；不直接写事实 |
| GET `/v1/memory/consciousness` | 无 | 当前快照和逾期状态 |
| POST `/v1/memory/search` | 范围、关键词、cursor | 授权后的引用页 |
| GET `/v1/context/{id}` | 诊断角色 | 脱敏 ContextManifest |
| GET `/v1/events` | after_seq、limit | 可恢复事件页 |
| GET `/v1/health` | 无 | db/core/runner/model 状态与队列积压 |

来源同步、意识更新和 P1 agent 派发使用内部 `/internal/v1/work`、`/internal/v1/work/{run_id}`、`/internal/v1/world/commit`。仅接受 Runner/内部执行身份，携带许可和 fencing；普通 CLI、模型和外部 Agent 不得调用 commit。

## 3. Runner 控制 API

| 方法与路径 | 行为 |
|---|---|
| GET `/v1/runs/{id}` | 查询执行与未知结果状态 |
| POST `/v1/runs/{id}/cancel` | 经过共享业务服务更新取消代际；可在 Core 离线时使用 |
| POST `/v1/jobs/{id}/trigger` | 用 request_id 手动触发一次，遵守 overlap |
| POST `/v1/alarms/{id}/stop` | 无模型停止指定播放会话 |
| POST `/v1/alarms/{id}/snooze` | 停止并原子登记延后 once job |
| GET `/v1/health` | 调度、播放、队列与最近扫描时间 |

stop/snooze 为受控类型化命令，不调用自然语言理解。延后请求必须携带秒数或目标时间，默认值由已保存的提醒配置决定，不能模型猜测。

## 4. CLI 交付面

```text
secretary init --data-dir <isolated-directory>
secretary chat [--session <id>] [--json]
secretary input --request-id <id> --text <text>
secretary items list|show|create|update
secretary tasks show|cancel
secretary jobs list|create|pause|resume|trigger
secretary runs show|cancel
secretary alarm stop|snooze
secretary world list|propose|correct
secretary memory status|search
secretary context show <id>
secretary doctor
secretary backup --output <directory>
secretary verify --suite <name> --report <directory>
secretaryd core --config <path>
secretaryd runner --config <path>
```

以上是待实现的命令接口，不声称当前可运行。CLI 默认友好文字输出，`--json` 提供稳定 Schema。退出码：0 成功、2 输入错误、3 冲突、4 权限错误、5 暂不可用、6 结果未知或需关注；202 异步受理为 0，但正文必须标明未完成。

## 5. 错误分类

400 INVALID_SCHEMA/INVALID_ARGUMENT；401 UNAUTHENTICATED；403 PERMISSION_DENIED/DISCLOSURE_DENIED；404 NOT_FOUND；409 CONFLICT/IDEMPOTENCY_CONFLICT/STALE_FENCE；413 INPUT_TOO_LARGE/CONTEXT_REQUIRED_OVERFLOW；422 UNSUPPORTED_VERSION/UNKNOWN_CAPABILITY；429 BACKPRESSURE/BUDGET_EXCEEDED；503 DEPENDENCY_UNAVAILABLE/DB_BUSY。RESULT_UNKNOWN 是执行状态，查询它本身返回 200。

模型输出错误最多在根预算内重试生成 2 次，原始失败保留脱敏诊断。不从自由文本或不完整 JSON 猜出命令执行；允许程序生成普通失败回复。服务商支持严格输出时传入其支持的等价 Schema 表达，同时始终执行完整本地校验。若不支持 allOf/if/then，可将判别联合编译为支持的分支表达；无法等价表达时以 JSON 模式生成并在本地严格拒绝非法输出，不宣称服务商原生保证了完整契约。

## 实施过程中发现的缺陷（D10，2026-09-14）

自然语言 Decision 创建 notify.local/alarm.play 计划，仅支持 misfire=FIRE_ONCE_WITHIN_GRACE、grace_seconds=300。模型生成非默认字段时，整笔决策拒绝，错误码 REMINDER_POLICY_UNSUPPORTED；反馈后沿用 Core 既定最多三次生成和持久预算。被拒绝的动作不提交 Item、计划、任务或通知；失败输入与诊断仍保存。

需要非默认策略时，用户通过已认证 Typed 请求明确提供字段，保留其值并按原调度语义执行。自然语言模型不能代写 Typed 通道；自定义自然语言请求应得到能力限制说明而非默认替代动作。此限制仅覆盖 CREATE_JOB 的两种提醒能力，不改变其他 job。讨论依据：`review/reminder-defaults-v1-boundary-deepseek.response.md` 与最终范围/重试澄清 `review/reminder-defaults-scope-deepseek.response.md`。


## 实施设计修订 D11：待答问题生命周期

CLI 输入增加 `--answer-to <question-id>`，与 `--session <原session>`、稳定 `--request-id` 同用。POST /v1/inputs 接受 InputEnvelope.answer_to_question_id；受理仍返回 202，通过既有 turn/request 查询取得稳定最终回复。成功登记一律追加固定文本块 `\n[question_id=<id> session_id=<session>]\n<text>`；成功回答一律追加 `\n[answered question_id=<id> session_id=<session>]\n<original question text>`。程序字段不可由模型生成。

仍在槽位且 resolved：409 QUESTION_ALREADY_RESOLVED；已回收、未知、错 session、主体不匹配：404 QUESTION_NOT_FOUND_IN_SESSION，文案仅“当前会话范围内不存在待答的该问题”。相同 request 已有回执先重放，已解决后同键重试不再校验成新请求；竞争失败固定结果后重试不再模型执行。原 session 显式 ID 回答是本期范围，跨 session 通过 READ_MEMORY 取回原文、session 与 ID，再恢复原 session；不支持任意 session 直接回答旧问题或自由指代自动解引用。

裁决：`review/D11-pending-question-deepseek.response.md`；原提案：`review/A09-pending-question-proposal.md`。无 DDL 变更。
### D11 文字非空边界（实施中发现，DeepSeek 补充同意）

带 `answer_to_question_id` 的文字回答必须包含非空白字符；统一验证以 `strings.TrimSpace(text)` 判空，但保存的原文不修剪。空白返回 `QUESTION_ANSWER_EMPTY`（400），纯附件回答不在此最小文字入口范围。问题提议的 text 同样不能全为空白，否则整笔决策拒绝为 `QUESTION_TEXT_EMPTY`，不登记问题或业务动作；可选的空 questions 数组不变。

讨论依据：[空白回答裁决](../review/D11-empty-answer-deepseek.response.md)、[空白问题裁决](../review/D11-empty-question-deepseek.response.md)。

上述 HTTP 状态适用于受理前的同步校验。已返回 202 的输入若在最终事务遇到竞争或模型问题校验失败，通过 InputTurn 的最终 reply/error_code 返回固定失败结果，不追溯改变原受理响应，也不把程序失败回复当作问题已解决。

## D12 实施缺陷修订：输入与派生分类

普通 CLI `input`、`chat`、类型化写入默认 PERSONAL，使用 `--data-class SYNTHETIC` 明确标记合成测试；支持 SYNTHETIC、PERSONAL、SENSITIVE、SECRET。HTTP 输入缺少 data_class 时由适配器填 PERSONAL；`/v1/actions` 和专用 typed 信封同样接受可选顶层 data_class，缺省 PERSONAL，显式 null、空串和非法值拒绝。类型化计划 PATCH 保守合并旧对象与本次输入分类。分类选择不授予服务商披露权限。

security.classification 为程序独占 extension；客户端或模型在任意层伪造该字段时整请求/整决策拒绝 CLASSIFICATION_INJECTION，原模型输出诊断不被改写。派生记录缺少可信分类时，以 OUTPUT_CLASS_UNKNOWN 拒绝披露；合法分类不被当前策略允许时仍为 DISCLOSURE_DENIED。已受理输入的异步失败边界同上。固定失败回执不复制原问题或用户文本，只有纯程序字面量可标 SYNTHETIC。

依据：[D12 最终裁决](../review/D12-output-class-final-contract.response.md)、[注入与对照澄清](../review/D12-injection-oracle-clarification.response.md)。这是实施中发现的分级传播缺陷修订，无 DDL 变更。
