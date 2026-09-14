#!/usr/bin/env python3
"""Issue #2 macOS PTY acceptance. Local authenticated synthetic HTTP only."""
import argparse, base64, datetime, fcntl, hashlib, http.server, json, os, pathlib, pty, re, select, signal, socketserver, struct, subprocess, tempfile, termios, threading, time, uuid

def main():
 ap=argparse.ArgumentParser();ap.add_argument('--binary',required=True);ap.add_argument('--report',required=True);args=ap.parse_args()
 root=pathlib.Path(__file__).resolve().parents[1];report=pathlib.Path(args.report);report.mkdir(parents=True,exist_ok=True)
 if (report/'report.json').exists():raise SystemExit('refusing to overwrite previous report')
 binary=pathlib.Path(args.binary).resolve();owned=pathlib.Path(tempfile.mkdtemp(prefix='ss-tui-',dir='/tmp'));(owned/'run').mkdir();token='issue2-synthetic-local-token';(owned/'run/client.token').write_text(token)
 config=json.loads((root/'release/config.example.json').read_text());config['data_dir']=str(owned);config['model_profile']['profile']='fixture';config['model_profile']['secret_ref']='';(owned/'config.json').write_text(json.dumps(config))
 session=str(uuid.uuid4());instance=str(uuid.uuid4());question=str(uuid.uuid4());lock=threading.Lock()
 state={'events':[], 'turns':{},'requests':{},'posts':[], 'controls':[], 'resolved':False,'delay':False,'drop':False,'core_offline':False,'history_queries':[]}
 for n in range(1,61):state['events'].append({'id':str(uuid.uuid4()),'session_id':session,'sequence':n,'role':'MASTER' if n%2 else 'ASSISTANT','text':f'合成历史第{n}条','turn_id':str(uuid.uuid4()),'delivery_state':'RECORDED'})
 def complete(turn):
  with lock:
   turn['state']='COMMITTED';turn['reply']={'text':'Secretary 已收到中文合成消息'}
   state['events'].append({'id':str(uuid.uuid4()),'session_id':session,'sequence':len(state['events'])+1,'role':'ASSISTANT','text':turn['reply']['text'],'turn_id':turn['id'],'delivery_state':'RECORDED'})
 class Handler(http.server.BaseHTTPRequestHandler):
  def log_message(self,*a):pass
  def send(self,result=None,code=200,error=None):
   data=json.dumps({'status':'OK' if code<400 else 'ERROR','result':result,'error':{'code':error} if error else None},ensure_ascii=False).encode()
   self.send_response(code);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(data)));self.end_headers()
   try:self.wfile.write(data)
   except BrokenPipeError:pass
  def auth(self):return self.headers.get('Authorization')=='Bearer '+token
  def do_GET(self):
   from urllib.parse import urlparse,parse_qs
   if not self.auth():self.send(code=401,error='UNAUTHENTICATED');return
   u=urlparse(self.path);q=parse_qs(u.query)
   if state['core_offline'] and self.server.kind=='core':self.send(code=503,error='DEPENDENCY_UNAVAILABLE');return
   with lock:
    if u.path=='/v1/conversation':return self.send({'instance_id':instance,'session_id':session,'revision':len(state['events']),'history_sequence':len(state['events']),'summary_through_sequence':10,'state':{'summary':'合成共享摘要','focus_entity_ids':[],'pending_questions':[{'id':question,'text':'请确认合成项目日期？','item_id':None,'created_sequence':1,'resolved':state['resolved']}]},'pending_turns':[t for t in state['turns'].values() if t['state']!='COMMITTED']})
    if u.path=='/v1/conversation/history':
     state['history_queries'].append(self.path)
     limit=int(q.get('limit',['50'])[0]);back=q.get('direction',[''])[0]=='backward';after=int(q.get('after_sequence',['0'])[0]);before=int(q.get('before_sequence',['0'])[0]);rows=[e for e in state['events'] if (e['sequence']<before or not before) and (back or e['sequence']>after)];more=len(rows)>limit;rows=rows[-limit:] if back else rows[:limit]
     return self.send({'session_id':session,'events':rows,'next_sequence':rows[-1]['sequence'] if rows else after,'previous_sequence':rows[0]['sequence'] if rows else 0,'has_more':more,'history_sequence':len(state['events'])})
    if u.path.startswith('/v1/requests/'):
     turn=state['requests'].get(u.path.rsplit('/',1)[1]);return self.send({'turn_id':turn['id'],'state':turn['state']}) if turn else self.send(code=404,error='NOT_FOUND')
    if u.path.startswith('/v1/turns/'):
     return self.send(state['turns'].get(u.path.rsplit('/',1)[1]),code=200)
    if u.path=='/v1/health':return self.send({self.server.kind:'ONLINE','model':'UNKNOWN'})
    rows={'tasks':[{'id':'synthetic-task','goal':'合成委托','revision':2,'state':'RUNNING'}],'jobs':[{'id':'synthetic-job','name':'合成计划','enabled':True,'revision':3}],'notifications':[{'id':'synthetic-notification','text':'合成通知\x1b]52;c;ZXZpbA==\x07','state':'RECORDED'}],'items':[{'id':'synthetic-item','title':'合成事项','status':'OPEN'}]}
    if u.path.split('/')[-1] in rows:return self.send({'items':rows[u.path.split('/')[-1]],'next_cursor':None})
   self.send(code=404,error='NOT_FOUND')
  def do_POST(self):
   body=json.loads(self.rfile.read(int(self.headers.get('Content-Length','0'))))
   if not self.auth():self.send(code=401,error='UNAUTHENTICATED');return
   if self.path!='/v1/inputs':
    with lock:state['controls'].append({'path':self.path,'body':body})
    return self.send({'acknowledged':True})
   with lock:
    state['posts'].append(body)
    if body['data_class']!='SYNTHETIC':return self.send(code=403,error='DISCLOSURE_DENIED')
    if body['session_id']!=session:return self.send(code=403,error='AUTHORITY_SESSION_MISMATCH')
    if body.get('answer_to_question_id'):
     if body['answer_to_question_id']!=question or state['resolved']:return self.send(code=409,error='QUESTION_ALREADY_RESOLVED')
     state['resolved']=True
    rid=body['request_id'];turn=state['requests'].get(rid)
    if not turn:
     turn={'id':str(uuid.uuid4()),'request_id':rid,'session_id':session,'state':'ACCEPTED','input':body};state['turns'][turn['id']]=turn;state['requests'][rid]=turn
     state['events'].append({'id':str(uuid.uuid4()),'session_id':session,'sequence':len(state['events'])+1,'role':'MASTER','text':body['text'],'turn_id':turn['id'],'delivery_state':'RECORDED'})
    drop=state['drop'];state['drop']=False;delay=state['delay']
   if not delay:complete(turn)
   if drop:self.connection.close();return
   self.send({'turn_id':turn['id'],'state':turn['state']},202)
 class Server(socketserver.ThreadingMixIn,socketserver.UnixStreamServer):daemon_threads=True
 servers=[]
 for kind in ['core','runner']:
  srv=Server(str(owned/f'run/{kind}.sock'),Handler);srv.kind=kind;servers.append(srv);threading.Thread(target=srv.serve_forever,daemon=True).start()
 transcripts=[];processes=[]
 def launch(extra=None):
  master,slave=pty.openpty();before=termios.tcgetattr(slave);fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',28,100,0,0));env=os.environ.copy();env.update(TERM='xterm-256color',LANG='en_US.UTF-8')
  p=subprocess.Popen([str(binary),'--config',str(owned/'config.json'),'--data-class','SYNTHETIC']+(extra or []),stdin=slave,stdout=slave,stderr=slave,start_new_session=True,env=env);buf=bytearray();entry=(p,master,slave,before,buf);processes.append(entry);return entry
 def pump(entry,seconds=.3):
  end=time.monotonic()+seconds
  while time.monotonic()<end:
   ready,_,_=select.select([entry[1]],[],[],min(.05,max(0,end-time.monotonic())))
   if ready:
    try:b=os.read(entry[1],65536)
    except OSError:break
    if b:entry[4].extend(b)
  return entry[4].decode('utf-8','replace')
 def send(entry,b):os.write(entry[1],b.encode() if isinstance(b,str) else b);pump(entry)
 def wait(predicate,entry,timeout=8):
  end=time.monotonic()+timeout
  while not predicate():
   pump(entry,.1)
   if time.monotonic()>end:raise AssertionError('timed out waiting for expected UI/backend state')
 def finish(entry):
  send(entry,b'\x03');entry[0].wait(timeout=5);pump(entry,.1);after=termios.tcgetattr(entry[2]);after[3]&=~getattr(termios,'PENDIN',0);before=list(entry[3]);before[3]&=~getattr(termios,'PENDIN',0);assert after==before,'user-configurable terminal modes not restored (macOS transient PENDIN excluded)';transcripts.append(bytes(entry[4]));os.close(entry[1]);os.close(entry[2])
 checks=[];error=None
 try:
  e=launch();wait(lambda:b'Secretary' in e[4],e);pump(e,1.5)
  send(e,'/history');send(e,b'\x13');wait(lambda:any('before_sequence=' in q for q in state['history_queries']),e)
  send(e,'第一轮中文');send(e,b'\x13');wait(lambda:len(state['posts'])==1,e);assert state['posts'][0]['text']=='第一轮中文';checks.append('TUI-01/02 default startup and first Chinese input')
  paste='第二轮多行中文\n粘贴第二行\x13草稿';send(e,b'\x1b[200~'+paste.encode()+b'\x1b[201~');assert len(state['posts'])==1
  fcntl.ioctl(e[2],termios.TIOCSWINSZ,struct.pack('HHHH',22,64,0,0));os.kill(e[0].pid,signal.SIGWINCH);pump(e,.3);send(e,b'\x13');wait(lambda:len(state['posts'])==2,e);assert state['posts'][1]['text']=='第二轮多行中文\n粘贴第二行草稿';checks.append('TUI-03 Chinese bracketed multiline paste, CtrlS in paste, resize retains one draft')
  state['delay']=True;state['drop']=True;send(e,'第三轮中文丢响应');send(e,b'\x13');wait(lambda:len(state['posts'])==3,e);send(e,'等待时的新草稿');send(e,b'\x1b[5~');pump(e,2);assert len(state['posts'])==3;checks.append('TUI-04/07 response loss keeps stable pending ID while draft and scroll respond')
  first_id=state['posts'][2]['request_id'];finish(e);state['delay']=False;complete(state['requests'][first_id]);e=launch();pump(e,2);assert len(state['posts'])==3;assert len(list((owned/'run/tui-pending').glob('*.json')))==0;checks.append('TUI-05/07 reconnect queries original request without POST; history tail paging')
  send(e,b'\x1bOQ');pump(e,.4);send(e,b'\r');send(e,'2026年10月1日');send(e,b'\x13');wait(lambda:len(state['posts'])==4,e);assert state['posts'][3]['answer_to_question_id']==question and state['resolved'];checks.append('TUI-06 structured question selection binds authoritative ID')
  state['core_offline']=True;send(e,b'\x1bOR');pump(e,1);send(e,b'\r');assert not state['controls'];send(e,'y');wait(lambda:len(state['controls'])==1,e);assert state['controls'][0]['path']=='/v1/tasks/synthetic-task/cancel' and state['controls'][0]['body']['expected_revision']==2;checks.append('TUI-09 explicit target/revision confirmation works with Core offline')
  send(e,b'\x1b[17~');pump(e,1);assert len(state['controls'])==1;send(e,b'\r');send(e,'y');wait(lambda:len(state['controls'])==2,e);assert state['controls'][1]['path'].endswith('/ack');assert b'\x1b]52;c;ZXZpbA==' not in e[4];checks.append('TUI-10/12 notification refresh never acks; explicit ack; OSC not rendered')
  finish(e);checks.append('TUI-12 CtrlC restores original terminal mode flags')
  state['core_offline']=False
  e=launch();pump(e,1);e[0].send_signal(signal.SIGTERM);e[0].wait(timeout=5);pump(e,.1);after=termios.tcgetattr(e[2]);before=list(e[3]);after[3]&=~getattr(termios,'PENDIN',0);before[3]&=~getattr(termios,'PENDIN',0);assert after==before,'SIGTERM did not restore user terminal modes';os.close(e[1]);os.close(e[2]);checks.append('TUI-12 catchable SIGTERM restores terminal modes')
  e=launch(['chat','--json']);e[0].wait(timeout=5);pump(e,.1);assert e[0].returncode!=0 and b'\x1b' not in e[4] and b'NONINTERACTIVE_INPUT_REQUIRED' in e[4],'JSON TTY waited for keyboard or emitted ANSI';os.close(e[1]);os.close(e[2]);checks.append('TUI-11 explicit JSON on TTY returns guidance without keyboard read or ANSI')
  result=subprocess.run([str(binary),'chat','--plain','--json','--config',str(owned/'config.json'),'--data-class','SYNTHETIC'],input='管道输入中文\n',text=True,capture_output=True,timeout=8);assert result.returncode==0,result.stderr;assert '\x1b' not in result.stdout;json.loads(result.stdout);checks.append('TUI-11 pipe EOF and JSON output without ANSI or keyboard wait')
 except Exception as ex:error=type(ex).__name__+': '+str(ex)
 finally:
  for entry in processes:
   if entry[0].poll() is None:
    entry[0].terminate();entry[0].wait(timeout=5)
  for srv in servers:srv.shutdown();srv.server_close()
  transcripts=[bytes(entry[4]) for entry in processes]
  for n,data in enumerate(transcripts):
   (report/f'pty-{n+1}.txt').write_bytes(data)
  result={'scope':'Issue2 TUI local synthetic authenticated HTTP + macOS PTY; no old baseline/long soak/model API','status':'FAIL' if error else 'PASS','error':error,'checks':checks,'binary_sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),'platform':os.uname().sysname,'posts':state['posts'],'controls':state['controls'],'history_queries':state['history_queries'],'terminal_transcripts':len(transcripts),'at':datetime.datetime.now(datetime.timezone.utc).isoformat()};(report/'report.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'status':result['status'],'checks':len(checks),'error':error},ensure_ascii=False))
  import shutil;shutil.rmtree(owned)
 if error:raise SystemExit(1)
if __name__=='__main__':main()
