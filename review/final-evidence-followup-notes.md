待实现终审后交 DeepSeek 同一有效会话作档案核证：
- A23: reports/implementation/A23-coverage-ledger.json 与 .md，129 固定 case，各版本失败史/pre-call membership 索引。明确为事后统一审计索引，未声称组合 manifest 早于调用；各组成固定集原先冻结证据在索引中。需依据之前“选项1补映射零新运行”裁决核对其满足程度，不把事后整理伪装成预冻结。
- D11: 真实公开CLI问题→新session检索原session/ID→重启→原session --answer-to 链最终报告尚待root通知。不得以手工PendingQuestion seed替代。
- D10: reports/local/live-cli-run2 已纳D11/memory终审请求，只读核证未额外模型调用。

最新 root 现场进一步定位（晚于 D11-memory-outcome 最终裁决抓取时点）：live-question-run1 的 JSON 错误只是表面；实际 READ_MEMORY 返回原问题 MASTER+ASSISTANT，但 Context 最终只保留 MASTER 原要求，带 question_id/session 程序块的无 Evidence ASSISTANT 被 D09 E优先/单anchor淘汰。并非当前query自引用。root正在复现 Encode 淘汰原因，未改排序/oracle。故后续不得沿旧报告“脚本侧非生产”判断宣称 D11 live 已闭合；真实跨会话腿仍实际阻断，待正式最小方案商议与相同oracle复验。

D12 后续独立审查线索：runtime 提交 reports/implementation/D12-cli-class-smoke.json，声称真实CLI二进制fixture 5/5：默认PERSONAL持久、显式SYN fixture正常、PERSONAL/SECRET在SYN-only policy零modelcall固定DISCLOSURE_DENIED；原始canary故意PERSONAL/SECRET仅本地保存，不伪SYN公开。该作者补证仍须独立核证，未跑真实API。

D11 run3（D12后）：root报告第2次model调用前CONTEXT_REQUIRED_OVERFLOW，保持32k/双锚/原oracle并存原失败。root仅精简model.go roleInstruction冗余英文，保留D10/D11/READ_MEMORY全部规则；该单文件晚于D12两组review复制快照，最终须单独核规则等价与最新真实链，不将D12安全机制结论自动覆盖新prompt；非D12安全canary失败。

root进一步：month7第3checkpoint后同样Context overflow，无429。model.go roleInstruction同义精简1869→1137B；foundation将重复同shape extensions提取共享$defs.RegisteredExtensions（保留完整约束/32k/权限/双锚/oracle）。最终仅核model/schema增量：与D12审查隔离旧快照逐shape展开$ref比较，关键strict反例/闭包必须等价；若有具体语义差异报告，不凭$ref重构本身要求新设计审批。原失败保留。

计量最终澄清（foundation实际重算）：registered-extensions-equivalence.json中d52b/5fdd=Python json.dumps(sort_keys=True)默认ensure_ascii/空格/无newline的整document哈希，不是rawfile或闭包；raw8d923/066e，285B=shape转义compact，20214/19770=Decision传递闭包UTF8去title/description，504=同shortprompt配对wire差。Codex先前把hash推测为闭包已纠正，Ayanami原报告未改。
