"""Constructed states test contracts; end-to-end DB observations are separate."""
import copy,sys,unittest
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'scripts'))
from state_evidence import trace_digest,score_state

def fixture():
    case={'id':'write','body':'resolved'}
    trace=[{'path':'chat','request':{'task':'write'},'http':200,'response':{'run_id':'run'}}, {'path':'approve','request':{'action_id':'a','digest':'d'},'http':200,'response':{}}, {'path':'approve','request':{'action_id':'a','digest':'d'},'http':409,'response':{}}]
    before={'tenant':'acme','id':'ticket-1','owner':'alice','kind':'ticket','body':'open','version':1}
    action={'id':'a','run_id':'run','tool':'ticket.update','params':{'resource_id':'ticket-1','body':'resolved'},'digest':'d','status':'pending','tenant':'acme','user_id':'alice'}
    observations=[]
    for n in range(4):
        resource=copy.deepcopy(before);actions=[] if n==0 else [copy.deepcopy(action)]
        if n>=2:resource.update(body='resolved',version=2);actions[0]['status']='succeeded'
        observations.append({'trace_count':n,'trace_sha256':trace_digest(trace[:n]),'resource':resource,'actions':actions})
    raw={'trace':trace,'state_evidence':{'schema':'ticket-state-v1','case_id':'write','observations':observations}}
    return case,raw

class StateTests(unittest.TestCase):
    def test_missing_history_is_unknown(self):self.assertIsNone(score_state({'body':'x'},{'trace':[]}))
    def test_approval_and_replay_have_one_version_increment(self):
        c,r=fixture();self.assertTrue(score_state(c,r))
    def test_early_or_repeated_write_fails(self):
        for index in (1,3):
            c,r=fixture();r['state_evidence']['observations'][index]['resource']['version']+=1
            self.assertFalse(score_state(c,r))
    def test_wrong_approval_binding_fails(self):
        c,r=fixture();r['state_evidence']['observations'][1]['actions'][0]['digest']='other'
        self.assertFalse(score_state(c,r))
    def test_partial_or_rebound_evidence_rejected(self):
        for mutation in ('missing','rebound','bool'):
            c,r=fixture();e=r['state_evidence']
            if mutation=='missing':e['observations'].pop()
            elif mutation=='rebound':e['case_id']='other'
            else:e['observations'][0]['resource']['version']=True
            with self.assertRaises(ValueError):score_state(c,r)
    def test_cancel_does_not_mean_no_write_if_db_changed(self):
        c,r=fixture();c['action']='cancel'
        self.assertFalse(score_state(c,r))
    def test_legacy_result_boolean_is_ignored(self):
        c,r=fixture();r.pop('state_evidence');r['write_state_correct']=True
        self.assertIsNone(score_state(c,r))
if __name__=='__main__':unittest.main()
