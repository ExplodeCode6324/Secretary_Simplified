# A23 coverage ledger v2

This is a retrospective index update. It does not claim a combined manifest existed before any model call. The reviewed v1 is preserved unchanged.

Current whole sets: **month-run8** (90 Item points and 30 snapshot reference checks), **scenarios-run4** (7 cases), **world-read-run2** (2 cases). No passing checkpoint is selected from another month run.

Integrity: 129 cases; 129 current PASS; 129 pre-call component memberships; 4018 asset references hash-verified.

## Full month history

| Run | Item points recorded | Item PASS | Reported failures | Persisted snapshots |
|---|---:|---:|---:|---:|
| month-run1 | 18 | 18 | 6 | not exported |
| month-run2 | 56 | 53 | 6 | not exported |
| month-run3 | 30 | 26 | 6 | not exported |
| month-run4 | 90 | 90 | 0 | 30 |
| month-run5 | 90 | 90 | 0 | 30 |
| month-run6 | 90 | 90 | 0 | 30 |
| month-run7-failed | 9 | 3 | 6 | 3 |
| month-run8 | 90 | 90 | 0 | 30 |

Month-run7-failed retains all six failures and all three persisted snapshots. Persistence alone is not claimed as an independent reference/prose audit for that failed run. Earlier local-only scenario/world histories remain explicitly local-only in the JSON.

## Separate A09/D11 history

| Run | Status | Scope |
|---|---|---|
| question-run1-failed | FAIL | Expecting value: line 1 column 1 (char 0) |
| question-run2-failed | FAIL | retrieval returned non-JSON product reply: Request was recorded, but no actions were committed: BUDGET_EXHAUSTED |
| question-run3-failed | FAIL | retrieval returned non-JSON product reply: Request was recorded, but no actions were committed: CONTEXT_REQUIRED_OVERFLOW |
| question-run4 | PASS | Full lifecycle including restart, explicit answer and idempotency/409 |

These four question runs are additional A09/D11 evidence, excluded from the 129 A23 cases.

## Evidence limits

- Month manifest binds all 180 authored input/oracle files; file hash checks verify current equality, not trusted timestamp attestation.
- Supplement global manifest explicitly identifies itself as current inventory, not retroactive freeze proof. Per-run fixture/oracle files and tool save-before-call ordering supply the archived pre-call membership evidence.
- This unified index is created after executions under the reviewer-authorized composition interpretation. It does not claim that a cryptographically timestamped combined-set manifest existed before the first call.
- Intermediate local-only runs are indexed by project-relative archive paths; no private absolute paths, runtime configs, tokens or grants are copied.

Current snapshot assertions check immutable Item revisions and exact EntityReadRef. They do not supply an invented free-text oracle. Historical prose samples apply only to their named original runs.

Machine-readable ledger: [A23-coverage-ledger-v2.json](A23-coverage-ledger-v2.json), SHA-256 `647dc592f3a1e444479aece2eda7c7900c82641ed617a97603e371f0f587d652`.

Generator: `python3 scripts/build_a23_ledger.py --root REPOSITORY_ROOT`. This indexes existing evidence only; it runs no model and changes no oracle.
