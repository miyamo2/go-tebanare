// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { analysisKey } from '../../src/shared/cachekey.js';
import { fnv1a32 } from '../../src/shared/hash.js';
import type { AnalyzeRequest } from '../../src/shared/messages.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';
import { BASE, CONFIG, HEAD, bannerTexts, buildPage, fileHtml, harness, hiddenRows, hide, pageSource } from './controller-setup.js';

let restore = () => {};
beforeEach(() => {
  restore = installChrome(new FakeChrome({ locale: 'en' }).extensionContext());
});
afterEach(() => restore());

const modified = () => fileHtml('classic-modified.html');
const added = () => fileHtml('classic-added.html');
const getter = { old: [], new: [hide(26, 29)] };
const analyzed = (h: ReturnType<typeof harness>) => h.bg.requests.filter((r): r is AnalyzeRequest => r.type === 'analyze');

describe('files', () => {
  it('keeps processing other files when one fetch fails', async () => {
    const [store, mock] = buildPage(modified(), added());
    const h = harness();
    h.bg.ranges.set('store/store.go', getter);
    h.bg.ranges.set('store/mock_store.go', { old: [], new: [hide(1, 15)] });
    h.fetcher.set(HEAD, 'store/mock_store.go', { ok: false, reason: 'sso', status: 403 });
    await h.controller.start();
    await h.settle();
    expect(hiddenRows(store!)).toHaveLength(4);
    expect(hiddenRows(mock!)).toEqual([]);
    const message = 'Nothing is hidden in store/mock_store.go. Your organization requires single sign-on. Authorize your SSO session and reload the page.';
    expect(bannerTexts()).toEqual([message]);
    expect(h.controller.status()).toMatchObject({ state: 'ready', files: 1, filesWithFolds: 1, linesHidden: 4, messages: [message] });
    expect(h.bg.types()).not.toContain('analyze store/mock_store.go');
  });

  it('hides nothing in a file whose source differs from the page', async () => {
    const [store] = buildPage(modified());
    const h = harness();
    h.bg.ranges.set('store/store.go', getter);
    const src = pageSource(HEAD, 'store/store.go')!.replace('func (s *Store) Owner() string {', 'func (s *Store) Owner() any {');
    h.fetcher.set(HEAD, 'store/store.go', src);
    await h.controller.start();
    await h.settle();
    expect(hiddenRows(store!)).toEqual([]);
    expect(store!.querySelector('[data-gotebanare-fold], [data-gotebanare-badge]')).toBeNull();
    expect(h.controller.status()).toMatchObject({
      files: 1,
      linesHidden: 0,
      messages: ['Nothing is hidden in store/store.go because the page does not match the analyzed source.'],
    });
  });

  it('uses a cached analysis without fetching sources', async () => {
    buildPage(modified());
    const first = harness();
    first.bg.ranges.set('store/store.go', getter);
    await first.controller.start();
    await first.settle();
    first.controller.dispose();

    const [store] = buildPage(modified());
    const h = harness({ bg: first.bg });
    first.bg.requests.length = 0;
    await h.controller.start();
    await h.settle();
    expect(h.fetcher.calls).toEqual(['base:.gotebanare.yml', 'base:.gotebanare.yaml']);
    expect(h.bg.types()).toEqual(['get-tab-state', 'compile', 'lookup']);
    expect(hiddenRows(store!)).toHaveLength(4);
  });

  it('fetches one side of added and deleted files', async () => {
    const [mock, legacy] = buildPage(added(), fileHtml('classic-deleted.html'));
    const h = harness();
    h.bg.ranges.set('store/mock_store.go', { old: [], new: [hide(1, 15)] });
    h.bg.ranges.set('store/legacy.go', { old: [hide(3, 6)], new: [] });
    await h.controller.start();
    await h.settle();
    expect(h.fetcher.calls.slice(2).sort()).toEqual(['base:store/legacy.go', 'head:store/mock_store.go']);
    const changes = analyzed(h).map((r) => r.change);
    expect(changes).toContainEqual({ newPath: 'store/mock_store.go', old: null, new: pageSource(HEAD, 'store/mock_store.go') });
    expect(changes).toContainEqual({ oldPath: 'store/legacy.go', old: pageSource(BASE, 'store/legacy.go'), new: null });
    expect(hiddenRows(mock!)).toHaveLength(15);
    expect(hiddenRows(legacy!)).toEqual(['-3', '-4', '-5', '-6']);
  });

  it('reads the old path of a renamed file', async () => {
    const [naming] = buildPage(fileHtml('classic-renamed.html'));
    const h = harness();
    h.bg.ranges.set('store/naming.go', { old: [hide(4, 4)], new: [hide(4, 4)] });
    await h.controller.start();
    await h.settle();
    // docs/a.go to docs/b.go has no diff table, so only one file is read.
    expect(h.fetcher.calls.slice(2)).toEqual(['base:store/names.go', 'head:store/naming.go']);
    const [req] = analyzed(h);
    expect(req?.change).toMatchObject({ oldPath: 'store/names.go', newPath: 'store/naming.go' });
    expect(req?.cacheKey).toBe(
      analysisKey({
        engineVersion: 'v0.0.0-test',
        configKey: `key-${fnv1a32(CONFIG)}`,
        oldSha: BASE,
        oldPath: 'store/names.go',
        newSha: HEAD,
        newPath: 'store/naming.go',
      }),
    );
    expect(hiddenRows(naming!)).toEqual(['-4', '+4']);
  });

  it('never hides a config file that the pull request changes', async () => {
    const config = fileHtml('classic-added.html', ['store/mock_store.go', '.gotebanare.yml']);
    const [store, cfg] = buildPage(modified(), config);
    const h = harness();
    h.bg.ranges.set('store/store.go', getter);
    h.bg.ranges.set('.gotebanare.yml', { old: [], new: [hide(1, 15)] });
    await h.controller.start();
    await h.settle();
    expect(hiddenRows(cfg!)).toEqual([]);
    expect(h.fetcher.calls).not.toContain('head:.gotebanare.yml');
    expect(hiddenRows(store!)).toHaveLength(4);
    expect(bannerTexts()).toEqual(['This pull request changes .gotebanare.yml. Folds follow the base branch config.']);
  });

  it('skips files that are not Go and split diffs', async () => {
    const readme = fileHtml('classic-added.html', ['store/mock_store.go', 'README.md']);
    const [md, split] = buildPage(readme, fileHtml('classic-split.html'));
    const h = harness();
    await h.controller.start();
    await h.settle();
    expect(h.fetcher.calls).toEqual(['base:.gotebanare.yml', 'base:.gotebanare.yaml']);
    expect(hiddenRows(md!)).toEqual([]);
    expect(split!.querySelector('[data-gotebanare-hidden]')).toBeNull();
    expect(bannerTexts()).toEqual(['Split view is not supported yet. Switch to the unified view to fold code.']);
  });

  it('analyzes at most 4 files at once', async () => {
    const paths = ['a', 'b', 'c', 'd', 'e', 'f'].map((n) => `gen/${n}.go`);
    const containers = buildPage(...paths.map((p) => fileHtml('classic-added.html', ['store/mock_store.go', p])));
    const h = harness();
    for (const p of paths) h.bg.ranges.set(p, { old: [], new: [hide(12, 15)] });
    h.fetcher.hold((p) => p.endsWith('.go'));
    await h.controller.start();
    await h.settle();
    expect(h.fetcher.calls.slice(2)).toEqual(paths.slice(0, 4).map((p) => `head:${p}`));
    h.fetcher.release();
    await h.settle();
    expect(h.fetcher.calls.slice(2)).toEqual(paths.map((p) => `head:${p}`));
    expect(containers.map((c) => hiddenRows(c).length)).toEqual([4, 4, 4, 4, 4, 4]);
    expect(h.controller.status()).toMatchObject({ files: 6, filesWithFolds: 6, linesHidden: 24 });
  });

  it('waits for a diff that loads later', async () => {
    buildPage(modified(), fileHtml('classic-no-diff.html'));
    const h = harness();
    h.bg.ranges.set('gen/tables.go', { old: [], new: [hide(12, 15)] });
    await h.controller.start();
    await h.settle();
    expect(h.fetcher.calls.some((c) => c.endsWith('gen/tables.go'))).toBe(false);

    // "Load diff" puts a diff table into the file.
    const tables = document.querySelector<HTMLElement>('.file-header[data-path="gen/tables.go"]')!.closest<HTMLElement>('.file')!;
    const table = new DOMParser().parseFromString(added(), 'text/html').querySelector('.js-file-content')!;
    tables.querySelector('.js-file-content')!.replaceWith(document.importNode(table, true));
    await h.settle();
    expect(h.fetcher.calls.slice(-1)).toEqual(['head:gen/tables.go']);
    expect(hiddenRows(tables)).toEqual(['+12', '+13', '+14', '+15']);
  });
});
