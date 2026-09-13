#!/usr/bin/env python3
"""Bounded two-hour real-process test, exclusively using owned synthetic state."""
import argparse,json,subprocess,time,pathlib,hashlib,sqlite3,uuid
p=argparse.ArgumentParser();p.add_argument('--metadata',required=True);p.add_argument('--cli',required=True);p.add_argument('--report',required=True);p.add_argument('--seconds',type=int,default=7200);a=p.parse_args()
m=json.loads(pathlib.Path(a.metadata).read_text());d=pathlib.Path(m['data_dir']);out=pathlib.Path(a.report);out.mkdir(parents=True,exist_ok=True);records=[];failures=0

def doctor():
 r=subprocess.run([a.cli,'doctor','--json','--config',str(d/'config.json')],capture_output=True,text=True,timeout=15)
 try:v=json.loads(r.stdout)
 except Exception:v={}
 return r.returncode,v

def ready(v):
 return all(v.get('heartbeats',{}).get(role,{}).get('state')=='RECENT' for role in ['core','runner'])

deadline=time.time()+15
while time.time()<deadline:
 rc,v=doctor()
 if rc==0 and ready(v):break
 time.sleep(.2)
else:raise SystemExit('FAIL: daemons did not become ready within 15 seconds')
start=time.time()
with sqlite3.connect(d/'state/secretary.sqlite') as db:baseline=db.execute('SELECT count(*) FROM item').fetchone()[0]
while time.time()-start<a.seconds:
 request=str(uuid.uuid4());title='synthetic-soak-%04d'%len(records)
 command=[a.cli,'items','create','--title',title,'--domain','project','--request-id',request,'--config',str(d/'config.json'),'--json']
 # Same request crosses two independent CLI processes; only one durable effect.
 writes=[subprocess.run(command,capture_output=True,text=True,timeout=15).returncode for _ in range(2)]
 rc,v=doctor()
 with sqlite3.connect(d/'state/secretary.sqlite') as db:
  count=db.execute('SELECT count(*) FROM item').fetchone()[0]
  turns=db.execute("SELECT count(*) FROM input_turn WHERE request_id=? AND state='COMMITTED'",(request,)).fetchone()[0]
 ok=all(x==0 for x in writes) and count==baseline+len(records)+1 and turns==1 and rc==0 and ready(v) and v.get('database',{}).get('integrity')=='ok' and v.get('database',{}).get('foreign_key_violations')==0 and all(n<=100 for n in v.get('database',{}).get('queues',{}).values() if isinstance(n,int))
 if not ok:failures+=1
 records.append({'elapsed_seconds':round(time.time()-start,2),'pass':ok,'item_count':count,'expected_item_count':baseline+len(records)+1,'committed_request_count':turns,'cli_exit_codes':writes,'doctor':v})
 data={'suite':'A25-real-process-soak','status':'RUNNING','build_sha256':m['build_sha256'],'sampler_sha256':hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest(),'clock_mode':'REAL_WALL_CLOCK','profile':'fixture','real_data':False,'elapsed_seconds':time.time()-start,'required_seconds':a.seconds,'failures':failures,'samples':records}
 (out/'report.json').write_text(json.dumps(data,indent=2));time.sleep(min(30,max(0,a.seconds-(time.time()-start))))
data['status']='PASS' if failures==0 else 'FAIL';data['elapsed_seconds']=time.time()-start;(out/'report.json').write_text(json.dumps(data,indent=2));print(data['status'],data['elapsed_seconds'],failures)
