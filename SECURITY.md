# Security

AgentGate enforces identity, delegation, resource authorization and approval checks for registered Agent tools. Read [the threat model](docs/THREAT_MODEL.md) for trust boundaries and remaining risks, and [the test report](docs/TEST_REPORT.md) for verification results.

Development authentication is restricted to loopback interfaces. Do not expose it publicly or commit keys, tokens, databases, or private tenant data. External deployments require a configured identity provider, TLS, secret management, quotas and an audit retention policy.

Report non-sensitive reproduction steps through a GitHub issue. Never include real credentials or private user data in a public report.
