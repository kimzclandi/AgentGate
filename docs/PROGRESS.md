# AgentGate 实施与验收记录

## 项目状态

AgentGate 是 Go 多租户 Agent 身份与工具执行平台，仓库位于 kimzclandi/AgentGate，由 kimzclandi 与 Lu-Ricardo-Y 共同制作。采用单实例 Go 控制面、SQLite 文档与工单连接器、管理控制台和可选模型规划器。

开发与验证环境：Go 1.27.1，macOS/arm64；GitHub CI 使用 Linux。当前仅执行内置工具，不依赖 Docker。

## 已实现且已验证

- [x] Go 单体 + SQLite 持久化、内嵌 v1 迁移、开发种子数据。
- [x] JWT 完整验证、可信租户映射、用户/Agent/运行 scopes 交集、固定委托受众与期限。
- [x] 文档/工单两种连接器统一授权入口，真实读取和人工审批写入。
- [x] 审批参数/身份/运行绑定，过期/重放/并发和幂等测试。
- [x] 每次复查实时权限；撤销和业务写入同一 DB 串行化，缓存为零。
- [x] Runtime 并发/步骤/时限/输出限制、取消、独立目录、重启恢复和清理测试。
- [x] 浏览器真实连接、读取、提案、确认、状态与审计显示。
- [x] 20 个顶层 Go 测试，race、vet；固定 HTTP 集 22/22 检查符合预期。
- [x] 0 可达漏洞；修复 go-jose；未调用 Windows 模块告警有判断记录。
- [x] 三种独立口径 benchmark 与原始输出、技术与使用文档。
- [x] 最终本地自审修复：token CLI 误恢复风险、独占锁、单步写任务终态、实际策略版本/request ID。

## 有实现但未经外部验证

- OIDC discovery/JWKS 验证适配：缺真实 issuer/client 与预置 subject 映射；未完成 Web 登录/服务身份流程。
- 真实模型兼容接口：有结构化解析、拒绝恶意参数、网络策略、超时和不重试测试；未用真实 API Key 发起付费 E2E。

## 本轮明确未实现（不标记为完成）

服务身份 OAuth client-credentials、完整生产登录/部署、向量 RAG、任意代码沙箱、远端业务写入及 unknown 对账、多实例运行、策略编辑 API、完整 metrics/清理率/租户配额、不可篡改审计。这些能力列入后续开发范围；当前版本的部署边界见威胁模型。

## 发布与续做

- [x] 源码/文档发布到 kimzclandi/AgentGate 并核对远端。
- [x] 更新 kimzclandi 个人主页导航，保留原有项目内容。
- [x] 检查远端 CI：Linux make check / make demo / make scan 全部通过。

独立继续：先读本文件、TEST_REPORT 与 THREAT_MODEL。真实模型最小输入为提供商兼容 endpoint、允许主机、模型名和私下配置的 key；只在用户授权费用后运行固定任务。OIDC 最小输入为测试 issuer/client ID 和 subject 映射；补登录与服务身份后再进行部署评审。公开托管后端仍需另行确定运行环境，本轮不购买云资源。

发布证据：代码提交 a9a754686490963e547f80fcbf66aeddd761d22d，个人主页提交 890e41a8f66f6640e487f9d32eda1218826e677c。首轮 Linux CI 测试/race/vet/demo 通过，但旧 govulncheck v1.1.4 的 SSA 解析崩溃；已升级到 v1.8.0，本地扫描通过，远端第二轮已通过。

最终远端证据：[CI 34697008208](https://github.com/kimzclandi/AgentGate/actions/runs/34697008208)，验证代码/工具配置提交 a672ea3340a8115e5c4cdd64faa28d9ce57c577f。后续仅调整文档与界面文案。
