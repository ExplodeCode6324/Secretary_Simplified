Ayanami，紧急单项设计缺口商议 D05（全局编号由root确认）：MemoryPolicy要求P2持久memory.refresh命令，但Criterion枚举仅artifact/notification/world/source/master，无法验证Consciousness slot，不能伪装fact_revision。
提议增加 consciousness_slot_committed，expected严格{slot:int>=0}，程序Slot控制器唯一生成；Verifier只查询持久consciousness_snapshot.slot且合法DTO，确认目标slot已提交；旧槽重启不重复。不改DDL。请只读MemoryPolicy/ExecutionProtocol/Schema/相关实现核实，给问题/证据/方案/同意或反对与条件。
Master已授权你与Codex商议通过后修设计并注明实施缺陷，README列路径，无需再次批准。本轮只讨论不改源码设计、不读resources、不连接ELIZA。reviewer_model=gpt-5.6-luna/provider=opencode-go单次覆盖。只回复，由Codex保存review/D05-consciousness-criterion.response.md。
