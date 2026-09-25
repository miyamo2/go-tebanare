// Chrome stops an idle service worker and evaluates background.js again on
// the next event. These tests import the entry module twice on one fake
// browser to check that the second worker life starts without an engine,
// creates one on the first request that needs it, and still finds the
// analysis records and tab state kept in session storage.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { analysisKey } from '../../src/shared/cachekey.js';
import { send } from '../../src/shared/messages.js';
import { FakeChrome, installChrome, type FakeExtensionApi } from '../fakes/chrome.js';
import { FAKE_ENGINE_VERSION, FakeEngine, range, result } from './fake-engine.js';

const engines: FakeEngine[] = [];
const createEngine = vi.fn(async (wasm: unknown) => {
  await wasm;
  const e = new FakeEngine();
  e.onAnalyze = () => result([], [range(1, 1)]);
  engines.push(e);
  return e;
});

let restore = () => {};

beforeEach(() => {
  vi.doMock('@go-tebanare/engine', async (importOriginal) => ({ ...(await importOriginal<object>()), createEngine }));
  vi.stubGlobal('fetch', vi.fn(async () => new Response(new Uint8Array(8))));
});

afterEach(() => {
  restore();
  vi.unstubAllGlobals();
  vi.doUnmock('@go-tebanare/engine');
  vi.resetModules();
});

/** startWorker evaluates the entry module in a new service worker context. */
async function startWorker(hub: FakeChrome): Promise<FakeExtensionApi> {
  const api = hub.extensionContext();
  restore();
  restore = installChrome(api);
  vi.resetModules();
  await import('../../src/background/index.js');
  return api;
}

/** stopWorker removes the listeners of a stopped worker, as Chrome does. */
function stopWorker(hub: FakeChrome, api: FakeExtensionApi): void {
  api.runtime.onMessage.listeners.splice(0);
  hub.tabRemoved.listeners.splice(0);
  hub.command.listeners.splice(0);
}

describe('service worker restart', () => {
  it('creates the engine lazily in each worker life', async () => {
    const hub = new FakeChrome();
    const tab = hub.addTab({ url: 'https://github.com/o/r/pull/1/files', active: true });
    const cs = hub.contentScript(tab.id).runtime;
    const k = analysisKey({ engineVersion: FAKE_ENGINE_VERSION, configKey: 'key-10', newSha: 'c'.repeat(40), newPath: 'a.go' });

    const first = await startWorker(hub);
    expect(createEngine).not.toHaveBeenCalled();
    expect(await send({ type: 'compile', yaml: 'version: 1' }, cs)).toMatchObject({ ok: true });
    expect(fetch).toHaveBeenCalledWith(`chrome-extension://${hub.id}/engine.wasm`);
    await send({ type: 'analyze', yaml: 'version: 1', cacheKey: k, change: { newPath: 'a.go', old: null, new: 'package a\n' } }, cs);
    await send({ type: 'set-tab-state', enabled: false, headPreview: true, pr: 'o/r#1' }, cs);
    expect(createEngine).toHaveBeenCalledTimes(1);

    stopWorker(hub, first);
    await startWorker(hub);
    expect(createEngine).toHaveBeenCalledTimes(1);
    expect(await send({ type: 'lookup', cacheKey: k }, cs)).toMatchObject({ ok: true, record: { newLines: { '1': expect.any(String) } } });
    expect(await send({ type: 'get-tab-state', pr: 'o/r#1' }, cs)).toEqual({ ok: true, enabled: false, headPreview: true });
    expect(createEngine).toHaveBeenCalledTimes(1);

    expect(await send({ type: 'compile', yaml: 'version: 1' }, cs)).toMatchObject({ ok: true, engineVersion: FAKE_ENGINE_VERSION });
    expect(createEngine).toHaveBeenCalledTimes(2);
    expect(engines[1]?.calls).toEqual(['compile version: 1']);
  });

  it('imports without the chrome namespace and starts nothing', async () => {
    vi.resetModules();
    expect('chrome' in globalThis).toBe(false);
    await expect(import('../../src/background/index.js')).resolves.toHaveProperty('startBackground');
  });
});
