import { afterEach, describe, expect, it } from 'vitest';
import {
  errorText,
  isBgRequest,
  isTabMessage,
  send,
  sendToTab,
  toFailure,
  type BgRequest,
  type PageStatus,
} from '../../src/shared/messages.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';

const status: PageStatus = {
  state: 'ready',
  repo: 'octo/repo',
  pr: 7,
  configSource: 'base',
  rules: 2,
  files: 3,
  filesWithFolds: 1,
  linesHidden: 12,
  messages: [],
  enabled: true,
  headPreview: false,
};

let restore = () => {};
afterEach(() => restore());

describe('type guards', () => {
  it('recognize request and tab message types', () => {
    expect(isBgRequest({ type: 'compile', yaml: '' })).toBe(true);
    expect(isBgRequest({ type: 'status' })).toBe(false);
    expect(isBgRequest(null)).toBe(false);
    expect(isBgRequest('compile')).toBe(false);
    expect(isTabMessage({ type: 'status' })).toBe(true);
    expect(isTabMessage({ type: 'lookup' })).toBe(false);
  });
});

describe('errorText and toFailure', () => {
  it('describe thrown values', () => {
    expect(errorText(new Error('boom'))).toBe('boom');
    expect(errorText(new TypeError(''))).toBe('TypeError');
    expect(errorText(42)).toBe('42');
    expect(toFailure(new Error('x'))).toEqual({ ok: false, error: 'x' });
  });
});

describe('send', () => {
  it('returns the background answer', async () => {
    const hub = new FakeChrome();
    const bg = hub.extensionContext();
    const seen: BgRequest[] = [];
    bg.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
      seen.push(msg as BgRequest);
      setTimeout(() => sendResponse({ ok: true, record: null }), 0);
      return true;
    });
    restore = installChrome(hub.contentScript(hub.addTab({ url: 'https://github.com/o/r/pull/1/files' }).id));
    const res = await send({ type: 'lookup', cacheKey: 'k' });
    expect(res).toEqual({ ok: true, record: null });
    expect(seen).toEqual([{ type: 'lookup', cacheKey: 'k' }]);
  });

  it('turns a missing receiver into a Failure', async () => {
    const hub = new FakeChrome();
    const res = await send({ type: 'get-tab-state' }, hub.contentScript(1).runtime);
    expect(res.ok).toBe(false);
    expect(!res.ok && res.error).toMatch(/Receiving end does not exist/);
  });

  it('turns an unanswered request into a Failure', async () => {
    const hub = new FakeChrome();
    hub.extensionContext().runtime.onMessage.addListener(() => undefined);
    const res = await send({ type: 'compile', yaml: 'version: 1' }, hub.contentScript(1).runtime);
    expect(res).toEqual({ ok: false, error: 'no valid response to compile' });
  });

  it('resolves with a Failure when chrome is not defined', async () => {
    expect('chrome' in globalThis).toBe(false);
    await expect(send({ type: 'get-tab-state' })).resolves.toEqual({ ok: false, error: 'chrome.runtime unavailable' });
  });

  /** answer returns what send() makes of reply to req. */
  async function answer(req: BgRequest, reply: unknown) {
    const hub = new FakeChrome();
    hub.extensionContext().runtime.onMessage.addListener((_msg, _sender, sendResponse) => void sendResponse(reply));
    return send(req, hub.contentScript(1).runtime);
  }

  const record = { result: { path: 'a.go' }, oldLines: {}, newLines: { '3': '0123abcd' } };

  it.each<[string, BgRequest, unknown]>([
    ['lookup without record', { type: 'lookup', cacheKey: 'k' }, { ok: true }],
    ['lookup with a partial record', { type: 'lookup', cacheKey: 'k' }, { ok: true, record: { result: {}, oldLines: {} } }],
    ['analyze with a null record', { type: 'analyze', yaml: '', cacheKey: 'k', change: { old: null, new: '' } }, { ok: true, record: null }],
    ['compile without rules', { type: 'compile', yaml: '' }, { ok: true, configKey: 'c', diagnostics: [], engineVersion: 'v' }],
    ['get-tab-state without flags', { type: 'get-tab-state' }, { ok: true, enabled: true }],
    ['a failure without error', { type: 'set-tab-state', enabled: false }, { ok: false }],
    ['a failure with malformed diagnostics', { type: 'compile', yaml: '' }, { ok: false, error: 'x', diagnostics: 'bad' }],
  ])('rejects %s', async (_name, req, reply) => {
    expect(await answer(req, reply)).toEqual({ ok: false, error: `no valid response to ${req.type}` });
  });

  it.each<[string, BgRequest, unknown]>([
    ['lookup with a record', { type: 'lookup', cacheKey: 'k' }, { ok: true, record }],
    ['analyze with a record', { type: 'analyze', yaml: '', cacheKey: 'k', change: { old: null, new: '' } }, { ok: true, record }],
    ['compile', { type: 'compile', yaml: '' }, { ok: true, configKey: 'c', diagnostics: [], rules: [], engineVersion: 'v' }],
    ['a compile failure with diagnostics', { type: 'compile', yaml: '' }, { ok: false, error: 'invalid', diagnostics: [] }],
    ['set-tab-state', { type: 'set-tab-state', enabled: false }, { ok: true }],
  ])('passes %s through', async (_name, req, reply) => {
    expect(await answer(req, reply)).toEqual(reply);
  });
});

describe('sendToTab', () => {
  it('returns the content script answer', async () => {
    const hub = new FakeChrome();
    const tab = hub.addTab({ active: true });
    hub.contentScript(tab.id).runtime.onMessage.addListener((msg, _sender, sendResponse) => {
      if (isTabMessage(msg) && msg.type === 'status') sendResponse(status);
    });
    const popup = hub.extensionContext('popup.html');
    expect(await sendToTab(tab.id, { type: 'status' }, popup.tabs)).toEqual(status);
  });

  it('returns undefined when the tab has no content script', async () => {
    const hub = new FakeChrome();
    const popup = hub.extensionContext('popup.html');
    expect(await sendToTab(9, { type: 'status' }, popup.tabs)).toBeUndefined();
  });

  it('returns undefined without chrome.tabs', async () => {
    await expect(sendToTab(1, { type: 'status' })).resolves.toBeUndefined();
    const hub = new FakeChrome();
    restore = installChrome(hub.contentScript(hub.addTab().id));
    await expect(sendToTab(1, { type: 'status' })).resolves.toBeUndefined();
  });
});
