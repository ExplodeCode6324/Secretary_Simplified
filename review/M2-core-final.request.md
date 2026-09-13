Ayanami，请直接对M2 core/context/model/memory当前冻结源码作真实修复收口，不委派子线程。原报告review/M2-core-memory-ayanami.md的F1-F10请逐项独立检查。
当前基础/Core冻结：D07 Context/外Manifest实际wire方案、反馈、READ_MEMORY、grant、summary全部接好；D05持久memory.refresh和slot也已接，具体实现以当前源码为准。请读相关实现与最新测试，不把旧快照问题自动当当前未修，也不能采信自测即通过。
重点旧高危：Provider class/Context规范边界、模型action grant/typed身份、持久epoch、模型原始输出与语义retry记录、pendingQuestions、完整Context+output_contract+外Manifest精确hash；旧中危readset、归档幂等、memory预算/omitted、summary durable retry。请执行针对旧反例的独立安全探针、列实际命令/结果和仍未完成项。受测真实模型调用仍由root负责，不读resources凭据；无ELIZA/外部消息/响铃/系统修改。业务源码只读，测试在隔离目录。
本次reviewer_model=gpt-5.6-luna/provider=opencode-go调用级覆盖。仅输出报告，由Codex保存review/M2-core-final-ayanami.md，不把完整M2或LIVE_MODEL验收混为一谈。
