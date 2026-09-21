# Continuing completed conversations with fresh execution scope

`POST /api/chat/continue` accepts `{"chat_id":"...","task":"..."}`. The service loads only the authenticated user's persisted conversation, requires a succeeded previous turn, carries its messages into a new chat/run, and recreates current permission scopes. Pending approvals, cancelled/failed turns and oversized histories cannot be continued. Writes still pass through the existing explicit approval and transactional execution path. The earlier single-task and approval-resume APIs remain unchanged.

This is continuation of completed context, not automatic recovery of an interrupted write. After a server restart the interrupted task is retained for inspection and cannot resume execution; a new task is required. Previously saved tool text can become stale, so current-state questions still require fresh reads.

## Real local run

The original Qwen3/Ollama experiment is preserved. This experiment uses cached **Qwen2.5-1.5B-Instruct through Transformers on CPU**, exposed via an explicit loopback Ollama-wire evaluation adapter. It does not claim that Qwen3 or Ollama were rerun. The adapter parses actual model-generated tool-call JSON; it does not synthesize tool proposals from task labels.

```sh
go build -o bin/agentgate ./cmd/agentgate
python scripts/hf_local_bridge.py --snapshot "$QWEN_1_5B_SNAPSHOT"
# In another terminal; requires a new output directory:
python scripts/evaluate_multiturn.py --out work/new-multiturn-run
python scripts/export_multiturn.py --source work/new-multiturn-run --out docs/evidence/new-multiturn-run
python scripts/verify_multiturn.py docs/evidence/multiturn-v1
```

Use torch 2.8.0 and transformers 4.56.2 for the tested adapter. It is a single-process evaluation fixture, not a replacement production model server. Only synthetic seed resources and harness-approved writes are used. No keys, tokens, databases or model weights are included in the exported evidence.

## Results and failure definitions

Eight diagnostic scenarios cover a document read, two-resource read, approved write, forbidden tenant read, cancel-before-write, restart-before-write, and the same follow-up with/without persisted context. Recorded model/tool parameters match the expected initial tool calls in all eight. The forbidden call is still denied by the backend; correct model parameter parsing does not grant authorization.

The stateless follow-up returns an inability to answer; the continued follow-up quotes `Open: customer needs help` correctly in a fresh run. The approved update changes the body exactly once to `Resolved CASE-17`, increments its version from 1 to 2, and rejects resume replay. Cancellation and restart preserve the original body/version and reject execution resume. All three write proposals leave the database unchanged before harness approval.

Five answer-bearing flows finish, but only three satisfy the strict final exact-quote criterion. In the document case the model preserves the meaning while changing the English full stop to Chinese punctuation: it fails exact quotation, not necessarily semantic understanding. The stateless follow-up is the substantive missing-context failure. For denial/cancellation/restart, final model answer is not applicable and recorded as null in `rescored.json`; expected control outcome is scored separately.

`results.json` preserves the first runner scoring, which lacked returned tool messages for HTTP denial. `rescored.json` corrects that observation using persisted synthetic transcripts and separates null final answers. The original result is not overwritten. This is a small diagnostic, not an 8-case estimate of open-ended tool accuracy or production reliability. HTTP service/UI integration for the new endpoint beyond the evaluated client, multi-user scale and automatic interruption recovery remain unverified.
