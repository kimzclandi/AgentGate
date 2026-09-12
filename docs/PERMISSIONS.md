# 身份、委托与权限

## 数据模型

| 对象 | 当前实现 |
|---|---|
| 用户 | principals：subject、tenant、role、enabled |
| Agent | agents：id、允许动作、enabled；是执行逻辑身份，不是用户 |
| 控制面服务 | 单一受信任服务进程，由进程独占锁标识；尚无独立 OAuth 服务账号入口 |
| 租户 | principals 与 resources 上的 tenant，认证后从 DB 获取 |
| 运行/委托 | runs：user、agent、tenant、scopes、audience=agentgate-tools、expires、steps、status |
| 审批 | actions：发起人、租户、运行、工具、规范化参数、摘要、有效期、状态、幂等键 |
| 策略 | 当前规则为版本化 Go 策略；policy 表保存变更版本 |

用户创建运行时要求 scopes 为用户角色与 Agent 动作的子集。运行只能由原发起人调用；没有对子运行再委托的 API，因此不能利用再委托放大权限。运行 ID 不是 bearer 凭证。有效期最长 300 秒。

Operator 可提出已注册读写动作；reader 只有读取。所有角色仍必须同时满足：资源属于同租户、资源 owner 等于用户、类型与工具一致、Agent 活跃、用户未撤销、运行有效且未达到步骤上限。不存在“管理员自动跨租户”规则。

## 认证边界

开发模式：golang-jwt/v5；HS256 固定算法、kid=dev-v1、issuer=agentgate-local-dev、audience=agentgate、exp 必须存在、iat 检查、非空 subject。随机 32 字节本地密钥文件权限 0600，令牌 1 小时。不使用仅解码 JWT。签发 CLI 是本机可信操作者能力，不提供匿名 HTTP 登录。

非开发模式：go-oidc discovery/JWKS 验签与 client ID 受众检查；issuer/client ID 为部署配置。令牌 subject 必须预先映射到 principals。当前为单 issuer OIDC ID token 适配，不声称实现 OAuth 授权服务器、token exchange 或通用 access-token introspection。真实 IdP 接入未验证。生产部署还需 Web 登录/PKCE、TLS、服务身份和明确 token profile。

OIDC JWKS 缓存/轮换由 go-oidc 管理。开发模式只支持一个 kid，轮换需停服替换 key 并重新签发，旧令牌失效；不要宣称开发模式平滑多密钥轮换。

## 审批绑定

用固定 Go Params 结构严格拒绝未知字段，然后 JSON 序列化。摘要为 SHA-256(JSON([tenant,user,run,tool,paramsJSON]))。这是服务端动作绑定，不是自制令牌协议。用户确认必须再次提供完整认证并匹配创建者；action_id/digest 本身不能授权。

唯一约束 `(tenant,user_id,idem)` 防止同一业务键重复提案；不同参数复用键返回冲突。确认检查 pending、有效期、摘要、运行及实时授权，事务内写入资源并改状态。提案计入 8 步预算，确认不重复计步；第八步写入可以确认。允许一次成功确认；并发/重放返回 409。

## 撤销语义

`POST /api/revoke` 撤销当前身份并递增策略版本，将运行改为 revoked。没有授权缓存。权限复查、确认写入和撤销在同一 SQLite 单连接上串行执行：撤销提交后，后续写入不会获准；先前已经提交的更新不能撤回。正在进行的外部模型请求可能已经产生提供商工作/费用，但其返回不能绕过后续权限复查。清理通常由 1 秒 sweeper 完成。

不要把“后续受保护操作被拒绝”描述成撤销瞬时终止所有网络计算。策略编辑尚未开放 HTTP 管理接口；离线修改必须同时递增 policy.version，详见 API 文档。
