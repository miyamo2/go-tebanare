// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SendFn } from '../../src/content/controller/file.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';
import { BASE, FakeBackground, PR_KEY, bannerTexts, buildPage, fileHtml, harness, hiddenRows, hide } from './controller-setup.js';

let restore = () => {};
beforeEach(() => {
  restore = installChrome(new FakeChrome({ locale: 'en' }).extensionContext());
});
afterEach(() => restore());

const modified = () => fileHtml('classic-modified.html');
const getter = { old: [], new: [hide(26, 29)] };

describe('failures', () => {
  it('reports a compile request that fails without diagnostics', async () => {
    buildPage(modified());
    const bg = new FakeBackground();
    const send: SendFn = async (req) => (req.type === 'compile' ? ({ ok: false, error: 'no valid response to compile' } as never) : bg.send(req));
    const h = harness({ bg, deps: { send } });
    await h.controller.start();
    await h.settle();
    expect(h.controller.status()).toMatchObject({
      state: 'error',
      messages: ['Nothing is hidden in .gotebanare.yml because the analysis failed: no valid response to compile'],
    });
  });

  it('hides nothing when the config cannot be fetched', async () => {
    buildPage(modified());
    const h = harness();
    h.fetcher.set(BASE, '.gotebanare.yml', { ok: false, reason: 'sso', status: 403 });
    await h.controller.start();
    await h.settle();
    expect(h.controller.status()).toMatchObject({ state: 'error', configPath: '.gotebanare.yml' });
    expect(bannerTexts()).toEqual([
      'Nothing is hidden because the config .gotebanare.yml could not be loaded. Your organization requires single sign-on. Authorize your SSO session and reload the page.',
    ]);
    expect(h.bg.types()).toEqual(['get-tab-state']);
  });

  it('turns an unexpected error into the error state', async () => {
    buildPage(modified());
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    const h = harness({ deps: { contexts: { resolve: () => { throw new Error('boom'); } } } });
    await h.controller.start();
    expect(h.controller.status().state).toBe('error');
    expect(warn).toHaveBeenCalledWith('go-tebanare: boom');
    warn.mockRestore();
  });

  it('hides nothing when the options cannot be read', async () => {
    const [store] = buildPage(modified());
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    // The options may exclude this repository.
    const h = harness({ deps: { loadOptions: () => Promise.reject(new Error('storage')) } });
    h.bg.ranges.set('store/store.go', getter);
    await h.controller.start();
    await h.settle();
    expect(h.controller.status()).toMatchObject({ state: 'error', files: 0, messages: [] });
    expect(warn).toHaveBeenCalledWith('go-tebanare: storage');
    expect(h.fetcher.calls).toEqual([]);
    expect(hiddenRows(store!)).toEqual([]);
    warn.mockRestore();
  });

  it('hides nothing when the tab state cannot be read, unless a tab-state message came', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    const bg = new FakeBackground();
    bg.ranges.set('store/store.go', getter);
    const send: SendFn = async (req) => (req.type === 'get-tab-state' ? ({ ok: false, error: 'no tab' } as never) : bg.send(req));
    let [store] = buildPage(modified());
    let h = harness({ bg, deps: { send } });
    await h.controller.start();
    await h.settle();
    // Hiding may be off for this tab.
    expect(h.controller.status()).toMatchObject({ state: 'error', files: 0 });
    expect(warn).toHaveBeenCalledWith('go-tebanare: get-tab-state: no tab');
    expect(hiddenRows(store!)).toEqual([]);
    // A failed controller stays failed.
    h.controller.setTabState({ type: 'tab-state', enabled: true, headPreview: true, pr: PR_KEY });
    await h.settle();
    expect(h.controller.status().state).toBe('error');
    h.controller.dispose();
    warn.mockRestore();

    [store] = buildPage(modified());
    h = harness({ bg, deps: { send } });
    const started = h.controller.start();
    h.controller.setTabState({ type: 'tab-state', enabled: true, headPreview: false });
    await started;
    await h.settle();
    expect(h.controller.status()).toMatchObject({ state: 'ready', enabled: true, linesHidden: 4 });
    expect(hiddenRows(store!)).toHaveLength(4);
  });

  it('reports a failed analysis and hides nothing in that file', async () => {
    const [store] = buildPage(modified());
    const h = harness();
    h.bg.ranges.set('store/store.go', getter);
    h.bg.analyzeError = 'engine crashed';
    await h.controller.start();
    await h.settle();
    expect(hiddenRows(store!)).toEqual([]);
    expect(h.controller.status()).toMatchObject({
      state: 'ready',
      files: 0,
      messages: ['Nothing is hidden in store/store.go because the analysis failed: engine crashed'],
    });
  });
});

describe('page setup', () => {
  it('warns when both config files exist', async () => {
    const [store] = buildPage(modified());
    const h = harness();
    h.fetcher.set(BASE, '.gotebanare.yaml', 'version: 1\n');
    h.bg.ranges.set('store/store.go', getter);
    await h.controller.start();
    await h.settle();
    expect(bannerTexts()).toEqual(['Both .gotebanare.yml and .gotebanare.yaml exist. Using .gotebanare.yml.']);
    expect(hiddenRows(store!)).toHaveLength(4);
  });

  it('starts with hiding off when the tab has it off', async () => {
    const [store] = buildPage(modified());
    const h = harness();
    h.bg.tab = { enabled: false, headPreview: false };
    h.bg.ranges.set('store/store.go', getter);
    await h.controller.start();
    await h.settle();
    expect(h.controller.status()).toMatchObject({ state: 'ready', enabled: false, files: 1, linesHidden: 0 });
    expect(hiddenRows(store!)).toEqual([]);
  });

  it('reports an unsupported UI and starts when a known diff appears', async () => {
    buildPage(fileHtml('classic-no-diff.html'));
    const h = harness();
    h.bg.ranges.set('store/store.go', getter);
    await h.controller.start();
    await h.settle();
    expect(h.controller.status()).toMatchObject({ state: 'unsupported-ui', messages: ['This GitHub diff layout is not supported yet, so nothing is hidden.'] });
    expect(bannerTexts()).toHaveLength(1);
    expect(h.bg.types()).toEqual(['get-tab-state', 'compile']);

    document.querySelector('.js-diff-progressive-container')!.insertAdjacentHTML('beforeend', modified());
    await h.settle();
    const store = document.querySelector<HTMLElement>('.file:last-child')!;
    expect(h.controller.status()).toMatchObject({ state: 'ready', messages: [], linesHidden: 4 });
    expect(hiddenRows(store)).toHaveLength(4);
    expect(bannerTexts()).toEqual([]);
  });

  it('outlines instead of hiding in debug mode', async () => {
    const [store] = buildPage(modified());
    const h = harness({ options: { debug: true } });
    h.bg.ranges.set('store/store.go', getter);
    await h.controller.start();
    await h.settle();
    expect(hiddenRows(store!)).toEqual([]);
    expect(store!.querySelectorAll('[data-gotebanare-debug]')).toHaveLength(4);
  });
});
