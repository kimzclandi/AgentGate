# Durable multi-turn service flow

This iteration adds a browser follow-up action and a persisted, tenant/user-scoped parent chat link. A completed turn can continue under a fresh run; pending approval uses resume. The UI blocks duplicate dispatch while a request is pending. A new migration creates `chat_links`; chat creation and parent linkage commit together. Existing chat records remain readable.

Code: `internal/gate/chat.go`, `internal/gate/store.go`, `internal/gate/migrations/003_chat_links.sql`, `web/app.js`, `web/index.html`.

## Experiment

Question: can an ambiguous follow-up modify the previously read ticket, only after approval, then verify its current value with a new tool call? Baseline starts the follow-up without history. The context arm inherits the completed conversation. Both see the same seeded resources, local Qwen2.5-1.5B model and follow-up text. Input is a natural-language read followed by “that ticket”; output includes proposed parameters, approval state, database state/version, a new read-back call and final response. Success requires the exact target/body, exactly one version increment, unchanged other resources, actual read-back and the expected final body. Wrong parameters or bypassing approval fail.

`service-flow-v1` retains the initial six cases. Stateless, continued and corrected-reference all completed; therefore this first comparison does not establish a context benefit. Cancel, restart and foreign-target probes preserved state and rejected continuation/resume as appropriate. Restart is safe interruption, not automatic task recovery. Synthetic approvals are issued by the evaluation harness only after exact parameter validation; these are not observations of human users.

After observing that stateless also selected ticket-9, `balanced-reference-v2` fixed two targets before inference. Same ambiguous follow-up, two arms per target, no prompt/model tuning:

| Target | Stateless | Continued |
|---|---|---|
| ticket-1 | Proposed ticket-9; harness cancelled; no write | Correct approved write, version 1→2, new read-back, correct final body |
| ticket-9 | Correct approved write and read-back | Correct approved write and read-back |

Thus 1/2 versus 2/2 completed in this diagnostic, not a population accuracy estimate. `protocol.json` retains the inherited global default `expected_target` and generic success wording; the explicit per-case `target` is authoritative for the balanced run. This clarification does not alter its frozen protocol. Approval replay was rejected in every completed case. Final-answer correctness means the expected synthetic body occurs in the final response; it is distinct from operation correctness and is not a general semantic judge.

Costs and limitations: history increases model input and storage; no throughput or production-load benchmark was performed. UI tests exercise DOM/fetch behavior, not browser visual layout. The small, synthetic English task set does not establish Chinese service quality. Persisted context may contain stale observations; the new read-back is essential.

## Reproduce

Use Python with torch 2.8.0 and transformers 4.56.2 and a local Qwen2.5-1.5B-Instruct snapshot, revision `989aa7980e4cf806f80c7fef2b1adb7bc71aa306`. CPU float32, four threads, greedy, at most 256 new tokens per call. No weight download is performed by the bridge.

```sh
go build -o bin/agentgate ./cmd/agentgate
python scripts/hf_local_bridge.py --snapshot "$QWEN_1_5B_SNAPSHOT"
# In another terminal; use NEW directories, never overwrite saved evidence:
python scripts/evaluate_service_flow.py --work work/service-new --out docs/evidence/service-new
python scripts/evaluate_service_flow.py --balanced --work work/balanced-new --out docs/evidence/balanced-new
python scripts/verify_service_flow.py docs/evidence/service-flow-v1
python scripts/verify_service_flow.py docs/evidence/balanced-reference-v2
go test ./...
node --test scripts/ui.test.cjs
```

The bridge listens on loopback port 11434; stop it after use. The fixture uses temporary local servers/databases. Exported evidence contains synthetic resource snapshots, HTTP response traces, model messages, source hashes and parent links; auth tokens, signing keys and runtime databases are not exported. The initial evaluator is archived byte-for-byte because the balanced run extended it. Both evidence sets pass independent saved-state/parameter/read-back verification. Go suite and four UI boundary tests passed locally.
