# AgentGate

[简体中文](README.md) | **English**

[![verify](https://github.com/kimzclandi/AgentGate/actions/workflows/ci.yml/badge.svg)](https://github.com/kimzclandi/AgentGate/actions/workflows/ci.yml)

**Identity checks, resource permissions and human approval for AI-agent document reads and ticket updates, implemented in Go.**

A user can ask a local model to read documents and tickets and summarize actual tool results. For an update, the user first reviews a backend-generated parameter preview, approves execution, then asks the model to respond using the result. The model cannot approve its own actions.

The code license remains subject to a joint decision by the authors. Public source availability does not grant an open-source license.

Coauthors: [@kimzclandi](https://github.com/kimzclandi) · [@Lu-Ricardo-Y](https://github.com/Lu-Ricardo-Y). [Contributions and dependencies](AUTHORS.md)

## Current capabilities

| Capability | Implementation and scope |
|---|---|
| Natural language and multi-step tools | Local Ollama model, verified with qwen3:1.7b; up to 9 model calls and 8 tool proposals |
| Document/ticket operations | Three built-in tools backed by actual SQLite data; initially four synthetic resources; no external ticket-system integration |
| Identity and delegation | Separate user, agent, tenant and run identities; role and resource-owner constraints; development JWT plus an OIDC adapter not verified against a real IdP |
| Approval for writes | Parameter-digest binding, confirmation by the original requester, live permission rechecks, transactional execution and replay protection |
| Runs and audit | Persistent conversations, cancellation, revocation, timeouts, restart interruption and tenant-isolated run/audit records |
| Admin console | Actual backend APIs displaying answers, tool traces, proposed parameters and run state |

This is a **single-instance reference implementation for controlled agent execution**. Local-model execution is verified; the remote compatible interface supports only single-step planning. Understanding and summarization can still be wrong. `succeeded` means the workflow completed, not that the answer passed a factual evaluation.

## Run an actual local agent

Requires Go 1.27.1, macOS or Linux, and [Ollama](https://ollama.com/download). See the [official Go installation guide](https://go.dev/doc/install). The initial model download needs network access and about 1.36 GB of disk space; no paid cloud API is required.

```sh
git clone https://github.com/kimzclandi/AgentGate.git
cd AgentGate
```

Start Ollama in terminal one. If a service already exists, handle the port conflict as described in the [local-model guide](docs/LOCAL_MODEL.md).

```sh
OLLAMA_NO_CLOUD=1 OLLAMA_HOST=127.0.0.1:11434 ollama serve
```

In terminal two, from the repository root:

```sh
ollama pull qwen3:1.7b
OLLAMA_MODEL=qwen3:1.7b make run
```

In terminal three, issue a development credential from the same repository root:

```sh
AGENTGATE_DEV=1 ./bin/agentgate -token alice
```

Open `http://127.0.0.1:8080`, paste the credential and connect. Use the following original Chinese test prompt (it asks for both records and a Chinese summary):

```text
请分别读取 doc-1 和 ticket-1，读取两条真实记录后，用中文总结。
```

Expand the tool-call/results view. It should show actual `document_read` and `ticket_read` results. The initial ticket content is `Open: customer needs help`.

Then use this prompt to request an update:

```text
请把 ticket-1 的内容更新为 Resolved by local Agent，然后告诉我执行结果。
```

Review the proposed parameters, approve execution, then click the control to continue answering after approval. Approval expires after at most 120 seconds and never exceeds the remaining run lifetime; the total run lifetime is 300 seconds. The backend rejects access to cross-tenant resource `doc-2`.

Startup failures, CPU compatibility settings and actual test records are covered in the [local-model guide](docs/LOCAL_MODEL.md). Stop services with Ctrl+C in their terminals. Data remains in `data/`.

## Check the backend without a model

Run `make run`, connect and use the fixed commands `read-doc doc-1`, `read-ticket ticket-1` and `update-ticket ticket-1 Resolved`. This is an explicitly identified deterministic test mode, not natural-language inference.

Optional remote-model configuration is documented in [.env.example](.env.example) and the [API reference](docs/API.md). The program neither automatically reads `.env` nor automatically sends tasks to a paid model.

## Verification

Python 3.10+ is used for HTTP/documentation checks, and Node.js 22 for console-state regression tests. Neither is a runtime dependency of the Go service.

```sh
make check            # Go tests, race detector and vet
make ui-test docs-check
make demo             # Fixed HTTP acceptance run with a temporary database
make scan             # Reachable-vulnerability scan for the current platform
make local-demo       # Requires running Ollama; checks actual tools and writes
```

See the [test report](docs/TEST_REPORT.md) for scope, historical performance and unverified areas. Raw records are in [docs/evidence](docs/evidence/README.md). Simulated tests and a few fixed tasks do not establish production reliability.

## Code and documentation

```text
cmd/agentgate/         Configuration, authentication setup, process lock and shutdown
internal/gate/        Authorization, transactional approvals, model adapters, conversations, migrations
web/                  Framework-free admin console
scripts/              HTTP, actual-model, UI-state and documentation checks
```

- Execution: [Architecture](docs/ARCHITECTURE.md) · [Permissions](docs/PERMISSIONS.md) · [Runtime](docs/RUNTIME.md)
- Usage: [Guide](docs/DEMO.md) · [API](docs/API.md) · [Local models](docs/LOCAL_MODEL.md)
- Status: [Tests](docs/TEST_REPORT.md) · [Progress](docs/PROGRESS.md) · [Review records](docs/REVIEW.md)
- Boundaries: [Threat model](docs/THREAT_MODEL.md) · [Security](SECURITY.md) · [Contributing](CONTRIBUTING.md)

Linked technical documents retain their original language. Production web login, OAuth service identities, multi-instance coordination, vector RAG, arbitrary-code isolation and reconciliation of external business writes are not implemented. GitHub hosts code and documentation; using the backend online requires separate deployment. Development authentication permits local access only.
