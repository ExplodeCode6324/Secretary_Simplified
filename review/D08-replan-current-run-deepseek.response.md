D08 补充裁决 — REPLAN 与"当前 run"规则（只读，不写文件）

reviewer_model: deepseek-v4.1-flash / provider opencode-go（本轮一致）。只读了 src/store/runtime_controls.go、runtime_criteria.go、runtime_wait.go、runtime_local.go 相关段、runtime.go createRunTx、001_baseline.sql、rowid 用例；未动文件、未跑测试、未触秘密/ELIZA/受测 API。无 429/连接失败（1 次搜索瞬态 EPERM 已换法恢复）。共 14 次紧凑调用。产出请 Codex 保存 review/D08-replan-current-run-deepseek.response.md。

════════ 一、裁决 ════════
同意 Master 判断：前轮 D08 的"某 Task 恰 1 JobRun"条件在合法 REPLAN 下会把三新 kind 永久压成 UNKNOWN，是我方句式的错误简化，必须改正。定稿方案 = 规则 D08-R1（当前 run := 该 Task 的最大 rowid 已接纳 run，唯一、追加序、与 REPLAN 自身的 previous 选取同一有序定义）+ 防御条 D08-R2（同任务更旧 run 处于非终态 → UNKNOWN，不得 PASS）。不采用 DDL/Schema 变更（Option A），不采用"程序维护 task.current_run_id"（Option B，条件化，见第五节）。绑定不弱化：从"全表恰一行"改为"最新代际唯一行"，一切证据 join 仍严格锁死该 run.id，旧 run（含成功旧 run）结构上不可借用，且禁止按状态回捞任何历史 run。

════════ 二、缺陷确认（证据） ════════
1) 现实现按"总共恰 1"判定：verifyRuntimeCriterion 遍历该 task 全部 job_run，任一列/DTO 不一致即 false，且 `count != 1` 直接 false（src/store/runtime_criteria.go:13-36，关键行 34）。这是三 kind 的唯一 run 入口（被 runtime_local.go:202-203 调用）。
2) REPLAN 合法地给同一 Task 增加新 run：守卫仅禁 RUNNING/RESULT_UNKNOWN（runtime_controls.go:93-99）；取消旧 QUEUED/CLAIMED（112-133）；复制 previous 新建 run（新 id、occurrence_key=prev+":replan:"+id、attempt_no=0、fencing=0、QUEUED，134-152；INSERT 149）；ReplanTx 仅把 task 置 PENDING（runtime_wait.go:148-164）。⇒ REPLAN 后 count≥2，三 kind 永久 false ⇒ UNKNOWN 死锁，确认。
3) run 行生成点全集（INSERT INTO job_run/createRunTx）：runtime.go:226-283（物化/登记，run 与 task 同事务创建，283 行 INSERT）、runtime_controls.go:149（REPLAN）、runtime_schedule.go:243/315、runtime_memory.go:72；REPLAN 之外没有任何路径给"已存在 task"追加 run。全库无 `DELETE FROM job_run`（DELETE 仅在 items.go），src 内无 VACUUM。
4) rowid 即本系统既有的插入序：REPLAN 自身用 `ORDER BY rowid DESC LIMIT 1` 取 previous（runtime_controls.go:101）；receipt 列表用 rowid 倒序（runtime_criteria.go:66）；事件游标以 rowid/seq 做水位（runtime_control.go:338，change_event 有 seq AUTOINCREMENT，001_baseline.sql:175）。job_run 为 `id TEXT PRIMARY KEY`（001_baseline.sql:132-139），无显式 seq，但无删除/无重排 ⇒ rowid 单调。

════════ 三、精确规则（定稿，最小） ════════
D08-R1 当前 run：对 Task t，当前 run := `SELECT id,attempt_no,fencing_token,payload_json FROM job_run WHERE task_id=? ORDER BY rowid DESC LIMIT 1`。若 0 行 → false。对取出的这一行保留现有全部一致性校验（run.ID==id、run.TaskID==t、attempt_no/fencing_token 列与 DTO 一致、JobRun 解码成功）。三 kind 的证据判定全部只用该行的 run.ID 做连接（保持 runtime_criteria.go:38-153 的警报/回执/事件逻辑不变）。
不变量（为什么它是"唯一且已接纳"）：
- 准入点唯一性：run 行只在写事务内插入——物化/登记（runtime.go:283，与 task 同事务）或 REPLAN 控制事务（runtime_controls.go:149）；不存在事务外插入。
- 追加性：无 DELETE、无 VACUUM、无重排；occurrence_key UNIQUE 保证 replan 新行身份唯一（149 行 & 001_baseline.sql:134）。
- 与 REPLAN 同一有序定义：REPLAN 的 previous 与 verifier 的 current 使用同一"最大 rowid"语义，二者不会分歧；REPLAN 之后 current 必然是新 QUEUED run。
- 排斥性：旧 run 的证据以旧 run.id 为键（alarm_session.run_id 且 DDL UNIQUE(run_id)，001_baseline.sql:190；receipt.run_id；change_event extensions.run_id），当前 run.id 不同 ⇒ 数学上不可满足。禁止任何"按 state 过滤找成功 run"的写法。
D08-R2 防御条（非常规路径转 UNKNOWN）：若存在同任务、rowid 小于当前 run 的 run 处于 {QUEUED, CLAIMED, RUNNING, RESULT_UNKNOWN} 任一状态 → 返回 false。依据：REPLAN 前置禁止 active（93-99）、且 REPLAN 会取消旧 QUEUED/CLAIMED（112-133）、物化只给新 task 建 run ⇒ 合法流程中恒不触发；它只把顺序/并发异常显性化为 UNKNOWN，绝不产生误判 PASS。
其余不动：criteria 冻结与 CRITERION_TAMPERED（runtime_local.go:192-194）、matchCriteria、CANCELLED 不回写 gate（263）均保持。

════════ 四、边界核对（要求的并发/CAS/取消/恢复/旧 fence） ════════
- 并发/串行化：REPLAN 在 Core 控制事务内（ApplyControlTx→runtime_controls.go:106），VerifyTaskTx 在 s.Write 写事务内（176-184）；SQLite 单写者 ⇒ 两序皆安全：①verify 先提交（成功→task SUCCEEDED）→ REPLAN 遭 TERMINAL_TASK（controls 25-28 / replan 154-157）；②REPLAN 先提交 → current=新 run，旧证据不满足 ⇒ UNKNOWN/NEEDS_ATTENTION，不出现旧 PASS。无需额外 CAS 字段。
- 取消：task CANCELLED 时 gate（263）保留状态不回写；current run 若为 CANCELLED ⇒ 三 kind 自然 false。终态 task 不能再 REPLAN（TERMINAL_TASK 双保险）。
- 恢复/重启：规则每次从 DB 现算、无内存态；备份复制文件保留行序（A20 路径），重启后可重复判定一致。
- 旧 fence：briefing 已按 receipt.AttemptNo/FencingToken==run 当前值过滤（runtime_criteria.go:79-87），旧 fence receipt 被排除且 len==0→false；source 事件 run_id 同但 attempt/fence 不符走 continue（143-145），不得 PASS；alarm 为 run 级证据（session 与 run 1:1，DDL UNIQUE，且派发幂等 PERMIT_ALREADY_CONSUMED）——语义是"本 run 已记录会话"，重新领取不重复创建、仍是该 run 的合法证据；文档必须写明 alarm 证据是 run 级而非 fencing 级，不要再加 fence 条件。
- 多次 REPLAN：只认最新行；被取消的中间 run 证据不满足（反例 3）。
- 重规划命令与冻结 criteria 的一致性：新 run 的命令参数仍须与 expected 交叉一致（如 device_id/audio_ref/source_id，40/63/113 行）；不一致 ⇒ UNKNOWN（正确暴露规划错误），这正是"不得放宽到任意旧 run"的边界，不得改判 PASS。
- RESULT_UNKNOWN 当前 run：维持不可完成（REPLAN 也被 93-99 挡），先对账，符合协议 §4/§6。

════════ 五、Schema/DDL 结论 ════════
Option A（推荐，本裁决）：零 Schema、零 DDL、零 Task DTO 变更；只改 verifyRuntimeCriterion 的 run 解析块 + 测试 + 文档。rowid 依赖有既有先例（REPLAN previous、事件游标），无删除无 VACUUM 使其成立；风险与事件游标机制同级，可接受。
Option B（task.current_run_id/代际）仅在以下情形才值得：将来引入 job_run 行删除/整理、或需要在多处共享"当前代际"注解。若采用，必须：新迁移列+Task DTO（两份 contracts 同步）+createRunTx/REPLAN 在插入同事务写 current_run_id+CAS（`UPDATE task SET current_run_id=新 WHERE id=? 且旧值匹配`，REPLAN 已持写锁可不强加 WHERE）+verifier 同事务读 task.current_run_id；并需回填（回填值=最大 rowid 行，即 Option A 规则一次计算）。任一环节缺位，Option B 不优于 A。当前不采纳。

════════ 六、必须修改路径与反例要求 ════════
必改：
1. src/store/runtime_criteria.go:12-36 —— 以 R1 查询替换 count==1 逻辑，加 R2 防御条；三 kind 分支（38-153 行）不动。
2. 文档：docs/ExecutionProtocol.md 的 D08 小节加"修订 1：当前 run 定义（REPLAN 兼容）"，明确纠正"恰 1 run"表述并说明不弱化；docs/DataStructure/Verification.md、TypeRegistry.md 同步 D08 修订；README 索引与 acceptance-matrix 相应更新。本裁决文件 review/D08-replan-current-run-deepseek.response.md 由 Codex 保存。
3. 测试（反例要求，必须逐条有程序断言）：
   a) 合法 REPLAN 后、旧 run 证据齐备而新 run 未执行 → 三 kind 全 false，任务不得 SUCCEEDED；
   b) REPLAN 后新 run 执行并产出新证据 → PASS，且证据必须来自新 run（删除/篡改旧 run 证据不影响 PASS；删除新 run 证据 → 不 PASS）；
   c) 连续两次 REPLAN → 只认最新 run，中间被取消 run 的证据不满足；
   d) 旧 fence：re-claim 后 fencing 递增场景下 briefing 旧 receipt 不满足；alarm 语义按 run 级（见第四节）；
   e) source：事件 extensions.run_id 指旧 run → 不满足；指当前 run 但 attempt/fence 不符 → 不 PASS；
   f) 并发两序：verify 先（后 REPLAN 遇 TERMINAL_TASK）与 REPLAN 先（旧证据不满足）各断言一次；
   g) 恢复：重启后重复上述关键断言（无内存态）。
4. 不动：三 kind 的 expected 定义、DeriveCriteria、schema 两份副本、DDL、matchCriteria、冻结/篡改检查。

════════ 参考资料 ════════
- src/store/runtime_criteria.go:12-36（恰一 run 缺陷）、38-153（三 kind 证据）
- src/store/runtime_controls.go:82-153（REPLAN：守卫 93-99、previous 101、取消 112-133、新 run 134-152、INSERT 149）
- src/store/runtime_wait.go:148-164（ReplanTx→PENDING）；src/store/runtime_local.go:176-274（VerifyTaskTx 与新派生）、120-138（MutedAlarm 会话写入）
- src/store/runtime.go:226-283（createRunTx 唯一插入点）；runtime_schedule.go:243/315、runtime_memory.go:72（其余生成点）
- src/store/001_baseline.sql:132-139（job_run，含 attempt_no/fencing 列）、175（change_event seq）、190（alarm_session UNIQUE(run_id)）、118-125（task，无 current_run_id）
- 先例：runtime_controls.go:101、runtime_criteria.go:66、runtime_control.go:338（rowid 序）；docs/ExecutionProtocol.md §4/§6（RESULT_UNKNOWN 与对账）

════════ 思路 ════════
把"当前 run"从结构计数（恰一行）改为有序择一（最新已接纳行），用同库既有的 rowid 插入序做唯一性证明（REPLAN 自己就是同一证明的使用者），再以 R2 把任何顺序异常降级为 UNKNOWN 而非 PASS；所有 kind 的严格性不靠"只存在一个 run"，而靠"一切证据键控当前 run.id + 旧键不可满足"。这样 REPLAN 兼容、旧证据不可借、无 DDL、无模型参与，且与协议 §4/§6 的 RESULT_UNKNOWN/对账语义不冲突。下一步交 Codex 实施第六节清单并补反例测试；文档中把"恰 1 run"字样作为已纠正缺陷记录在 D08 修订 1。 ( _ _ )

—— Ayanami Rei / reviewer_model: deepseek-v4.1-flash (opencode-go)
