import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterEach, describe, expect, it } from 'vitest';
import { ConfigError, createEngine, EngineError, type Engine, type RecreateReason } from '../src/index.js';
import { yamlSyntaxMessage, type EngineImpl } from '../src/engine.js';
import { haveWasm, loadEngine, wasmPath } from './helpers.js';

const config = 'version: 1\npresets: [getter]\n';
const getter = 'package p\n\ntype U struct{ name string }\n\nfunc (u *U) Name() string { return u.name }\n';

describe.skipIf(!haveWasm)('engine', () => {
  let engine: Engine | undefined;
  afterEach(() => engine?.dispose());

  it('reports its versions and the config file names', async () => {
    engine = await loadEngine();
    expect(engine.apiVersion).toBe(1);
    expect(engine.engineVersion).not.toBe('');
    expect(engine.configFileNames).toEqual(['.gotebanare.yml', '.gotebanare.yaml']);
  });

  it('loads engine.wasm from a URL', async () => {
    const wasm = readFileSync(wasmPath);
    const server = createServer((req, res) => {
      if (req.url === '/engine.wasm') {
        res.writeHead(200, { 'content-type': 'application/wasm' }).end(wasm);
      } else if (req.url === '/octet/engine.wasm') {
        res.writeHead(200, { 'content-type': 'application/octet-stream' }).end(wasm);
      } else {
        res.writeHead(404).end();
      }
    });
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
    try {
      const base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
      for (const src of [new URL('/engine.wasm', base), `${base}/octet/engine.wasm`]) {
        const e = await createEngine(src);
        expect(e.apiVersion).toBe(1);
        e.dispose();
      }
      await expect(createEngine(`${base}/missing.wasm`)).rejects.toThrow('HTTP 404');
    } finally {
      server.close();
    }
  });

  it('compiles and analyzes an added file', async () => {
    engine = await loadEngine();
    const rs = await engine.compile(config);
    expect(rs.key).toMatch(/^[0-9a-f]{64}$/);
    expect(rs.rules).toEqual([
      { id: 'getter', description: expect.any(String), target: 'func', preset: 'getter' },
    ]);
    const res = await engine.analyzeChange(rs, { newPath: 'p.go', new: getter });
    expect(res.skipped).toBe('');
    expect(res.old).toEqual([]);
    expect(res.new.map((r) => [r.start, r.end])).toEqual([[5, 5]]);
    expect(res.new[0]?.hits[0]?.preset).toBe('getter');
  });

  it('returns the cached ruleset for the same YAML', async () => {
    engine = await loadEngine();
    expect(await engine.compile(config)).toBe(await engine.compile(config));
  });

  it('rejects an invalid config with positions', async () => {
    engine = await loadEngine();
    const err = await engine.compile('version: 2\n').catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ConfigError);
    const d = (err as ConfigError).diagnostics[0];
    expect(d?.severity).toBe('error');
    expect(d?.line).toBe(1);
  });

  it('maps a YAML syntax error trap to config-syntax and recovers', async () => {
    const reasons: RecreateReason[] = [];
    engine = await loadEngine({ onRecreate: (r) => reasons.push(r) });
    const err = await engine.compile('version: 1\npresets: [').catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ConfigError);
    expect((err as ConfigError).diagnostics).toEqual([
      { severity: 'error', code: 'config-syntax', message: yamlSyntaxMessage, line: 2 },
    ]);
    expect(reasons).toEqual(['crash']);
    const rs = await engine.compile(config);
    const res = await engine.analyzeChange(rs, { newPath: 'p.go', new: getter });
    expect(res.new).toHaveLength(1);
  });

  it.each([
    ['a tab', 'version: 1\n\tpresets: []\n', 2],
    ['bad indentation', 'version: 1\npresets:\n  - getter: a\n    noop: b\n   bad: 1\n', 5],
    ['an unclosed quote', "version: 1\npresets:\n  - 'getter\n", 3],
    ['an unknown anchor', 'version: 1\npresets: *nope\n', 2],
    ['a second document', 'version: 1\n---\na: b: c\n', 3],
  ])('gives the line where the YAML parser stopped on %s', async (_name, yaml, line) => {
    engine = await loadEngine();
    const err = await engine.compile(yaml).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ConfigError);
    expect((err as ConfigError).diagnostics).toEqual([
      { severity: 'error', code: 'config-syntax', message: yamlSyntaxMessage, line },
    ]);
  });

  it('ignores an exception from onRecreate', async () => {
    let calls = 0;
    engine = await loadEngine({
      maxCallsPerInstance: 2,
      onRecreate: () => {
        calls++;
        throw new Error('callback failed');
      },
    });
    // init() was the first call; compile is the second and drops the instance.
    const rs = await engine.compile(config);
    for (let i = 0; i < 3; i++) {
      const res = await engine.analyzeChange(rs, { newPath: 'p.go', new: getter });
      expect(res.new).toHaveLength(1);
    }
    expect(calls).toBeGreaterThanOrEqual(2);
  });

  it('recreates the instance after a trap and recompiles cached rulesets', async () => {
    const reasons: RecreateReason[] = [];
    engine = await loadEngine({ onRecreate: (r) => reasons.push(r) });
    const rs = await engine.compile(config);
    await expect((engine as EngineImpl).trapForTesting()).rejects.toThrow();
    expect(reasons).toEqual(['crash']);
    const res = await engine.analyzeChange(rs, { newPath: 'p.go', new: getter });
    expect(res.new).toHaveLength(1);
  });

  it('resolves analyzeChange with engine-error when the instance crashes', async () => {
    const reasons: RecreateReason[] = [];
    engine = await loadEngine({ onRecreate: (r) => reasons.push(r) });
    const rs = await engine.compile(config);
    (engine as EngineImpl).faultNextCallForTesting();
    const res = await engine.analyzeChange(rs, { newPath: 'p.go', new: getter });
    expect(res).toEqual({
      old: [],
      new: [],
      diagnostics: [{ severity: 'error', code: 'engine-crashed', message: expect.stringContaining('RuntimeError') }],
      skipped: 'engine-error',
    });
    expect(reasons).toEqual(['crash']);
    const next = await engine.analyzeChange(rs, { newPath: 'p.go', new: getter });
    expect(next.skipped).toBe('');
    expect(next.new).toHaveLength(1);
  });

  it('recreates the instance after maxCallsPerInstance calls', async () => {
    const reasons: RecreateReason[] = [];
    engine = await loadEngine({ maxCallsPerInstance: 3, onRecreate: (r) => reasons.push(r) });
    const rs = await engine.compile(config);
    for (let i = 0; i < 4; i++) {
      const res = await engine.analyzeChange(rs, { newPath: 'p.go', new: getter });
      expect(res.new).toHaveLength(1);
    }
    expect(reasons).toContain('calls');
  });

  it('recreates the instance when memory grows past the limit', async () => {
    const reasons: RecreateReason[] = [];
    engine = await loadEngine({ maxMemoryBytes: 1, onRecreate: (r) => reasons.push(r) });
    const rs = await engine.compile(config);
    await engine.analyzeChange(rs, { newPath: 'p.go', new: getter });
    expect(reasons).toContain('memory');
  });

  it('serializes concurrent calls', async () => {
    engine = await loadEngine();
    const rs = await engine.compile(config);
    const results = await Promise.all(
      Array.from({ length: 20 }, (_, i) =>
        engine!.analyzeChange(rs, { newPath: `p${i}.go`, new: getter }),
      ),
    );
    for (const r of results) expect(r.new).toHaveLength(1);
  });

  it('handles deleted and renamed files', async () => {
    engine = await loadEngine();
    const rs = await engine.compile(config);
    const deleted = await engine.analyzeChange(rs, { oldPath: 'p.go', old: getter });
    expect(deleted.old.map((r) => r.start)).toEqual([5]);
    expect(deleted.new).toEqual([]);
    const renamed = await engine.analyzeChange(rs, {
      oldPath: 'p.go',
      newPath: 'p.txt',
      old: getter,
      new: getter,
    });
    expect(renamed.skipped).toBe('not-target');
  });

  it('lists presets', async () => {
    engine = await loadEngine();
    const names = (await engine.presets()).map((p) => p.name);
    expect(names).toEqual(['getter', 'iferr', 'noop']);
  });

  it('rejects calls after dispose', async () => {
    engine = await loadEngine();
    const rs = await engine.compile(config);
    engine.dispose();
    // The ruleset is cached, and compile still rejects.
    await expect(engine.compile(config)).rejects.toBeInstanceOf(EngineError);
    await expect(engine.compile('version: 1\n')).rejects.toThrow('disposed');
    await expect(engine.analyzeChange(rs, { newPath: 'p.go', new: getter })).rejects.toThrow('disposed');
    await expect(engine.presets()).rejects.toThrow('disposed');
  });
});
