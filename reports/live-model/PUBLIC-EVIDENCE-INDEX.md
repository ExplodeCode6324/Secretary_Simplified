# Public synthetic evidence

| Dataset | Result | Inspectable evidence |
|---|---|---|
| [Month run8](month-run8/report.json) | 90 Item checkpoints and actual persisted slots0–29 passed independent audits | [Historical identity audit](month-run8/report-identity-correction.json), [30 snapshot reference audit](month-run8/consciousness-reference-audit.json), all 336 reachable synthetic objects; public-only audits reproduce exactly |
| [Question run4](question-run4/report.json) | Full D11 chain PASS | Natural-language linked question creation, actual cross-session READ_MEMORY IDs, Core restart, explicit answer, unchanged OPEN Item, stable request replay and resolved-question HTTP409; same frozen original oracle |
| [Question run2 failure](question-run2-failed/report.json) | Correct original question IDs reached the model, but repeated READ_MEMORY exhausted budget | Exact archived-wire diagnosis, original outputs and fixed oracle preserved; no restart/answer success claimed |
| [Question run3 failure](question-run3-failed/report.json) | Required context overflow after question creation | Available pre-overflow wire and read-only diagnostics retained; no nonexistent final overflow wire reconstructed |
| [Month run7 failure](month-run7-failed/report.json) | 3 Item checkpoints passed, 6 failed with required-context overflow; 3 snapshots persisted | Original 9-checkpoint partial report, actual model calls and immutable audit export; not a complete month pass or full snapshot free-text audit |
| [CLI run1](cli-run1/report.json) / [run2](cli-run2/report.json) | Original real CLI reminder outcomes preserved | CLI transcripts, model evidence and runtime records within each declared scope |
| [Question run1 failure](question-run1-failed/report.json) | Question creation passed; cross-session retrieval exhausted budget | Frozen input/oracle, actual READ_MEMORY attempts and original failure preserved; no restart/answer success claimed |
| [Month run6](month-run6/report.json) | 90 item checkpoints and 30 generated snapshots; independent historical state and reference audits PASS | [Identity audit](month-run6/report-identity-correction.json), [reference audit](month-run6/consciousness-reference-audit.json), 344 verified synthetic objects and public-only byte-for-byte reproduction |
| [Month run5](month-run5/report.json) | 90 item checkpoints and30 generated snapshots passed within this source-time scope | Original IDs match independent reconstruction; [identity audit](month-run5/report-identity-correction.json), [reference audit](month-run5/consciousness-reference-audit.json), [free-text samples](month-run5/consciousness-free-text-samples.json), all Context/decision/object evidence |
| [Month run4](month-run4/report.json) | 90 item checkpoints and 30 generated consciousness snapshots passed within declared scope | Original report preserved; [independent identity correction](month-run4/report-identity-correction.json), [reference audit](month-run4/consciousness-reference-audit.json), [free-text samples](month-run4/consciousness-free-text-samples.json), logical audit rows, Context/decision records, model calls and objects |
| [Original scenario run2](scenarios-run2-failed/report.json) | 6/7; retrieval failure retained | Original input/oracle, actual Context and raw output, manifests and evidence objects |
| [Final scenario run4](scenarios-run4/report.json) | 7/7 | Same seven frozen semantic oracles; full evidence per scenario |
| [World reading run2](world-read-run2/report.json) | 2/2 | Fixed fact IDs, exact conflict group members, real authorization-permit setup and model reading of conflict/retraction; no claim of model-authorized world mutation |

Recent publication directories have a publication-manifest.json with exact file hashes and sizes. Legacy month-run1/2/3 lack this manifest and are explicitly marked LEGACY_NO_MANIFEST by the verifier. All copied objects are SYNTHETIC and verified against the original object metadata. Only reference-reachable objects are included. Configurations, API keys, authentication tokens, authorization-grant files, process state and SQLite databases are excluded. Existing historical month-run1/2/3 directories remain unchanged.

The original month report reused a mutable identity map; it remains intact as evidence of that reporting defect. The separate correction replays immutable Item events up to each turn's actual completion timestamp, checks one Item event per non-overlapping checkpoint interval, and compares the reconstructed state to authored oracles. To reproduce without a private database, copy month-run4/report.json into a new temporary output directory, then run:

```sh
python3 scripts/audit_month_evidence.py reports/live-model/month-run4 OUTPUT_DIRECTORY src/tests/fixtures/month
```

This writes two independent audit files in OUTPUT_DIRECTORY. The exported logical records contain the exact original turn JSON and immutable event rows; they do not include credentials or a runnable database. [Public-only reproduction](../implementation/public-evidence-reproduction.json) matched both existing audit files byte for byte.

These are synthetic model and offline verification results. Scope and remaining language limitations are recorded in [A23 supplemental evidence](../implementation/A23-supplemental-evidence.md); this is not a declaration of full M1–M3 acceptance or REAL_USE.

Month run5 is a later schema/source-time result, not evidence for any subsequent default-reminder or provider-prompt change. Its original per-checkpoint identity maps match the independent reconstruction; no alias correction was required. The same public-only command also works with month-run5 and reproduces its audit files exactly.

Month run6 preserves the original source report unchanged. Its 90 checkpoint states and all 30 snapshot references were independently checked against immutable Item events and the authored month oracle. Both audit files reproduce byte for byte using only public exports. This audit does not claim exhaustive free-text correctness, REAL_USE, or validation of later production changes.

Run `python3 scripts/verify_public_evidence.py --help` for usage. A normal invocation discovers every public dataset directory, scans both actual credential values without emitting them, checks published manifests and original report bytes where available, and reconstructs every available month audit from public exports. `--check-only` performs these checks without rewriting summaries. Month runs4,5,6,8 currently reproduce both audit files exactly.
