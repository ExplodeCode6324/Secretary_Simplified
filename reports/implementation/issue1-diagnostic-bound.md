# Issue #1: bounded diagnostic publication after cancellation

During AUD-01/AUD-02 integration review, `RecordingModel.Generate` still archived returned synthetic output through `PutObject(context.WithoutCancel(ctx), ...)` with no deadline. The new cancellable publication lock could therefore be waited on indefinitely before Core reached its bounded receipt persistence. This was a code-path finding, distinct from the separately reproduced CoreWork defect.

The fix wraps only this diagnostic object persistence in a five-second timeout around WithoutCancel. Model execution stays on the original context. Existing `return result, e` propagation is preserved; errors are not silently ignored and a model result is not treated as a committed business effect.

Actual targeted command from `src`:

```sh
go test -race ./diagnostics -run 'TestIssue1CancelledOutputArchiveHasBoundedLockWait|TestRecordingModelPersistsActualFixtureEvidence' -count=1
```

PASS (2026-09-14, 7.169 seconds). A fake model returns output as its caller is cancelled and an independently opened flock descriptor holds the publication lock. The archive returns DeadlineExceeded within the test's eight-second guard; normal request/response evidence still archives correctly. This establishes cancellable lock waiting, not interruption of arbitrary filesystem sync calls or a whole-handler wall-clock guarantee.

Ayanami's agreement and precise scope correction: [initial bounded-context approval](../../review/issue1-diagnostic-bound.response.md), [error propagation correction](../../review/issue1-diagnostic-bound-correction.response.md). [Independent implementation verification](../../review/issue1-diagnostic-bound-final.response.md) reran both race tests (PASS, 7.834 seconds), recorded both source hashes and found no must-fix in this incremental scope. It does not certify the rest of AUD-01/AUD-02.
