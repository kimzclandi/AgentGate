# API、配置与数据操作

所有 `/api/*` 需要 `Authorization: Bearer <token>`，不使用 cookie。请求 `Content-Type: application/json`；未知字段、额外 JSON 文档和超限输入拒绝。响应带 `X-Request-ID`，错误 JSON 包含稳定 error 与 request_id。不接受 tenant_id，由认证 subject 在 DB 映射租户。

| 方法 / 路径 | 输入 / 作用 |
|---|---|
| GET /healthz | 存活；不代表 DB 可用 |
| GET /readyz | DB ping，失败 503 |
| GET /api/capabilities | 是否配置本地/远端模型；不代表模型健康检查 |
| POST /api/chat | `{"task":"查询我的工单"}`；Ollama 多步流程 |
| GET /api/chat?id=… | 本人同租户的持久对话，隐藏 system 提示 |
| POST /api/chat/resume | `{"chat_id":"…"}`；审批成功后继续模型 |
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
| GET /api/overview?offset=0 | 当前用户的运行/审批/审计/对话，每类每页 50 条，offset 上限 100000 |
| GET /api/metrics | 当前进程 HTTP 请求总量/状态码 >=400 的响应数/累计延迟/并发；非完整 Prometheus 指标 |

写入示例：

```json
{"run_id":"运行ID","tool":"ticket.update","params":{"resource_id":"ticket-1","body":"Resolved"},"idempotency_key":"incident-2026-001"}
```

返回待审批对象：id、run_id、tool、params、digest、status、expires。确认时不再接受新参数。写请求响应丢失时重新用同一 key 提案或 GET overview 查询；确认成功响应丢失时查 actions 的 succeeded 状态。重新确认返回 replay，不意味着业务未完成。

401 unauthenticated；403 access_denied（缺失/跨租户/无权等统一）；400 invalid_input/invalid_task 等；409 approval_mismatch/approval_expired/approval_replayed/idempotency_conflict；429 capacity_exceeded；504 timeout；503 dependency_unavailable。模型结果未知单独返回 502 model_result_unknown。

## 配置

`.env.example` 是参考，程序不会隐式读取 .env。用 shell 环境变量设置。运行路径应是仓库根目录（静态文件位于 web/）。AGENTGATE_DATA 默认 data，目录需要服务用户独占可写；AGENTGATE_ADDR 默认 127.0.0.1:8080。开发模式只接受 loopback IP。

OIDC：取消 AGENTGATE_DEV=1，设置 OIDC_ISSUER、OIDC_CLIENT_ID。必须由可信管理员预置 issuer subject 对应 principals，生产模式不自动种子化用户。

远端模型：设置 MODEL_ENDPOINT、MODEL_ALLOWED_HOST、MODEL_NAME、MODEL_API_KEY；支持 Chat Completions 工具调用兼容接口，模型名由部署者配置。远端模型仅用于 `/api/agent/remote`；本地模型通过 OLLAMA_MODEL 配置，供 `/api/chat` 使用。代码不会自动发送演示任务给付费模型。

## 迁移与离线管理

`internal/gate/migrations/001_init.sql` 与 `002_chats.sql` 通过 go:embed 编入。v2 为 v1 添加 chats 表，不删除已有业务数据。单实例启动执行幂等迁移，并将旧活跃运行与对话标记 interrupted；不恢复旧审批执行。尚不支持滚动升级或自动降级。升级前停止服务并备份数据库。

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

## 本地对话状态

返回 id、run_id、messages、status、answer、rounds，以及可选 pending_action_id。写入返回 awaiting_approval；前端先调用 approve，再调用 chat/resume。恢复前必须核实原审批已成功，不能用 resume 代替审批。重复恢复返回 409；状态可通过 GET 查询。运行总有效期 300 秒。每个审批有效期为创建后 120 秒与运行剩余时间的较小值；超时需创建新任务，已经批准的写入不会回滚。

本地依赖错误为 503 local_model_unavailable，不暴露模型响应正文；审批未完成返回 409 approval_required。对话最多 9 次模型调用、8 次工具提案；单次模型等待最多 90 秒、一次驱动最多 240 秒。succeeded 表示执行流程结束，不是回答正确性的保证。会话及工具返回存于 SQLite；部署者需制定数据保留策略。

分页 offset 缺省为 0，只接受范围内的十进制整数；格式错误或越界返回 400。overview 中对话和未执行审批的状态会结合运行终态展示，取消/撤销的动作不再显示为可审批。

metrics 计数包括健康检查和静态资源请求；requests 包含当前进行中的请求，errors 在响应完成后计数，latency 为已完成请求累计时间，进程重启归零。不能用它代表业务成功率、模型正确率或 P95 延迟。
