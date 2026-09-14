#!/usr/bin/env python3
"""Check the requested public working-tree candidate without printing secrets."""
import argparse, hashlib, json, re, subprocess
from pathlib import Path

def main():
 p=argparse.ArgumentParser();p.add_argument('--report',default='reports/implementation/publication-scan.json');a=p.parse_args()
 root=Path(__file__).resolve().parents[1]
 values=[]
 for source in (root/'resource').glob('*.md'):
  if 'api' not in source.name.lower():continue
  text=source.read_text().strip()
  candidates=[x.strip().strip('`') for x in text.splitlines() if re.fullmatch(r'[A-Za-z0-9_-]{20,}',x.strip().strip('`'))]
  if not candidates:candidates=re.findall(r'sk-[A-Za-z0-9_-]{20,}',text)
  if len(set(candidates))!=1:raise SystemExit('Credential source parsing failed; no values emitted')
  values.append(candidates[0].encode())
 if len(values)!=2:raise SystemExit('Expected two explicit credential sources')
 names=set(subprocess.check_output(['git','ls-files','-co','--exclude-standard','-z'],cwd=root).split(b'\0'))
 failures=[];count=0;size=0
 forbidden={'release/secrets','release/state','release/run','release/logs','release/objects','release/reports','reports/local','review/private'}
 for raw in sorted(names):
  if not raw:continue
  rel=raw.decode();path=root/rel
  if not path.is_file() or rel==a.report:continue
  if any(rel==x or rel.startswith(x+'/') for x in forbidden) or rel.endswith(('.key','.migration.lock','.stderr.log','.usage.json')):
   failures.append({'file':rel,'reason':'private_or_runtime_file'})
  data=path.read_bytes();count+=1;size+=len(data)
  if any(v in data for v in values):failures.append({'file':rel,'reason':'credential_bytes'})
  if str(Path.home()).encode() in data:failures.append({'file':rel,'reason':'private_home_path'})
  if path.suffix in {'.md','.py','.go','.json','.txt','.sh','.sql'}:
   try:data.decode('utf-8')
   except UnicodeDecodeError:failures.append({'file':rel,'reason':'invalid_utf8'})
 result={'scope':'tracked plus unignored candidate files; actual credential bytes and private paths never emitted','status':'PASS' if not failures else 'FAIL','files_scanned':count,'bytes_scanned':size,'actual_credential_sources_checked':len(values),'failures':failures,'binary_sha256':{n:hashlib.sha256((root/'release'/n).read_bytes()).hexdigest() for n in ['secretary','secretaryd']}}
 out=root/a.report;out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
 return bool(failures)
if __name__=='__main__':raise SystemExit(main())
