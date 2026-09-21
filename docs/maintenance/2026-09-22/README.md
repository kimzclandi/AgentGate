# Versioned trajectory scoring — 2026-09-22

## Problem

The original live diagnostic evaluator accepted any tool list containing the requested names, so an extra/duplicate tool could still pass selection. The two-resource task required both quotations but checked only one. The exporter paired protocol and results with `zip`, allowing incomplete result coverage to truncate silently. Its later rescore already fixed exact initial tool selection, so that existing behavior is reused rather than credited as new.

## Change

`scripts/score_trajectory.py` consumes the existing exported `trace` and `persisted_chats` structure. It requires protocol/result ID coverage; checks the exact initial tool multiset and arguments; separately reports terminal flow, denial HTTP, stop HTTP, required quotations and the follow-up no-tool constraint after the last user message. Each metric carries its own eligible denominator. Denied/stopped cases have no answer score; missing persisted-write evidence is explicitly null. Failure categories remain separate. No mixed success rate is emitted.

The old evaluators, exports and frozen reports remain historical artifacts. For new offline interpretation use this v2 scorer; do not compare its numbers as if the historical model had improved. `results.json` is used for coverage, never trusted for scoring booleans. Full source hashes and input hashes accompany the new scores. Exact quotation presence is a narrow deterministic proxy, not semantic correctness or sufficient evidence of a database transaction.

## Reproduce

Python 3.10+ standard library, repository root:

```sh
python3 -m unittest discover -s tests -v
python3 scripts/score_trajectory.py docs/evidence/multiturn-v1 --output work/trajectory-demo
```

Go 1.27.1, Python and Node.js for the existing runtime checks:

```sh
make check ui-test docs-check trajectory-test
make demo
```

`make demo` builds the binary and runs a temporary real HTTP/SQLite service using deterministic commands; no model or normal user database is involved. If macOS blocks `make` on Xcode licensing, execute the underlying Makefile commands with an installed Go toolchain: `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build -trimpath -o bin/agentgate ./cmd/agentgate`, then `python3 scripts/demo.py`. This run did not change system license settings.

## Actual results and boundaries

Eight scorer regressions passed; ordinary Go tests, race and vet passed; four UI tests and repository documentation checks passed. Fixed HTTP/SQLite acceptance passed 22/22. The latter required permission for a temporary loopback listener after the restricted attempt failed.

New scoring of 8 saved Qwen2.5 model cases: initial tool selection 8/8, parameters 8/8, successful terminal flows 5/8, denial HTTP 1/1, stop HTTP 2/2, required final quotes 3/5, no-tool follow-up 2/2. Database state from these exported transcripts alone is unverified (0 eligible); current fixed HTTP/SQLite tests provide separate execution evidence. Historical Qwen3-1.7B/Ollama runs are a different evidence set. No new model inference was executed. No answer-quality gain was measured; v2 did not change these historical case outcomes.

CI now runs the scorer regressions and saved-trajectory replay in addition to existing runtime checks. Work outputs and Python caches are ignored; reviewed evidence is copied explicitly. The project remains a jointly authored, AI-assisted, local single-instance reference prototype. This change does not establish personal contribution shares, production deployment, arbitrary-code isolation or complete security.
