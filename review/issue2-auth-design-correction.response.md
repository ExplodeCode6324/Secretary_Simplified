接受 Master 的措辞更正：本轮结论仅表示"未通过 Secretary 产品路径发起模型调用、未使用任何 provider key"，不涉及 reviewer 自身身份；本轮由真实 DeepSeek 会话完成，措辞为我的表述不严谨，已按此理解归档。

同 session 收口，逐条明确表态：四点全部同意，无反对项；只对两处做精确化措辞（不改边界、不加门槛）。

一、C3 收紧（同意，采纳更严边界）
  同意并替换我上一轮的回退规则：
  - 省略与显式必须分开裁决，且省略判定按 raw 字段存在性在 Decode 之前完成。
  - stored-session 回退仅在"HTTP 明确省略 session 字段 + 命中同 principal/request_id"时启用：补原 session 后重算 hash，等值 → 返回原回执原文（原 session、原 hash、不分配 seq、不建对象、不新建分支）。
  - 显式提供不同 session：一律以实际提交的原信封 hash 判定，不得回退、不得改写为精确重放；不等 → IDEMPOTENCY_CONFLICT。
  - 直接 Store（含 Typed）提交完整信封：按完整信封精确 hash，无任何回退。
  - 旧真实原 session 的完整信封：仍先重放、后 authority 检查（顺序同 C2）。
  精确化两点（非新条件）：回退路径不得产生任何写入（回执只读返回）；回退不得在"无同 request_id 回执"时被触发，那种情况按新受理走权威绑定。

二、C1 修正（同意）
  既有"未知 version 2"的旧测试代码与报告保留、不改写成 3，作为历史证据不动；本轮 Open/backup 的版本识别变更只需编译/静态检查 + 新增一个 Issue2 定向短样例，用 version 3 验证"未知链仍拒绝"。
  不要求旧套件或全仓。补充一句事实供实现参考（不作为证据）：旧用例在 002 存在后仍会被拒，但原因是 checksum 无匹配，而非版本号；因此不得依赖它作为本轮 unknown-chain 证据。
  精确化一点：Open 与 backup/restore 必须共用同一个版本识别 helper，避免两处漂移；backup 仍不得接受未来版本。

三、C9 修正（同意）
  撤销我上一轮"409 body 必带 head turn_id/accepted_seq"的要求。错误信封保持现状，不强制新增字段；客户端需要时从同一 authority snapshot（GET /v1/conversation）只读查询。保留的仅此三条：409 与 BACKPRESSURE 代码可区分且稳定；零受理副作用（不写回执、不分配 accepted_seq、不产对象）；不消耗 request_id。

四、foundation/root 补充（同意，并入本轮条件）
  F1 Upgrade 持锁与停止语义：Upgrade 显式持有实际 dataDir/run 下两角色各自持有的锁文件（core.lock、runner.lock 为准，以仓库实际路径与既有 helper 为准，不新造并行机制）后再迁移；两角色必须已停止，取锁失败即中止并给出可操作提示；任何情况下不得 kill 进程。若仓库现有角色的启动锁路径与假设名不同，以实际为准。
  F2 registry 自动新生与选择：仅当该库不存在任何 MASTER 历史（跨全部会话统计，含非默认会话）时，才自动登记新的随机权威会话；存在 MASTER 历史则必须显式选择；internal-only（SYSTEM/internal 任务来源、非 Master 面向的会话）不可被选为权威，legacy 一律 READ_ONLY 映射，不合库、不改旧 hash、不改旧 ID。
  F3 history 形状：支持 direction=backward（初始取 tail）+ before_sequence 回翻，返回仍为正序；首调不得扫全库 —— 该点以 (session_id, sequence) 的索引访问路径为准，若 001 无对应索引，允许 002 additive 补索引，实现时以实际查询计划确认，不新增性能测试门槛。
  F4 requests/turn 读法：沿用既有 /v1/requests（保留 turn_id/state 回执，现实现即含 reply/operation_keys/extensions），再 GET /v1/turns/{id} 取完整 turn；两块均无需改形状。

五、frozen prefix eligibility 修正（同意）
  纳入已终结失败的前轮：eligible = 所有已终结轮次（非 PENDING/PROCESSING），包含失败终结的原文与失败结果（当前代码的失败终结同样落为终态 turn，其 ASSISTANT 事件为固定失败文本），不得只取成功提交而遗漏失败历史。摘要侧同理：watermark 只在"非终结事件"处设界，失败终结事件与文本可被覆盖。精确化一点：若实现层引入显式 FAILED 终态，必须同步纳入 eligible 集合与"无假成功"渲染规则；未引入时以现有状态集（PENDING/PROCESSING 非终结，其余终结）为准。不新增旧 Context 质量测试。

六、最终条件集（供开工，不再要求许可）
  生效版本：C1(按第二点修正)、C2 不变、C3(按第一点修正)、C4 不变、C5 不变、C6 不变、C7 不变、C8 不变、C9(按第三点修正)、C10 不变、C11 不变、C12 不变，另加 F1-F4 与第五点 eligibility 修正。
  阻断项维持 C1/C3/C4(有旧 pending 时)/C5/C7/C8；其余为必做。范围仍限 AUTH-01—08、TUI、DOC、BUILD，结构新增仅 authority_registry/authority_turn 一组迁移样例，不扩大历史恢复矩阵，不重开 issue #1。

状态：整份方案 + 上述生效条件集足以开工，无需再次许可；代码未改，本轮无构建/测试执行、无产品模型调用、无 key、无生产修改，不宣称任何测试通过。
