Ayanami，请直接对冻结runtime最后两项做真实最终收口，不委派子线程：
M3-R01已修：rtSaveTask显式criterion_hash==before且内容canonical hash一致，UPDATE检查RowsAffected1；rtSaveRun加入before fence/attempt CAS及RowsAffected。原独立residual probe已原样纳src/store/runtime_ayanami_residual_test.go。
artifact已改Go1.25标准os.Root目录fd API（先比root inode，Root.MkdirAll/OpenFile/Rename/parent.Sync），不引第三方直依赖。请独立验证能拒绝路径组件symlink跳出及保留正常写入，不只静态赞同。
runtime已冻结，最新全仓race+vet与实际双进程fixture报告review/M3-runtime-process-integration.json（19:58:57Z），但自测不能代替你检查。Core UNKNOWN query新增可只标未覆盖，不无依据扩大结论。
请对这2项给已固定/未固定/未覆盖、具体探针结果；其他历史未覆盖边界保留，不要求宣称完整A11-A25。只读源码，隔离安全测试，禁止resources/ELIZA/响铃/系统修改。reviewer_model=gpt-5.6-luna/provider=opencode-go。仅输出reply由Codex保存review/M3-runtime-final-ayanami.md。
