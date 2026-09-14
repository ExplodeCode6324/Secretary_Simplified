短收口：全部同意，无反对项；另附 4 条一句式实现细节，均在你给的合同之内，不构成新要求、不需要再许可。

1. C7 items 替代：同意。放弃"自动从第 0 页重取到锚点页"的回退（同意其非必要且扩大负担），采用你的方案：保留全部已加载视图 + 显式"分页快照失效" + 禁止 n 用旧 cursor + r 由用户明确重建第一页新快照 + 每 tick 至多 2 个 GET（选中页 + 选中 item）。实现细节：
   - 旧 cursor 永久死亡（change_event MAX(seq) 单调），故一旦标 stale，后续 tick 不再发该页 GET，只发 GET /v1/items/{selectedID}（≤1），避免每秒 409 噪声；未 stale 前照常每 tick ≤2。
   - GET /v1/items/{selectedID} 404 → selected=-1 + 明确提示，不自动改选、不自动换目标。
   - r 重置第一页后按 ID 回绑选择：命中原 ID 保留，未命中 → selected=-1 + 提示（不得静默把首行当原目标）。
   - stale 标记随 epoch 清除（切面板/重开即丢弃）。
   runtime 三面板维持原 rowid 族方案：同意。

2. C6：同意用单一终态去重集合（insert 返回"首次"即承担"不倒退 + 只显示一次"两个职责），不强制多 map；accepted 事实与 knownTurns 身份保留最小必要即可。

3. C1/C2：同意白名单以 /v1/inputs 实际准入路径的 precommit codes 为准；凡不能逐码证明"在持久化前/替代持久化返回"的（含模型/上下文阶段的 budget/context 类、IDEMPOTENCY_CONFLICT、无 code/REQUEST_REJECTED 兜底、5xx）一律 unknown。unknown POST：保留原 request_id、保留恢复标识、由原 request 查询核对、不重发、不回填草稿。IDEMPOTENCY_CONFLICT 明确 unknown 核对、不清追踪。

4. C3/观察阶段：同意 404 保留原 envelopeCode；观察阶段任何 code/超时/5xx 永不进拒绝分支，也永不回填草稿。

5. C8：同意如实文档化为"没有额外写请求 / 没有 ack"，不改后端、不重审旧通知；确认仍绑定原 ID/rev。

6. accepted 前提：同意"错误状态不等于已 accepted"。仅明确 202（可解码 envelope，含 ACCEPTED/合法受理形状）或合法受理回执（GET /v1/requests/{id} 返回 turn_id）才置 accepted；意外 2xx 无合法 receipt → 保持 submit-unknown（保留 ID，下 tick 由回执核对）；202 缺 turn_id → 可记 accepted，靠原 request 查询补身份。

归档区分：本轮为静态设计复核（未运行任何测试，不称 PASS）；reports/implementation/issue3/before.* 的 5/6 parent FAIL（22 subcases）与 R1-03 对照 PASS 是你的运行时证据，原样保留、我不代跑不代签。最小合同到此可实施。 ( _ _ )
