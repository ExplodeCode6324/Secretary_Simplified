# D11 output classification propagation probe

Isolated temporary SQLite database; fake model only; invented canary bytes labeled PERSONAL, no actual personal data and no network call. Production files unchanged. Probe source: D11-output-class-probe.go.txt (copy into src/tests/output_class_probe_test.go to reproduce; remove afterward).

Command: `cd src && go test -race ./tests -run '^TestProbeOutputClassInheritance$' -count=1 -v`

Observed:

```
creation_effective_class=PERSONAL
creation_input_class=SYNTHETIC
retrieved_classes=[SYNTHETIC]
retrieved_role=ASSISTANT
new_request_class=SYNTHETIC
synthetic_only_encode=PASS
canary_in_wire=true
ok secretarysimplified/tests 2.750s
```

Probe PASS means the suspected information-flow defect was reproduced, not a security acceptance PASS. The original PERSONAL input lacks the unique query token; only the later ASSISTANT question matches. Its originating model request was effectively PERSONAL, but SearchMemoryPage inherits its generating turn's input.data_class=SYNTHETIC. A fresh session allowing only SYNTHETIC then builds and encodes that generated text, including the invented canary, as SYNTHETIC. Current full-session input classification correctly protects the original session; it does not carry the output taint across this retrieval boundary.
