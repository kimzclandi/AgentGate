"""Synthetic SQLite reconstruction from public traces; no inference."""
import json,sqlite3,subprocess,sys,tempfile,unittest
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
class ExportTests(unittest.TestCase):
    def setUp(self):
        (ROOT/'work').mkdir(exist_ok=True)
        self.temp=tempfile.TemporaryDirectory(dir=ROOT/'work');self.addCleanup(self.temp.cleanup)
        self.base=Path(self.temp.name);self.source=self.base/'source';self.source.mkdir();self.out=self.base/'export'
        saved=ROOT/'docs/evidence/multiturn-v1'
        for f in ('protocol.json','results.json','source_hashes.json'):(self.source/f).write_bytes((saved/f).read_bytes())
        self.cases=json.loads((saved/'protocol.json').read_text())['cases']
        for c in self.cases:
            d=self.source/c['id'];d.mkdir();raw=json.loads((saved/(c['id']+'.json')).read_text())
            (d/'trace.json').write_text(json.dumps(raw['trace']))
            with sqlite3.connect(d/'agentgate.db') as con:
                con.execute('CREATE TABLE chats (id TEXT,run_id TEXT,messages TEXT,status TEXT,answer TEXT)')
                con.executemany('INSERT INTO chats VALUES (?,?,?,?,?)',[(r['id'],r['run_id'],json.dumps(r['messages']),r['status'],r.get('answer','')) for r in raw['persisted_chats']])
    def run_export(self):
        return subprocess.run([sys.executable,str(ROOT/'scripts/export_multiturn.py'),'--source',str(self.source),'--out',str(self.out)],capture_output=True,text=True)
    def state_fixture(self):
        sys.path.insert(0,str(ROOT/'scripts'))
        from state_evidence import capture
        case=next(c for c in self.cases if c['id']=='document')
        protocol=json.loads((self.source/'protocol.json').read_text());protocol['cases']=[case];protocol['state_evidence_schema']='ticket-state-v1'
        (self.source/'protocol.json').write_text(json.dumps(protocol))
        (self.source/'results.json').write_text(json.dumps([{'id':case['id']}]))
        folder=self.source/case['id'];database=folder/'agentgate.db'
        with sqlite3.connect(database) as con:
            con.execute('CREATE TABLE resources (tenant,id,owner,kind,body,version)')
            con.execute("INSERT INTO resources VALUES ('acme','ticket-1','alice','ticket','open',1)")
            con.execute('CREATE TABLE actions (id,run_id,tool,params,digest,status,tenant,user_id)')
        trace=json.loads((folder/'trace.json').read_text())
        observations=[capture(database,trace[:n]) for n in range(len(trace)+1)]
        (folder/'state-evidence.json').write_text(json.dumps({'schema':'ticket-state-v1','case_id':case['id'],'observations':observations}))
        return folder
    def test_state_export_survives_source_database_removal(self):
        folder=self.state_fixture();result=self.run_export();self.assertEqual(result.returncode,0,result.stderr)
        (folder/'agentgate.db').unlink()
        sys.path.insert(0,str(ROOT/'scripts'))
        from score_trajectory import evaluate
        result=evaluate(self.out)
        self.assertIsNone(result['cases'][0]['write_state_correct'])
        self.assertIn('state_evidence',json.loads((self.out/'document.json').read_text()))
    def test_live_database_drift_blocks_export(self):
        folder=self.state_fixture()
        with sqlite3.connect(folder/'agentgate.db') as con:con.execute("UPDATE resources SET body='changed'")
        self.assertNotEqual(self.run_export().returncode,0);self.assertFalse(self.out.exists())
    def test_declared_state_contract_cannot_silently_downgrade(self):
        folder=self.state_fixture();(folder/'state-evidence.json').unlink()
        self.assertNotEqual(self.run_export().returncode,0);self.assertFalse(self.out.exists())
    def test_missing_result_fails_without_partial_export(self):
        p=self.source/'results.json';p.write_text(json.dumps(json.loads(p.read_text())[:-1]))
        self.assertNotEqual(self.run_export().returncode,0);self.assertFalse(self.out.exists())
    def test_missing_database_is_not_created(self):
        p=self.source/self.cases[-1]['id']/'agentgate.db';p.unlink()
        self.assertNotEqual(self.run_export().returncode,0);self.assertFalse(p.exists());self.assertFalse(self.out.exists())
    def test_invalid_last_trace_leaves_no_output(self):
        p=self.source/self.cases[-1]['id']/'trace.json';p.write_text('[]')
        self.assertNotEqual(self.run_export().returncode,0);self.assertFalse(self.out.exists())
        self.assertFalse(list(self.base.glob('.export-*')))
    def test_duplicate_ids_rejected(self):
        p=self.source/'results.json';rows=json.loads(p.read_text());rows[-1]=rows[0]
        p.write_text(json.dumps(rows))
        self.assertNotEqual(self.run_export().returncode,0);self.assertFalse(self.out.exists())
    def test_shuffled_legacy_booleans_cannot_change_scores(self):
        p=self.source/'results.json';rows=json.loads(p.read_text())
        for r in rows:r.update(flow_completed=False,final_answer_correct=False)
        p.write_text(json.dumps(rows[::-1]));result=self.run_export();self.assertEqual(result.returncode,0,result.stderr)
        scores=json.loads((self.out/'scores.json').read_text())
        self.assertEqual(scores['metrics']['tool_selection_exact']['passed'],8)
        self.assertEqual(scores['metrics']['flow_succeeded']['passed'],5)
        self.assertEqual(scores['metrics']['final_required_quotes_present']['passed'],3)
if __name__=='__main__':unittest.main()
