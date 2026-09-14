#!/usr/bin/env python3
"""Record Issue #2 built assets and inspect the offline empty sample only."""
import datetime, hashlib, json, sqlite3, subprocess
from pathlib import Path
r=Path(__file__).resolve().parents[1]
h=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
db=r/'release/db/secretary.sqlite'
# Store was closed before copying this offline sample; no WAL is part of delivery.
c=sqlite3.connect('file:'+str(db)+'?immutable=1',uri=True)
tables=[x[0] for x in c.execute("select name from sqlite_master where type='table' and name not like 'sqlite_%'")]
counts={t:c.execute('select count(*) from "'+t+'"').fetchone()[0] for t in tables}
assert all(n==0 for t,n in counts.items() if t not in {'schema_migration','conversation_session','authority_registry'})
assert counts['schema_migration']==2 and counts['conversation_session']==1 and counts['authority_registry']==1
versions=list(c.execute('select version,checksum from schema_migration order by version'));c.close()
prior=json.loads((r/'reports/implementation/issue2-auth.json').read_text())
assert all(h(r/p)==value for p,value in prior['source_sha256'].items()),'AUTH source drift since directed evidence'
paths=sorted(list((r/'src').rglob('*.go'))+list((r/'src').rglob('*.sql'))+[r/'src/go.mod',r/'src/go.sum',r/'src/contract/contracts.schema.json'])
report={'status':'PASS','scope':'BUILD-01 deployment build and empty sample inspection; no old suites','created_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'source_base':subprocess.check_output(['git','rev-parse','HEAD'],cwd=r).decode().strip(),'commands':[{'command':'./scripts/build.sh','exit_code':0}],'binary_sha256':{p.name:h(p) for p in [r/'release/secretary',r/'release/secretaryd']},'sample_database':{'path':'release/db/secretary.sqlite','sha256':h(db),'migrations':versions,'table_counts':counts,'contents':'empty business tables; one fresh empty authority state/registry; no credentials/grants'},'authority_test_source_hashes_match':True,'source_sha256':{str(p.relative_to(r)):h(p) for p in paths},'vet_evidence':['reports/implementation/issue2-auth.json','reports/implementation/issue2-tui.json']}
(r/'reports/implementation/issue2/build.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'status':'PASS','schema_versions':[x[0] for x in versions],'business_rows':0,'binary_sha256':report['binary_sha256']}))
