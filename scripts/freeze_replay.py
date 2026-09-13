#!/usr/bin/env python3
"""Author fixed synthetic inputs and independent expected item states."""
import json,hashlib
from pathlib import Path
from datetime import datetime,timedelta,timezone
base=Path(__file__).resolve().parents[1]/'src/tests/fixtures/month'
state={};manifest=[]
for i in range(90):
 day=i//3;point=i%3;stamp=datetime(2026,9,14,tzinfo=timezone.utc)+timedelta(days=day,hours=point*6)
 if i<20:
  title=f'合成事项{i+1:02d}';domain=['work','project','school','life'][i//5];due=(stamp+timedelta(days=4)).strftime('%Y-%m-%dT%H:%M:%S.000Z');state[title]={'domain':domain,'status':'OPEN','due_at':due,'time_state':'CONFIRMED'}
  text=f'这是合成测试，当前业务日期为{stamp.isoformat()}。创建且仅创建一个事项，标题必须原样为“{title}”，domain使用{domain}，截止时间严格为{due}，不要创建提醒或其他任务。'
 else:
  title=f'合成事项{(i-20)%20+1:02d}';op=(i-20)//20
  if op==0:
   due=(stamp+timedelta(days=7)).strftime('%Y-%m-%dT%H:%M:%S.000Z');state[title]['due_at']=due;state[title]['time_state']='CONFIRMED';text=f'这是Master明确修订：只把“{title}”截止时间改为{due}，保持其余事项不变；这是合成数据。'
  elif op==1:
   state[title]['status']='DONE';text=f'Master明确确认“{title}”已经完成。只将此合成事项标记DONE，其他事项不变，不安排额外任务。'
  elif op==2:
   state[title]['status']='CANCELLED';text=f'Master明确修订“{title}”：将此合成事项改为CANCELLED，其他字段不变，不创建额外任务。'
  else:
   state[title]['due_at']=None;state[title]['time_state']='UNKNOWN';text=f'Master明确修订“{title}”：原截止日期不再有效，目前时间未知。将due_at设为null，其余字段不变。'
 inp={'index':i,'day':day,'point':point,'as_of':stamp.isoformat(),'text':text};oracle={'index':i,'items':json.loads(json.dumps(state))}
 for sub,value in [('input',inp),('oracle',oracle)]:
  p=base/sub/f'{i:03d}.json';raw=(json.dumps(value,ensure_ascii=False,sort_keys=True,indent=2)+'\n').encode();p.write_bytes(raw);manifest.append({'path':str(p.relative_to(base)),'sha256':hashlib.sha256(raw).hexdigest()})
(base/'manifest.json').write_text(json.dumps({'version':1,'seed':'authored-month-v1','points':90,'files':manifest},indent=2)+'\n')
print('Frozen 90 raw-input checkpoints and separately authored item oracles.')
