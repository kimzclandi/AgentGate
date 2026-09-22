"""Deterministic mock proposals, real HTTP/SQLite state observations; no model."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import socket
import subprocess
import time
import urllib.request
import urllib.error
from state_evidence import capture, score_state
ROOT=Path(__file__).resolve().parents[1]

def write(path,value):path.write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')

def run(output):
    output=Path(output).resolve()
    if not output.is_relative_to(ROOT/'work') or output==ROOT/'work':raise ValueError('New work/ directory required')
    output.mkdir(parents=True,exist_ok=False)
    results=[]
    for name in ('approved','cancel','restart'):
        folder=output/name;folder.mkdir();trace=[];observations=[]
        case=dict(id=name,body='STATE-PROBE-'+name)
        if name!='approved':case['action']=name
        with socket.socket() as sock:sock.bind(('127.0.0.1',0));port=sock.getsockname()[1]
        env={**os.environ,'AGENTGATE_DEV':'1','AGENTGATE_DATA':str(folder),'AGENTGATE_ADDR':f'127.0.0.1:{port}'}
        env.pop('OLLAMA_MODEL',None)
        binary=str(ROOT/'bin/agentgate');base=f'http://127.0.0.1:{port}'
        tokens={u:subprocess.check_output([binary,'-token',u],env=env,text=True).strip() for u in ['alice','bob']}
        def start():
            proc=subprocess.Popen([binary],env=env,cwd=ROOT,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            for _ in range(100):
                try:
                    with urllib.request.urlopen(base+'/readyz',timeout=.2) as response:
                        if response.status==200:return proc
                except (OSError,urllib.error.URLError):pass
                if proc.poll() is not None:raise RuntimeError('server exited')
                time.sleep(.05)
            proc.terminate();proc.wait(timeout=10);raise RuntimeError('startup timeout')
        def req(path,body,expected,user='alice'):
            request=urllib.request.Request(base+'/api/'+path,data=json.dumps(body).encode(),headers={'Authorization':'Bearer '+tokens[user],'Content-Type':'application/json'})
            try:
                with urllib.request.urlopen(request,timeout=10) as response:status,value=response.status,json.load(response)
            except urllib.error.HTTPError as e:status,value=e.code,json.load(e)
            trace.append(dict(path=path,request=body,http=status,response=value))
            observations.append(capture(folder/'agentgate.db',trace))
            if status!=expected:raise ValueError((path,status,expected))
            return value
        proc=start()
        try:
            observations.append(capture(folder/'agentgate.db',trace))
            proposal=req('agent',{'task':'update-ticket ticket-1 '+case['body']},200)
            action=proposal['action'];approval={'action_id':action['id'],'digest':action['digest']}
            if name=='approved':
                req('approve',dict(approval,digest='modified'),409)
                req('approve',approval,403,user='bob')
                req('approve',approval,200)
                req('approve',approval,409)
            elif name=='cancel':
                req('cancel',{'run_id':proposal['run_id']},200)
                req('approve',approval,403)
            else:
                proc.terminate();proc.wait(timeout=10);proc=start()
                trace.append(dict(path='process/restart',request={},http=200,response={'operation':'terminated_then_started'}))
                observations.append(capture(folder/'agentgate.db',trace))
                req('approve',approval,403)
        finally:
            proc.terminate();proc.wait(timeout=10);(folder/'dev.key').unlink(missing_ok=True)
        raw=dict(trace=trace,state_evidence=dict(schema='ticket-state-v1',case_id=name,observations=observations))
        passed=score_state(case,raw)
        write(output/(name+'.json'),dict(case=case,raw=raw))
        if passed is not True:raise ValueError('State invariant failed: '+name)
        results.append(dict(id=name,write_state_correct=passed,requests=len(trace)))
    result=dict(schema='state-demo-v1',mode='deterministic_mock_real_http_sqlite',cases=results,
                scope='Seeded request-boundary observations, not new model inference or concurrent safety proof.',
                code_sha256={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [Path(__file__),ROOT/'scripts/state_evidence.py']},
                binary_sha256=hashlib.sha256((ROOT/'bin/agentgate').read_bytes()).hexdigest(),
                input_sha256={r['id']:hashlib.sha256((output/(r['id']+'.json')).read_bytes()).hexdigest() for r in results})
    write(output/'result.json',result);print(json.dumps(result,indent=2))

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--output',type=Path,required=True);a=p.parse_args();run(a.output)
if __name__=='__main__':main()
