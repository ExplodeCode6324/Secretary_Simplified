Ayanami，请直接对当前M3 runtime新快照复核原M3-01至07，不再委派子agent/线程。实现者修复说明需独立验证，不复述即通过。
M3-01 local查expiry/cap/attempt/hash并同tx消费permit；artifact对比持久permit/expiry/current fence/cancel后消费。
M3-02 Runner active task→cancel context，已执行RemoteCore HTTP context可取消。
M3-03 Receipt保留Task CANCELLED同时run SUCCEEDED/effect observed。
M3-04 command canonical hash与durable绑定。
M3-05 alarm expiry同tx sessionSTOPPED+runUNKNOWN，后补effect receipt/预算/Verifier，崩溃下一扫描仍补。
M3-06 D06已商议同意，scheduled_job.skipped同tx before/after推进+稳定ID；DSTGapHasAtomicAudit。
M3-07 event忽略自身Task/calendar audit，实际effect hash无变化skip，consumer_cursor同tx持久，Task含causationID。
原独立probe已复制src/tests/runtime_ayanami_probe_test.go，5项均过；实现者双进程fixture集成PASS(2提醒、Core停后第二仍通知、ItemOPEN、双token交叉401)，报告review/M3-runtime-process-integration.json。
请用你自己的原反例重跑当前快照并挑剩余高危边界，给固定/未固定/未覆盖与行号/具体证据。只读业务/设计、隔离安全测试，禁止resources/ELIZA/响铃/改系统；不得以旧快照缺陷称当前未修。reviewer_model=gpt-5.6-luna/provider=opencode-go单次覆盖。输出保存review/M3-runtime-fix-ayanami.md。
