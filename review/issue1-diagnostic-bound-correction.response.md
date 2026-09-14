确认一：更正确认——保留 model_records.go 现有错误传播（Output PutObject 失败仍直接 `return result, e`，不吞错、不改语义），本次仅加 5s deadline；并且 Recorder 位于业务 apply 之前，模型返回成功不构成业务已提交的依据，provider 记录不伪 SUCCESS/FAIL，我前述"best-effort"措辞不作为新增吞错要求。
确认二：5s 上限仅保证 ctx-aware 的 flock 等待与 DB 路径有界返回，不宣称同步 fsync 或任何内核级阻塞可被 ctx 打断；对应短测按此口径断言，不扩为其他保证。
