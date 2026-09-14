# Ayanami 模块复核

此目录保存本地 Hermes Ayanami 对实现模块的原始复核回复。复核调用由 Codex 协调，回复不等同于程序验收通过；正式验收仍须依据 `docs/Acceptance.md` 的独立测试证据。

- 入口：本机 `hermes chat`，default profile；身份通过本机 SOUL 的名称核实。
- 会话：`secretary-simplified-module-review`。
- `00-ayanami-handshake.md`：设计基线与复核准备确认。
- 后续每个模块使用独立编号文件保存请求和原始回复，并记录相关测试结果及修复复核。
- stderr 单独保留用于追踪调用失败；不包含密钥；发布前需统一扫描。
- 不读取 `resources` 中的凭据，不连接远程 ELIZA，不将 Codex 自审包装成 Hermes 的独立审查。

模块只有在真实回复到达后才标记已复核。API 失败、超时或空回复均记录为未完成，不视为赞同。

## 复核索引

- [设计修订授权确认](01-design-change-authorization.response.md)：Ayanami 已确认 Master 授权范围。
- [M1 foundation 初审](M1-foundation-ayanami.md)：有条件；独立探针揭示需修复项及 tombstone 契约缺陷。原始测试探针/日志位于 [M1-evidence](M1-evidence)。不得当成最终验收通过。
- [D01 计划事件登记](D01-scheduled-job-event.response.md)：同意，含映射/原子事务/事件边界/测试与文档同步条件。
- D02、M3 runtime、M3 diagnostics 初次调用遭遇 API Connection error，调用失败不代表复核否决或通过；重试与最终状态以对应 attempt 及 response 文件为准。

## Reviewer 模型切换记录

M1 初审与 D01 成功回复使用本地 Ayanami default 模型 `deepseek-v4.1-flash`（provider `opencode-go`）。其后 D02/runtime/diagnostics 遭遇连接失败，原始失败分别保留 attempt 文件。经当前任务协调方授权，复核仅对单次 `hermes chat` 使用 `--provider opencode-go --model gpt-5.6-luna`；身份、记忆、会话与持久配置未变。用户已明确选择该模型。

[切换握手](02-luna-reviewer-handshake.response.md) 已真实成功。此后 D02、M3 runtime、M3 diagnostics 的当前 response 文件来自 `gpt-5.6-luna`；各自原始 deepseek 失败均不构成审查结论。审查意见仍须源码与测试证据支持，不以模型赞同取代验收。

## 后续已到达结论

- [D02 周期通知身份](D02-notification-occurrence.response.md)：同意实例化绑定，要求版本化无歧义编码；禁止裸拼接、禁止 REPLAN 改 criterion。
- [M3 runtime 初审](M3-runtime-ayanami.md)：需修复；独立 probe 发现 permit/cancel/reconciliation 缺陷。证据位于 [M3-runtime-evidence](M3-runtime-evidence)。
- [M3 diagnostics 初审](M3-diagnostics-ayanami.md)：有条件；manifest、doctor 与恢复 CLI 集成需修复。
- [M2 world 初审](M2-world-ayanami.md)：需修复；跨 run permit、旧 fence receipt 与事实准入权限等问题已交实现者。
- [D05 意识槽验收条件](D05-consciousness-criterion.response.md)：同意，须实现唯一 SlotController、命令指定 slot、精确 DTO 验证与移除直接 Refresh 旁路。
- 以上 D02/M2/M3/D05 均使用单次 `gpt-5.6-luna`，原有 Ayanami 身份与记忆保留。

所有初审结论限定于报告中的源码快照。并行实施后的修复必须由后续报告明确核实；不能静默覆写原失败或把旧结论迁移为当前通过。

## Master 最新 reviewer 要求（优先于历史记录）

Master 明确要求 Hermes Ayanami 的最终复核模型为 **DeepSeek v4.1 Flash**，禁止 fallback Luna。已手动固定 Hermes 默认模型 `deepseek-v4.1-flash`；现有 provider 为 `opencode-go`。此前所有 Luna 报告仅为 **model-qualified 历史证据**，不得充当最终复核裁决；报告头部已注明，原始问题、结论、测试输出没有改写。最终模块与 D01–D07 设计有效性须由 DeepSeek 重新检视。原 DeepSeek M1 初审/D01 结论仍保留，但不能自动覆盖后续源码变化。

## DeepSeek 新裁决索引

- [D01–D07 独立重新裁决](D01-D07-deepseek-revalidation-resume1.response.md)：DeepSeek 认可设计原则，DOC+CODE 只读层；D06 停机跨 gap 追赶审计边界需修复。动态测试 NOT_RUN，不能当模块终验。
- [D08 能力准入与固定 criteria](D08-capability-criteria-deepseek.response.md)：三能力最小严格方案同意，按当前 run 的程序持久证据验证；实施与 ACC 用例待后续核验。
- [D09 检索预算条件裁决](D09-retrieval-budget-deepseek-resume1.response.md)：条件同意；必须补 RetrievalServed 零命中信封、计数口径和多命中测试后同步正式设计。

D01–D07 与 D09 首次到工具上限时遭遇 Hermes 强制摘要通道 HTTP 400 MissingSessionID，原错误保留；有效裁决来自随后同一 DeepSeek 会话的正常无工具收口调用，不是换模型或隐藏失败。

- [D08 REPLAN 当前 run 补充](D08-replan-current-run-deepseek.response.md)：认可最新已接纳 run 与旧未决 run 阻断规则，旧证据不得复用。
- [D10 自然语言提醒策略边界](reminder-defaults-v1-boundary-deepseek.response.md) 与 [最终范围澄清](reminder-defaults-scope-deepseek.response.md)：仅 NL CREATE_JOB 的 notify.local/alarm.play 强制既有默认策略；Typed 显式配置不变。撤销 substring 授权与额外两次调用上限建议，沿用已有预算；实现须独立复核。
- [M3 runtime 源码与独立探针](M3-runtime-deepseek-final.response.md)：13/13 源码哈希一致，CAS/D08/D06/remote effect 等定向检查通过；artifact 根目录交换补验当时尚待，后续报告负责收口。
- [M3 transport/CLI 静态与 UDS](M3-transport-cli-deepseek-final-resume1.response.md)：34 个真实 UDS 探针等通过，结论局限于所列范围。
- [M3 CLI 实际主链路](M3-cli-e2e-deepseek.response.md)：actions、trigger、ack、cancel、search、doctor 等实测通过；参数顺序和帮助信息建议非阻断。
- [M3 restore/query 实际补验](M3-restore-query-only-deepseek.response.md)：真正可领取 QUEUED seed 的备份恢复、双冻结与新凭据空权限、verify 正负向和 Query 零 POST 独立计数均通过。二进制快照见报告，不等于后续 D10 构建或整体验收。
- [M1 / World / Diagnostics 与 artifact 补验](foundation-world-diagnostics-deepseek-final.response.md)：DeepSeek 独立验证修复，无 must-fix；artifact v2 两次无逃逸且正常控制成功。未注入的 World finalize/末次 backup verify 失败仍仅代码级。A19 baseline v1 适用 PASS、旧业务迁移 N/A；A23 无“同一 DB”文字门槛，但组合集冻结覆盖映射与全部失败史须明确，不能追认事后挑选为预冻结。
- [M2 Core / Memory 与 D09/D10](M2-core-memory-deepseek-final.response.md)：DeepSeek 独立反例与定向测试确认多数修复，D09/D10 离线通过；两项实际 must-fix 为待答问题生命周期及 memory/consciousness 语义失败持久记录。其第一节自提方案不是最终设计，以后出的 D11 专门裁决为准。
- [D11 待答问题准入专门裁决](D11-pending-question-deepseek.response.md)：三点同意，可选问题内容 proposal + 显式 answer_to_question_id；程序独占身份/序号/状态，同 session 回答、跨 session 检索范围满足本期 A09。任一 proposal 无效整决策拒绝，非 MASTER 携带问题明确拒绝，resolved/未知错误码分界必须钉死；实施后再独立复核。
- [可公开模型运行来源](model-provenance.json)：从实际 Hermes 持久会话元数据提取，关联报告 SHA 与模型产出文本/写文件参数；无密钥、本机身份路径或私有会话参数。该文件证明本地 reviewer route 与已留存产出关联，不充当提供商模型版本的服务端签名。
- [D11 / memory 语义记录增量终审](D11-memory-outcome-deepseek-final.response.md)：两项生产机制 must-fix 独立闭合，D10 live run2 档案核证通过；D11 真实链未通过。**时点补充**：root 后续定位 live-question-run1 中 D09 裁剪丢失助手问题程序块，原报告“脚本侧故障”判断不足以覆盖新证据，见 final-evidence-followup-notes.md；跨会话腿仍阻断。
- [D11 空白回答](D11-empty-answer-deepseek.response.md) / [空白问题](D11-empty-question-deepseek.response.md)：同意统一 TrimSpace 存在性校验，原文不修改，不引入真实性判断，失败整决策无业务副作用；实施反例另验。
- [A23 统一映射核证](A23-ledger-deepseek-resume1.response.md)：129 case、1902 个引用哈希校验，选项1成立，可按所列组合范围定稿；事后整理索引、原先各集冻结来源与完整失败史分开，不声称同一DB整合世界史或密码学时间戳。第一次强制总结HTTP400保留，有效裁决来自同一DeepSeek正常收口。
- [D09 双锚补充裁决](D09-dual-anchor-deepseek.response.md)：同意有 Evidence/无 Evidence 历史各最新一条为不可淘汰锚；其余顺序、净引用省略口径、权限与 32000 字节上限不变。替代旧单锚定义；原 run1 失败保留，实施与相同 oracle 真实跨 session 链待验证，不以本设计同意当测试通过。
- D12 初稿 [output-class 初次裁决](D12-output-class-deepseek.response.md) **未获实施方同意，已被后续最终契约替代**：原称 extension 不能承载递归 data_class、将 Item/Task/World 同类路径排为后续问题均已明确撤回。
- [D12 最终可实施契约](D12-output-class-final-contract.response.md)：严格程序独占 security.classification、完整已确认派生链 join 不降级、旧缺标保留 unknown 并明确拒绝、默认 PERSONAL/显式 SYNTHETIC。属于设计同意，尚非实施安全验收；同会话独立 fake canary 已复现原缺陷。不得以 D09/D11 真实语义通过替代 D12 canary 安全回归。
- [D12 注入/对照澄清](D12-injection-oracle-clarification.response.md)：采用任意层伪程序分类整请求/Decision拒绝；原模型诊断不改写。canary禁止仅指未授权分类出网，合法PERSONAL对照允许且应证明传播。
- [READ_MEMORY 最终答案提示修复](READ_MEMORY-final-answer-clarification.response.md)：确认只是告知既有状态机，最终答案controls为空；预算/互斥/oracle不变，不静默丢control。真实run2失败保留，同模板/同oracle复验待完成。
- [D12 Runtime/World 独立终审](D12-runtime-world-deepseek-final.response.md)：M3/M4 PASS，8目标+7独立反例+限定旧回归，17源码哈希匹配；command hash不受receipt升级污染，legacy拒绝零改写，未发现mustfix。
- [D12 Core/Memory/入口独立终审](D12-core-memory-deepseek-final.response.md)：M1/M2/M5/M6/M7/M8与审计传播闭合，6独立canary探针通过；PERSONAL正对照/未授权拒绝/深注入/空与缺省/legacy零改写均实测。真实语义链NOT_RUN；后续model.go精简/RegisteredExtensions提取需单独等价核证，旧快照结论不自动覆盖。

口径说明：报告中的“零真实模型调用／LIVE_MODEL NOT_RUN”指该次审查没有另行运行 Secretary 产品的真实模型试验，并非复核器未使用真实 DeepSeek。Ayanami 复核调用的实际 provider/model 与持久产出关联由 model-provenance.json 单独记录。
- [最终 Schema/prompt 等价核证](final-schema-prompt-deepseek.response.md)：独立展开40处RegisteredExtensions并删除唯一helper后，完整Schema与前版深相等；严格测试、再生成一致。roleInstruction逐句保留既有规则（非自然语言形式化证明），文档/CLI说明一致。未发现语义变化。计量澄清：d52b/5fdd是Python排序序列化整document哈希，raw文件为8d923/066e；20214/19770是Decision闭包字节，504是短prompt配对wire差，详作者报告重算记录，不能混用；真实run4/月8当轮仅看到存在，待最终档案补核。
- [最终真实档案/hash核证](final-live-archive-deepseek.response.md) 与 [末次元数据补正](final-archive-metadata-correction.response.md)：question-run4公开/本地report逐字节相同，主要流程有直接工件，Core重启/幂等不重调保留脚本断言级边界，不新增门槛；构建两binary匹配。month-run8公开/本地report相同，90身份审计/30快照引用审计PASS且绑定原报告；不扩称自由文本全正确。A23 v2同129构成、v1原hash未改，v2新增全量索引审计为作者证据未由本轮重演。末次补正替代前一报告的publisher待完成状态与hash口径误述；Schema/prompt等价PASS不变。

## GitHub Issue #1 新增复核

静态审计原文见 [issue1-original.md](issue1-original.md)。实际复现和修复验收分开保存；旧项目终审与原持续测试证据不改写。本轮明确不重跑两小时或新增持续时长门槛。

- [AUD01/03 最终协议](issue1-AUD01-AUD03-design-correction.response.md)：同意 flock + WriteObjects；明确替代首稿 SQLite-only 清理、锁外 I/O 和暂存清掃条件。实施后独立核证待完成。
- [AUD02/04 最终协议](issue1-AUD02-AUD04-design-correction.response.md)：同意 bounded 收尾、同代有证据恢复与只读 GET；替代首稿扩展字段和跨代旧失败重绑定判断。实施后独立核证待完成。
- [GATE01/02 独立核证](issue1-gates-review.response.md)：指定分支限定范围 PASS，无 mustfix；实际定向测试及 race 通过，fake transport 非真实资料试运行。
- [诊断归档等待上限](issue1-diagnostic-bound.response.md)及[事实更正](issue1-diagnostic-bound-correction.response.md)：同意仅给输出归档增加 5s deadline；保留原错误传播，不吞错，不承诺同步文件系统调用可抢占。[独立锁竞争核证](issue1-diagnostic-bound-final.response.md)已限定范围 PASS，实际定向 race 两项通过。

Issue #1 实施独立复核已收口：

- [AUD01/03 存储终审](issue1-storage-final.response.md)：限定 PASS，亲跑定向 race 与独立 crash/兼容探针。
- [AUD03.2 业务先写再回滚补证](issue1-storage-typed-final.response.md)：限定 PASS，新测试 e92d20b6；亲跑全表行/对象字节基线回滚与幂等重试。
- [AUD02/04 执行与读限终审](issue1-runtime-final.response.md)：限定 PASS，最终 11 项工件 hash 核对、实际 UDS/fake HTTP/race；GET 只读与旧 fence 反例通过。

本轮无开放 review must-fix；最终全仓/release短 smoke 由 root 另行归档。原两小时证据保持原构建与 hash，本轮不重跑、不增加持续时长门槛。
