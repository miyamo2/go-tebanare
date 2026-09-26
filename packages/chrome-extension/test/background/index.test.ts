import { describe, expect, it, vi } from 'vitest';
import type { Engine } from '@go-tebanare/engine';
import { messageListener, startBackground } from '../../src/background/index.js';
import { tabKey } from '../../src/background/tabs.js';
import { analysisKey } from '../../src/shared/cachekey.js';
import { lineHash } from '../../src/shared/hash.js';
import { send, type TabMessage } from '../../src/shared/messages.js';
import { asChrome, FakeChrome } from '../fakes/chrome.js';
import { badConfigDiagnostic, FAKE_ENGINE_VERSION, FakeEngine, range, result } from './fake-engine.js';

const SRC = 'package a\n\nfunc (u *User) Name() string { return u.name }\n';
const PR = 'o/r#1';
const key = (path: string) => analysisKey({ engineVersion: FAKE_ENGINE_VERSION, configKey: 'key-10', newSha: 'b'.repeat(40), newPath: path });

function setup(factory: () => Promise<Engine> = async () => new FakeEngine()) {
  const hub = new FakeChrome();
  const tab = hub.addTab({ url: 'https://github.com/o/r/pull/1/files', active: true });
  const sw = hub.extensionContext();
  const bg = startBackground(asChrome(sw), vi.fn(factory));
  const cs = hub.contentScript(tab.id);
  const received: TabMessage[] = [];
  cs.runtime.onMessage.addListener((m) => void received.push(m as TabMessage));
  const popup = hub.extensionContext('popup.html');
  return { hub, tab, bg, cs: cs.runtime, popup: popup.runtime, area: sw.storage.session, received };
}

describe('message listener', () => {
  it('keeps the channel open for requests and ignores other messages', async () => {
    const { bg } = setup();
    const replies: unknown[] = [];
    expect(bg.listener({ type: 'get-tab-state' }, { tab: { id: 1 } }, (r) => replies.push(r))).toBe(true);
    expect(bg.listener({ type: 'status' }, {}, (r) => replies.push(r))).toBe(false);
    expect(bg.listener('compile', {}, (r) => replies.push(r))).toBe(false);
    expect(replies).toEqual([]);
    await vi.waitFor(() => expect(replies).toEqual([{ ok: true, enabled: true, headPreview: false }]));
  });

  it('answers a thrown error with a failure', async () => {
    const engine = new FakeEngine();
    engine.compile = async () => {
      throw new Error('boom');
    };
    const { cs } = setup(async () => engine);
    expect(await send({ type: 'compile', yaml: 'version: 1' }, cs)).toEqual({ ok: false, error: 'boom' });
  });

  it('answers a failed engine load with a failure and loads again on the next request', async () => {
    const engine = new FakeEngine();
    const factory = vi.fn<() => Promise<Engine>>().mockRejectedValueOnce(new Error('loading engine.wasm failed: HTTP 404')).mockResolvedValue(engine);
    const { cs } = setup(factory);
    expect(await send({ type: 'compile', yaml: 'version: 1' }, cs)).toEqual({ ok: false, error: 'loading engine.wasm failed: HTTP 404' });
    expect(await send({ type: 'compile', yaml: 'version: 1' }, cs)).toMatchObject({ ok: true });
  });

  it('answers a synchronous throw with a failure', async () => {
    const replies: unknown[] = [];
    const listener = messageListener(() => {
      throw new TypeError('bad request');
    });
    expect(listener({ type: 'lookup', cacheKey: 'analysis:x' }, {}, (r) => replies.push(r))).toBe(true);
    await vi.waitFor(() => expect(replies).toEqual([{ ok: false, error: 'bad request' }]));
  });

  it('survives a sender that went away', async () => {
    const { bg } = setup();
    const gone = vi.fn(() => {
      throw new Error('Attempting to use a disconnected port object');
    });
    expect(bg.listener({ type: 'get-tab-state' }, {}, gone)).toBe(true);
    await vi.waitFor(() => expect(gone).toHaveBeenCalledTimes(1));
  });

  it('rejects malformed fields', async () => {
    const { cs } = setup();
    const bad = (req: object) => send(req as never, cs);
    expect(await bad({ type: 'compile', yaml: 5 })).toEqual({ ok: false, error: 'yaml must be a string' });
    expect(await bad({ type: 'analyze', yaml: 'v', cacheKey: key('a.go'), change: { old: null, new: null } })).toEqual({
      ok: false,
      error: 'change needs an old or a new side',
    });
    expect(await bad({ type: 'set-tab-state', tabId: 1, enabled: 'yes' })).toEqual({ ok: false, error: 'enabled must be a boolean' });
  });
});

describe('compile', () => {
  it('returns the ruleset summary and engine version', async () => {
    const { cs } = setup();
    expect(await send({ type: 'compile', yaml: 'version: 1' }, cs)).toEqual({
      ok: true,
      configKey: 'key-10',
      diagnostics: [],
      rules: [{ id: 'getter', target: 'func', preset: 'getter' }],
      engineVersion: FAKE_ENGINE_VERSION,
    });
  });

  it('returns ConfigError diagnostics with the failure', async () => {
    const { cs } = setup();
    expect(await send({ type: 'compile', yaml: 'invalid' }, cs)).toEqual({
      ok: false,
      error: badConfigDiagnostic.message,
      diagnostics: [badConfigDiagnostic],
    });
  });
});

describe('lookup and analyze', () => {
  it('misses, analyzes, then hits the cache', async () => {
    const engine = new FakeEngine();
    engine.onAnalyze = () => result([], [range(3, 3)]);
    const { cs } = setup(async () => engine);
    const k = key('a.go');
    expect(await send({ type: 'lookup', cacheKey: k }, cs)).toEqual({ ok: true, record: null });
    const change = { newPath: 'a.go', old: null, new: SRC };
    const analyzed = await send({ type: 'analyze', yaml: 'version: 1', cacheKey: k, change }, cs);
    expect(analyzed).toEqual({
      ok: true,
      record: { result: result([], [range(3, 3)]), oldLines: {}, newLines: { '3': lineHash('func (u *User) Name() string { return u.name }') } },
    });
    expect(await send({ type: 'lookup', cacheKey: k }, cs)).toEqual(analyzed);
    expect(engine.calls.filter((c) => c.startsWith('analyze'))).toEqual(['analyze a.go']);
  });

  it('does not cache engine-error results', async () => {
    const engine = new FakeEngine();
    engine.onAnalyze = () => result([], [], 'engine-error');
    const { cs, area } = setup(async () => engine);
    const k = key('b.go');
    const res = await send({ type: 'analyze', yaml: 'version: 1', cacheKey: k, change: { newPath: 'b.go', old: null, new: SRC } }, cs);
    expect(res).toMatchObject({ ok: true, record: { result: { skipped: 'engine-error' } } });
    expect(await send({ type: 'lookup', cacheKey: k }, cs)).toEqual({ ok: true, record: null });
    expect(area.data.size).toBe(0);
  });

  it('refuses cache keys outside the analysis namespace before analyzing', async () => {
    const engine = new FakeEngine();
    const { cs } = setup(async () => engine);
    const res = await send({ type: 'analyze', yaml: 'version: 1', cacheKey: 'tab:1', change: { old: null, new: SRC } }, cs);
    expect(res).toEqual({ ok: false, error: 'not an analysis cache key: tab:1' });
    expect(await send({ type: 'lookup', cacheKey: 'tab:1' }, cs)).toMatchObject({ ok: false });
    expect(engine.calls).toEqual([]);
  });
});

describe('tab state', () => {
  it('answers get-tab-state for the sender tab and fails without one', async () => {
    const { cs, popup } = setup();
    expect(await send({ type: 'get-tab-state', pr: PR }, cs)).toEqual({ ok: true, enabled: true, headPreview: false });
    expect(await send({ type: 'get-tab-state', pr: PR }, popup)).toEqual({ ok: false, error: 'get-tab-state needs a sender tab' });
  });

  it('applies set-tab-state from the popup and notifies the tab', async () => {
    const { cs, popup, tab, received } = setup();
    expect(await send({ type: 'set-tab-state', tabId: tab.id, headPreview: true, pr: 'O/R#1' }, popup)).toEqual({ ok: true });
    expect(received).toEqual([{ type: 'tab-state', enabled: true, headPreview: true, pr: PR }]);
    expect(await send({ type: 'set-tab-state', enabled: false }, cs)).toEqual({ ok: true });
    expect(await send({ type: 'get-tab-state', pr: PR }, cs)).toEqual({ ok: true, enabled: false, headPreview: true });
    expect(await send({ type: 'set-tab-state' }, popup)).toEqual({ ok: false, error: 'set-tab-state needs tabId or a sender tab' });
  });

  it('rejects a preview without a pull request and malformed pull request keys', async () => {
    const { cs, popup, tab, area } = setup();
    expect(await send({ type: 'set-tab-state', tabId: tab.id, headPreview: true }, popup)).toEqual({ ok: false, error: 'headPreview: true needs pr' });
    const badKey = { ok: false, error: 'pr must be "<owner>/<repo>#<number>"' };
    expect(await send({ type: 'set-tab-state', tabId: tab.id, headPreview: true, pr: 'o/r' }, popup)).toEqual(badKey);
    expect(await send({ type: 'get-tab-state', pr: 7 as never }, cs)).toEqual(badKey);
    expect(area.data.size).toBe(0);
  });

  it('keeps a head config preview to the pull request it was turned on for', async () => {
    const { hub, cs, popup, tab } = setup();
    await send({ type: 'set-tab-state', tabId: tab.id, headPreview: true, pr: PR }, popup);
    expect(await send({ type: 'get-tab-state', pr: PR }, cs)).toEqual({ ok: true, enabled: true, headPreview: true });
    // A reload of the same pull request keeps the preview.
    const reloaded = hub.contentScript(tab.id).runtime;
    expect(await send({ type: 'get-tab-state', pr: PR }, reloaded)).toEqual({ ok: true, enabled: true, headPreview: true });
    // A content script that names no pull request never gets the preview.
    expect(await send({ type: 'get-tab-state' }, reloaded)).toEqual({ ok: true, enabled: true, headPreview: false });
    // Another pull request in the same tab, in any repository, uses the base config and ends the preview.
    hub.tabs.set(tab.id, { ...tab, url: 'https://github.com/x/y/pull/2/files' });
    const next = hub.contentScript(tab.id).runtime;
    expect(await send({ type: 'get-tab-state', pr: 'x/y#2' }, next)).toEqual({ ok: true, enabled: true, headPreview: false });
    const back = hub.contentScript(tab.id).runtime;
    expect(await send({ type: 'get-tab-state', pr: PR }, back)).toEqual({ ok: true, enabled: true, headPreview: false });
  });

  it('toggles the active tab on the toggle-hiding command', async () => {
    const { hub, tab, area, received } = setup();
    hub.runCommand('toggle-hiding');
    await vi.waitFor(() => expect(received).toEqual([{ type: 'tab-state', enabled: false, headPreview: false }]));
    expect(area.data.get(tabKey(tab.id))).toEqual({ enabled: false, headPreviewFor: null });
  });

  it('drops the state when the tab closes', async () => {
    const { hub, tab, popup, area } = setup();
    await send({ type: 'set-tab-state', tabId: tab.id, enabled: false }, popup);
    expect(area.data.has(tabKey(tab.id))).toBe(true);
    hub.removeTab(tab.id);
    await vi.waitFor(() => expect(area.data.has(tabKey(tab.id))).toBe(false));
  });
});
