#!/usr/bin/env python3
"""Real CLI/model/Core/Runner reminder chain using only synthetic content."""
import argparse,datetime,hashlib,json,pathlib,sqlite3,subprocess,tempfile,time,uuid
p=argparse.ArgumentParser();p.add_argument('--config',required=True);p.add_argument('--report',required=True);p.add_argument('--binaries',default='release');a=p.parse_args()
root=pathlib.Path.cwd();binary=(root/a.binaries).resolve();out=pathlib.Path(a.report).resolve();out.mkdir(parents=True,exist_ok=False)
data=pathlib.Path(tempfile.mkdtemp(prefix='ss-livecli-',dir='/tmp'));ps=[];logs=[]
report={'suite':'LIVE_MODEL-real-CLI-reminder','status':'RUNNING','real_data':False,'build_sha256':{n:hashlib.sha256((binary/n).read_bytes()).hexdigest() for n in ['secretary','secretaryd']}}

def cli(*args):
 r=subprocess.run([str(binary/'secretary'),*args,'--data-class','SYNTHETIC','--config',str(data/'config.json'),'--json'],capture_output=True,text=True,timeout=20)
 if r.returncode:raise AssertionError('CLI exit '+str(r.returncode)+': '+r.stderr[:300])
 return json.loads(r.stdout)

def check_quota():
 for f in (data/'reports'/'model_calls').glob('*.json'):
  try:v=json.loads(f.read_text())
  except Exception:continue
  if v.get('error_code')=='MODEL_HTTP_429':raise RuntimeError('MODEL_HTTP_429')

try:
 subprocess.run([str(binary/'secretary'),'init','--data-dir',str(data),'--json'],stdout=subprocess.DEVNULL,check=True)
 c=json.loads((data/'config.json').read_text());template=json.loads(pathlib.Path(a.config).read_text());c['model_profile']=template['model_profile']
 if c['model_profile']['profile'] not in ['opencode-go','deepseek']:raise ValueError('real profile required')
 c['provider_policy']={'allowed_data_classes':['SYNTHETIC'],'source_ids':[]};(data/'config.json').write_text(json.dumps(c));report['provider_profile']=c['model_profile']['profile'];report['model']=c['model_profile']['model']
 for role in ['core','runner']:
  f=open(data/(role+'.log'),'w');logs.append(f);ps.append(subprocess.Popen([str(binary/'secretaryd'),role,'--config',str(data/'config.json')],stdout=f,stderr=f))
 deadline=time.monotonic()+15
 while time.monotonic()<deadline:
  if (data/'run/core.sock').exists() and (data/'run/runner.sock').exists():break
  time.sleep(.1)
 else:raise AssertionError('startup timeout')
 now=datetime.datetime.now(datetime.timezone.utc);due=now+datetime.timedelta(seconds=90);stamp=due.isoformat(timespec='milliseconds').replace('+00:00','Z');request=str(uuid.uuid4());title='synthetic-cli-reminder'
 text='创建一个 project 领域 TASK 事项，标题严格为 synthetic-cli-reminder，截止时间为 '+stamp+' (UTC)。同时在该时间发一次本地文字通知，通知文字为 synthetic-cli-reminder。只发通知，事项保持 OPEN，不要声称已完成事项。'
 (out/'input-oracle.json').write_text(json.dumps({'request_id':request,'text':text,'expected':{'title':title,'domain':'project','status':'OPEN','due_at':stamp,'notification_count':1}},ensure_ascii=False,indent=2))
 accepted=cli('input','--request-id',request,'--text',text);report['accepted']=accepted;turn_id=accepted['result']['turn_id'];deadline=time.monotonic()+150
 while time.monotonic()<deadline:
  check_quota();turn=cli('turns',turn_id)['result']
  if '429' in str(turn.get('reply')):raise RuntimeError('MODEL_HTTP_429')
  if turn['state'] in ['COMMITTED','FAILED']:break
  time.sleep(.5)
 else:raise AssertionError('turn timeout')
 report['turn']=turn
 if turn['state']!='COMMITTED' or len(turn['committed_operation_keys'])!=2:raise AssertionError('expected one Item and one scheduled reminder')
 repeated=cli('input','--request-id',request,'--text',text)
 if repeated['result']['turn_id']!=turn_id:raise AssertionError('input retry duplicated turn')
 deadline=time.monotonic()+120
 while time.monotonic()<deadline:
  check_quota();page=cli('notifications')['result'];notes=page['items']
  if notes:break
  time.sleep(.5)
 else:raise AssertionError('scheduled notification timeout')
 items=cli('items','list')['result']['items'];report['items']=items;report['notifications']=notes
 if len(items)!=1 or any(items[0][k]!=v for k,v in {'title':title,'domain':'project','status':'OPEN','due_at':stamp}.items()):raise AssertionError('strict Item oracle mismatch')
 if len(notes)!=1 or notes[0]['text']!=title:raise AssertionError('notification oracle mismatch')
 with sqlite3.connect(data/'state/secretary.sqlite') as db:
  counts={table:db.execute('SELECT count(*) FROM '+table).fetchone()[0] for table in ['input_turn','item','notification']}
  report['counts']=counts
  if counts!={'input_turn':1,'item':1,'notification':1}:raise AssertionError('duplicate durable effect')
 ps[0].terminate();ps[0].wait(timeout=10)
 cli('notifications','ack',notes[0]['id']);report['ack_with_core_offline']=True
 report['status']='PASS'
except Exception as e:
 report['status']='FAIL';report['error']=str(e)
finally:
 try:
  with sqlite3.connect(data/'state/secretary.sqlite') as db:
   state={table:[json.loads(row[0]) for row in db.execute('SELECT payload_json FROM '+table)] for table in ['scheduled_job','job_run','task','executor_receipt']}
   (out/'runtime-state.json').write_text(json.dumps(state,ensure_ascii=False,indent=2))
 except Exception as evidence_error:
  report['evidence_error']=str(evidence_error)
 for proc in ps:
  if proc.poll() is None:proc.terminate()
 for proc in ps:
  try:proc.wait(timeout=10)
  except subprocess.TimeoutExpired:proc.kill();proc.wait(timeout=5)
 for f in logs:f.close()
 report['completed_at']=datetime.datetime.now(datetime.timezone.utc).isoformat();(out/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2));(out/'private-data-path.txt').write_text(str(data))
 print(json.dumps({'status':report['status'],'error':report.get('error'),'report':str(out)},ensure_ascii=False))
raise SystemExit(0 if report['status']=='PASS' else 1)
