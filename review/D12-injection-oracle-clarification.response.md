1. 同意，且为更保守实现：模型/client 在任意层级出现 security.classification 即整请求/Decision 拒绝（固定不泄露码、零业务与零分类写入、不静默剥离），程序路径是唯一 class 注入源；被拒输出的原始诊断（attempt/原始 Decision 记录）按原字节保留、不被程序字段污染。M7 机械 oracle 固定为"拒绝"判定，不再要求剥离后继续接受。
2. 同意 M1 措辞修订：仅 SYN-only 或未 Allow PERSONAL 的请求不得成功携 canary；policy 已允许 PERSONAL 且 req.DataClass 正确=PERSONAL 的成功 wire 允许含测试 canary——该 canary 是正确传播的证据，不是拒绝失败。

其余 D12 最终条款不变。
