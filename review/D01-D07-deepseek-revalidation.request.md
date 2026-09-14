Ayanami，Master指定必须DeepSeek v4.1 Flash；本轮新会话避免长历史。请直接对D01–D07既有设计修订进行独立有效性复核，只读，不委派子agent，不跑全仓测试。此前Luna意见仅线索，不能沿用其赞同。
请先用rg定位README设计缺陷表和docs中D01-D07标记，只读修改段与必要Schema/实现契约片段，不通读全部大文档。
D01 scheduled_job.updated登记事件与计划同tx（原DeepSeek曾同意，后续仍核对）。
D02 recurring notification确定性occ:v1 base64url绑定command/criterion后冻结，模板不变。
D03合法deleted tombstone允许VALID normalized=null/error=null（原DeepSeek M1指出）。
D04 A05重复对象包括object_ref，先dedup/验证再对象/quarantine。
D05 consciousness_slot_committed精确slot+DTO，SlotController唯一程序生成持久memory.refresh，固定epoch、command目标slot，移除旁路。
D06 scheduled_job.skipped audit-only事件，完整same revision before/after游标推进，严格calendar_skip，稳定rule/date ID+同tx幂等、防自触发，不造假不存在local UTC。
D07 Context移除自引用manifest；身份/as_of/snapshot_seq+严格sections含retrieved_evidence+output_contract闭包仅一次，外Manifest readset/hash精确最终wire，不伪hash；检索计数/bytes语义准确。
逐D项给认可/需修订/证据不足与具体矛盾、最小修正及文档路径；必须确认固定criteria、授权、幂等、快照/hash等原则未被弱化。D08已由你另会话裁决，本轮不重审D08。
这是设计审议，允许只读源码核实可实施性但不对全模块验收背书。禁止resources/ELIZA/受测API/响铃/修改文件。若429/连接失败即报告不换模型。reviewer_model=deepseek-v4.1-flash/provider opencode-go。仅输出，Codex保存review/D01-D07-deepseek-revalidation.response.md。
