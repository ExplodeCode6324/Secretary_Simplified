Ayanami，Master指定DeepSeek v4.1 Flash真实最终复核。本轮新会话，请直接审当前runtime冻结差异与关键原缺陷，不委派、不跑全仓test/race/vet，只集中定向测试+少量独立反例；已有自测不能替代你的裁决。请主动在完成约束检查后最终回复，不耗尽工具上限。
重点1 旧P1：rtSaveTask criterion_hash字段篡改+零行UPDATE必须拒绝且不发伪事件，rtSaveRun fence/attempt CAS/RowsAffected；原探针src/store/runtime_ayanami_residual_test.go。
重点2 os.Root artifact根dirfd/symlink containment及正常写入；原Lstat→Rename漏洞需独立否证，不泛称安全。
重点3 D08三能力固定criteria与REPLAN修订R1/R2（你已裁决）：runtime_criteria.go当前Task ORDER BY rowid DESC最新已接纳run，旧任一非终态或当前UNKNOWN不得PASS，全部ID/task/attempt/fence/state DTO列一致；三kind严格当前证据。runtime_d08_replan_test.go含双REPLAN/reopen/新旧证据/两序/source旧fence/旧run未决，gate已去；核对文档4处R1。
重点4 RemoteCore FAILED无effect才可重试3次ceil，有effect只1次保原receipt字段；runtime_remote_effect_test.go。Core UNKNOWN Query应查持久结果而不重发。
重点5 D06停机gap已改due→next并runtime_dst_downtime_test.go；UpdateJobRequest六字段白名单/原回执幂等/取消旧队列；既有cancel/fence/expiry/action hash安全不得回归。
请读取review/D08-capability-criteria-deepseek.response.md和D08-replan-current-run-deepseek.response.md必要规则片段，实施证据reports/implementation/D08-runtime-checks.json仅线索。实际测试限定相关名，不重复全仓。输出按重点已固定/需修/未覆盖，附行号与实际命令结果；不声称完整A11-A25或真实设备验收。
只读业务/设计，隔离synthetic，无resources/ELIZA/受测模型/响铃/系统修改。429立即报告禁止换模型。reviewer_model=deepseek-v4.1-flash/provider=opencode-go。报告Codex保存review/M3-runtime-deepseek-final.response.md。
