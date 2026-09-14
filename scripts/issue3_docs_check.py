#!/usr/bin/env python3
"""DOC-R only: file inventory, current links/mirrors/export, no runtime suites."""
import argparse
import hashlib
import json
import re
import subprocess
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BASE = 'c84d92453a1d8338be728422e3164777ef822579'
OUT = ROOT / 'reports/implementation/issue3'
ZIP = 'docs/deliverables/Secretary_Simplified-design-v1.1.1-2026-09-14.zip'

def git(*args):
    return subprocess.check_output(['git', *args], cwd=ROOT)

def sha(data):
    return hashlib.sha256(data).hexdigest()

def historical(p):
    if p in {'review/README.md', 'review/final-pending.md', 'review/model-provenance.json', 'reports/implementation/issue2/README.md'}:
        return False
    if p.startswith(('reports/', 'review/')):
        return not p.startswith(('reports/implementation/issue3', 'review/issue3-'))
    return (p.startswith('docs/archive/') or p == 'docs/Review.md' or
            (p.startswith('docs/checks/') and p != 'docs/checks/README.md') or
            (p.startswith('docs/deliverables/') and p != 'docs/deliverables/README.md' and 'v1.1.1-' not in p))

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--export', action='store_true')
    args = parser.parse_args()
    OUT.mkdir(parents=True, exist_ok=True)
    files = sorted({p for p in git('ls-files', '-co', '--exclude-standard', '-z').decode().split('\0') if p and (ROOT/p).is_file()})
    changed = set(git('diff', '--name-only', BASE).decode().splitlines()) | set(git('ls-files', '-o', '--exclude-standard').decode().splitlines())
    checks, errors, records = [], [], []
    def check(name, passed):
        checks.append({'name': name, 'passed': bool(passed)})
        if not passed:
            errors.append(name)
    for p in ['docs/contracts.schema.json', 'src/contract/contracts.schema.json', 'docs/schema.sql', 'src/store/001_baseline.sql', 'src/store/002_authority.sql', 'docs/migrations/002_authority.sql', 'src/go.mod', 'src/go.sum', 'release/secretaryd', 'release/db/secretary.sqlite']:
        check('Unchanged contract/dependency/daemon/db: '+p, (ROOT/p).read_bytes() == git('show', BASE+':'+p))
    check('Schema mirror', (ROOT/'docs/contracts.schema.json').read_bytes() == (ROOT/'src/contract/contracts.schema.json').read_bytes())
    for p in files:
        if (Path(p).suffix not in {'.md', '.json', '.txt', '.sql', '.yaml', '.yml', '.toml', '.zip', '.sha256'} and p not in {'release/SHA256SUMS', 'src/go.mod', 'src/go.sum'}) or p.startswith('reports/implementation/issue3/docs-'):
            continue
        old = historical(p)
        data = (ROOT/p).read_bytes()
        headings = [line.lstrip('# ').strip() for line in data.decode().splitlines() if line.startswith('#')] if p.endswith('.md') else ['whole file']
        if old:
            reason = 'Historical evidence/snapshot; preserve its original result and build attribution. No old suite rerun.'
        elif p in changed:
            reason = 'Updated residual submit/observe or panel navigation/refresh rules, current index, or new evidence. No backend authority/schema migration.'
        else:
            reason = 'Scope scan: existing contract/operation does not change for these client-only state transitions; authoritative backend semantics retained.'
        records.append({'path': p, 'classification': 'historical' if old else 'current', 'change': 'modified' if p in changed else 'unaffected', 'reason': reason, 'sections': headings, 'requirements': ['DOC-R'] + (['R1-01..03', 'R2-01..03'] if p in changed and not old else []), 'implementation_files': [] if old else ['src/cli/tui/model.go', 'src/cli/tui/client.go', 'src/cli/tui/panels.go'], 'sha256': sha(data)})
        if old and p in changed and p in git('ls-tree', '-r', '--name-only', BASE, '--', p).decode().splitlines():
            check('Historical bytes preserved: '+p, data == git('show', BASE+':'+p))
        if not old and p.endswith('.md') and not p.startswith(('review/', 'reports/')):
            text = re.sub(r'```.*?```', '', data.decode(), flags=re.S)
            for link in re.findall(r'\]\(([^)]+)\)', text):
                link = link.split('#')[0].strip('<>')
                if link and not re.match(r'\w+://', link) and not (ROOT/p).parent.joinpath(link).exists():
                    errors.append('Missing link: '+p+' -> '+link)
    members = sorted(p for p in files if (p.startswith('docs/') and not p.startswith(('docs/archive/', 'docs/checks/', 'docs/deliverables/')) and p != 'docs/Review.md') or p in {'README.md', 'release/README.md', 'release/API.md'} or p.startswith('release/examples/'))
    if args.export:
        with zipfile.ZipFile(ROOT/ZIP, 'w', zipfile.ZIP_DEFLATED) as z:
            for p in members:
                info = zipfile.ZipInfo(p, date_time=(2026, 9, 14, 0, 0, 0))
                info.compress_type = zipfile.ZIP_DEFLATED
                z.writestr(info, (ROOT/p).read_bytes())
        (ROOT/ZIP.replace('.zip', '.sha256')).write_text(sha((ROOT/ZIP).read_bytes())+'  '+Path(ZIP).name+'\n')
    if (ROOT/ZIP).exists():
        with zipfile.ZipFile(ROOT/ZIP) as z:
            check('Current export member set', set(z.namelist()) == set(members))
            for p in z.namelist():
                check('Export mirror '+p, z.read(p) == (ROOT/p).read_bytes())
        check('Export checksum', (ROOT/ZIP.replace('.zip', '.sha256')).read_text().split()[0] == sha((ROOT/ZIP).read_bytes()))
    else:
        errors.append('Current export missing')
    # Export can replace its bytes during this invocation; inventory final bytes.
    for record in records:
        record['sha256'] = sha((ROOT/record['path']).read_bytes())
    report = {'scope': 'DOC-R file-only check; no old runtime/model tests', 'baseline': BASE, 'status': 'FAIL' if errors else 'PASS', 'checks': checks, 'errors': errors, 'files': records}
    (OUT/'docs-impact.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
    lines = ['# Issue #3 document impact inventory', '', 'Per-file sections, implementation links and hashes are in docs-impact.json. Previous reports remain historical evidence.', '', '| Path | Classification / change | Reason |', '|---|---|---|']
    lines += ['| `'+x['path']+'` | '+x['classification']+' / '+x['change']+' | '+x['reason']+' |' for x in records]
    (OUT/'docs-impact.md').write_text('\n'.join(lines)+'\n')
    print(json.dumps({'status': report['status'], 'files': len(records), 'checks': len(checks), 'errors': errors}))
    return bool(errors)

if __name__ == '__main__':
    raise SystemExit(main())
