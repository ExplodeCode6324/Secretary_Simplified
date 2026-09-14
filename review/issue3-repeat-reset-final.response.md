最终静态核证完成（Ayanami；本轮未跑任何 tests/probe，未改源码/文档，未读 keys，未调后端/模型；12/37 与 vet 为 runtime 执行，我只读核验，不代称亲跑）。

一、明确裁决：该一行充分收口原观察，无 mustfix

panels.go:108-119 现为：
```
selected := m.selectedID()
if selected == "" { selected = m.panelSelection }   // 新增行
cmd := m.showPanel(m.panel)
m.panelSelection = selected
if selected != "" { m.selected = -1 }
```
- 顺序正确：先读 selectedID()（失败 reset 后为 -1 → 空）→ 回退到仍保留的 panelSelection → 再 showPanel（其在 :94 清 panelSelection）→ 重新写回意图。因此直接重复调用 resetPanel 时，原待回绑 ID 不再被空值覆盖，与我原观察指向的失效点一一对应。
- 精确范围（按 runtime 说明逐条静态核对，属实）：普通用户路径中 `r` 仅在 `m.panelInvalid==true` 时进入 resetPanel；reset 后 showPanel 置 panelInvalid=false，故第二次 `r` 走 `openPanel(name,false)`（该路径上轮已被 R203ResetFailureRetainsSelectionIntent 覆盖，意图本就保留）。本行因此是**防御边界**：当 reset 后无行被选中再次触发 resetPanel 时（例如第二次 `r` 走 openPanel 又遇 409 使 panelInvalid 复为 true、第三次 `r` 再进 resetPanel），意图同样不丢。范围界定准确，不是主路径行为变化。
- 两个反例覆盖 present/absent 两结局：:502-523 的 R203RepeatedResetRetainsSelectionIntent（selected=25 → 原 ID 复选；selected=75 → selected=-1 + 「原选中」提示），与既有 R203ResetFailureRetainsSelectionIntent 互补，收口完整。

二、证据核验（runtime 执行，逐项只读核对；我方未执行）

- before-retry.log：新父例真实 FAIL（:516「repeated reset lost present original ID」、:519「repeated reset selected unrelated row」），行号与当前测试修订一致 → FAIL 为同修订真实复现，非事后归化。hash 12bfd4b5…
- after-retry.log：`ok secretarysimplified/cli/tui 1.666s`（hash 19ec28d8…）；after-retry.json：status PASS、tests exit 0、vet exit 0、original_suites_run=false、12 parent/37 case、production hashes 与我独立复算**完全一致**（client 098bb781… 与 model 8933b156… 未变；panels c819d9a9…；test e9aaa2c9…）。vet-retry.log 0 字节（空摘要 e3b0c442…）。
- 计数自证：`grep '^func TestIssue2Residual'` = 12 个父例（11+新1）；37 case = 前轮 35 + 新 2 子例，与 json 自述一致。
- 我方上次亲跑仍有效部分：我的亲跑（11 parent/33 leaves+2=35 case、vet 0）对应的 model.go/clayout hash 与本次完全相同（model.go 未变、client.go 未变）；本次唯一源变更是 panels.go 加 1 行（d3306373… → c819d9a9…）与测试 +1 父例（5488cc51… → e9aaa2c9…）。故我**亲跑的是同一选择器上一修订**，本行之后的结果我依据 runtime 日志 + 静态读码核验，不冒充亲跑。
- 旧证据保留：evidence-index.json（dd6bd849…）中 before.*/after.*/after-final.*/vet*.log 各条 hash 与上轮核验值逐一相同，history_preserved=true；review/issue3-evidence/ 内我历次产物原样（ayanami-probe.go.txt 402c57d0…、probe.log 4c43a621…、residual-race 97f1c546…、residual-verbose 435a8b8c…、ayanami-delta-two-tests.log 4cc0ffb2…=我 /tmp/i3_delta2tests.log 原样、delta-vet e3b0c442…）；manifest.json 更新为 c9b19532…。
- DOC 正文：本轮不动；我复核 docs/SingleConversationTUI.md:102/:104 三处澄清（位置首次/值后页优先、0 新 ID 停止、r 读取失败保留待回绑 ID）仍在且与本行新逻辑语义一致（hash 35650aae…，仅记录，未做前后逐字节对比）。
- 范围外（不代核）：build.json cf32b534… 为新 CLI 的 root 本地构建核验；implementation.md 更新为 b9ade3aa…（我未重读正文，仅核 index 声明）；docs-impact.json/md 14:14 刷新过，非本轮核验目标。生产无 backend/schema 变动（client.go 零 diff、model.go 仅字段、panels.go 仅该函数回退）。

三、最终 source/test hash（本行定稿身份）

- src/cli/tui/client.go 098bb781363742fad17b9c9e99b78cca2e8a12dd1775613bb29e27b102c1970e
- src/cli/tui/model.go 8933b1563631e8372002defb560466af4f08daeba5d5774dfb9d9e869d997537
- src/cli/tui/panels.go c819d9a9d1f9d9fb74302d1861d8c26daa293d39c9c59794fa4badbb8652a973
- src/cli/tui/residual_test.go e9aaa2c991ceaa19098fae06584fd132eb823de71646a2759c993809e11ea11d
- reports：before-retry.log 12bfd4b52625f9920faca476f2e5fc1ce1e09938ac66b70a4a92ef460077a7b8；after-retry.log 19ec28d8e64dc26474e4792984421c6be0adf999de96954c278900c86a0444f4；after-retry.json 23f5be0b44fe392e64af24560a3f04d581966f08f0d9e20766a3b986569fa837；vet-retry.log e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855（空）；evidence-index.json dd6bd84942b9938f94d1f9bdf0e977fe5b6945736a01dbdb90141699e98fd373

四、结论与边界

该行修复针对我原观察的失效点精确、无副作用（不改变主路径、不引入请求或状态写），裁决为：**RES delta 终态 限定 PASS，0 mustfix**，我方观察已收口，后续 root 只刷新 build 证据与 review 清单即可，不循环再审。未测边界如实：本轮 12/37/vet 为 runtime 环境结果（hash 已验证、内容已读），我未亲跑；我未做终构建、未跑旧套件/后端/长测；文档仅核对三处澄清文本在场，未做字节级前后对比。

思路：只读三处源/测试差异 + 失败与通过日志 + index 自证 hash 的反向复算，把「我亲跑的部分到哪一修订为止」与「哪些结论来自 runtime 日志」严格分层，再按原观察的失效路径闭合性作裁决，不越界重跑也不把证据核对写成亲跑。( _ _ )
