# 测试与性能报告

验证日期：2026-09-12。原始输出：[完整验证](verification-output.txt)、[HTTP 固定任务集](e2e-output.txt)、[benchmark](benchmark-output.txt)、[漏洞扫描详情](vulnerability-output.txt)。

## 环境与复现

macOS 26.6.2（25G83），darwin/arm64，Go 1.27.1，Apple M4（Go benchmark 检测）。当前权限无法读取 sysctl 内存信息，不编造 RAM 规格。SQLite 真实临时磁盘库、WAL、单连接；初始 3 用户/1 Agent/4 资源，无授权缓存。运行/审计行随 benchmark 扩大，未做固定大型租户压测。

```sh
make check
make demo
make bench
make scan
# 更详细地显示非可达模块告警：
go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 -show verbose ./...
```

结果：20 个顶层 Go 测试（含多组子用例）通过；race 和 vet 通过；22/22 个固定 HTTP 验收检查通过。22/22 表示检查符合预期，包含有意触发的 401/403/409，**不是 100% 真实模型业务成功率**。

## 风险覆盖

| 风险 | 证据 |
|---|---|
| 正常/拒绝、租户读写隔离 | TestAuthorizationAndTenantIsolation；HTTP 文档/工单用例 |
| 伪造/过期/算法/issuer/受众错误 | TestCredentials |
| 超范围委托、客户端租户覆盖 | TestDelegationCannotExpand；TestHTTPBoundary |
| 实时撤销/角色和 Agent 权限变化 | TestRevokeAfterApproval；TestLivePolicyChangeAndAudience |
| 参数替换/过期/重放/并发确认 | TestApprovalBindingAndReplay；TestExpiryCancelCleanupAndLimits；20 并发确认 |
| 幂等键冲突/并发提案 | TestIdempotency；TestConcurrentProposalAndCapacity |
| 非注册工具/提示注入 | TestAuthorizationAndTenantIsolation；TestUntrustedDataAndPaths |
| 路径输入/符号链接清理 | TestUntrustedDataAndPaths；TestCleanupDoesNotFollowSymlink |
| 网络限制/模型恶意参数 | TestNetworkPolicy；TestModelUntrustedAndNoRetry |
| 超时/取消/DB 不可用 | TestDependencyFailureAndContext；TestDatabaseWaitHonorsDeadline；TestModelTimeoutCancellation |
| 重启与审批持久化 | TestRestartRecovery；真实进程重启 HTTP 用例 |
| 单次审批写入的终态与回收 | TestSingleStepWriteCompletesRun |

管理 UI 在浏览器实际验证：认证、文档返回、工单提案、参数预览、点击确认、成功状态与审计展示。源码中所有结果来自 fetch API；没有硬编码执行成功。未声称跨浏览器完整 UI 自动回归。

## 依赖扫描判断

首次扫描发现 GO-2026-4945（go-jose v4.1.3），已升级到 v4.1.4 并复检。当前 0 个可达符号/导入包漏洞。

仍有模块级 GO-2026-5024：x/sys v0.37.0 的 Windows NewNTUnicodeString，修复版本 v0.44.0。当前仅支持/验证 macOS 与 Linux，不导入该 Windows 路径，因此报告为不可达，非本轮阻断项。未来支持 Windows 前应升级并补全替代 flock 的进程锁方案。不把“0 个可达漏洞”写成所有依赖不存在漏洞。

## 性能：三种独立口径

`GOMAXPROCS=4`，每项 2 秒目标时长，连续 3 次。测试使用 Go testing.B 自适应迭代，非固定请求总数。

| 测量 | 本轮范围 | 中位数 | 含义 |
|---|---:|---:|---|
| Evaluate 纯策略 | 9.322–10.03 ns/op | 9.336 ns/op | 内存属性、无 I/O，steps 在 0..8 循环，包含拒绝；0 分配 |
| 完整鉴权 handler | 15,718–15,765 ns/op | 15,723 ns/op | httptest + JWT 验证 + 用户 DB 查询，4 个并发 worker 的摊销时间；约 63.6k op/s 聚合吞吐 |
| 单步 mock Agent | 450,819–458,521 ns/op | 451,875 ns/op | 串行建运行/目录、DB 授权读取/审计、终态/清理；约 2.21k run/s |

完整鉴权是 handler 内存 HTTP 测试，未含 socket/TLS/代理/公网。并发 ns/op 不是单请求 wall-clock 延迟，也不是 P95/P99。纯策略使用极小热数据集，不能外推复杂 ACL 或多租户生产策略。

没有真实模型耗时、token/费用指标、远端连接器压测、长期存储/内存曲线或可用性 SLA。Docker 未运行（不执行任意代码，当前架构不依赖 Docker）。

## 自审修复

- 签发 token CLI 不打开数据库，避免误触服务重启恢复。
- 服务启动加独占 flock，避免多个进程共用数据目录并将活跃任务误标 interrupted。
- 修复可达 JOSE 漏洞依赖。
- 单步审批写成功后，业务/审批/运行成功在事务内同步，目录清理有 sweeper 重试。
- 审计 policy_version 从事务内真实版本读取；HTTP context 贯穿 request ID。
- 纯策略 benchmark 使用实际 Evaluate 和变化输入，替换会被编译器消除的恒定角色检查；旧数字不作为结果。

## 未验证 / 未完成

外部模型与真实 IdP 缺配置/凭证，不发起付费调用；OAuth 服务身份、生产 Web 登录与部署、远端写 unknown 对账、任意代码隔离未实现。CI 配置随仓库提供，GitHub 运行结果需单独确认，不能用本地通过代替远端 CI。
