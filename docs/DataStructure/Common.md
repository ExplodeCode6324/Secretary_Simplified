# 通用字段与校验

## 1. 类型

所有外部 DTO 为 UTF-8 JSON；初版 `schema_version=1`。ID 为 UUID，时间为合法 RFC3339，持久化统一 UTC 毫秒。JSON null 表示已知没有值或未知，具体由状态字段解释；缺少 required 字段表示结构错误。空数组表示该次查询无所选成员，不能代替 omitted_count。

主要结构的 extensions 必须出现，未使用时为 `{}`。顶层 additionalProperties=false。数值禁止非有限值；版本为正整数，创建的 expected_revision=0。正文、数组与检索受 Schema 和上层字节预算双重限制。

EvidenceRef 包含 object_id、sha256、locator、origin_id 和 data_class。VersionRef 为 id/revision；ReadRef 再带 entity_type。引用必须由服务验证存在及可访问；JSON Schema 的 format 校验须显式启用，默认忽略 format 的校验器不满足要求。

entity_id 初版可指已登记的 Master／设备配置实体、Item 或 SourceState，实体类型由调用上下文和注册表验证。来源事项 ID 从固定命名空间及 source_id/external_id 派生，版本变化不换实体 ID。关系目标必须已经登记；模型不得凭空生成 UUID 当作已存在实体。

## 2. 空值与扩展

- 字段没有可靠值时用 null 并设置 UNKNOWN、CONTESTED 或缺失原因，不补成当前日期、0 或空文本。
- 上游新增字段保存到原文，适配器可放入有命名空间的扩展对象；未登记的扩展不能驱动任务或事实准入。
- 修改通用状态、权限或时间语义必须升级主契约；扩展字段不能覆盖核心字段。
- SourceRecord.normalized、Observation.value、WorldFact.value 等载荷由 [TypeRegistry.md](TypeRegistry.md) 的类型 Schema 继续约束，不是无限制执行入口。

## 3. 校验顺序

大小／JSON 解析 → major 版本 → 完整 Schema 与 format → 类型注册表 → 引用与当前版本 → 身份与披露策略 → 业务状态／完成条件 → 持久去重 → 事务提交。任一步失败都不能产生部分业务效果。

同组 action 可以用 item_operation_key 引用该组 CREATE_ITEM 的 operation_key。程序预分配新对象 ID，解析所有组内引用后执行校验和一个事务；禁止前向引用到组外或同名 key。item_id 与 item_operation_key 恰好一个非空（不关联事项时两者可空），不能同时有值。

## 4. 序列化与哈希

语义幂等载荷使用确定性编码：对象键递归排序、数组保序、UTF-8、无多余空白；禁止重复 JSON 键，schema_version 和 null 字段保留。哈希输入不包含自身哈希、接收时间和重试统计。Go 实现需以固定 golden fixture 验证跨语言一致，不能默认任意序列化库都会产生相同数字格式；身份相关载荷首版只用整数和字符串表达数值。

原文哈希直接计算原始字节；Context.request_hash 计算最终模型请求字节。二者不使用语义规范化，不可混为一个哈希。每个哈希的来源和算法写进对应 manifest。


## 实施设计修订 D12：派生输出分类

统一程序独占 `extensions["security.classification"]={"data_class":<enum>}`；enum 为 SYNTHETIC/PERSONAL/SENSITIVE/SECRET，严格单字段、禁止额外成员。分类是披露上界，与事实真假、授权或证据质量独立。程序使用实际冻结成功模型请求的有效 class，按旧对象／本次请求／逐字复制来源取最高分类；更新和复制不降级。无 Evidence 或只有低分类 Evidence 均不能证明派生文本为低分类。

模型／客户端在任何结构层注入此键，整请求／整 Decision 拒绝；不读取其标签决定业务，原始拒绝证据保持原字节。持久写入仅使用程序值，其他合法 extension 保留。缺失／非法的旧派生标签保留 unknown，在披露／重推导入口返回 OUTPUT_CLASS_UNKNOWN，不回填 SYNTHETIC、不伪写 SECRET、不删除数据。空内容程序脚手架可无标，固定且不含用户／模型内容的字面量可显式 SYNTHETIC；真实输入原文仍用其原始 data_class。

裁决：`review/D12-output-class-final-contract.response.md`（整体替代初稿），反注入补充：`review/D12-injection-oracle-clarification.response.md`。无 DDL 或顶层 class 字段扩张。

D12 编码等价整理：40 个完全相同的 extensions 机器定义提取为 `$defs.RegisteredExtensions`，各载体使用 `$ref`；展开该新增间接引用与原 Schema 完整结构相等。此整理只消除输出闭包中的重复字节，不改变字段、注入拒绝、分类规则、32K 限制或验收 oracle。验证见 `reports/implementation/registered-extensions-equivalence.json` 与 `TestRegisteredExtensionsEquivalentToOriginalInlineShape`。
