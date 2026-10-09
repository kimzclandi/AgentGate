# Reject misleading control outcomes and mismatched trajectories — 2026-09-22 follow-up

## Failure-first findings

The v2 stop metric accepted any non-200 response, including HTTP 500 or an error from an unrelated route. An empty `required_quotes` list passed vacuously; a string was iterated as characters. Task/chat mismatches and a 403 with an unrelated error type also passed relevant checks. Six mutation test groups reproduced nine failing assertions before this repair; later tests additionally cover missing requests, missing write targets and failed cancellation.

## Changes in trajectory-score-v3

- Validate protocol, nonempty task/rubric fields, source/result ID coverage, and persisted/HTTP record shapes.
- Bind the protocol task to the first HTTP request and persisted user message; bind chat/run identities and follow-up parent/response identities. A malformed or mismatched evidence bundle raises an error rather than yielding a score.
- Permission denial requires 403 plus `access_denied`. Cancellation requires a recorded successful cancel for the same run and a matching resume rejection. Restart requires the matching 409 error and recorded interrupted chat state. Arbitrary 4xx/5xx responses do not count as expected controls.
- Quote rubrics must be nonempty lists of nonempty strings; tool, argument, flow, quote and no-tool-follow-up metrics retain separate denominators. Database correctness remains unassessed from transcripts alone.
- The output uses `saved_output_replay`, carries the model declaration from the protocol, and does not promote arbitrary saved fixtures to real model execution. `recorded_chat_status` describes the saved chat state; for cancellation it may still be `awaiting_approval`, so it must not be called a proven terminal status.

This scorer is for the fixed synthetic `multiturn-v1` protocol, not a general evaluator for arbitrary tools/resources. Stop errors are deliberately version-specific. The archived v2 scorer under `baseline/` remains available for historical comparison; no v2 scores or raw traces were overwritten.

## Verification

Python 3.10+ standard library, repository root:

```sh
python3 -m unittest discover -s tests -v
python3 scripts/score_trajectory.py docs/evidence/multiturn-v1 --output work/detail-score-new
python3 scripts/verify_multiturn.py docs/evidence/multiturn-v1
python3 scripts/check_docs.py
```

All 17 scorer tests passed. Replaying the eight original saved trajectories leaves all existing metric totals unchanged: tool/arguments 8/8, flow 5/8, quotes 3/5, denial 1/1, expected stops 2/2, no-tool follow-ups 2/2, database state 0 eligible. This is stronger rejection of defective evidence, not model improvement. Existing Go/runtime implementation was not changed in this follow-up; its earlier local tests are separate evidence, and the PR CI re-runs the full workflow.

No new model inference or semantic annotation. Same joint authorship, AI-assisted work and single-instance prototype boundary. Prior evidence remains unchanged; this dated directory contains only new mutation tests, replay results, source binding and sanitized logs.
