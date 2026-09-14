Ayanami，Master指定仅deepseek-v4.1-flash/opencode-go。D12 Runtime/World模块冻结，请实际独立增量复核，仅有限M3/M4链。Core/contract/Memory另会话审，不重跑全仓。有效设计是 review/D12-output-class-final-contract.response.md + D12-injection-oracle-clarification.response.md，初稿顶层class/邻近advisory/SECRET伪标均撤销。
线索：src/store/runtime_classification.go/runtime*.go/world_repo.go + executor/executor.go；src/tests/runtime_d12_test.go 和 reports/implementation/D12-runtime-checks.json。
- strict程序security.classification extension，Job/Command→Task/Run/Attempt/Permit/Receipt/notification/alarm/REPLAN/WAIT完整join不降，旧unknown不猜SYN、OUTPUT_CLASS_UNKNOWN；authorization扩展merge；出站metadata/产物继承durableclass。
- SUBMIT_ARTIFACT从DB ref取class，伪低不能降低；artifact.write后metadata fault保留effect=true RESULT_UNKNOWN、无盲retry。
- WorldProposal→WorldFact join旧版本，class低的引用不证明整个proposal低；内部program/class绑定不受客户端注入，proposalhash/permit/receipt权限不因升级绕过。
- 特别独立审：rtSaveRun不将新receipt较高class反写旧Command（固定command hash必须稳）；PutProposalTx evidence升级后旧caller p hash会拒绝，root说当前工作路径读取DBproposal正确，必须查实际而不是采信。
- D12更新不可破既有D08 currentRun/REPLAN fixedcriteria、stale fence/cancel+permit、artifact效果不确定隔离。仅回归相关关键探针，不要求重审所有旧模块。
请在隔离临时源码副本执行runtime_d12目标测试，至少自写/改造关键反例验证req PERSONAL+低引用/伪低artifact class、RunHash稳定、world版本不降，清楚列出实际命令/结果/源码hash。禁止修改业务树/同名review报告，不委派、不触resources凭据/真实模型/真实数据/响铃/其他服务；无全仓test/race/vet。
目标是具体M3/M4闭合裁决；只将实证缺陷或明确现有契约缺口列mustfix，不把任意额外分支NOT_RUN加新门槛。真实CLI未完成另验，不拿夹具当真实语义。预留3轮自然最终报告；遇429立停报告，不自行换reviewer或provider。
