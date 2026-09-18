import test from 'node:test';
import assert from 'node:assert/strict';
import { setTimeout as delay } from 'node:timers/promises';
import { Agent } from '@earendil-works/pi-agent-core';
import { AssistantMessageEventStream } from '@earendil-works/pi-ai/utils/event-stream';
import { stream as responseStream } from '@earendil-works/pi-ai/api/openai-responses';
import { Type } from 'typebox';
import { DatabaseSync, backup } from 'node:sqlite';
import { mkdtemp, readFile, writeFile, rm } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { spawn, spawnSync } from 'node:child_process';
import { once } from 'node:events';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';

const model = {
  id: 'synthetic-probe', name: 'synthetic-probe', api: 'openai-responses',
  provider: 'openai', baseUrl: 'https://invalid.example/v1', reasoning: false,
  input: ['text'], cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
  contextWindow: 32768, maxTokens: 4096,
};
const usage = { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, totalTokens: 0,
  cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 } };
function message(content, stopReason = 'stop') {
  return { role: 'assistant', content, api: model.api, provider: model.provider,
    model: model.id, usage: structuredClone(usage), stopReason, timestamp: Date.now() };
}
function done(msg) {
  const stream = new AssistantMessageEventStream();
  queueMicrotask(() => {
    stream.push(msg.stopReason === 'error' || msg.stopReason === 'aborted'
      ? { type: 'error', reason: msg.stopReason, error: msg }
      : { type: 'done', reason: msg.stopReason, message: msg });
    stream.end();
  });
  return stream;
}
const final = () => done(message([{ type: 'text', text: 'synthetic result' }]));
const toolCall = (id, value = 1) => ({ type: 'toolCall', id, name: 'probe', arguments: { value } });
function scriptedAgent(tool, options = {}) {
  let calls = 0;
  return new Agent({
    initialState: { model, tools: [tool] }, toolExecution: 'sequential',
    streamFn: () => calls++ === 0 ? done(message([toolCall('a'), toolCall('b')], 'toolUse')) : final(),
    ...options,
  });
}
function tool(execute) {
  return { name: 'probe', label: 'Probe', description: 'Synthetic only',
    parameters: Type.Object({ value: Type.Number({ minimum: 1 }) }), execute };
}

test('P01: actual Agent awaits the journal listener before tool execution', async () => {
  let durable = false, effects = 0;
  const agent = scriptedAgent(tool(async () => {
    assert.equal(durable, true); effects++;
    return { content: [{ type: 'text', text: 'recorded' }], details: {} };
  }));
  agent.subscribe(async event => {
    if (event.type === 'message_end' && event.message.role === 'assistant'
      && event.message.stopReason === 'toolUse') {
      await delay(15); durable = true;
    }
  });
  await agent.prompt('synthetic');
  assert.equal(effects, 2);
});

test('P02: explicit sequential tools never overlap', async () => {
  let active = 0, peak = 0;
  const agent = scriptedAgent(tool(async () => {
    active++; peak = Math.max(active, peak); await delay(15); active--;
    return { content: [{ type: 'text', text: 'ok' }], details: {} };
  }));
  await agent.prompt('synthetic');
  assert.equal(peak, 1);
});

test('P03: final execute guard sees mutated args and prevents side effects', async () => {
  let effects = 0, guarded = 0;
  const agent = scriptedAgent(tool(async (_id, args) => {
    // Secretary must revalidate at execute, even after Pi parameter validation.
    if (args.value < 1) { guarded++; throw new Error('EXECUTE_SCHEMA_REJECTED'); }
    effects++;
    return { content: [{ type: 'text', text: 'ok' }], details: {} };
  }), { beforeToolCall: async ({ args }) => { args.value = -1; } });
  await agent.prompt('synthetic');
  assert.equal(guarded, 2); assert.equal(effects, 0);
});

test('P04: blocking hook prevents actual tool execution', async () => {
  let effects = 0;
  const agent = scriptedAgent(tool(async () => { effects++; throw new Error('must not execute'); }), {
    beforeToolCall: async () => ({ block: true, reason: 'REVOKED', terminate: true }),
  });
  await agent.prompt('synthetic');
  assert.equal(effects, 0);
});

test('P05: rejected final Responses payload causes zero fetches', async () => {
  let fetches = 0, checked = false;
  const stream = responseStream(model, { messages: [{ role: 'user', content: 'synthetic', timestamp: 0 }] }, {
    apiKey: 'dummy-not-a-secret', maxRetries: 0,
    onPayload: async payload => { checked = true; assert.ok(payload.input); throw new Error('POLICY_DENIED'); },
    fetch: async () => { fetches++; throw new Error('No real network permitted'); },
  });
  const result = await stream.result();
  assert.equal(checked, true); assert.equal(fetches, 0); assert.equal(result.stopReason, 'error');
});

test('P06: payload guard precedes fetch and maxRetries=0 means one attempt', async () => {
  let fetches = 0, checked = false, serialized;
  const stream = responseStream(model, { messages: [{ role: 'user', content: 'synthetic', timestamp: 0 }] }, {
    apiKey: 'dummy-not-a-secret', maxRetries: 0,
    onPayload: async payload => { checked = true; serialized = JSON.stringify(payload); },
    fetch: async (_url, init) => {
      fetches++; assert.equal(checked, true);
      assert.deepEqual(JSON.parse(String(init.body)), JSON.parse(serialized));
      return new Response(JSON.stringify({ error: { message: 'synthetic transient error' } }), {
        status: 503, headers: { 'content-type': 'application/json' },
      });
    },
  });
  const result = await stream.result();
  assert.equal(fetches, 1); assert.equal(result.stopReason, 'error');
});

test('P07: abort propagates to stream and Agent settles', async () => {
  let begun;
  const started = new Promise(resolve => { begun = resolve; });
  const agent = new Agent({ initialState: { model }, streamFn: (_m, _c, options) => {
    const s = new AssistantMessageEventStream(); begun();
    options.signal.addEventListener('abort', () => {
      const msg = message([], 'aborted'); msg.errorMessage = 'synthetic abort';
      s.push({ type: 'error', reason: 'aborted', error: msg }); s.end();
    }, { once: true });
    return s;
  }});
  const running = agent.prompt('synthetic'); await started; agent.abort();
  await running; await agent.waitForIdle(); assert.equal(agent.state.isStreaming, false);
});

test('P08: rotating Agent uses only explicitly rebuilt history', async () => {
  const old = new Agent({ initialState: { model }, streamFn: final });
  await old.prompt('old synthetic detail'); assert.ok(old.state.messages.length > 0);
  let sent;
  const current = new Agent({ initialState: { model, systemPrompt: 'rebuilt requirements' },
    streamFn: (_m, context) => { sent = context; return final(); } });
  await current.prompt('current');
  assert.equal(JSON.stringify(sent).includes('old synthetic detail'), false);
  assert.equal(JSON.stringify(sent).includes('rebuilt requirements'), true);
});

test('P09: actual Node SQLite loads baseline, persists and backs up', async () => {
  const dir = await mkdtemp(path.join(os.tmpdir(), 'secretary-schema-probe-'));
  let db, restored;
  try {
    db = new DatabaseSync(path.join(dir, 'source.sqlite'));
    db.exec(await readFile(new URL('../contracts/schema.sql', import.meta.url), 'utf8'));
    db.prepare('INSERT INTO system_state VALUES(1,?,?,?,?,?,?,?)').run('1.0', 'probe', 1, 0, 0, 'FIXTURE', 'READY');
    await backup(db, path.join(dir, 'backup.sqlite')); db.close(); db = undefined;
    restored = new DatabaseSync(path.join(dir, 'backup.sqlite'));
    assert.equal(restored.prepare('SELECT owner_epoch FROM system_state').get().owner_epoch, 1);
    assert.equal(restored.prepare('PRAGMA integrity_check').get().integrity_check, 'ok');
    assert.deepEqual(restored.prepare('PRAGMA foreign_key_check').all(), []);
  } finally { db?.close(); restored?.close(); await rm(dir, { recursive: true, force: true }); }
});

test('P10: production-candidate Ajv validates every declared fixture', async () => {
  const ajv = new Ajv2020({ allErrors: true, strict: true, strictTypes: false }); addFormats(ajv);
  const schema = JSON.parse(await readFile(new URL('../contracts/contracts.schema.json', import.meta.url), 'utf8'));
  const validate = ajv.compile(schema);
  const manifest = JSON.parse(await readFile(new URL('../examples/manifest.json', import.meta.url), 'utf8'));
  for (const entry of manifest.examples) {
    const data = JSON.parse(await readFile(new URL(`../examples/${entry.file}`, import.meta.url), 'utf8'));
    assert.equal(validate(data), entry.valid, entry.file + JSON.stringify(validate.errors));
  }
  const toolSchema = JSON.parse(await readFile(new URL('../contracts/tools.schema.json', import.meta.url), 'utf8'));
  const validateTool = ajv.compile(toolSchema);
  const cases = JSON.parse(await readFile(new URL('../examples/tool-cases.json', import.meta.url), 'utf8'));
  for (const c of cases.cases) assert.equal(validateTool(c.payload), c.valid, c.case + JSON.stringify(validateTool.errors));
  assert.equal(validateTool({ tool: 'file.read', arguments: { path: 'synthetic.txt', offset: 0, max_bytes: 100 } }), true);
  assert.equal(validateTool({ tool: 'file.read', arguments: { path: 'synthetic.txt', offset: 0, max_bytes: 100, approved: true } }), false);
});

test('P11: native OS lock rejects a second owner and is released on termination', async () => {
  const dir = await mkdtemp(path.join(os.tmpdir(), 'secretary-lock-probe-'));
  let owner;
  try {
    const bin = path.join(dir, 'state-lock'); const lock = path.join(dir, 'owner.lock');
    const compiled = spawnSync('/usr/bin/cc', ['-Wall', '-Wextra', '-Werror',
      new URL('./state-lock.c', import.meta.url).pathname, '-o', bin], { encoding: 'utf8' });
    assert.equal(compiled.status, 0, compiled.stderr);
    owner = spawn(bin, [lock, process.execPath, '-e', 'process.stdout.write("ready\\n");setInterval(()=>{},1000)'], {
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    const ready = await Promise.race([once(owner.stdout, 'data'), delay(3000).then(() => { throw new Error('owner not ready'); })]);
    assert.match(String(ready[0]), /ready/);
    assert.equal(spawnSync(bin, [lock, '/usr/bin/true']).status, 75);
    const exited = once(owner, 'exit'); owner.kill('SIGKILL'); await exited; owner = undefined;
    assert.equal(spawnSync(bin, [lock, '/usr/bin/true']).status, 0);
  } finally { owner?.kill('SIGKILL'); await rm(dir, { recursive: true, force: true }); }
});

test('P12: macOS sandbox primitive allows one synthetic file and denies another', {
  skip: process.platform !== 'darwin',
}, async () => {
  const dir = await mkdtemp(path.join(os.tmpdir(), 'secretary-sandbox-probe-'));
  try {
    const allowed = path.join(dir, 'allowed.txt'), denied = path.join(dir, 'denied.txt');
    await writeFile(allowed, 'allowed synthetic'); await writeFile(denied, 'denied synthetic');
    const profile = `(version 1)(deny default)(allow process-exec process-fork signal)
      (allow sysctl-read)(allow mach-lookup)(allow file-map-executable)(allow file-read-metadata)
      (allow file-read-data (literal "/") (subpath "/System") (subpath "/usr") (subpath "/bin")
        (subpath "/dev") (literal "${allowed}"))`;
    const ok = spawnSync('/usr/bin/sandbox-exec', ['-p', profile, '/bin/cat', allowed], { encoding: 'utf8' });
    assert.equal(ok.status, 0, `signal=${ok.signal}; ${ok.stderr}`); assert.equal(ok.stdout, 'allowed synthetic');
    const blocked = spawnSync('/usr/bin/sandbox-exec', ['-p', profile, '/bin/cat', denied], { encoding: 'utf8' });
    assert.equal(blocked.signal, null); assert.equal(blocked.status, 1);
    assert.match(blocked.stderr, /Operation not permitted/);
    assert.equal(blocked.stdout.includes('denied synthetic'), false);
  } finally { await rm(dir, { recursive: true, force: true }); }
});
