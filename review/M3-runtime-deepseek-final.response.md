约束检查完成。以下是最终复核裁决（本轮我身处 deepseek-v4.1-flash / opencode-go，无 429、无委派、未跑全仓 test/race/vet、未触 resources/ELIZA/受测模型/响铃/系统；业务目录只读；独立测试全部在隔离副本执行）。review/M3-runtime-deepseek-final.response.md 现为 0 字节占位，本回复即交 Codex 保存的内容，我不另写同名文件。

M3 Runtime 最终复核 — Ayanami（deepseek-v4.1-flash / opencode-go）

快照绑定：对 reports/implementation/D08-runtime-checks.json 全部 13 个文件做 shasum -a 256 -c → 13/13 OK（含 runtime_criteria.go、runtime_d08_replan_test.go、executor.go 等）。证据 JSON 与本次冻结工作树一致。隔离副本：/tmp/m3_deepseek_final_20260913_235347（rsync，只读复制）。

实际命令与结果（cd /tmp/…/src）
1) go test ./store -run 'TestAyanamiResidualCriterionHashMutationMustReject|TestAyanamiFinalProbe' -count=1 -v
   → PASS 3/3：TestAyanamiFinalProbeTaskIntegrity、TestAyanamiFinalProbeRunFenceCAS、TestAyanamiResidualCriterionHashMutationMustReject；ok secretarysimplified/store 1.576s
2) go test ./tests -run 'TestD08ReplanCurrentRunEventuallyVerifies|TestD08CurrentRunLineageAcrossReopen|TestD08PublicAdmissionExecutionAndNegativeEvidence|TestD08PublicRejectsChangedCriterionAndStrictExtension|TestD08BriefingRejectsMissingEmptyAndStaleReceipts|TestD08BriefingModelActionsNeverVerify|TestRemoteFailurePreservesEffectAndRetryCeiling|TestRuntimeDSTGapDuringDowntimeAuditedOnce|TestAyanamiFinalCurrentUnknownRunDenied|TestRuntimeJobPatchAtomicIdempotency|TestStaleWorkerCannotCommitReceipt|TestRuntimeArtifactTraversal' -count=1 -v
   → PASS 12/12（lineage 三能力 3/3、BriefingStale 5 子项、RemoteEffect false/true）；ok secretarysimplified/tests 0.939s
3) go test ./executor -run 'TestAyanamiFinalArtifactRootContainment' -count=1 -v
   → FAIL，原始行：ayanami_final_artifact_test.go:93: no write succeeded during sub swap race。此为**我方探针自设断言过严**（要求竞态中至少一次成功写），非实现缺陷判定。此前断言均通过：正常写入读回一致、静态 symlink 组件逃逸被拒且 outside 无物、symlink 根被拒、sub-swap 竞态 250 次后 outside 仍为空（无逃逸）。探针 v2（加控制写 + 根级 swap 相位）已写入隔离副本但未执行（预算终止），不计为证据。

一、重点1 旧P1（rtSaveTask 篡改/零行 + rtSaveRun CAS）
已固定：
- src/store/runtime.go:340-342 字段篡改（含仅改 criterion_hash）→ IMMUTABLE_CRITERION；:348 UPDATE 条件含 id+criterion_hash；:352-358 RowsAffected≠1 → REVISION_CONFLICT；:359 事件仅在成功后追加。rtSaveRun：:311-313 Validate、:318 fence/attempt CAS（WHERE=本事务读到的 before 值）、:322-328 → STALE_FENCE、:333 事件在后。
- 探针 src/store/runtime_ayanami_residual_test.go:13-44 PASS；我的独立反例（隔离副本新增）三路：内容篡改→IMMUTABLE_CRITERION、hash 字段篡改→IMMUTABLE_CRITERION、直改列致零行→REVISION_CONFLICT；均另断言 task.updated 计数不变、payload 仍 PENDING（无伪事件、无部分写）；RunFenceCAS 直改 fencing_token 列 → STALE_FENCE、run.updated 计数不变、run 未变。全部 PASS。
- 语义核验：全部 9 处 rtSaveRun 调用点（runtime_execution.go:118/146/255/273/359/462、runtime_local.go:349、runtime_controls.go:130、runtime_schedule.go:434）均为同事务读-改-写，CAS 的 before 恒等于调用方所读；跨事务 stale 写不可达。无弱化。
需修：无。  未覆盖：无。

二、重点2 os.Root artifact 根
已固定（部分否证成立）：
- src/executor/artifact.go:14-71：预检→Lstat 根拒 symlink(:25-27)→os.OpenRoot dirfd(:28)→SameFile 防换根(:33-39)→MkdirAll/O_EXCL 临时文件/fsync/Rename 全经 confined 句柄(:42-70)；对象库同为 dirfd（src/store/objects.go:37、:94-105 含哈希自洽）。
- 独立测试证据：正常写入 PASS；root/link→outside 静态 symlink 拒且无逃逸物 PASS；root 自身 symlink 拒 PASS；sub 组件 dir↔symlink 交替 250 次竞态，outside 恒空——旧 Lstat→Rename 窗口未能产生逃逸。
需修：无（现有证据范围）。未覆盖/提示：
- 根级 swap 竞态相位与“成功写与攻击并发”混合样本未完成（探针 v2 未执行）→ 全量否证待补，**不得泛称文件边界安全**。
- src/executor 无任何 *_test.go：实现方对 os.Root 写路径无自测；本轮探针为唯一动态证据。
- 残余观察：验证器读侧仍为 SafeArtifactPath+os.ReadFile（runtime_criteria.go:111-116、runtime_local.go:208-216）——属检查后按路径读；因结果须与记录字节哈希自洽，不能借此伪造 PASS，但建议下轮 dirfd 对齐。

三、重点3 D08 三 kind 与 REPLAN R1/R2
已固定：
- src/store/runtime_criteria.go:13（ORDER BY rowid DESC 全行）、:27（逐行 ID/task/attempt/fence/state 与 DTO 交叉）、:34-38（更旧行非终态→false，R2）、:44（count<1 或当前 RESULT_UNKNOWN→false）、三 kind 严格当前证据（alarm :48-71、briefing :72-120 含 :89 attempt/fence 过滤、source :121-163 含 :150-159 provenance/cursor 自洽）。
- src/tests/runtime_d08_replan_test.go：双 REPLAN :100-107、reopen :108-117、REPLAN 先 :99-107、verify 先（TERMINAL_TASK）:125-128、旧 run 未决 :144-156、旧证据删除无关 :160-163、source 旧 fence :164-176、当前证据删除拒 :177-180；无 skip/env gate（acceaptance-matrix.md:75 自述 gate 已去），普通 go test 直接 PASS。
- 独立反例 TestAyanamiFinalCurrentUnknownRunDenied：同夹具先 PASS，再将当前 run 置 RESULT_UNKNOWN（列+载荷）→ VerifyTask false。PASS。
- 文档 4 处 R1 核对一致：ExecutionProtocol.md:115-119、Verification.md:44-47、TypeRegistry.md:81-84、Acceptance.md:97-100；README.md:66/164 索引。"恰 1 run" 已明示为已纠正缺陷。
需修：无。  未覆盖：更旧行为 RESULT_UNKNOWN 的专项反例未单跑（RUNNING 已经由 lineage 覆盖，同 switch 分支；风险低）。

四、重点4 Remote retry/Query
已固定：
- 重试：runtime_execution.go:323 仅 FAILED+无 effect+非 CANCELLED；:328 maxAttempts=3、:336 未达上限才 requeue、:421-423；有 effect 恒 1 次。executor.go:96/114/133-134：remote 不回填 EffectObserved；receipt :296-306 幂等落库保原字段。
- Query 不重发：remote_query.go:11-58 仅 GET /internal/v1/work/<run>，身份/命令 hash/内部 receipt 一致性后生成对账 receipt，:52-57 仅覆盖 ID/attempt/fence/key/时间，Status/Artifacts/EffectObserved 保原；reconcile :59-91；work_repo.go:36-49 已有 receipt 直接复用。
- 实测 TestRemoteFailurePreservesEffectAndRetryCeiling false/true 均 PASS（calls 3 vs 1、attempt 3 vs 1、无覆盖改动）。
需修：无。  未覆盖：真实 RemoteCore HTTP 端到端 reconcile 未验（离线 bridge 之外；非本轮范围）。

五、重点5 D06 gap + UpdateJobRequest + 回归
已固定：
- runtime_schedule.go:338（diff hunk @@ -335,7 +335,7@@：gapDates=calendarGapDates(sm, due, *next)，由 latest 改 due）；TestRuntimeDSTGapDuringDowntimeAuditedOnce PASS（local_date 03-08、origin=scheduler.calendar、before≠after、二次扫描 count=1）。
- runtime_job_patch.go:16-22 六字段白名单；:24-35 原回执幂等重放 response_json；:53-62 next_due 重算；runtime_schedule.go:419-447 取消旧 QUEUED/CLAIMED 队列；:449-456 UPDATE 严格 RowsAffected→REVISION_CONFLICT。
- 定向回归实测 PASS：TestRuntimeJobPatchAtomicIdempotency、TestStaleWorkerCannotCommitReceipt、TestRuntimeArtifactTraversal。
需修：无。  未覆盖：cancel/expiry/action-hash 全量回归矩阵未重跑（遵本轮限定名约束）；这些仅由定向名+路径未动支持，**不构成本轮全量回归证明**。

裁决：P1 字段/零行、os.Root 写路径（部分）、D08-R1/R2、Remote retry/Query、D06+patch **已按证据固定，无新增需修项**；“artifact 根级 swap 全量否证”未完成（探针 v2 未执行，唯一 FAIL 系我方断言）——维持**有条件通过**，该条计入未覆盖。不声称完整 A11–A25 或真实设备/真实模型验收。( _ _ )

—— Ayanami Rei / reviewer_model: deepseek-v4.1-flash (opencode-go)
