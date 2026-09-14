# Issue #3 验收与工作索引

基线：`c84d92453a1d8338be728422e3164777ef822579`。仅处理 [Issue #3](https://github.com/ExplodeCode6324/Secretary_Simplified/issues/3) 的两个客户端遗留问题，不重开 #1/#2。本页是当前索引，原失败、首轮通过和独立复核均保留各自文件身份。

## 实施与验收

| 要求 | 修改与证据 |
|---|---|
| R1-01 | POST 受理与 GET 观察错误分开；4xx 不清除已受理身份、恢复索引或回填重发草稿 |
| R1-02 | 查询超时/5xx、重建客户端后按原 request/turn 查询，终态去重并清理 |
| R1-03 | 确定提交前拒绝、提交结果未知与迟到结果分开；不覆盖新草稿，不自动重新 POST |
| R2-01 | 四类业务面板保留已加载页、稳定选中 ID、尾游标；只刷新所选页 |
| R2-02 | 面板代次隔离迟到/重复响应；过期 items 游标停止翻页，只查原对象 |
| R2-03 | 更新版本可见，消失目标取消选择并提示；确认保持原 ID/版本，无隐式 ack |
| DOC-R | [逐文件影响清单](docs-impact.md) / [章节与哈希 JSON](docs-impact.json)，新 v1.1.1 导出，历史包与报告不覆盖 |
| BUILD-R | [CLI 构建与源码指纹](build.json)，daemon、空库、依赖与原测试文件保持原字节 |

代码细节和测试映射见 [implementation.md](implementation.md)，原始要求见 [issue3-original.md](../../../review/issue3-original.md)。本单使用新增 `^TestIssue2Residual` 定向 race 测试及 `go vet ./cli/tui`；未跑原 13/27 项、旧 Context/Store/持久化/恢复套件、全仓测试、付费模型、月回放或长时 A25。合成客户端状态机测试不冒充新的后端或 PTY 验收。

## 工作记录与复核

1. 原提交实际复现： [before.json](before.json) / [before.log](before.log)；真实 items 游标契约补充失败见 [before-items-cursor.json](before-items-cursor.json)。原日志与测试修订身份不改写。
2. Ayanami 的 [设计讨论](../../../review/issue3-design.response.md) 与 [生效更正](../../../review/issue3-design-correction.response.md) 明确观察错误、选中页刷新及事项快照失效行为；实施缺陷修改路径已同步根 README。
3. [首轮实际代码复核](../../../review/issue3-runtime-final.response.md) 使用指定 DeepSeek，核心验收限定 PASS；发现的无新 ID 页和重载失败后选择意图两处细节继续补齐，不把初审当最终交付结论。
4. [文档初审](../../../review/issue3-docs-review.response.md) 如实保留活动编辑期间的导出失配；正文冻结后重新导出并另行终审。

最终自测：[after-retry.json](after-retry.json) / [日志](after-retry.log)，12 个父测试、37 个场景 PASS，race 1.666 s，vet PASS。[after-final.json](after-final.json) 保留 11 项 / 35 场景中间修订，[after.json](after.json) 保留首轮 9 项 / 29 场景身份。新增复核边界的实际修前失败见 [before-review-delta.log](before-review-delta.log) 与 [before-retry.log](before-retry.log)。最后一处是重复调用重置函数的防御边界，普通 r 路径本已保留选择，未宣称函数级测试证明实际按键故障。

CLI 已重新构建，SHA-256：`0c0b3f2c90283904595b9f0e946f160ea851f8b996900db961714fc4835b31d5`；[build-initial.json](build-initial.json) 和 [build-delta.json](build-delta.json) 保留此前构建。当前 [build.json](build.json) 绑定最终测试与生产源码指纹，daemon/DB/依赖/原测试文件与基线逐字节一致。

Ayanami [增量复核](../../../review/issue3-runtime-delta.response.md) 实际运行 35 场景及独立反向失败验证；[最终静态核证](../../../review/issue3-repeat-reset-final.response.md) 对最后回退逻辑、37 场景日志和源码哈希给出限定 PASS、0 mustfix。后者没有再次亲跑测试，两类证据严格区分。

[DOC/BUILD 静态终审](../../../review/issue3-docs-final.response.md) PASS、0 mustfix：四处关键规范逐行核对，简短 DOC 检查命令实际通过；此前两条复合命令被 Hermes 自动审批解析器拦截，未执行。该审查的构建叙述仍绑定 35 场景 / `59d93dd7…` CLI 的中间快照，不能冒作最终字节核验。[Root 最终 30 项文件验证](final-validation.json) 独立绑定最终 37 场景源码、`0c0b3f2c…` CLI 及 `e158d49d…` 导出包，全部 PASS。旧导出包、旧报告与原代码测试未覆盖改写。

**结论：R1-01—03、R2-01—03、DOC-R、BUILD-R 完成，无开放 mustfix，可以安排受控真实文字接入。** 发布前 [凭据与私人路径扫描](publication-scan.json) PASS；GitHub 实际提交与关闭状态以 [Issue #3](https://github.com/ExplodeCode6324/Secretary_Simplified/issues/3) 的交付评论为准。

## 使用边界

查询失败时保留原请求，恢复认证/连接后继续查询，勿换新键重发。items 快照失效时 n 暂停，r 明确回到第一页；旧目标不在新页时取消选择。自动刷新只更新所选页或失效快照中的所选对象，其他已加载页是缓存。既有通知 GET 可写 DELIVERED，不等于 ack。

由 Master 按 [真实文字试用说明](../../../docs/RealDataTrial.md) 提供受控 PERSONAL 文字、允许使用的模型服务商和资料外发范围。真实资料尚未接入，文件来源同步尚未开放。
