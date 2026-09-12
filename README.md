# AgentGate

[![verify](https://github.com/kimzclandi/AgentGate/actions/workflows/ci.yml/badge.svg)](https://github.com/kimzclandi/AgentGate/actions/workflows/ci.yml)

**基于 Go 的多租户 Agent 身份与工具执行平台。**

AgentGate 为文档访问和工单处理提供统一的 Agent 执行流程：用户提交任务，Agent 提出工具调用，后端校验身份与资源权限，高风险操作经人工确认后执行，并记录运行状态和审计事件。

共同作者：[@kimzclandi](https://github.com/kimzclandi) · [@Lu-Ricardo-Y](https://github.com/Lu-Ricardo-Y)

## 功能

- **Agent 任务执行**：支持文档读取、工单查询与更新；支持 Ollama 本地模型理解自然语言、多次工具调用、审批后继续回答；另保留固定命令测试和远端单步模型接口。
- **身份与委托**：区分用户、Agent 和运行；运行权限受用户角色、Agent 允许动作与请求范围共同约束。
- **资源访问控制**：每次工具调用检查租户、资源所有者、动作、身份状态和运行有效期。
- **人工审批**：工单更新先生成参数预览；审批绑定用户、租户、运行、工具与参数摘要。
- **幂等与撤销**：持久幂等键、事务确认和重放保护；身份撤销后阻止后续受保护操作。
- **运行管理**：步骤、并发、时限和输出限制，支持取消、重启恢复与工作目录清理。
- **管理控制台**：查看身份、工具、策略、待审批动作、运行轨迹和审计记录。

文档与工单连接器使用真实 SQLite 数据库。模型只提出调用，授权和审批始终由后端执行。

## 快速开始

环境：Go 1.27.1、macOS 或 Linux。Python 3.10+ 仅用于端到端脚本；本地启动无需 Docker 或独立数据库服务。

```sh
git clone https://github.com/kimzclandi/AgentGate.git
cd AgentGate
make run
```

打开 `http://127.0.0.1:8080`。另开终端，在仓库根目录签发本地开发令牌：

```sh
AGENTGATE_DEV=1 ./bin/agentgate -token alice
```

将输出粘贴到控制台并连接。输入以下任务：

```text
read-doc doc-1
read-ticket ticket-1
update-ticket ticket-1 Resolved
```

读取任务直接返回数据库内容；更新任务会出现在待审批列表，确认参数后执行。默认数据是本地合成数据。

## 不购买 API：本地模型运行

安装 [Ollama](https://ollama.com/download)，在独立终端启动本地服务：

```sh
OLLAMA_NO_CLOUD=1 OLLAMA_HOST=127.0.0.1:11434 ollama serve
```

另开终端下载模型、启动 AgentGate（先停止之前的 `make run`）：

```sh
ollama pull qwen3:1.7b
OLLAMA_MODEL=qwen3:1.7b make run
```

连接控制台后选择“本地模型 · 自然语言”，输入：

- 请分别读取 doc-1 和 ticket-1，读取两条真实记录后，用中文总结。
- 请把 ticket-1 的内容更新为 Resolved by local Agent，然后告诉我执行结果。

写入先停在待审批状态。核对参数并确认执行，再点击“审批后继续回答”。实际运行需要电脑提供算力，但无需购买云端模型 API。模型首次下载需要网络，下载后本地推理不需要云端密钥。

已在本机用 Ollama 0.34.0 + qwen3:1.7b 跑通读取、多工具查询、审批及写后查询。安装、CPU 兼容设置、复现与限制见[本地模型指南](docs/LOCAL_MODEL.md)。

## 可选远端模型接入

未设置 OLLAMA_MODEL 时，控制台使用固定命令测试模式。远端模型入口为 `POST /api/agent/remote`，请求格式为 `{"task":"读取 doc-1 文档"}`，需要身份凭证并显式配置：

```sh
export MODEL_ENDPOINT=https://your-provider.example/v1/chat/completions
export MODEL_ALLOWED_HOST=your-provider.example
export MODEL_NAME=your-supported-model
# MODEL_API_KEY 通过私有环境变量或密钥管理服务设置
make run
```

模型客户端只允许配置中的 HTTPS 主机，拒绝私网目标和重定向。模型响应经结构化校验后进入相同授权与审批流程。配置详见 [.env.example](.env.example) 和 [API 文档](docs/API.md)。

## 开发与验证

```sh
make check   # Go 测试、race、vet
make demo    # 独立临时数据库上的完整 HTTP 流程
make scan   # 可达漏洞扫描
make bench  # 策略、鉴权请求与 Agent 运行三种测量
```

测试覆盖租户隔离、委托范围、凭证校验、审批替换与重放、并发确认、权限撤销、运行超时/取消和重启恢复。执行记录及测量口径见[测试报告](docs/TEST_REPORT.md)。

## 目录

```text
cmd/agentgate/         配置、启动、OIDC、进程锁、优雅退出
internal/gate/
  auth.go             本地 JWT 与 OIDC 验证
  policy.go           委托属性与集中授权
  engine.go           运行、提案、审批、撤销与事务
  connectors.go       文档/工单连接器与 SQLite 实现
  model.go            远端单步规划与受限 HTTPS 出口
  ollama.go           本地模型与工具调用协议
  chat.go             多步对话、持久化、审批恢复与取消
  http.go             HTTP API、请求校验与并发控制
  migrations/         内嵌数据库迁移
  *_test.go           单元与集成测试
web/                  管理控制台
scripts/demo.py       端到端流程验证
.github/workflows/    持续集成
```

## 文档

- [架构与执行流程](docs/ARCHITECTURE.md)
- [身份、委托与权限](docs/PERMISSIONS.md)
- [Runtime 与网络限制](docs/RUNTIME.md)
- [API、配置与数据库操作](docs/API.md)
- [使用指南](docs/DEMO.md)
- [威胁模型](docs/THREAT_MODEL.md)与[安全说明](SECURITY.md)
- [测试与性能报告](docs/TEST_REPORT.md)
- [开发状态](docs/PROGRESS.md)与[作者及依赖](AUTHORS.md)

## 当前边界

当前为单实例、受信任内置工具执行架构。已验证本地模型，远端付费模型与真实 OIDC IdP 尚未外部集成验证。小模型可能选错工具或回答不准确；运行成功表示流程完成，不保证回答语义正确。尚不支持任意代码执行、向量 RAG、远端业务写入对账或多实例协调。

开发认证仅允许 loopback 绑定。对外部署需要配置真实身份入口、TLS、租户配额、密钥管理和审计保留策略。GitHub 仓库提供代码与文档，运行中的后端需要独立部署。
