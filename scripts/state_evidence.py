"""Read-only request-boundary evidence for isolated seeded ticket fixtures.

Observations are not a tamper-proof audit or a continuous concurrent write log.
"""
import hashlib
import json
from pathlib import Path
import sqlite3


def trace_digest(trace):
    return hashlib.sha256(json.dumps(trace,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode()).hexdigest()


def capture(database, trace):
    con=sqlite3.connect(Path(database).resolve().as_uri()+'?mode=ro',uri=True)
    try:
        con.row_factory=sqlite3.Row
        con.execute('BEGIN')  # Resource and action reads share one SQLite snapshot.
        row=con.execute("SELECT tenant,id,owner,kind,body,version FROM resources WHERE tenant='acme' AND id='ticket-1'").fetchone()
        if row is None:raise ValueError('Seeded ticket-1 missing')
        actions=[dict(r) for r in con.execute("SELECT id,run_id,tool,params,digest,status,tenant,user_id FROM actions WHERE tenant='acme' AND user_id='alice' AND tool='ticket.update' ORDER BY rowid")]
        for action in actions:action['params']=json.loads(action['params'])
        return dict(trace_count=len(trace),trace_sha256=trace_digest(trace),resource=dict(row),actions=actions)
    finally:con.close()


def score_state(case, raw):
    evidence=raw.get('state_evidence')
    if evidence is None:return None
    trace=raw['trace']
    if not isinstance(evidence,dict) or evidence.get('schema')!='ticket-state-v1' or evidence.get('case_id')!=case['id']:
        raise ValueError('State schema/case binding mismatch')
    observations=evidence.get('observations')
    if not isinstance(observations,list) or len(observations)!=len(trace)+1:
        raise ValueError('Complete request-boundary state coverage required')
    for n,s in enumerate(observations):
        if not isinstance(s,dict) or type(s.get('trace_count')) is not int or s['trace_count']!=n or s.get('trace_sha256')!=trace_digest(trace[:n]):
            raise ValueError('State trace binding mismatch')
        r=s.get('resource');actions=s.get('actions')
        if (not isinstance(r,dict) or set(r)!={'tenant','id','owner','kind','body','version'} or
                (r['tenant'],r['id'],r['owner'],r['kind'])!=('acme','ticket-1','alice','ticket') or
                not isinstance(r['body'],str) or type(r['version']) is not int or r['version']<1 or not isinstance(actions,list)):
            raise ValueError('Invalid seeded resource state')
        for a in actions:
            if (not isinstance(a,dict) or set(a)!={'id','run_id','tool','params','digest','status','tenant','user_id'} or
                    any(not isinstance(a[k],str) or not a[k] for k in ['id','run_id','digest','status']) or
                    a['tool']!='ticket.update' or a['tenant']!='acme' or a['user_id']!='alice' or not isinstance(a['params'],dict)):
                raise ValueError('Invalid action snapshot')
    if observations[0]['actions']:raise ValueError('Initial fixture must have no prior write actions')
    if not case.get('body'):return None
    initial=observations[0]['resource']
    if case.get('action'):
        return all(s['resource']==initial and all(a['status']!='succeeded' for a in s['actions']) for s in observations)
    approvals=[n for n,t in enumerate(trace) if t.get('path')=='approve' and t.get('http')==200]
    if len(approvals)!=1:return False
    n=approvals[0];request=trace[n].get('request',{})
    if not isinstance(request,dict):raise ValueError('Approval request must be an object')
    if n==0 or len(observations[n]['actions'])!=1:return False
    pending=observations[n]['actions'][0]
    if (pending['status']!='pending' or pending['params']!={'resource_id':'ticket-1','body':case['body']} or
            request.get('action_id')!=pending['id'] or request.get('digest')!=pending['digest'] or
            pending['run_id']!=trace[0].get('response',{}).get('run_id')):
        return False
    expected=dict(initial,body=case['body'],version=initial['version']+1)
    for i,s in enumerate(observations):
        if s['resource']!=(initial if i<=n else expected):return False
        if i>n and s['actions']!=[dict(pending,status='succeeded')]:return False
    return True


def main():
    import argparse
    parser=argparse.ArgumentParser(description='Replay saved state_demo observations without a server or database.')
    parser.add_argument('--run',type=Path,required=True);args=parser.parse_args()
    receipt=json.loads((args.run/'result.json').read_text())
    if receipt.get('schema')!='state-demo-v1' or set(receipt.get('input_sha256',{}))!={'approved','cancel','restart'}:
        raise ValueError('Unexpected state demo receipt')
    rows=[]
    for name,digest in receipt['input_sha256'].items():
        path=args.run/(name+'.json')
        if hashlib.sha256(path.read_bytes()).hexdigest()!=digest:raise ValueError('State demo input changed')
        data=json.loads(path.read_text())
        if data['case']['id']!=name:raise ValueError('State case ID mismatch')
        rows.append(dict(id=name,write_state_correct=score_state(data['case'],data['raw'])))
    print(json.dumps(dict(execution_mode='saved_snapshot_replay',cases=rows),indent=2))
    if any(r['write_state_correct'] is not True for r in rows):raise SystemExit(1)

if __name__=='__main__':main()
