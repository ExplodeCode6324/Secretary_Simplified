#!/usr/bin/env python3
"""Independent read-only reconstruction; never edits the source replay report."""
import argparse, hashlib, json, sqlite3
from pathlib import Path
from urllib.parse import quote
p=argparse.ArgumentParser();p.add_argument('db');p.add_argument('report_dir');p.add_argument('fixture_root');a=p.parse_args()
def sha(b):return hashlib.sha256(b).hexdigest()
def canonical(v):return json.dumps(v,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode()
root=Path(a.report_dir);original=(root/'report.json').read_bytes();report=json.loads(original)
exported=None
if Path(a.db).is_dir():
 exported=json.loads((Path(a.db)/'audit-records.json').read_text());db=None
 events=[e for e in exported['change_events'] if e['event_type'] in ['item.created','item.updated']]
else:
 suffix='?mode=ro' if Path(str(Path(a.db).resolve())+'-wal').exists() else '?mode=ro&immutable=1'
 db=sqlite3.connect('file:'+quote(str(Path(a.db).resolve()))+suffix,uri=True)
 events=[json.loads(r[0]) for r in db.execute("SELECT payload_json FROM change_event WHERE event_type IN ('item.created','item.updated') ORDER BY seq")]

results=[];failures=[];last_boundary=None;stable={}
for point in report['results']:
 i=point['index']
 if exported is None:raw,cutoff=db.execute('SELECT payload_json,updated_at FROM input_turn WHERE request_id=?',(point['request_id'],)).fetchone()
 else:
  row=next(r for r in exported['input_turn_rows'] if r['request_id']==point['request_id']);raw,cutoff=row['payload_json'],row['updated_at']
 turn=json.loads(raw)
 eligible=[e for e in events if e['created_at']<=cutoff];future=[e for e in events if e['created_at']>cutoff];state={}
 for event in eligible:state[event['entity_id']]=event['change']['after']
 mapping={v['title']:id for id,v in state.items()};oracle=json.loads((Path(a.fixture_root)/'oracle'/f'{i:03d}.json').read_text())['items'];actual={v['title']:{k:v[k] for k in ['domain','status','due_at','time_state']} for v in state.values()};mismatch=[]
 if len(mapping)!=len(state):mismatch.append('duplicate title in independent historical event projection')
 if actual!=oracle:mismatch.append('historical state differs from independently authored oracle')
 if turn['state']!='COMMITTED' or turn['updated_at']!=cutoff:mismatch.append('turn not committed or duplicated timestamp inconsistent')
 if last_boundary is not None and cutoff<=last_boundary:mismatch.append('non-increasing physical completion boundary')
 for title,id in mapping.items():
  if title in stable and stable[title]!=id:mismatch.append('persistent ID changed for '+title)
  stable[title]=id
 added=[e for e in eligible if last_boundary is None or e['created_at']>last_boundary]
 if len(added)!=1:mismatch.append('expected exactly one Item event in this isolated checkpoint interval')
 results.append({'index':i,'request_id':point['request_id'],'turn_id':turn['id'],'turn_payload_sha256':sha(raw.encode()),'commit_updated_at':cutoff,'previous_commit_updated_at':last_boundary,'interval_item_events':[{'seq':e['seq'],'id':e['id'],'entity_id':e['entity_id'],'revision':e['entity_revision'],'created_at':e['created_at']} for e in added],'last_included_item_seq':eligible[-1]['seq'] if eligible else None,'first_excluded_item_event':{'seq':future[0]['seq'],'created_at':future[0]['created_at']} if future else None,'item_ids':mapping,'oracle_title_count':len(oracle),'actual_title_count':len(mapping),'status':'FAIL' if mismatch else 'PASS','mismatches':mismatch})
 failures+=mismatch;last_boundary=cutoff
output={'source_report_sha256':sha(original),'source_item_events_sha256':sha(canonical(events)),'method':'Read immutable Item change_event rows in seq order; for each InputTurn physical updated_at retain only Item events created_at <= that timestamp, replay by entity_id, then compare the resulting title set and fields to independent oracle. Verify increasing non-overlapping completion boundaries and exactly one Item event per checkpoint. Original aliased item_ids map is never read for reconstruction.','status':'FAIL' if failures else 'PASS','failures':failures,'results':results}
(root/'report-identity-correction.json').write_text(json.dumps(output,ensure_ascii=False,indent=2)+'\n')
snapshots=exported['consciousness_snapshots'] if exported is not None else [json.loads(r[0]) for r in db.execute('SELECT payload_json FROM consciousness_snapshot ORDER BY slot')];audits=[]
for snap in snapshots:
 state={}
 for event in events:
  if event['seq']<=snap['snapshot_seq']:state[event['entity_id']]=event['change']['after']
 bad=[];refs=[]
 for group in ['focal_goals','priority_items','open_loops','important_changes']:
  for entry in snap[group]:
   ref=entry['entity'];source=state.get(ref['id']);ok=ref['entity_type']=='Item' and source is not None and source['revision']==ref['revision'];refs.append({'group':group,'ref':ref,'matches_immutable_event_revision':ok})
   if not ok:bad.append(ref)
 audits.append({'slot':snap['slot'],'snapshot_id':snap['id'],'snapshot_seq':snap['snapshot_seq'],'reference_checks':refs,'status':'FAIL' if bad else 'PASS','summary':snap['brief_summary'],'historical_item_count':len(state),'historical_open_count':sum(x['status'] in ['OPEN','IN_PROGRESS','BLOCKED'] for x in state.values())})
(root/'consciousness-reference-audit.json').write_text(json.dumps({'scope':'Independent immutable event/revision verification, not complete free-text correctness','count':len(audits),'status':'PASS' if len(audits)==30 and all(x['status']=='PASS' for x in audits) else 'FAIL','results':audits},ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'identity_checkpoints':len(results),'identity_status':output['status'],'snapshot_count':len(audits),'snapshot_reference_failures':sum(x['status']=='FAIL' for x in audits)}))
