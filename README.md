# AgentGate

**简体中文** | [English](README.en.md)

[![verify](https://github.com/kimzclandi/AgentGate/actions/workflows/ci.yml/badge.svg)](https://github.com/kimzclandi/AgentGate/actions/workflows/ci.yml)

**用 Go 为 AI Agent 的文档读取和工单修改增加身份校验、资源权限与人工审批。**

用户可以说“读取我的文档和工单并总结”，由本地模型调用工具获取真实数据；也可以要求修改工单，先核对后端生成的参数预览，确认后执行，再由模型根据执行结果回答。模型无权批准自己的操作。

代码许可证尚待共同作者决定；公开源码不等于已授予开源许可证。

共同作者：[@kimzclandi](https://github.com/kimzclandi) · [@Lu-Ricardo-Y](https://github.com/Lu-Ricardo-Y)。[贡献与依赖说明](AUTHORS.md)

## 当前能做什么

| 能力 | 实现与边界 |
|---|---|
| 自然语言与多步工具调用 | Ollama 本地模型，已用 qwen3:1.7b 验证；最多 9 轮模型调用、8 次工具提案 |
| 文档 / 工单读写 | 三个内置工具、真实 SQLite 数据；初始为四条合成资源，未对接外部工单系统 |
| 身份与委托 | 用户、Agent、租户、运行分离；角色与资源 owner 约束；开发 JWT，另有未经真实 IdP 验证的 OIDC 适配 |
| 写操作审批 | 参数摘要绑定、原发起人确认、实时复查权限、事务执行和重放保护 |
| 运行与审计 | 持久对话、取消、撤销、超时、重启中断、租户隔离的运行及审计记录 |
| 管理控制台 | 真实后端 API，显示答案、工具轨迹、待审批参数及运行状态 |

当前定位是**单实例 Agent 安全执行参考实现**。本地模型已验证，远端兼容接口仅支持单步规划。模型理解和概括仍可能出错，`succeeded` 表示流程完成，不代表答案已通过事实评测。

## 运行真实本地 Agent

需要 Go 1.27.1、macOS 或 Linux，以及 [Ollama](https://ollama.com/download)。Go 安装见[官方说明](https://go.dev/doc/install)。首次模型下载需要网络和约 1.36 GB 磁盘空间，无需购买云端 API。

```sh
git clone https://github.com/kimzclandi/AgentGate.git
cd AgentGate
```

终端一启动 Ollama（若已有服务，先按[本地指南](docs/LOCAL_MODEL.md)处理端口冲突）：

```sh
OLLAMA_NO_CLOUD=1 OLLAMA_HOST=127.0.0.1:11434 ollama serve
```

终端二，在仓库根目录执行：

```sh
ollama pull qwen3:1.7b
OLLAMA_MODEL=qwen3:1.7b make run
```

终端三，在同一仓库根目录签发开发凭证：

```sh
AGENTGATE_DEV=1 ./bin/agentgate -token alice
```

打开 `http://127.0.0.1:8080`，粘贴凭证并连接，输入：

```text
请分别读取 doc-1 和 ticket-1，读取两条真实记录后，用中文总结。
```

展开“工具调用与返回”，应看到 `document_read` 与 `ticket_read` 的真实返回。初始工单内容为 `Open: customer needs help`。

再输入：

```text
请把 ticket-1 的内容更新为 Resolved by local Agent，然后告诉我执行结果。
```

核对待审批参数 → 确认执行 → 点击“审批后继续回答”。审批最长 120 秒且不超过运行剩余时间；运行总有效期 300 秒。请求跨租户资源 `doc-2` 会由后端拒绝。

模型启动失败、CPU 兼容设置与真实测试记录见[本地模型指南](docs/LOCAL_MODEL.md)。完成体验后在服务终端按 Ctrl+C 停止进程；数据保留在 `data/`。

## 不加载模型也能检查后端

仅运行 `make run`，连接后使用固定命令 `read-doc doc-1`、`read-ticket ticket-1`、`update-ticket ticket-1 Resolved`。这是明确标识的确定性测试模式，不是自然语言推理。

可选远端模型配置见 [.env.example](.env.example) 与 [API 文档](docs/API.md)。程序不会自动读取 `.env`，也不会自动向付费模型发送任务。

## 验证

Python 3.10+ 用于 HTTP/文档检查，Node.js 22 用于控制台状态回归；它们不是 Go 服务的运行依赖。

```sh
make check            # Go 测试、race、vet
make ui-test docs-check
make demo             # 临时数据库上的固定 HTTP 验收
make scan             # 当前平台可达漏洞扫描
make local-demo       # 需正在运行的 Ollama；检查真实工具与写入结果
```

验收范围、历史性能数据和未验证项集中在[测试报告](docs/TEST_REPORT.md)。原始记录位于 [docs/evidence](docs/evidence/README.md)，不把模拟测试或少量固定任务外推成生产可靠性。

## 代码与文档导航

```text
cmd/agentgate/         配置、认证初始化、进程锁与退出
internal/gate/        授权、事务审批、模型适配、对话及数据迁移
web/                 无框架管理控制台
scripts/             HTTP、真实模型、UI 状态与文档检查
```

- 理解执行流程：[架构](docs/ARCHITECTURE.md) · [权限](docs/PERMISSIONS.md) · [Runtime](docs/RUNTIME.md)
- 使用和配置：[使用指南](docs/DEMO.md) · [API](docs/API.md) · [本地模型](docs/LOCAL_MODEL.md)
- 检查完成度：[测试报告](docs/TEST_REPORT.md) · [项目状态](docs/PROGRESS.md) · [审查记录](docs/REVIEW.md)
- 了解边界：[威胁模型](docs/THREAT_MODEL.md) · [安全说明](SECURITY.md) · [贡献指南](CONTRIBUTING.md)

尚未实现生产 Web 登录、OAuth 服务身份、多实例协调、向量 RAG、任意代码隔离和外部业务写入对账。GitHub 提供代码和文档，浏览者在线使用后端还需要独立部署；开发认证仅允许本机访问。

[2026-09-22 implementation and verification](docs/maintenance/2026-09-22/README.md)

[2026-09-22 detail review and regression fixes](docs/maintenance/2026-09-22-detail/README.md)

Further review: [2026-09-22 evidence and export hardening](docs/maintenance/2026-09-22-readiness/README.md).
