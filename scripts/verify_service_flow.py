"""Independently reconstruct operation, lineage and read-back outcomes."""
import argparse
import json
from pathlib import Path

ap = argparse.ArgumentParser()
ap.add_argument("run", type=Path)
a = ap.parse_args()
protocol = json.loads((a.run / "protocol.json").read_text())
expected_body = protocol["expected_body"]
summary = []
for case in protocol["cases"]:
    saved = json.loads((a.run / (case["id"] + ".json")).read_text())
    result, trace, chats = saved["result"], saved["trace"], saved["chats"]
    if "error" in result:
        summary.append({"id": case["id"], "error": result["error"]})
        continue
    before, after = result["before"], result["after"]
    target = case.get("target", protocol["expected_target"])
    operation = after[target] == {"body": expected_body, "version": before[target]["version"]+1} and all(after[key] == value for key,value in before.items() if key != target)
    assert operation == result["tool_operation_correct"]
    assert all(after[key] == value for key,value in before.items() if key != target)
    ids = {c["id"] for c in chats}
    for link in saved["links"]:
        assert link["child_id"] in ids and link["parent_id"] in ids
        assert link["tenant"] == "acme" and link["user_id"] == "alice"
        assert link["child_id"] != link["parent_id"]
    if "parameters_correct" in result:
        assert result["parameters_correct"] == (result["proposed_parameters"] == {"resource_id": target, "body": expected_body})
    if case.get("stop") and "safe_control_outcome" in result:
        blocked_resume = any(t["path"] == "chat/resume" and t["status"] != 200 for t in trace)
        blocked_followup = any(t["path"] == "chat/continue" and t["status"] == 409 for t in trace)
        assert result["safe_control_outcome"] == (blocked_resume and blocked_followup and before == after)
    if case.get("foreign"):
        assert result.get("safe_control_outcome") == (any(t["path"] == "chat/continue" and t["status"] == 403 for t in trace) and before == after)
    if result.get("final_answer_correct") is not None:
        final = chats[-1]
        assert result["final_answer_correct"] == (expected_body in final["answer"])
        last_user = max(i for i, m in enumerate(final["messages"]) if m["role"] == "user")
        new_calls = [c["function"] for m in final["messages"][last_user+1:] for c in m.get("tool_calls", [])]
        actual_read = any(c["name"] == "ticket_read" and c["arguments"] == {"resource_id": target} for c in new_calls)
        assert actual_read == result.get("readback_tool_correct")
        assert result["workflow_completed"] == (operation and actual_read and result["final_answer_correct"] and final["status"] == "succeeded")
    summary.append({"id": case["id"], "workflow_completed": result["workflow_completed"],
                    "tool_operation_correct": operation, "control": result.get("safe_control_outcome"), "links": len(saved["links"])})
print(json.dumps(summary, indent=2))
print("PASS: saved resources, parameters, parent links and new read-back calls independently verified")
