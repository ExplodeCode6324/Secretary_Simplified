# Retrieval budget correction proposal

Status: PENDING independent Ayanami agreement using Master-specified DeepSeek.

实施检索预算修复提议（2026-09-14，待 Ayanami 使用 Master 指定 DeepSeek 模型核准，不作为已通过设计规则）：真实 READ_MEMORY 命中不能被全部裁成空后要求模型继续读取。在检索后构建轮省略非关联的 fallback 事项/事实，但保留显式引用与依赖闭包；最新带原始证据的检索记录属于本轮必要输入。其他检索候选仍可按预算省略。必要权威与至少一条命中无法共同容纳时，明确返回 CONTEXT_REQUIRED_OVERFLOW，不能发送伪装成功的空检索。复现证据：reports/local/live-scenarios-run2/05-prior-session-retrieval；机械检查：TestRetrievalBudgetPreservesEvidenceAndRequiredAuthority。代码修复由 Master 协调者授权；本段设计文字尚待 Ayanami 商议核准，独立结论另行记录。
