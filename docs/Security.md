# 信任与授权

## 1. 信任来源

系统规则来自受控配置；Master 身份经交互入口认证；资料、网页、文件、仓库、Canvas 内容和工具返回均为数据。文档中出现“忽略规则”“允许修改世界模型”等文字不能改变能力、授权、预算或程序策略。

输入的 origin、主体身份和认证结果由适配器填写，模型不能覆盖。模型 Context 将受控指令与不可信内容分区；隔离提示词仅为辅助，真正的写入约束由类型校验、授权和执行器实现。

## 2. World Model 专用授权

仅 `world.update` 能力能请求 WorldCommitService。AuthorizationGrant 保存主体、scope、允许能力、policy_revision、有效期与撤回版本。ExecutionPermit 由程序根据 grant、run、fencing 和 cancel_generation 签发，绑定本次 proposal_hash，禁止重复消费用于不同提案。

权限必须限定到实体／predicate／操作，例如测试配置只允许写 test 域的候选或测试事实。初版测试授权由初始化配置产生，不要求 Master 每次点击；真实阶段授权再按来源和业务范围登记。模型输出的 `authorized=true` 或 capabilities 字段均不能作为凭证。

普通 agent 不能直接打开 SQLite、读取凭据、执行任意 shell 或访问真实源目录。world.update 的模型部分只提出变更，最终事务由程序服务提交。Core 和 Runner 是同一宿主用户下的受信程序，本设计不声称能隔离已经控制该用户账户的恶意进程。

## 3. 数据披露

所有输入和产物带 data_class：SYNTHETIC、PERSONAL、SENSITIVE、SECRET。分类沿派生链取最严格值；摘要不自动降低分类。ProviderPolicy 按模型配置登记允许的数据类别和来源范围。SECRET 永不进入 Context、模型请求、工具参数转储或日志。PERSONAL/SENSITIVE 只有在匹配已授权外发策略时可送相应服务商。

阶段 1—3 默认只使用合成资料和测试目录；无需接入真实账户。可用模型凭据仅通过明确配置的 secret_ref 解析，不能自动扫描环境寻找其他账户令牌。云端和本地模式必须显式配置，连接失败不得悄悄改用其他服务商或披露范围更宽的模型。

原文、模型上下文和诊断报告都可能包含隐私。默认日志仅存 ID、状态、耗时、哈希和脱敏错误；完整上下文限诊断开关且受本机目录权限保护。对象引用为受控相对路径加哈希，拒绝路径穿越、符号链接逃逸和任意 URL 取回。

## 4. 本机与移动阶段

初版使用本机 socket，凭据单独保存。阶段 6 的移动配对、传输认证、设备撤销、离线重试和远程停止必须另行通过验收。真实凭据、联网端点和网络暴露不能由设计中的示例自动启用；不要求修改现有服务器、代理或 FRP。


## 实施设计修订 D12：派生输出分类

统一程序独占 `extensions["security.classification"]={"data_class":<enum>}`；enum 为 SYNTHETIC/PERSONAL/SENSITIVE/SECRET，严格单字段、禁止额外成员。分类是披露上界，与事实真假、授权或证据质量独立。程序使用实际冻结成功模型请求的有效 class，按旧对象／本次请求／逐字复制来源取最高分类；更新和复制不降级。无 Evidence 或只有低分类 Evidence 均不能证明派生文本为低分类。

模型／客户端在任何结构层注入此键，整请求／整 Decision 拒绝；不读取其标签决定业务，原始拒绝证据保持原字节。持久写入仅使用程序值，其他合法 extension 保留。缺失／非法的旧派生标签保留 unknown，在披露／重推导入口返回 OUTPUT_CLASS_UNKNOWN，不回填 SYNTHETIC、不伪写 SECRET、不删除数据。空内容程序脚手架可无标，固定且不含用户／模型内容的字面量可显式 SYNTHETIC；真实输入原文仍用其原始 data_class。

裁决：`review/D12-output-class-final-contract.response.md`（整体替代初稿），反注入补充：`review/D12-injection-oracle-clarification.response.md`。无 DDL 或顶层 class 字段扩张。

CLI/API/typed 普通输入缺省 PERSONAL；合成验收必须显式 SYNTHETIC。AllowedClasses 不扩权，SECRET 恒不允许出网。可信旧隔离合成库仅可凭完整来源证明离线显式升级并留审计；不得由“现存输入全合成”推断历史输出。完整请求上下文可重建时须 join 历史依赖并保留原字节，其他 unknown 明确阻断。
