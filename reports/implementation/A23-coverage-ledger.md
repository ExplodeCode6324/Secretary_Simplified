# A23 combined fixed-set coverage ledger

Created: 2026-09-14T00:12:47.531617+00:00. Ledger SHA256: `bf066de63e449f62e7995905d919d79c1b32a893466c6631f875846abe58a310`.

This is a read-only retrospective index approved as the composition-record option in [the DeepSeek review](../../review/foundation-world-diagnostics-deepseek-final.response.md). It does not modify or regenerate inputs/oracles, does not backdate this document, and is not itself an independent acceptance verdict.

**Index integrity: 129/129 current cases are PASS in their complete selected runs, and 129/129 map to the pre-existing component manifest or saved pre-call fixture.** All 180 month input/oracle file hashes and all three supplemental authored-file hashes match. The authored scope is 30 days × 3 points, four domains × five items, 30 consciousness anchors, seven supplemental scenarios and two world-reading cases.

The evaluation unit uses whole final runs: month-run5 (all 90 points and 30 snapshots), scenarios-run4 (all seven cases), world-read-run2 (both cases). Earlier versions and failures are attached to each case, including month-run1–4, scenarios-run1–3, directed retrieval regression and world-read-run1. No case is selected merely because one attempt happened to pass.

The complete machine-readable mapping is [A23-coverage-ledger.json](A23-coverage-ledger.json). Each entry links the original oracle bytes/hash, coverage clauses, every recorded version/run status, request/response/context artifacts and visibility.

## Freeze evidence and boundaries

- Month: [original manifest](../../src/tests/fixtures/month/manifest.json) binds all authored input/oracle files. This audit verified hashes and linked original archived requests back to authored inputs; it did not read outputs to create expected values.
- Supplemental: the [global manifest](../fixtures/live-scenarios/manifest.json) explicitly says it is a current inventory, not retroactive freeze proof. The actual pre-call artifacts are each run’s saved fixture.json and oracle.json. The harness saves the whole parsed fixture before iteration and each oracle/input before Process/Generate. All reported current cases exactly equal their saved fixture step; all authored fields match the saved current oracle. This is archived workflow evidence, not a trusted timestamp signature.
- This combined mapping is created now. It cannot claim a single cryptographically timestamped combined manifest existed before all calls. Independent review must judge the reviewer-authorized retrospective mapping against the preserved component freeze evidence.
- World mutation is authorized frozen setup; the live model evaluates conflict/retraction reading. No claim of model-generated world mutation authorization is added.
- Consciousness language is sampled only at days 0/14/29. Exact EntityReadRefs are audited for all snapshots; the daily authored state anchors are not fabricated free-text oracles.
- Local-only historical archives are identified by project-relative paths in the JSON and remain unavailable from a public checkout. No runtime DB/config/token/grant or private absolute path was copied.

## Full run history

| Run | Reported failure count | Recorded scope | Visibility |
|---|---:|---:|---|
| month-run1 | 6 | 18 item points | PUBLIC |
| month-run2 | 6 | 56 item points | PUBLIC |
| month-run3 | 6 | 30 item points | PUBLIC |
| month-run4 | 0 | 90 item points | PUBLIC |
| month-run5 | 0 | 90 item points | PUBLIC |
| scenarios-run1 | unknown: no report | 7 cases | LOCAL_ONLY_NOT_PUBLISHED |
| scenarios-run2 | 1 | 7 cases | PUBLIC |
| scenarios-run3 | 0 | 7 cases | LOCAL_ONLY_NOT_PUBLISHED |
| scenarios-run4 | 0 | 7 cases | PUBLIC |
| retrieval-regression-run1 | 0 | 3 cases | LOCAL_ONLY_NOT_PUBLISHED |
| world-read-run1 | 0 | 2 cases | LOCAL_ONLY_NOT_PUBLISHED |
| world-read-run2 | 0 | 2 cases | PUBLIC |

Retained history details:

- scenarios-run1 preserved fixture but has no report/case calls: outcome remains NOT_REACHED_OR_NO_REPORT, never invented as PASS/FAIL.
- scenarios-run2 original retrieval FAIL is retained alongside run3/run4 PASS; run3 and directed regression remain LOCAL_ONLY_NOT_PUBLISHED.
- Scenario oracle serialization variants add world_before/expected_world and later expected_conflict_members as null; full variant hashes remain distinct. Existing fields compare unchanged.
- world-read-run1 had two passing weaker read oracles; world-read-run2 strengthens exact conflict-membership oracle before that run. Both versions are retained, not treated as identical retries.
- month-run4 report identity alias defect is retained with its independent correction; month-run5 original identities match correction. All five runs and all aborted/unreached suffixes remain listed.

## Every fixed case

| Case | Original/current oracle SHA256 | Complete selected run | Status |
|---|---|---|
| month.item.000 | `98d88db12404ffef42751b7366313053875b49ca998ed6502ddd045963c6f3fe` | month-run5 | PASS |
| month.item.001 | `344cbc71ba7af4eb984fb3f25b86df6c1bae2448da024f91e9979d5cdf5994d2` | month-run5 | PASS |
| month.item.002 | `75ad6ea80641843c99151997eb1e29bd68a73a7426d4308d67f26b0643e4a2c6` | month-run5 | PASS |
| month.item.003 | `0674bfd642d10d5c858dd4a5ceac9be268696a1680b62be4aa1ab7f08fa71a73` | month-run5 | PASS |
| month.item.004 | `d5e95dcf622cead56b5f8958dd2cdb5d85021a57eac8724659fb3548f606718f` | month-run5 | PASS |
| month.item.005 | `2fd916ab70c2acd44f81879ca19cdc0a0e92b6c13f645e7f111da7ecc311361a` | month-run5 | PASS |
| month.item.006 | `03a0c3c5042e01ca18ff68c66e908d59818e36ad1ae8e945168ac8b73e2fa55a` | month-run5 | PASS |
| month.item.007 | `658916cefef03ef5aeb64bc428a3e59bd1f73103c13790eb8dc3c7627a62e648` | month-run5 | PASS |
| month.item.008 | `5873a122bfe079f4fdc442b9f9094b41fda9394f2f06a85c468779acafe5f982` | month-run5 | PASS |
| month.item.009 | `2d53f7c07143253edc8e25dc5e2714f5c672436745c81f8eebe57c7bf6207464` | month-run5 | PASS |
| month.item.010 | `5237761405e5bac30d5333614e0358af0a2cdf605885190b4450d0dd39da2347` | month-run5 | PASS |
| month.item.011 | `48cfeac3f6d6c28dca90e68b77afd224ad32b9e18d4747bba757004023fd74e4` | month-run5 | PASS |
| month.item.012 | `829bb33e9e9fcfee1ae4c76a478cbc03e9c1fca93ec187d1858e352f451cf186` | month-run5 | PASS |
| month.item.013 | `f8c31188c9c8105411de1807a27298b48c62ddc5cfae477dc6c499fa48d3da31` | month-run5 | PASS |
| month.item.014 | `c88542e6fae282347baa4a74eb9d5ef8e1c24612642a39494a0f6596a1390886` | month-run5 | PASS |
| month.item.015 | `7fc27b940d0a9bf35df3bcde2f158c06e32d34f22bc14424bee26770624386ab` | month-run5 | PASS |
| month.item.016 | `48b245579cc16e5fe020ae17d2bb45fcd8ca9aa9f88f924868876928b666f7ad` | month-run5 | PASS |
| month.item.017 | `bd80a95edf230e2bf30d8f26121980a9cc69667e8a83248ba114fe7251c2a0e0` | month-run5 | PASS |
| month.item.018 | `d3810531306d91efa2ca6746c198d6fe0dca9aca9f00f38a455e986823556fba` | month-run5 | PASS |
| month.item.019 | `61ec7661d47c9dcce0944d2afc349c63e9d3a02441eaace9b15efe84198d96b3` | month-run5 | PASS |
| month.item.020 | `5c4a7fff2ee8645ce03b371f42c38d61366fa266ea14484418d6e208ff4155a8` | month-run5 | PASS |
| month.item.021 | `7e713f31fd7ec5624319bf4a2f37c4dd951a7714b503d7776ce756ed239e9ef6` | month-run5 | PASS |
| month.item.022 | `1987ac759c467d3a8634331fd46f5998fb32419e93548cbc6ac48b791b328188` | month-run5 | PASS |
| month.item.023 | `6b5a2399496f592aa5b819e1cfd789702f86b4c5904cad4719bbd6adb1305b92` | month-run5 | PASS |
| month.item.024 | `98d9d52ebc36d8784336dca690f36d01c58f7f0530db9129abd3f61e58080408` | month-run5 | PASS |
| month.item.025 | `1b906b3d1ae964975f04ed2e3749ea2385aee309de662dfcc3790de05ae0cbf1` | month-run5 | PASS |
| month.item.026 | `179093d43eb55f5691849abf19738eb6aac9ef2bdca982e9cd49335263f92149` | month-run5 | PASS |
| month.item.027 | `38664a38dfb605d9c9efde9836c6a276dc29f2512a53ecb1abb316239b210aaf` | month-run5 | PASS |
| month.item.028 | `8e61b57290b43ee777a00c1b799b399552b85c1d8d10adcf15f51b9613387583` | month-run5 | PASS |
| month.item.029 | `933bc554a71345050cbe907190b951701842e36d4d8a260dd986871d12b65fed` | month-run5 | PASS |
| month.item.030 | `fd19a30af2b4d219e8baaea9eff6606dacfd9074772a3feaef030ebc396fef2e` | month-run5 | PASS |
| month.item.031 | `59fc96fff75a16ed795bbf0e1bccfca00331f064e8d4130aa069b5e7b43e762a` | month-run5 | PASS |
| month.item.032 | `7924d8787aa1d68b983a723e97baacb4f85d3f5215993d0b12c3133af91cca8f` | month-run5 | PASS |
| month.item.033 | `8b077bf43b75038828ab3d5301676ef94db23284678b77d8afaf1495e5324891` | month-run5 | PASS |
| month.item.034 | `20612fd7a46300c76142cbbb9c6019b351e133ec0106bb89a640b40bdda9f040` | month-run5 | PASS |
| month.item.035 | `ee1ce13f19f2d00b13feccb378aa0532934002828a8f9071d9f7bd73cba6a609` | month-run5 | PASS |
| month.item.036 | `9b710a457dc319e59814e8946acad635dfb8f75abbbf514978571ff3255aedac` | month-run5 | PASS |
| month.item.037 | `fffda392a03c46c0788e31bcb1ec225818c618706867fb07d03a9de4aacfa3e0` | month-run5 | PASS |
| month.item.038 | `7be2fb0729b0cbedcf58f4480e708bb79feb9a3011b004ad2ce024b0c6cee6f7` | month-run5 | PASS |
| month.item.039 | `91cdb92c574e82060fed26d171dcd7e9080e12a689d37fd45308b6c61a01e627` | month-run5 | PASS |
| month.item.040 | `c23764951eef80c432a96792acec53ab2926395c2753fdc36d962b94a974d0a8` | month-run5 | PASS |
| month.item.041 | `608d18e9437038e41455ff60be249aa9072eb10d9b20688ce3a98182c7331ae1` | month-run5 | PASS |
| month.item.042 | `1738d3f8dd21b341ae050977d2e365c4706e355105256eca72bf14f20ba3755c` | month-run5 | PASS |
| month.item.043 | `451fe30293ed9e5c3a4f3cb87214e97c9fda9bdfbadb6a81e8014f92400d3da0` | month-run5 | PASS |
| month.item.044 | `f8744b67b1ba25e654b73086a2e53bf9f5f07d7b38ba46262a94631d2a7037be` | month-run5 | PASS |
| month.item.045 | `4fb5c8c9fbd4f1dc4a47eb46201512285e8c25d7eae929f28bd76c0c80f3db3b` | month-run5 | PASS |
| month.item.046 | `12ebdd2a53f1611d7a53750fcc5acb4c2cd1328a20e58108bd6d648dd7d2971b` | month-run5 | PASS |
| month.item.047 | `4b0ec1a9ced8d6f3adba1a0063beb1a409d4a921c5c59ab37f840c072ba351aa` | month-run5 | PASS |
| month.item.048 | `330132a3f0bd16c7e150a9576d01bb19f9ae161d854894d2729b211cc441bf29` | month-run5 | PASS |
| month.item.049 | `3a99ff844e21412b22afae3db4ca4eae4f3cda408cf6dcf23501cf99ffc09487` | month-run5 | PASS |
| month.item.050 | `2cd9a901e51e2a443e50da4cf427dde5d9d8f894274c095fe4183eb6850a569b` | month-run5 | PASS |
| month.item.051 | `9a9f1b329fc3ccf200fc52dbac1ba451a1ef4e6705dc808462cabb0f20f7c3ed` | month-run5 | PASS |
| month.item.052 | `c999bae42c5b3cf55771dbc68a70690bc25cf8e812374154cae1ba9f277e959f` | month-run5 | PASS |
| month.item.053 | `4ce6b6c6ccfbf462df5ed36b11ed9bff5d91c7d1349894da83d169d661978aa7` | month-run5 | PASS |
| month.item.054 | `37615f7211d340a412aca139822f83629f9371c001dc5319353f9af76f3d0597` | month-run5 | PASS |
| month.item.055 | `d795873d0debd305638a970a25deb2dfc1a56ba237ca3612426ee5a35620e137` | month-run5 | PASS |
| month.item.056 | `10a5144002eb07ba93b4a995c5aa65cbd9c0b3c441f717fcfba610c16962c034` | month-run5 | PASS |
| month.item.057 | `36b8f8167d735c177b04c03211ddbca92734a1d6eaf9822dd333f2cdc8d59212` | month-run5 | PASS |
| month.item.058 | `e34b011d161c4614c4860f680148ee3be0dcd8a2767b898193dda5e7d8c68740` | month-run5 | PASS |
| month.item.059 | `ef28beef1651cc9f55f01a5fff89472787ff13e182856e92a5152efb22a6c10e` | month-run5 | PASS |
| month.item.060 | `1a2ea59dbcb79673a4bb361422c7e791bd2451ee92aa5b18d1260c5eca60606f` | month-run5 | PASS |
| month.item.061 | `9c06d3fcbc6f554211890f84c40d06050138ddc566b9cae43d2e53f0371cdfcc` | month-run5 | PASS |
| month.item.062 | `b8e3016a541efc5c685da28bd7aeda7d303256bffd07867dde95c5f7c7c1e948` | month-run5 | PASS |
| month.item.063 | `53cd25850b24bf61fc817c92b023ed4e6e32cfce7c73c584db372f55c328b81a` | month-run5 | PASS |
| month.item.064 | `705ebc142918a221950fecd35d1e4c2beb2580fffb74ac483bf3cfa538e9a599` | month-run5 | PASS |
| month.item.065 | `3cc95e72ca4841f41b0b20eb86f163be3cf52d1d58ebf8b26a7f77c2e7f1a2a3` | month-run5 | PASS |
| month.item.066 | `5a607c894ab815fec2b4481633e251d7800b5a593974c1d3676e67636685d46e` | month-run5 | PASS |
| month.item.067 | `92de9d4dc7ee7ce8493d55a0899a3da415b4557db4904cf3fd3205f5273efa0e` | month-run5 | PASS |
| month.item.068 | `ca339278b24237fb77f017c6123798d7941e581ca67dfe4145626f6aa6493814` | month-run5 | PASS |
| month.item.069 | `84923f5e1b798d52d6e173ecda8de4416c58f70541ee31d1fc25eeba1a9b35c6` | month-run5 | PASS |
| month.item.070 | `f3e807711e45e94d2e15bd14f15a65418e71ad33bdc98a8ec0c3c513b4383093` | month-run5 | PASS |
| month.item.071 | `6829a5dd60be28d665f01e3dd632a2379b575bb30ef54660357112a60614acdb` | month-run5 | PASS |
| month.item.072 | `e5aa63b1cd769b4d30ba31c2f39e4cc7b83977102a7b1056dad231dbb5efa48b` | month-run5 | PASS |
| month.item.073 | `224daec37e9f47e9fd82a4dba9cc6d17b6f6c3ecf5c4647e594598a6a5c5347f` | month-run5 | PASS |
| month.item.074 | `5517003e296eb2ed90229112ebd93572a1e5be3b0dae8a2a7cddc3b19f88f8f3` | month-run5 | PASS |
| month.item.075 | `2f624e127bc8afc221170da8ef0a11750b126fe290fdf6361281cc8a6ece031f` | month-run5 | PASS |
| month.item.076 | `195b69312dec0909503c58a26293e8975c0033c4c4d9f358d6f37f05124119ce` | month-run5 | PASS |
| month.item.077 | `2640f722600ccfbf4509fd110469d2ba2841bcb614e6e21b014a583389c68ae0` | month-run5 | PASS |
| month.item.078 | `a61615fdb3f1e26883be21bb9625a1d46edb5b6d12e05e1c8d188e4a61136f9a` | month-run5 | PASS |
| month.item.079 | `bd9b8d79425fc560f72976427e31deebb76323e3263c01f09a382d97678f0d17` | month-run5 | PASS |
| month.item.080 | `190b6c6565586abaaf09a6b4ce114350256cfad347ed485be700d59968dc0afe` | month-run5 | PASS |
| month.item.081 | `77df9744a015f67b094719ce375c996bd88598ab137d9976dee49efdd2a29a20` | month-run5 | PASS |
| month.item.082 | `cab2903a7637f4f4780d8dc17cc345c2ecd384472bc06f434dcdb52262d1ce8a` | month-run5 | PASS |
| month.item.083 | `5d66e9aa1d2f1f8f7fa7efd37b8de34b9d063233b85b1d16256a828c13ebbf73` | month-run5 | PASS |
| month.item.084 | `7fb0120bc1325c0f6b9b0769d11bdc9b461b93ad576dc32511aa98d453fcfad8` | month-run5 | PASS |
| month.item.085 | `de9f7afa2c71c7395699c6c0fb78ac7b258898c917be20de44689842cf0b9baa` | month-run5 | PASS |
| month.item.086 | `2406ec646e496d80154a4c7eb2ed62025acee9d39c502bab46499f539297c36f` | month-run5 | PASS |
| month.item.087 | `30eaea69af74377f261de157be89a0b8b05794a8ec8f83eb6cbb327bb52c434e` | month-run5 | PASS |
| month.item.088 | `218d47a26214d43abdfd1c06a0a6051cd9776c17c2366e19273e435fca389d1b` | month-run5 | PASS |
| month.item.089 | `b8e42a29ffc1f52fec738daaa6ed4df0e0f0e673802981050b201a21a19f12bf` | month-run5 | PASS |
| month.consciousness.00 | `75ad6ea80641843c99151997eb1e29bd68a73a7426d4308d67f26b0643e4a2c6` | month-run5 | PASS |
| month.consciousness.01 | `2fd916ab70c2acd44f81879ca19cdc0a0e92b6c13f645e7f111da7ecc311361a` | month-run5 | PASS |
| month.consciousness.02 | `5873a122bfe079f4fdc442b9f9094b41fda9394f2f06a85c468779acafe5f982` | month-run5 | PASS |
| month.consciousness.03 | `48cfeac3f6d6c28dca90e68b77afd224ad32b9e18d4747bba757004023fd74e4` | month-run5 | PASS |
| month.consciousness.04 | `c88542e6fae282347baa4a74eb9d5ef8e1c24612642a39494a0f6596a1390886` | month-run5 | PASS |
| month.consciousness.05 | `bd80a95edf230e2bf30d8f26121980a9cc69667e8a83248ba114fe7251c2a0e0` | month-run5 | PASS |
| month.consciousness.06 | `5c4a7fff2ee8645ce03b371f42c38d61366fa266ea14484418d6e208ff4155a8` | month-run5 | PASS |
| month.consciousness.07 | `6b5a2399496f592aa5b819e1cfd789702f86b4c5904cad4719bbd6adb1305b92` | month-run5 | PASS |
| month.consciousness.08 | `179093d43eb55f5691849abf19738eb6aac9ef2bdca982e9cd49335263f92149` | month-run5 | PASS |
| month.consciousness.09 | `933bc554a71345050cbe907190b951701842e36d4d8a260dd986871d12b65fed` | month-run5 | PASS |
| month.consciousness.10 | `7924d8787aa1d68b983a723e97baacb4f85d3f5215993d0b12c3133af91cca8f` | month-run5 | PASS |
| month.consciousness.11 | `ee1ce13f19f2d00b13feccb378aa0532934002828a8f9071d9f7bd73cba6a609` | month-run5 | PASS |
| month.consciousness.12 | `7be2fb0729b0cbedcf58f4480e708bb79feb9a3011b004ad2ce024b0c6cee6f7` | month-run5 | PASS |
| month.consciousness.13 | `608d18e9437038e41455ff60be249aa9072eb10d9b20688ce3a98182c7331ae1` | month-run5 | PASS |
| month.consciousness.14 | `f8744b67b1ba25e654b73086a2e53bf9f5f07d7b38ba46262a94631d2a7037be` | month-run5 | PASS |
| month.consciousness.15 | `4b0ec1a9ced8d6f3adba1a0063beb1a409d4a921c5c59ab37f840c072ba351aa` | month-run5 | PASS |
| month.consciousness.16 | `2cd9a901e51e2a443e50da4cf427dde5d9d8f894274c095fe4183eb6850a569b` | month-run5 | PASS |
| month.consciousness.17 | `4ce6b6c6ccfbf462df5ed36b11ed9bff5d91c7d1349894da83d169d661978aa7` | month-run5 | PASS |
| month.consciousness.18 | `10a5144002eb07ba93b4a995c5aa65cbd9c0b3c441f717fcfba610c16962c034` | month-run5 | PASS |
| month.consciousness.19 | `ef28beef1651cc9f55f01a5fff89472787ff13e182856e92a5152efb22a6c10e` | month-run5 | PASS |
| month.consciousness.20 | `b8e3016a541efc5c685da28bd7aeda7d303256bffd07867dde95c5f7c7c1e948` | month-run5 | PASS |
| month.consciousness.21 | `3cc95e72ca4841f41b0b20eb86f163be3cf52d1d58ebf8b26a7f77c2e7f1a2a3` | month-run5 | PASS |
| month.consciousness.22 | `ca339278b24237fb77f017c6123798d7941e581ca67dfe4145626f6aa6493814` | month-run5 | PASS |
| month.consciousness.23 | `6829a5dd60be28d665f01e3dd632a2379b575bb30ef54660357112a60614acdb` | month-run5 | PASS |
| month.consciousness.24 | `5517003e296eb2ed90229112ebd93572a1e5be3b0dae8a2a7cddc3b19f88f8f3` | month-run5 | PASS |
| month.consciousness.25 | `2640f722600ccfbf4509fd110469d2ba2841bcb614e6e21b014a583389c68ae0` | month-run5 | PASS |
| month.consciousness.26 | `190b6c6565586abaaf09a6b4ce114350256cfad347ed485be700d59968dc0afe` | month-run5 | PASS |
| month.consciousness.27 | `5d66e9aa1d2f1f8f7fa7efd37b8de34b9d063233b85b1d16256a828c13ebbf73` | month-run5 | PASS |
| month.consciousness.28 | `2406ec646e496d80154a4c7eb2ed62025acee9d39c502bab46499f539297c36f` | month-run5 | PASS |
| month.consciousness.29 | `b8e42a29ffc1f52fec738daaa6ed4df0e0f0e673802981050b201a21a19f12bf` | month-run5 | PASS |
| supplement.exact-id-conflict | `2abb65e95f1c2f22fb268976d935d691a28338b1ac5f9729f6218e8076b6dbe3` | scenarios-run4 | PASS |
| supplement.dependency-completion | `8a236429b5929a4995f3953e37d23ace58f1aa042d5ae6a2acd922c9029bc45e` | scenarios-run4 | PASS |
| supplement.dependency-unlock | `fa46adf5b4a83a94ff7268202958f8a29267df667007c550cfc9dcc82beb0b2b` | scenarios-run4 | PASS |
| supplement.remember-agreement | `8ac7c094f0ae9b82e2ad96c2171bf562a4cf26ce6f1aa50ed1cdaf20e897eb1e` | scenarios-run4 | PASS |
| supplement.withdraw-agreement | `58aa0e24a393e78d49f452dd5c21389d01a33b551f9190aacb19a43159af52ba` | scenarios-run4 | PASS |
| supplement.prior-session-retrieval | `f865cec43447d268a602b4a8b895a05efc58dc1ff4c9f61b931a280297c08f28` | scenarios-run4 | PASS |
| supplement.stale-source | `dc09aff1114503780835d0c3a8e1cbaa8bf08709a05cce39b0fed678b7a688b2` | scenarios-run4 | PASS |
| supplement.world-conflict | `9aaba191cc68d751bcacb3b50f2c9773a6e4ceefb1c51c0757242b286a970981` | world-read-run2 | PASS |
| supplement.world-retraction | `3e21f60e951f96479725ed3b0ebb9b0dd830e9327fcace5d0a78c6ae212aba63` | world-read-run2 | PASS |

All distinct serialized oracle versions and call failures remain in the JSON; this table shows the original month or current complete supplemental oracle solely for compact lookup. Presence of an oracle hash is not a claim that differing versions are identical.

Audit result: index integrity passed; independent A23 acceptance review remains separate.
