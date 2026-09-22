"""Export seeded multiturn DB transcripts, then independently replay the scorer.

Run only on completed, quiescent evaluations. No model call or database copy.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import sqlite3
import tempfile

from score_trajectory import evaluate, read_protocol


def export(source, output):
    source, output = Path(source).resolve(), Path(output).resolve()
    root = Path(__file__).resolve().parents[1]
    if not output.is_relative_to(root / 'work') or output == root / 'work':
        raise ValueError('Output must be a new repository work/ child')
    if output.exists():
        raise FileExistsError(output)
    _, cases = read_protocol(source)  # Reject incomplete or duplicate result coverage first.
    payloads = {name: (source / name).read_bytes()
                for name in ('protocol.json', 'results.json', 'source_hashes.json')}
    for case in cases:
        directory = source / case['id']
        trace = json.loads((directory / 'trace.json').read_text())
        database = directory / 'agentgate.db'
        # mode=ro prevents accidental creation and any writes to the source DB.
        connection = sqlite3.connect(database.as_uri() + '?mode=ro', uri=True)
        try:
            connection.row_factory = sqlite3.Row
            chats = [dict(row) for row in connection.execute(
                'SELECT id,run_id,messages,status,answer FROM chats ORDER BY rowid')]
        finally:
            connection.close()
        for chat in chats:
            chat['messages'] = json.loads(chat['messages'])
        payloads[case['id'] + '.json'] = (json.dumps(
            {'trace': trace, 'persisted_chats': chats}, ensure_ascii=False, indent=2) + '\n').encode()
    output.parent.mkdir(parents=True, exist_ok=True)
    stage = Path(tempfile.mkdtemp(prefix='.export-', dir=output.parent))
    try:
        for name, content in payloads.items():
            (stage / name).write_bytes(content)
        scores = evaluate(stage)  # Legacy booleans are never copied into new scores.
        scores['receipt'] = {
            'input_sha256': {name: hashlib.sha256(data).hexdigest() for name, data in payloads.items()},
            'exporter_sha256': hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
            'scorer_sha256': hashlib.sha256((Path(__file__).parent / 'score_trajectory.py').read_bytes()).hexdigest(),
            'source_state': 'Caller must stop the evaluation before export; not a cross-case live DB snapshot.'}
        (stage / 'scores.json').write_text(json.dumps(scores, ensure_ascii=False, indent=2) + '\n')
        if output.exists():
            raise FileExistsError(output)
        os.rename(stage, output)
        return scores
    finally:
        if stage.exists():
            shutil.rmtree(stage)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    print(json.dumps(export(args.source, args.out)['metrics'], indent=2))


if __name__ == '__main__':
    main()
