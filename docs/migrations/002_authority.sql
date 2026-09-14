-- Issue #2 additive singleton authority registry; 001 baseline remains immutable.
CREATE TABLE authority_registry (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1),
 instance_id TEXT NOT NULL UNIQUE,
 session_id TEXT NOT NULL UNIQUE REFERENCES conversation_session(id),
 registered_at TEXT NOT NULL,
 mapping_reason TEXT NOT NULL
);
CREATE TABLE authority_turn (
 turn_id TEXT PRIMARY KEY REFERENCES input_turn(id),
 accepted_seq INTEGER NOT NULL UNIQUE CHECK(accepted_seq>=1),
 prefix_sequence INTEGER,
 prefix_rowid INTEGER,
 frozen_state_json TEXT CHECK(frozen_state_json IS NULL OR json_valid(frozen_state_json))
);
