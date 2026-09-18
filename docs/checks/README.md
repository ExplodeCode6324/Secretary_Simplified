# 设计附件检查

在仓库根运行：

```sh
CHECK_VENV="${TMPDIR:-/tmp}secretary-design-check"
python3 -m venv "$CHECK_VENV"
"$CHECK_VENV/bin/python" -m pip install -r docs/checks/requirements.txt
"$CHECK_VENV/bin/python" docs/checks/validate_design.py
```

先按 [研究环境](../research/README.md) 获取固定 Pi 源码；检查器核对真实源码 hash。检查器只在临时目录创建 SQLite，报告写 [latest-report.json](latest-report.json)。

检查范围：JSON Schema/format 与正反例、主 DTO/工具覆盖、文档相对链接、阶段与状态机、源码及分发物锁、SQLite 的引用/唯一性/不可变记录/预算/回滚/备份约束。

若检查失败，修复前另存失败报告，后续运行会更新 latest-report.json。本轮附件检查结果见上述报告；研究探针的初次失败另存于 research。最终 PASS 仅代表 DOC_ONLY；Pi 接入探针是独立研究结果，应用验收仍为 NOT_RUN。
