# M1 Foundation implementation evidence

Implemented the complete baseline DDL in `src/store/001_baseline.sql`, embedded Draft 2020-12 contract and all generated DTOs, strict format/type validation, registered runtime extensions, duplicate-key rejection and semantic integer JSON golden test. DTO regeneration: `python3 src/contract/generate.py` from repository root.

Storage uses modernc SQLite, WAL/FULL, foreign keys and 5000 ms busy timeout on each connection. `Init` exclusively locks migration and installs baseline into a new database; `Open` requires an existing matching version/checksum. Transaction writers are serialized per Store and use SQLite immediate transactions across processes. ObjectStore syncs immutable files before registering their references, constrains reads with `os.Root`, and checks hash/size.

Item repository implements CAS, dependency DAG checks, parent/evidence validation and transactional before/after events. Fixture ingestion retains raw records, deduplicates source versions, preserves the admitted record on same-version/different-hash conflict, and records incoming quarantine metadata. Fixture absence does not delete records or silently promote source data to authoritative Items.

Verified with `go test ./contract ./platform ./store ./ingest`. Tests include strict unknown fields and date format, closed output schemas, duplicate keys, canonical bytes, Item CAS/dependency-cycle rollback/event count, all four connection pragmas, object tampering, source duplicate/conflict/quarantine. These are synthetic local tests; they do not certify real sources, provider inference quality or operational deployment. Ayanami independent review requested separately under `review/`.

Dependency selection checked against official package documentation and Go module resolution on implementation day: modernc.org/sqlite v1.58.0, github.com/santhosh-tekuri/jsonschema/v6 v6.0.3; local Go 1.25.6. Exact transitive dependencies are in src/go.mod and src/go.sum. References: https://pkg.go.dev/modernc.org/sqlite and https://github.com/santhosh-tekuri/jsonschema/releases . No claim of independent security audit is made.

Known boundary: incoming conflict quarantine is an immutable file plus ObjectRef rather than a second SourceRecord sharing the forbidden unique source/version key. Object creation may leave an unreferenced immutable blob on later transaction failure; no automatic garbage collection removes evidence.
