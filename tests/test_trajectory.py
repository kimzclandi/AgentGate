import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
ROOT=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('scorer',ROOT/'scripts/score_trajectory.py')
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)

class TrajectoryTests(unittest.TestCase):
    def setUp(self):
        self.case=dict(id='two-resources',tools=['document_read','ticket_read'],answer='Open: customer needs help')
        self.raw=dict(trace=[dict(http=200)],persisted_chats=[dict(status='succeeded',answer='Open: customer needs help',messages=[dict(tool_calls=[
            dict(function=dict(name='document_read',arguments=dict(resource_id='doc-1'))),
            dict(function=dict(name='ticket_read',arguments=dict(resource_id='ticket-1')))])])])
    def test_partial_answer_does_not_pass_two_resource_task(self):
        r=module.score(self.case,self.raw)
        self.assertTrue(r['flow_succeeded']);self.assertFalse(r['final_required_quotes_present'])
    def test_extra_or_duplicate_tool_fails(self):
        self.raw['persisted_chats'][0]['messages'][0]['tool_calls']*=2
        self.assertFalse(module.score(self.case,self.raw)['tool_selection_exact'])
    def test_wrong_argument_separate_from_tool_selection(self):
        self.raw['persisted_chats'][0]['messages'][0]['tool_calls'][0]['function']['arguments']['resource_id']='doc-2'
        r=module.score(self.case,self.raw)
        self.assertTrue(r['tool_selection_exact']);self.assertFalse(r['parameters_exact'])
    def test_denied_has_no_answer_score_or_db_claim(self):
        self.case.update(denied=True);self.raw['trace'][0]['http']=403
        r=module.score(self.case,self.raw)
        self.assertTrue(r['permission_denied']);self.assertIsNone(r['final_required_quotes_present']);self.assertIsNone(r['write_state_correct'])
    def test_followup_tools_are_scored_after_latest_user_only(self):
        self.case['followup']='Repeat without a tool'
        messages=self.raw['persisted_chats'][0]['messages']
        messages.append(dict(role='user',content=self.case['followup']))
        self.assertTrue(module.score(self.case,self.raw)['followup_no_tools'])
        messages.append(copy.deepcopy(messages[0]))
        self.assertFalse(module.score(self.case,self.raw)['followup_no_tools'])
    def test_missing_trace_rejected(self):
        self.raw['trace']=[]
        with self.assertRaises(ValueError):module.score(self.case,self.raw)
    def test_result_id_coverage_required(self):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp);(p/'protocol.json').write_text(json.dumps(dict(cases=[self.case])))
            (p/'results.json').write_text('[]')
            with self.assertRaisesRegex(ValueError,'coverage'):module.evaluate(p)
    def test_historical_replay_denominators(self):
        r=module.evaluate(ROOT/'docs/evidence/multiturn-v1')
        self.assertEqual(len(r['cases']),8)
        self.assertEqual(r['metrics']['final_required_quotes_present']['n'],5)
        self.assertEqual(r['metrics']['write_state_correct']['n'],0)

if __name__=='__main__':unittest.main()
