#!/usr/bin/env python3
"""Build retrospective A23 ledger v2 from archived frozen component evidence.
Never writes fixtures, prior run reports, v1 ledger, or production source.
"""
import json,hashlib,pathlib,datetime,collections,argparse
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--root',type=pathlib.Path,default=pathlib.Path(__file__).resolve().parents[1])
args=parser.parse_args()
R=args.root.resolve()
def load(p):return json.loads((R/p).read_text())
def digest(p):return hashlib.sha256((R/p).read_bytes()).hexdigest()
def canon(v):return hashlib.sha256(json.dumps(v,ensure_ascii=False,sort_keys=True,separators=(',',':')).encode()).hexdigest()
def asset(p):
 p=pathlib.Path(p);return {'path':p.as_posix(),'sha256':digest(p),'visibility':'LOCAL_ONLY_NOT_PUBLISHED' if str(p).startswith('reports/local/') else 'PUBLIC'} if (R/p).exists() else None
runs=[];cases=[];issues=[]
month=pathlib.Path('src/tests/fixtures/month');manifest=load(month/'manifest.json');entries={x['path']:x['sha256'] for x in manifest['files']}
for name,h in entries.items():
 if digest(month/name)!=h:issues.append('month manifest mismatch '+name)
inputs=[load(month/f'input/{i:03}.json') for i in range(90)];oracles=[load(month/f'oracle/{i:03}.json') for i in range(90)]
request_to_case={};month_calls={};month_reports={}
for n in range(1,9):
 name=f'month-run{n}'+('-failed' if n==7 else '');base=pathlib.Path('reports/live-model')/name;report=load(base/'report.json');month_reports[name]=report;byrequest={x['request_id']:x['index'] for x in report['results']};callgroups=collections.defaultdict(list);unassigned=[]
 for p in sorted((R/base/'model_calls').glob('*.json')):
  if p.name.endswith('.request.json'):continue
  rel=p.relative_to(R);m=load(rel);request=rel.with_name(p.stem+'.request.json');entry={'record':asset(rel),'call_id':m.get('call_id'),'status':m.get('status'),'error_code':m.get('error_code'),'started_at':m.get('started_at'),'finished_at':m.get('finished_at'),'provider_profile':m.get('provider_profile'),'model_id':m.get('model_id'),'context_id':m.get('context_id')}
  key=None
  if (R/request).exists():
   ref=load(request);blob=base/'objects'/ref['relative_path'];entry['request_ref']=asset(request);entry['request_object']=asset(blob);entry['request_object_hash_matches']=bool((R/blob).exists() and digest(blob)==ref['sha256'])
   if entry['request_object_hash_matches']:
    wire=load(blob)
    for message in wire.get('input',wire.get('messages',[])):
     if message.get('role')!='user':continue
     try:context=json.loads(message['content'])
     except (ValueError,TypeError):continue
     if 'current_input' in context:
      inp=context['current_input'];i=byrequest.get(inp.get('request_id'));key=f'month.item.{i:03}' if i is not None else None
      if i is not None:entry['authored_input_text_matches']=inp.get('text')==inputs[i]['text'];entry['request_id']=inp.get('request_id')
     elif 'slot' in context:key=f'month.consciousness.{context["slot"]:02}'
   else:issues.append('missing/corrupt archived request '+str(blob))
  out=m.get('output_ref');entry['output_object']=asset(base/'objects'/out['relative_path']) if out else None
  if key:callgroups[key].append(entry)
  else:unassigned.append(entry)
 month_calls[name]=callgroups
 runs.append({'run_id':name,'family':'month','report':asset(base/'report.json'),'results_recorded':len(report['results']),'reported_failures':report['failures'],'item_passes':sum(x.get('status')=='PASS' for x in report['results']),'consciousness_passes':sum(x.get('consciousness_status')=='PASS' for x in report['results']),'call_count':sum(map(len,callgroups.values()))+len(unassigned),'unassigned_calls':unassigned,'oracle_manifest':asset(month/'manifest.json'),'input_oracle_manifest_hashes_verified':True,'identity_correction':asset(base/'report-identity-correction.json'),'reference_audit':asset(base/'consciousness-reference-audit.json'),'raw_call_history_visibility':'PUBLIC','not_reached_points':list(range(len(report['results']),90))})
for i,inp in enumerate(inputs):
 tags=['A23.30_days_three_points_per_day','A23.exact_ID_status_time_null','A23.four_domains_five_items']
 previous=oracles[i-1]['items'] if i else {};now=oracles[i]['items'];changes=[]
 for name,obj in now.items():
  old=previous.get(name)
  if old is None:changes.append('creation')
  elif obj!=old:
   if obj.get('status')!=old.get('status'):changes.append('completion' if obj['status']=='DONE' else 'cancellation' if obj['status']=='CANCELLED' else 'status_change')
   if obj.get('due_at')!=old.get('due_at'):changes.append('unknown_time' if obj.get('due_at') is None else 'rescheduling')
 tags+=['A23.'+c for c in sorted(set(changes))]
 history=[]
 for name,report in month_reports.items():
  result=next((x for x in report['results'] if x['index']==i),None);history.append({'run_id':name,'report_result_index':i if result else None,'status':result.get('status') if result else 'NOT_REACHED','report':asset(pathlib.Path('reports/live-model')/name/'report.json'),'oracle_version':digest(month/f'oracle/{i:03}.json'),'calls':month_calls[name].get(f'month.item.{i:03}',[])})
 cases.append({'case_id':f'month.item.{i:03}','family':'month','day':inp['day'],'point':inp['point'],'coverage_clauses':tags,'authored_input':asset(month/f'input/{i:03}.json'),'original_oracle':asset(month/f'oracle/{i:03}.json'),'precall_manifest':asset(month/'manifest.json'),'manifest_membership_verified':entries[f'input/{i:03}.json']==digest(month/f'input/{i:03}.json') and entries[f'oracle/{i:03}.json']==digest(month/f'oracle/{i:03}.json'),'histories':history})
for day in range(30):
 i=day*3+2;history=[]
 for name,report in month_reports.items():
  result=next((x for x in report['results'] if x['index']==i),None);history.append({'run_id':name,'report_result_index':i if result else None,'status':result.get('consciousness_status','NOT_RECORDED') if result else 'NOT_REACHED','error_code':result.get('consciousness_error') if result else None,'calls':month_calls[name].get(f'month.consciousness.{day:02}',[]),'reference_audit':asset(pathlib.Path('reports/live-model')/name/'consciousness-reference-audit.json')})
 cases.append({'case_id':f'month.consciousness.{day:02}','family':'month','day':day,'coverage_clauses':['A23.daily_generated_consciousness','A23.exact_EntityReadRef_at_snapshot','A23.sampled_free_text_only'],'original_oracle':asset(month/f'oracle/{i:03}.json'),'oracle_scope':'Daily authoritative-state anchor; EntityReadRef validity is audited against immutable events. No fabricated authored free-text oracle; samples are qualitative review only.','precall_manifest':asset(month/'manifest.json'),'manifest_membership_verified':f'oracle/{i:03}.json' in entries,'histories':history})
# Enumerate every stored supplemental version, including aborted and intermediate local-only runs.
archives=[('scenarios-run1','live-scenarios-run1',None),('scenarios-run2','live-scenarios-run2','scenarios-run2-failed'),('scenarios-run3','live-scenarios-run3',None),('scenarios-run4','live-scenarios-run4','scenarios-run4'),('retrieval-regression-run1','live-retrieval-regression-run1',None),('world-read-run1','live-world-read-run1',None),('world-read-run2','live-world-read-run2','world-read-run2')]
bycase=collections.defaultdict(list)
clauses={'exact-id-conflict':['A23.exact_persistent_ID','A23.optimistic_revision_conflict'],'dependency-completion':['A23.completion','A23.dependency_preservation'],'dependency-unlock':['A23.dependency_unlock'],'remember-agreement':['A23.conversation_commitment'],'withdraw-agreement':['A23.conversation_withdrawal'],'prior-session-retrieval':['A23.prior_session_corrected_agreement'],'stale-source':['A23.source_staleness'],'world-conflict':['A23.exact_conflict_membership','A23.unresolved_conflict'],'world-retraction':['A23.fact_retraction_read_semantics','A23.remaining_conflict_state']}
for runid,localname,publicname in archives:
 local=pathlib.Path('reports/local')/localname;base=pathlib.Path('reports/live-model')/publicname if publicname else local
 if not (R/base/'fixture.json').exists():issues.append('missing archive '+runid);continue
 fixture=load(base/'fixture.json');report=load(base/'report.json') if (R/base/'report.json').exists() else None;results={x['name']:x for x in report['results']} if report else {};run={'run_id':runid,'family':'world' if runid.startswith('world') else 'scenarios','fixture':asset(base/'fixture.json'),'report':asset(base/'report.json'),'reported_failures':report.get('failures') if report else None,'status':'ABORTED_NO_REPORT' if report is None else ('PASS' if report['failures']==0 else 'FAILED'),'case_count':len(fixture['steps']),'visibility':'PUBLIC' if publicname else 'LOCAL_ONLY_NOT_PUBLISHED','precall_fixture_basis':'livescenarios saves full parsed fixture before iteration and saves oracle+input before Process/Generate; no later output-to-oracle conversion','local_original_archive_id':localname};runs.append(run)
 for i,step in enumerate(fixture['steps']):
  name=step['name'];dirs=list((R/base).glob(f'*-{name}'));casebase=dirs[0].relative_to(R) if dirs else None;oracle=casebase/'oracle.json' if casebase else None;result=results.get(name);eq=bool(oracle and (R/oracle).exists() and load(oracle)==step);callhist=[]
  if casebase:
   for call in sorted((R/casebase).glob('call-*.result.json')):
    rel=call.relative_to(R);v=load(rel);callhist.append({'result':asset(rel),'error':v.get('error'),'output_present':bool(v.get('output')),'validation_issues':v.get('validation_issues'),'context':asset(rel.with_name(call.name.replace('.result.json','.context.json')))})
  obs={'run_id':runid,'status':result.get('status') if result else 'NOT_REACHED_OR_NO_REPORT','reported_error':result.get('error') if result else None,'reported_mismatches':result.get('mismatches') if result else None,'calls_reported':result.get('calls') if result else None,'fixture':asset(base/'fixture.json'),'fixture_step_pointer':f'/steps/{i}','fixture_step_canonical_sha256':canon(step),'original_oracle':asset(oracle) if oracle else None,'oracle_canonical_sha256':canon(load(oracle)) if oracle and (R/oracle).exists() else None,'oracle_matches_precall_fixture_step':eq if oracle else None,'input':asset(casebase/'input.json') if casebase else None,'report':asset(base/'report.json'),'calls':callhist,'visibility':'PUBLIC' if publicname else 'LOCAL_ONLY_NOT_PUBLISHED'}
  if result and not eq:issues.append('reported case has no matching frozen oracle '+runid+'/'+name)
  bycase[name].append(obs)
for name,history in bycase.items():
 versions={h['fixture_step_canonical_sha256'] for h in history};cases.append({'case_id':'supplement.'+name,'family':'world' if name.startswith('world-') else 'scenarios','coverage_clauses':clauses.get(name,['A23.supplemental']), 'oracle_versions':sorted(versions),'oracle_version_count':len(versions),'precall_manifest_kind':'PER_RUN_PRECALL_FIXTURE_NOT_GLOBAL_INVENTORY','histories':history})
# Current complete sets are selected as whole runs, never as isolated passing cases.
selected={'month':'month-run8','scenarios':'scenarios-run4','world':'world-read-run2'}
for c in cases:
 runid=selected[c['family']];h=next((h for h in c['histories'] if h['run_id']==runid),None);c['current_complete_run']=runid;c['current_status']=h['status'] if h else 'MISSING';c['current_precall_membership_verified']=c.get('manifest_membership_verified',False) if c['family']=='month' else bool(h and h.get('oracle_matches_precall_fixture_step'))
 if not h or c['current_status']!='PASS':issues.append('current case not PASS '+c['case_id'])
 if not c['current_precall_membership_verified']:issues.append('current membership missing '+c['case_id'])
# This audit is created now. It cannot cryptographically backdate a composition.
ledger={'schema_version':2,'index_version':2,'prior_reviewed_index':asset('reports/implementation/A23-coverage-ledger.json'),'prior_review':asset('review/A23-ledger-deepseek-resume1.response.md'),'created_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'purpose':'Read-only retrospective cross-reference of pre-existing frozen component sets; not a new oracle or a claim this composition file existed before calls','decision_basis':asset('review/foundation-world-diagnostics-deepseek-final.response.md'),'composition_policy':{'whole_runs':selected,'no_checkpoint_cherry_picking':True,'all_discovered_prior_versions_and_failures_retained':True,'same_database_30_day_requirement':False,'world_mutations':'Frozen authorized setup; live model evaluates reading conflict/retraction, not generation of mutation authorization','free_text':'Historical qualitative samples remain scoped to their source runs. Current month8 coverage here is exact authoritative state and snapshot references, not complete prose correctness.'},'precall_evidence_limits':['Month manifest binds all 180 authored input/oracle files; file hash checks verify current equality, not trusted timestamp attestation.','Supplement global manifest explicitly identifies itself as current inventory, not retroactive freeze proof. Per-run fixture/oracle files and tool save-before-call ordering supply the archived pre-call membership evidence.','This unified index is created after executions under the reviewer-authorized composition interpretation. It does not claim that a cryptographically timestamped combined-set manifest existed before the first call.','Intermediate local-only runs are indexed by project-relative archive paths; no private absolute paths, runtime configs, tokens or grants are copied.'], 'authored_manifests':[asset(month/'manifest.json'),asset('reports/fixtures/live-scenarios/manifest.json')],'harness_evidence':[{'file':asset('src/tools/livescenarios/main.go'),'rule':'Full fixture save occurs before loop; oracle/input save occurs before model invocation'},{'file':asset('src/tools/livereplay/main.go'),'rule':'Reads authored input/oracle files; never writes oracle files'}],'run_inventory':runs,'cases':cases,'verification':{'case_count':len(cases),'current_pass_count':sum(c['current_status']=='PASS' for c in cases),'current_precall_membership_verified_count':sum(c['current_precall_membership_verified'] for c in cases),'month_manifest_entries_verified':len(entries),'issues':issues,'status':'PASS_INDEX_INTEGRITY' if not issues else 'GAPS_FOUND','not_acceptance_verdict':True}}
# Append D11/A09 as a separate history, never inflate the 129 A23 cases.
question_history=[]
for n in range(1,5):
 name=f'question-run{n}'+('-failed' if n<4 else '')
 base=pathlib.Path('reports/live-model')/name
 report=load(base/'report.json')
 question_history.append({'run_id':name,'status':report['status'],'error':report.get('error'),'report':asset(base/'report.json'),'fixture':asset(base/'fixture.json'),'manifest':asset(base/'manifest.json'),'publication_manifest':asset(base/'publication-manifest.json'),'wire_diagnosis':asset(base/'wire-retrieval-diagnosis.json'),'scope':'A09/D11 additional lifecycle evidence, excluded from A23 case_count'})
ledger['additional_A09_D11_history']=question_history
# Every available raw asset is indexed with its actual current byte hash.
checked=0
input_matches=0
def verify(value):
 global checked,input_matches
 if isinstance(value,dict):
  if 'path' in value and 'sha256' in value:
   p=pathlib.Path(value['path'])
   if p.is_absolute() or '..' in p.parts or not (R/p).is_file() or digest(p)!=value['sha256']:issues.append('asset hash mismatch '+str(p))
   checked+=1
  if 'authored_input_text_matches' in value:
   if not value['authored_input_text_matches']:issues.append('archived model input differs from authored fixture')
   else:input_matches+=1
  for x in value.values():verify(x)
 elif isinstance(value,list):
  for x in value:verify(x)
for run in runs:
 if run['family']=='month':
  base=pathlib.Path('reports/live-model')/run['run_id']
  if (R/base/'audit-records.json').exists():
   snapshots=load(base/'audit-records.json')['consciousness_snapshots']
   run['persisted_snapshot_count']=len(snapshots)
   run['persisted_slots']=[x['slot'] for x in snapshots]
   run['snapshot_authority']=asset(base/'audit-records.json')
for c in cases:
 if c['case_id'].startswith('month.consciousness.'):
  day=c['day']
  for h in c['histories']:
   base=pathlib.Path('reports/live-model')/h['run_id']
   audit=base/'consciousness-reference-audit.json'
   if (R/audit).exists():
    matched=[r for r in load(audit)['results'] if r['slot']==day]
    h['independent_snapshot_reference_status']=matched[0]['status'] if len(matched)==1 else 'MISSING'
    h['snapshot_id']=matched[0]['snapshot_id'] if len(matched)==1 else None
    if h['run_id']=='month-run8' and (len(matched)!=1 or matched[0]['status']!='PASS'):issues.append('current snapshot reference audit missing or failed')
   elif (R/base/'audit-records.json').exists():
    h['persisted_snapshot_present']=any(x['slot']==day for x in load(base/'audit-records.json')['consciousness_snapshots'])
    h['independent_snapshot_reference_status']='NOT_AUDITED'
verify(ledger)
ledger['verification'].update({'asset_hashes_verified':checked,'archived_input_matches_verified':input_matches,'issues':issues,'status':'PASS_INDEX_INTEGRITY' if not issues else 'GAPS_FOUND'})
out=R/'reports/implementation/A23-coverage-ledger-v2.json'
out.write_text(json.dumps(ledger,ensure_ascii=False,indent=2)+'\n')
lines=['# A23 coverage ledger v2','', 'This is a retrospective index update. It does not claim a combined manifest existed before any model call. The reviewed v1 is preserved unchanged.','',
 'Current whole sets: **month-run8** (90 Item points and 30 snapshot reference checks), **scenarios-run4** (7 cases), **world-read-run2** (2 cases). No passing checkpoint is selected from another month run.','',
 f"Integrity: {len(cases)} cases; {ledger['verification']['current_pass_count']} current PASS; {ledger['verification']['current_precall_membership_verified_count']} pre-call component memberships; {checked} asset references hash-verified.",'',
 '## Full month history','', '| Run | Item points recorded | Item PASS | Reported failures | Persisted snapshots |','|---|---:|---:|---:|---:|']
for r in runs:
 if r['family']=='month':lines.append(f"| {r['run_id']} | {r['results_recorded']} | {r['item_passes']} | {r['reported_failures']} | {r.get('persisted_snapshot_count','not exported')} |")
lines += ['', 'Month-run7-failed retains all six failures and all three persisted snapshots. Persistence alone is not claimed as an independent reference/prose audit for that failed run. Earlier local-only scenario/world histories remain explicitly local-only in the JSON.','',
 '## Separate A09/D11 history','', '| Run | Status | Scope |','|---|---|---|']
for h in question_history:lines.append(f"| {h['run_id']} | {h['status']} | {h.get('error') or 'Full lifecycle including restart, explicit answer and idempotency/409'} |")
lines += ['', 'These four question runs are additional A09/D11 evidence, excluded from the 129 A23 cases.','',
 '## Evidence limits',''] + ['- '+x for x in ledger['precall_evidence_limits']]
lines += ['', 'Current snapshot assertions check immutable Item revisions and exact EntityReadRef. They do not supply an invented free-text oracle. Historical prose samples apply only to their named original runs.','',
 f"Machine-readable ledger: [A23-coverage-ledger-v2.json](A23-coverage-ledger-v2.json), SHA-256 `{hashlib.sha256(out.read_bytes()).hexdigest()}`.",'',
 'Generator: `python3 scripts/build_a23_ledger.py --root REPOSITORY_ROOT`. This indexes existing evidence only; it runs no model and changes no oracle.','']
(R/'reports/implementation/A23-coverage-ledger-v2.md').write_text('\n'.join(lines))
print(json.dumps(ledger['verification'],ensure_ascii=False));print('bytes',out.stat().st_size)
