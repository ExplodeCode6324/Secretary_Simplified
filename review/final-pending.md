> Issue #2 最新状态：AUTH/TUI/DOC/BUILD限定范围实际DeepSeek复核已收口，无开放mustfix；最后两句文档与导出hash已确认，46份实际模型来源记录。新增证据仅收编清单，不循环复审。以下全部为先前协调历史。

> Issue #2 新增范围：仅 AUTH-01–08 / TUI-01–13 / DOC-01–05 / BUILD-01。架构提案和新模块复核待进行；旧 issue1 及上下文/持久状态/全仓/月回放/长期测试不重开。下方为旧收口记录。

> Issue #1 当前收口：AUD01–04、GATE01/02、诊断等待与 AUD03.2 补证均已真实 DeepSeek 独立限定 PASS，无开放 review must-fix。有效报告索引见 README，实际模型 provenance 共36份。root 最终全仓/release短 smoke 单独归档，不重跑两小时或替代时长门槛。下方是历史协调状态。

> 当前新增 Issue #1：AUD01/03 和 AUD02/04 设计已获真实 DeepSeek 同意，实施后独立复核待进行；GATE01/02 指定分支独立核证已 PASS。此新增状态覆盖下方旧项目收口状态。Master 明确不重跑两小时或替代时长门槛，原证据 hash 保持。

> 当前协调状态：全部模块、D12两组、最终Schema/prompt等价与真实档案/hash核证已完成，无开放的review must-fix。question-run4脚本断言级边界/自由文本抽样范围保持；month8公开报告与90身份/30快照审计绑定已核对。26份实际DeepSeek来源见 model-provenance.json。产品最终2h稳定性由root继续，不属于未完成review。下方旧BLOCKED/Luna/历史缺口记录均已被后续报告取代。

# 最终复核待办（Codex 协调记录）

状态：**BLOCKED_NO_REVIEW_CONCLUSION**。这是复核协调状态，不是 Ayanami 的技术裁决，也不是验收通过。

提供商 OpenCode Go 本轮五小时额度耗尽后，已停止新的 Luna 推理调用。三项在途终审没有形成最终回复；保留实际工具证据后，仅停止了本轮自行启动的三个 Hermes CLI。项目服务、其他 Hermes gateway、持久模型配置、身份和凭据未改动。后续须等可用额度恢复后继续，不能以实现者自测或已执行部分工具检查替代最终复核结论。

## 已完成与未完成的边界

| 模块 | 最新有效复核 | 未完成部分 |
|---|---|---|
| M1 foundation | 原 F1–F8 修复已独立核验；F8 澄清见 `M1-M3-fix-clarification.response.md` | 完整 A02 等端到端验收不由基础包结论覆盖 |
| M2 world / restore | 原 F1–F4 当前可达路径及冻结恢复 Runner 独立验证；F5 当前外部调用链可接受 | 内部 DTO 完整性、证据 blob/lineage、冲突组覆盖边界；后续变更须重新核对 |
| M3 runtime | 原 M3-01–07 主路径修复已有真实审查 | 最后 criterion_hash 零行更新修复、os.Root 文件边界终审未形成结论；新增 PATCH/query 等另需覆盖 |
| M3 diagnostics | manifest 主缺陷修复已核验，CLI restore/verify 曾独立验证 | budget/overdue/history 与 canonical timestamp 后续修复的最终复核，以及最新 Doctor epoch |
| M2 core/context/model/memory | 初审 F1–F10 与 D05/D07 设计商议已完成 | 本轮修复终审未形成结论；不得将最新自测直接替代 |
| 最终 transport/CLI | 早期恢复冻结与 token 隔离有真实复核 | 新增公共接口/CLI 最终整体复核未形成结论 |

## 在途终审保留证据

| 本轮任务 | Hermes 会话 | 保留工具输出数 | 结论 |
|---|---|---:|---|
| M3 runtime 最后两项 | `20260914_030710_0b1a8b` | 13 | 无最终复核裁决 |
| M2 core 修复终审 | `20260914_033325_89da88` | 43 | 无最终复核裁决 |
| M3 transport/CLI | `20260914_040103_f0abc3` | 72 | 无最终复核裁决 |

实际消息/工具证据仅存本地 `review/private/*-partial-evidence.json`，不公开原始会话上下文。机器可读状态见 `final-review-pending.json`。原始失败、曾成功的初审和补充设计结论均保留。当前终审 stdout 文件为空、报错或被中断时，均不能视为 PASS。

## 续审增量清单

1. **M2 core/context/model/memory**：逐项复核原 F1–F10；D07 正式 Context、唯一 output_contract、外部 Manifest 精确 wire hash；Provider 嵌套分类；ValidationFeedback 与 semantic retry 的原始输出/校验记录；READ_MEMORY 真检索、三次持久预算、retrieved_evidence selected/omitted；summary pending questions、CAS、attempt/cooldown；D05 固定持久 epoch、指定槽与持久任务路径；输入原文与 semantic hash 幂等。新增 memory 超过 100 条 delta 的处理须用独立边界输入核验，不得静默截断或误报完整。
2. **M3 runtime 最后残余**：重跑原 criterion_hash 字段篡改/零行 UPDATE 探针，确认 Task/Run CAS 与事件事务；对 os.Root 正常路径、symlink 父组件、并发替换做可复现范围内负向验证。原始独立探针不得只靠改成符合实现的断言来通过。
3. **M3 runtime 新增**：`UpdateJobRequest`/PATCH 六字段白名单、完整 DTO 校验、next_due 重算、取消旧 QUEUED/CLAIMED、严格 RowsAffected、回执同事务、跨后续修改重试返回原响应；Core UNKNOWN query 的持久结果查询与不重发未知效果边界。记录最新双进程 fixture 的重放命令、原日志和源码时点。
4. **公共 HTTP/transport**：新 typed routes、`public_api` 分页及 cursor/limit 错误；requestID 派生 Typed Item ID 的确定性和冲突；PublicCode 错误脱敏；错误响应 HTTP 至少为 400，不能成功状态承载错误；client/internal token 交叉拒绝。检查新路径不会绕过 grant、request receipt 或输入严格校验。
5. **CLI**：typed actions `--file` 严格加载；trigger 的 CAS/version 与返回版本；notifications ack；schema 搜索；`--json` 与默认友好输出；verify smoke 套件和 verify-backup；request ID 重试不重复效果。禁止将只检查 DB health 的路径冒充备份完整校验。
6. **部署/恢复/控制**：restore 新独立凭据、空 capability grant、文件/config 双重冻结；真实恢复 Runner 可启动但不 claim/dispatch/effect；Doctor 使用实际 epoch，并明确未知 telemetry；run cancel 返回 Task revision 与持久状态一致，已发生效果如实记录。
7. **报告界限**：所有最终结论注明源码快照/测试命令/独立结果/未覆盖项；不得把 DOC/OFFLINE/LIVE_MODEL/REAL_USE 混用；新增接口不能套用此前冻结快照的终审结论。正式 30 天真实模型回放、至少两小时真实本机运行等仍以各验收报告为准。

## 额度恢复后的续审命令

以下命令目前**未执行**。在项目根目录运行；先把上述增量补入对应 request 文件，保留原请求和既有响应后再开始。身份/会话保留，模型仅调用级覆盖，不修改 Hermes 持久配置。

```sh
hermes chat --in . --resume 20260914_030710_0b1a8b --provider opencode-go --model gpt-5.6-luna --oneshot -Q --max-turns 30 --run-budget 600 --query-file review/M3-runtime-final.request.md
hermes chat --in . --resume 20260914_033325_89da88 --provider opencode-go --model gpt-5.6-luna --oneshot -Q --max-turns 40 --run-budget 720 --query-file review/M2-core-final.request.md
hermes chat --in . --resume 20260914_040103_f0abc3 --provider opencode-go --model gpt-5.6-luna --oneshot -Q --max-turns 35 --run-budget 600 --query-file review/M3-transport-cli-final.request.md
```

每轮分别保存真实 stdout/stderr，不覆盖已有失败；报告完成后运行 `scripts/hermes_publicize_reviews.py` 生成仅路径规范化的 public 副本，原始报告本地保留 `review/private/`。不委派额外 Hermes 子线程，以免再次只有等待说明而无审查结论。

### M2 F9 最新实施者补充（待独立复核）

新增 `src/memory/manifest.go` 与更新 `src/memory/memory.go`：实现者声明先按 Schema max100 裁剪再 Validate，total 纳入 Snapshot.DeltaOmitted，保留旧 MissingReasons 并追加准确省略数；每次 refresh 生成新 ContextID，持久化标准 Manifest DTO，包含 ReadSet、输出 Schema hash、最终 provider wire hash、各段计数/bytes。`TestMemoryMoreThanHundredDeltasPersistsExactManifest` 使用 125 条真实 DB 变更通过，24h 测试通过；证据见 `reports/implementation/M2-core-memory.md`。本段仅记录实现者交接，不构成 Ayanami 已核验结论；额度恢复后一并检查。

### 恢复进度

Master 已确认额度恢复，本轮改为单并发续审，首先恢复 M2 原会话并保存新 `M2-core-final-resume1-*` attempt；随后依次 runtime、transport/CLI。上文 BLOCKED 是上一轮暂停时的历史状态，最终裁决须以新的真实 response 文件为准。出现 429 即停止新增调用并报告，不重跑已有全仓测试，不把恢复调用本身视为通过。

### Master 指定 DeepSeek 的新终审要求

Master 已明确禁止 Luna 作为 reviewer。上方续审 Luna 命令仅为历史记录，**不得再执行**。已停止本轮恢复的自建 Luna CLI，固定 Hermes 默认 `deepseek-v4.1-flash`；真实握手 usage 见 `03-deepseek-reviewer-restored.usage.json`，确认 provider=opencode-go、model=deepseek-v4.1-flash、completed=true。所有 Luna 报告只能作为 model-qualified 历史线索，最终模块与 D01–D07 需 DeepSeek 检视；新增 D08 能力准入/criteria 也先由 DeepSeek 商议。为避免旧会话约30万token历史重复消耗，使用同一 Ayanami 身份的全新专用会话、单并发、明确禁止 fallback。

### M2 检索预算新增（DeepSeek 待审）

真实场景 run2 出现 READ_MEMORY 三条命中均被预算裁为空、耗尽检索预算。实施者在 `src/context/context.go` 修复：检索轮取消非关联 fallback items/facts，保留显式事项/依赖闭包；按最新有原始 Evidence 命中排序，禁止全部裁空，必要内容仍放不下返回 CONTEXT_REQUIRED_OVERFLOW。独立待核验 `src/tests/retrieval_budget_test.go` 的 `TestRetrievalBudgetPreservesEvidenceAndRequiredAuthority`；实现者普通测试通过、race 运行中，真实同 oracle run3 由 root 控制。本段不是 DeepSeek 已通过结论。
