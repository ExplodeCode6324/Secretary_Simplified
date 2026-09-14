【存档/hash 补核完毕 — 最后一轮，3 批工具内完成，无模型调用、无源码重扫、无全仓测试、未改任何业务/报告文件】

一、question-run4 双副本哈希与流程证据
- 独立复算：reports/local/live-question-run4/report.json 与 reports/live-model/question-run4/report.json 的 sha256 均为 48c99b58d30c0c52f0799555d6d2db68b23810648d31d6a20db6ed6634bed156，且逐字节相等（byte-equal True）——与 Codex 所测完全一致 ✓。
- 公开副本工件齐全：publication-manifest.json 在册绑定 report.json（48c99b58…）及 cli-transcript（011dc04a…）、durable-state（4cb78aae…）、precall-bindings（43813d1c…）、program-identity-bindings（28c2b450…）、diagnostics/model_calls/decision_attempts、12 个 objects 等；范围声明为纯 SYNTHETIC（exact_key_matches=0、non_synthetic_objects=0，无 token/私有内容，我也未打印任何此类内容）。
- 报告字段（D11-real-CLI-question-lifecycle，PASS，real_data=false）与可用工件对照：
  * 创建真实问题：cli-transcript 创建命令 + durable-state 问题记录（resolved:false）+ precall-bindings（Item OPEN、reply.questions 登记文本）+ program-identity-bindings（question_id d305db51…、session 25d022b5…）→ 有独立工件 ✓
  * 新 session READ_MEMORY：transcript 含新 session 指示实际调用 READ_MEMORY（query=synthetic-question-project-date）；diagnostics/model_calls/bef147b6….json + .request.json 与报告 read_memory_records.attempt1 call_id 对应 → 有独立工件 ✓
  * 原 session 显式回答：transcript 含 --answer-to d305db51…（响应 ACCEPTED）+ durable-state input.answer_to_question_id=d305db51… → 有独立工件 ✓
  * resolved：durable-state 同一问题 resolved:false → resolved:true 演进可见 ✓；Item 保持 OPEN（precall 与最终态均 OPEN，无任务完成）✓
  * 新 request 409：报告 resolved_rejection（http_status 409、QUESTION_ALREADY_RESOLVED、retryable=false）+ transcript 中该错误响应 → 有独立工件 ✓
  * Core 重启：关键词 "restart" 在 transcript/durable-state 零命中，未发现独立命名工件；目前仅编排脚本/公开索引句层面声明 → 限界记录（不判 PASS，不判 FAIL，不重演）。
  * 幂等不重调：无独立"重放"工件；可见的 model_calls/decision_attempts 共 4 份（07194ccf/33c590d3/6e6aad0a/bef147b6），未见重复调用留痕，与"重放不新增调用"不矛盾但属数量一致性间接支持 → 限界记录。
- 结论：报告声明的六个环节中，创建/取回/回答/resolved+ItemOPEN/409 有直接工件；重启与幂等重放两项为脚本断言级，已按限界标注，未凭存在升格。

二、冻结版本绑定（只核，不重 build）
- q4 报告 build_sha256 = cli e60c16167cc42cff76f231adfbf31c44711932389e7262c7b2dd1d843d365c17、daemon 6682c7395f1d9ca6a7aa840f7d6f9798c96f0257a698bd7c30e5632632d12950。
- 当前 release/secretary 与 release/secretaryd 实测哈希与上述两值逐一相等（未变更，无自动失败事项）；release/SHA256SUMS 亦在册同两哈希。绑定核对 PASS。（历史 final-build-manifest.json 记录的旧构建 60abe73b/6b96e377 属更早版本，仅记录。）另：6682c7… 亦见于 acceptance-matrix.md 与 final2-offline.json 绑定行，交叉一致。

三、month-run8 计数与审计状态（只核报告）
- 仅本地副本存在：reports/local/live-month-run8/report.json（sha256 1dae0f9ab063b868c028906a31ffa987828fd236a1d39fd974b26800a21e9183，122302B，mtime 09:41）。reports/live-model/ 下无 month-run8 目录 → 公开副本不存在，明确待 publisher 对比 report hash。
- 报告计数：suite=LIVE_MODEL-month-items-v1；failures=0；results 长度 90（与"90 checkpoints 失败 0"一致）；scope 声明显式注明"90 item changes plus 30 generated consciousness snapshots; does not alone cover all A23 world/conflict/source semantics"；snapshot 关键词 31 处且状态 PASS；未见 audit/checkpoint 字段；model_calls 246 份。
- 审计收口状态：本地目录无 report-identity-correction.json / consciousness-reference-audit.json；PUBLIC-EVIDENCE-INDEX.md（09:34）已提前链接 month-run8 的 30 snapshot reference audit 并写"currently reproduce both audit files exactly"，但对应工件尚未落盘于公开目录 → 与"独立审计尚在收口"吻合；按要求不升级为 PASS、不重演模型，仅记录（含索引措辞先行于工件这一观察，未判失败）。

四、等价报告数字口径（非阻断，只读一句）
- registered-extensions-equivalence.json 未更新（仍 691B、无基准注记）。按 Master 口径记录：20214→19770 与 d52b→f5dd 指"生成的 DecisionSchema 子闭包"（非整个 contracts.schema 文件），不重测、不阻断。旁证（只读）：q4 公开 objects 中含 31965B 与 31461B 两份 wire blob，31965−31461=504，与报告 measured_wire_savings_identical_content=504 算术一致，亦与"finalwire 31461≤32000"对应——口径自洽。

参考资料：
- reports/local/live-question-run4/report.json、reports/live-model/question-run4/{report.json, publication-manifest.json, precall-bindings.json, program-identity-bindings.json}、reports/local/live-question-run4/{cli-transcript.json, durable-state.json, diagnostics/}
- release/{secretary, secretaryd, SHA256SUMS}
- reports/local/live-month-run8/report.json；reports/live-model/PUBLIC-EVIDENCE-INDEX.md
- reports/implementation/registered-extensions-equivalence.json；证据脚本 /tmp/d12_verify_ayanami/{archive_check.py, verify_schema.py}

思路：先独立复算双副本 sha 并逐字节比对；再以「报告字段↔独立工件」逐环节配对，缺工件的（重启、幂等重放）明确限界而不升格；版本项以报告绑定值与当前文件实测值直接相等确定；月报只核计数与审计可得性，把"收口/待 publisher"如实保留；全程未触模型、未写项目文件，429/400 未发生，未使用任何额度相关工具。

( _ _ ) 补核结束。若需 publisher 侧 month-run8 落盘后复对 report hash，或对 q4 重启/幂等两项做下一轮专用取证，请另行开轮。
