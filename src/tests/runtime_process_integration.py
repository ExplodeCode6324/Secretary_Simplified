#!/usr/bin/env python3
"""Isolated fixture process test; only processes started here are terminated."""
import copy, datetime, http.client, json, os, pathlib, socket, sqlite3, subprocess, tempfile, time, uuid
root = pathlib.Path(__file__).resolve().parents[2]
work = pathlib.Path(tempfile.mkdtemp(prefix='ss-m3-', dir='/tmp'))
class UnixHTTP(http.client.HTTPConnection):
    def __init__(self, path): super().__init__('localhost', timeout=5); self.path=str(path)
    def connect(self): self.sock=socket.socket(socket.AF_UNIX,socket.SOCK_STREAM); self.sock.settimeout(5); self.sock.connect(self.path)
def call(path,token,method,url,body=None):
    c=UnixHTTP(path); c.request(method,url,None if body is None else json.dumps(body),{'Authorization':'Bearer '+token,'Content-Type':'application/json'})
    r=c.getresponse(); result=(r.status,json.loads(r.read()));c.close();return result
processes=[]; logs=[]; result={'profile':'fixture','real_data':False,'real_model':False}
try:
    for binary in ['secretary','secretaryd']:
        subprocess.run(['go','build','-o',str(work/binary),'./cmd/'+binary],cwd=root/'src',check=True)
    subprocess.run([str(work/'secretary'),'init','--data-dir',str(work/'data')],check=True,stdout=subprocess.DEVNULL)
    data=work/'data'; run=data/'run'; client=(run/'client.token').read_text().strip(); internal=(run/'internal.token').read_text().strip()
    for role in ['core','runner']:
        log=open(work/(role+'.log'),'w');logs.append(log)
        processes.append(subprocess.Popen([str(work/'secretaryd'),role,'--config',str(data/'config.json')],stdout=log,stderr=log))
    deadline=time.monotonic()+15
    while time.monotonic()<deadline:
        try:
            if call(run/'core.sock',client,'GET','/v1/health')[0]==200 and call(run/'runner.sock',client,'GET','/v1/health')[0]==200:break
        except (OSError,http.client.HTTPException):pass
        time.sleep(.1)
    else:raise AssertionError('daemons unavailable')
    assert call(run/'core.sock',internal,'GET','/v1/health')[0]==401
    assert call(run/'core-internal.sock',client,'POST','/internal/v1/work',{})[0]==401
    result['client_internal_token_separation']=True
    example=json.loads((root/'docs/examples/decision-create-reminder.json').read_text())
    actions=copy.deepcopy(example['actions']);now=datetime.datetime.now(datetime.timezone.utc)
    stamp=lambda seconds:(now+datetime.timedelta(seconds=seconds)).isoformat(timespec='milliseconds').replace('+00:00','Z')
    actions[0]['payload']['due_at']=stamp(4)
    actions[1]['payload']['schedule']['at']=stamp(4)
    second=copy.deepcopy(actions[1]);second['operation_key']='create_reminder_second';second['payload']['schedule']['at']=stamp(10)
    second['payload']['command']['arguments']['notification_key']='process-fixture:second'
    second['payload']['task_template']['criteria'][0]['expected']['notification_key']='process-fixture:second'
    actions.append(second)
    status,body=call(run/'core.sock',client,'POST','/v1/actions',{'schema_version':1,'request_id':str(uuid.uuid4()),'session_id':str(uuid.uuid4()),'actions':actions})
    assert status==200,(status,body)
    deadline=time.monotonic()+9
    while time.monotonic()<deadline:
        status,body=call(run/'runner.sock',client,'GET','/v1/notifications')
        if len(body['result']['items'])>=1:break
        time.sleep(.1)
    else:raise AssertionError('first reminder missing')
    processes[0].terminate();processes[0].wait(timeout=10)
    deadline=time.monotonic()+12
    while time.monotonic()<deadline:
        status,body=call(run/'runner.sock',client,'GET','/v1/notifications')
        if len(body['result']['items'])==2:break
        time.sleep(.1)
    else:raise AssertionError('offline second reminder missing')
    assert call(run/'runner.sock',client,'GET','/v1/health')[0]==200
    db=sqlite3.connect(data/'state/secretary.sqlite')
    states=[r[0] for r in db.execute('SELECT status FROM item')];assert states==['OPEN'],states
    assert db.execute("SELECT count(*) FROM job_run WHERE state='SUCCEEDED' AND json_extract(payload_json,'$.command.capability')='notify.local'").fetchone()[0]==2
    db.close()
    result.update({'typed_action_commit':True,'two_notification_runs_succeeded':True,'core_offline_second_notification':True,'runner_control_online':True,'business_item_still_open':True,'notification_count':2})
finally:
    for p in processes:
        if p.poll() is None:p.terminate()
    for p in processes:
        try:p.wait(timeout=10)
        except subprocess.TimeoutExpired:p.kill();p.wait(timeout=5)
    for f in logs:f.close()
result['completed_at']=datetime.datetime.now(datetime.timezone.utc).isoformat()
(root/'review/M3-runtime-process-integration.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps(result,indent=2))
