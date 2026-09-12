# 架构与决策

AgentGate 是模块化 Go 单体，不是微服务集合。浏览器与模型均不持有数据库访问权。只有内置受信任代码能访问 Store；不支持装载不可信插件。当前 SQLite 单连接、进程锁约束为单实例运行。

```mermaid
flowchart LR
 U[用户 / 管理 UI] --> H[HTTP 认证与校验]
 H --> A[Agent 任务解析 / 可选模型规划]
 A --> E[统一工具入口 PEP]
 H --> E
 E --> P[资源授权 PDP]
 P --> DB[(SQLite 身份 / 委托 / 资源)]
 E --> Q[参数绑定审批]
 Q --> P
 Q --> C[文档 / 工单连接器]
 E --> C
 C --> DB
 E --> L[审计与运行轨迹]
```

```mermaid
sequenceDiagram
 participant U as 已认证用户
 participant G as Go 控制面
 participant M as mock / 可选模型
 participant D as SQLite
 U->>G: 创建任务
 G->>M: 提出一个结构化工具调用
 M-->>G: 工具名与参数（不可信）
 G->>D: 创建受限运行 / 检查实时权限
 alt 文档或工单读取
 G->>D: 授权事务内读取并追加审计
 G-->>U: 数据与运行结果
 else 工单更新
 G->>D: 持久化参数摘要和待审批动作
 G-->>U: 参数预览、摘要、有效期
 U->>G: action_id + digest + 身份凭证
 G->>D: 同一事务复查权限、审批和运行
 G->>D: 写入工单 + 消耗审批 + 成功审计
 G-->>U: 更新成功，单步运行完成
 end
```

## 模块责任

- `auth.go`：已验证 subject 映射到 DB identity；不信任请求 tenant_id。
- `policy.go`：默认拒绝，检查角色、动作、Agent、资源所有者、租户、运行期限/步骤。`Evaluate` 为纯计算部分，`authorize` 在事务中加载真实属性。
- `engine.go`：唯一公开业务入口，控制审批与副作用的原子状态变更。
- `connectors.go`：两种资源类型的连接器契约。当前均使用真实 SQLite，未模拟第三方 SaaS。
- `model.go`：可选外部规划器无 DB、用户凭证或审批接口；模型仅选择一个工具，不循环解析工具结果。
- `http.go`：schema 等效的严格 Go 字段校验、请求大小、认证、有界并发、分页、错误映射与 HTTP request ID。

## 为什么事务，而不是分布式 exactly-once

本地资源与审批在同一 DB。串行事务同时解决确认重放、权限变更竞态和写入原子性。HTTP 响应丢失时，通过审批状态查询确认结果，不通过盲目重放确认推断失败。远端副作用不在此事务中，本轮不接入远端业务写入。

## 本轮未采用

无模型训练/微调、向量 RAG、MCP、消息队列、Kubernetes、多 Agent 或任意代码执行。没有原有仓库，无法声称保留了现成 Agent 能力。工程扩展应先解决真实场景，再引入独立 worker/outbox。
