"""Mutation tests for saved evidence; these are not new model generations."""
import copy
import importlib.util
import json
from pathlib import Path
import unittest

ROOT=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('scorer', ROOT/'scripts/score_trajectory.py')
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
FOLDER=ROOT/'docs/evidence/multiturn-v1'
CASES={c['id']:c for c in json.loads((FOLDER/'protocol.json').read_text())['cases']}

def load(name):
    return copy.deepcopy(CASES[name]),json.loads((FOLDER/(name+'.json')).read_text())

class EvidenceContractTests(unittest.TestCase):
    def test_duplicate_followup_task_cannot_hide_earlier_tool_calls(self):
        case = next(c for c in CASES.values() if c.get('followup'))
        case,raw=load(case['id'])
        raw['persisted_chats'][-1]['messages'].append({'role':'user','content':case['followup']})
        with self.assertRaises(ValueError):module.score(case,raw)
    def test_server_failure_is_not_expected_stop(self):
        for name in ('cancel','restart'):
            with self.subTest(name=name):
                case,raw=load(name)
                raw['trace'][-1].update(http=500,response={'error':'internal_error'})
                self.assertFalse(module.score(case,raw)['expected_stop_http'])
    def test_unrelated_http_error_is_not_expected_stop(self):
        case,raw=load('cancel');raw['trace'][-1]['path']='overview'
        self.assertFalse(module.score(case,raw)['expected_stop_http'])
    def test_forbidden_error_must_be_permission_denial(self):
        case,raw=load('foreign');raw['trace'][0]['response']={'error':'upstream_error'}
        self.assertFalse(module.score(case,raw)['permission_denied'])
    def test_empty_or_string_quote_rubric_cannot_vacuously_pass(self):
        for quotes in ([],[''],'Open: customer needs help'):
            with self.subTest(quotes=quotes):
                case,raw=load('document');case['required_quotes']=quotes
                with self.assertRaises(ValueError):module.score(case,raw)
    def test_missing_initial_request_is_schema_error(self):
        case,raw=load('document');raw['trace'][0]['request']=None
        with self.assertRaises(ValueError):module.score(case,raw)
    def test_write_rubric_requires_explicit_body(self):
        case,raw=load('approved-write');del case['body']
        with self.assertRaises(ValueError):module.score(case,raw)
    def test_failed_cancel_call_cannot_count_as_control_success(self):
        case,raw=load('cancel')
        next(t for t in raw['trace'] if t['path']=='cancel')['http']=500
        self.assertFalse(module.score(case,raw)['expected_stop_http'])
    def test_swapped_task_evidence_is_rejected(self):
        case,raw=load('document');raw['trace'][0]['request']['task']='a different task'
        with self.assertRaises(ValueError):module.score(case,raw)
    def test_unrelated_persisted_chat_is_rejected(self):
        case,raw=load('document');raw['persisted_chats'][0]['id']='another-chat'
        with self.assertRaises(ValueError):module.score(case,raw)

if __name__=='__main__':unittest.main()
