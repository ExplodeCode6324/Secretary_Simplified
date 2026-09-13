# M3 implementation evidence (synthetic fixtures only)

Implementation: `src/store/runtime*.go`, `src/executor`, `src/scheduler`, `src/policy`, `src/cmd/secretaryd`.

Executed command: `cd src && go test ./tests -run Runtime -v`.

Passing checks:
- Durable notification creation, distinct CLI delivery/acknowledgement, deterministic verification.
- Expired PREPARED claim recovery increments fencing; old dispatch denied.
- Expired DISPATCHED execution becomes RESULT_UNKNOWN and is not automatically retried.
- Cancellation prevents queued dispatch.
- Persisted model-call and replan budgets cannot reset within a root.
- America/New_York DST spring gap skip and fall first-fold-only occurrence.
- Artifact traversal and existing symlink components are rejected.
- WAIT timeout wakes persistently and REPLAN does not change criterion hash.
- Once schedule restart cannot materialize the same occurrence twice.
- Scheduled-job registration emits a valid typed atomic ChangeEvent; unknown event rejected.
- Local unknown notification effect reconciles from persisted authoritative evidence without redispatch or duplicate notification.

`go test ./executor ./scheduler ./policy` passes package compilation. These commands are implementation checks, not completed formal acceptance or real-data validation. Ayanami review is tracked independently in `M3-runtime-ayanami.md`.

D01 scheduled-job event registry gap was approved by Ayanami (`D01-scheduled-job-event.response.md`) and implemented. D02 recurring notification-key binding was approved by Ayanami and implemented with versioned base64url occurrence identities; see `D02-notification-occurrence.response.md`.

## Virtual long-horizon execution

`go test ./tests -run RuntimeVirtual -v` passed on 2026-09-14. ManualClock drove real SQLite scheduling, Task/JobRun writes, local atomic artifact writes, and deterministic artifact-hash verification:

- 72 consecutive hourly occurrences: 72 successful Tasks and 72 distinct execution roots; no real-time 72-hour uptime is claimed.
- 30 consecutive daily occurrences: 30 successful Tasks and 30 distinct execution roots. On days 10, 20, and 25 the command content and template criterion changed through explicit job revisions; four immutable JobRun revision snapshots remained represented.

This uses synthetic artifact workloads, not real data, not live model semantics, and not recurring-notification acceptance. D02 was subsequently approved after review connectivity recovered; see below.

## D02 recurring notification binding

`TestRuntimeRecurringNotificationBinding` passes: command and criterion receive the same `occ:v1` key before immutable hash calculation; stored job template stays unchanged; duplicate occurrence scans create nothing; two occurrences create two notifications; REPLAN preserves the hash; mismatched template command/criterion keys are rejected at registration. Attempts and job revisions are absent from the encoding.

Runner dispatch now reserves 30 seconds of durable active-time budget before work and uses a 30-second execution context. The first persisted final receipt settles unused time; interrupted work retains its reservation and later reconciliation cannot refund twice. Local notification/alarm mutation transactions recheck grant revision, fencing and cancellation generation.

## Ayanami findings and repair evidence

The first independent M3 review reported three failing adversarial probes. Their original code is retained as `src/tests/runtime_ayanami_probe_test.go`; all five probes now pass. The residual immutable-hash probe is retained as `src/store/runtime_ayanami_residual_test.go` and also passes.

Repairs include durable permit expiry/capability/attempt/fence/cancel rechecks and single-use local transactions; exact durable command hashes; cancelled Tasks remain CANCELLED when effect receipts prove a race; active executor contexts receive cancellation; alarm expiry enters an explicit recoverable UNKNOWN state and then reconciles persistent session evidence; DST gaps have atomic `scheduled_job.skipped` audit events; event rules ignore self-generated and calendar-audit events and persist consumer watermarks/effect hashes. Task updates reject both changed criteria content and a changed criterion_hash field, and Task/Run SQL updates require exactly one affected row.

Artifact writes use Go 1.25 `os.Root` descriptor-backed operations with configured-root inode checking, confined creation/rename and directory fsync, avoiding a new direct third-party dependency.

`src/tests/runtime_process_integration.py` starts only its own isolated Core and Runner processes. Its successful report is `M3-runtime-process-integration.json`: two typed once reminders executed, the second executed after this script stopped its own Core process, Runner control remained online, client/internal credentials were not interchangeable, and the business Item remained OPEN. The first process run exposed a missing initial next_due_at calculation; registration was corrected and the process test was rerun successfully. This is a fixture plumbing result, not real-model or real-user acceptance.

Additional mechanical tests cover 5-minute/30-minute memory retry spacing with a maximum of three attempts, true dispatched-context cancellation, canonical UTF-8 `<>&中文` criterion/command hashes, REPLAN producing a new method/run without changing the criterion, and CAS/idempotent controls with bounded query-bound pagination.
