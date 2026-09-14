#!/usr/bin/env python3
"""Export an allowlisted, public reviewer provenance manifest from local Hermes metadata."""
from pathlib import Path
import argparse, datetime, hashlib, json, re, subprocess
p=argparse.ArgumentParser()
p.add_argument('reports', nargs='+')
a=p.parse_args()
root=Path(__file__).resolve().parents[1]
records=[]
cache={}
def sha(data): return hashlib.sha256(data).hexdigest()
def utc(v): return datetime.datetime.fromtimestamp(v, datetime.timezone.utc).isoformat().replace('+00:00','Z') if v else None
for arg in a.reports:
 report=root/arg
 stem=report.name.removesuffix('.response.md')
 stderr=report.with_name(stem+'.stderr.log')
 ids=re.findall(r'\b\d{8}_\d{6}_[0-9a-f]{6}\b',stderr.read_text())
 if not ids: raise RuntimeError('No session provenance for '+report.name)
 sid=ids[-1]
 if sid not in cache:
  res=subprocess.run([str(Path.home()/'.local/bin/hermes'),'sessions','export','-','--session-id',sid,'--redact','--yes'],capture_output=True,text=True,check=True)
  cache[sid]=json.loads(res.stdout)
 d=cache[sid]
 rawpath=report.parent/'private'/report.name
 raw=rawpath.read_bytes() if rawpath.exists() else report.read_bytes()
 rawtext=raw.decode().strip()
 candidates=[]
 for message in d.get('messages',[]):
  if message.get('role')!='assistant': continue
  candidates.append({**message,'evidence_kind':'assistant_final_text'})
  for call in message.get('tool_calls') or []:
   function=call.get('function',{})
   if function.get('name')!='write_file': continue
   try: args=json.loads(function.get('arguments','{}'))
   except (ValueError,TypeError): continue
   if Path(args.get('path','')).name==report.name:
    candidates.append({**message,'content':args.get('content',''),'evidence_kind':'assistant_write_file_tool_argument'})
 matches=[m for m in candidates if str(m.get('content','')).strip()==rawtext]
 match_kind='exact'
 if not matches:
  matches=[m for m in candidates if '***' in str(m.get('content','')) and re.fullmatch(re.escape(str(m.get('content','')).strip()).replace(r'\*\*\*',r'.{1,200}?'),rawtext,flags=re.S)]
  match_kind='redacted_export_equivalent'
 if not matches: raise RuntimeError('Report does not match persisted model-authored content: '+report.name)
 m=matches[-1]
 if d.get('model')!='deepseek-v4.1-flash' or d.get('billing_provider')!='opencode-go': raise RuntimeError('Unexpected reviewer route for '+report.name)
 rec={'report':str(report.relative_to(root)), 'provider':d['billing_provider'], 'model_id':d['model'], 'response_at_utc':utc(m.get('timestamp')), 'response_sha256':sha(report.read_bytes()), 'raw_response_sha256':sha(raw), 'persisted_assistant_response_match':match_kind, 'authored_evidence_kind':m['evidence_kind'], 'exported_assistant_response_sha256':sha(str(m.get('content','')).encode()), 'metadata_source':'Hermes persisted session model and billing_provider; stored assistant response match; redaction-equivalent cases explicitly labeled', 'scope':'Review report only; not proof of product acceptance or provider-side model attestation'}
 records.append(rec)
 private=report.parent/'private'/(stem+'.provenance.json')
 private.parent.mkdir(exist_ok=True)
 private.write_text(json.dumps({**rec,'session_id':sid,'session_started_at_utc':utc(d.get('started_at')),'session_api_call_count':d.get('api_call_count')},ensure_ascii=False,indent=2)+'\n')
out={'schema_version':1,'generated_at_utc':utc(datetime.datetime.now().timestamp()),'reviewer_requirement':'DeepSeek v4.1 Flash only; Luna historical reports excluded','records':records}
(root/'review/model-provenance.json').write_text(json.dumps(out,ensure_ascii=False,indent=2)+'\n')
print('Wrote',len(records),'verified reviewer provenance records')
