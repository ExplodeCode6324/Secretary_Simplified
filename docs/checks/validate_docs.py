#!/usr/bin/env python3
"""Validate design artifacts only; this is not a Secretary runtime test."""
import argparse
import hashlib
import json
import re
import sqlite3
import sys
import tempfile
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import unquote

try:
    from jsonschema import Draft202012Validator, FormatChecker
    import rfc3339_validator  # Enables jsonschema's optional date-time format checker.
except ImportError:
    raise SystemExit('Missing document dependency: install checks/requirements-docs.txt in an isolated environment.')

ROOT = Path(__file__).resolve().parents[1]
EXPECTED_DRAFT = '81c528c11ca512c638d79c067df8e091fe50514ac3600d6c78630c6673f25ef5'
errors = []
counts = {}


def check(ok, message):
    if not ok:
        errors.append(message)


def sql_checks():
    checks = 0
    with tempfile.TemporaryDirectory(prefix='secretary-doc-ddl-') as temp:
        conn = sqlite3.connect(str(Path(temp) / 'test.sqlite'))
        conn.executescript((ROOT / 'schema.sql').read_text())
        tables = {r[0] for r in conn.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")}
        counts['sqlite_tables'] = len(tables)
        storage = (ROOT / 'Storage.md').read_text()
        for table in tables:
            check(table in storage, 'Table absent from Storage.md: ' + table)

        def reject(statement, params, label):
            nonlocal checks
            checks += 1
            conn.execute('SAVEPOINT negative')
            try:
                conn.execute(statement, params)
            except sqlite3.IntegrityError:
                pass
            else:
                errors.append('SQL accepted invalid case: ' + label)
            finally:
                conn.execute('ROLLBACK TO negative')
                conn.execute('RELEASE negative')

        now = '2026-09-14T01:00:00.000Z'
        insert_item = 'INSERT INTO item VALUES(?,?,?,?,?,?,?,?)'
        conn.execute(insert_item, ('item-1', 1, 'project', 'OPEN', None, '{}', now, now))
        reject(insert_item, ('item-1', 1, 'project', 'OPEN', None, '{}', now, now), 'item identity')
        reject(insert_item, (None, 1, 'project', 'OPEN', None, '{}', now, now), 'null primary key')
        reject(insert_item, ('item-2', 0, 'project', 'OPEN', None, '{}', now, now), 'revision zero')
        reject(insert_item, ('item-2', 1, 'project', 'UNKNOWN', None, '{}', now, now), 'invalid item state')
        reject(insert_item, ('item-2', 1, 'project', 'OPEN', None, '{', now, now), 'malformed JSON')
        reject('INSERT INTO item_dependency VALUES(?,?)', ('item-1', 'missing'), 'dependency foreign key')
        reject('INSERT INTO item_dependency VALUES(?,?)', ('item-1', 'item-1'), 'self dependency')
        conn.execute('INSERT INTO request_receipt VALUES(?,?,?,?,?,?,?)', ('master', 'req-1', 'hash', 'intent-1', 'ACCEPTED', '{}', now))
        reject('INSERT INTO request_receipt VALUES(?,?,?,?,?,?,?)', ('master', 'req-1', 'other', 'intent-2', 'ACCEPTED', '{}', now), 'request deduplication')
        conn.execute('INSERT INTO command_ledger VALUES(?,?,?,?,?,?,?,?)', ('cmd-1', 'intent-1', 'op', 'hash', None, None, '{}', now))
        reject('INSERT INTO command_ledger VALUES(?,?,?,?,?,?,?,?)', ('cmd-2', 'intent-1', 'op', 'hash', None, None, '{}', now), 'intent-operation deduplication')
        conn.execute('INSERT INTO consciousness_snapshot VALUES(?,?,?,?,?,?)', ('cs-1', 0, 1, 0, now, '{}'))
        reject('INSERT INTO consciousness_snapshot VALUES(?,?,?,?,?,?)', ('cs-2', 0, 1, 0, now, '{}'), 'consciousness slot uniqueness')
        reject('INSERT INTO world_fact_head VALUES(?,?)', ('missing', 1), 'fact head foreign key')
        conn.execute('INSERT INTO root_budget VALUES(?,?,?)', ('root-1', 1, '{}'))
        conn.execute('INSERT INTO task VALUES(?,?,?,?,?,?,?,?,?,?,?)', ('task-1', 1, 'root-1', None, None, None, 'PENDING', 'hash', 0, '{}', now))
        run = ('run-1', 'task-1', None, None, 'occurrence-1', now, 'QUEUED', 0, 0, None, None, 'external-1', '{}', now)
        # Derive placeholder count to keep this test bound to the explicit tuple.
        insert_run = 'INSERT INTO job_run VALUES(' + ','.join('?' for _ in run) + ')'
        conn.execute(insert_run, run)
        reject(insert_run, ('run-2',) + run[1:11] + ('external-2',) + run[12:], 'occurrence uniqueness')
        conn.commit()
        conn.execute('BEGIN IMMEDIATE')
        conn.execute(insert_item, ('rolled-back', 1, 'life', 'OPEN', None, '{}', now, now))
        conn.rollback()
        checks += 1
        check(conn.execute("SELECT count(*) FROM item WHERE id='rolled-back'").fetchone()[0] == 0, 'SQL rollback failed')
        check(conn.execute('PRAGMA foreign_key_check').fetchall() == [], 'Foreign key check failed')
        check(conn.execute('PRAGMA integrity_check').fetchone()[0] == 'ok', 'SQLite integrity check failed')
        backup = sqlite3.connect(str(Path(temp) / 'backup.sqlite'))
        conn.backup(backup)
        checks += 1
        check(backup.execute('SELECT count(*) FROM item').fetchone() == conn.execute('SELECT count(*) FROM item').fetchone(), 'SQLite backup mismatch')
        backup.close()
        conn.close()
    counts['sql_constraint_and_smoke_checks'] = checks


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--report', type=Path)
    args = parser.parse_args()
    check(not FormatChecker().conforms('tomorrow', 'date-time'), 'RFC3339 format checker is inactive')
    draft_hash = hashlib.sha256((ROOT / 'Design2.md').read_bytes()).hexdigest()
    check(draft_hash == EXPECTED_DRAFT, 'Original Design2.md changed')
    schema = json.loads((ROOT / 'contracts.schema.json').read_text())
    Draft202012Validator.check_schema(schema)
    definitions = schema['$defs']

    def references(value):
        if isinstance(value, dict):
            if '$ref' in value:
                ref = value['$ref']
                check(ref.startswith('#/$defs/') and ref[8:] in definitions, 'Unresolved Schema ref: ' + ref)
            for child in value.values():
                references(child)
        elif isinstance(value, list):
            for child in value:
                references(child)

    references(schema)
    contracts = {k: v for k, v in definitions.items() if 'schema_version' in v.get('properties', {})}
    for name, spec in contracts.items():
        path = ROOT / 'DataStructure' / (name + '.md')
        check(path.exists(), 'Missing DTO document: ' + name)
        if path.exists():
            fields = set(re.findall(r'^\| ([a-z][a-z0-9_]*) \|', path.read_text(), re.M))
            check(fields == set(spec['properties']), 'DTO fields differ from Schema: ' + name)
    counts['primary_contracts'] = len(contracts)
    counts['schema_definitions'] = len(definitions)
    counts['positive_examples'] = counts['negative_examples'] = 0
    for entry in json.loads((ROOT / 'examples/manifest.json').read_text()):
        data = json.loads((ROOT / 'examples' / entry['file']).read_text())
        target = {'$schema': schema['$schema'], '$defs': definitions, '$ref': '#/$defs/' + entry['definition']}
        violations = list(Draft202012Validator(target, format_checker=FormatChecker()).iter_errors(data))
        check((not violations) == entry['valid'], 'Example mismatch: ' + entry['file'] + ('; ' + violations[0].message if violations else ''))
        counts['positive_examples' if entry['valid'] else 'negative_examples'] += 1
        if entry['valid'] and entry['definition'] == 'ObjectRef':
            source = ROOT / 'examples' / data['relative_path']
            check(source.exists(), 'Example object is missing: ' + data['relative_path'])
            if source.exists():
                check(hashlib.sha256(source.read_bytes()).hexdigest() == data['sha256'], 'Example object hash differs: ' + data['relative_path'])
                check(source.stat().st_size == data['byte_size'], 'Example object size differs: ' + data['relative_path'])
    markdown = sorted(ROOT.rglob('*.md'))
    links = 0
    for path in markdown:
        content = path.read_text()
        for target in re.findall(r'\[[^\]]+\]\(([^)]+)\)', content):
            target = target.strip('<>')
            if re.match(r'^[a-zA-Z][a-zA-Z0-9+.-]*:', target) or target.startswith('#'):
                continue
            target = unquote(target.split('#')[0])
            links += 1
            check((path.parent / target).exists(), f'Broken local link: {path.relative_to(ROOT)} -> {target}')
    counts['markdown_files'] = len(markdown)
    counts['local_links'] = links
    sql_checks()
    report = {'checked_at': datetime.now(timezone.utc).isoformat(), 'scope': 'DOC_ONLY',
              'status': 'PASS' if not errors else 'FAIL', 'draft_sha256': draft_hash,
              'contracts_sha256': hashlib.sha256((ROOT / 'contracts.schema.json').read_bytes()).hexdigest(),
              'ddl_sha256': hashlib.sha256((ROOT / 'schema.sql').read_bytes()).hexdigest(),
              'sqlite_version': sqlite3.sqlite_version, 'counts': counts, 'errors': errors,
              'not_tested': ['Go runtime', 'Go SQLite driver', 'model calls', 'real source data', 'audio', 'continuous real-world operation']}
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 1 if errors else 0


if __name__ == '__main__':
    sys.exit(main())
