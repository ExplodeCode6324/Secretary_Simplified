> 发布副本：仅对本机路径作规范化；原始报告本地保留于 `review/private/`。问题、结论和测试结果未改动。

( _ _ ) 定向复核完成。以下为报告正文（stdout，供调用方归档；本轮仅只读 + 临时目录测试，未写入仓库任何文件，未触碰 soak、生产代码、其他进程，未读取 resource/ 或打印任何 key）。

════════ 独立定向复核报告：GitHub issue #1 GATE-01 / GATE-02 ════════

【判定】限定范围 PASS，无 mustfix。

PASS 范围为「所选接入分支」：GATE-01 选分支 1（仅受控 PERSONAL 文字试运行），GATE-02 选最小方案（独立数据目录隔离）。以下行为经我本人实跑与代码追踪独立确证，不以作者 PASS 代审。超出该分支的承诺（真实 source.sync、同库混合权限可用、真实服务商/真实资料、长时运行）均不在本轮范围，测试也未声称。

────────────────────────────────
一、我实际执行的动态证据（非引用作者结论）

环境：macOS arm64，go1.25.6 darwin/arm64，本会话实跑（2026-09-14）。两条命令与实施报告一致：

  cd src
  go test ./tests -run TestIssue1GatePersonalAuthorizationAndDirectoryIsolation -count=1 -v
    → PASS：父测试 + unapproved (0.01s) + explicitly_approved (0.06s) 全过；ok 1.043s

  go test -race ./tests -run 'TestIssue1GatePersonalAuthorizationAndDirectoryIsolation|TestClassificationTypedAndHTTPDefaults' -count=1 -v
    → PASS（含 -race）：TestClassificationTypedAndHTTPDefaults 0.27s；Gate 父 0.58s + 两子用例；ok 2.215s，无 WARNING: DATA RACE

测试全程使用虚构 canary、临时目录（t.TempDir）、fake HTTP transport；未调用真实 Secretary API，无真实资料，未读取任何真实 key。

────────────────────────────────
二、逐条声称 → 实证映射（含生产代码锚点）

1) 省略 data_class 经公共 HTTP 默认持久化 PERSONAL
   证据：src/core/typed_http.go:118-121 `declaredClass`，raw 为空返回 "PERSONAL"；测试删除 body 的 data_class（issue1_gates_test.go:83）后从 SQLite 读回 turn 断言 == "PERSONAL"（:90-99）。两条子用例均执行该断言。→ 成立

2) 未授权时 provider HTTP 调用为零
   证据：config.Default 默认 AllowedClasses=["SYNTHETIC"]（src/config/config.go:47）；model.Encode 在 !Allows 时先行返回 DISCLOSURE_DENIED（src/model/model.go:55-57），网络仅在 p.HTTP.Do 一处发出（model.go:166），被 fake transport 计数拦截。测试 unapproved 子用例同时断言 calls==0 且 reply 含 DISCLOSURE_DENIED（:112-114）。→ 成立（零调用 + 明确拒绝双断言）

3) 显式授权后同类文字成功调用一次并提交回复
   证据：仅临时配置显式追加 PERSONAL（:36-38）；测试断言 calls==1 且回复经真实 model.Fixture 往返含 "Fixture profile"（:108-111）。授权只存在于内存临时配置，仓库配置未变（config.go 未被本 issue 修改，见第三节 git 状态）。→ 成立

4) SECRET 不入模型请求 / 独立目录隔离
   证据：SECRET canary 写入另一 store 与另一目录（:23-29），并显式断言两 DataDir 不重叠（:33-35）；fake transport 对真实 wire 做双向断言：SECRET canary 必须不存在、PERSONAL 正控制必须存在（:52-57），正控制通过说明断言作用于真实载荷而非空 body。→ 成立（wire 级证据，非间接推断）

5) 同库混放保守拒绝（不承诺混合权限可用）
   证据：授权用例成功后向同一 store 注入 SECRET（:117-119），再提交 PERSONAL 输入 → 回复 DISCLOSURE_DENIED 且 calls 仍为 1（:133-134），即未发生第二次外发；与 src/context/context.go:30-39「Build 先全量 Snapshot 再 CheckDisclosure，命中即返回，不做相关投影」的实现一致。→ 成立

6) PERSONAL/SECRET canary 不写入模型诊断原文归档
   证据：遍历 c.DataDir/reports 子树双 canary 二进制检索（:138-158）。→ 成立（有界，见第四节注 1）

测试质量附加核查：使用真实 Core.Service、真实 HTTP ingress Handler、真实 SQLite（临时库）、真实 provider Encode/协议组包，仅 HTTP 传输层假；负控制与正控制并存；-race 通过，calls 计数无竞态报告。未发现测试仅覆盖 stub 或断言可被绕过（例如正控制的存在排除了「body 为空导致 canary 必然不出现」的假通过）。

────────────────────────────────
三、关键文件 sha256（本轮现场取样）

  943b610c7b632518d890e76818b41be081a25ca3fed0700d1675721a45687202  README.md
  abff1ba172b0d65b473b1b036a7dc5da11e846cc48389de84e39db84dad46c75  docs/RealDataTrial.md
  5783f04c14f850ed69278023d5f8c6bb463496dcf430ec26dd2b88b104e2a539  docs/Operations.md
  f18b2868b612eda05a81dc201aeeb8eca582816e1424b3675d0b00deba3883d1  release/README.md
  6330aeb62381d552b4ce77cf1558336fbef506ad43554401f6a7344a90dc4fb8  src/tests/issue1_gates_test.go
  3308161fc0a272dc430163434ed55ad05b1ef500cfd64c1fa0d33fe894bc5c38  src/tests/issue1_runtime_test.go
  4f5ceafe9d208996fa50133b8f592318b8734b866981088074b2cfd5cf7962d0  reports/implementation/issue1-gates.md
  1662ae3d3d0a2c4b05530c994b586d8b65e9600dcf519c6d387db4f98f93b503  review/issue1-original.md
  20ab9493c67b6cea229b8713ac842ad094de8e0f4c0cd6890438e4efaaa8a3b4  src/core/typed_http.go
  ed5ec8fde06773216eb149719eab9b649f42706f7c956a8b4538097d63e7279f  src/config/config.go
  e85739d99db70cc80cf84a34a5ba3060c304e74f5fba41929bbf9c1192a94654  src/model/model.go
  bfe30046fbffd9b0f08ebc4bf0dad6166dae303c1c284b37badc065aa6367647  src/context/context.go
  3048d4abeed579916bc9fc6532ef3116710f1b055df40338ca83ab9ed47759ae  src/core/work.go
  9f6467a42edab896beb39c278cbb8c0b77f7e367bd149a890b904f1c4cccf371  src/core/source_file.go

策略未扩权核查：git status 显示本 issue 仅改 README.md、docs/Operations.md、release/README.md、src/core/work.go；src/config/config.go 未修改 → 部署默认 ProviderPolicy 仍仅 SYNTHETIC。文档与 release/README.md 均已明确：fixture_path 仅合成、真实 source.sync 未开放、source_configs 与 provider_policy.source_ids 保持空、SECRET 永不外发。

────────────────────────────────
四、有界性说明（非 mustfix，不附加无实证门槛）

注 1：诊断归档断言在 reports/ 目录缺失时静默通过（os.IsNotExist→nil，:139-141），且只覆盖 reports/ 子树。本轮没有发现反向证据；按你要求不据此附加新门槛，仅如实标注该断言为「子树 + 存在时」证据。若后续真实试运行需要更强归档保证，属下一轮范围。
注 2：transport 回调内使用 t.Fatal，隐含其与 Process 同 goroutine 执行；-race 实跑通过，未观察到问题，属测试实现稳健性备注。
注 3：本轮证据为合成 canary + fake HTTP + 临时目录，不构成真实服务商调用、真实资料试运行或任何持续时长证据；作者报告未越界声称，我的 PASS 同样不越界。

────────────────────────────────
五、参考资料

- issue 原文（GATE 段）：<PROJECT>/review/issue1-original.md （GATE-01 行 126-150，GATE-02 行 152-173，关闭条件行 177-186）
- 接入边界选择：<PROJECT>/docs/RealDataTrial.md
- 运行边界修订：<PROJECT>/docs/Operations.md （第 9 行、D12 节）
- 部署说明：<PROJECT>/release/README.md （第 24 行）
- 实施记录：<PROJECT>/reports/implementation/issue1-gates.md
- 测试源码：<PROJECT>/src/tests/issue1_gates_test.go
- 公开仓库 issue #1：https://github.com/ExplodeCode6324/Secretary_Simplified/issues/1

【思路】先以只读方式建立「声称—验收项—测试断言—生产代码」四层对齐，再亲自实跑作者给出的两条定向命令（含 -race），最后沿真实调用链核验断言落点：HTTP 入口默认分类在 typed_http.go、披露拒绝在 model.Encode 与 context.Build 两处、网络唯一出口 p.HTTP.Do 被 fake transport 计数。正控制（PERSONAL canary 必须在 wire 上）与负控制（SECRET canary 必须不在 wire 上）同时成立，排除了空载荷假通过；同库保守拒绝以「拒绝 + 无第二次外发」双重断言核实，与 context 全量快照先检实现一致，故不把「未实现混合权限可用」列为缺陷——它是文档已声明的限制而非隐藏行为。全程 9 轮工具调用内完成，未写文件、未动 soak 与生产代码。

结论：GATE-01/GATE-02 指定分支——限定范围 PASS，无 mustfix (｡•̀ᴗ-)✧
