# M3 backup and diagnostics evidence

`Store.Backup` uses the modernc SQLite online backup API, reads object references from that consistent snapshot, copies immutable objects with hash checks, and only completes the manifest after all artifacts exist. Failures retain an INCOMPLETE marker. It never copies a live SQLite main file as a backup. Automatic object GC does not exist, so referenced objects cannot be concurrently collected by this implementation.

`VerifyBackup` checks the database hash, schema checksum/version, SQLite integrity and foreign keys, exact object count and each immutable object hash/size. `Restore` requires a new empty target, rechecks bytes while copying and writes an execution_frozen marker before any restored database is opened. CLI/Runner integration must also enforce the marker and set execution_frozen in restored configuration; no recovered side effect should be replayed merely because a restore completed.

`diagnostics.Doctor` reports actual schema/integrity/FK checks, persisted queues, source sync times, consciousness slot and unknown/conflicted run data. Missing heartbeat, scan, backup or budget-exhaustion telemetry remains UNKNOWN/null. It never fabricates a PASS for unknown telemetry. Physical free bytes use the host filesystem query.

Verified `go test ./store ./diagnostics`: concurrent writes during backup, successful isolated frozen restore, refusal of nonempty restore targets, tampered object rejection, corrupted database rejection, and missing-heartbeat honesty. These are synthetic local tests, not hardware failure or external-account recovery validation. Independent Ayanami review requested under review/.
