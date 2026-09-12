# 验证证据索引

原始输出与当前说明分开存放。文件中的运行 ID、用户及内容来自合成开发数据，不包含开发令牌或模型密钥。历史输出保留原样，不用新代码覆盖旧测量结果。

| 文件 | 验证范围 / 版本 |
|---|---|
| [review-verification.txt](review-verification.txt) | 本次边界修复后的 Go、UI、文档、固定 HTTP 与扫描结果 |
| [verification-output.txt](verification-output.txt)、[e2e-output.txt](e2e-output.txt)、[clean-start-output.txt](clean-start-output.txt) | 初始版本的 20 个 Go 测试与 22 项 HTTP 检查；非当前测试计数 |
| [static-output.txt](static-output.txt)、[vulnerability-output.txt](vulnerability-output.txt) | 初始静态检查与依赖告警详情，不能代替最新扫描 |
| [benchmark-output.txt](benchmark-output.txt) | 初始版本三类 benchmark；未作为当前版本性能结果重新标注 |
| [local-development-tests.txt](local-development-tests.txt) | 206abe4 本地模型升级：26 个 Go 测试及固定 HTTP/扫描 |
| [local-model-results.json](local-model-results.json) | 206abe4，Ollama 0.34.0 / qwen3:1.7b CPU 真实固定任务与工具返回 |

历史 CI：[a672ea3](https://github.com/kimzclandi/AgentGate/actions/runs/34697008208)、[206abe4](https://github.com/kimzclandi/AgentGate/actions/runs/34698847056)。当前分支 CI 见 [Actions](https://github.com/kimzclandi/AgentGate/actions/workflows/ci.yml)。CI 不下载模型，真实本地推理与模拟边界测试的证据分别记录。
