// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import type { PullRequestContext } from '../../src/content/context.js';
import { Run } from '../../src/content/controller/run.js';
import { detectVariant } from '../../src/content/dom/variant.js';
import { Semaphore } from '../../src/content/fetcher.js';
import { removeBanner } from '../../src/content/ui/banner.js';
import type { ConfigSource } from '../../src/shared/messages.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';
import { BASE, CONFIG, FakeBackground, FakeFetcher, HEAD, PAGE, addedMarks, bannerTexts, buildPage, fileHtml, hiddenRows, hide } from './controller-fakes.js';

let restore = () => {};
beforeEach(() => {
  restore = installChrome(new FakeChrome({ locale: 'en' }).extensionContext());
});
afterEach(() => restore());

const CTX: PullRequestContext = { ...PAGE, baseSha: BASE, headSha: HEAD };
const modified = () => fileHtml('classic-modified.html');

/** setup creates a run over the page with fakes, and a fetcher that serves CONFIG at both commits. */
function setup(ctx: PullRequestContext | null = CTX, source: ConfigSource = 'base') {
  const bg = new FakeBackground();
  bg.ranges.set('store/store.go', { old: [], new: [hide(26, 29)] });
  const fetcher = new FakeFetcher();
  fetcher.set(BASE, '.gotebanare.yml', CONFIG);
  fetcher.set(HEAD, '.gotebanare.yml', CONFIG);
  const deps = { doc: document, send: bg.send, fetcher, limit: new Semaphore(4), detectVariant, debug: false, enabled: () => true };
  return { run: new Run(deps, ctx, source), bg, fetcher };
}

async function settle(): Promise<void> {
  for (let i = 0; i < 8; i++) await new Promise((resolve) => setTimeout(resolve, 0));
}

describe('Run', () => {
  it('loads the config and shows the files', async () => {
    const [store] = buildPage(modified());
    const { run } = setup();
    await run.start();
    await settle();
    expect(hiddenRows(store!)).toEqual(['+26', '+27', '+28', '+29']);
    expect(run.status()).toEqual({
      state: 'ready',
      configSource: 'base',
      configPath: '.gotebanare.yml',
      rules: 1,
      files: 1,
      filesWithFolds: 1,
      linesHidden: 4,
      messages: [],
    });
  });

  it.each<[string, (s: ReturnType<typeof setup>) => void, object, string[]]>([
    ['no config', (s) => s.fetcher.files.clear(), { state: 'no-config' }, []],
    [
      'a config that cannot be fetched',
      (s) => s.fetcher.set(BASE, '.gotebanare.yml', { ok: false, reason: 'network', status: 0 }),
      { state: 'error', configPath: '.gotebanare.yml' },
      ['Nothing is hidden because the config .gotebanare.yml could not be loaded. The request failed because of a network error.'],
    ],
    [
      'an invalid config',
      (s) => s.fetcher.set(BASE, '.gotebanare.yml', 'invalid: true\n'),
      { state: 'config-error', configPath: '.gotebanare.yml' },
      ['.gotebanare.yml has errors, so nothing is hidden.', 'line 3, field presets[0](gettr): unknown preset "gettr"'],
    ],
  ])('hides nothing with %s', async (_, arrange, status, messages) => {
    const [store] = buildPage(modified());
    const s = setup();
    arrange(s);
    await s.run.start();
    await settle();
    expect(s.run.status()).toMatchObject({ ...status, messages, rules: 0, files: 0 });
    expect(bannerTexts()).toEqual(messages);
    expect(hiddenRows(store!)).toEqual([]);
    expect(s.bg.types()).not.toContain('lookup');
  });

  it('ends in the error state without commits', async () => {
    buildPage(modified());
    const { run, fetcher } = setup(null);
    await run.start();
    expect(run.status()).toMatchObject({ state: 'error', messages: ['Could not read the pull request commits from this page, so nothing is hidden.'] });
    expect(fetcher.calls).toEqual([]);
  });

  it('reads the head config for a preview', async () => {
    buildPage(modified());
    const { run, fetcher } = setup(CTX, 'head');
    await run.start();
    await settle();
    expect(fetcher.calls.slice(0, 2)).toEqual(['head:.gotebanare.yml', 'head:.gotebanare.yaml']);
    expect(run.status()).toMatchObject({ state: 'ready', configSource: 'head', linesHidden: 4 });
    expect(bannerTexts()).toEqual(['Previewing with the head branch config .gotebanare.yml. The pull request author controls this config.']);
  });

  it('waits for a known diff UI', async () => {
    buildPage(fileHtml('classic-no-diff.html'));
    const { run } = setup();
    await run.start();
    expect(run.status().state).toBe('unsupported-ui');
    document.querySelector('.js-diff-progressive-container')!.insertAdjacentHTML('beforeend', modified());
    run.update({ targets: [document.body], added: [] });
    await settle();
    expect(run.status()).toMatchObject({ state: 'ready', messages: [], linesHidden: 4 });
  });

  it('removes what it added on stop and ignores late results', async () => {
    const [store] = buildPage(modified(), fileHtml('classic-added.html'));
    const { run, bg, fetcher } = setup();
    fetcher.hold((p) => p === 'store/mock_store.go');
    await run.start();
    await settle();
    expect(hiddenRows(store!)).toHaveLength(4);
    run.stop();
    // The banner, still showing the loading indicator for the held file, is
    // left for the next run or the controller to replace.
    removeBanner(document);
    expect(addedMarks()).toBe(0);
    fetcher.release();
    run.update({ targets: [document.body], added: [] });
    await settle();
    expect(addedMarks()).toBe(0);
    expect(bg.types()).not.toContain('analyze store/mock_store.go');
  });
});
