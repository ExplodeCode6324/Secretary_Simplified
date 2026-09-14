# Issue #2 AUTH backend implementation

This report covers only new AUTH-01—08 backend directed evidence. The selected test command does not run historical Context/Store/summary/consciousness tests, full-repository race, monthly replay, real model calls or A25. Original tests and evidence remain unchanged. TUI, macOS PTY, deployment build and document-wide checks are owned by the coordinator/runtime agent and are not claimed here.

Approved design: `review/issue2-auth-design.response.md` with effective corrections in `review/issue2-auth-design-correction.response.md`. In particular only an **omitted** HTTP/session argument may resolve from an accepted legacy request. An explicitly different session never changes an old hash into a matching one. Typed BUSY uses the existing error envelope rather than adding an unapproved mandatory head-info field.

## Implementation

- Additive `002_authority.sql` introduces `authority_registry` and `authority_turn`; embedded 001 bytes are untouched. Fresh Init creates random persistent backend instance/session IDs. Existing v1 Open refuses with an actionable migration requirement. `store.UpgradeAuthority(path, objects, selectedID) (*Store,error)` explicitly holds both real Core and Runner daemon role locks plus the migration lock, validates the full installed checksum chain, requires existing MASTER selection when such history exists, rejects internal-only selection, and never merges or rewrites old payloads. No MASTER history permits a fresh registered session. Unknown schema versions still refuse; backup verification accepts validated historical v1 and current v2 chains without silently upgrading.
- Store final admission performs exact idempotency replay first, then authority/answer/queue checks before publishing input objects. Queue count and PendingTurns only concern the registered session. Durable `accepted_seq`, not received_at, orders processing. Legacy other-session pending turns remain read-only and do not occupy the active queue.
- A fixed, no-follow, per-acquisition `.authority-consumer.lock` flock serializes Process through its summary, bounded synchronous Typed and direct main-state finalizers. Ordering is consumer lock → object publish lock → Store mutex → immediate transaction. No database transaction spans model waiting. An earlier pending turn or occupied consumer makes new Typed return AUTHORITY_BUSY/409 without consuming its request ID or archiving it; accepted request replay remains available.
- First processing persists the frozen state and event/row boundary in authority_turn. One predicate filters recent event content, input classification collection and READ_MEMORY. Completed prior turns and the current input are eligible; future pending input is excluded. Immutable MASTER event admission no longer increments cognitive revision. Terminal ASSISTANT and summary updates still CAS cognitive state under the consumer lock; current business readsets remain checked.
- Summary gets a fresh terminal-only prefix and stops before the first queued physical event, preserving contiguous coverage. SaveConversation rejects a claimed range containing pending turns. SYSTEM briefing uses explicit TaskLocal read-only authority projection while retaining task correlation IDs; public MASTER cannot use this builder mode and no internal conversation is created or advanced.

## Actual public API / migration differences

Existing `transport.Envelope` is unchanged: payload is **result**, not data.

`GET /v1/conversation` result: `instance_id`, `session_id`, `revision`, `history_sequence`, `summary_through_sequence`, `state` (full ConversationState including structured `pending_questions`), bounded `pending_turns`, and bounded `legacy_sessions` (`id`, `mode:READ_ONLY`). Legacy mapping list is capped at 100 and is not a writable selector.

`GET /v1/conversation/history` accepts nonnegative decimal `after_sequence` and `before_sequence`, `direction=forward|backward`, and `limit=1..100` (default 50). Missing bounds are zero; backward with no before bound selects the tail. Returned events are always ascending. Result also has `next_sequence`, `previous_sequence`, `has_more` (for the selected direction), `history_sequence` and `session_id`. Invalid strings, empty explicit values, signs, duplicates and overflowing integers are rejected rather than becoming zero. This log sequence is distinct from durable turn acceptance sequence and business event sequence.

POST inputs may omit session; POST actions/typed routes likewise resolve omission on the backend. Explicit null is rejected. GET requests retains its original receipt result with turn_id/state; GET turns supplies the complete InputTurn. AUTHORITY_SESSION_MISMATCH is 403, AUTHORITY_BUSY and AUTHORITY_TURN_NOT_HEAD are 409. New normal clients query the server; CLI migration is a management operation, not a conversation switch.

## This round's execution

`cd src && go test -race ./store ./tests -run '^TestIssue2AUTH' -count=1` — PASS, store 1.947s; tests 4.765s.

`cd src && go vet ./store ./core ./context ./memory ./transport` — PASS.

The JSON companion maps each AUTH identifier to actual selected test names and lists source hashes. These use isolated synthetic data, fake models, in-process actual HTTP handlers, and short database close/reopen samples. They are not real-model semantics or whole-system persistence requalification. TUI socket/PTY lifecycle evidence is separately recorded by runtime.

Initial test failures were test harness mistakes: the response helper expected `data` instead of existing `result`; a positive PERSONAL fixture changed service policy but retained the old fixture provider's SYNTHETIC-only encoder; a legacy fixture used a random principal instead of authenticated `master`. Those fixtures were corrected to represent the intended oracle. The public history integer parser was also found too permissive and fixed with a new directed rejection test. No failing production behavior has been relabeled as a pass without an executed correction.

Independent implementation review is requested separately; design approval is not a final implementation verdict. No Git commit or release build is claimed by this module report.
