【末次元数据补正 · 本次取证时间 2026-09-14T01:49:39Z（HKT 09:49）· 仅一批只读检查】

一、month-run8 公开副本（已落盘）
- reports/live-model/month-run8/ 现存在（12 项：report.json、两份 audit、objects、model_calls、publication-manifest 等）。公开与本地 report.json 均 SHA=1dae0f9ab063b868c028906a31ffa987828fd236a1d39fd974b26800a21e9183，byte-equal=True，与预期一致。
- report-identity-correction.json：status=PASS，failures 空列表（len 0），results len=90；source_report_sha256=1dae0f9a…（与公开/本地 report 均相等，绑定核对通过）。
- consciousness-reference-audit.json：status=PASS，count=30，scope 自述"Independent immutable event/revision verification, not complete free-text correctness"。
- 记录范围：以上仅为审计档案状态与报告绑定的核对；不重演、不升级自由文本全正确结论（审计 scope 自身亦如此限定）。此前"待 publisher"状态随之解除。

二、等价报告口径更正（替代我上条消息第 4 节）
- 报告新增 serialized_basis / hash_verification / actual_file_sha256 / closure_length_reproduction；原字段保留。读得：
  * d52b…/5fdd…＝Python json.dumps(parsed_document, sort_keys=True) 默认（ensure_ascii=True、默认分隔符、无尾换行）的整 document 规范化哈希——非 raw file、非 Decision 闭包；
  * raw file＝8d923009…（before）/ 066e6b7a…（after），与我此前独立实测的两个文件哈希逐一相同；
  * 20214/19770＝Decision 传递闭包（作者用 contract.Schema 闭包镜像复现，matches_reported=true）；
  * 504＝配对 wire 差（与 q4 旁证的 31965−31461=504 一致）。
- 更正生效：我上一轮第 4 节的"d52b/5fdd 指 Decision 子闭包"表述作废，以本条为准；Schema/prompt 深相等 PASS 结论不受影响、不重测。

三、A23 覆盖台账 v2（最终事后索引）
- A23-coverage-ledger-v2.json（1,818,520B，sha 647dc592…，mtime 09:47）与 .md（3,091B）存在；v1（A23-coverage-ledger.json，mtime 08:14:57，sha bf066de6…）未改——v2 的 prior_reviewed_index.sha256 与 v1 现值相等，哈希层面确认未动。
- 结构读得：composition_policy.whole_runs = month-run8 + scenarios-run4 + world-read-run2；cases len=129；verification: case_count=129、current_pass_count=129、current_precall_membership_verified_count=129、month_manifest_entries_verified=180、issues 空、status=PASS_INDEX_INTEGRITY、not_acceptance_verdict=true（作者另记录 asset_hashes_verified=4018、archived_input_matches_verified=493，我未重做该全量 oracle 审计）；另有 run_inventory 15 与 additional_A09_D11_history 4 等独立段落，question 系列不并入 129。
- 仅归档层确认（同 129 构成与 v1 未改），不重审 A23 设计、不加新门槛。

四、保持不变的结论
- question-run4 上轮逐环节"直接证据/脚本断言"边界维持；重启/幂等命名工件缺失不列为新必需条款、未重演。
- 两 binary 匹配结论维持：q4 报告绑定 e60c16…/6682c7…＝当前 release 文件实测＝SHA256SUMS 在册（未变更）。
- 全程无真实模型调用、无凭据/真实数据/私有路径输出、未用额度工具；无 429/400 事件。

记录到此为止，不再追加轮次。( _ _ )
