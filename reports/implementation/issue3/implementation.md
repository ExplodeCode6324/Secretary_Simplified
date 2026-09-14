# Issue #3: two Issue #2 client residuals

Baseline: `c84d92453a1d8338be728422e3164777ef822579`. This report covers only R1-01—R1-03, R2-01—R2-03 and the client part of BUILD-R. It does not reopen #1/#2 or rerun their original suites, old context/persistence baselines, model evaluations or A25. DOC-R, current exports, release CLI hash and publication are coordinated in the current issue index; the daemon is unchanged.

## Actual pre-fix reproduction

`before.json` confirms the production client/model files matched the named Git baseline byte-for-byte when the new tests ran. `before.log` is the actual failed race run, not a predicted outcome:

- POST 202 followed by GET 400/401/403/404 cleared the recovery file and pending entry, lost the known turn identity and restored the original draft with an input-rejected notice.
- The timeout/503 branch also lost known acceptance/turn identity.
- All four business panels loaded 100 rows, selected row 75, then a tick reduced them to 50 and selected row 49. Delayed-response and visible-version checks failed as well.
- The submission-rejection versus lost-POST control passed.

The first run contained six parent tests: five failed and the rejection/lost-POST control passed. A separate additional pre-fix test for the real items cursor-invalidation contract also failed; its command exit code and log are preserved in `before-items-cursor.json` / `.log`. The fixture rejection code was subsequently refined from a generic fake code to the actual precommit `INVALID_SCHEMA` code per Ayanami's contract review; this refinement does not rewrite the original test-source hash or logs.

## RES-01 implementation

`model.go` now separates submission-stage errors from observation errors. A validated 202 receipt carries the original request ID, known turn ID and acceptance fact into the UI even if the following GET fails. A legal request-receipt lookup also establishes acceptance before fetching the turn, so a rebuilt client retains the known identity while turn reads remain forbidden. Query errors never erase that fact or restore a resend draft. Authentication failures explicitly request restoration of authentication.

A monotonic completion set prevents late submission/query/synchronization messages from resurrecting pending state or regressing a completed turn. Terminal history remains sequence/ID deduplicated. Only contract-proven precommit submission codes permit cleanup and optional draft restoration; another draft is never overwritten. Idempotency conflict, model/context budget codes, unclassified errors, 5xx, transport loss and unexpected success shapes remain unknown and are resolved through the original request without automatically POSTing again. `client.go` preserves the server's explicit error code on 404.

No API, DTO, database or recovery-file format changed. Recovery still stores only instance ID and request ID, never private text; the known turn can be recovered through the existing request lookup.

## RES-02 implementation

`panels.go` separates loaded page caches, current object selection and asynchronous request generations. A normal automatic refresh reads only the selected object's page (at most 50 rows), merges loaded pages by stable ID and restores selection by that ID. The tail cursor remains tied to the last loaded page. Stale or duplicate responses from an earlier request/panel visit cannot replace or append into the current view. Objects outside the selected page remain cached until their page is visited/refreshed; this is not a claim of scanning every loaded object on each tick.

Items use a different existing backend cursor family: its global change watermark can invalidate a cursor with 409. On that response, the UI retains its loaded view, labels the snapshot invalid and performs at most one additional GET for the selected item. Later ticks use only that item GET (or no item GET when there is no selection), never retry the dead page cursor. `n` is disabled; `r` explicitly reloads the first page with a fresh snapshot. It rebinds the original selection only if that ID is present; otherwise selection becomes -1 with a clear prompt. A 404 on the selected item also clears selection instead of silently choosing another object.

An open confirmation remains bound to its original target ID and revision. The client issues no mutation POST or implicit ack during refresh. This statement does not erase existing backend behavior: notification GET may mark delivery as DELIVERED; that backend behavior is unchanged and is not an acknowledgment.

## Final directed evidence

Commands executed, and only these test selectors:

```sh
cd src
go test -race ./cli/tui -run '^TestIssue2Residual' -count=1 -timeout=30s
go vet ./cli/tui
```

Initial result: PASS, race test package 2.588 s, vet exit 0. `after.log`, `vet.log` and `after.json` record the actual commands/results and exact production/test source hashes. Nine new parent tests cover 29 table/equivalent cases:

| Requirement | New test/evidence |
|---|---|
| R1-01 | `R101AcceptedQuery4xx`: four observed query codes, POST→GET order, exact request/turn identity, file and draft assertions, preserved explicit 404 code |
| R1-02 | `R102UnknownObservationRebuildAndLateResult`: timeout/503, rebuilt client still denied, original request lookup, eventual terminal cleanup, late-message nonregression, one POST/one displayed event |
| R1-03 | `R103SubmissionRejectionVersusLostResponse` and `R103AmbiguousSubmissionCodesAndUnexpectedSuccess`: actual precommit rejection, new-draft protection, lost response, ambiguous 4xx, unexpected/malformed success shapes |
| R2-01 | `R201PanelRefreshKeepsSecondPage`: tasks/jobs/items/notifications, 100 rows, selected stable ID, tail cursor, no duplicates or mutation requests |
| R2-02 | `R202PanelResponsesIgnoreOldScope`: delayed refresh, newer page, duplicate response and reopened panel visit; all four panel families |
| R2-03 | `R203RefreshVersionConfirmationAndRemovedTarget`: all four panel families; version update, unchanged confirmation, disappearance and explicit no-selection |
| R2-03 items contract | `R203ItemsInvalidatedCursorKeepsExplicitNavigation` and `R203ItemsStaleTicksAndMissingTarget`: invalidation notice, selected-object GET, n disabled/r explicit reset, at most one later item GET, refreshed version and 404 handling |

All test data/callers are isolated and synthetic. This is a client-state-machine/API-call-path test, not a renewed backend storage qualification or a new PTY/platform qualification. Existing Issue #2 reports, tests and raw evidence remain unchanged.

Design approvals: `review/issue3-design.response.md` (initial proposal, refined by correction) and `review/issue3-design-correction.response.md` (final local contract). Actual implementation review is recorded separately by Ayanami; design approval and our own PASS are not mislabeled as her runtime review.


## Ayanami review delta

The two additional boundary tests first failed on the initial implementation: all four panel kinds continued a cursor after receiving zero new IDs, and both reset cases consumed the original target intent on a failed first-page request. The genuine failure output is preserved in `before-review-delta.log`. Original `after.*` and `vet.log` remain unchanged.

The client now stops pagination with an explicit no-new-ID notice and a nil cursor after a zero-progress append. Refresh ticks cannot re-enable that cursor; reopening the panel resets navigation. Deduplication keeps each ID at its first position, while later loaded-page payloads replace earlier payloads for that ID. This is deterministic later-page precedence, not first-payload precedence or a server-wide latest-revision claim.

A reset's selected-ID intent is now consumed only after a successful page application. A failed reset followed by a successful retry either reselects that exact ID when present, or leaves selection at -1 with a notice when absent. It cannot silently select row zero.

Final directed result: 11 parent tests / 35 cases PASS, `go test -race ./cli/tui -run '^TestIssue2Residual' -count=1 -timeout=30s`, package time 1.669 s. `go vet ./cli/tui` PASS. Evidence and final source hashes: `after-final.log`, `vet-final.log`, `after-final.json`. No old tests or backend suites were run.


### Repeated reset fallback

A follow-up review found that calling reset again after a failed reset could overwrite the retained target with an empty current selection. `resetPanel` now falls back to its pending selection intent when there is no selected row. A new two-case test directly repeats the reset helper and checks both a present and absent original ID. Both failed before the fallback (`before-retry.log`) and pass afterward. This is a defensive repeated-reset boundary; the ordinary r key on a non-invalid empty panel already requests the same page without resetting. Final residual selector: 12 parent tests / 37 cases PASS, with vet PASS; `after-retry.json`, `after-retry.log`, and `vet-retry.log` bind this final snapshot. Prior after and after-final files remain unchanged.
