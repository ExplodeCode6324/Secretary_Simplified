Ayanami，本轮只做3个具体未覆盖项，严禁新静态审计、CLI其它功能、全仓测试、子agent；请集中脚本执行后直接最终回复。前两轮CLI主干已通过，不再重做。
1 在临时synthetic目录实际CLI backup/restore（含一个可领取QUEUED run或立即可触发计划，不能仅空队列证明），校验fresh tokens不同且互异（只布尔不输出值）、空cap grant、config+file双冻结；真实启动恢复Runner 2秒，断言队列/attempt/效果计数不变后结束仅自建进程。
2 实际verify --suite smoke、verify --backup正向与复制backup篡DB后负向，记录exitcode/hash拒绝。
3 go test ./tests -run '^TestRuntimeRemoteQueryRebindsEvidence$' -count=1 -v；读该测试是否断言Query后Execute/POST计数不增加，若已断言无需重造。没有断言则仅加临时独立probe，不改仓库。
已核实CLI语法：init --data-dir DIR；之后 --config DIR/config.json；backup --output PATH；restore --backup PATH --target DIR；verify --suite smoke --report PATH；verify --backup PATH。布尔--json放命令末尾。实际参数以src/cmd/secretary/main.go:85-154为准。
可复用/tmp/m3e/bin之前本轮DeepSeek构建二进制或当前release/bin，但须记hash和快照，不自行重新全build若无必要。若需合法QUEUED seed，可隔离复制src后临时测试复用现有tests/runtime fixture helper创建并Backup，不能向主仓库加测试，不篡坏DTO造不合法队列。目标只3项，约8个批量工具动作完成；主动收口别耗尽预算。
禁止resources/真实API/ELIZA/响铃/系统修改，不触旧/tmp/m3ds或release/soak现用进程。模型必须deepseek-v4.1-flash/opencode-go，429报告。输出review/M3-restore-query-only-deepseek.response.md由Codex保存，明确实际PASS/FAIL及未覆盖，不泛称全部验收。
