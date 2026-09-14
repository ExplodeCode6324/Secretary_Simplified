短收口。仅基于既有事实与本轮补充，不重审、无工具。

证据口径更正
- 初稿"未调用模型"仅指未调用 Secretary 被测模型；reviewer 本会话自身为真实 DeepSeek 调用（deepseek-v4.1-flash / opencode-go，Master 指定），属 reviewer 侧成本与痕迹，不计入产品证据层（LIVE_MODEL/REAL_USE 不受此影响，仍以产品侧真实调用为准）。
- runtime agent 已动态复现：取消 socket 后 CoreWork 保持 RUNNING、receipt=nil（真实 Unix HTTP + fake 模型，before 日志已保留）。复现事实接受，作为动态证据与本轮静态裁决分开记录；我方未在本轮独立重跑。

同意实现的规则（收口版）
1. FinishWorkTx 接受条件＝(run, task, attempt, fence, command_hash 同代) + job_run ∈ {RUNNING, RESULT_UNKNOWN} + permit(run_id, fence).cancel_generation 校验；只写 core_work，不碰 job_run；CANCELLED 的 run 不接受。
2. 迟到/取消竞争标记只沿用已注册的 runtime.cancellation shape（{effect_observed, cancel_generation}），不得新增未登记扩展字段。
3. core_work 已有 final receipt 时重复结果幂等（同结果 no-op），不允许真实结果被反复改写；不同结果拒绝。
4. 同代"有证据 FAILED/effect=false"到达（如对账取回 Core 迟到收据）时，允许原 run 就 RESULT_UNKNOWN + attempt.dispatch_state=FINISHED 的状态进入既有重试：次数上限与 1s/5s 间隔（memory.refresh 5m/30m）不变；FINISHED 仅在收据到达前 run 为 RESULT_UNKNOWN 时可重试；active_ms 结算仍由 dispatch_state != FINISHED 一次性守卫，同一 attempt 不得重复结算。这不是无证据 UNKNOWN 的盲重试。
5. briefing 成功唯一证据＝本次 attempt 经 foundation 已批准的 Store.WriteObjects（flock+tx）与 FinishWorkTx 原子绑定的 ObjectRef；不做旧对象搜索式成功。
6. memory slot：Core GET 保持只读、不改动；exact slot 证明放在 Runner.reconcile，用同库现有 Store 只读查证（合法 DTO + 命令 slot + root 谱系 + 当前 attempt/fence），程序生成 reconciled receipt 后走既有 RecordReceipt 事务持久；不新增 HTTP 接口/DDL；查不实即维持 UNKNOWN。GET 暗写 CoreWork 一律不做。
7. 更正初稿：remote_query.go 需收紧——跨代重绑定仅允许可独立验证的 SUCCEEDED 效果；FAILED/CANCELLED 仅同 attempt/fence，否则返回 not-known 保持诊断，不得用旧失败覆盖新执行。
8. BeginWork 同代 RUNNING 拒重复 Execute（稳定错误码，不进模型调用）。
9. 收尾仅 5s WithTimeout(WithoutCancel(ctx)) 做持久化；业务全程 HTTP 取消 ctx；无后台延长执行。
10. AUD04 维持现有实现（O_NONBLOCK open、非 regular 拒、max+1 实际读限、正确 close）；只补一条定向集成：拒绝后 source cursor 不变、无成功 receipt。

不变量（保持）：取消 Task 不 SUCCEEDED、不复活；旧 attempt/fence 只审计不覆盖；5s 仅持久化；无新 DTO/DDL；soak 与其 data/binary、原证据原 hash 不动；不重跑两小时、不新增任何替代时长门槛。

待实施后独立轮次：定向回归 + race + build + 短 HTTP fake 模型 e2e（六场景），届时形成实测证据并与本静态收口分层报告。
