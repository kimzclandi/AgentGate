#!/usr/bin/env python3
"""Check repository-local Markdown links and accidental runtime artifacts."""
from pathlib import Path
import re, subprocess
from urllib.parse import unquote
root=Path(__file__).resolve().parents[1]
tracked=subprocess.check_output(['git','ls-files'],cwd=root,text=True).splitlines()
untracked=subprocess.check_output(['git','ls-files','--others','--exclude-standard'],cwd=root,text=True).splitlines()
files=[root/p for p in sorted(set(tracked+untracked)) if (root/p).is_file()]
errors=[];links=0
for file in files:
    rel=file.relative_to(root)
    if file.suffix in ('.db','.key','.gguf','.dylib') or rel.parts[0] in ('bin','data','work'):
        errors.append(f'runtime artifact: {rel}')
    if file.suffix!='.md':continue
    text=re.sub(r'```.*?```','',file.read_text(),flags=re.S)
    for target in re.findall(r'\]\(([^)]+)\)',text):
        target=target.strip().split(' "',1)[0].strip('<>')
        if re.match(r'^[a-zA-Z][a-zA-Z0-9+.-]*:',target) or target.startswith('#'):continue
        path=unquote(target.split('#',1)[0])
        if path:
            links+=1
            resolved=(file.parent/path).resolve()
            if not resolved.is_relative_to(root) or not resolved.exists():
                errors.append(f'{rel}: missing or external local target {target}')
if errors:raise SystemExit('\n'.join(errors))
print(f'PASS: {links} local Markdown links; {len(files)} source files inspected for runtime artifacts (not a full secret scan).')
