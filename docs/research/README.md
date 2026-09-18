# 源码研究与可重复接入探针

本目录是研究附件，不是已完成的 Secretary 应用。证据类别为 UPSTREAM_PROBE；真实模型请求和实际业务数据均未使用。

## 1. 从零还原源码

在仓库根运行；若 pi_src_origin 已存在，先检查其状态，不覆盖未提交改动。

```sh
git clone --no-checkout https://github.com/earendil-works/pi.git pi_src_origin
git -C pi_src_origin checkout --detach d981de1229ef899957bbe968bc8dcda02a21f477
git -C pi_src_origin status --short --branch
```

该 SHA 是 npm 0.85.1 的 gitHead 与 v0.85.1 tag。本次已经在本机检出完整源码；`.gitignore` 只避免外层仓库重复提交它，不影响本地阅读。

## 2. 运行探针与类型蓝图

需要宿主 npm 用于安装研究依赖，macOS 原生探针还需要 `/usr/bin/cc` 和 `/usr/bin/sandbox-exec`。本目录将固定 Node 24.21.0 安装为本地开发依赖，不替换系统 Node。

```sh
npm ci --prefix docs/research --no-audit --no-fund
npm test --prefix docs/research
npm run typecheck --prefix docs/research
```

脚本实际调用本目录锁定的 Node，而非宿主默认版本。所有提供方 transport 都是假 fetch；使用 dummy 字符串，不读取真实 API key。SQLite、OS 锁和沙箱探针只使用自动清理的合成临时目录。

`probes.test.mjs` 覆盖 P01—P12；结果见 [probe-output.txt](probe-output.txt)，类型检查见 [typecheck-output.txt](typecheck-output.txt)。非 macOS 的沙箱探针会显示 SKIP，不得当作通过。P12 仅证明最小允许/拒绝文件原语，不证明完整生产策略、网络、链接逃逸或进程生命周期已安全。

本次实际平台、退出状态、计数与证据 hash 记录于 [results.json](results.json)。文件变更后须重新运行并更新该记录，不能沿用旧 PASS。

可编译 [blueprint.ts](blueprint.ts) 展示 Agent 的真实构造签名与 awaited journal；[state-lock.c](state-lock.c) 是可迁移到 `native/state-lock.c` 的最小锁包装器。二者不是完整运行应用。

## 3. 已发现并处理的偏差

1. **源码 main 与发布包同号但接口不同。** 初评 `a8b3dd1` 使用 transcript system messages；发布 0.85.1 的 Context 保留 systemPrompt/messages/tools 顶层。最初 P08 错用 main 假设失败，记录见 [initial-probe-output.txt](initial-probe-output.txt)。已锁定发布 gitHead，并检查完整 Context，不隐藏失败。
2. **依赖声明的 optional peer 影响严格类型检查。** Pi AI 引入的 Google SDK 类型引用 MCP SDK，首次 typecheck 缺该模块，记录见 [initial-typecheck-output.txt](initial-typecheck-output.txt)。已显式固定开发依赖 `@modelcontextprotocol/sdk=1.25.2`，没有使用 skipLibCheck 掩盖问题；这不启用 MCP 服务。
3. **Pi beforeToolCall 后参数可改变。** P03 证明 execute 能看到修改后的值；Secretary execute 必须再次校验再触碰资源。
4. **直接 provider adapter 与 coding-agent 扩展语义不同。** P05/P06 验证直接 Responses adapter 的最终 payload 拒绝会阻止 fetch；本设计不用 coding-agent 的扩展异常处理作为门禁。
5. **deny-default 沙箱需要最小启动依赖。** 初始 P12 的进程因缺少启动所需权限而 SIGABRT，见 [initial-sandbox-probe-output.txt](initial-sandbox-probe-output.txt)。诊断后显式加入 executable mapping 和根目录本身的读取许可；`(literal "/")` 不递归允许根目录下的文件。最终同时断言允许文件成功、禁止文件由正常启动的 cat 以 EPERM 失败，避免把进程无法启动误判为隔离有效。完整命令运行时仍须在 P1 按解释器逐项验证最小策略。

## 4. 文件与证据归属

- [upstream-lock.json](upstream-lock.json)：源码 SHA、关键文件 hash、分发物 integrity。
- [source-map.json](source-map.json)：符号、真实行号及文件 hash，供后续实施定位。
- [package.json](package.json) / [package-lock.json](package-lock.json)：精确运行与类型依赖。
- [PiIntegration](../PiIntegration.md)：复用点和真实调用链。
- [ImplementationMap](../ImplementationMap.md)：需要新建的 Secretary 文件、接口和测试。

研究日志中的仓库绝对路径可被统一替换为 `<repo>`，不改动测试内容或结果。依赖下载与编译探针成功不代表业务持久化、真实模型质量、生产隔离或长期真实使用已验收。
