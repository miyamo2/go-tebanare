// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { PullRequestContext } from '../../src/content/context.js';
import type { PullPage } from '../../src/content/page.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';
import { BASE, CONFIG, HEAD, PAGE, PR_KEY, addedMarks, bannerTexts, buildPage, fileHtml, harness, hiddenRows, hide } from './controller-setup.js';

let restore = () => {};
beforeEach(() => {
  restore = installChrome(new FakeChrome({ locale: 'en' }).extensionContext());
});
afterEach(() => restore());

const modified = () => fileHtml('classic-modified.html');
const getter = { old: [], new: [hide(26, 29)] };

describe('pipeline', () => {
  it('hides the rows the analysis names and reports them', async () => {
    const [store] = buildPage(modified());
    const h = harness();
    h.bg.ranges.set('store/store.go', getter);
    await h.controller.start();
    await h.settle();
    expect(hiddenRows(store!)).toEqual(['+26', '+27', '+28', '+29']);
    expect(store!.querySelectorAll('[data-gotebanare-fold]')).toHaveLength(1);
    expect(store!.querySelector('[data-gotebanare-badge]')?.getAttribute('aria-label')).toBe('Show the 4 lines that gotebanare hid in this file');
    expect(h.fetcher.calls).toEqual(['base:.gotebanare.yml', 'base:.gotebanare.yaml', 'base:store/store.go', 'head:store/store.go']);
    expect(h.bg.types()).toEqual(['get-tab-state', 'compile', 'lookup', 'analyze store/store.go']);
    expect(h.bg.requests[0]).toEqual({ type: 'get-tab-state', pr: PR_KEY });
    expect(h.bg.requests[1]).toEqual({ type: 'compile', yaml: CONFIG });
    expect(h.controller.status()).toEqual({
      state: 'ready',
      repo: 'octo-org/octo-repo',
      pr: 7,
      configPath: '.gotebanare.yml',
      configSource: 'base',
      rules: 1,
      files: 1,
      filesWithFolds: 1,
      linesHidden: 4,
      messages: [],
      enabled: true,
      headPreview: false,
    });
    expect(bannerTexts()).toEqual([]);
  });

  it('does nothing in an excluded repository', async () => {
    const [store] = buildPage(modified());
    const h = harness({ options: { excludedRepos: ['Octo-Org/*'] } });
    h.bg.ranges.set('store/store.go', getter);
    await h.controller.start();
    await h.settle();
    expect(h.controller.status()).toMatchObject({ state: 'excluded', messages: [], rules: 0 });
    expect(h.fetcher.calls).toEqual([]);
    expect(h.bg.types()).toEqual(['get-tab-state']);
    expect(hiddenRows(store!)).toEqual([]);
    expect(addedMarks()).toBe(0);
  });

  it('does nothing when the repository has no config', async () => {
    buildPage(modified());
    const h = harness();
    h.fetcher.files.clear();
    await h.controller.start();
    await h.settle();
    expect(h.controller.status()).toMatchObject({ state: 'no-config', messages: [] });
    expect(h.controller.status().configPath).toBeUndefined();
    expect(h.fetcher.calls).toEqual(['base:.gotebanare.yml', 'base:.gotebanare.yaml']);
    expect(h.bg.types()).toEqual(['get-tab-state']);
    expect(addedMarks()).toBe(0);
  });

  it('hides nothing and shows the diagnostics for an invalid config', async () => {
    const [store] = buildPage(modified());
    const h = harness();
    h.fetcher.set(BASE, '.gotebanare.yml', 'invalid: true\n');
    h.bg.ranges.set('store/store.go', getter);
    await h.controller.start();
    await h.settle();
    const messages = ['.gotebanare.yml has errors, so nothing is hidden.', 'line 3, field presets[0](gettr): unknown preset "gettr"'];
    expect(h.controller.status()).toMatchObject({ state: 'config-error', configPath: '.gotebanare.yml', messages });
    expect(bannerTexts()).toEqual(messages);
    // The banner sits right before the first file.
    expect(document.querySelector('[data-gotebanare-banner]')?.nextElementSibling).toBe(store);
    expect(h.bg.types()).toEqual(['get-tab-state', 'compile']);
    expect(hiddenRows(store!)).toEqual([]);
  });

  it('hides nothing when the page does not name the commits', async () => {
    buildPage(modified());
    for (const input of document.querySelectorAll('input[type=hidden]')) input.remove();
    const h = harness();
    await h.controller.start();
    await h.settle();
    expect(h.controller.status()).toMatchObject({ state: 'error', messages: ['Could not read the pull request commits from this page, so nothing is hidden.'] });
    expect(h.fetcher.calls).toEqual([]);
  });

  it("reads the commits from the server's copy of a page that does not name them", async () => {
    const [store] = buildPage(modified());
    for (const input of document.querySelectorAll('input[type=hidden]')) input.remove();
    const fetchContext = vi.fn(async (p: PullPage) => ({ ...p, baseSha: BASE, headSha: HEAD }));
    const h = harness({ deps: { fetchContext } });
    h.bg.ranges.set('store/store.go', getter);
    await h.controller.start();
    await h.settle();
    expect(fetchContext).toHaveBeenCalledWith(PAGE);
    expect(hiddenRows(store!)).toEqual(['+26', '+27', '+28', '+29']);
    expect(h.controller.status()).toMatchObject({ state: 'ready', linesHidden: 4 });
    // Later DOM changes still name no commits; the run keeps the fetched ones.
    const types = h.bg.types().length;
    document.body.append(document.createElement('div'));
    await h.settle();
    expect(fetchContext).toHaveBeenCalledTimes(1);
    expect(h.bg.types()).toHaveLength(types);
    expect(hiddenRows(store!)).toEqual(['+26', '+27', '+28', '+29']);
  });

  it('hides nothing when the server copy does not name the commits either', async () => {
    buildPage(modified());
    for (const input of document.querySelectorAll('input[type=hidden]')) input.remove();
    const h = harness({ deps: { fetchContext: async () => null } });
    await h.controller.start();
    await h.settle();
    expect(h.controller.status()).toMatchObject({ state: 'error', messages: ['Could not read the pull request commits from this page, so nothing is hidden.'] });
    expect(h.fetcher.calls).toEqual([]);
  });

  it('starts no run when disposed while the page is fetched', async () => {
    buildPage(modified());
    for (const input of document.querySelectorAll('input[type=hidden]')) input.remove();
    let answer: (ctx: null) => void = () => {};
    const h = harness({ deps: { fetchContext: () => new Promise((resolve) => (answer = resolve)) } });
    const started = h.controller.start();
    await h.settle();
    h.controller.dispose();
    answer(null);
    await started;
    await h.settle();
    expect(bannerTexts()).toEqual([]);
    expect(addedMarks()).toBe(0);
  });

  it('does not fetch the page when the DOM names the commits', async () => {
    buildPage(modified());
    const fetchContext = vi.fn(async () => null);
    const h = harness({ deps: { fetchContext } });
    await h.controller.start();
    await h.settle();
    expect(fetchContext).not.toHaveBeenCalled();
    expect(h.controller.status()).toMatchObject({ state: 'ready' });
  });

  it('answers loading until the config is compiled', async () => {
    buildPage(modified());
    const h = harness();
    h.fetcher.hold();
    void h.controller.start();
    await h.settle();
    expect(h.controller.status()).toMatchObject({ state: 'loading', rules: 0, files: 0 });
    h.fetcher.release();
    await h.settle();
    expect(h.controller.status().state).toBe('ready');
  });
});

describe('loading indicator', () => {
  const loading = () => document.querySelector('[data-gotebanare-banner] .gotebanare-banner-loading') !== null;
  // The tests below check what the indicator shows, not when; the delay has its own tests.
  const now = { loadingDelayMs: 0 };

  it('shows while the commits are fetched and until every file has its result', async () => {
    const [store] = buildPage(modified());
    for (const input of document.querySelectorAll('input[type=hidden]')) input.remove();
    let answer: (ctx: PullRequestContext) => void = () => {};
    const h = harness({ deps: { ...now, fetchContext: () => new Promise((resolve) => (answer = resolve)) } });
    h.bg.ranges.set('store/store.go', getter);
    h.fetcher.hold((p) => p === 'store/store.go');
    const started = h.controller.start();
    await h.settle();
    expect(loading()).toBe(true);
    answer({ ...PAGE, baseSha: BASE, headSha: HEAD });
    await started;
    await h.settle();
    // The config is compiled, but store.go waits for its sources.
    expect(h.controller.status().state).toBe('ready');
    expect(loading()).toBe(true);
    h.fetcher.release();
    await h.settle();
    expect(hiddenRows(store!)).toEqual(['+26', '+27', '+28', '+29']);
    expect(loading()).toBe(false);
    expect(document.querySelector('[data-gotebanare-banner]')).toBeNull();
  });

  it('keeps the messages when loading ends in an error', async () => {
    buildPage(modified());
    for (const input of document.querySelectorAll('input[type=hidden]')) input.remove();
    const h = harness({ deps: { ...now, fetchContext: async () => null } });
    await h.controller.start();
    await h.settle();
    expect(loading()).toBe(false);
    expect(bannerTexts()).toEqual(['Could not read the pull request commits from this page, so nothing is hidden.']);
  });

  it('never shows in an excluded repository', async () => {
    buildPage(modified());
    const h = harness({ options: { excludedRepos: ['octo-org/*'] }, deps: now });
    await h.controller.start();
    await h.settle();
    expect(addedMarks()).toBe(0);
  });

  it('hides at once while hiding is off, and waits the delay again when it comes back on', async () => {
    buildPage(modified());
    const h = harness({ deps: now });
    h.fetcher.hold((p) => p === 'store/store.go');
    await h.controller.start();
    await h.settle();
    expect(loading()).toBe(true);
    h.controller.setTabState({ type: 'tab-state', enabled: false, headPreview: false });
    expect(loading()).toBe(false);
    h.controller.setTabState({ type: 'tab-state', enabled: true, headPreview: false });
    expect(loading()).toBe(false);
    await h.settle();
    expect(loading()).toBe(true);
    h.fetcher.release();
    await h.settle();
    expect(loading()).toBe(false);
  });
});

describe('loading delay', () => {
  const banner = () => document.querySelector('[data-gotebanare-banner]');
  const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

  it('never shows the indicator when the lines are known within the delay', async () => {
    const [store] = buildPage(modified());
    const h = harness();
    h.bg.ranges.set('store/store.go', getter);
    const seen: boolean[] = [];
    const observer = new MutationObserver(() => seen.push(banner() !== null));
    observer.observe(document.body, { childList: true, subtree: true });
    await h.controller.start();
    await h.settle();
    observer.disconnect();
    expect(hiddenRows(store!)).toHaveLength(4);
    expect(seen).not.toContain(true);
    expect(banner()).toBeNull();
  });

  it('shows the indicator once loading lasts the delay, across the context fetch and the run', async () => {
    buildPage(modified());
    for (const input of document.querySelectorAll('input[type=hidden]')) input.remove();
    let answer: (ctx: PullRequestContext) => void = () => {};
    const h = harness({ deps: { loadingDelayMs: 200, fetchContext: () => new Promise((resolve) => (answer = resolve)) } });
    h.bg.ranges.set('store/store.go', getter);
    h.fetcher.hold((p) => p === 'store/store.go');
    const started = h.controller.start();
    await wait(20);
    answer({ ...PAGE, baseSha: BASE, headSha: HEAD });
    await started;
    await h.settle();
    // 20 ms in the fetch plus the run so far: the delay has not passed.
    expect(banner()).toBeNull();
    await wait(250);
    expect(banner()?.querySelector('.gotebanare-banner-loading')).not.toBeNull();
    h.fetcher.release();
    await h.settle();
    expect(banner()).toBeNull();
  });

  it('cancels the timer on dispose', async () => {
    buildPage(modified());
    const h = harness({ deps: { loadingDelayMs: 20 } });
    h.fetcher.hold((p) => p === 'store/store.go');
    await h.controller.start();
    h.controller.dispose();
    await wait(40);
    expect(banner()).toBeNull();
  });
});
