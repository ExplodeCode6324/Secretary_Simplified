import pathlib,subprocess,tempfile,json,time,hashlib,sqlite3,uuid,datetime
repo=pathlib.Path.cwd();binary=repo/'release';out=repo/'reports/local/d12-cli-class-smoke';out.mkdir(parents=True,exist_ok=False)
report={'suite':'D12-real-CLI-fixture-classification','status':'RUNNING','live_api_calls':False,'build_sha256':{n:hashlib.sha256((binary/n).read_bytes()).hexdigest() for n in ['secretary','secretaryd']},'cases':[]};procs=[];handles=[];dirs=[]
def check(ok,msg):
 if not ok:raise AssertionError(msg)
def run_case(name,words,expected,blocked=False):
 data=pathlib.Path(tempfile.mkdtemp(prefix='ss-d12cli-',dir='/tmp'));dirs.append(str(data));logs=[]
 subprocess.run([str(binary/'secretary'),'init','--data-dir',str(data),'--json'],capture_output=True,check=True)
 config=json.loads((data/'config.json').read_text());check(config['model_profile']['profile']=='fixture','fixture profile mandatory');config['provider_policy']={'allowed_data_classes':['SYNTHETIC'],'source_ids':[]};(data/'config.json').write_text(json.dumps(config))
 handle=open(data/'core.log','w');handles.append(handle);proc=subprocess.Popen([str(binary/'secretaryd'),'core','--config',str(data/'config.json')],stdout=handle,stderr=handle);procs.append(proc)
 def cli(args):
  r=subprocess.run([str(binary/'secretary'),*args,'--config',str(data/'config.json'),'--json'],capture_output=True,text=True,timeout=15)
  try:v=json.loads(r.stdout)
  except:raise AssertionError('unparsed CLI stdout')
  logs.append({'args':args,'returncode':r.returncode,'response':v,'stderr':r.stderr});check(r.returncode==0,'CLI failed');return v['result']
 deadline=time.monotonic()+15
 while not (data/'run/core.sock').exists():
  check(proc.poll() is None,'Core exited');check(time.monotonic()<deadline,'Core startup timeout');time.sleep(.1)
 result=cli(words)
 if words[0]=='input':
  deadline=time.monotonic()+30
  while True:
   turn=cli(['turns',result['turn_id']])
   if turn['state'] in ['FAILED','COMMITTED']:break
   check(time.monotonic()<deadline,'turn timeout');time.sleep(.15)
  check(turn['input']['data_class']==expected,'input durable class mismatch')
  records=[json.loads(f.read_text()) for f in (data/'reports/model_calls').glob('*.json') if not f.name.endswith('.request.json')]
  if blocked:
   check(len(records)==0,'blocked input entered recording provider')
   check('no actions were committed' in str(turn['reply']['text']),'missing fixed blocked reply')
  else:
   check(len(records)>=1,'synthetic input never reached fixture model');check(all(v['provider_profile']=='fixture' for v in records),'nonfixture call');check('no actions were committed' not in str(turn['reply']['text']),'synthetic turn rejected')
  state={'turn':turn,'model_call_count':len(records),'model_calls':records}
 else:
  with sqlite3.connect(data/'state/secretary.sqlite') as db:
   items=[json.loads(r[0]) for r in db.execute('SELECT payload_json FROM item')];turns=[json.loads(r[0]) for r in db.execute('SELECT payload_json FROM input_turn')]
  check(len(items)==1 and items[0]['extensions']['security.classification']['data_class']==expected,'item durable class mismatch');check(turns[0]['input']['data_class']==expected,'typed input class mismatch');state={'items':items,'turns':turns}
 proc.terminate();proc.wait(timeout=10)
 case={'name':name,'status':'PASS','expected_class':expected,'provider_blocked':blocked,'state':state,'cli':logs};report['cases'].append(case);(out/(name+'.json')).write_text(json.dumps(case,ensure_ascii=False,indent=2))
try:
 run_case('default-item-personal',['items','create','--title','synthetic-smoke-default','--domain','project'],'PERSONAL')
 run_case('explicit-item-synthetic',['items','create','--title','synthetic-smoke-explicit','--domain','project','--data-class','SYNTHETIC'],'SYNTHETIC')
 run_case('default-input-blocked',['input','--session',str(uuid.uuid4()),'--text','Synthetic fixture ordinary input'],'PERSONAL',True)
 run_case('explicit-input-synthetic',['input','--session',str(uuid.uuid4()),'--text','Synthetic fixture normal reply','--data-class','SYNTHETIC'],'SYNTHETIC')
 run_case('secret-input-blocked',['input','--session',str(uuid.uuid4()),'--text','Synthetic fixture SECRET classification canary','--data-class','SECRET'],'SECRET',True)
 report['status']='PASS'
except Exception as e:report['status']='FAIL';report['error']=str(e)
finally:
 for proc in procs:
  if proc.poll() is None:proc.terminate()
 for proc in procs:
  try:proc.wait(timeout=10)
  except subprocess.TimeoutExpired:proc.kill();proc.wait()
 for h in handles:h.close()
 report['owned_processes_stopped']=all(p.poll() is not None for p in procs);report['completed_at']=datetime.datetime.now(datetime.timezone.utc).isoformat();(out/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2));(out/'private-data-paths.json').write_text(json.dumps(dirs));print(json.dumps({'status':report['status'],'error':report.get('error'),'cases':len(report['cases']),'cleanup':report['owned_processes_stopped']}))
