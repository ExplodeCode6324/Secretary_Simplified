Ayanami，M3-06 DST gap持久审计修复拟采用以下方案（暂全局编号D06，以root最终编号为准），请确认可否按此修及文档最小增补：
登记schedule.skipped→ScheduledJob（entity_type scheduled_job），before/after完整同revision计划，ChangeEvent.created_at为扫描时间，extensions runtime.calendar_skip={local_date:'YYYY-MM-DD',timezone:string,local_time:'HH:MM',reason:'DST_GAP'}；避免不存在local时刻伪造JobRun.scheduled_for UTC。以jobID+localdate事件稳定ID/UNIQUE防重复，计划next_due推进同事务。
这是响应你M3报告“SKIPPED记录或calendar-skip event”建议之一。请只读相关TypeRegistry/ExecutionProtocol/Schema，给具体同意/反对及必要条件，确认同revision before/after仅作为无业务变更的skip审计是否需要单独声明，避免与event防自触发冲突。
Master已授权双方商议通过后修设计、注明实施缺陷、README路径，不需再申请。本次不修改文件，不读resources，不连ELIZA。reviewer_model=gpt-5.6-luna单次覆盖。只输出回复，由Codex存review/D06-calendar-skip.response.md。
