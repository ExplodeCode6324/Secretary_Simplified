A23 Coverage Ledger 最终裁决（独立只读核证 · DeepSeek v4.1 Flash / opencode-go · 未审源码/未跑真实模型/未委派）

一、本轮实际检查了什么（全部只读，命令与数字为本会话实测）

1. 读 review/foundation-world-diagnostics-deepseek-final.response.md（§7 选项1条款与缺口②），实测该文件 sha256 = 4cdfa9800b770966eb7d0727b8a238e674db38eb98d534fc6bf91d7294b0afcc，与 ledger 的 decision_basis 声明一致。
2. 程序化读 reports/implementation/A23-coverage-ledger.json（1,185,171 字节）：实测 sha256 = bf066de63e449f62e7995905d919d79c1b32a893466c6631f875846abe58a310，与 .md 头部 "Ledger SHA256" 声明一致（md 描述的正是该 JSON 字节）。
3. 哈希总量核对：从 JSON 抽取全部 (path, sha256) 引用对共 1902 个唯一项，shasum -a 256 -c 批量校验：1902/1902 OK，"FAILED" 出现 0 次 —— 覆盖 129 个 case 的 authored_input/original_oracle/precall 工件、month 五轮报告与每次 model call 的 record/request/response/context/output blob、scenarios/world 各轮工件、identity correction、reference audit、两个 manifest、harness 引用文件。
4. month membership 独立复核（不采信 "manifest_membership_verified": true 自述）：manifest.json（version 1、seed authored-month-v1、points 90、files 180 条）与 120 个 month case 做哈希 join：oracle 命中 120/120、input 命中 90/90、不符 0 条；180 个文件（90 input + 90 oracle）全部在 1902 中通过字节校验。
5. supplement 侧：9 个 case（7 scenarios + 2 world）全部为 PER_RUN_PRECALL_FIXTURE_NOT_GLOBAL_INVENTORY 索引；35 条 history 中 PASS 27 / FAIL 1 / NOT_REACHED_OR_NO_REPORT 7；所有 PASS 项 oracle_matches_precall_fixture_step=true；fixture_step_canonical 与 oracle_canonical 不符 0 处；oracle 版本数 3（scenarios）/2（world）分列保留。
6. 全失败史：histories 状态分布 PASS 387 / FAIL 19 / NOT_REACHED 222 / NOT_REACHED_OR_NO_REPORT 7。19 条 FAIL 全表已枚举：item.026–029@run3、item.053–055@run2、consciousness.00–05@run1、consciousness.08–09@run3、consciousness.10–12@run2、prior-session-retrieval@scenarios-run2。
7. 与真实报告互验（不凭自述）：run2 报告 FAIL 索引 53/54/55 与 ledger .053–.055@run2 一致；run3 报告 FAIL 26/27/28/29 与 .026–.029@run3 一致；run1 报告 18 条全 PASS、failures 计数 6 与 ledger 6 条 consciousness FAIL 对应；reported_failures 6/6/6/0/0 全对齐。run5/run4 报告 results 90 全 PASS；identity correction 均 status=PASS、90 项、failures 空；reference audit 均 count=30、status=PASS。scenarios-run2-failed：failures 计数 1、7 项；scenarios-run4：0 失败、7 项；world-read-run2：2/2 PASS（实际回复 UNRESOLVED_CONFLICT / COFFEE_WITHDRAWN_UNRESOLVED），其报告 scope 自带 "not model-generated world mutation authorization acceptance"。
8. 时间线/事后性：最早调用 2026-09-13T19:31:58Z、最晚 23:52:25Z；ledger created_at 2026-09-14T00:12:47Z（约在末次调用后 20 分钟）；review 文件 mtime 00:06Z 在前、ledger 00:12–00:14Z 在后。month fixtures mtime 2026-09-13 19:29Z（03:29 +0800）早于首调约 2 分钟；json 内无任何"组合 manifest 先于调用"的声称。
9. 抽样（n=1）：item.000/run5 的 request object blob（818c8e89….blob，字节校验通过）实际包含 authored input 文本片段，命中 1 次。
10. .md 表与 JSON 逐行对比：129 行 vs 129 行；month 120 行逐字节一致；supplement 9 行为表述差异（JSON 侧 supplement 走 oracle_versions 数组、无 case 级单一 oracle 字段；md 按页脚说明取"当前完整运行 oracle"），其中 4 条（exact-id-conflict、prior-session-retrieval、world-conflict、world-retraction）md 显示哈希已对上磁盘文件。

二、未复现 / 未直接验证的项（精确边界，均非反证）

1. canonical 规范化函数未复现：2827a301…、8cfdb6ad…、22fe7eba… 三个 canonical 值仅在 ledger 内出现（reports/ 下 grep 无踪迹）；我用三种中立归一化（jq -S 排序紧凑、去 null 排序、原始序紧凑）均不能得到该值。可独立确证的替代链：底稿字节哈希全过；抽样（run2 prior-session-retrieval）fixture step 与 oracle 在独立归一化下内容相等（同为 1973ca9d…）；canonical 链自洽 0 不符。结论：canonical 函数属"未由我复现"（本轮不审源码），如实记为限制。
2. 抽取口径说明：我的 1902 项只含 path+sha256 结构的引用；verification 块中 retrieval-regression.json 的 expected=actual=d42d194c 属自述+全局 manifest 绑定，未单独复核（scenarios.json、world.json 及 manifest 本身已字节校验；无任何反证）。
3. md 表其余 5 条 supplement 行（dependency-completion、dependency-unlock、remember-agreement、withdraw-agreement、stale-source）显示哈希未逐条比对（无任何反证）。
4. harness save-before-call 次序为文档化规则，未审源码复核；ledger 自述已声明其为 "archived workflow evidence, not a trusted timestamp signature"，与本轮边界一致。
5. run1 六条 consciousness FAIL 的 per-entry 索引基数未独立重导（run1 报告仅含 18 条 item 结果；计数层面一致）。
6. pass 的语义判定逻辑未重导（属工具内部校验，本轮范围外）；索引层 pass/失败账目与真实报告工件已互验一致。
7. review/A23-ledger-deepseek.response.md 我检查时为空文件（0 行）——按"Codex 保存最终文本"约定属预期，不计缺口。
8. 措辞备忘：ledger 称选项1被 "approved as the composition-record option"；§7(D) 原文为该评审"建议选项1、二选一由协调方裁决"，与本轮协调方指令语境一致，且 ledger 自我定位仍为 not_acceptance_verdict=true，非独立验收。

三、裁决

1. 是否满足原选项1：满足。四要件在可独立核证层面全部成立——映射真实（1902/1902 字节级关联；month 120/120 回归预冻结 180 文件 manifest；supplement 逐 run 预调用 fixture 链自洽）；哈希真实（ledger 自哈希、决策依据文件哈希、全部引用工件哈希）；全通过有完整运行支撑（run5 90+30、run4 7/7、world 2/2，报告内部计数与我核对一致）；失败与重试全保留（19 FAIL / 222 NOT_REACHED / 7 NOT_REACHED_OR_NO_REPORT，run1 无报告如实标记、从未被当作 PASS/FAIL）。
2. 同时满足各条红线：未新增"同一 DB 30 天"门槛（same_database_30_day_requirement=false）；未把新 oracle 当旧冻结（month 全轮次同版本；supplement 3/2 版本分列，不视为同一重试）；未忽略失败/重试；World set-up 与模型语义分开（三处写明只评读语义、非模型生成授权）；事后性标注诚实（purpose、precall_evidence_limits、md 均明示"现在整理，不能声称组合 manifest 先于调用"，且实测时间线吻合）。
3. 结论：A23 可按所列组合固定集范围定稿（选项1路径成立）；先前条款逐条对照无未闭合的必需缺口，不需要选项2的最小新运行，不新增任何组合集预注册要求。
4. 定稿须附带：①范围限制（不宣称整合 30 天世界史；free-text 仅 0/14/29 抽样；World 读语义 ≠ 变更授权）；②本裁决第二节残余限制如实列为 limit，不得升级为"已复现 canonical 函数/已密码学时间戳证明 pre-call 次序"；③ledger 自身不是 A23 验收结论，不得改名为 M1–M3 或 REAL_USE 验收。若协调方日后要求"canonical 函数可复现"或"密码学级 pre-call 证明"，那是超出选项1的加严条件，需另行指示。

四、合规与边界声明

全程只读；未修改仓库任何文件（临时校验清单仅写入 /tmp）；未触凭据/系统/其他进程；未新增测试、未改 docs；未跑真实模型、未委派、未遇 429。强制总结通道一次 HTTP 400 MissingSessionID（opencode-go 路由问题，非 429，原样保留，未更换模型），不影响上述裁决结论。

参考资料（本会话实际读取/核验路径）
- review/foundation-world-diagnostics-deepseek-final.response.md（§7；实测 sha256 4cdfa980…）
- reports/implementation/A23-coverage-ledger.json（实测 bf066de6…）与 .md（183 行）
- reports/implementation/A23-supplemental-evidence.md（§7 直接引用）
- src/tests/fixtures/month/manifest.json（version 1 / seed / 180 files）
- reports/fixtures/live-scenarios/manifest.json（自述 "not retroactive freeze proof"）
- reports/live-model/{month-run1…5, scenarios-run2-failed, scenarios-run4, world-read-run2}/report.json 及其被引用工件
- 批量校验清单 /tmp/a23_checksums.txt（1902 项，全部 OK）

思路（简述）：按 §7(D) 选项1 的四要件拆解核证——映射真实性→全引用对字节校验 + manifest join；全通过→与真实报告计数互验；失败史→histories 全量 + 报告索引对；事后诚实性→purpose/limits/时间线。无法在"只读且不审源码"下复现的项（canonical 函数、harness 次序、校验语义）明确列为限制而非缺口，避免把"未复现"误判为"不通过"，也避免以作者自述替代实证 ( _ _ )
