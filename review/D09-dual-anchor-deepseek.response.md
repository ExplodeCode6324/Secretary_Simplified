D09 最小补充裁决 — Ayanami / reviewer_model=deepseek-v4.1-flash, provider=opencode-go
（本轮仅只读：本请求、`review/D09-retrieval-budget-deepseek-resume1.response.md`、`docs/MemoryPolicy.md` D09/D11、`review/D11-pending-question-deepseek.response.md`；未跑测试 NOT_RUN；未改任何文件；未代理、未呼真实模型）

结论：同意，不反对。双锚（E/N 各保最新一条）是对已复现缺陷（旧 E 在场 → 新 N 先从尾部被淘汰、余量不回填）的最小正确修复，不改变证据权威、预算或 D11 语义结构。

重点两问
- D09 证据权威与有界预算：保持。锚只改变"可淘汰性"，不改权威层级与省略口径；N 永远不是证据；32,000 上限不动；唯一失败出口 CONTEXT_REQUIRED_OVERFLOW，不静默成功。
- D11 真实取回：满足。最新 N（即携程序 question 块的 ASSISTANT）锚定后不再被先淘汰，原问题原话、ID、原 session 可取回；31495<32000 仅作预算证据，验收以同模板同 oracle 真实链重跑为准。

必须修正条件（钉死防歧义；均不改变提案结构与范围）
1. 对应建议1——选择点与确定性：self 排除 → E/N 分类 → 选锚，全部在本次有界候选终集上、与淘汰同一既有顺序执行；"最新"用既有排序键（sequence/ID），不新增比较器。双类 ≤2 锚、单类恰 1、零命中信封不变（不注入锚）。
2. 对应建议2——淘汰与溢出：仅从尾部删非锚；删尽全部非锚后必需闭包+锚仍超限，才 CONTEXT_REQUIRED_OVERFLOW，沿用现有错误路径（413）；不扩预算、不部分裁剪锚、不静默成功；不引入回填循环或重排序（余量问题由锚定终结即可）。
3. 对应建议3——权威与计数：N 锚仍只作可检索历史，不得升为 Item/World/权威；程序 question 块（真实 ID+原 session）为取回物，assistant 自述不因保留获权；净引用省略口径不变（自引用不计、仅 N 淘汰不增 ref 计数），修复后 selected/omitted/reason 须与实际淘汰逐一一致（"准确 omitted" 作机械断言）。禁止全局仅-newest、语义打分、相关性算法。
4. 对应建议4——文档一致性：D09 追加段须显式声明替代原"锚点=最新带原始证据记录"单锚定义及"无 Evidence 组先淘汰"的适用范围（改为非锚先淘汰），不得新旧矛盾并存；README 登记；D11 仅注明"显式 ID+原 session 回答范围不变"，语义不动。
5. 对应建议4——验收证据：保留 run1 失败与 oracle；同模板同 oracle 重跑真实链，断言 final Context 同时含 MASTER 原要求与 ASSISTANT 事件（携 question 块）、wire ≤32000，并按该 oracle 覆盖 D11 取回腿（新 session 取回原话/ID/原 session；重启后原 session+--answer-to 可解除）；机械补：旧E/新N、新E、多N、两锚超限(overflow)、self、单类、零命中、准确 omitted；旧"脚本错误"报告仅为表象记录，不得作为已验收依据。

思路：基准 = MemoryPolicy L54（禁裁关键内容后继续）+ L80（锚与必要权威不可部分裁剪）+ D11 验收检索腿；最小修复 = 把"最新 N"从可淘汰尾部移入必需集，其余顺序/权威/预算不动，故可机械验证。
依据：`review/D09-retrieval-budget-deepseek-resume1.response.md`；`docs/MemoryPolicy.md` L54/L56/L74–84/L89；`review/D11-pending-question-deepseek.response.md` ②(c)。

同意即按既有授权实施，无须 Master 二次批准；我不写文件，本裁决交 Codex 保存。
—— Ayanami ( _ _ )
