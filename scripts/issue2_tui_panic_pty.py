#!/usr/bin/env python3
"""TUI-12 synthetic panic in actual Model wrapper; no production hooks or old tests."""
import argparse,fcntl,hashlib,json,os,pathlib,pty,select,struct,subprocess,tempfile,termios,time

def main():
 ap=argparse.ArgumentParser(description=__doc__);ap.add_argument('--helper',required=True);ap.add_argument('--report',required=True);a=ap.parse_args();root=pathlib.Path(__file__).resolve().parents[1];out=pathlib.Path(a.report);out.mkdir(parents=True,exist_ok=False);helper=pathlib.Path(a.helper).resolve();master,slave=pty.openpty();before=termios.tcgetattr(slave);fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',28,100,0,0));env=dict(os.environ,TERM='xterm-256color',LANG='en_US.UTF-8',SECRETARY_ISSUE2_PANIC_PTY='1');p=subprocess.Popen([str(helper),'-test.run=^TestIssue2TUIPanicPTYHelper$','-test.timeout=10s'],stdin=slave,stdout=slave,stderr=slave,start_new_session=True,env=env);buf=bytearray();error=None
 def pump(seconds):
  until=time.monotonic()+seconds
  while time.monotonic()<until:
   if select.select([master],[],[],.05)[0]:
    try:buf.extend(os.read(master,65536))
    except OSError:break
 try:
  deadline=time.monotonic()+8
  while b'Secretary' not in buf:
   pump(.1);assert p.poll() is None and time.monotonic()<deadline,'helper not ready before injection'
  os.write(master,b'\x18');deadline=time.monotonic()+5
  while p.poll() is None:
   pump(.1);assert time.monotonic()<deadline,'panic recovery did not exit'
  pump(.2);after=termios.tcgetattr(slave);before[3]&=~getattr(termios,'PENDIN',0);after[3]&=~getattr(termios,'PENDIN',0);assert before==after,'terminal configuration not restored';assert b'synthetic-issue2-catchable-panic' in buf,'no actual panic';assert p.returncode==0,'helper did not observe recovered panic'
 except Exception as e:error=str(e)
 finally:
  if p.poll() is None:p.terminate();p.wait(timeout=5)
  os.close(master);os.close(slave)
 private=root/'reports/local/issue2-tui-panic';private.mkdir(parents=True,exist_ok=True);raw=private/(out.name+'.raw');raw.write_bytes(buf);raw.chmod(0o600)
 # The original Go stack has host-specific module/workspace paths. Public copy is
 # explicitly normalized; original bytes are retained privately, never called exact.
 public=buf.decode('utf-8','replace').replace(str(root),'$WORKSPACE').replace(str(pathlib.Path.home()),'$HOME')
 (out/'pty.normalized.txt').write_text(public)
 r={'scope':'TUI-12 child-only synthetic panic wrapper around actual Model and identical production Run Bubble Tea options','status':'FAIL' if error else 'PASS','error':error,'helper_sha256':hashlib.sha256(helper.read_bytes()).hexdigest(),'raw_sha256':hashlib.sha256(buf).hexdigest(),'public_transcript':'pty.normalized.txt','transcript_notice':'Normalized workspace/home paths, not byte-exact original; private raw retained under reports/local','production_release_changed':False,'terminal_comparison':'all termios fields exact except approved transient macOS PENDIN'};(out/'report.json').write_text(json.dumps(r,indent=2)+'\n');print(json.dumps({'status':r['status'],'error':error}));raise SystemExit(1 if error else 0)
if __name__=='__main__':main()
