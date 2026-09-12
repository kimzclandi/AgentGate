# API、配置与数据操作

所有 `/api/*` 需要 `Authorization: Bearer <token>`，不使用 cookie。请求 `Content-Type: application/json`；未知字段、额外 JSON 文档和超限输入拒绝。响应带 `X-Request-ID`，错误 JSON 包含稳定 error 与 request_id。不接受 tenant_id，由认证 subject 在 DB 映射租户。

| 方法 / 路径 | 输入 / 作用 |
|---|---|
| GET /healthz | 存活；不代表 DB 可用 |
| GET /readyz | DB ping，失败 503 |
| GET /api/me | 当前身份 |
| GET /api/tools | 名称、输入 Schema、动作、类型定位、风险、超时、副作用 |
| POST /api/runs | `{"agent":"assistant","scopes":["document:read"],"ttl_seconds":60}` |
| POST /api/tools/call | `{"run_id":"…","tool":"document.read","params":{"resource_id":"doc-1"}}` |
| POST /api/tools/call | 写入用 ticket.update，加 body 和 idempotency_key；仅创建预览 |
| POST /api/approve | `{"action_id":"…","digest":"…"}`；需原发起者认证 |
| POST /api/cancel | `{"run_id":"…"}` |
| POST /api/revoke | `{}`；仅撤销自己，影响全部自己的活跃运行 |
| POST /api/agent | `{"task":"read-doc doc-1"}`，确定性单步 mock |
| POST /api/agent/remote | `{"task":"读取 doc-1 文档"}`，仅显式配置模型时可用 |
| GET /api/overview?offset=0 | 当前用户的运行/审批/审计，每类每页 50 条，offset 上限 100000 |
| GET /api/metrics | 当前进程 HTTP 请求总量/错误/累计延迟/并发；非完整 Prometheus 指标 |

写入示例：

```json
{"run_id":"运行ID","tool":"ticket.update","params":{"resource_id":"ticket-1","body":"Resolved"},"idempotency_key":"incident-2026-001"}
```

返回待审批对象：id、run_id、tool、params、digest、status、expires。确认时不再接受新参数。写请求响应丢失时重新用同一 key 提案或 GET overview 查询；确认成功响应丢失时查 actions 的 succeeded 状态。重新确认返回 replay，不意味着业务未完成。

401 unauthenticated；403 access_denied（缺失/跨租户/无权等统一）；400 invalid_input/invalid_task 等；409 approval_mismatch/approval_expired/approval_replayed/idempotency_conflict；429 capacity_exceeded；504 timeout；503 dependency_unavailable。模型结果未知单独返回 502 model_result_unknown。

## 配置

`.env.example` 是参考，程序不会隐式读取 .env。用 shell 环境变量设置。运行路径应是仓库根目录（静态文件位于 web/）。AGENTGATE_DATA 默认 data，目录需要服务用户独占可写；AGENTGATE_ADDR 默认 127.0.0.1:8080。开发模式只接受 loopback IP。

OIDC：取消 AGENTGATE_DEV=1，设置 OIDC_ISSUER、OIDC_CLIENT_ID。必须由可信管理员预置 issuer subject 对应 principals，生产模式不自动种子化用户。

远端模型：设置 MODEL_ENDPOINT、MODEL_ALLOWED_HOST、MODEL_NAME、MODEL_API_KEY；支持 Chat Completions 工具调用兼容接口，模型名由部署者配置。仅 `/api/agent/remote` 使用模型。代码不会自动发送演示任务给付费模型。

## 迁移与离线管理

`internal/gate/migrations/001_init.sql` 用 go:embed 编入二进制，启动初始化表与 v1 迁移记录。仅当前 v1，尚无滚动迁移/多版本升级系统。不要跨版本复用未声明兼容的开发数据库。

演示撤销后，先停止服务，再用 Python 标准库恢复 alice（仅用于本地演示数据库；不要直接操作生产数据）：

```sh
python3 - <<'PY'
import sqlite3
with sqlite3.connect('data/agentgate.db') as db:
    db.execute("UPDATE principals SET enabled=1 WHERE id='alice' AND tenant='acme'")
    db.execute("UPDATE policy SET version=version+1 WHERE id=1")
PY
make run
```

已撤销运行仍保持终态，需要创建新运行。更安全的独立复现方式是指定新的 `AGENTGATE_DATA` 目录；不要删除已有用户数据。
