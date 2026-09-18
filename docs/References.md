# 证据与参考

## 本项目

- 起点：`1167fae73dcb6def987a9bbe190a167407b5ce8c`，[原计划](archive/PLAN-2026-09-17.md)。
- [设计前评审](InitialReview.md)：R1—R8 与 T15—T26 的来源；本文设计不是对旧实现的恢复。

## Pi 固定源码快照

实际接入基线是发布提交 `d981de1229ef899957bbe968bc8dcda02a21f477` / tag `v0.85.1`，与 npm 包 gitHead 对齐；见 [源码锁](research/upstream-lock.json)、[发布版调用链](PiIntegration.md) 和 [精确符号位置](research/source-map.json)。下面保留的 `a8b3dd1` 链接是初评 main 快照，部分 Context 接口与发布版不同，不能将其签名作为当前运行实现依据。

- [Agent 构造和运行接口](https://github.com/earendil-works/pi/blob/a8b3dd19983883be2907253e566dd52989aee936/packages/agent/src/agent.ts)：可传入流函数和工具，事件监听 Promise 被等待，工具默认并行。本设计显式串行工具。
- [流函数契约](https://github.com/earendil-works/pi/blob/a8b3dd19983883be2907253e566dd52989aee936/packages/agent/src/types.ts)：错误以流终态编码；本设计的网关拒绝应返回错误流，且真实发送数为零。
- [底层循环](https://github.com/earendil-works/pi/blob/a8b3dd19983883be2907253e566dd52989aee936/packages/agent/src/agent-loop.ts)：上下文转换和执行工具的顺序。
- [SDK 扩展执行器](https://github.com/earendil-works/pi/blob/a8b3dd19983883be2907253e566dd52989aee936/packages/coding-agent/src/core/extensions/runner.ts)：部分上下文/provider 钩子的异常会被记录后继续；不作为硬门禁。
- [SDK 会话分发](https://github.com/earendil-works/pi/blob/a8b3dd19983883be2907253e566dd52989aee936/packages/coding-agent/src/core/agent-session.ts) 与 [会话文件](https://github.com/earendil-works/pi/blob/a8b3dd19983883be2907253e566dd52989aee936/packages/coding-agent/src/core/session-manager.ts)：与业务确认不同的持久边界。
- [默认读取](https://github.com/earendil-works/pi/blob/a8b3dd19983883be2907253e566dd52989aee936/packages/coding-agent/src/core/tools/read.ts)、[shell](https://github.com/earendil-works/pi/blob/a8b3dd19983883be2907253e566dd52989aee936/packages/coding-agent/src/core/tools/bash.ts)、[沙箱示例](https://github.com/earendil-works/pi/blob/a8b3dd19983883be2907253e566dd52989aee936/packages/coding-agent/examples/extensions/sandbox/index.ts)：可参考接口，不复制未启用隔离时的宿主执行回退。

上述链接是初评静态证据；随后已在发布版运行 [接入探针](research/README.md)，应用 P1 仍需实现自有适配层再验收。运行版本升级必须重新核对相关接口和门槛。

## 存储与标准

- [SQLite PRAGMA](https://www.sqlite.org/pragma.html)：事务连接参数的官方依据。
- [SQLite Online Backup API](https://www.sqlite.org/backup.html)：生成一致数据库快照；对象目录仍需本项目额外协调。
- [Node SQLite API](https://nodejs.org/api/sqlite.html)：P1 按实际锁定 Node 版本核对；不能把 Python SQLite 附件检查当作 Node 驱动验证。
- [JSON Schema 2020-12](https://json-schema.org/draft/2020-12)：机器对象契约。format 检查须显式启用。
