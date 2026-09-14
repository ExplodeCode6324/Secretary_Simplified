# Issue #1 GATE-01 / GATE-02

Scope: invented data, isolated temporary directories, real Core/SQLite/Provider policy and encoding, fake HTTP transport; no external account calls and no changes to deployed ProviderPolicy.

Chosen delivery boundary: existing PERSONAL text input only; real source-file synchronization remains unavailable. Independent data directories isolate permitted trial data from local-only/unknown-class data. Same-database mixed-permission availability is not promised.

Changes: `docs/RealDataTrial.md`, `docs/Operations.md`, root and release README clarify fixture-only source paths, explicit provider/data-scope authorization, empty source configuration, SECRET prohibition and conservative whole-snapshot disclosure behavior. No production disclosure check removed or lowered.

Actual regression commands (from `src`):

```sh
go test ./tests -run TestIssue1GatePersonalAuthorizationAndDirectoryIsolation -count=1
go test -race ./tests -run 'TestIssue1GatePersonalAuthorizationAndDirectoryIsolation|TestClassificationTypedAndHTTPDefaults' -count=1
```

Both PASS (2026-09-14). Test assertions cover public HTTP default PERSONAL, unauthorized zero provider calls, authorized one-call committed reply, separate-directory SECRET canary absence from actual wire, same-database conservative denial and no PERSONAL/SECRET canary in model diagnostic archives. Existing CLI default coverage is recorded in `D12-cli-class-smoke.json`; it retains its own historical build identity.

This is direct implementation-test evidence. [Ayanami's actual DeepSeek review](../../review/issue1-gates-review.response.md) independently ran both commands, including race, and returned PASS with no must-fix in the selected scope. Diagnostic scanning covers the reports subtree when present, not an invented whole-filesystem confidentiality proof. The original A25 run is untouched; none of these results claims two-hour coverage for an audit-fix binary.
