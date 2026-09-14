确认：可作为正常 prompt 修复，无协议冲突。引擎语义本就如此（control 仅探索、带 control 的回复不提交、每次 READ_MEMORY 计检索预算），该 roleInstruction 只是把既有状态机语义显式告知模型（最终答案 controls=[]；无足够信息时照旧检索并受预算约束、不伪造答案），落在既有 system_rules/role 指令位即可，不动 DTO、互斥校验、预算、oracle 与 D12 契约。

复验口径：原 run2 失败原样保留；用原自然语言模板与同 oracle 重跑新一次；若模型仍带 control 答案则照旧失败（零放宽、不静默接受）。Codex 保存此短结论。
