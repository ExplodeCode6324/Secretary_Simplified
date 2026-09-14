# Issue #1 runtime and source-reader work

Scope: AUD-02 and AUD-04 from GitHub issue #1. All tests use independent temporary stores, synthetic canaries and fake/local model endpoints. The existing final2 soak is untouched; Master explicitly requires no new two-hour run for these fixes. Its eventual result remains tied to its original binary.

## AUD-04 — read-time source size bound

`core/source_file.go` reads through `io.LimitReader(..., 1 MiB + 1)` and rejects the extra byte. A nonblocking open followed by descriptor `Stat` rejects FIFOs/directories without waiting for a writer. The actual read remains bounded if an already opened/stat'ed file grows. `core/work.go` only invokes the ingest adapter after this bounded read succeeds.

Executed:

- `go test -race ./core -run TestIssue1Source -count=1`: PASS. Covers 0/small/exact 1 MiB/+1/64 MiB counting readers, growth after open/stat, ordinary small file, FIFO and directory rejection.
- `go test ./tests -run TestIssue1OversizedSource -count=1`: PASS. Actual Core↔Runner authenticated Unix HTTP path reads an oversized synthetic file, persists a failed/no-business-effect CoreWork, and preserves the pre-existing source state payload, record count and source.synced event count without advancing a success cursor.

No real file source is enabled by this change; the fixture adapter remains a synthetic-only boundary.

## AUD-02 — actual pre-fix reproduction

`TestIssue1CancelledSocketWorkSettlesAfterRunnerUnknown` uses a real Unix HTTP connection and a fake model held until cancellation. Runner persists RESULT_UNKNOWN first, then the handler returns from the cancelled model. On the pre-fix path the test failed after five seconds: CoreWork remained RUNNING with no receipt. This is an executed reproduction, distinct from the original issue's static finding.

The approved repair uses a five-second detached settlement context, exact durable run/task/attempt/fence/command identity, and a registered cancellation audit extension. The model and source adapter never continue on the detached context. Same-generation UNKNOWN may receive a final receipt; a known no-effect failure uses the existing retry budget and memory 5/30-minute backoff without double active_ms settlement. Old-generation FAILED receipts cannot rebind. Same-generation duplicate POST is rejected while work is running.

Briefing publication binds object ref and CoreWork receipt in one WriteObjects transaction. Memory recovery validates the exact persisted slot and controller root, records a program reconciliation receipt in the same transaction as its CoreWork update, then uses ordinary receipt verification. GET stays read-only. Source failures after entering the adapter remain unknown rather than inventing no-effect certainty.

Executed targeted race tests (actual authenticated Unix HTTP, local fake endpoints; no real model API):

- `TestIssue1CancelledSocketWorkSettlesAfterRunnerUnknown`: request cancellation precedes final CoreWork; same-generation FAILED/no-effect restores QUEUED with the existing memory backoff; no second model call.
- `TestIssue1QueuedBackgroundCancellationNeverCallsModelHTTP`: a foreground call occupies the actual provider, a queued background request expires, only one fake HTTP request exists, and its durable failed receipt enables bounded retry.
- `TestIssue1OversizedSourceSocketDoesNotAdvanceCursor`: actual source work returns INPUT_TOO_LARGE, with previous cursor/records/success events unchanged.
- `TestIssue1CommittedMemoryCrashGapReconcilesAfterReopen`: a SQLite failure trigger rejects final CoreWork updates after the snapshot commit; a newly opened Store/socket recovers the exact slot without a second model call. Future-slot and wrong-fence evidence are rejected.
- `TestIssue1LostBriefingResponseUsesAtomicArtifactWithoutModelReplay`: a real HTTP connection closes after the handler commits; a newly opened Store/socket resolves UNKNOWN from the atomic object/receipt without calling the model again.
- `TestIssue1DuplicateSocketPostAndLateCancelledReceipt`: duplicate POST does not reenter the model, a late cancellation receipt preserves CANCELLED, and old FAILED cannot be rebound to a new attempt/fence.
- `TestStaleWorkerCannotCommitReceipt`: an old worker cannot commit when the durable execution fence changed.

Design review approvals are in `review/issue1-AUD02-AUD04-design-correction.response.md` and `review/issue1-memory-reconcile-clarification.response.md`. Implementation has been sent separately for genuine Ayanami review; the design approval alone is not an implementation-pass claim. These are controlled fault injections with reopened database connections, not a claim of OS power-loss testing or new sustained-duration acceptance.

Final targeted command: `go test -race ./core ./store ./executor ./tests -run 'TestIssue1|TestStaleWorkerCannotCommitReceipt|TestRuntimeRemoteQuery' -count=1 -timeout=90s` — PASS (tests 6.017s, core 1.308s). `issue1-runtime-checks.json` records the exact source hashes. The unknown-to-failed reconciliation test additionally asserts unchanged active_ms after the first UNKNOWN settlement. Documentation validator reports DOC_ONLY PASS. Full-repository race/vet and final release smoke are coordinated by the root agent separately.
