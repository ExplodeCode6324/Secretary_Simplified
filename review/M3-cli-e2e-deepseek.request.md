Ayanami，Master要求DeepSeek最终复核。本轮只补已经列明的CLI实机缺口，禁止扩展新静态审计或重复全仓测试；请直接执行集中shell脚本/定向tests，随后明确裁决，不委派。本轮新会话减少历史。
已完成传输/typed UDS独立审查在review/M3-transport-cli-deepseek-final-resume1.response.md（DeepSeek），无需重做；仅按该报告§4.2–4.4补：
1 CLI actions --file合法与未知字段/缺文件；jobs create/trigger CAS+同request幂等；notifications ack；runs cancel通过Task revisionCAS（按设计响应仅cancel_ack，不要自加必须返回revision）；schema搜索，--json和默认输出，doctor，verify smoke/verify-backup。
2 CLI backup→restore新短tmp目录，查新独立tokens（不打印值）+空cap grant+双冻结；启动恢复Runner，证明队列不claim/dispatch/effect，结束仅你创建的进程。
3 定向执行TestRuntimeRemoteQueryRebindsEvidence和真实或fake阻塞Core已持久结果的UNKNOWN查询/不重发探针，记录POST调用数不增加；复用相关测试足够，无须自己实现整套服务。
可先查看src/cmd/secretary help/dispatch实际参数，避免猜命令；所有synthetic配置使用fixture默认，不读resources/真实API。之前评审创建/tmp/m3ds环境与binary，但它不作为本轮当前快照保证；请按需要隔离构建两个入口。
只读业务/设计，禁止ELIZA/响铃/系统设置；token只查布尔/哈希不打印。实际已执行与未覆盖严格区分，要求尽量一次完成全部具体缺口，不再把启动但不收集的后台测试当结束。reviewer_model=deepseek-v4.1-flash/provider opencode-go，不换模型，429报告停止。
输出简洁终审结论+执行命令/证据/未完成项到回复，由Codex保存review/M3-cli-e2e-deepseek.response.md。
