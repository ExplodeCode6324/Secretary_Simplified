# Issue #2 AUTH-01—08 minimum proposal (awaiting Ayanami)

Only new AUTH/TUI/DOC/BUILD directed validation; no old Context/Store suite, whole-repository tests, month replay or A25. Preserve issue #1 protocol and all old evidence.

## Authority and compatibility

Add an additive, checksummed migration 002 (do not edit 001 bytes): `authority_registry` singleton row (instance UUID, active session UUID, registration timestamp/mapping reason) and `authority_turn` (turn ID unique, durable accepted sequence unique, frozen conversation state/prefix metadata). Retain conversation/event/input rows unchanged. Explicit migration operation upgrades a compatible v1 database; normal Open validates installed versions, does not silently migrate an unknown database.

Fresh init registers a random backend session and instance. Existing v1 data requires an explicit selected MASTER conversation ID if history exists; no automatic concatenation, classification rewriting, ID rewriting or implicit activation of internal task sessions. Other sessions remain readable as legacy mappings, never eligible for new public input. Existing exact request replay is checked before current authority rejection and retains its original session/hash. Pending old non-authority turns are retained but not consumed; report them read-only rather than silently executing or deleting. Root CLI upgrade provides the explicit selection flag/configuration.

Store AcceptInput/AcceptTyped validate the authority in their final transaction after original idempotent replay, before archive or new records. HTTP may omit session and bind the registered ID before schema decode; explicit null/foreign ID is rejected. Typed and direct Store calls cannot create a branch. Config/client identity is not authority. Public mutation of world/tasks remains existing bounded executor control and does not create a separate conversation.

## Ordering, prefix and summary

Every newly accepted main turn gets a server durable acceptance sequence in the same transaction. PendingTurns uses this sequence, never client timestamps, and only the registered session. Main Process is protected by a separate fixed-inode, per-acquisition OS flock across Store/processes, with cancellable waiting; no SQLite transaction spans model waiting. Under lock, require the earliest nonterminal turn, and freeze its conversation view/prefix on first processing in authority_turn. Preserve this frozen view across process restart and semantic retries. Completed prior replies plus own MASTER event are eligible; later pending MASTER events are excluded even if their physical log sequence precedes an earlier ASSISTANT reply. Physical conversation sequence remains immutable audit append order, acceptance sequence remains separate.

Admission appends immutable MASTER events but must not increment cognitive ConversationState revision (it does not change summary/questions/focus). Finished ASSISTANT updates continue to advance cognitive revision; Builder records frozen cognitive revision plus business readset. Later admission therefore neither leaks text/classification nor creates spurious conversation CAS conflicts. SnapshotForTurn filters recent and source-input classification collection by eligible turns, replaces conversation view with its frozen prefix state; READ_MEMORY must also exclude future/unprocessed turns from the current authoritative session. Global authoritative Item/Task/Fact checks and original permission checks remain intact.

Summary runs only under the same main consumer lock after a terminal turn; only terminal prefix + corresponding events are eligible. It cannot summarize queued future input. Its watermark is the largest fully covered physical sequence for which no preceding unprocessed MASTER event is being skipped (or stops before such a gap); no claiming contiguous coverage across an excluded future event. Keep existing summary CAS and problem ownership. Background consciousness is a derived role, not an independent session.

For deterministic Typed while an earlier model turn is pending: proposed minimal behavior is explicit `AUTHORITY_BUSY` (409), no acceptance/object/side effect. Under the same consumer lock, Typed can synchronously accept/commit only when the queue is empty; this preserves ordering without persisting arbitrary callbacks as new commands. Existing request replay bypasses this new busy rejection. Confirm whether this explicit rejection satisfies unified admission; alternative is a durable typed-command queue with a registered action payload, requiring larger change. No model waiting for a rejected Typed request.

## Shared authenticated API (transport envelope unchanged)

GET `/v1/conversation` data:
`{instance_id,session_id,revision,history_sequence,summary_through_sequence,state:ConversationState,pending_turns:InputTurn[],legacy_sessions:[{id,mode:"READ_ONLY"}]}`.
State includes structured pending_questions, summary, focus. Pending list bounded to existing queue maximum. This is an atomic read snapshot with latest event watermark, not client history upload.

GET `/v1/conversation/history?after_sequence=0&before_sequence=0&direction=forward&limit=50` data:
`{session_id,events:ConversationEvent[],next_sequence,previous_sequence,has_more,history_sequence}`. Ascending returned events; direction=backward selects latest events before an exclusive optional boundary (omitted boundary means latest page), forward selects after_sequence. Limit 1..100, stable server sequence. Clients keep view cursor, not cognitive state. No raw-body identity parsing.

GET `/v1/requests/{request_id}` keeps existing receipt compatibility including turn_id/state; caller then fetches `/v1/turns/{id}` for full final turn. Unknown request 404. Normal new requests no longer need --session; explicitly supplied foreign IDs are rejected except exact accepted legacy replay. A legacy history query is read-only and explicitly identified, not a conversation selector.

## AUTH directed evidence

Small fake-model suite: two HTTP clients same backend ID; shared pagination and A→B context; structured cross-client answer and race; delayed first model with future canary excluded and terminal ordering; lost response exact replay + one backend reopen; direct Store/HTTP/Typed foreign session denial + exact legacy receipt replay; explicit legacy mapping with preserved bytes and separate instance IDs; preseeded summary/questions/watermark resync. No old suites. Root updates all affected current docs/mirrors and BUILD artifacts; runtime owns TUI/PTY.

Migration also holds the actual dataDir/run/core.lock and runner.lock plus migration.lock; both roles must be stopped. Existing v1 with no MASTER history may explicitly migrate to a fresh authority; multiple candidates require selection and internal-only IDs cannot be selected. Backup schema compatibility only validates the installed checksum chain, no old backup suite is executed.
