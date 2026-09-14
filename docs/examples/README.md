# 契约样例

manifest.json 指定每个 JSON 文件对应的 `$defs` 及预期通过／拒绝结果。输入完全合成，不代表 Master 的真实事项或偏好。

主例由 input、decision-create-reminder、item、source-record、consciousness-input、consciousness-draft、receipt 和 wait 展示不同阶段；world-proposal 单独展示偏好更新。原文与证明样本在 fixtures，ObjectRef 样例提供哈希。它们是结构与语义说明，不是已经运行的服务回放，也不是一份能直接导入生产数据库的快照。

负例覆盖伪造授权字段、未知能力、错误能力参数、缺少时间、错误 major、非法日期、CONFIRMED/null 矛盾和冲突的组内事项绑定。拒绝来源是 Schema；真实权限撤销、引用存在性、执行效果等还需实施后的业务测试。

## Issue #2 公共入口与内部 DTO

原 input.json 的固定 session_id 是合成内部 DTO 示例，不是客户端创建任意会话的请求模板。公共输入可以省略 session_id，由后端绑定，再校验内部 InputEnvelope；typed 公共例子见 release/API.md。保留原契约示例用于结构说明，不把旧多会话样例当作当前使用建议。

公共省略会话的完整信封见 [release input-public.json](../../release/examples/input-public.json)；后端绑定后仍按同一 InputEnvelope Schema 校验，原 manifest 不把公共适配层信封误当内部 DTO。
