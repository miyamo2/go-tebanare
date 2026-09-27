// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { classicVariant as classic } from '../../src/content/dom/classic.js';
import type { DiffUiVariant } from '../../src/content/dom/variant.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';
import { HEAD, PR_KEY, addedMarks, bannerTexts, buildPage, fileHtml, harness, hiddenRows, hide } from './controller-setup.js';

let restore = () => {};
beforeEach(() => {
  restore = installChrome(new FakeChrome({ locale: 'en' }).extensionContext());
});
afterEach(() => restore());

const modified = () => fileHtml('classic-modified.html');
const getter = { old: [], new: [hide(26, 29)] };
const HEAD_CONFIG = 'version: 1\npresets: [getter, iferr]\n';
const previewBanner = 'Previewing with the head branch config .gotebanare.yml. The pull request author controls this config.';

async function ready() {
  const [store] = buildPage(modified());
  const h = harness();
  h.bg.ranges.set('store/store.go', getter);
  await h.controller.start();
  await h.settle();
  return { h, store: store! };
}

describe('tab state', () => {
  it('clears every file when hiding turns off and applies again when it turns on', async () => {
    const { h, store } = await ready();
    const requests = h.bg.requests.length;
    h.controller.setTabState({ type: 'tab-state', enabled: false, headPreview: false });
    expect(addedMarks()).toBe(0);
    await h.settle();
    expect(addedMarks()).toBe(0);
    expect(h.controller.status()).toMatchObject({ enabled: false, files: 1, filesWithFolds: 0, linesHidden: 0 });

    h.controller.setTabState({ type: 'tab-state', enabled: true, headPreview: false });
    await h.settle();
    expect(hiddenRows(store)).toEqual(['+26', '+27', '+28', '+29']);
    expect(store.querySelectorAll('[data-gotebanare-fold]')).toHaveLength(1);
    expect(h.controller.status()).toMatchObject({ enabled: true, linesHidden: 4 });
    // The analysis is reused.
    expect(h.bg.requests.length).toBe(requests);
    expect(h.fetcher.calls.filter((c) => c.endsWith('store.go'))).toHaveLength(2);
  });

  it('switches to the head config for a preview and back', async () => {
    const r = await ready();
    const { h } = r;
    let { store } = r;
    h.fetcher.set(HEAD, '.gotebanare.yml', HEAD_CONFIG);
    let before = h.fetcher.calls.length;
    h.controller.setTabState({ type: 'tab-state', enabled: true, headPreview: true, pr: PR_KEY });
    await h.settle();
    // Another config key means another analysis of the file.
    expect(h.fetcher.calls.slice(before)).toEqual(['head:.gotebanare.yml', 'head:.gotebanare.yaml', 'base:store/store.go', 'head:store/store.go']);
    expect(h.bg.requests.filter((r) => r.type === 'compile').map((r) => r.type === 'compile' && r.yaml)).toContain(HEAD_CONFIG);
    expect(h.controller.status()).toMatchObject({ state: 'ready', configSource: 'head', headPreview: true, messages: [previewBanner] });
    expect(bannerTexts()).toEqual([previewBanner]);
    expect(h.bg.types().filter((t) => t === 'analyze store/store.go')).toHaveLength(2);
    expect(hiddenRows(store)).toHaveLength(4);

    // The banner stays while the preview is on, also when GitHub renders the files again.
    document.querySelector('#files')!.innerHTML = `<div class="js-diff-progressive-container">${modified()}</div>`;
    expect(bannerTexts()).toEqual([]);
    await h.settle();
    expect(bannerTexts()).toEqual([previewBanner]);
    store = document.querySelector<HTMLElement>('#files .file')!;
    expect(hiddenRows(store)).toHaveLength(4);

    // A preview for another pull request does not apply here. The base
    // analysis is still in the background cache, so no source is fetched.
    before = h.fetcher.calls.length;
    h.controller.setTabState({ type: 'tab-state', enabled: true, headPreview: true, pr: 'octo-org/octo-repo#8' });
    await h.settle();
    expect(h.controller.status()).toMatchObject({ configSource: 'base', headPreview: false, messages: [] });
    expect(bannerTexts()).toEqual([]);
    expect(h.fetcher.calls.slice(before)).toEqual(['base:.gotebanare.yml', 'base:.gotebanare.yaml']);
    expect(hiddenRows(store)).toHaveLength(4);
  });

  it('starts with the head config when the tab has the preview on', async () => {
    buildPage(modified());
    const h = harness();
    h.bg.tab = { enabled: true, headPreview: true };
    await h.controller.start();
    await h.settle();
    expect(h.fetcher.calls.slice(0, 2)).toEqual(['head:.gotebanare.yml', 'head:.gotebanare.yaml']);
    expect(h.controller.status()).toMatchObject({ configSource: 'head', headPreview: true });
    expect(bannerTexts()).toEqual([previewBanner]);
  });

  it('keeps a tab-state message that arrives before the tab state answer', async () => {
    buildPage(modified());
    const h = harness();
    h.bg.tab = { enabled: true, headPreview: false };
    const started = h.controller.start();
    h.controller.setTabState({ type: 'tab-state', enabled: false, headPreview: true, pr: PR_KEY });
    await started;
    await h.settle();
    expect(h.controller.status()).toMatchObject({ enabled: false, headPreview: true, configSource: 'head' });
  });
});

describe('re-rendering', () => {
  it('applies again to a re-rendered file without a new analysis', async () => {
    const { h, store } = await ready();
    const requests = h.bg.requests.length;
    const html = store.outerHTML;
    store.outerHTML = fileHtml('classic-modified.html');
    await h.settle();
    const [again] = document.querySelectorAll<HTMLElement>('#files .file');
    expect(again).not.toBe(store);
    expect(hiddenRows(again!)).toEqual(['+26', '+27', '+28', '+29']);
    expect(again!.querySelectorAll('[data-gotebanare-fold]')).toHaveLength(1);
    expect(again!.querySelectorAll('[data-gotebanare-badge]')).toHaveLength(1);
    expect(again!.outerHTML).toBe(html);
    expect(h.bg.requests.length).toBe(requests);
  });

  it('keeps folds the reader opened, and hides them again from the same row', async () => {
    const { h, store } = await ready();
    store.querySelector<HTMLElement>('.gotebanare-fold-toggle')!.click();
    expect(hiddenRows(store)).toEqual([]);
    expect(h.controller.status()).toMatchObject({ filesWithFolds: 0, linesHidden: 0 });
    store.outerHTML = fileHtml('classic-modified.html');
    await h.settle();
    const again = document.querySelector<HTMLElement>('#files .file')!;
    expect(hiddenRows(again)).toEqual([]);
    expect(again.querySelectorAll('[data-gotebanare-fold][data-gotebanare-open]')).toHaveLength(1);
    again.querySelector<HTMLElement>('.gotebanare-fold-toggle')!.click();
    expect(hiddenRows(again)).toEqual(['+26', '+27', '+28', '+29']);
    expect(h.controller.status()).toMatchObject({ filesWithFolds: 1, linesHidden: 4 });
  });

  it('reads again only the files a change touched', async () => {
    buildPage(modified(), fileHtml('classic-added.html'));
    const read: string[] = [];
    const variant: DiffUiVariant = {
      ...classic,
      rows: (c) => {
        read.push(classic.filePath(c).path);
        return classic.rows(c);
      },
    };
    const h = harness({ deps: { detectVariant: (doc) => (classic.detect(doc) ? variant : null) } });
    await h.controller.start();
    await h.settle();
    expect(read).toEqual(['store/store.go', 'store/mock_store.go']);
    read.length = 0;
    // A hovercard somewhere else on the page.
    document.body.append(document.createElement('div'));
    await h.settle();
    expect(read).toEqual([]);
    document.querySelector('.file-header[data-path="store/mock_store.go"]')!.append(document.createElement('span'));
    await h.settle();
    expect(read).toEqual(['store/mock_store.go']);
  });

  it('checks a row again when its code changes in place', async () => {
    const { h, store } = await ready();
    store.querySelector('[data-gotebanare-hidden] .blob-code-inner')!.textContent = 'os.RemoveAll("/")';
    await h.settle();
    const mismatch = 'Nothing is hidden in store/store.go because the page does not match the analyzed source.';
    expect(hiddenRows(store)).toEqual([]);
    expect(bannerTexts()).toEqual([mismatch]);

    // The same for a text node that changes without a child list change.
    store.outerHTML = modified();
    await h.settle();
    const again = document.querySelector<HTMLElement>('#files .file')!;
    expect(hiddenRows(again)).toHaveLength(4);
    const text = [...again.querySelectorAll('[data-gotebanare-hidden] .blob-code-inner')]
      .flatMap((el) => [...el.childNodes])
      .find((n): n is Text => n.nodeType === 3 && (n as Text).data.trim() !== '')!;
    text.data = 'os.Exit(1)';
    await h.settle();
    expect(hiddenRows(again)).toEqual([]);
    expect(bannerTexts()).toEqual([mismatch]);
  });

  it('settles after applying', async () => {
    const { h } = await ready();
    const frames = h.frames.requested;
    await h.settle();
    expect(h.frames.requested).toBe(frames);
    expect(h.frames.queue.size).toBe(0);
  });
});

describe('dispose', () => {
  it('removes everything and ignores results that arrive later', async () => {
    const [store] = buildPage(modified());
    const h = harness();
    h.bg.ranges.set('store/store.go', getter);
    h.fetcher.hold((p) => p.endsWith('.go'));
    await h.controller.start();
    await h.settle();
    expect(h.bg.types()).toEqual(['get-tab-state', 'compile', 'lookup']);
    h.controller.dispose();
    h.fetcher.release();
    await h.settle();
    expect(h.bg.types()).toEqual(['get-tab-state', 'compile', 'lookup']);
    expect(hiddenRows(store!)).toEqual([]);
    expect(addedMarks()).toBe(0);
    // A disposed controller ignores tab state.
    h.controller.setTabState({ type: 'tab-state', enabled: true, headPreview: true, pr: PR_KEY });
    await h.settle();
    expect(h.fetcher.calls.filter((c) => c.startsWith('head:.gotebanare'))).toEqual([]);
  });

  it('removes the folds and the banner it added', async () => {
    const { h, store } = await ready();
    h.fetcher.set(HEAD, '.gotebanare.yml', HEAD_CONFIG);
    h.controller.setTabState({ type: 'tab-state', enabled: true, headPreview: true, pr: PR_KEY });
    await h.settle();
    expect(hiddenRows(store)).toHaveLength(4);
    expect(bannerTexts()).toHaveLength(1);
    h.controller.dispose();
    expect(addedMarks()).toBe(0);
    store.outerHTML = fileHtml('classic-modified.html');
    await h.settle();
    expect(addedMarks()).toBe(0);
  });
});
