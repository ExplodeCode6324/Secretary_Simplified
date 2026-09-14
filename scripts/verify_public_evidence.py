#!/usr/bin/env python3
"""Verify all public synthetic datasets and reproduce all available month audits."""
import argparse
import hashlib
import json
import re
import shutil
import subprocess
import tempfile
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check-only', action='store_true', help='Verify without rewriting summary reports')
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[1]
    public = repo/'reports/live-model'
    keys = []
    for path in (repo/'resource').glob('*.md'):
        if 'api' not in path.name.lower():
            continue
        text = path.read_text().strip()
        values = [s.strip().strip('`') for s in text.splitlines()
                  if re.fullmatch(r'[A-Za-z0-9_-]{20,}', s.strip().strip('`'))]
        values = values or re.findall(r'sk-[A-Za-z0-9_-]{20,}', text)
        if len(set(values)) != 1:
            raise SystemExit('Credential source format requires review; no values emitted')
        keys.append(values[0].encode())
    if len(keys) != 2:
        raise SystemExit('Two actual credential sources are required')
    problems, datasets, reproductions = [], [], []
    allfiles = [p for p in public.rglob('*') if p.is_file()]
    for directory in sorted(p for p in public.iterdir() if p.is_dir()):
        name = directory.name.removesuffix('-failed')
        local = repo/'reports/local'/('live-'+name)
        manifest = directory/'publication-manifest.json'
        files = [p for p in directory.rglob('*') if p.is_file()]
        obj_count = None
        if manifest.exists():
            metadata = json.loads(manifest.read_text())
            obj_count = metadata['object_count']
            listed = set()
            for row in metadata['files']:
                rel = Path(row['path'])
                if rel.is_absolute() or '..' in rel.parts:
                    problems.append('unsafe manifest path')
                    continue
                listed.add(rel.as_posix())
                path = directory/rel
                if not path.is_file():
                    problems.append('manifest file missing')
                    continue
                raw = path.read_bytes()
                if len(raw) != row['bytes'] or hashlib.sha256(raw).hexdigest() != row['sha256']:
                    problems.append('manifest integrity failure')
            if {p.relative_to(directory).as_posix() for p in files} != listed | {'publication-manifest.json'}:
                problems.append('manifest file inventory mismatch')
        source = local/'report.json'
        same = None
        if source.exists() and (directory/'report.json').exists():
            same = source.read_bytes() == (directory/'report.json').read_bytes()
            if not same:
                problems.append('original report changed')
        datasets.append({'path': directory.relative_to(repo).as_posix(), 'files': len(files),
                         'objects': obj_count, 'bytes': sum(p.stat().st_size for p in files),
                         'manifest_integrity': 'PASS' if manifest.exists() else 'LEGACY_NO_MANIFEST',
                         'original_report_bytes_preserved': same})
        audit_names = ('report-identity-correction.json', 'consciousness-reference-audit.json')
        if all((directory/n).exists() for n in (*audit_names, 'audit-records.json')):
            with tempfile.TemporaryDirectory(prefix='public-month-audit-') as temporary:
                target = Path(temporary)
                shutil.copyfile(directory/'report.json', target/'report.json')
                run = subprocess.run(['python3', str(repo/'scripts/audit_month_evidence.py'),
                                      str(directory), str(target), str(repo/'src/tests/fixtures/month')],
                                     capture_output=True, text=True, check=True)
                counts = json.loads(run.stdout)
                exact = {n: (target/n).read_bytes() == (directory/n).read_bytes() for n in audit_names}
                if not all(exact.values()) or counts['identity_status'] != 'PASS' or counts['snapshot_reference_failures']:
                    problems.append('public audit reproduction failure')
                reproductions.append({'source': (directory/'audit-records.json').relative_to(repo).as_posix(),
                                      'source_sha256': hashlib.sha256((directory/'audit-records.json').read_bytes()).hexdigest(),
                                      'exact_byte_reproduction': exact,
                                      'identity_checkpoints': counts['identity_checkpoints'],
                                      'consciousness_snapshots': counts['snapshot_count']})
    for path in allfiles:
        raw = path.read_bytes()
        if any(key in raw for key in keys):
            problems.append('actual credential bytes present')
        if re.search(rb'/Users/|/private/(?:tmp|var)/|/tmp/sec-|reports/local/', raw):
            problems.append('private path present')
        if re.search(r'(?:config|token|grant|\.sqlite(?:-|$)|\.db$)', path.name, re.I):
            problems.append('excluded filename present')
    if problems:
        raise SystemExit(json.dumps({'status': 'FAIL', 'problem_categories': sorted(set(problems))}))
    reproduction = {'status': 'PASS', 'script': 'scripts/audit_month_evidence.py',
                    'private_database_required': False, 'datasets': reproductions}
    report = {'status': 'PASS', 'datasets': datasets, 'scanned_files': len(allfiles),
              'total_dataset_objects': sum(d['objects'] or 0 for d in datasets),
              'actual_credential_sources_checked': 2, 'exact_credential_byte_matches': 0,
              'home_and_private_data_path_matches': 0, 'manifest_integrity_failures': 0,
              'original_reports_preserved': sum(d['original_report_bytes_preserved'] is True for d in datasets),
              'public_only_audit_reproduction': {'status': 'PASS', 'datasets': len(reproductions)},
              'notes': 'All public dataset directories scanned. Legacy directories without publication manifests are explicitly distinguished; their inventory is not claimed independently frozen.'}
    if not args.check_only:
        for name, value in [('public-evidence-publication.json', report),
                            ('public-evidence-reproduction.json', reproduction)]:
            (repo/'reports/implementation'/name).write_text(json.dumps(value, indent=2)+'\n')
    print(json.dumps(report))


if __name__ == '__main__':
    main()
