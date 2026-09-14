Ayanami，Master指定仅DeepSeek v4.1 Flash/opencode-go。聚焦D09最小补充设计商议，事实已由root程序复现，禁止重开全审/代理/真实模型；可仅读本请求与 review/D09-retrieval-budget-deepseek-resume1.response.md、docs/MemoryPolicy D09、D11-pending-question-deepseek.response.md 必要段，最多2轮读工具，其余直接裁决。
已核事实：D11真实公共CLI在新session READ_MEMORY返回原问题MASTER和ASSISTANT两条，但最终Context只留MASTER原要求，携程序question_id/session块的无Evidence ASSISTANT被淘汰。排除了当前query自引用与schema失败。现wire30972bytes；在原Context加回所需ASSISTANT后Encode成功31495<32000，报告 reports/local/question-two-events-context.json 及root临时probe schema/Encode均nil。成因是初始候选尚有旧E（Evidence）记录；D09 E优先/N尾部淘汰先丢新N，再丢旧E，最后虽有余量却不回填N。实际跨session问题检索因此失败，旧报告脚本错误是表象，不能当已验收。
root与Codex共同建议D09追加最小规则：
1. 排除self之后，有E与N两类历史匹配时，各类最新一条均作为required anchor（最多2条）；只一类仍1条；零命中现信封不变。
2. 其余仍E组newest→oldest、N组newest→oldest；从尾删除非anchor，任何anchor不可删除。必需Item/World/依赖闭包与两anchors确实容不下即CONTEXT_REQUIRED_OVERFLOW，不扩32000预算、不静默成功。
3. net唯一EvidenceRef省略口径不变，N无Evidence的会话文本只作可检索历史，不将assistant自述提升为Item/World权限；程序question块可取回真实ID和原session。避免改为全局仅newest让助手自述挤掉Master纠正。
4. docs/MemoryPolicy D09追加并README列明实施缺陷；D11明确ID原session回答范围不变。原run1失败/oracle保留，同模板同oracle重跑真实链；机械补E旧/N新/E新/多N/两锚超限/self/单类/零类和准确omitted。
请给明确同意/反对及必须修正条件，重点是否保持D09证据权威与有界预算，是否满足D11真实取回要求；无须Master二次批准。不要为此添加global relevance算法、额外语义打分或新执行权限。输出短且具体，若同意即可按既有授权实施。你不修改文件，Codex保存最终裁决。
