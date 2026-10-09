# Request-boundary state evidence (2026-09-22 follow-up)

The old exported chats and HTTP traces did not contain independently replayable before/after database states. Legacy booleans cannot fill this gap. Historical trajectories retain `write_state_correct=null`.

`scripts/state_evidence.py` reads seeded acme/ticket-1 and its Alice write actions through a read-only SQLite connection. A read transaction binds resource and action rows to the same snapshot. The record contains the full trace-prefix digest and request index, resource body/version, and action ID/run/tool/parameters/digest/status. No authentication token, key, database file or arbitrary tenant data is exported by this capture function. Use only isolated seeded fixtures, not a production DB.

The current real-model runner records an initial observation and one observation after each request. Its protocol declares `ticket-state-v1`. Export rejects missing declared observations and a final saved state differing from the current database. All new exported state is validated before publication. The real-model runner's new capture hook is implemented but no new real-model run was performed in this follow-up.

The v4 scorer keeps tool selection, arguments, control HTTP, flow, answer quotation and state correctness separate. Approved writes must preserve state until a matching successful approval, change to exactly the requested body and increment version once, then remain unchanged. The pending action must bind the action ID, digest, run and parameters. Cancel/restart cases check unchanged observed state; their control-flow success remains a separate metric. No state data means unknown, never a pass.

## Reproduce

Python 3.12+ standard library, Go 1.27.1 for the existing server. No model required:

```sh
go build -trimpath -o bin/agentgate ./cmd/agentgate
python3 scripts/state_demo.py --output work/state-new
python3 scripts/state_evidence.py --run work/state-new
python3 -m unittest discover -s tests -v
python3 scripts/score_trajectory.py docs/evidence/multiturn-v1 --output work/legacy-new
```

The state demo uses deterministic `/api/agent` proposals with real HTTP and SQLite. Three cases cover approved write plus digest substitution/foreign approval/replay, cancellation, and process restart. All three should show state invariants true. This is not model accuracy. Offline replay requires only `result.json` and the three case JSON files; it does not need DB files or a running server.

For a new actual model run, stop the run before using the existing `export_multiturn.py` command. Its `scores.json` is v4. Old exported cases remain untouched and still have zero eligible state observations.

## Evidence limits

Version/body comparisons observe request boundaries on an isolated single-instance fixture. They do not rule out a transient change restored between observations, arbitrary external SQL writes, or concurrent writers. Hashes bind local contents, not honest production attestation. Approval enforcement still comes from the existing transactional runtime and its Go tests; this work adds observable evidence, not an enterprise security claim. The database write happens during `approve`, not during chat `resume`.

Mutation tests reject incomplete/rebound state and detect an early write, a second version increment, approval mismatch and source DB drift. Test fixtures are constructed; the separate state demo records actual database reads. Historical model replay, deterministic mock execution and future live model runs must remain separate.
