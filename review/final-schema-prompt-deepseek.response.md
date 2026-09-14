> 发布副本：仅对本机路径作规范化；原始报告本地保留于 `review/private/`。问题、结论和测试结果未改动。

【D12 冻结增量核证 — 源码快照限定 · Ayanami 复核】

基线锚定：old = /tmp/d12_review/iso1/src（sha256 8d923009…）；new = 工作区 src（sha256 066e6b7a…，与 docs/contracts.schema.json 字节一致）。全程离线只读；脚本/测试仅在 /tmp/d12_verify_ayanami（含隔离副本 iso2）；未委派、未触凭据/真实数据/API/其他进程；未改任何 report；无 429/400 事件（未触真实模型）。

一、Schema $ref 等价（我程序复算，非采信自述）
- $defs 增量：仅新增 RegisteredExtensions；old 无任何缺失 def。
- 新 ref 共 40 处指向 #/$defs/RegisteredExtensions，全部位于 $defs 内；old 0 处。
- 载体（properties.extensions 的 def）名称集合 old/new 完全一致（40 个）；old 内联 shape 精确出现 40 次，new 仅 1 次（即该 def 本体）。
- 规范化比较：把 new 全部 40 处 ref 展开为该 def 深拷贝、删除唯一新增 helper def（展开后残留 ref = 0），与 old 全文档深度比较 → 零差异、零键序差异，结果 EQUAL。保留其他 $refs 语义（仅展开 RegisteredExtensions）。
- 由全文档深相等直接保证（非推测）：extension registry 形态（propertyNames 模式 / additionalProperties={"type":"object"} / security.classification→$ref SecurityClassification）、DataClass enum、不可注入约束的 schema 面、以及完整 output_contract 闭包全部未变。
- 定向测试（隔离副本）：go test ./contract -run 'TestRegisteredExtensionsEquivalentToOriginalInlineShape|TestSchemaClosure' -count=1 → 两项 PASS（等价测试内含非法 class：INVALID/多字段/nil 拒绝 + PERSONAL 接受）。
- 生成链：隔离重跑 generate.py（docs→src）对比：SCHEMA-REGEN-IDENTICAL、DGO-REGEN-IDENTICAL（dto 的 type RegisteredExtensions = map[string]any alias 属生成器既有输出，Go 赋值兼容保持）。
- 差异点（非语义，需澄清基准）：报告 before/after sha256（d52b1004…/5fddcbe5…）与我测的 old 快照（8d923009…）、工作区文件（066e6b7a…）均不吻合；20214/19770/285/504 四个数字未能用文件级方法复现（我测：Go 整文件 marshal 85914→77553，Δ8361 ≈ 40×216 − 279；shape GoMarshal=255、ref=39）。这些字段疑似指"服务端输出闭包（contract.Schema 子集）"的序列化，不在本轮范围。语义等价结论不受影响，建议作者澄清该报告数字基准。

二、roleInstruction 句级对照（旧 model.go:72 → 新 model.go:72，逐条均无删除/反转）
- 禁输出 security.classification（程序所有）→ Never emit program-owned security.classification ✓
- READ_MEMORY 仅探索缺失信息；已够则 controls=[]、不重复同 query/null cursor → 新版同义合并表述 ✓（"继续仅当缺失"折叠进首句，未反转）
- 返回 DecisionEnvelope 实例而非 schema ✓（新版点名）
- 普通回复 reply.evidence=[] ✓（压缩掉"不抄随机哈希"理由句，规则保留）
- world 提案逐字复制 ObjectRef/EvidenceRef、不缩短/不虚构哈希 ✓
- D10：notify.local/alarm.play 仅 FIRE_ONCE_WITHIN_GRACE + grace_seconds=300；once 不等于可跳迟到 ✓
- D10 自定义 late/misfire → 说明需认证 Typed、no actions/controls、不静默替代默认 ✓
- 其他能力保留各自 policy ✓
- reply.questions 仅 text+item_id（既有 Item ID 或 null）、可选、简洁 ✓（"an exact existing"→"existing Context"属压缩，范围不变）
- question ID/序列/resolved 由程序赋值 ✓（新版正面表述）
- answer_to_question_id = 本会话该注册问题的显式回答；不得推断他目标、不得视作答为完成 Item/任务 ✓
结论：原有约束无删除、无反转，纯压缩改写；不宣称自然语言形式化等价。上表即源码句级对照证据。（1869→1137B 为作者数字；本轮脚本该段未执行，字节数未重测。）

三、文档与 CLI 说明
- docs/DataStructure/Common.md 末段（D12 节）："D12 编码等价整理"正式说明存在——40 个相同 extensions 提取为 $defs.RegisteredExtensions、各载体用 $ref、展开后与原 Schema 完整结构相等、只消除重复字节、不改字段/注入拒绝/分类规则/32K/验收 oracle，并引报告与测试名 ✓
- docs/README.md:39-41 D12 修订索引列出修改路径（含 contracts.schema.json 与 DataStructure/Common.md 等）✓（编码去重本身在 Common.md 内定位）
- CLI：src/cmd/secretary/main.go:82 普通帮助 "[--data-class PERSONAL|SYNTHETIC|SENSITIVE|SECRET] (default PERSONAL)"，与代码默认 dataClass=PERSONAL（:188）及 JoinClass 校验（:192）一致 ✓；显式 SYN 示例以现有 smoke 证据为准（reports/implementation/D12-cli-class-smoke.json 与 reports/local/d12-cli-class-smoke/report.json：explicit-item/explicit-input synthetic 均 PASS，命令含 --data-class SYNTHETIC）✓

四、live-question-run4 / live-month-run8
- 两目录及 report.json 存在（只读观察：q4、m8 均 09:32）。本轮未能展开判读完整性 → 标注待后续核实；作者所述"带 grant、原 run3 Context 只读复原 finalwire 31461<=32000、不改原 DB/noAPI"本轮未核证，不冒充真实链。真实 CLI 与月回放另存档验收；冻结 cli e60c16…/daemon 6682c7… 本轮未涉及。

总结论：两处等价增量经独立程序复算，深相等 + 定向测试 + 再生成一致全部成立，无实质语义差异 → 等价重构成立，不需设计再批准、不加门槛。唯一待澄清：作者报告的字节/哈希基准字段（不影响语义结论）。

参考资料：
- /tmp/d12_review/iso1/src（旧快照）；<PROJECT>/src/contract/contracts.schema.json 与 docs/contracts.schema.json（工作区）
- reports/implementation/registered-extensions-equivalence.json；src/contract/extensions_equivalence_test.go；src/contract/contract_test.go（TestSchemaClosure）
- 证据脚本：/tmp/d12_verify_ayanami/verify_schema.py、measure.go；隔离测试副本 /tmp/d12_verify_ayanami/iso2
- docs/DataStructure/Common.md（D12 节）；docs/README.md:39-41；src/cmd/secretary/main.go:82,188-194

思路：以"全文档深相等（展开新增 def、删未引用 helper）"作为唯一够强的语义判据；再用 Go 侧字节测量与生成器重跑交叉验证实现等价；roleInstruction 用句级映射替代自然语言等价主张；报告数字与文件事实不符处如实标注为待澄清而非通过。

(｡•̀ᴗ-)✧ 复核完毕。需要我下一轮补测 DecisionSchema 闭包字节基准或 run4/run8 完整性判读，请指定。
