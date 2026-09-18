-- Secretary v1.0 empty-database baseline. Runtime services enforce business guards.
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA synchronous = FULL;
PRAGMA busy_timeout = 5000;

CREATE TABLE system_state (
  singleton INTEGER PRIMARY KEY CHECK(singleton=1),
  schema_version TEXT NOT NULL CHECK(schema_version='1.0'),
  logical_session_id TEXT NOT NULL UNIQUE,
  owner_epoch INTEGER NOT NULL CHECK(owner_epoch>=0),
  input_generation INTEGER NOT NULL CHECK(input_generation>=0),
  world_revision INTEGER NOT NULL CHECK(world_revision>=0),
  mode TEXT NOT NULL CHECK(mode IN ('FIXTURE','LIVE_SYNTHETIC','REAL_USE')),
  lifecycle TEXT NOT NULL CHECK(lifecycle IN ('RECOVERING','READY','PAUSED','DEGRADED'))
);
CREATE TABLE objects (
  id TEXT PRIMARY KEY,
  sha256 TEXT CHECK(sha256 IS NULL OR (length(sha256)=64 AND sha256 NOT GLOB '*[^0-9a-f]*')),
  bytes INTEGER NOT NULL CHECK(bytes>=0),
  media_type TEXT NOT NULL,
  classification TEXT NOT NULL CHECK(classification IN ('SYNTHETIC','PUBLIC','PERSONAL','SECRET')),
  state TEXT NOT NULL CHECK(state IN ('AVAILABLE','DELETED','MISSING')),
  relative_path TEXT UNIQUE,
  created_at TEXT NOT NULL,
  CHECK(state!='AVAILABLE' OR (sha256 IS NOT NULL AND relative_path IS NOT NULL))
);
CREATE TABLE requests (
  id TEXT PRIMARY KEY,
  client_id TEXT NOT NULL,
  payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64),
  kind TEXT NOT NULL,
  body_ref TEXT NOT NULL REFERENCES objects(id),
  state TEXT NOT NULL CHECK(state IN ('ACCEPTED','PROCESSING','COMPLETED','PAUSED','FAILED')),
  received_at TEXT NOT NULL,
  input_generation INTEGER NOT NULL CHECK(input_generation>=0)
);
CREATE TABLE events (
  seq INTEGER PRIMARY KEY AUTOINCREMENT CHECK(seq>0 AND seq<=9007199254740991),
  id TEXT NOT NULL UNIQUE,
  kind TEXT NOT NULL,
  source TEXT NOT NULL CHECK(source IN ('MASTER','CORE','WORKER','MODEL','SYSTEM')),
  request_id TEXT REFERENCES requests(id),
  task_id TEXT REFERENCES tasks(id) DEFERRABLE INITIALLY DEFERRED,
  cause_seq INTEGER REFERENCES events(seq),
  payload_ref TEXT REFERENCES objects(id),
  occurred_at TEXT NOT NULL,
  received_at TEXT NOT NULL
);
CREATE TRIGGER events_no_update BEFORE UPDATE ON events BEGIN SELECT RAISE(ABORT,'events are append-only'); END;
CREATE TRIGGER events_no_delete BEFORE DELETE ON events BEGIN SELECT RAISE(ABORT,'events are append-only'); END;
CREATE TABLE root_budgets (
  id TEXT PRIMARY KEY,
  request_id TEXT REFERENCES requests(id),
  budget_kind TEXT NOT NULL CHECK(budget_kind IN ('REQUEST','MAINTENANCE')),
  deadline_at TEXT NOT NULL,
  model_calls_limit INTEGER NOT NULL CHECK(model_calls_limit>0),
  model_calls_used INTEGER NOT NULL DEFAULT 0 CHECK(model_calls_used>=0),
  tool_calls_limit INTEGER NOT NULL CHECK(tool_calls_limit>0),
  tool_calls_used INTEGER NOT NULL DEFAULT 0 CHECK(tool_calls_used>=0),
  attempts_limit INTEGER NOT NULL CHECK(attempts_limit>0),
  attempts_used INTEGER NOT NULL DEFAULT 0 CHECK(attempts_used>=0),
  tokens_limit INTEGER NOT NULL CHECK(tokens_limit>0),
  tokens_used INTEGER NOT NULL DEFAULT 0 CHECK(tokens_used>=0),
  tokens_reserved INTEGER NOT NULL DEFAULT 0 CHECK(tokens_reserved>=0),
  money_limit_microunits INTEGER CHECK(money_limit_microunits>0),
  money_used_microunits INTEGER NOT NULL DEFAULT 0 CHECK(money_used_microunits>=0),
  money_reserved_microunits INTEGER NOT NULL DEFAULT 0 CHECK(money_reserved_microunits>=0),
  CHECK(model_calls_used<=model_calls_limit),
  CHECK(tool_calls_used<=tool_calls_limit),
  CHECK(attempts_used<=attempts_limit),
  CHECK(tokens_used+tokens_reserved<=tokens_limit),
  CHECK(money_limit_microunits IS NULL OR money_used_microunits+money_reserved_microunits<=money_limit_microunits),
  CHECK((budget_kind='REQUEST' AND request_id IS NOT NULL) OR budget_kind='MAINTENANCE')
);
CREATE TABLE tasks (
  id TEXT PRIMARY KEY,
  request_id TEXT NOT NULL REFERENCES requests(id),
  current_revision INTEGER NOT NULL CHECK(current_revision>0),
  state TEXT NOT NULL CHECK(state IN ('QUEUED','RUNNING','VERIFYING','PAUSED','SUCCEEDED','FAILED','CANCEL_REQUESTED','CANCELLED','RESULT_UNKNOWN')),
  pause_reason TEXT,
  cancel_generation INTEGER NOT NULL DEFAULT 0 CHECK(cancel_generation>=0),
  root_budget_id TEXT NOT NULL REFERENCES root_budgets(id),
  updated_at TEXT NOT NULL,
  FOREIGN KEY(id,current_revision) REFERENCES task_versions(task_id,revision) DEFERRABLE INITIALLY DEFERRED,
  CHECK(state!='PAUSED' OR pause_reason IS NOT NULL)
);
CREATE TABLE task_versions (
  task_id TEXT NOT NULL REFERENCES tasks(id) DEFERRABLE INITIALLY DEFERRED,
  revision INTEGER NOT NULL CHECK(revision>0),
  definition_ref TEXT NOT NULL REFERENCES objects(id),
  basis_event_seq INTEGER NOT NULL REFERENCES events(seq),
  created_at TEXT NOT NULL,
  PRIMARY KEY(task_id,revision)
);
CREATE TRIGGER task_versions_no_update BEFORE UPDATE ON task_versions BEGIN SELECT RAISE(ABORT,'immutable task version'); END;
CREATE TRIGGER task_versions_no_delete BEFORE DELETE ON task_versions BEGIN SELECT RAISE(ABORT,'immutable task version'); END;
CREATE TABLE grants (
  id TEXT PRIMARY KEY,
  revision INTEGER NOT NULL CHECK(revision>0),
  principal_id TEXT NOT NULL,
  task_id TEXT REFERENCES tasks(id),
  policy_ref TEXT NOT NULL REFERENCES objects(id),
  state TEXT NOT NULL CHECK(state IN ('ACTIVE','REVOKED','EXPIRED')),
  expires_at TEXT NOT NULL,
  basis_request_id TEXT NOT NULL REFERENCES requests(id)
);
CREATE TABLE attempts (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL,
  task_revision INTEGER NOT NULL,
  owner_epoch INTEGER NOT NULL CHECK(owner_epoch>0),
  state TEXT NOT NULL CHECK(state IN ('PREPARED','RUNNING','REPORTED','ABORTING','ENDED','UNKNOWN')),
  deadline_at TEXT NOT NULL,
  root_budget_id TEXT NOT NULL REFERENCES root_budgets(id),
  FOREIGN KEY(task_id,task_revision) REFERENCES task_versions(task_id,revision),
  UNIQUE(id,task_id,task_revision)
);
CREATE UNIQUE INDEX one_active_attempt_per_task ON attempts(task_id) WHERE state IN ('PREPARED','RUNNING','ABORTING');
CREATE TABLE operations (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL,
  task_revision INTEGER NOT NULL,
  originating_attempt_id TEXT NOT NULL,
  intent_ref TEXT NOT NULL REFERENCES objects(id),
  arguments_hash TEXT NOT NULL CHECK(length(arguments_hash)=64),
  grant_id TEXT NOT NULL REFERENCES grants(id),
  grant_revision INTEGER NOT NULL CHECK(grant_revision>0),
  owner_epoch INTEGER NOT NULL CHECK(owner_epoch>0),
  cancel_generation INTEGER NOT NULL CHECK(cancel_generation>=0),
  effect_class TEXT NOT NULL CHECK(effect_class IN ('READ_ONLY','IDEMPOTENT_WRITE','NON_IDEMPOTENT_WRITE')),
  state TEXT NOT NULL CHECK(state IN ('INTENDED','DISPATCHED','SUCCEEDED','FAILED','UNKNOWN','CANCELLED')),
  target_idempotency_key TEXT,
  idempotency_expires_at TEXT,
  FOREIGN KEY(originating_attempt_id,task_id,task_revision) REFERENCES attempts(id,task_id,task_revision),
  CHECK(effect_class!='IDEMPOTENT_WRITE' OR (target_idempotency_key IS NOT NULL AND idempotency_expires_at IS NOT NULL))
);
CREATE TABLE permits (
  id TEXT PRIMARY KEY,
  operation_id TEXT NOT NULL REFERENCES operations(id),
  executing_attempt_id TEXT NOT NULL REFERENCES attempts(id),
  owner_epoch INTEGER NOT NULL CHECK(owner_epoch>0),
  cancel_generation INTEGER NOT NULL CHECK(cancel_generation>=0),
  grant_revision INTEGER NOT NULL CHECK(grant_revision>0),
  arguments_hash TEXT NOT NULL CHECK(length(arguments_hash)=64),
  expires_at TEXT NOT NULL,
  consumed_at TEXT,
  revoked_at TEXT,
  CHECK(consumed_at IS NULL OR revoked_at IS NULL)
);
CREATE UNIQUE INDEX one_unconsumed_permit ON permits(operation_id) WHERE consumed_at IS NULL AND revoked_at IS NULL;
CREATE TABLE receipts (
  id TEXT PRIMARY KEY,
  operation_id TEXT NOT NULL REFERENCES operations(id),
  attempt_id TEXT NOT NULL REFERENCES attempts(id),
  owner_epoch INTEGER NOT NULL CHECK(owner_epoch>0),
  payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64),
  payload_ref TEXT NOT NULL REFERENCES objects(id),
  disposition TEXT NOT NULL CHECK(disposition IN ('ACCEPTED','QUARANTINED')),
  observed_at TEXT NOT NULL
);
CREATE TRIGGER receipts_no_update BEFORE UPDATE ON receipts BEGIN SELECT RAISE(ABORT,'immutable receipt'); END;
CREATE TRIGGER receipts_no_delete BEFORE DELETE ON receipts BEGIN SELECT RAISE(ABORT,'immutable receipt'); END;
CREATE TABLE verifications (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL,
  task_revision INTEGER NOT NULL,
  attempt_id TEXT NOT NULL REFERENCES attempts(id),
  payload_ref TEXT NOT NULL REFERENCES objects(id),
  outcome TEXT NOT NULL CHECK(outcome IN ('PASS','FAIL','UNVERIFIED')),
  event_seq INTEGER NOT NULL REFERENCES events(seq),
  FOREIGN KEY(task_id,task_revision) REFERENCES task_versions(task_id,revision)
);
CREATE TABLE obligations (
  id TEXT PRIMARY KEY,
  revision INTEGER NOT NULL CHECK(revision>0),
  kind TEXT NOT NULL CHECK(kind IN ('REQUIREMENT','PROHIBITION','PROMISE','QUESTION')),
  text_ref TEXT NOT NULL REFERENCES objects(id),
  scope_kind TEXT NOT NULL CHECK(scope_kind IN ('GLOBAL','SESSION','TASK')),
  scope_ref TEXT,
  source_event_seq INTEGER NOT NULL REFERENCES events(seq),
  state TEXT NOT NULL CHECK(state IN ('ACTIVE','RESOLVED','CANCELLED')),
  due_at TEXT,
  resolution_event_seq INTEGER REFERENCES events(seq),
  CHECK((scope_kind='GLOBAL' AND scope_ref IS NULL) OR (scope_kind!='GLOBAL' AND scope_ref IS NOT NULL)),
  CHECK((state='ACTIVE' AND resolution_event_seq IS NULL) OR (state!='ACTIVE' AND resolution_event_seq IS NOT NULL))
);
CREATE TABLE entities (id TEXT PRIMARY KEY, names_ref TEXT NOT NULL REFERENCES objects(id), revision INTEGER NOT NULL CHECK(revision>0));
CREATE TABLE proposals (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL CHECK(kind IN ('FACT','OBLIGATION','TASK_PATCH')),
  revision INTEGER NOT NULL CHECK(revision>0),
  payload_ref TEXT NOT NULL REFERENCES objects(id),
  state TEXT NOT NULL CHECK(state IN ('PENDING','ACCEPTED','REJECTED','STALE')),
  source_event_seq INTEGER NOT NULL REFERENCES events(seq),
  decision_event_seq INTEGER REFERENCES events(seq)
);
CREATE TABLE facts (
  id TEXT PRIMARY KEY,
  current_revision INTEGER NOT NULL CHECK(current_revision>0),
  subject_id TEXT NOT NULL REFERENCES entities(id),
  predicate TEXT NOT NULL,
  status TEXT NOT NULL CHECK(status IN ('ACTIVE','CONFLICTED','SUPERSEDED','RETRACTED')),
  valid_from TEXT,
  valid_to TEXT,
  supersedes_fact_id TEXT REFERENCES facts(id),
  FOREIGN KEY(id,current_revision) REFERENCES fact_versions(fact_id,revision) DEFERRABLE INITIALLY DEFERRED
);
CREATE TABLE fact_versions (
  fact_id TEXT NOT NULL REFERENCES facts(id) DEFERRABLE INITIALLY DEFERRED,
  revision INTEGER NOT NULL CHECK(revision>0),
  payload_ref TEXT NOT NULL REFERENCES objects(id),
  event_seq INTEGER NOT NULL REFERENCES events(seq),
  PRIMARY KEY(fact_id,revision)
);
CREATE TRIGGER fact_versions_no_update BEFORE UPDATE ON fact_versions BEGIN SELECT RAISE(ABORT,'immutable fact version'); END;
CREATE TRIGGER fact_versions_no_delete BEFORE DELETE ON fact_versions BEGIN SELECT RAISE(ABORT,'immutable fact version'); END;
CREATE TABLE working_memory (
  version INTEGER PRIMARY KEY CHECK(version>0),
  id TEXT NOT NULL UNIQUE,
  watermark INTEGER NOT NULL CHECK(watermark>=0),
  history_exit_seq INTEGER NOT NULL CHECK(history_exit_seq>=0 AND history_exit_seq<=watermark),
  payload_ref TEXT NOT NULL REFERENCES objects(id),
  world_revision INTEGER NOT NULL CHECK(world_revision>=0)
);
CREATE TABLE memory_cursor (
  singleton INTEGER PRIMARY KEY CHECK(singleton=1),
  memory_version INTEGER REFERENCES working_memory(version),
  watermark INTEGER NOT NULL CHECK(watermark>=0),
  history_exit_seq INTEGER NOT NULL CHECK(history_exit_seq>=0 AND history_exit_seq<=watermark)
);
CREATE TABLE memory_jobs (
  id TEXT PRIMARY KEY,
  from_seq INTEGER NOT NULL CHECK(from_seq>0),
  to_seq INTEGER NOT NULL CHECK(to_seq>=from_seq),
  state TEXT NOT NULL CHECK(state IN ('PENDING','RUNNING','FAILED','BLOCKED','COMPLETED')),
  retries INTEGER NOT NULL DEFAULT 0 CHECK(retries>=0),
  draft_ref TEXT REFERENCES objects(id),
  error_ref TEXT REFERENCES objects(id),
  root_budget_id TEXT NOT NULL REFERENCES root_budgets(id)
);
CREATE TABLE contexts (
  id TEXT PRIMARY KEY,
  manifest_ref TEXT NOT NULL REFERENCES objects(id),
  cut_seq INTEGER NOT NULL CHECK(cut_seq>=0),
  input_generation INTEGER NOT NULL CHECK(input_generation>=0),
  memory_version INTEGER REFERENCES working_memory(version),
  payload_hash TEXT NOT NULL CHECK(length(payload_hash)=64)
);
CREATE TABLE model_calls (
  id TEXT PRIMARY KEY,
  root_budget_id TEXT NOT NULL REFERENCES root_budgets(id),
  context_id TEXT NOT NULL REFERENCES contexts(id),
  role TEXT NOT NULL CHECK(role IN ('MAIN','CHILD','MEMORY','VERIFIER')),
  profile_id TEXT NOT NULL,
  model_id TEXT NOT NULL,
  status TEXT NOT NULL CHECK(status IN ('RESERVED','SENT','COMPLETED','FAILED','UNKNOWN','REJECTED')),
  reserved_tokens INTEGER NOT NULL CHECK(reserved_tokens>=0),
  reserved_money_microunits INTEGER NOT NULL DEFAULT 0 CHECK(reserved_money_microunits>=0),
  charged_money_microunits INTEGER NOT NULL DEFAULT 0 CHECK(charged_money_microunits>=0),
  input_tokens INTEGER NOT NULL DEFAULT 0 CHECK(input_tokens>=0),
  output_tokens INTEGER NOT NULL DEFAULT 0 CHECK(output_tokens>=0),
  usage_known INTEGER NOT NULL DEFAULT 0 CHECK(usage_known IN (0,1)),
  created_at TEXT NOT NULL
);
CREATE TABLE replies (
  id TEXT PRIMARY KEY,
  request_id TEXT NOT NULL REFERENCES requests(id),
  input_generation INTEGER NOT NULL CHECK(input_generation>=0),
  body_ref TEXT NOT NULL REFERENCES objects(id),
  task_id TEXT,
  task_revision INTEGER,
  delivery_state TEXT NOT NULL CHECK(delivery_state IN ('PENDING','SENT','ACKED','UNKNOWN')),
  committed_seq INTEGER NOT NULL REFERENCES events(seq),
  FOREIGN KEY(task_id,task_revision) REFERENCES task_versions(task_id,revision),
  CHECK((task_id IS NULL AND task_revision IS NULL) OR (task_id IS NOT NULL AND task_revision IS NOT NULL))
);
CREATE TABLE outbox (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL CHECK(kind IN ('DISPATCH','CANCEL','REPLY')),
  dedup_key TEXT NOT NULL UNIQUE,
  payload_ref TEXT NOT NULL REFERENCES objects(id),
  state TEXT NOT NULL CHECK(state IN ('PENDING','CLAIMED','DELIVERED','BLOCKED')),
  owner_epoch INTEGER,
  available_at TEXT NOT NULL,
  claimed_until TEXT,
  attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts>=0)
);
CREATE TABLE object_links (
  object_id TEXT NOT NULL REFERENCES objects(id),
  owner_kind TEXT NOT NULL,
  owner_id TEXT NOT NULL,
  relation TEXT NOT NULL,
  PRIMARY KEY(object_id,owner_kind,owner_id,relation)
);
CREATE TABLE deletions (
  seq INTEGER PRIMARY KEY AUTOINCREMENT,
  request_id TEXT NOT NULL REFERENCES requests(id),
  object_id TEXT NOT NULL REFERENCES objects(id),
  deleted_at TEXT NOT NULL,
  policy_version TEXT NOT NULL
);
CREATE TABLE migrations (
  version TEXT PRIMARY KEY,
  checksum TEXT NOT NULL,
  applied_at TEXT NOT NULL
);
CREATE INDEX event_task_seq ON events(task_id,seq);
CREATE INDEX task_state_updated ON tasks(state,updated_at);
CREATE INDEX outbox_ready ON outbox(state,available_at);
CREATE INDEX fact_subject_predicate ON facts(subject_id,predicate,status);
CREATE INDEX obligations_active ON obligations(state,scope_kind,scope_ref);
PRAGMA user_version = 100;
