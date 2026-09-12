# 本地自然语言 Agent

本地模型负责理解任务、提出工具调用、根据真实结果回答。Go 后端负责身份、权限、审批与数据事务。无需云端 API Key；模型下载、磁盘、内存和推理算力由本机提供。GitHub 提供源码，不能直接运行 Go 后端或替浏览者提供本地推理。

## 启动

1. 安装 [Ollama](https://ollama.com/download)。本轮验证版本为 0.34.0。
2. 独立终端运行 `OLLAMA_NO_CLOUD=1 OLLAMA_HOST=127.0.0.1:11434 ollama serve`。若已有 Ollama 服务，先从其原有启动方式停止，避免端口冲突。
3. `ollama pull qwen3:1.7b`；模型约 1.36 GB，支持工具调用。下载完成后无需云端模型服务。
4. 仓库根目录运行 `OLLAMA_MODEL=qwen3:1.7b make run`。
5. 另开终端运行 `AGENTGATE_DEV=1 ./bin/agentgate -token alice`，将本地开发令牌粘贴进控制台连接。令牌不要提交到仓库。
6. 输入“请分别读取 doc-1 和 ticket-1，读取两条真实记录后，用中文总结。”，展开工具调用与返回查看真实数据。
7. 输入“请把 ticket-1 的内容更新为 Resolved by local Agent，然后告诉我执行结果。”，核对待审批参数，确认执行，再点击“审批后继续回答”。

运行总有效期 300 秒；每个审批最长 120 秒，且不超过运行剩余有效期。超时或服务重启后，旧对话仅供查看，需要新建任务；已提交的写入不会因为最终回答失败而撤销。

## CPU 兼容运行

本轮 macOS 受限执行环境无法初始化 Metal command queue。Ollama 的 `num_gpu:0` 单独设置仍失败；针对本次 0.34.0 所带 llama-server，以下官方命令帮助中列出的环境参数成功关闭设备卸载。停止原 Ollama 服务，再运行：

```sh
OLLAMA_NO_CLOUD=1 OLLAMA_HOST=127.0.0.1:11434 \
OLLAMA_NUM_PARALLEL=1 OLLAMA_MAX_LOADED_MODELS=1 OLLAMA_MAX_QUEUE=2 \
LLAMA_ARG_DEVICE=none LLAMA_ARG_N_GPU_LAYERS=0 LLAMA_ARG_FIT=off \
ollama serve
```

这是已验证版本的兼容方法，不保证其他版本参数相同。正常桌面环境可先使用默认硬件加速。AgentGate 不会修改系统 Ollama 配置或自动下载模型。

## 复现与证据

Ollama 启动并下载模型后执行：

```sh
OLLAMA_MODEL=qwen3:1.7b make local-demo
```

脚本新建临时数据库和独立服务端口，不修改正常 data/。测试脚本以开发用户对合成工单执行明确审批，应用和模型自身不能跳过审批。断言包括真实工具返回、审批前数据库未变、审批后仅增加一个版本、恢复重放拒绝和跨租户拒绝。模型如果选错工具、直接声称完成或拒绝调用，脚本会失败，不把 HTTP 200 当业务验收成功。

最新原始输出：[local-model-results.json](evidence/local-model-results.json)。本机 Apple M4，Ollama 报告系统内存 16 GiB；CPU 推理。模型 digest：`8f68893c685c3ddff2aa3fffce2aa60a30bb2da65ca488b61fff134a4d1730e7`。首个冷启动文档读取约 3.85 秒；这不是吞吐基准或 SLA。精确复现仍可能受模型版本、采样、已有上下文和平台影响。

## 实际发现的限制

- 最初 Metal 初始化失败，未作为成功推理记录；采用上述 CPU 设置后跑通。
- 第一轮工具描述只有动作名，小模型曾用 document_read 查询 ticket，后端拒绝；另一次读错文档后回答工单不可用。
- 补充工具说明后，多资源查询成功，但模型曾根据已有内容直接声称更新成功。进一步加入读写区别提示，并在独立初始数据上验证完整审批。提示词能改善行为，不能保证任何输入都正确。
- 本轮固定任务通过不等于开放任务准确率。原始工具结果可核查；自然语言概括仍可能误译、遗漏或夸大。status=succeeded 仅表示流程结束。
- 小模型可完成文档/工单流程，当前没有联网搜索、向量 RAG、任意代码、自动任务分解评测或多 Agent 协作。
- 本地服务为受信任依赖；敏感内容会进入本机模型及 SQLite 对话记录。模型权重、密钥、实际数据库不随源码发布。
- 远端付费模型和真实 OIDC IdP 仍未完成外部集成验证。

协议参考：[Ollama 工具调用](https://docs.ollama.com/capabilities/tool-calling)、[Chat API](https://docs.ollama.com/api/chat)。
