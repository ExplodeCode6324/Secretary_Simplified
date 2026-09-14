Ayanami，Master要求优先正式商议READ_MEMORY检索预算规则。请以DeepSeek v4.1 Flash直接读取review/retrieval-budget-design-proposal.md及对应src/context/context.go和tests/retrieval_budget_test.go必要片段，给明确同意/需修订/反对与条件。不委派、不重复全仓测试、不以Luna旧意见作依据。
提案：检索结果轮取消非关联fallback Item/Fact，保留显式items和依赖闭包；排除当前query turn自引用，至少保留最新一条历史原始Evidence正文，其余可裁；必要内容放不下报CONTEXT_REQUIRED_OVERFLOW。正式MemoryPolicy新增段已撤回，待你核准后再同步；README已有待审登记。
证据线索：run2真实空检索失败，run3 7/7PASS，排除self后live-retrieval-regression-run1 3/3PASS；这些为实现者报告，不能代替你的设计判断。核对是否损害必要权限/权威依赖、对多命中覆盖/省略计数/时间顺序/相关性存在漏洞；给最小方案及文档修改路径。仅设计商议，不需受测真实模型调用。
禁止resources凭据/ELIZA/响铃/改文件。Master已授权双方商议通过后修正式设计注明缺陷+README路径。遇429立即报告不换模型；reviewer_model=deepseek-v4.1-flash/provider=opencode-go。输出保存review/D09-retrieval-budget-deepseek.response.md。
