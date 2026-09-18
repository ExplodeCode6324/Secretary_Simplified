#!/usr/bin/env python3
"""Validate design artifacts; this is not application/runtime acceptance."""
import argparse
import datetime as dt
import hashlib
import json
import pathlib
import re
import sqlite3
import subprocess
import tempfile
from urllib.parse import unquote

from jsonschema import Draft202012Validator, FormatChecker
import rfc3339_validator  # Explicit format dependency; checked below.

DOCS = pathlib.Path(__file__).resolve().parents[1]
ROOT = DOCS.parent
checks = []


def check(name, function):
    try:
        detail = function()
        checks.append({"name": name, "status": "PASS", "detail": detail})
    except Exception as error:
        checks.append({"name": name, "status": "FAIL", "error": str(error)})


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def load(relative):
    return json.loads((DOCS / relative).read_text())


def contracts():
    schema = load('contracts/contracts.schema.json')
    Draft202012Validator.check_schema(schema)
    formats = FormatChecker()
    require(not formats.conforms('tomorrow', 'date-time'), 'date-time checker disabled')
    require(not formats.conforms('bad-id', 'uuid'), 'UUID checker disabled')
    validator = Draft202012Validator(schema, format_checker=formats)
    manifest = load('examples/manifest.json')['examples']
    for entry in manifest:
        accepted = validator.is_valid(load('examples/' + entry['file']))
        require(accepted == entry['valid'], 'Unexpected example result: ' + entry['file'])
    declared = load('contracts/command-registry.json')['records']
    covered = {e['record'] for e in manifest if e['valid']}
    require(set(declared) == covered, 'Every primary DTO needs a positive example')
    tools = load('contracts/tools.schema.json')
    Draft202012Validator.check_schema(tools)
    names = {v['properties']['tool']['const'] for v in tools['oneOf']}
    registry = load('contracts/tool-registry.json')
    require(names == set(registry['main'] + registry['child']), 'Tool registry differs from schemas')
    require(not set(registry['main']) & set(registry['child']), 'Role tool overlap')
    require('command.run' not in registry['main'], 'Main exposes external command tool')
    from referencing import Registry, Resource
    references = Registry().with_resource(schema['$id'], Resource.from_contents(schema))
    tool_validator = Draft202012Validator(tools, registry=references, format_checker=formats)
    tool_cases = load('examples/tool-cases.json')['cases']
    for case in tool_cases:
        require(tool_validator.is_valid(case['payload']) == case['valid'], 'Tool example: ' + case['case'])
    return {'primary_dtos': len(declared), 'definitions': len(schema['$defs']),
            'positive_examples': sum(e['valid'] for e in manifest),
            'negative_examples': sum(not e['valid'] for e in manifest), 'tools': len(names), 'tool_cases': len(tool_cases)}


def document_links():
    count = 0
    files = list(DOCS.rglob('*.md')) + [ROOT / 'README.md']
    files = [p for p in files if 'node_modules' not in p.parts]
    for path in files:
        text = path.read_text()
        require(text.count('```') % 2 == 0, f'Unclosed code fence: {path.name}')
        for raw in re.findall(r'\[[^\]]*\]\(([^)]+)\)', text):
            if raw.startswith(('https://', 'http://', '#', 'mailto:')):
                continue
            target = unquote(raw.split('#')[0].strip('<>'))
            require((path.parent / target).exists(), f'Broken link: {path.relative_to(ROOT)} -> {raw}')
            count += 1
    return {'markdown_files': len(files), 'local_links': count}


def traceability():
    acceptance = load('contracts/acceptance.json')
    scenarios = acceptance['scenarios']
    require({r['id'] for r in scenarios} == {f'T{i:02}' for i in range(1, 31)}, 'Scenario coverage incomplete')
    require(all(r['status'] == 'NOT_RUN' for r in scenarios), 'Design must not claim app acceptance')
    graph = load('contracts/state-machines.json')
    states = set(graph['task_states'])
    require(all(t['from'] in states and t['to'] in states and t['guard'] for t in graph['transitions']), 'Invalid state edge')
    require(not any(t['from'] in graph['terminal'] for t in graph['transitions']), 'Terminal task must not silently reopen')
    defaults = load('contracts/defaults.json')
    require(defaults['mode'] == 'FIXTURE' and not defaults['live_model_enabled'], 'Default live calls enabled')
    require(defaults['tool_execution'] == 'sequential', 'Default tools are not sequential')
    require(defaults['model']['soft_input_trigger'] < defaults['model']['context_ceiling']
            - defaults['model']['output_reserve'] - defaults['model']['safety_margin'], 'Invalid budget hierarchy')
    require(defaults['pi_commit'] == load('research/upstream-lock.json')['source_commit'], 'Pi source locks differ')
    return {'scenarios': len(scenarios), 'task_states': len(states), 'guarded_transitions': len(graph['transitions'])}


def source_fingerprints():
    source = ROOT / 'pi_src_origin'
    lock = load('research/upstream-lock.json')
    require(source.is_dir(), 'Restore pi_src_origin per research/README.md before validating source')
    commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=source, text=True).strip()
    require(commit == lock['source_commit'], 'Wrong Pi checkout')
    require(not subprocess.check_output(['git', 'status', '--porcelain'], cwd=source, text=True).strip(), 'Pi checkout modified')
    for entry in lock['source_files']:
        require(hashlib.sha256((source / entry['path']).read_bytes()).hexdigest() == entry['sha256'], 'Source hash mismatch: ' + entry['path'])
    for entry in load('research/source-map.json')['entries']:
        lines = (source / entry['path']).read_text().splitlines()
        require(all(entry['symbol_match'] in lines[i-1] for i in entry['lines']), 'Source symbol location changed')
    package_lock = load('research/package-lock.json')['packages']
    for name, spec in lock['packages'].items():
        actual = package_lock['node_modules/' + name]
        require(actual['version'] == spec['version'] and actual.get('integrity') == spec.get('integrity'), 'Package lock mismatch: ' + name)
    return {'source_files': len(lock['source_files']), 'source_commit': commit, 'locked_packages': len(lock['packages'])}


def sql_checks():
    assertions = []
    with tempfile.TemporaryDirectory(prefix='secretary-design-sql-') as tmp:
        path = pathlib.Path(tmp) / 'state.sqlite'
        db = sqlite3.connect(path, isolation_level=None)
        db.executescript((DOCS / 'contracts/schema.sql').read_text())
        require(db.execute('PRAGMA foreign_keys').fetchone()[0] == 1, 'Foreign keys off')
        require(db.execute('PRAGMA journal_mode').fetchone()[0] == 'wal', 'WAL off')
        require(db.execute('PRAGMA synchronous').fetchone()[0] == 2, 'FULL durability off')
        assertions.extend(['foreign_keys', 'wal', 'synchronous_full'])
        db.execute("INSERT INTO system_state VALUES(1,'1.0','session',1,0,0,'FIXTURE','READY')")
        db.execute("INSERT INTO objects VALUES('o',?,1,'text/plain','SYNTHETIC','AVAILABLE','objects/o','2026-09-18T00:00:00Z')", ('a'*64,))
        db.execute("INSERT INTO requests VALUES('r','client',?,'input','o','ACCEPTED','2026-09-18T00:00:00Z',1)", ('b'*64,))
        db.execute("INSERT INTO events(id,kind,source,request_id,payload_ref,occurred_at,received_at) VALUES('e','INPUT','MASTER','r','o','t','t')")
        db.execute("INSERT INTO root_budgets(id,request_id,budget_kind,deadline_at,model_calls_limit,tool_calls_limit,attempts_limit,tokens_limit) VALUES('b','r','REQUEST','t',40,64,3,100)")
        db.execute('BEGIN IMMEDIATE')
        db.execute("INSERT INTO tasks VALUES('t','r',1,'QUEUED',NULL,0,'b','t')")
        db.execute("INSERT INTO task_versions VALUES('t',1,'o',1,'t')")
        db.execute('COMMIT')
        db.execute("INSERT INTO grants VALUES('g',1,'master','t','o','ACTIVE','t','r')")
        db.execute("INSERT INTO attempts VALUES('a','t',1,1,'RUNNING','t','b')")
        db.execute("INSERT INTO operations VALUES('op','t',1,'a','o',?,'g',1,1,0,'READ_ONLY','INTENDED',NULL,NULL)", ('c'*64,))
        db.execute("INSERT INTO permits VALUES('p','op','a',1,0,1,?,'t',NULL,NULL)", ('c'*64,))
        db.execute("INSERT INTO receipts VALUES('rc','op','a',1,?,'o','ACCEPTED','t')", ('d'*64,))
        db.execute("INSERT INTO outbox VALUES('out','REPLY','reply:r','o','PENDING',NULL,'t',NULL,0)")

        def rejected(name, sql, params=()):
            db.execute('SAVEPOINT reject_case')
            failed = False
            try:
                db.execute(sql, params)
            except sqlite3.IntegrityError:
                failed = True
            finally:
                db.execute('ROLLBACK TO reject_case'); db.execute('RELEASE reject_case')
            require(failed, 'SQL unexpectedly accepted ' + name); assertions.append(name)

        rejected('event_immutable', "UPDATE events SET kind='forged'")
        rejected('event_no_delete', "DELETE FROM events")
        rejected('task_version_immutable', "UPDATE task_versions SET definition_ref='o'")
        rejected('receipt_immutable', "UPDATE receipts SET disposition='QUARANTINED'")
        rejected('duplicate_request', "INSERT INTO requests SELECT * FROM requests")
        rejected('missing_body_reference', "UPDATE requests SET body_ref='missing'")
        rejected('budget_no_double_spend', "UPDATE root_budgets SET tokens_used=60,tokens_reserved=50")
        rejected('money_no_double_spend', "UPDATE root_budgets SET money_limit_microunits=100,money_used_microunits=60,money_reserved_microunits=50")
        rejected('unknown_task_state', "UPDATE tasks SET state='DONE'")
        rejected('pause_requires_reason', "UPDATE tasks SET state='PAUSED'")
        rejected('one_active_attempt', "INSERT INTO attempts VALUES('a2','t',1,1,'RUNNING','t','b')")
        rejected('operation_attempt_binding', "INSERT INTO operations SELECT 'op2',task_id,2,originating_attempt_id,intent_ref,arguments_hash,grant_id,grant_revision,owner_epoch,cancel_generation,effect_class,state,target_idempotency_key,idempotency_expires_at FROM operations")
        rejected('idempotent_write_requires_key', "UPDATE operations SET effect_class='IDEMPOTENT_WRITE'")
        rejected('one_live_permit', "INSERT INTO permits SELECT 'p2',operation_id,executing_attempt_id,owner_epoch,cancel_generation,grant_revision,arguments_hash,expires_at,consumed_at,revoked_at FROM permits")
        rejected('receipt_dedup', "INSERT INTO receipts SELECT * FROM receipts")
        rejected('outbox_dedup', "INSERT INTO outbox SELECT 'out2',kind,dedup_key,payload_ref,state,owner_epoch,available_at,claimed_until,attempts FROM outbox")
        rejected('obligation_resolution_basis', "INSERT INTO obligations VALUES('ob',1,'PROMISE','o','GLOBAL',NULL,1,'RESOLVED',NULL,NULL)")
        rejected('memory_exit_not_ahead', "INSERT INTO working_memory VALUES(1,'wm',1,2,'o',0)")
        rejected('reply_task_pair', "INSERT INTO replies VALUES('rp','r',1,'o','t',NULL,'PENDING',1)")
        rejected('available_object_requires_hash', "INSERT INTO objects VALUES('bad',NULL,0,'text/plain','SYNTHETIC','AVAILABLE','bad','t')")

        db.execute('BEGIN IMMEDIATE')
        db.execute("UPDATE tasks SET current_revision=2 WHERE id='t'")
        failed = False
        try:
            db.execute('COMMIT')
        except sqlite3.IntegrityError:
            failed = True; db.execute('ROLLBACK')
        require(failed, 'Missing task head version accepted'); assertions.append('deferred_task_head')
        require(db.execute("SELECT current_revision FROM tasks WHERE id='t'").fetchone()[0] == 1, 'Rollback lost task version')
        assertions.append('transaction_rollback')
        require(db.execute('PRAGMA foreign_key_check').fetchall() == [], 'FK corruption')
        copy = sqlite3.connect(pathlib.Path(tmp) / 'backup.sqlite')
        db.backup(copy)
        require(copy.execute('PRAGMA integrity_check').fetchone()[0] == 'ok', 'Backup corrupt')
        require(copy.execute('SELECT count(*) FROM requests').fetchone()[0] == 1, 'Backup lost request')
        assertions.append('backup_roundtrip')
        tables = [r[0] for r in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")]
        copy.close(); db.close()
        reopened = sqlite3.connect(path)
        require(reopened.execute('SELECT count(*) FROM tasks').fetchone()[0] == 1, 'Reopen lost task')
        reopened.close(); assertions.append('reopen')
    return {'tables': len(tables), 'assertions': len(assertions), 'checks': assertions, 'sqlite_version': sqlite3.sqlite_version}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--report', default=str(DOCS / 'checks/latest-report.json'))
    args = parser.parse_args()
    pathlib.Path(args.report).write_text(json.dumps({'scope': 'DOC_ONLY', 'status': 'RUNNING'})+'\n')
    for name, function in [('contracts', contracts), ('document_links', document_links),
                           ('traceability', traceability), ('source_fingerprints', source_fingerprints), ('sql_constraints', sql_checks)]:
        check(name, function)
    result = {'scope': 'DOC_ONLY', 'generated_at': dt.datetime.now(dt.timezone.utc).isoformat(),
              'status': 'PASS' if all(c['status']=='PASS' for c in checks) else 'FAIL', 'checks': checks,
              'not_verified': ['Secretary application runtime', 'real model semantics', 'complete sandbox policy', 'long-term real use']}
    pathlib.Path(args.report).write_text(json.dumps(result, ensure_ascii=False, indent=2)+'\n')
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if result['status']=='PASS' else 1


if __name__ == '__main__':
    raise SystemExit(main())
