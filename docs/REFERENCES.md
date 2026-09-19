# 官方资料与依赖

读取/核对日期：2026-09-12。精确依赖版本见 go.mod/go.sum。

- JWT 标准：[RFC 7519](https://www.rfc-editor.org/rfc/rfc7519)。本地实现使用成熟库验证，不自建密码学。
- JWT 校验选项：[golang-jwt/v5 API](https://pkg.go.dev/github.com/golang-jwt/jwt/v5)。本版 v5.3.1，固定算法及 issuer/audience/exp。
- OIDC：[OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html)、[go-oidc API](https://pkg.go.dev/github.com/coreos/go-oidc/v3/oidc)。本版 v3.17.0，外部 IdP 未实测。
- 可选模型工具调用：[Chat Completions 官方接口](https://developers.openai.com/api/reference/cli/resources/chat)。只依赖兼容工具调用格式，模型名由配置提供；mock 默认不访问提供商。
- Go 工具链：[下载](https://go.dev/dl/)、[版本记录](https://go.dev/doc/devel/release)。2026-09-12 从官方 JSON 获取 Go 1.27.1 darwin/arm64 并校验 SHA-256。
- [GO-2026-5024](https://pkg.go.dev/vuln/GO-2026-5024)：未导入的 Windows 路径模块告警，详见 TEST_REPORT。

- [govulncheck 官方文档](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck)：扫描器更新固定为 v1.8.0，以支持 Go 1.27.1 工具链。
