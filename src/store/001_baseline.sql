-- Secretary_Simplified v1 design DDL. Apply only to a new, isolated database.
-- Connection initialization in the application additionally sets busy_timeout=5000.
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA synchronous = FULL;
BEGIN IMMEDIATE;
CREATE TABLE schema_migration (
  version INTEGER PRIMARY KEY, checksum TEXT NOT NULL, applied_at TEXT NOT NULL
);
CREATE TABLE object_ref (
  id TEXT NOT NULL PRIMARY KEY, relative_path TEXT NOT NULL UNIQUE, sha256 TEXT NOT NULL,
  media_type TEXT NOT NULL, byte_size INTEGER NOT NULL CHECK(byte_size >= 0),
  data_class TEXT NOT NULL CHECK(data_class IN ('SYNTHETIC','PERSONAL','SENSITIVE','SECRET')),
  created_at TEXT NOT NULL
);
CREATE TABLE request_receipt (
  principal_id TEXT NOT NULL, request_id TEXT NOT NULL, payload_hash TEXT NOT NULL,
  intent_id TEXT NOT NULL, state TEXT NOT NULL CHECK(state IN ('ACCEPTED','COMMITTED','REJECTED')),
  response_json TEXT NOT NULL CHECK(json_valid(response_json)), created_at TEXT NOT NULL,
  PRIMARY KEY(principal_id, request_id)
);
CREATE TABLE source_state (
  id TEXT NOT NULL PRIMARY KEY, kind TEXT NOT NULL, revision INTEGER NOT NULL CHECK(revision >= 1),
  cursor_json TEXT CHECK(cursor_json IS NULL OR json_valid(cursor_json)),
  last_success_at TEXT, stale_after_seconds INTEGER NOT NULL CHECK(stale_after_seconds > 0),
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json))
);
CREATE TABLE source_record (
  id TEXT NOT NULL PRIMARY KEY, source_id TEXT NOT NULL REFERENCES source_state(id),
  external_id TEXT NOT NULL, source_version TEXT NOT NULL, content_hash TEXT NOT NULL,
  raw_ref TEXT NOT NULL REFERENCES object_ref(id), observed_at TEXT NOT NULL,
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),
  UNIQUE(source_id, external_id, source_version)
);
CREATE TABLE observation (
  id TEXT NOT NULL PRIMARY KEY, source_record_id TEXT REFERENCES source_record(id),
  entity_id TEXT NOT NULL, kind TEXT NOT NULL, observed_at TEXT NOT NULL, valid_until TEXT,
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json))
);
CREATE TABLE item (
  id TEXT NOT NULL PRIMARY KEY, revision INTEGER NOT NULL CHECK(revision >= 1), domain TEXT NOT NULL,
  status TEXT NOT NULL CHECK(status IN ('OPEN','IN_PROGRESS','BLOCKED','DONE','CANCELLED')),
  due_at TEXT, payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX item_due ON item(status, due_at);
CREATE TABLE item_dependency (
  item_id TEXT NOT NULL REFERENCES item(id), depends_on_id TEXT NOT NULL REFERENCES item(id),
  PRIMARY KEY(item_id, depends_on_id), CHECK(item_id <> depends_on_id)
);
CREATE TABLE world_proposal (
  id TEXT NOT NULL PRIMARY KEY, request_id TEXT NOT NULL, proposal_hash TEXT NOT NULL,
  state TEXT NOT NULL CHECK(state IN ('PENDING','ACCEPTED','CONTESTED','REJECTED')),
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), created_at TEXT NOT NULL
);
CREATE TABLE world_fact_version (
  fact_id TEXT NOT NULL, revision INTEGER NOT NULL CHECK(revision >= 1),
  entity_id TEXT NOT NULL, predicate TEXT NOT NULL,
  status TEXT NOT NULL CHECK(status IN ('CANDIDATE','ACTIVE','CONTESTED','RETRACTED','SUPERSEDED')),
  proposal_id TEXT NOT NULL REFERENCES world_proposal(id),
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), created_at TEXT NOT NULL,
  PRIMARY KEY(fact_id, revision)
);
CREATE TABLE world_fact_head (
  fact_id TEXT NOT NULL PRIMARY KEY, revision INTEGER NOT NULL,
  FOREIGN KEY(fact_id, revision) REFERENCES world_fact_version(fact_id, revision)
);
CREATE INDEX world_fact_lookup ON world_fact_version(entity_id, predicate, status);
CREATE TABLE conversation_session (
  id TEXT NOT NULL PRIMARY KEY, revision INTEGER NOT NULL CHECK(revision >= 1),
  through_sequence INTEGER NOT NULL DEFAULT 0, payload_json TEXT NOT NULL CHECK(json_valid(payload_json))
);
CREATE TABLE conversation_event (
  id TEXT NOT NULL PRIMARY KEY, session_id TEXT NOT NULL REFERENCES conversation_session(id),
  sequence INTEGER NOT NULL CHECK(sequence >= 1), role TEXT NOT NULL,
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), created_at TEXT NOT NULL,
  UNIQUE(session_id, sequence)
);
CREATE TABLE input_turn (
  id TEXT NOT NULL PRIMARY KEY, session_id TEXT NOT NULL REFERENCES conversation_session(id),
  principal_id TEXT NOT NULL, request_id TEXT NOT NULL, intent_id TEXT NOT NULL,
  state TEXT NOT NULL CHECK(state IN ('PENDING','PROCESSING','COMMITTED','FAILED')),
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), updated_at TEXT NOT NULL,
  UNIQUE(principal_id, request_id),
  FOREIGN KEY(principal_id, request_id) REFERENCES request_receipt(principal_id, request_id)
);
CREATE TABLE consciousness_snapshot (
  id TEXT NOT NULL PRIMARY KEY, slot INTEGER NOT NULL UNIQUE CHECK(slot >= 0), revision INTEGER NOT NULL,
  snapshot_seq INTEGER NOT NULL, created_at TEXT NOT NULL,
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json))
);
CREATE TABLE context_manifest (
  id TEXT NOT NULL PRIMARY KEY, intent_id TEXT NOT NULL, snapshot_seq INTEGER NOT NULL,
  request_hash TEXT NOT NULL, artifact_ref TEXT REFERENCES object_ref(id),
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), created_at TEXT NOT NULL
);
CREATE TABLE decision_record (
  id TEXT NOT NULL PRIMARY KEY, intent_id TEXT NOT NULL, context_id TEXT NOT NULL REFERENCES context_manifest(id),
  state TEXT NOT NULL CHECK(state IN ('VALID','INVALID','COMMITTED','CONFLICT')),
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), created_at TEXT NOT NULL
);
CREATE TABLE authorization_grant (
  id TEXT NOT NULL PRIMARY KEY, principal_id TEXT NOT NULL, revision INTEGER NOT NULL CHECK(revision >= 1),
  revoked INTEGER NOT NULL DEFAULT 0 CHECK(revoked IN (0,1)), expires_at TEXT NOT NULL,
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json))
);
CREATE TABLE root_budget (
  root_id TEXT NOT NULL PRIMARY KEY, revision INTEGER NOT NULL CHECK(revision >= 1),
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json))
);
CREATE TABLE scheduled_job (
  id TEXT NOT NULL PRIMARY KEY, revision INTEGER NOT NULL CHECK(revision >= 1),
  root_id TEXT NOT NULL REFERENCES root_budget(root_id),
  enabled INTEGER NOT NULL CHECK(enabled IN (0,1)), next_due_at TEXT,
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), updated_at TEXT NOT NULL
);
CREATE INDEX scheduled_job_due ON scheduled_job(enabled, next_due_at);
CREATE TABLE task (
  id TEXT NOT NULL PRIMARY KEY, revision INTEGER NOT NULL CHECK(revision >= 1),
  root_id TEXT NOT NULL REFERENCES root_budget(root_id), item_id TEXT REFERENCES item(id),
  job_id TEXT REFERENCES scheduled_job(id), occurrence_key TEXT UNIQUE,
  state TEXT NOT NULL CHECK(state IN ('PENDING','RUNNING','VERIFYING','WAITING','NEEDS_ATTENTION','SUCCEEDED','FAILED','CANCELLED')),
  criterion_hash TEXT NOT NULL, cancel_generation INTEGER NOT NULL DEFAULT 0,
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), updated_at TEXT NOT NULL
);
CREATE TABLE command_ledger (
  id TEXT NOT NULL PRIMARY KEY, intent_id TEXT NOT NULL, operation_key TEXT NOT NULL,
  payload_hash TEXT NOT NULL, task_id TEXT REFERENCES task(id), job_id TEXT REFERENCES scheduled_job(id),
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), created_at TEXT NOT NULL,
  UNIQUE(intent_id, operation_key)
);
CREATE TABLE job_run (
  id TEXT NOT NULL PRIMARY KEY, task_id TEXT NOT NULL REFERENCES task(id), job_id TEXT REFERENCES scheduled_job(id),
  job_revision INTEGER, occurrence_key TEXT NOT NULL UNIQUE, scheduled_for TEXT NOT NULL,
  state TEXT NOT NULL CHECK(state IN ('QUEUED','CLAIMED','RUNNING','SUCCEEDED','FAILED','RESULT_UNKNOWN','CANCELLED','SKIPPED')),
  attempt_no INTEGER NOT NULL DEFAULT 0, fencing_token INTEGER NOT NULL DEFAULT 0,
  lease_owner TEXT, lease_until TEXT, external_idempotency_key TEXT NOT NULL UNIQUE,
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), updated_at TEXT NOT NULL
);
CREATE INDEX job_run_queue ON job_run(state, scheduled_for);
CREATE TABLE core_work (
  run_id TEXT NOT NULL PRIMARY KEY REFERENCES job_run(id), attempt_no INTEGER NOT NULL,
  fencing_token INTEGER NOT NULL, command_hash TEXT NOT NULL,
  state TEXT NOT NULL CHECK(state IN ('QUEUED','RUNNING','SUCCEEDED','FAILED','RESULT_UNKNOWN','CANCELLED')),
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), updated_at TEXT NOT NULL
);
CREATE TABLE execution_attempt (
  run_id TEXT NOT NULL REFERENCES job_run(id), attempt_no INTEGER NOT NULL CHECK(attempt_no >= 1),
  fencing_token INTEGER NOT NULL, dispatch_state TEXT NOT NULL CHECK(dispatch_state IN ('PREPARED','DISPATCHED','FINISHED')),
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), PRIMARY KEY(run_id, attempt_no)
);
CREATE TABLE execution_permit (
  id TEXT NOT NULL PRIMARY KEY, run_id TEXT NOT NULL REFERENCES job_run(id),
  grant_id TEXT NOT NULL REFERENCES authorization_grant(id), grant_revision INTEGER NOT NULL,
  fencing_token INTEGER NOT NULL, cancel_generation INTEGER NOT NULL, expires_at TEXT NOT NULL,
  proposal_hash TEXT, consumed_at TEXT,
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json))
);
CREATE TABLE executor_receipt (
  id TEXT NOT NULL PRIMARY KEY, run_id TEXT NOT NULL REFERENCES job_run(id), attempt_no INTEGER NOT NULL,
  receipt_key TEXT NOT NULL, payload_hash TEXT NOT NULL, received_at TEXT NOT NULL,
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), UNIQUE(run_id, receipt_key),
  FOREIGN KEY(run_id, attempt_no) REFERENCES execution_attempt(run_id, attempt_no)
);
CREATE TABLE verification (
  id TEXT NOT NULL PRIMARY KEY, task_id TEXT NOT NULL REFERENCES task(id), criterion_hash TEXT NOT NULL,
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), created_at TEXT NOT NULL
);
CREATE TABLE wait_subscription (
  id TEXT NOT NULL PRIMARY KEY, task_id TEXT NOT NULL REFERENCES task(id), generation INTEGER NOT NULL,
  cursor_seq INTEGER NOT NULL, deadline_at TEXT NOT NULL, state TEXT NOT NULL CHECK(state IN ('ARMED','WOKEN','EXPIRED','CANCELLED')),
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), UNIQUE(task_id, generation)
);
CREATE TABLE change_event (
  seq INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT NOT NULL UNIQUE, root_id TEXT NOT NULL,
  entity_type TEXT NOT NULL, entity_id TEXT NOT NULL, entity_revision INTEGER NOT NULL,
  event_type TEXT NOT NULL, causation_id TEXT, created_at TEXT NOT NULL,
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json))
);
CREATE INDEX change_event_entity ON change_event(entity_type, entity_id, seq);
CREATE TABLE consumer_cursor (
  consumer_id TEXT NOT NULL PRIMARY KEY, last_seq INTEGER NOT NULL, revision INTEGER NOT NULL
);
CREATE TABLE rule_state (
  rule_id TEXT NOT NULL, root_id TEXT NOT NULL, next_allowed_at TEXT,
  no_progress_count INTEGER NOT NULL DEFAULT 0, cursor_seq INTEGER NOT NULL DEFAULT 0,
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), PRIMARY KEY(rule_id, root_id)
);
CREATE TABLE alarm_session (
  id TEXT NOT NULL PRIMARY KEY, run_id TEXT NOT NULL UNIQUE REFERENCES job_run(id), revision INTEGER NOT NULL,
  state TEXT NOT NULL CHECK(state IN ('STARTING','PLAYING','STOPPING','STOPPED','UNKNOWN','FAILED')),
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json))
);
CREATE TABLE notification (
  id TEXT NOT NULL PRIMARY KEY, run_id TEXT REFERENCES job_run(id), notification_key TEXT NOT NULL UNIQUE,
  state TEXT NOT NULL CHECK(state IN ('PENDING','DELIVERED','ACKNOWLEDGED')),
  payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), created_at TEXT NOT NULL
);
COMMIT;
