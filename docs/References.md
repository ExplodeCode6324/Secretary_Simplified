# 来源与证据边界

## 1. 本轮依据

主来源为 Master 2026-09-14 的六步设想及本目录 [原始 Design2](archive/Design2-v0.1.md)。原稿 SHA-256：`81c528c11ca512c638d79c067df8e091fe50514ac3600d6c78630c6673f25ef5`。原稿保持原字节，正式入口为小写 design.md。

先前 Secretary_Simple 的 Mac/Go/SQLite 讨论和上下文回放经验用于继承权威数据与摘要分离、跨重启幂等、未知／冲突保留等原则。旧实验的文件布局和依赖可能已变化，本包不依赖其运行目录，也不将旧实验通过结果算作本项目完成。

先前较完整 Secretary 的审计用于检查稳定意图去重、最终许可、结果未知、固定验收条件、原子等待与事件循环。本次按单机实际需要落地这些约束，没有沿用它的复杂物理架构。

## 2. 本次核对的公开资料

| 来源 | 本文采用的内容 | 没有据此承诺的内容 |
|---|---|---|
| [MemGPT 原论文](https://arxiv.org/html/2310.08560v2) | 工作上下文、外部存储、按需检索及上下文管理的思路 | 无限正确记忆、长期自主运行可靠性 |
| [SQLite WAL](https://www.sqlite.org/wal.html) | 单机 WAL 与事务并发设计依据 | 外部副作用的 exactly-once |
| [SQLite PRAGMA](https://www.sqlite.org/pragma.html) | 显式外键、busy timeout、持久性配置 | 任意存储硬件绝不丢数据 |
| [SQLite Backup](https://www.sqlite.org/backup.html) | 一致性数据库快照 | 单独备份 db 就覆盖全部文件产物 |
| [modernc SQLite](https://pkg.go.dev/modernc.org/sqlite) | 纯 Go 驱动接口候选 | 旧实验版本仍是当前最佳版本 |
| [jsonschema Go](https://github.com/santhosh-tekuri/jsonschema) | Draft 2020-12 校验能力 | 已完成本项目 Go 集成 |

查阅日期：2026-09-14。具体模型服务商、模型 ID、版本、费用和音频后端不在本文冻结；实施时按实际授权配置核实，不能根据旧记忆中的订阅名称推断协议。

## Issue #2 当前实现依据

- [Issue #2](https://github.com/ExplodeCode6324/Secretary_Simplified/issues/2)：单一权威会话、TUI 与本轮限定验收。
- [Bubble Tea v1.3.10](https://github.com/charmbracelet/bubbletea/releases/tag/v1.3.10) 与 [Bubbles v0.21.0](https://github.com/charmbracelet/bubbles/releases/tag/v0.21.0)：异步终端 UI 与输入/滚动组件，锁定版本见 `src/go.mod`、`src/go.sum`。这是经 Ayanami 同意的直接依赖扩展，终端库不接管业务状态。
- `review/issue2-auth-design.response.md`、`review/issue2-auth-design-correction.response.md`、`review/issue2-tui-design.response.md`、`review/issue2-tui-design-correction.response.md`：实际 DeepSeek 的方案与生效更正。设计同意不等于运行验收。

终端渲染还直接使用 `github.com/charmbracelet/x/ansi v0.10.1`（原终端库传递依赖提升为直接依赖），按 cell 宽度换行/截断中文显示；版本锁定于 go.mod/go.sum。
