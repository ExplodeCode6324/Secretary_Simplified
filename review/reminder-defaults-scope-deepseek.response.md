确认（reviewer_model: deepseek-v4.1-flash / provider: opencode-go）。两点均无实质矛盾，同意，以下为最终并入文本。

1. Scope 确认：同意。guard 作用域收窄为——自然语言 Decision CREATE_JOB 且 capability ∈ {notify.local, alarm.play}；其它非提醒 job 不加 5 分钟限制；已认证 Typed 通道不变。自然语言明确要求自定义迟到策略时，系统只能说明须用显式 Typed 配置，不生成替代默认动作（与 v2 K4"不得静默替换用户意图"一致）；guard 仅拒绝模型越出 v1 能力，不声称证明任意 NL 语义。此为对语义证明责任的正确收缩。

2. K2 更正：同意并撤回。我方 v2 中"总计 ≤2 次模型调用"系自设常量，作废，不作为任何实现或验收依据（也不得叠加保留）。K2 更正为：沿用既有已文档化的 Core 语义校验重试上限与持久预算，不新增、不放宽、不重置次数；耗尽时无副作用失败、反馈留档；不为本缺陷单独新设常量。必要性说明：无——本缺陷属信息通道遗漏，非重试预算不足，我方无依据主张改动既有 Core 上限，故不改。

其余 v2 条款（通道契约、REMINDER_POLICY_UNSUPPORTED 拒绝原子性、T1–T4 固定 criteria、不改 runtime/oracle/run1、失败保留、有依据修复后可复测、不降标）维持不变。本次唯一调整即 K2 文本替换。未使用工具、未改动任何文件。
