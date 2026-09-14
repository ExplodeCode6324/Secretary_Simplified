Ayanami，Master指定仅deepseek-v4.1-flash/opencode-go。D12已按最终契约冻结，请实际独立增量复核Core/contract/store/context/memory/diagnostics/CLI入口这一组，Runtime/World另会话审。不要全仓审计，不将实现者测试自述当验收。先读 review/D12-output-class-final-contract.response.md 和 D12-injection-oracle-clarification.response.md 最终规则（初稿已撤销），READ_MEMORY-final-answer-clarification.response.md仅prompt语义。不得因初稿要求顶层class或SECRET伪标复开。
有限清单/实现线索：
- contract/classification.go+schema strict security.classification {data_class:enum}程序独占；legacy缺/非法=OUTPUT_CLASS_UNKNOWN，不持久伪SECRET、不默认SYN。
- store/core_repo/questions/retrieval/memory_repo/items，Core成功reqclass→apply所有actions/controls、FinishDecisionTurn/event/State问题聚合；old max(new)不可降。原InputTurn class/EvidenceRef保持真实原字节。
- Context Build/CheckDisclosure双保险；model/input_policy.go按精确schema路径检查derived carriers必须mark，unknown拒绝（重点避免把普通数据误当carrier与漏递归）；RecordingModel/Provider拒伪security.classification注入。
- memory summary/slot标记，包括既存slot重读Class/Allows；ModelCallRecord+raw wire/output ObjectRef+manifest+attempts实际reqclass，不把PERSONAL档案写SYN。
- TypedClass/普通CLI input/chat/API默认PERSONAL，显式SYN支持；RawMessage区分缺省与null/空非法；保持AllowedClasses不扩权。
- tests/output_class_propagation_test.go M1/M2/M5/M6及classification_ingress_test.go M7/M8；foundation定向race通过仅作线索。请隔离复制源码，执行这些有意义定向测试并写独立关键canary反例。验证SYN-only不成功携PERSONAL；PERSONAL allowed正对照必须能带canary，非一律拒绝。
- READ_MEMORY system roleInstruction按现有控制语义追加最终controls=[]，不得静默移除control、不增预算/互斥/oracle。真实run2失败保留，最终live链未运行，明确NOT_RUN，不用夹具替代。
业务源码只读；测试/输出仅临时隔离副本，禁止写同名review报告；不委派、不碰凭据/ELIZA/真实数据/真实模型/响铃/其他进程，不跑全仓test/race/vet。可参考 reports/implementation/M2-core-memory.md与D12相关机械报告，但必须实际检查。
目标：M1/M2/M5/M6/M7/M8及审计传播是否闭合、具体实证mustfix、范围边界。无证据理论附加项别升级退出门槛；必要失败必须准确定位并保存独立probe。保留原始失败（自身probe错误需区分）。预留3轮自然最终报告，不要耗到强制总结MissingSessionID。429立停报告，不自行换model/provider。
