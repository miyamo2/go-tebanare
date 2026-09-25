// @vitest-environment happy-dom
// The page changing under a running controller: other commits without a
// navigation, and marks that an earlier controller left in the page.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { FakeChrome, installChrome } from '../fakes/chrome.js';
import { BASE, CONFIG, HEAD, addedMarks, bannerTexts, buildPage, fileHtml, harness, hiddenRows, hide, pageSource, type HarnessOptions } from './controller-setup.js';

let restore = () => {};
beforeEach(() => {
  restore = installChrome(new FakeChrome({ locale: 'en' }).extensionContext());
});
afterEach(() => restore());

const modified = () => fileHtml('classic-modified.html');
const getter = { old: [], new: [hide(26, 29)] };
const A = 'a'.repeat(40);
const B = 'b'.repeat(40);
const contextError = 'Could not read the pull request commits from this page, so nothing is hidden.';

/** setCommits replaces the hidden inputs of the page with ones naming base and head, or removes them. */
function setCommits(commits: [base: string, head: string] | null): void {
  for (const input of document.querySelectorAll('input[type=hidden]')) input.remove();
  if (!commits) return;
  const [base, head] = commits;
  document.body.insertAdjacentHTML(
    'afterbegin',
    `<input type="hidden" name="comparison_start_oid" value="${base}"><input type="hidden" name="comparison_end_oid" value="${head}">`,
  );
}

describe('commits that change without a navigation', () => {
  it('starts over with the commits the page names now', async () => {
    const [store] = buildPage(modified());
    const h = harness();
    h.bg.ranges.set('store/store.go', getter);
    await h.controller.start();
    await h.settle();
    expect(hiddenRows(store!)).toHaveLength(4);

    // At the new head the getter gained logic, so the analysis hides nothing.
    h.bg.ranges.set('store/store.go', { old: [], new: [] });
    h.fetcher.set(A, '.gotebanare.yml', CONFIG);
    h.fetcher.set(A, 'store/store.go', pageSource(BASE, 'store/store.go')!);
    h.fetcher.set(B, 'store/store.go', pageSource(BASE, 'store/store.go')!);
    const before = h.fetcher.calls.length;
    setCommits([A, B]);
    await h.settle();
    expect(h.fetcher.calls.slice(before)).toEqual([`${A}:.gotebanare.yml`, `${A}:.gotebanare.yaml`, `${A}:store/store.go`, `${B}:store/store.go`]);
    expect(hiddenRows(store!)).toEqual([]);
    expect(h.controller.status()).toMatchObject({ state: 'ready', files: 1, linesHidden: 0 });
  });

  it('follows a page that Turbo replaced without turbo:load', async () => {
    buildPage();
    const h = harness();
    await h.controller.start();
    await h.settle();
    expect(h.controller.status().state).toBe('unsupported-ui');
    document.body.innerHTML = `<main><div id="files"><div class="js-diff-progressive-container">${modified()}</div></div></main>`;
    setCommits([A, B]);
    await h.settle();
    // No config at the new base commit.
    expect(h.fetcher.calls.slice(-2)).toEqual([`${A}:.gotebanare.yml`, `${A}:.gotebanare.yaml`]);
    expect(h.fetcher.calls.filter((c) => c.endsWith('store.go'))).toEqual([]);
    expect(h.controller.status()).toMatchObject({ state: 'no-config', messages: [] });
    expect(addedMarks()).toBe(0);
  });

  it('hides nothing while the page names no commits', async () => {
    const [store] = buildPage(modified());
    setCommits(null);
    const h = harness();
    h.bg.ranges.set('store/store.go', getter);
    await h.controller.start();
    await h.settle();
    expect(h.controller.status()).toMatchObject({ state: 'error', messages: [contextError] });

    setCommits([BASE, HEAD]);
    await h.settle();
    expect(h.controller.status()).toMatchObject({ state: 'ready', messages: [], linesHidden: 4 });
    expect(hiddenRows(store!)).toHaveLength(4);

    setCommits(null);
    await h.settle();
    expect(h.controller.status()).toMatchObject({ state: 'error', messages: [contextError], linesHidden: 0 });
    expect(hiddenRows(store!)).toEqual([]);
    expect(bannerTexts()).toEqual([contextError]);
  });
});

describe('marks left in the page', () => {
  it.each<[string, HarnessOptions]>([
    ['an excluded repository', { options: { excludedRepos: ['octo-org/*'] } }],
    ['options that cannot be read', { deps: { loadOptions: () => Promise.reject(new Error('storage')) } }],
  ])('are removed at start, also for %s', async (_, opts) => {
    buildPage(modified());
    const first = harness();
    first.fetcher.set(BASE, '.gotebanare.yaml', CONFIG);
    first.bg.ranges.set('store/store.go', getter);
    await first.controller.start();
    await first.settle();
    expect(bannerTexts()).toHaveLength(1);
    // A copy of the page as Turbo or the back/forward cache keeps it.
    const saved = document.body.innerHTML;
    first.controller.dispose();
    document.body.innerHTML = saved;
    // 4 hidden rows, a fold, a badge, and the banner.
    expect(addedMarks()).toBe(7);

    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    const h = harness(opts);
    await h.controller.start();
    await h.settle();
    warn.mockRestore();
    expect(h.controller.status().state).not.toBe('ready');
    expect(hiddenRows(document.querySelector<HTMLElement>('#files .file')!)).toEqual([]);
    expect(addedMarks()).toBe(0);
  });
});
