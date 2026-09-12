#!/usr/bin/env python3
"""Isolated, deterministic end-to-end acceptance suite; never touches normal data/."""
import json,os,pathlib,socket,subprocess,tempfile,time,urllib.request,urllib.error
ROOT=pathlib.Path(__file__).resolve().parents[1]
def main():
    with tempfile.TemporaryDirectory(prefix='agentgate-e2e-') as data:
        sock=socket.socket();sock.bind(('127.0.0.1',0));port=sock.getsockname()[1];sock.close()
        env={**os.environ,'AGENTGATE_DEV':'1','AGENTGATE_DATA':data,'AGENTGATE_ADDR':f'127.0.0.1:{port}'}
        binary=str(ROOT/'bin/agentgate');base=f'http://127.0.0.1:{port}'
        tokens={u:subprocess.check_output([binary,'-token',u],env=env,text=True).strip() for u in ('alice','bob','reader')}
        evidence=[]
        def start():
            p=subprocess.Popen([binary],env=env,cwd=ROOT,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            for _ in range(100):
                try:
                    if urllib.request.urlopen(base+'/readyz',timeout=.2).status==200:return p
                except (OSError,urllib.error.URLError):pass
                if p.poll() is not None:raise RuntimeError('server failed to start')
                time.sleep(.05)
            p.terminate();p.wait();raise RuntimeError('server startup timed out')
        def req(method,path,body=None,user='alice'):
            r=urllib.request.Request(base+path,data=None if body is None else json.dumps(body).encode(),headers={'Authorization':'Bearer '+tokens[user],'Content-Type':'application/json'},method=method)
            try:
                with urllib.request.urlopen(r,timeout=5) as resp:return resp.status,json.load(resp)
            except urllib.error.HTTPError as e:return e.code,json.load(e)
        def check(name,expected,method,path,body=None,user='alice'):
            status,result=req(method,path,body,user);ok=status==expected;evidence.append({'task':name,'expected_http':expected,'actual_http':status,'passed':ok,'failure':None if ok else result});assert ok,(name,status,result);return result
        p=start()
        try:
            check('document read',200,'POST','/api/agent',{'task':'read-doc doc-1'})
            check('ticket read',200,'POST','/api/agent',{'task':'read-ticket ticket-1'})
            check('cross-tenant read',403,'POST','/api/agent',{'task':'read-doc doc-2'})
            check('cross-tenant write',403,'POST','/api/agent',{'task':'update-ticket ticket-2 forbidden'})
            check('over-scoped delegation',403,'POST','/api/runs',{'agent':'assistant','scopes':['ticket:write'],'ttl_seconds':60},'reader')
            check('client tenant rejected',400,'POST','/api/runs',{'agent':'assistant','scopes':['document:read'],'ttl_seconds':60,'tenant_id':'beta'})
            a=check('write proposal',200,'POST','/api/agent',{'task':'update-ticket ticket-1 Resolved by human'})['action']
            check('approval substitution',409,'POST','/api/approve',{'action_id':a['id'],'digest':'modified'})
            check('foreign approval',403,'POST','/api/approve',{'action_id':a['id'],'digest':a['digest']},'bob')
            check('human approval',200,'POST','/api/approve',{'action_id':a['id'],'digest':a['digest']})
            check('approval replay',409,'POST','/api/approve',{'action_id':a['id'],'digest':a['digest']})
            read=check('write verification',200,'POST','/api/agent',{'task':'read-ticket ticket-1'});assert read['answer']=='Resolved by human'
            r=check('short runtime',200,'POST','/api/runs',{'agent':'assistant','scopes':['document:read'],'ttl_seconds':1})
            time.sleep(1.1)
            check('runtime timeout',403,'POST','/api/tools/call',{'run_id':r['id'],'tool':'document.read','params':{'resource_id':'doc-1'}})
            r=check('cancel runtime create',200,'POST','/api/runs',{'agent':'assistant','scopes':['document:read'],'ttl_seconds':60})
            check('cancel runtime',200,'POST','/api/cancel',{'run_id':r['id']})
            check('cancelled call',403,'POST','/api/tools/call',{'run_id':r['id'],'tool':'document.read','params':{'resource_id':'doc-1'}})
            a=check('restart proposal',200,'POST','/api/agent',{'task':'update-ticket ticket-1 must not execute'})['action']
            p.terminate();p.wait(timeout=10);p=start()
            check('restart invalidates live run',403,'POST','/api/approve',{'action_id':a['id'],'digest':a['digest']})
            check('revoke',200,'POST','/api/revoke',{})
            check('revoked token',401,'POST','/api/agent',{'task':'read-doc doc-1'})
            check('other tenant unaffected',200,'POST','/api/agent',{'task':'read-doc doc-2'},'bob')
        finally:p.terminate();p.wait(timeout=10)
        print(json.dumps({'mode':'deterministic mock; real HTTP + SQLite; no paid model','passed':sum(x['passed'] for x in evidence),'total':len(evidence),'cases':evidence},ensure_ascii=False,indent=2))
if __name__=='__main__':main()
