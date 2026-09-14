# TUI-12 supplemental panic and process-cleanup evidence

This supplement preserves `issue2-tui.json` as the exact earlier test/source snapshot. Only the new child-only test helper and its PTY script were added after that snapshot; production code and the tested release binary hashes did not change.

`TestIssue2TUIPanicPTYHelper` wraps the actual `tui.Model` and uses the same Bubble Tea options as production `Run`. Only its test-only Update wrapper injects `synthetic-issue2-catchable-panic` on Ctrl+X. The production CLI has no injection flag or hook. The independent helper binary was compiled with `go test -c` (no suite execution), then exercised through macOS PTY. Bubble Tea's default panic recovery returned an error and restored every termios field except the already approved transient PENDIN state bit. This supplements the actual release Ctrl+C and SIGTERM tests; it is not falsely labeled a panic injected into the release binary.

Commands:

- `cd src && go test -c -o /tmp/issue2-tui-testhelper ./cli/tui`
- `python3 scripts/issue2_tui_panic_pty.py --helper /tmp/issue2-tui-testhelper --report reports/implementation/issue2-tui-panic-pty-run2`: PASS.
- `cd src && go test -race ./cli/tui ./cmd/secretary -run TestIssue2 -count=1`: PASS, tui 1.462 s / CLI 1.580 s. The same 13 normal Issue2 tests executed; the child-only helper is skipped unless its dedicated environment marker is set by the PTY script.

The first attempt in `issue2-tui-panic-pty/report.json` remains FAIL. Its harness sent the injection after a fixed startup delay and then waited without draining the PTY; readiness/panic were not established and it timed out. The second attempt waits for actual rendered readiness and drains PTY output while awaiting exit. Neither production logic nor the terminal-restoration assertion was changed. The original helper stack output is retained privately under `reports/local`; the published transcript explicitly normalizes workspace/home paths and does not claim byte-exact equivalence. Its original-byte SHA-256 is recorded for comparison.

SIGHUP was not tested and is not claimed as supported recovery. The actual release catches Ctrl+C/SIGTERM; the isolated component probe covers a synthetic recoverable panic. This is the bounded TUI-12 scope, not a new platform-wide signal audit.

All owned PTY clients, fake HTTP servers, actual fixture Core/Runner processes and helper processes exited. A process inventory found zero remaining owned processes; see `issue2-tui-cleanup.json`. No old acceptance or soak service was started, stopped or reused.
