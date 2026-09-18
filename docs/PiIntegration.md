# Pi 接入设计与源码结论

## 1. 运行基线

运行候选锁定：Node `24.21.0`；`@earendil-works/pi-agent-core=0.85.1`、`@earendil-works/pi-ai=0.85.1`；对应发布 gitHead/tag 为 `d981de1229ef899957bbe968bc8dcda02a21f477` / `v0.85.1`。完整源码位于仓库根 `pi_src_origin/`，保持 detached HEAD 且无本地改动。安装依赖锁与 integrity 在 [研究 package-lock](research/package-lock.json)，源码哈希在 [upstream-lock](research/upstream-lock.json)。

初次评审的 main 提交 `a8b3dd1` 与发布包虽同号，Context 接口已经不同。实际探针首次因此失败：发布版的 systemPrompt/tools 在 Context 顶层，较新 main 将它们放入 transcript system messages。实现以本节发布基线为准；不要把初评 main 的函数签名复制到当前 adapter。

`pi_src_origin` 是本地独立上游 checkout，外层 Git 忽略它，避免提交整个嵌套仓库。没有它时按 [研究 README](research/README.md) 的固定 SHA 步骤重建。

## 2. 代码调用链与使用点

下列路径均相对 `pi_src_origin/`；精确符号位置及哈希由 [source-map.json](research/source-map.json) 固定。

| 路径/符号 | 观察到的行为 | Secretary 使用方式 |
| --- | --- | --- |
| packages/agent/src/agent.ts / Agent constructor | 显式 streamFn；默认工具并行 | `src/pi/runtime.ts` 注入 gateway streamFn 和 sequential |
| 同文件 / prompt、abort、waitForIdle | prompt 等待循环完成；abort 发信号；idle 晚于终止事件监听器 | 调度器持有 Promise 而不阻塞入口；停止后等待 idle，再轮换 |
| 同文件 / subscribe、processEvents | 按订阅顺序等待监听 Promise | `src/pi/journal.ts` 建消息落盘屏障；失败 abort 并停止业务派发 |
| packages/agent/src/agent-loop.ts / streamAssistantResponse | transformContext → convertToLlm → 含 systemPrompt/messages/tools 的 Context → streamFn | ContextBuilder 显式提供固定系统规则与保留历史；不只计算 messages 长度 |
| 同文件 / prepareToolCall | 参数校验在 beforeToolCall 前，钩子可改 validated args | `src/execution/gateway.ts` 在 execute 再校验最终参数 |
| 同文件 / executeToolCalls、executeToolCallsSequential | 串行/并行可选；tool batch 与结果有确定次序 | 初版串行，仍按 toolCallId 关联原始记录 |
| packages/agent/src/types.ts / StreamFn | 请求错误应返回 error/aborted 流终态 | 网关门禁拒绝生成错误流、真实请求数为零，不悬挂 result() |
| packages/ai/src/api/openai-responses.ts / stream | buildParams 后 await onPayload，再 responses.create；异常转错误流 | 直接调用该 adapter，提供不可替换的 onPayload 与受控 fetch |
| 同文件 / requestOptions、retryProviderRequest | SDK maxRetries=0，但 Pi 外层还有 retry helper | 同时设置 options.maxRetries=0，由 Secretary supervisor 统一重试 |
| packages/ai/src/utils/event-stream.ts | 推送 done/error 后 result 可结束 | 复用事件流类型，不自行伪造 Promise 协议 |
| packages/coding-agent/src/core/extensions/runner.ts | 部分钩子异常记录后继续 | 不作为本版硬门禁或模型出口 |
| packages/coding-agent/src/core/session-manager.ts | 会话文件持久化与业务确认不同 | 不作为权威存储，不创建双写会话库 |
| packages/session-backends/sqlite-node/src/index.ts | 包装 Node SQLite 为 Pi SessionRepo | 阅读其驱动用法；不复用为任务/授权数据库 |

源文件所用的 package API 与 runtime 发布物都在研究依赖锁中；源码位置调整不等于 API 不变，升级必须重新运行探针和关键验收。

## 3. 是否修改上游

**本版预计零个 Pi 源码补丁。** 需要的控制点已由 Agent 的构造参数、受控工具 execute、直接 provider adapter、awaited listener 和历史重建提供。新增代码全部放 Secretary `src/`，不用 fork Pi 的业务逻辑。也不启用 Agent Core 导出的完整 harness/SessionRepo/默认 compaction；它们会引入另一套生命周期。

如后续某 provider 无法保证最终载荷门禁/禁用隐式重试，先将该 profile 标为 UNSUPPORTED。只有有失败用例证明公共接口无法满足契约时，才在单独上游 worktree 修改具体 adapter，并保存最小补丁、版本与回归；不得直接编辑 `pi_src_origin` 再当作原版使用。

## 4. runtime 与消息映射

`createSecretaryAgent(trustedContext, model, systemPrompt, journal, gateway, messages, tools)` 只接收程序构建的参数；参考可编译 [blueprint.ts](research/blueprint.ts)。

- systemPrompt 使用 [main prompt](prompts/main.md) 的固定模板；动态状态以受控消息块加入。发布版 `initialState.systemPrompt` 单独传入。
- messages 是保存/恢复的合法 user/assistant/toolResult 序列。稳定外部 record ID 映射到 Pi toolCallId，不使用模型文本做去重键。
- 主工具集来自代码固定 registry；不读 `.pi/`、AGENTS.md、skills 或工作区可执行扩展。安装期可信配置可加入经过检查的工具，模型不能动态注册。
- 每次 message_end 保存完整最终消息；token 增量仅用于 UI，可丢失，不必每 token 写库。实际工具证据由网关独立保存，不能只依赖 Agent 的工具结果通知。
- 应用语义的任务成功只能由 CommitService 产生；agent_end 表示 Pi 循环结束，不等价于业务成功。
- journal 出错应设置全局 STORAGE_DEGRADED、abort，停止新 tool/provider 调用；就算 Pi 将异常转成错误消息，也不能放开协调器门禁。
- 轮换前等待 idle，卸载监听器、释放旧 Agent 和结果缓冲，再从持久状态重组。unfinished tool-call 区间不能剪断。

## 5. 模型网关的具体实现顺序

初版只实现一个明确配置的 Responses 协议 profile 与 fixture；这里选择的是协议 adapter，不自动选择 OpenAI 账号/任何真实供应商。

1. `src/model/gateway.ts` 创建 model_call_id，检查真实调用模式、deadline、允许分类、预算和 abort signal。
2. 在事务中预留额度，构建 ContextManifest，选择固定 `model` 和认证信息；输入不能覆盖 baseUrl、fetch、onPayload、headers 或重试参数。
3. 调用 `@earendil-works/pi-ai/api/openai-responses` 的 `stream`，传入 `maxRetries:0`、超时、maxTokens、signal、受控 fetch 与 onPayload。
4. onPayload 接收实际 provider 参数后完成最终预算、classification、系统规则/工具存在性检查，冻结/记录其 hash。失败直接抛给该直接 adapter，后者产生 error 流且不触发 fetch。不要通过 coding-agent 的扩展分发器绕一层。
5. 受控 fetch 再检查 URL/headers 允许值、最终 JSON body 与允许 hash 一致、epoch/取消/披露策略仍有效，持久记录发送意图，才调用真正 transport。测试把 transport 替换为计数器；业务参数不能替换它。
6. 每次真实 fetch 至多一次；503/429 的重试由 supervisor 新建 model_call_id、复用根预算。部分流响应不得当作完整回复提交；usage 未知保守记账。
7. 将流完整消费到终态，写模型结果对象并一次性结算预算；任务/事实提案仍走业务校验。

序列化后 token 计数器作为 profile 的必需能力：初版用提供方匹配 tokenizer 或已实测的保守上界；计数方式、误差界和字段覆盖必须入 profile。fixture 使用可精确计数的假 tokenizer，不能据此声称真实 token 上界已验证。P3 启用真实 profile 前补真实 usage 对照。

## 6. 已有研究与后续 P1 门槛

[探针](research/probes.test.mjs) 使用发布包与假模型/fetch 验证：事件监听等待、串行工具、钩子后复验、阻止工具、拒绝载荷零 fetch、允许载荷一次 fetch、abort、历史轮换及真实 Node SQLite 空库/备份。没有调用真实提供方。

这些是 **UPSTREAM_PROBE**，比静态阅读多一步，但不等于 Secretary P1/P2 已完成。仍需集成自己的 Journal/ObjectStore/协调器后验证：落盘失败阻断、输入恢复、预算并发、撤权竞态、沙箱实际能力与不可用路径、断电假设、真正执行者恢复。
