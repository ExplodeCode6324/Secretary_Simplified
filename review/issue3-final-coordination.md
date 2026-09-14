# Issue #3 final evidence boundary (Codex coordination)

No open review mustfix. Actual Ayanami DeepSeek reviewed the local R1/R2 contract, implementation, two delta fixes and final one-line defensive fallback. The reviewer personally ran 29-case and 35-case revisions; the final 37-case result was run by runtime and checked by Ayanami statically against source and log hashes. The repeated-reset helper is a defensive boundary, not a claim that an ordinary second `r` key necessarily reproduced the fault.

DOC-R final review independently read the four affected small diffs and ran the Issue3 checker successfully (2316 records / 111 checks at that snapshot). Two earlier compound verification commands were rejected by Hermes automatic approval parsing and did not execute; the later allowed simple checker call succeeded. No approval was bypassed.

The DOC review's build/archive observations bind its earlier `after-final` snapshot (CLI `59d93dd7…`, zip `5cfbc3ff…`, 35 cases), not the last defensive fallback build. The final CLI `0c0b3f2c90283904595b9f0e946f160ea851f8b996900db961714fc4835b31d5`, archive `e158d49d12cec638077447a20d1cb289f29ac0e6f39ed70c26e6c398e527abb4` and 37-case snapshot are checked by root's actual [final-validation.json](../reports/implementation/issue3/final-validation.json). This is coordinator verification, not a claim that Hermes reran those final checks. Old reports retain their original findings and hashes.

Temporary reviewer evidence is preserved under [issue3-evidence](issue3-evidence/manifest.json). Further review/provenance additions only enter the evidence inventory; no normative document changes and no recursive re-review. Old 13/27, Context/Store, whole-repository, monthly and sustained suites were not rerun.
