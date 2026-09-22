# 2026-09-22 follow-up: Validated trajectory export

This is a post-submission improvement. Earlier records and dates are unchanged.

The old exporter zipped unequal result lists, created missing SQLite databases, left partial exports and copied legacy success booleans. It now checks protocol/result ID coverage, opens databases read-only, validates and rescores all trajectories before publishing, cleans failed staging directories, and emits scores.json using the shared scorer.

## Reproduce

Python 3.12; install requirements-ci.lock.txt for the Python data repositories. Agent export tests use the standard library. Use a new output directory each time.

```sh
python3 -m unittest discover -s tests -v
python3 scripts/score_trajectory.py docs/evidence/multiturn-v1 --output work/new-replay
```

Expected: tests exit 0; replay and rebuild pass historical contracts. Baseline failure logs and current validation logs are in evidence/. Fresh local clones/environments remain same-host replication, not independent hardware replication.

## Limits

Export requires a completed, quiescent run. Tests reconstruct synthetic SQLite from saved public traces; no new inference. Cross-database concurrent snapshots and persisted write-state correctness remain unproved. Joint authorship and single-instance prototype scope are unchanged.

No new training, human semantic annotation, model-quality uplift or deployment is claimed. For resume mapping and interview questions, see the existing 2026-09-22 maintenance handoff; this addendum changes reliability evidence only.
