# AgentGate

**多租户 AI Agent 身份与安全执行平台 · Go / IAM / Agent Infrastructure**

共同制作：[@kimzclandi](https://github.com/kimzclandi) · [@Lu-Ricardo-Y](https://github.com/Lu-Ricardo-Y)。[作者与开源复用说明](AUTHORS.md)

Agent 可以提出行动，是否允许行动由后端决定。AgentGate 用一条真实的文档读取与工单更新链路，展示身份、委托、资源授权、人工审批和执行证据如何协作。

> 工程展示版本。默认使用确定性 mock，不需要模型费用。数据库读写、授权、审批和管理界面均真实运行。可选真实模型/OIDC 接入有实现，但没有外部凭证实测；不宣称生产认证系统或通用安全沙箱。

## 面试官从这里开始

1. [架构与核心链路](docs/ARCHITECTURE.md)：为什么选择模块化单体和本地事务。
2. [验证与性能证据](docs/TEST_REPORT.md)：命令、固定任务集、并发测试、测量口径和限制。
3. [威胁模型](docs/THREAT_MODEL.md)：授权边界和剩余风险。
4. [演示脚本](docs/DEMO.md)：复现读取、跨租户拒绝、审批、防重放、撤销和超时。
5. [面试讲解与 20 个追问](docs/INTERVIEW.md)。

GitHub 页面可直接阅读源码与报告。**仓库页面不是在线运行的后端**；管理控制台需按下文启动。没有部署付费云服务，也没有在公开页面放置开发令牌。

## 五分钟启动

环境：Go 1.27.1、Python 3.10+（仅演示脚本）、macOS 或 Linux。无需 Node、Docker、数据库服务或模型 API Key。

```sh
git clone https://github.com/kimzclandi/AgentGate.git
cd AgentGate
make run
```

打开 `http://127.0.0.1:8080`。另开终端，在仓库根目录执行：

```sh
AGENTGATE_DEV=1 ./bin/agentgate -token alice
```

将输出粘贴到控制台，连接后输入 `read-doc doc-1`，再输入 `update-ticket ticket-1 Resolved`，检查预览并点击确认。所有演示数据均是本地合成数据。

```sh
make check   # unit/integration + race + vet
make demo    # 独立临时数据库，完整 HTTP 验收，不修改 data/
make scan    # Go 可达漏洞扫描
make bench   # 三种口径，固定 GOMAXPROCS=4
```

## 已实现

| 能力 | 实现与证据 |
|---|---|
| 可信租户身份 | JWT 签名、算法、issuer/audience/exp/kid 校验；用户到租户的服务端映射 |
| 受限委托 | 用户角色 ∩ Agent 动作 ∩ 运行 scopes；固定工具受众、到期时间；无再次委托入口 |
| 统一授权 | 两种 SQLite 资源连接器；每次调用复查身份、资源所有者、租户、动作和运行 |
| 审批与幂等 | 参数规范化摘要、操作者绑定、到期、持久唯一键、事务确认；并发只写一次 |
| 撤销 | 无授权缓存；撤销事务提交后后续操作拒绝，已提交副作用不能撤回 |
| Runtime | 16 活跃运行、32 并发 API、8 步、1 秒工具时限、最长 300 秒运行、工作目录清理 |
| Agent | 确定性任务解析；可选 Chat Completions 兼容模型仅提出调用，不能直接执行或审批 |
| 管理界面 | 真实身份、工具、策略、审批、运行轨迹、当前用户审计 |

## 项目结构

```text
cmd/agentgate/         配置、启动、OIDC、进程锁、优雅退出
internal/gate/
  auth.go             开发 JWT 与 OIDC 验证
  policy.go           委托属性与集中判定
  engine.go           运行、提案、审批、撤销与事务
  connectors.go       文档/工单连接器契约及 SQLite 实现
  model.go            可选模型规划、受限 HTTPS 出口
  http.go             认证边界、请求校验、有界并发、管理 API
  migrations/         内嵌初始化迁移
  *_test.go           风险与业务行为测试
web/                  无构建依赖的真实管理控制台
scripts/demo.py       固定任务端到端验收
.github/workflows/    CI 配置
```

## 边界与当前状态

没有现有仓库，因此没有可继承的模型/RAG：本轮提供数据库文档访问，不虚称实现向量 RAG。未实现任意代码执行、远端业务写连接器、服务身份 OAuth client-credentials、通用审批管理员、跨运行子委托或多实例协调。外部模型没有真实端到端结果，OIDC 未对接真实 IdP。相关限制与续做条件见[进度清单](docs/PROGRESS.md)和[Runtime 设计](docs/RUNTIME.md)。

开发模式仅允许 loopback 绑定，令牌签发是本机开发便利能力。公开部署前需要真实 OIDC、TLS、身份映射、独立服务身份、外部审计与运营控制；不能把开发模式直接映射公网。

更多：[API / 配置](docs/API.md) · [权限设计](docs/PERMISSIONS.md) · [简历表述](docs/RESUME.md) · [安全报告](SECURITY.md)
