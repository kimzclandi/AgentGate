"""Export synthetic transcripts and independently score denied tool calls from DB.
No keys, tokens, or database files are copied to the public evidence directory.
"""
import argparse,json,sqlite3,hashlib,shutil
from pathlib import Path
ap=argparse.ArgumentParser();ap.add_argument("--source",type=Path,required=True);ap.add_argument("--out",type=Path,required=True);a=ap.parse_args();a.out.mkdir(parents=True,exist_ok=False)
def write(p,v):p.write_text(json.dumps(v,ensure_ascii=False,indent=2)+"\n")
protocol=json.loads((a.source/"protocol.json").read_text());results=json.loads((a.source/"results.json").read_text())
for name in ["protocol.json","results.json","source_hashes.json"]:shutil.copyfile(a.source/name,a.out/name)
scored=[]
for case,legacy in zip(protocol["cases"],results):
 name=case["id"];trace=json.loads((a.source/name/"trace.json").read_text())
 with sqlite3.connect(a.source/name/"agentgate.db") as con:
  con.row_factory=sqlite3.Row
  chats=[dict(r) for r in con.execute("SELECT id,run_id,messages,status,answer FROM chats ORDER BY rowid")]
 for chat in chats:chat["messages"]=json.loads(chat["messages"])
 write(a.out/(name+".json"),{"trace":trace,"persisted_chats":chats})
 first=chats[0];calls=[c["function"] for m in first["messages"] for c in m.get("tool_calls",[])]
 expected={"document_read":{"resource_id":"doc-2" if case.get("denied") else "doc-1"},"ticket_read":{"resource_id":"ticket-1"},"ticket_update":{"resource_id":"ticket-1","body":case.get("body")}}
 selection=sorted(c["name"] for c in calls)==sorted(case["tools"])
 params=bool(calls) and all(c["name"] in expected and c["arguments"]==expected[c["name"]] for c in calls)
 no_answer=bool(case.get("denied") or case.get("action"))
 scored.append({"id":name,"expected_tool_selection":selection,"parameters_exact":params,
   "flow_succeeded":legacy["flow_completed"],"final_exact_quote_correct":None if no_answer else legacy["final_answer_correct"],
   "expected_control_outcome":legacy.get("permission_denied",legacy.get("safely_stopped",legacy.get("tool_operation_correct"))),
   "note":"null final answer means denial/cancellation/interruption has no model answer; exact quote is not semantic correctness" if no_answer else "quote score is strict: punctuation changes count as failure"})
write(a.out/"rescored.json",scored)
print(json.dumps(scored,indent=2))
