# Issue #2 TUI and CLI acceptance

This report covers TUI-01—TUI-13 and their shared AUTH/BUILD paths only. Every execution below used synthetic data and local fixture/fake services; no real model API, old baseline suite, month replay or A25 run was executed. Backend AUTH evidence is separately recorded in `issue2-auth.md` / `.json`.

## Implementation and interface

`src/cli/tui` is a thin Bubble Tea terminal client. Its event loop owns draft/selection/scroll state; bounded asynchronous commands query authenticated Core and Runner Unix HTTP. It neither opens SQLite nor calls a CLI subprocess or model. All external text is sanitized before rendering; Unicode cell wrapping uses the locked ANSI helper. Core connectivity is inferred from the lightweight authoritative conversation query, avoiding periodic calls to the old diagnostic/integrity endpoint.

Normal TTY default startup, `chat` and `tui` enter the same implementation. Enter inserts a newline; Ctrl+S or Alt+Enter submits. Bracketed paste only changes the draft, even if it contains a submit control byte. PgUp/PgDn scroll; Ctrl+P/N recall in-memory drafts; F1 help, F2 structured questions, F3 tasks, F4 jobs, F5 items, F6 notifications; Tab opens questions or returns from a panel. Panel arrows select, Enter selects an answer or prepares a precisely identified control, y/n confirms/cancels the control, n fetches the next page, r refreshes, Esc returns. Ctrl+D expands diagnostic IDs; `/status` includes pending request IDs. Ctrl+C and `/quit` exit only the client.

Slash commands: `/help`, `/status`, `/history` (older page), `/items`, `/jobs`, `/tasks`, `/notifications`, `/answer`, `/reconnect`, `/quit`. There is no session picker or `/new`. Undoing accepted turns and aborting model generation are not implemented. `chat --plain` reads lines until EOF, prints acceptance receipts and does not impersonate task completion. `--json` remains machine-readable; a TTY stdin with JSON mode or redirected stdout returns guidance instead of waiting for keyboard input. Administrative `migrate --authority-session` is separate from the TUI and calls the backend upgrade API with its daemon-lock checks.

Core API: GET `/v1/conversation`; initial/older history GET `/v1/conversation/history?direction=backward&before_sequence=…&limit=50`; incremental `after_sequence=…`; GET `/v1/requests/{request_id}` then the returned `turn_id` through GET `/v1/turns/{turn_id}`; POST `/v1/inputs`; GET `/v1/items`. Structured questions come from `state.pending_questions`, never parsed from assistant text. Runner API: GET health/tasks/jobs/notifications; POST task cancel, job pause/resume and notification ack. Controls capture target ID/revision/request ID before confirmation. The TUI never auto-acks on refresh.

Before submitting, a 0600 atomic file is persisted in `data_dir/run/tui-pending`: only `instance_id` and `request_id`, no body, credentials or independent history. Each request uses a separate file, so clients do not overwrite a shared index. Recovery filters by instance and only queries the original request, without an automatic POST or new ID. Unknown receipt state remains visible; definite client rejection restores the unsent text when possible. Draft and received history are memory-only. An instance/session identity change is rejected instead of blending views. The client caps unresolved IDs and draft recall at 100.

## Executed evidence

- `cd src && go test -race ./cli/tui ./cmd/secretary -run TestIssue2 -count=1`: PASS; 13 selected tests, tui 2.177 s, CLI 2.274 s. Includes an injected 20 ms call deadline, observed for 60 ms while editing, with the same unresolved request retained. No old test selectors ran.
- `cd src && go vet ./cli/tui ./cmd/secretary`: PASS.
- `python3 scripts/issue2_tui_pty.py --binary release/secretary --report reports/implementation/issue2-tui-pty-release`: PASS, 11 checks on macOS PTY. Real terminal IO plus authenticated fake Unix HTTP; three Chinese turns, bracketed multiline paste, resize, draft editing during unresolved work, explicit older-page fetch, dropped response/reconnect without new POST, structured question selection, Core-offline Runner control, explicit notification ack, OSC sanitization, Ctrl+C/SIGTERM restoration and non-TTY/JSON compatibility.
- `python3 scripts/issue2_tui_daemon_bridge.py --cli release/secretary --daemon release/secretaryd --report reports/implementation/issue2-tui-daemon-release`: PASS. Actual fixture CLI/Core/Runner: after TUI exit, one short notification fires; one owned Core process is terminated/restarted; authority IDs, original history and shared summary/focus/questions remain unchanged; reconnect reads the existing notification without retriggering. Empty question state in this particular process fixture is supplemented by the nonempty structured-question AUTH fixtures, not claimed as a separate nonempty-question process restart test.
- Backend TUI-13: `TestIssue2AUTHTUI13PublicEntryDisclosureAndOmittedSession` in `issue2-auth.json` proves actual public-input PERSONAL/default and SECRET synthetic canaries result in zero fake model calls and persistent rejection. TUI tests separately prove classes are transmitted without downgrading and private draft text is absent from recovery metadata.

The release CLI hash is `8e1f42206a9438e4018afbeb60421a568f6aea6349342daa2e497a5d17a17e0f`; daemon hash is `e3756a1a4219149dd0228e0252be95b05c9530b377ca61a0e28ee27e600bbb75`. Reports bind the actual binaries, not an assumed source revision.

| Requirement | Evidence |
|---|---|
| TUI-01 | CLI missing-config/no-init test, default startup PTY, Core-offline PTY |
| TUI-02 | Three Chinese turns in release PTY; original ordered/ID-deduplicated history unit test |
| TUI-03 | Bracketed paste including Ctrl+S byte, resize, long Chinese wrapping tests and PTY |
| TUI-04 | Injected short timeout state-machine test; PTY unresolved response with continued draft and scroll |
| TUI-05 | Release PTY original-request recovery/older-page retrieval; real daemon bridge authoritative reconnect; instance-isolation test |
| TUI-06 | Structured question selection in release PTY; backend AUTH-03 competition evidence |
| TUI-07 | Release PTY accepted response loss/reconnect makes no repeated POST; recovery metadata test |
| TUI-08 | Release real-daemon bridge: one notification after exit, no retrigger after reconnect |
| TUI-09 | Release PTY task cancel with Core offline; job pause/resume revision snapshot and conflict test |
| TUI-10 | Explicit-ack PTY, rejection/unknown state tests; no model text interpreted as execution success |
| TUI-11 | CLI boolean flag/default/pipe tests and release PTY JSON-on-TTY guard/pipe EOF |
| TUI-12 | External control-sequence test, release PTY OSC rejection and Ctrl+C/SIGTERM terminal restoration |
| TUI-13 | Client class/no-private-archive test plus actual backend zero-call test in AUTH evidence |

## Retained failures and limits

`issue2-tui-pty-run1` and `run2` retain actual early FAIL reports. Their terminal probe compared every termios bit and found only macOS `PENDIN` (kernel transient “retype pending input (state)”) changed after raw→canonical restoration. Ayanami inspected the installed header and approved masking precisely that state bit in `review/issue2-pty-pendin.response.md`; every other termios field remains an exact comparison. No production restoration behavior was weakened. `run2` retains its raw transcript; `run1` did not archive a transcript before the assertion, and one is not fabricated now. Runs 3—5 and the earlier daemon bridge keep their original intermediate binary hashes; final release evidence uses the paths above.

The PTY backend in the main interaction script is deliberately synthetic and does not prove backend authorization by itself. Backend proof is cited separately. Supported platform evidence is macOS only; the Linux terminal helper compiles by design but was not runtime-tested. No mobile networking, per-token streaming, new source integration, real audio, new model capability or arbitrary shell was added. Genuine implementation review is separate from the design approvals and tracked by the coordinating agent.
