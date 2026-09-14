收到，短收口裁决：五点全部同意，无反对项。按你给定收敛执行，边界如下（仅澄清，不扩大范围）：

1. 发送键 — 同意固定 Ctrl+S（raw 模式关 IXON）+ 固定 Alt+Enter 备用 + Enter 换行；不新增发送键配置项/配置系统（Issue 只要求通用可用按键，可配置不是验收项）。附带两条非新增条件：PTY 用例须证明 (a) ^S 实际送达并提交成功、(b) 退出后 termios 恢复；帮助文本写明三键。个别环境若吞键，处理方式是帮助/文档提示（可含一行 stty -ixon），不引入配置。

2. metadata — 同意每客户端独立文件 + instanceID 绑定，天然避免共写；保留原子写（temp+rename）作为崩溃安全。仅当未来某设计真的共享同一文件时才需要 flock，本轮不引入。恢复真源仍在服务端（GET /v1/requests/{id} 为既有路径）。

3. 非 TTY — 同意：pipe 逐行到 EOF，允许阻塞等待 stdin，但无键盘接管、无全屏控制序列；`--json` 保持纯 JSON（现有 output 在 --json 路径本就只输出 JSON，状态行仅出现在非 json 分支，行为不变）；逐行模式本轮不做额外语义变更。

4. history / authority — 同意：history 用独立 conversation sequence，不与业务 events seq 混用（§5.1 正是要求两者不可混用）；questions/pending 聚合进 authority snapshot，不强制另建独立路由。唯一非新增条件：snapshot 查询有界（不全库扫）、条目带稳定 ID/revision 供去重与刷新，history 仍为 cursor 分页。

5. 粘贴 — 同意：bracketed paste 整段作为草稿插入并过滤控制键（含抑制其中的 ^S 触发发送）；未 bracketed 输入无法识别粘贴边界，明确作为文档化限制；任何 ESC/控制内容都不得升级为操作或提交。

无旧测试/长测门槛、无新增远程/付费调用、race 仍限本轮选定用例。以上即本轮设计定稿；实现与 PTY 证据落地后按 AUTH/TUI 编号复核即可。( _ _ )
