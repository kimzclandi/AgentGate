# 贡献指南

先阅读 README、docs/ARCHITECTURE.md 和 docs/THREAT_MODEL.md，再围绕具体问题提交小范围变更。

## 开发检查

使用 Go 1.27.1、Python 3.10+、Node.js 22：

```sh
make check ui-test docs-check demo scan
```

改变本地模型协议或工具行为时，还需在 Ollama 启动后运行 `make local-demo`。这个命令只修改临时合成数据；不会使用正常 data/。没有本地模型时，请明确记录未执行该项，不使用 mock 输出替代。

## 变更要求

- 说明具体问题、改变后的行为和验证结果。
- 新业务工具必须经过统一授权入口；模型不能直接访问数据库或审批接口。
- 数据迁移须保持已有数据；涉及状态机时补充取消、失败、重放及权限检查。
- 文档说明须与最终实现一致。性能数据注明版本、环境和测量范围。
- 不提交凭证、数据库、模型权重或用户数据；配置仅提供占位符。

仓库尚未选定项目代码许可证；许可证需要共同作者确认，本指南不授予额外使用许可。依赖仍遵循各自许可证。

## Community and documentation checks

Use the issue forms for reproducible bugs and scoped feature requests. Include actual
validation results in the PR template. Run `python3 .github/scripts/check_docs.py`
after editing entry-point documents. See [community conduct](CODE_OF_CONDUCT.md) and
the [maintenance guide](docs/MAINTAINING.md).
