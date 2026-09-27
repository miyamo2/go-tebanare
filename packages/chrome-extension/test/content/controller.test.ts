// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { FakeChrome, installChrome } from '../fakes/chrome.js';
import { BASE, CONFIG, PR_KEY, addedMarks, bannerTexts, buildPage, fileHtml, harness, hiddenRows, hide } from './controller-setup.js';

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
