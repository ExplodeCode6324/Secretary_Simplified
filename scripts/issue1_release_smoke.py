#!/usr/bin/env python3
"""Short isolated release-binary smoke; no API, audio or duration acceptance."""
import argparse
import datetime
import hashlib
import json
import pathlib
import sqlite3
import subprocess
import tempfile
import time
import uuid


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--report', required=True)
    args = parser.parse_args()
    repo = pathlib.Path(__file__).resolve().parents[1]
    out = pathlib.Path(args.report).resolve()
    out.mkdir(parents=True, exist_ok=False)
    data = pathlib.Path(tempfile.mkdtemp(prefix='ss-i1-', dir='/tmp'))
    bins = repo / 'release'
    processes, handles = [], []
    started = time.monotonic()
    report = {'suite': 'issue1-short-release-smoke', 'status': 'RUNNING',
              'profile': 'fixture', 'real_data': False, 'external_api_calls': False,
              'build_sha256': {n: hashlib.sha256((bins/n).read_bytes()).hexdigest()
                               for n in ['secretary', 'secretaryd']}}

    def check(condition, code):
        if not condition:
            raise AssertionError(code)

    def cli(*words):
        run = subprocess.run([str(bins/'secretary'), *words, '--config', str(data/'config.json'),
                              '--json'], capture_output=True, text=True, timeout=15)
        check(run.returncode == 0, 'CLI_FAILED')
        body = json.loads(run.stdout)
        return body if words[0] == 'doctor' else body['result']

    try:
        subprocess.run([str(bins/'secretary'), 'init', '--data-dir', str(data), '--json'],
                       capture_output=True, check=True, timeout=15)
        config = json.loads((data/'config.json').read_text())
        check(config['model_profile']['profile'] == 'fixture', 'FIXTURE_REQUIRED')
        for role in ['core', 'runner']:
            handle = (data/(role+'.log')).open('w')
            handles.append(handle)
            processes.append(subprocess.Popen([str(bins/'secretaryd'), role, '--config',
                                               str(data/'config.json')], stdout=handle, stderr=handle))
        deadline = time.monotonic()+15
        while not all((data/'run'/(role+'.sock')).exists() for role in ['core', 'runner']):
            check(all(p.poll() is None for p in processes), 'DAEMON_EXITED')
            check(time.monotonic() < deadline, 'STARTUP_TIMEOUT')
            time.sleep(.05)
        request = str(uuid.uuid4())
        words = ('items', 'create', '--title', 'invented-issue1-smoke', '--domain', 'project',
                 '--request-id', request, '--data-class', 'SYNTHETIC')
        cli(*words)
        cli(*words)
        action = json.loads((bins/'examples/reminder.json').read_text())
        action['payload']['schedule']['at'] = (datetime.datetime.now(datetime.timezone.utc)
                                              + datetime.timedelta(seconds=1)).isoformat(timespec='milliseconds').replace('+00:00', 'Z')
        path = data/'reminder.json'
        path.write_text(json.dumps(action))
        cli('actions', '--file', str(path), '--data-class', 'SYNTHETIC')
        deadline = time.monotonic()+30
        while True:
            notes = cli('notifications')['items']
            if notes:
                break
            check(time.monotonic() < deadline, 'NOTIFICATION_TIMEOUT')
            time.sleep(.1)
        check(len(notes) == 1, 'DUPLICATE_NOTIFICATION')
        with sqlite3.connect(data/'state/secretary.sqlite') as db:
            count = db.execute('SELECT count(*) FROM item').fetchone()[0]
            turns = db.execute('SELECT count(*) FROM input_turn WHERE request_id=?', (request,)).fetchone()[0]
            states = [json.loads(r[0])['status'] for r in db.execute('SELECT payload_json FROM item')]
        check(count == 1 and turns == 1 and states == ['OPEN'], 'IDEMPOTENCY_OR_ITEM_STATE')
        doctor = cli('doctor')
        check(doctor['database']['integrity'] == 'ok', 'INTEGRITY')
        check(doctor['database']['foreign_key_violations'] == 0, 'FOREIGN_KEYS')
        check(all(doctor['heartbeats'][role]['state'] == 'RECENT' for role in ['core', 'runner']), 'HEARTBEAT')
        processes[0].terminate()
        processes[0].wait(timeout=10)
        cli('notifications', 'ack', notes[0]['id'])
        report.update(status='PASS', item_count=count, idempotent_request_turns=turns,
                      notification_count=len(notes), ack_with_core_offline=True,
                      doctor=doctor)
    except Exception as error:
        report.update(status='FAIL', error_type=type(error).__name__, error=str(error) if isinstance(error, AssertionError) else 'execution failure; inspect private fixture logs')
    finally:
        for process in processes:
            if process.poll() is None:
                process.terminate()
        for process in processes:
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=5)
        for handle in handles:
            handle.close()
        report['owned_processes_stopped'] = all(p.poll() is not None for p in processes)
        report['elapsed_seconds'] = time.monotonic()-started
        report['completed_at'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
        (out/'report.json').write_text(json.dumps(report, indent=2)+'\n')
        # This location is private by the required reports/local parent.
        private = repo/'reports/local/issue1-smoke-data-paths.json'
        previous = json.loads(private.read_text()) if private.exists() else []
        previous.append(str(data))
        private.write_text(json.dumps(previous))
        print(json.dumps({k: report.get(k) for k in ['status', 'error', 'elapsed_seconds', 'owned_processes_stopped']}))
    return 0 if report['status'] == 'PASS' else 1


if __name__ == '__main__':
    raise SystemExit(main())
