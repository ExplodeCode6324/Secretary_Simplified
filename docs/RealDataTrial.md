# 受控真实文字试运行范围

本文件记录实施审计 issue #1 的 GATE-01/GATE-02 接入限制与本轮选择。它收窄真实接入承诺，不把已经完成的 SYNTHETIC 合成验证改称真实资料验证。

## 本轮入口

仅支持 TUI、plain chat、普通 CLI/API 的 PERSONAL 文字输入，不开放真实来源文件同步。input/chat/typed 默认 PERSONAL；受理状态 202 只表示本机保存，不代表已经调用模型或完成业务。查询 turn 的最终结果确认处理状态。

`source.sync` 当前只有合成 fixture 适配器，SourceState、ObjectRef、SourceRecord 按 SYNTHETIC 登记。`source_configs[].fixture_path` 只可指向人为构造的合成验收材料；禁止替换为真实邮件、日历或其他真实文件。文件正文自称“合成”不形成可信分类。不得把真实文字改为 SYNTHETIC 来绕过披露拒绝。

当前部署的 ProviderPolicy 仍仅允许 SYNTHETIC，未被本轮文档或测试自动扩权。Master 接入时先明确服务商、允许外发的文字范围，再在专用目录的配置中显式允许 PERSONAL。`source_configs` 和 `provider_policy.source_ids` 在本轮文字试运行中保持空数组。SECRET 永不得交给模型；凭据仍使用独立 secret_ref 文件，不放输入正文。

## 独立目录隔离

本轮选择独立 data_dir，完整隔离 SQLite、objects、配置、凭据及运行 socket，不复用合成验收库，不导入备份中的仅本地或未知分类资料，不共享对象目录。初始化新目录时使用短路径以满足 macOS Unix socket 长度限制。受控目录只保存 Master 明确允许用于本轮模型交互的资料；仅本地资料使用另一独立目录。

当前 Context 在选取相关内容前对全量快照检查披露权限。同库放入 SECRET、未授权来源或分类未知的旧派生记录，即使表面与当前请求无关，也可能使普通请求返回 DISCLOSURE_DENIED 或 OUTPUT_CLASS_UNKNOWN。这是现有保守拒绝限制，本轮不承诺同库混合权限可用。禁用来源不等于删除其快照或取得披露授权，新增/恢复来源仍必须遵守该边界。

不移除分类检查、不自动补低分类、不批量给全部数据扩权来恢复可用性。本轮不进行旧库自动迁移或清理；已有权威记录和证据保留。

## 可复核证据

`src/tests/issue1_gates_test.go` 的 `TestIssue1GatePersonalAuthorizationAndDirectoryIsolation` 使用全部虚构内容，实际公共 HTTP 默认分类、Core、SQLite、Provider 编码和模拟 HTTP 响应路径：

- 省略 data_class 后持久化为 PERSONAL；未授权时 provider HTTP 调用为零。
- 显式测试授权后同类文字成功调用一次并提交回复；授权仅存在于临时测试配置。
- 独立目录中的 SECRET canary 不出现在允许目录的模型请求；同库混放反而明确拒绝，不宣称混合权限已实现。
- PERSONAL/SECRET canary 不写入模型诊断原文归档。

测试不是外部服务商或真实资料试运行，也未将现有机器策略改为允许 PERSONAL。Master 的真实数据范围确认和后续试运行结果另行记录。

## Issue #2 交互入口

更新版在同一配置目录下使用 `secretary --config <配置>` 或 `secretary chat --config <配置>` 打开 TUI，`chat --plain` 保留纯文本模式。所有客户端共享后端唯一会话；无需输入 session UUID，也不通过新窗口创建新的认知状态。隔离真实试用目录拥有独立实例，不能自动导入合成库或其他会话历史。

旧库先按 Operations 的离线显式迁移流程升级；Issue #3 仅客户端修复，不增加迁移。TUI 不代替 PERSONAL/provider 披露授权，不自动启用真实来源。完成当前遗留修复限定验收后由 Master 提供获授权的少量真实文本，记录实际结果；fake 服务证据不代表真实使用质量。遇到已受理后的查询失败，保留原请求等待查询恢复，不把原文另起新键重复提交。
