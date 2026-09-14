Ayanami，Master指定仍deepseek-v4.1-flash/opencode-go。最终两处等价增量已冻结，D12 Core/Runtime均独立PASS，不重开全审。请只核Schema $ref提取+model.go提示同义精简+README修改路径说明，以及CLI帮助data-class默认说明。
旧独立受审快照 /tmp/d12_review/iso1/src；当前项目src。报告线索 reports/implementation/registered-extensions-equivalence.json（作者称40相同shape、展开新增$defs.RegisteredExtensions后整个JSON deep-equal，不采信自述请你程序复算）。原DecisionSchema20214→19770；旧model roleInstruction1869→1137B。实现src/contract/generate.py Go RegisteredExtensions alias map保持兼容；tests TestRegisteredExtensionsEquivalentToOriginalInlineShape原shape/40carrier/非法class拒绝；docs/Common说明纯编码去重，README列路径。
必须实际核证：
1. 比较旧/新完整schema，展开RegisteredExtensions并去仅新增未直接约束数据的helper def，深相等（保留其他$refs语义）；输出相等/差异具体path。严格extension registry/DataClass enum/不可注入约束与完整output_contract闭包不变。定向跑contract该新等价测试及必要invalid class/闭包用例即可，不全仓test/race/vet。
2. 逐条对照旧新roleInstruction：控制/动作互斥、合法ID/readset/不猜引用、D10默认FIRE_ONCE_WITHIN_GRACE300且NL自定义只Typed、D11程序问题/显式回答、READ_MEMORY已够则controls=[]且不重复同query/nullcursor、数据/authority规则等实际原有约束无删除/反转。不宣称自然语言形式化等价；明确源码规则对照证据。
3. README/正式Common修改路径说明存在，普通CLI帮助--data-class默认PERSONAL与显式SYN示例准确。
4. 若reports/local/live-question-run4或month-run8已完整，仅可只读说明有报告，不重打模型；尚未完成明确待后续。当前作者报告实际带grant原run3 Context只读复原finalwire31461<=32000，不改原DB/noAPI，可核其报告但不冒充真实链。
最新冻结构建cli e60c16167cc42cff76f231adfbf31c44711932389e7262c7b2dd1d843d365c17、daemon6682c7395f1d9ca6a7aa840f7d6f9798c96f0257a698bd7c30e5632632d12950。报告需以源码快照限定，真实CLI与月回放另存档验收。任何实质语义差异给具体依据；纯等价重构不用设计再批准，不加门槛。
业务源码只读，临时脚本/测试只在隔离目录，不委派、不触凭据/真实数据/API/其他进程，不改同名report。预留3轮自然总结，最多有限工具批次，不扩展其他模块。429立即停止报告，400 MissingSessionID按既定同session正常总结，绝不换reviewer。
