#!/usr/bin/env python3
"""Copy allowlisted synthetic evidence with object-closure/hash/class checks.
Never copies config, credentials, authorization files, runtime databases or logs.
"""
import argparse,hashlib,json,sqlite3
from pathlib import Path
from urllib.parse import quote
p=argparse.ArgumentParser();p.add_argument('db');p.add_argument('local_report');p.add_argument('destination');a=p.parse_args();dbpath=Path(a.db).resolve();local=Path(a.local_report).resolve();dest=Path(a.destination)
if dest.exists():raise SystemExit('Destination already exists; refusing overwrite')
suffix='?mode=ro' if Path(str(dbpath)+'-wal').exists() else '?mode=ro&immutable=1'
db=sqlite3.connect('file:'+quote(str(dbpath))+suffix,uri=True);db.row_factory=sqlite3.Row
staged={};wanted=set()
def inspect(v):
 if isinstance(v,dict):
  if v.get('data_class') in ['PERSONAL','SENSITIVE','SECRET']:raise ValueError('Non-synthetic data class encountered')
  if isinstance(v.get('object_id'),str):wanted.add(v['object_id'])
  if isinstance(v.get('relative_path'),str) and isinstance(v.get('sha256'),str) and isinstance(v.get('id'),str):wanted.add(v['id'])
  for x in v.values():inspect(x)
 elif isinstance(v,list):
  for x in v:inspect(x)
 elif isinstance(v,str) and v.lstrip().startswith(('{','[')):
  try:x=json.loads(v)
  except (ValueError,RecursionError):return
  inspect(x)
def add(name,raw):
 try:inspect(json.loads(raw))
 except (UnicodeDecodeError,json.JSONDecodeError):pass
 staged[name]=raw
for f in local.rglob('*.json'):
 rel=f.relative_to(local)
 if any(x in {'data','state','objects','run','secrets','logs'} for x in rel.parts):continue
 if 'config' in f.name or 'token' in f.name or 'grant' in f.name:continue
 add(rel.as_posix(),f.read_bytes())
audit={'input_turn_rows':[dict(r) for r in db.execute('SELECT request_id,updated_at,payload_json FROM input_turn ORDER BY rowid')],'change_events':[json.loads(r[0]) for r in db.execute('SELECT payload_json FROM change_event ORDER BY seq')],'consciousness_snapshots':[json.loads(r[0]) for r in db.execute('SELECT payload_json FROM consciousness_snapshot ORDER BY slot')]}
add('audit-records.json',(json.dumps(audit,ensure_ascii=False,indent=2)+'\n').encode())
for table in ['context_manifest','decision_record','world_fact_version','world_proposal']:
 rows=[json.loads(r[0]) for r in db.execute('SELECT payload_json FROM '+table+' ORDER BY rowid')]
 add(table+'.json',(json.dumps(rows,ensure_ascii=False,indent=2)+'\n').encode())
objects=dbpath.parent.parent/'objects';copied={}
while wanted-set(copied):
 id=sorted(wanted-set(copied))[0];r=db.execute('SELECT * FROM object_ref WHERE id=?',(id,)).fetchone()
 if r is None:raise ValueError('Referenced object missing')
 ref=dict(r)
 if ref['data_class']!='SYNTHETIC':raise ValueError('Referenced object is not synthetic')
 rel=Path(ref['relative_path'])
 if rel.is_absolute() or '..' in rel.parts:raise ValueError('Unsafe object relative path')
 raw=(objects/rel).read_bytes()
 if len(raw)!=ref['byte_size'] or hashlib.sha256(raw).hexdigest()!=ref['sha256']:raise ValueError('Object integrity failure')
 copied[id]=ref;add('objects/'+rel.as_posix(),raw)
add('object-index.json',(json.dumps(list(copied.values()),ensure_ascii=False,indent=2)+'\n').encode())
# Scan actual credential bytes without emitting any credential or matching snippet.
repo=Path(__file__).resolve().parents[1];credential_sources=list((repo/'resource').glob('*.md'));secret_values=[]
import re
for source in credential_sources:
 if 'api' not in source.name.lower():continue
 text=source.read_text().strip();candidates=[x.strip().strip('`') for x in text.splitlines() if re.fullmatch(r'[A-Za-z0-9_-]{20,}',x.strip().strip('`'))]
 if not candidates:candidates=re.findall(r'sk-[A-Za-z0-9_-]{20,}',text)
 if len(set(candidates))!=1:raise ValueError('Credential scan requires exactly one identifiable key per credential source')
 secret_values.append(candidates[0].encode())
if len(secret_values)!=2:raise ValueError('Expected two actual credential sources for exact-byte scan')
private_markers=[b'/Users/',str(dbpath.parent.parent).encode(),str(local).encode()]
for name,raw in staged.items():
 if any(key in raw for key in secret_values):raise ValueError('Credential scan failed')
 if any(x in raw for x in private_markers):raise ValueError('Private path scan failed for '+name)
manifest={'scope':'SYNTHETIC public review copy; no runtime DB/config/token/grant files','files':[{'path':name,'sha256':hashlib.sha256(raw).hexdigest(),'bytes':len(raw)} for name,raw in sorted(staged.items())],'object_count':len(copied),'scan':{'actual_credential_sources':2,'exact_key_matches':0,'home_or_private_data_path_matches':0,'non_synthetic_objects':0},'notes':'Source report bytes preserved. Object closure follows embedded Context/request/output JSON and EvidenceRef/ObjectRef references; every object checked against authoritative metadata hash/size/class before copying.'}
staged['publication-manifest.json']=(json.dumps(manifest,indent=2)+'\n').encode();dest.mkdir(parents=True)
for name,raw in staged.items():
 f=dest/name;f.parent.mkdir(parents=True,exist_ok=True);f.write_bytes(raw)
print(json.dumps({'destination':dest.as_posix(),'files':len(staged),'objects':len(copied),'bytes':sum(map(len,staged.values())),'scan':'PASS','credential_sources_checked':2}))
