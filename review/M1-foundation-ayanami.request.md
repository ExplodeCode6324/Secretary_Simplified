请立即复核已完成的 M1 foundation 模块，只读业务源码，可执行隔离安全测试。源码范围：src/contract（完整 Schema 与生成 DTO/闭包）、src/platform（Clock/Lock）、src/store/store.go、items.go、objects.go、sources.go、src/ingest/fixture.go，以及 foundation_test.go、fixture_test.go、contract_test.go。
实现 agent 报告 `go test ./contract ./platform ./store ./ingest` 全过，包含 CAS/DAG 回滚/事件原子性、四连接 PRAGMA、对象篡改、fixture 重复和冲突 quarantine 测试。请自主核实，不直接把报告当证据。
已知待补：应用扩展注册 runtime.authorization/misfire；receipt 基础由 root core_repo 实现。其他 agent 正并行修改其他业务模块，勿修改任何源码。禁止读取 resources 凭据或连接 ELIZA。
请依设计重点检查 A01/A03/A04/A05/A19 相关基础行为，列出具体文件行号、可复现场景、严重性、修复建议。不能因为单元测试通过就泛称完整验收通过。若设计缺陷，逐项给出商议意见，已获 Master 授权商议通过后修订设计，无须再次请 Master 批准。
回复应包括结论（通过/有条件/需修复）、执行的检查、发现和未覆盖项。只输出回复，由 Codex 保存 review/M1-foundation-ayanami.md，不另写同名文件。
