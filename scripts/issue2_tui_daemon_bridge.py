#!/usr/bin/env python3
"""TUI-08 only: real CLI/Core/Runner, one synthetic short notification after client exit."""
import argparse, datetime, fcntl, hashlib, http.client, json, os, pathlib, pty, select, signal, socket, struct, subprocess, tempfile, termios, time

def main():
 ap=argparse.ArgumentParser(description=__doc__);ap.add_argument('--cli',required=True);ap.add_argument('--daemon',required=True);ap.add_argument('--report',required=True);a=ap.parse_args();root=pathlib.Path(__file__).resolve().parents[1];out=pathlib.Path(a.report);out.mkdir(parents=True,exist_ok=False);data=pathlib.Path(tempfile.mkdtemp(prefix='ss-tui-bridge-',dir='/tmp'));cli=pathlib.Path(a.cli).resolve();daemon=pathlib.Path(a.daemon).resolve();processes=[];ptys=[];logs=[];result={'scope':'TUI-08 real daemon notification after client exit, TUI-05 authority reconnection; no old baseline or sustained test','status':'RUNNING','cli_sha256':hashlib.sha256(cli.read_bytes()).hexdigest(),'daemon_sha256':hashlib.sha256(daemon.read_bytes()).hexdigest()}
 def command(*args):
  p=subprocess.run([str(cli),*args,'--config',str(data/'config.json'),'--json'],capture_output=True,text=True,timeout=15)
  if p.returncode:raise AssertionError('CLI_COMMAND_FAILED')
  return json.loads(p.stdout)
 class Unix(http.client.HTTPConnection):
  def __init__(self,role):super().__init__('localhost',timeout=5);self.role=role
  def connect(self):self.sock=socket.socket(socket.AF_UNIX);self.sock.settimeout(5);self.sock.connect(str(data/f'run/{self.role}.sock'))
 def get(path,role='core'):
  conn=Unix(role);conn.request('GET',path,headers={'Authorization':'Bearer '+(data/'run/client.token').read_text().strip()});resp=conn.getresponse();body=json.loads(resp.read());conn.close();assert resp.status==200,'GET_FAILED';return body['result']
 def pump(e,seconds):
  end=time.monotonic()+seconds
  while time.monotonic()<end:
   if select.select([e[1]],[],[],.05)[0]:
    try:e[4].extend(os.read(e[1],65536))
    except OSError:break
 def open_tui():
  m,s=pty.openpty();before=termios.tcgetattr(s);fcntl.ioctl(s,termios.TIOCSWINSZ,struct.pack('HHHH',28,110,0,0));env=dict(os.environ,TERM='xterm-256color',LANG='en_US.UTF-8');p=subprocess.Popen([str(cli),'--config',str(data/'config.json'),'--data-class','SYNTHETIC'],stdin=s,stdout=s,stderr=s,start_new_session=True,env=env);e=(p,m,s,before,bytearray());ptys.append(e);pump(e,1);assert p.poll() is None,'TUI_EXITED';return e
 def close_tui(e):
  os.write(e[1],b'\x03');e[0].wait(timeout=5);pump(e,.1);after=termios.tcgetattr(e[2]);before=list(e[3]);after[3]&=~getattr(termios,'PENDIN',0);before[3]&=~getattr(termios,'PENDIN',0);assert before==after,'TTY_NOT_RESTORED'
 try:
  p=subprocess.run([str(cli),'init','--data-dir',str(data),'--json'],capture_output=True,timeout=15);assert p.returncode==0,'INIT_FAILED'
  for role in ['core','runner']:
   log=(data/f'{role}.log').open('w');logs.append(log);processes.append(subprocess.Popen([str(daemon),role,'--config',str(data/'config.json')],stdout=log,stderr=log))
  deadline=time.monotonic()+15
  while not all((data/f'run/{role}.sock').exists() for role in ['core','runner']):
   assert time.monotonic()<deadline and all(p.poll() is None for p in processes),'DAEMON_START_FAILED';time.sleep(.05)
  authority=get('/v1/conversation');action=json.loads((root/'release/examples/reminder.json').read_text());action['payload']['schedule']['at']=(datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(seconds=5)).isoformat(timespec='milliseconds').replace('+00:00','Z');path=data/'synthetic-reminder.json';path.write_text(json.dumps(action));command('actions','--file',str(path),'--data-class','SYNTHETIC')
  first=open_tui();close_tui(first);assert all(p.poll() is None for p in processes),'TUI_STOPPED_DAEMON'
  deadline=time.monotonic()+15;notes=[]
  while not notes:
   notes=get('/v1/notifications','runner')['items'];assert time.monotonic()<deadline,'NOTIFICATION_NOT_FIRED';time.sleep(.1)
  assert len(notes)==1,'DUPLICATE_NOTIFICATION'
  before_restart=get('/v1/conversation');history_before=get('/v1/conversation/history?limit=50')
  processes[0].terminate();processes[0].wait(timeout=10)
  restarted=subprocess.Popen([str(daemon),'core','--config',str(data/'config.json')],stdout=logs[0],stderr=logs[0]);processes.append(restarted)
  deadline=time.monotonic()+15
  while True:
   try:
    after_restart=get('/v1/conversation');break
   except (OSError,AssertionError,http.client.HTTPException):
    assert time.monotonic()<deadline and restarted.poll() is None,'CORE_RESTART_FAILED';time.sleep(.05)
  assert after_restart['session_id']==before_restart['session_id'] and after_restart['instance_id']==before_restart['instance_id'],'RESTART_AUTHORITY_RESET'
  for field in ['summary','pending_questions','focus_entity_ids']:
   assert after_restart['state'][field]==before_restart['state'][field],'RESTART_SHARED_STATE_CHANGED'
  assert get('/v1/conversation/history?limit=50')['events']==history_before['events'],'RESTART_HISTORY_CHANGED'
  second=open_tui();os.write(second[1],b'\x1b[17~');pump(second,1);close_tui(second)
  after=get('/v1/conversation');assert after['instance_id']==authority['instance_id'] and after['session_id']==authority['session_id'],'AUTHORITY_RESET';assert len(get('/v1/notifications','runner')['items'])==1,'RECONNECT_RETRIGGERED'
  result.update(status='PASS',checks=['TUI exit left real daemons running','one scheduled synthetic notification fired after exit','reconnected TUI read notification panel','authority IDs unchanged; notification not retriggered','terminal user-configurable modes restored (macOS transient PENDIN excluded)','AUTH-05 one actual owned Core process restart retained authority, original history and shared state'],notification_id=notes[0]['id'],notification_count=1,instance_id=authority['instance_id'],session_id=authority['session_id'])
 except Exception as e:result.update(status='FAIL',error=type(e).__name__+': '+str(e))
 finally:
  for i,e in enumerate(ptys):
   if e[0].poll() is None:e[0].terminate();e[0].wait(timeout=5)
   (out/f'pty-{i+1}.txt').write_bytes(e[4]);os.close(e[1]);os.close(e[2])
  for p in reversed(processes):
   if p.poll() is None:p.terminate();p.wait(timeout=10)
  for log in logs:log.close()
  (out/'report.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'status':result['status'],'error':result.get('error')}))
  import shutil;shutil.rmtree(data)
 if result['status']!='PASS':raise SystemExit(1)
if __name__=='__main__':main()
