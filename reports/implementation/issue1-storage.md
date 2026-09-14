# Issue #1 — AUD-01 / AUD-03 storage repair

Scope: implementation and synthetic directed regression for [GitHub issue #1](https://github.com/ExplodeCode6324/Secretary_Simplified/issues/1). No real model/network side effect, no credential payload, no modification of the ongoing final2 soak directory or binary. No Git commit performed by this module agent.

## Design and implementation

The effective Ayanami design is `review/issue1-AUD01-AUD03-design-correction.response.md`, superseding the earlier SQLite-only/lock-outside assumptions while preserving its original report. No DDL change.

Every ObjectWriter publication and input admission now uses `Store.WriteObjects`: distinct no-follow lock fd, fixed `.publish.lock` inode, cancellable flock waiting, then SQLite immediate transaction. The same lock remains held after automatic cancellation rollback and through bounded outcome inspection. Normal final names appear only through hard-link no-replace of a fully written, fsynced and closed same-directory temporary file. File and directory durability precede the object_ref insert. Unsupported hardlink fails closed.

Committed object corruption is rejected and its bytes retained. A full unreferenced orphan is explicitly file/dir synced again before adoption. A damaged unreferenced final is moved to an `.orphan-<uuid>` evidence name and reconstructed. `RecoverObjectStaging` only removes recognized UUID temporary names under the publication lock, capped at 1–1000 removals per call and protecting all referenced paths; it does not delete final blobs, quarantine or unknown filenames. It is an available Go API, not a fabricated existing CLI command.

AcceptInput/AcceptTyped keep final queue checks inside the admission transaction, before ObjectWriter.Put. ObjectRef, InputTurn, receipt, conversation and typed callback writes share this transaction. Failed admission deletes only this attempt's newly published files after a bounded uncancelled query confirms no committed reference while still holding flock. Uncertain queries retain recoverable full orphans. Automatic text reference shape and inputSemanticHash stripping stay compatible.

AUD-02 integration uses the same exported WriteObjects/ObjectWriter API for its separately owned briefing artifact/FinishWorkTx transaction; that integration is reviewed and tested by the runtime agent, not claimed by these storage-only tests.

## Actual directed results

`cd src && go test -race ./store -run '^TestObject|^TestAdmission' -count=1` — PASS, store 5.442s.

- Four real child-process exits: temporary partial write, before atomic publication, after publication before metadata, and before metadata transaction commit. Reopen/retry yields exactly one valid ref, full bytes/hash and reclaimable known temporary files.
- Two independent processes synchronize at a barrier and concurrently publish the same payload. Final-name observer sees only complete bytes; both callers receive identical stable metadata without unique-key errors.
- Damaged orphan recovery preserves quarantine bytes; committed tamper rejects reads/writes without replacing evidence. A separate adoption checkpoint proves file/dir sync precedes metadata registration.
- Twelve distinct full-queue inputs and twelve typed callback failures add no formal refs/files. Two Stores competing for the last queue slot admit exactly one; successful original retries remain stable.
- Cancellation after publication rolls back formal refs/files; held flock wait respects a 40ms cancelled context and keeps the same lock inode. Simulated lost acknowledgement after an actual COMMIT preserves the committed file/ref and retries identically.
- Bounded staging recovery skips committed references even when deliberately given a temp-shaped path in this isolated fixture, skips unknown names, and respects the cleanup limit.

`cd src && go test -race ./store ./ingest ./diagnostics -count=1 && go vet ./store ./ingest ./diagnostics` — PASS (store 6.327s, ingest 1.946s, diagnostics 7.849s; vet success).

`docs/checks/validate_docs.py` — PASS, 2026-09-14T03:03:18Z, unchanged DDL and schema, 242 local links.

## Preserved intermediate findings / evidence limits

The first independent-Store test observed `openat .publish.lock: no such file or directory` using concurrent os.Root.OpenFile creation on this host. Lock creation was changed to distinct `os.OpenFile(... O_NOFOLLOW)` descriptors; repeated and barrier-controlled process tests then passed. The published-object paths still use os.Root containment.

A wider concurrent intermediate run passed Store/Ingest/Diagnostics but failed two runtime-owned tests while AUD-02 was being edited: TestIssue1QueuedBackgroundCancellationNeverCallsModelHTTP and TestStaleWorkerCannotCommitReceipt. These failures were delivered to the runtime/root agents; this report does not silently relabel that run PASS. Final whole-repository regression/short HTTP smoke and reviewer verdict are recorded separately by the coordinator.

No new two-hour test is required or started. The original soak report retains its original build hashes and duration; these directed tests are new repair evidence, not a claim that the old soak exercised the repaired binaries.

## Source snapshot

- `src/store/objects.go`: `05bc43a9997916d9058636d01be7b0a89765ad8e54f1dbd3d120ae377f79fb39`
- `src/store/core_repo.go`: `8633054d086201a119c3e282e03722f2ff8a82fb9bb0d0041be58b90d5a2ac96`
- `src/store/issue1_storage_test.go`: `02514c20572da1ae11c53c2c0ae4de80c56879350bc4a2322e47b848cf11b840`
- `docs/Storage.md`: `e6f52d1c8a8ee0d320559a21b7cdd27fd0050847c38153e4b8c41fe3fed2b375`
- `docs/DataFlow.md`: `61ee6dbc786630341b4fb0e952b0ab11af2fc1adb5eeffe5062886e3f4620776`
- `docs/DataStructure/ObjectRef.md`: `507be15354aa565b30c72d91e94282b5b93d94a757725a4b91426110f0abb68b`
- `docs/Operations.md`: `848047966c1c7c4b8ccbe567e61f7f3d35b9f3246173e82b38c948cae4c1191b`

## AUD-03.2 evidence supplement

Read-only checklist reconciliation found that the original Typed rejection test asserted refs/files only and its callback returned immediately. It was not evidence of a business-write rollback. The additional `TestAdmissionTypedBusinessWriteRollbackFullBaseline` first commits a pre-existing input/object and Item baseline, then performs an actual `PutItemTx` inside AcceptTyped, confirms both Item and change_event are visible in that transaction, and returns an injected error. It compares serialized content of every SQLite table (including request/turn/conversation/event/receipt/object_ref and SQLite sequences) and every formal blob name/byte against the complete baseline. Re-submitting the exact rejected request succeeds; its next successful replay invokes the business callback only once.

`cd src && go test -race ./store -run '^TestAdmissionTypedBusinessWriteRollbackFullBaseline$' -count=1` — PASS, 2.321s. Only the test and reports changed; this supplement does not claim a newly run full repository suite, real filesystem fault, or additional soak. The earlier immediate-error test and the checklist's discovered evidence gap are retained as historical context, now directly covered by this extra test.
