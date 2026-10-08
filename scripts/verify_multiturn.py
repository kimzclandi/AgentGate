"""Recompute model selection and exact quotes from exported synthetic transcripts."""
import argparse,json
from pathlib import Path
ap=argparse.ArgumentParser();ap.add_argument("run",type=Path);a=ap.parse_args()
protocol=json.loads((a.run/"protocol.json").read_text());saved={r["id"]:r for r in json.loads((a.run/"rescored.json").read_text())}
for case in protocol["cases"]:
 raw=json.loads((a.run/(case["id"]+".json")).read_text());chats=raw["persisted_chats"];calls=[c["function"] for m in chats[0]["messages"] for c in m.get("tool_calls",[])]
 expected={"document_read":{"resource_id":"doc-2" if case.get("denied") else "doc-1"},"ticket_read":{"resource_id":"ticket-1"},"ticket_update":{"resource_id":"ticket-1","body":case.get("body")}}
 assert (sorted(c["name"] for c in calls)==sorted(case["tools"]))==saved[case["id"]]["expected_tool_selection"]
 assert (bool(calls) and all(c["name"] in expected and c["arguments"]==expected[c["name"]] for c in calls))==saved[case["id"]]["parameters_exact"]
 if case.get("denied"):assert raw["trace"][0]["http"]==403
 elif case.get("action"):assert raw["trace"][-1]["http"]!=200
 else:
  quote=case.get("body",case.get("answer","Open: customer needs help"))
  assert (quote in chats[-1]["answer"])==saved[case["id"]]["final_exact_quote_correct"]
print("PASS: tool names, parameters, quoted answers and control HTTP outcomes recomputed from transcripts")
