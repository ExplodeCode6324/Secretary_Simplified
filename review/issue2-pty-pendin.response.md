# PENDIN 判据与 x/ansi 提升 — 快速判据核查结论（Ayanami）

结论先行
- PENDIN 精确排除：允许。已用本机头文件/手册核实其语义为内核态瞬态，不是用户配置项；但排除必须是「差异集合 ⊆ {PENDIN}」的位级精确规则，不得泛化；生产侧无需任何改动。
- x/ansi v0.10.1 提升 direct：允许，作为既有 TUI 依赖例外的最小补充；无新模块、无版本变动、无新供应链面，文档已同步属实。
- 范围：只涉及新 TUI-12 终端恢复判据 + 依赖标记；不重跑旧测试/长测；run1/2/3 报告保留不动。

一、PENDIN 语义核实（本机证据）
1. MacOSX.sdk/usr/include/sys/termios.h:259：`#define PENDIN 0x20000000 /* XXX retype pending input (state) */` —— 头文件直接标注为「(state)」，即内核状态位，不是用户可配置模式。sys/ioctl_compat.h:173 同值注释 `tp->t_rawq needs reread`；sys/tty.h:100 `TS_TYPEN 0x08000 /* Retyping suspended input (PENDIN) */`；man 4 termios 的 lflag 列表同样带 "retype pending input (state)"。GOROOT syscall 常量表亦定义 PENDIN=0x20000000（BSD 家族语义一致）。
2. 数值核算（已用 shell 算术复核）：before lflag=1483=0x5CB，after=536872395=0x200005CB，delta 恰为 0x20000000=PENDIN；掩掉该位后完全相等；xor 结果 == PENDIN。run2 原始转储也显示 iflag=11010、oflag=3、cflag=19200、speed、cc[20] 全部逐字节一致，仅 lflag 差这一位——即 ICANON/ECHO/ISIG 等真实配置确已还原。
3. 用户态恢复路径不管理该位：charmbracelet/x/term、bubbletea、muesli/termenv 模块源码中 PENDIN 零引用；生产 src/ 内 PENDIN 零引用。即「恢复」语义上根本没有 PENDIN 需要还原，排除它不掩盖任何真实还原缺陷。

二、允许的排除形式（必须精确，不能泛屏蔽）
- 规则应写成：容忍差异集合 ⊆ {PENDIN}，等价于 `(after ^ before) & ~PENDIN == 0`。现探针 scripts/issue2_tui_pty.py:89 的写法（对前后两份快照的 lflag 仅掩 PENDIN，再整组严格相等；其余 iflag/oflag/cflag/cc/ispeed/ospeed 全量比较）正是这一规则，且只作用于 lflag 单一字段——判据合格。
- PENDIN 出现在 before 或 after 任一快照都属同一瞬态类，掩双侧处理正确。
- 附加建议（非新门槛）：校正说明中记录原始 before/after lflag 十六进制与头文件引用（run2 错误串已留存原始数组，可引用）；如需更强置信度，可加一条负向对照（故意不还原 ECHO 必须 FAIL）证明探针仍具判别力——可选，不强求本轮。

三、生产恢复（明确不需要改）
- 不得在生产或探针里用 tcsetattr 试图「清除」PENDIN：头文件标注为 (state)，由内核管理（输入重放用），写入无意义。TUI-12 的生产要求仍是既有两条：退出路径（含 Ctrl+C/SIGTERM 等）完整走标准 termios 还原（bubbletea RestoreTerminal 路径），探针只做校验、不做修复。生产代码保持零 PENDIN 引用即为正确状态。

四、x/ansi v0.10.1 提升为直接依赖
- 证据：src/go.mod:17 仍为 `// indirect`；go.sum 已有该版本 h1 pin；docs/EngineeringArchitecture.md:78 与 docs/References.md:30 已写明「传递依赖提升为直接依赖，按 cell 宽度换行/截断中文显示，版本锁定于 go.mod/go.sum」——文档同步属实。
- 宽度源一致性：x/ansi width.go 基于 rivo/uniseg + mattn/go-runewidth（文书 grapheme 簇 + 回退），与 bubbles textarea 内部来源同族（textarea.go:21-22），不会引入两套宽度口径。
- 允许条件（与此前例外同规）：版本保持 v0.10.1 不动；仅用于渲染层的 cell 宽度测量/换行/截断；import 落地后 `go mod tidy` 使标记转 direct；归入原 TUI 依赖例外条目，不新开供应链审查（同一已锁定构件，无新下载）。

五、报告与历史
- run1（"terminal modes not restored"，无原始转储）、run2（含 before/after 原始数组）保留为「校正前判据」的 FAIL 历史；run3 为校正判据下的 PASS；三份均不改写。校正记录写清楚：排除位= PENDIN（0x20000000，内核 state），引用 sys/termios.h:259，并说明其余字段严格比较——即算「清楚报告 probe 校正」达标。

无新增门槛、无生产改动、无旧测试/长测介入；判据层面我方同意以上两项收敛。( _ _ )
