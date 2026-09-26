import { describe, expect, it, vi } from 'vitest';
import type { Engine } from '@go-tebanare/engine';
import { EngineHost, wasmEngineFactory } from '../../src/background/engine-host.js';
import { FAKE_ENGINE_VERSION, FakeEngine } from './fake-engine.js';

describe('EngineHost', () => {
  it('creates the engine on first use and only once', async () => {
    const engine = new FakeEngine();
    const factory = vi.fn(async (): Promise<Engine> => engine);
    const host = new EngineHost(factory);
    expect(factory).not.toHaveBeenCalled();
    const [a, b] = await Promise.all([host.compile('version: 1'), host.compile('version: 1')]);
    expect(a.key).toBe(b.key);
    expect(await host.engineVersion()).toBe(FAKE_ENGINE_VERSION);
    expect(factory).toHaveBeenCalledTimes(1);
  });

  it('forwards analyzeChange', async () => {
    const engine = new FakeEngine();
    const host = new EngineHost(async () => engine);
    const rs = await host.compile('version: 1');
    await host.analyzeChange(rs, { newPath: 'a.go', old: null, new: 'package a\n' });
    expect(engine.calls).toEqual(['compile version: 1', 'analyze a.go']);
  });

  it('tries again after a failed creation', async () => {
    const engine = new FakeEngine();
    const factory = vi
      .fn<() => Promise<Engine>>()
      .mockRejectedValueOnce(new Error('loading engine.wasm failed: HTTP 404'))
      .mockResolvedValue(engine);
    const host = new EngineHost(factory);
    await expect(host.compile('version: 1')).rejects.toThrow('HTTP 404');
    await expect(host.compile('version: 1')).resolves.toMatchObject({ yaml: 'version: 1' });
    expect(factory).toHaveBeenCalledTimes(2);
  });

  it('turns a factory that throws synchronously into a rejection', async () => {
    const host = new EngineHost(() => {
      throw new Error('no wasm');
    });
    await expect(host.engine()).rejects.toThrow('no wasm');
  });
});

describe('wasmEngineFactory', () => {
  it('fetches engine.wasm from the extension and passes the response to createEngine', async () => {
    const engine = new FakeEngine();
    const response = new Response(new Uint8Array([0, 97, 115, 109]));
    const fetchFn = vi.fn(async () => response);
    const create = vi.fn(async (src: unknown) => {
      expect(await src).toBe(response);
      return engine;
    });
    const factory = wasmEngineFactory((p) => `chrome-extension://abc/${p}`, fetchFn, create);
    expect(await factory()).toBe(engine);
    expect(fetchFn).toHaveBeenCalledWith('chrome-extension://abc/engine.wasm');
  });
});
