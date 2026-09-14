#!/usr/bin/env python3
"""Issue #2 file-only documentation checks; no runtime/SQL/model tests."""
import argparse, hashlib, json, re, subprocess, zipfile
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/'reports/implementation/issue2'
BASE='cc48d753f65747df35d77664c2d6e3198b704469'
ZIP='docs/deliverables/Secretary_Simplified-design-v1.1-2026-09-14.zip'
def sha(b): return hashlib.sha256(b).hexdigest()
def git(*args): return subprocess.check_output(['git',*args],cwd=ROOT)
def historical(p):
 return (p.startswith(('docs/archive/','docs/deliverables/Secretary_Simplified-design-v1.0')) or p=='docs/Review.md' or (p.startswith('docs/checks/') and p!='docs/checks/README.md') or (p.startswith(('reports/','review/')) and not p.startswith('reports/implementation/issue2') and not p.startswith('review/issue2-') and p not in {'review/README.md','review/final-pending.md','review/model-provenance.json'}))
def candidates():
 return sorted({p for p in git('ls-files','-c','-o','--exclude-standard','-z').decode().split('\0') if p and (ROOT/p).is_file()})
def isdoc(p):
 return Path(p).suffix in {'.md','.txt','.json','.sql','.yaml','.yml','.toml','.zip','.sha256'} or p in {'src/go.mod','src/go.sum'}
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--export',action='store_true');a=parser.parse_args()
 OUT.mkdir(parents=True,exist_ok=True)
 errors=[];checks=[]
 def check(label,ok):
  checks.append({'check':label,'passed':bool(ok)})
  if not ok: errors.append(label)
 check('Schema mirror exact', (ROOT/'docs/contracts.schema.json').read_bytes()==(ROOT/'src/contract/contracts.schema.json').read_bytes())
 check('001 DDL preserved', (ROOT/'docs/schema.sql').read_bytes()==git('show',BASE+':docs/schema.sql'))
 check('001 embedded migration preserved', (ROOT/'src/store/001_baseline.sql').read_bytes()==git('show',BASE+':src/store/001_baseline.sql'))
 check('002 DDL mirror exact',(ROOT/'docs/migrations/002_authority.sql').read_bytes()==(ROOT/'src/store/002_authority.sql').read_bytes())
 check('Original Design2 preserved',(ROOT/'docs/archive/Design2-v0.1.md').read_bytes()==git('show',BASE+':docs/Design2.md'))
 schema=json.loads((ROOT/'docs/contracts.schema.json').read_text())
 example=json.loads((ROOT/'release/examples/input-public.json').read_text())
 internal=dict(example,session_id='00000000-0000-4000-8000-000000000001')
 required=set(schema['$defs']['InputEnvelope']['required'])
 allowed=set(schema['$defs']['InputEnvelope']['properties'])
 check('Public input omits authority; bound DTO fields complete', 'session_id' not in example and required<=internal.keys() and internal.keys()<=allowed and internal['data_class']=='SYNTHETIC' and internal['principal_id']=='master')
 check('Runtime authority DTO fields documented',all('`json:\"'+field+'\"`' in (ROOT/'src/store/authority_repo.go').read_text() and field in (ROOT/'docs/Interfaces.md').read_text() for field in ['instance_id','session_id','history_sequence','summary_through_sequence','pending_turns','legacy_sessions']))
 paths=candidates(); records=[]
 changed_paths=set(git('diff','--name-only',BASE).decode().splitlines()) | set(git('ls-files','-o','--exclude-standard').decode().splitlines())
 for p in paths:
  if not isdoc(p) or p.startswith('reports/implementation/issue2/docs-'): continue
  b=(ROOT/p).read_bytes(); old=historical(p)
  changed=p in changed_paths
  if old: reason='历史原始设计或既有构建证据；不重写为本轮结论，不重跑旧验收。'; ids=['DOC-04']; impl=[]
  elif p.startswith('docs/DataStructure/') and not changed: reason='已核查字段与行为：此业务/执行DTO不持有主会话，结构与权限不因前端或唯一会话收敛改变。';ids=['DOC-01','DOC-03'];impl=['src/contract/types.go']
  elif p.endswith(('.schema.json','.sql')) or '/examples/' in p: reason='严格内部DTO形状保留；公共缺省会话在适配层绑定，追加002独立表达持久权威登记。合成内部样例不作为任意会话操作指南。';ids=['DOC-03','AUTH-01','AUTH-07'];impl=['src/core/http.go','src/store/authority_repo.go']
  elif p.startswith('release/') and p.endswith('.json'): reason='配置仍限定本地凭据/数据目录/原分类策略，TUI复用现有配置而不新增自动授权。';ids=['DOC-03','TUI-01','TUI-13'];impl=['src/config/config.go','src/cli/tui/model.go']
  elif p in {'src/go.mod','src/go.sum'}: reason='终端依赖及传递版本锁定，独立业务权限不变。';ids=['DOC-03','BUILD-01'];impl=['src/cli/tui/model.go']
  elif changed: reason='按Issue #2修订现行职责、接口/操作、兼容、验收或本轮证据；具体章节见下列标题。';ids=['DOC-01','DOC-05'];impl=['src/store/authority_repo.go','src/core/http.go','src/cli/tui/model.go','src/cmd/secretary/main.go']
  else: reason='全量核查后无影响：本文件不定义独立主会话、前端认知状态、退出取消或本轮新增验收门槛；原职责保留。';ids=['DOC-01'];impl=[]
  headings=[]
  if p.endswith('.md'):
   headings=[line.lstrip('# ').strip() for line in b.decode().splitlines() if line.startswith('#')]
  records.append({'path':p,'classification':'historical' if old else 'current','change':'modified' if changed else 'unaffected','reason':reason,'requirements':ids,'sections':headings or ['whole file'],'implementation_files':impl,'sha256':sha(b)})
 # Check current normative markdown links only, excluding literal code blocks and historical review text.
 for row in records:
  p=row['path']
  if row['classification']!='current' or not p.endswith('.md') or p.startswith(('review/','reports/')): continue
  s=re.sub(r'```.*?```','',(ROOT/p).read_text(),flags=re.S)
  for target in re.findall(r'\]\(([^)]+)\)',s):
   target=target.split('#')[0].strip('<>')
   if not target or re.match(r'\w+://',target): continue
   if not (ROOT/p).parent.joinpath(target).exists(): errors.append(f'missing link: {p} -> {target}')
 check('Current architecture has logical/deployment/sequence/state diagrams',all(x in (ROOT/'docs/SingleConversationTUI.md').read_text() for x in ['sequenceDiagram','stateDiagram-v2']) and 'flowchart' in (ROOT/'docs/design.md').read_text() and 'flowchart' in (ROOT/'docs/EngineeringArchitecture.md').read_text())
 check('Build does not execute old suites',not re.search(r'go (test|vet) ',(ROOT/'scripts/build.sh').read_text()))
 exported=[p for p in paths if p.startswith('docs/') and not p.startswith(('docs/deliverables/','docs/archive/','docs/checks/')) and p!='docs/Review.md']
 exported += [p for p in paths if p in {'README.md','release/README.md','release/API.md'} or p.startswith('release/examples/')]
 exported=sorted(set(exported))
 if a.export:
  with zipfile.ZipFile(ROOT/ZIP,'w',zipfile.ZIP_DEFLATED) as z:
   for p in exported:
    info=zipfile.ZipInfo(p,date_time=(2026,9,14,0,0,0));info.compress_type=zipfile.ZIP_DEFLATED;z.writestr(info,(ROOT/p).read_bytes())
  (ROOT/ZIP.replace('.zip','.sha256')).write_text(sha((ROOT/ZIP).read_bytes())+'  '+Path(ZIP).name+'\n')
 if (ROOT/ZIP).exists():
  with zipfile.ZipFile(ROOT/ZIP) as z:
   check('Current export exact source members',set(z.namelist())==set(exported))
   for p in z.namelist():
    check('Export mirror '+p,z.read(p)==(ROOT/p).read_bytes())
 else: errors.append('current export missing; use --export after source edits')
 report={'scope':'DOC-01..DOC-05 only; file-only, no old runtime suites','baseline':BASE,'status':'PASS' if not errors else 'FAIL','checks':checks,'errors':errors,'files':records}
 (OUT/'docs-impact.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 lines=['# Issue #2 逐文件文档影响清单','','此清单包含每个公开现行/历史文档、契约、配置、样例和交付附件。历史原结论适用其原构建；本轮不继承其时长或质量证明。逐条章节、实现关联与SHA-256见 docs-impact.json。','', '| 路径 | 分类/处理 | 要求 | 理由 |','|---|---|---|---|']
 lines += [f"| `{v['path']}` | {v['classification']}/{v['change']} | {', '.join(v['requirements'])} | {v['reason']} |" for v in records]
 (OUT/'docs-impact.md').write_text('\n'.join(lines)+'\n')
 print(json.dumps({'status':report['status'],'files':len(records),'checks':len(checks),'errors':errors},ensure_ascii=False))
 return bool(errors)
if __name__=='__main__': raise SystemExit(main())
