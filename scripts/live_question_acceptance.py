#!/usr/bin/env python3
"""D11 real CLI acceptance. Explicit invocation runs real calls; no fallback provider.

Authored oracle is frozen before calls. Generated IDs are relational bindings, not
model-derived expected answers. All output stays under reports/local by default.
"""
import argparse
import datetime as dt
import hashlib
import http.client
import json
import pathlib
import shutil
import socket
import sqlite3
import subprocess
import tempfile
import time
import uuid
from urllib.parse import urlparse

TITLE = 'synthetic-question-project-date'
QUESTION = TITLE + ' 的截止日期是什么？'
ANSWER = '2026-10-01T09:00:00Z'


def digest(value):
    return hashlib.sha256(json.dumps(value, ensure_ascii=False, sort_keys=True,
                                     separators=(',', ':')).encode()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', default='/tmp/sec-month5-20260914/config.json')
    parser.add_argument('--report', default='reports/local/live-question-run1')
    parser.add_argument('--binaries', default='release')
    parser.add_argument('--fixture', help='Reuse a prior authored fixture verbatim for a fresh isolated run')
    args = parser.parse_args()
    binary = pathlib.Path(args.binaries).resolve()
    out = pathlib.Path(args.report).resolve()
    out.mkdir(parents=True, exist_ok=False)
    data = pathlib.Path(tempfile.mkdtemp(prefix='ss-liveq-', dir='/tmp'))
    processes, handles, transcript = [], [], []
    report = {'suite': 'D11-real-CLI-question-lifecycle', 'status': 'RUNNING',
              'real_data': False, 'provider_fallback': False,
              'build_sha256': {n: hashlib.sha256((binary/n).read_bytes()).hexdigest()
                               for n in ('secretary', 'secretaryd')}}

    def save(name, value):
        (out/name).write_text(json.dumps(value, ensure_ascii=False, indent=2)+'\n')

    def cli(*words, failure=False):
        result = subprocess.run([str(binary/'secretary'), *words, '--data-class', 'SYNTHETIC', '--config',
                                 str(data/'config.json'), '--json'],
                                capture_output=True, text=True, timeout=30)
        try:
            body = json.loads(result.stdout)
        except ValueError:
            body = {'unparsed_stdout': result.stdout}
        transcript.append({'args': list(words), 'returncode': result.returncode,
                           'response': body, 'stderr': result.stderr})
        save('cli-transcript.json', transcript)
        if not failure and result.returncode:
            raise AssertionError('CLI failed: '+str(body.get('error')))
        return body, result.returncode

    def result(*words):
        return cli(*words)[0]['result']

    def rows(table):
        with sqlite3.connect(data/'state/secretary.sqlite') as db:
            return [json.loads(r[0]) for r in db.execute('SELECT payload_json FROM '+table)]

    def state(session):
        return next(v for v in rows('conversation_session') if v['id'] == session)

    def quota():
        for f in (data/'reports/model_calls').glob('*.json'):
            try:
                value = json.loads(f.read_text())
            except ValueError:
                continue
            if value.get('error_code') == 'MODEL_HTTP_429':
                print('MODEL_HTTP_429: stop; notify coordinator immediately', flush=True)
                raise RuntimeError('MODEL_HTTP_429')

    def await_turn(turn_id):
        deadline = time.monotonic()+240
        while time.monotonic() < deadline:
            quota()
            turn = result('turns', turn_id)
            if turn['state'] in ('COMMITTED', 'FAILED'):
                assert turn['state'] == 'COMMITTED', turn
                return turn
            time.sleep(.4)
        raise AssertionError('turn timeout')

    def start():
        log = open(data/('core-'+str(len(processes))+'.log'), 'w')
        handles.append(log)
        proc = subprocess.Popen([str(binary/'secretaryd'), 'core', '--config',
                                 str(data/'config.json')], stdout=log, stderr=log)
        processes.append(proc)
        deadline = time.monotonic()+20
        while time.monotonic() < deadline:
            assert proc.poll() is None, 'Core exited at startup'
            if (data/'run/core.sock').exists():
                try:
                    with socket.socket(socket.AF_UNIX) as sock:
                        sock.connect(str(data/'run/core.sock'))
                    return proc
                except OSError:
                    pass
            time.sleep(.1)
        raise AssertionError('Core startup timeout')

    def unchanged_item(original):
        assert rows('item') == [original], 'question flow mutated Item'

    session, other = str(uuid.uuid4()), str(uuid.uuid4())
    requests = {k: str(uuid.uuid4()) for k in ('create', 'retrieve', 'answer', 'resolved')}
    fixture = {
        'schema_version': 1, 'kind': 'AUTHORED_BEFORE_CALL', 'title': TITLE,
        'question_text': QUESTION, 'answer_value': ANSWER,
        'session_id': session, 'retrieval_session_id': other, 'requests': requests,
        'create_template': '现有 Item {item_id}（'+TITLE+'）缺少截止日期。请仅通过 reply.questions 登记一个关联这个 exact Item ID 的问题，text 严格为“'+QUESTION+'”。actions 和 controls 必须为空；不要修改事项、猜测日期或创建新事项。',
        'retrieval_text': '请实际调用 READ_MEMORY，query 使用 synthetic-question-project-date，entity_ids 为空。取回之前真实登记的截止日期问题；从原始 ASSISTANT 程序问题块恢复 question_id 与原 session_id，并核对关联 Item。最终 reply.text 只返回 JSON 对象，字段严格为 question_id、session_id、item_id、text，值是原始问题记录，不要推测 ID。不创建问题，不修改任何事项。',
        'answer_text': '对显式指定问题的回答为 '+ANSWER+'。此值仅用于确认该问题已回答，不授权更新 Item 日期、状态或任何其他字段，不安排任务；actions 和 controls 为空，也不提出新问题。',
        'oracle': {'one_question': True, 'exact_question_text': QUESTION,
                   'question_item_id': 'typed_precondition_item.id',
                   'created_sequence': 'actual ASSISTANT event sequence for creation turn',
                   'retrieval': 'all four fields equal original program-created question bindings',
                   'actual_read_memory_required': True, 'restart_before_answer': True,
                   'resolved_after_answer': True, 'item_unchanged': True,
                   'same_request_no_new_turn_event_or_model_call': True,
                   'new_request_resolved_http_status': 409,
                   'new_request_resolved_error': 'QUESTION_ALREADY_RESOLVED'}}
    if args.fixture:
        fixture = json.loads(pathlib.Path(args.fixture).read_text())
        session, other = fixture['session_id'], fixture['retrieval_session_id']
        requests = fixture['requests']
    save('fixture.json', fixture)
    save('manifest.json', {'fixture_sha256': digest(fixture),
                           'created_before_model_calls': True, 'expected_is_relational': True})
    try:
        subprocess.run([str(binary/'secretary'), 'init', '--data-dir', str(data), '--json'],
                       stdout=subprocess.DEVNULL, check=True)
        source = pathlib.Path(args.config).resolve()
        template = json.loads(source.read_text())
        conf = json.loads((data/'config.json').read_text())
        profile = dict(template['model_profile'])
        # This run is only authorized to inherit Go. No automatic official fallback.
        assert urlparse(profile['endpoint']).hostname == 'opencode.ai', 'official provider forbidden for this run'
        secret = pathlib.Path(profile['secret_ref'])
        if not secret.is_absolute():
            profile['secret_ref'] = str((source.parent/secret).resolve())
        conf['model_profile'] = profile
        conf['provider_policy'] = {'allowed_data_classes': ['SYNTHETIC'], 'source_ids': []}
        (data/'config.json').write_text(json.dumps(conf))
        report['provider'] = {k: profile[k] for k in ('profile', 'model', 'endpoint')}
        proc = start()
        result('items', 'create', '--title', TITLE, '--domain', 'project')
        items = rows('item')
        assert len(items) == 1 and items[0]['status'] == 'OPEN' and items[0]['due_at'] is None
        original = items[0]
        create_text = fixture['create_template'].replace('{item_id}', original['id'])
        save('precall-bindings.json', {'item': original, 'create_text': create_text,
                                       'fixture_sha256': digest(fixture)})
        accepted = result('input', '--session', session, '--request-id', requests['create'], '--text', create_text)
        created = await_turn(accepted['turn_id'])
        questions = state(session)['pending_questions']
        assert len(questions) == 1
        question = questions[0]
        assert question['text'] == QUESTION and question['item_id'] == original['id'] and not question['resolved']
        events = rows('conversation_event')
        assistant = [e for e in events if e['turn_id'] == created['id'] and e['role'] == 'ASSISTANT']
        assert len(assistant) == 1 and question['created_sequence'] == assistant[0]['sequence']
        assert not created['committed_operation_keys']
        unchanged_item(original)
        report['question_created'] = question
        expected = {'question_id': question['id'], 'session_id': session,
                    'item_id': original['id'], 'text': QUESTION}
        save('program-identity-bindings.json', expected)
        accepted = result('input', '--session', other, '--request-id', requests['retrieve'], '--text', fixture['retrieval_text'])
        retrieved = await_turn(accepted['turn_id'])
        reply_text = retrieved['reply']['text']
        try:
            recovered = json.loads(reply_text)
        except ValueError as error:
            raise AssertionError('retrieval returned non-JSON product reply: '+reply_text) from error
        assert recovered == expected, 'retrieval identity/original text mismatch'
        attempts = [json.loads(p.read_text()) for p in (data/'reports/decision_attempts').glob('*.json')]
        read_records = [v for v in attempts if v['intent_id'] == retrieved['intent_id'] and v['status'] == 'READ_MEMORY']
        assert read_records, 'no actual READ_MEMORY call'
        assert not retrieved['committed_operation_keys'] and not state(other)['pending_questions']
        report['read_memory_records'] = read_records
        unchanged_item(original)
        proc.terminate()
        proc.wait(timeout=15)
        proc = start()
        assert not state(session)['pending_questions'][0]['resolved']
        words = ('input', '--session', session, '--request-id', requests['answer'],
                 '--answer-to', question['id'], '--text', fixture['answer_text'])
        answered = await_turn(result(*words)['turn_id'])
        resolved = state(session)['pending_questions'][0]
        assert resolved == dict(question, resolved=True)
        assert answered['reply']['answered_question_id'] == question['id']
        assert not answered['committed_operation_keys']
        unchanged_item(original)
        def snapshot():
            return {'turns': rows('input_turn'), 'events': rows('conversation_event'),
                    'states': rows('conversation_session'),
                    'calls': sorted(p.name for p in (data/'reports/model_calls').glob('*.json'))}
        before = snapshot()
        assert result(*words)['turn_id'] == answered['id']
        assert result('turns', answered['id']) == answered
        assert snapshot() == before, 'duplicate request changed durable state or called model'
        denied, code = cli('input', '--session', session, '--request-id', requests['resolved'],
                           '--answer-to', question['id'], '--text', fixture['answer_text'], failure=True)
        assert code != 0 and denied['error']['code'] == 'QUESTION_ALREADY_RESOLVED'
        # Supplement the normal CLI rejection with its exact wire HTTP status.
        connection = http.client.HTTPConnection('localhost', timeout=10)
        connection.sock = socket.socket(socket.AF_UNIX)
        connection.sock.connect(str(data/'run/core.sock'))
        envelope = {'schema_version': 1, 'request_id': requests['resolved'],
                    'session_id': session, 'principal_id': 'master', 'origin': 'MASTER_CLI',
                    'text': fixture['answer_text'], 'answer_to_question_id': question['id'],
                    'received_at': dt.datetime.now(dt.timezone.utc).isoformat(timespec='milliseconds').replace('+00:00', 'Z'),
                    'data_class': 'SYNTHETIC', 'attachment_refs': [], 'extensions': {}}
        token = (data/'run/client.token').read_text().strip()
        connection.request('POST', '/v1/inputs', json.dumps(envelope),
                           {'Authorization': 'Bearer '+token, 'Content-Type': 'application/json'})
        response = connection.getresponse()
        wire = json.loads(response.read())
        assert response.status == 409 and wire['error']['code'] == 'QUESTION_ALREADY_RESOLVED'
        connection.close()
        report['resolved_rejection'] = {'http_status': response.status, 'response': wire}
        report['status'] = 'PASS'
    except Exception as error:
        report['status'] = 'BLOCKED_429' if 'MODEL_HTTP_429' in str(error) else 'FAIL'
        report['error'] = str(error)
    finally:
        for proc in processes:
            if proc.poll() is None:
                proc.terminate()
        for proc in processes:
            try:
                proc.wait(timeout=15)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=5)
        for handle in handles:
            handle.close()
        try:
            tables = ('item', 'input_turn', 'conversation_session', 'conversation_event',
                      'decision_record', 'context_manifest')
            save('durable-state.json', {table: rows(table) for table in tables})
            shutil.copytree(data/'reports', out/'diagnostics')
        except Exception as error:
            report['evidence_error'] = str(error)
            if report['status'] == 'PASS':
                report['status'] = 'FAIL'
        # Immutable objects retain exact model request/response bytes in private data.
        (out/'private-data-path.txt').write_text(str(data)+'\n')
        report['completed_at'] = dt.datetime.now(dt.timezone.utc).isoformat()
        save('report.json', report)
        print(json.dumps({'status': report['status'], 'error': report.get('error'),
                          'report': str(out)}, ensure_ascii=False), flush=True)
    return 0 if report['status'] == 'PASS' else 75 if report['status'] == 'BLOCKED_429' else 1


if __name__ == '__main__':
    raise SystemExit(main())
