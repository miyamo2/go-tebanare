// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import {
  BANNER_ATTR,
  diagnosticText,
  fetchReasonText,
  noticeMessages,
  removeBanner,
  renderBanner,
  type BannerMessage,
  type Notice,
} from '../../src/content/ui/banner.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';

let restore = () => {};
beforeEach(() => {
  restore = installChrome(new FakeChrome({ locale: 'en' }).extensionContext());
});
afterEach(() => restore());

describe('texts', () => {
  it('formats diagnostics with their location', () => {
    expect(diagnosticText({ severity: 'error', code: 'x', message: 'must be at least 1, found 0', line: 3, column: 25, ruleId: 'getter', field: 'presets[0](getter).max_depth' })).toBe(
      'line 3, column 25, rule getter, field presets[0](getter).max_depth: must be at least 1, found 0',
    );
    expect(diagnosticText({ severity: 'error', code: 'x', message: 'bad', field: 'version' })).toBe('field version: bad');
    expect(diagnosticText({ severity: 'warning', code: 'x', message: 'no location' })).toBe('no location');
  });

  it('explains every fetch failure', () => {
    for (const reason of ['not-found', 'unauthorized', 'sso', 'network', 'too-large'] as const) {
      expect(fetchReasonText(reason)).toMatch(/\.$/);
    }
    expect(fetchReasonText('sso')).toContain('single sign-on');
  });

  it('lists a config error and its diagnostics', () => {
    expect(
      noticeMessages({
        kind: 'config-error',
        path: '.gotebanare.yml',
        diagnostics: [
          { severity: 'error', code: 'x', message: 'invalid glob "["', line: 4, field: 'files.exclude[0]' },
          { severity: 'warning', code: 'y', message: 'unused preset' },
        ],
      }),
    ).toEqual([
      { level: 'error', text: '.gotebanare.yml has errors, so nothing is hidden.' },
      { level: 'error', text: 'line 4, field files.exclude[0]: invalid glob "["' },
      { level: 'warning', text: 'unused preset' },
    ]);
  });

  it.each<[Notice, BannerMessage]>([
    [{ kind: 'config-changed', path: '.gotebanare.yml' }, { level: 'warning', text: 'This pull request changes .gotebanare.yml. Folds follow the base branch config.' }],
    [{ kind: 'both-configs' }, { level: 'warning', text: 'Both .gotebanare.yml and .gotebanare.yaml exist. Using .gotebanare.yml.' }],
    [{ kind: 'head-preview', path: '.gotebanare.yml' }, { level: 'warning', text: 'Previewing with the head branch config .gotebanare.yml. The pull request author controls this config.' }],
    [{ kind: 'config-fetch-failed', path: '.gotebanare.yml', reason: 'network' }, { level: 'error', text: 'Nothing is hidden because the config .gotebanare.yml could not be loaded. The request failed because of a network error.' }],
    [{ kind: 'fetch-failed', path: 'a.go', reason: 'too-large' }, { level: 'warning', text: 'Nothing is hidden in a.go. The file is larger than 1 MiB.' }],
    [{ kind: 'source-mismatch', path: 'a.go' }, { level: 'warning', text: 'Nothing is hidden in a.go because the page does not match the analyzed source.' }],
    [{ kind: 'analysis-failed', path: 'a.go', error: 'boom' }, { level: 'warning', text: 'Nothing is hidden in a.go because the analysis failed: boom' }],
    [{ kind: 'context-error' }, { level: 'error', text: 'Could not read the pull request commits from this page, so nothing is hidden.' }],
    [{ kind: 'unsupported-ui' }, { level: 'info', text: 'This GitHub diff layout is not supported yet, so nothing is hidden.' }],
    [{ kind: 'split-view' }, { level: 'info', text: 'Split view is not supported yet. Switch to the unified view to fold code.' }],
  ])('describes %o', (notice, want) => {
    expect(noticeMessages(notice)).toEqual([want]);
  });
});

describe('renderBanner', () => {
  const messages: BannerMessage[] = [
    { level: 'warning', text: 'first' },
    { level: 'error', text: 'second' },
  ];

  function page(): HTMLElement {
    document.body.innerHTML = '<main><h1>PR</h1><div id="files"><div class="file" id="f1"></div><div class="file"></div></div></main>';
    return document.getElementById('f1') as HTMLElement;
  }

  it('puts one banner before the anchor', () => {
    const anchor = page();
    const banner = renderBanner(document, messages, anchor);
    expect(banner?.nextElementSibling).toBe(anchor);
    expect(banner?.querySelector('.gotebanare-banner-title')?.textContent).toBe('gotebanare');
    expect([...(banner?.querySelectorAll('li') ?? [])].map((li) => [li.dataset['level'], li.textContent])).toEqual([
      ['warning', 'first'],
      ['error', 'second'],
    ]);
    expect(document.querySelectorAll(`[${BANNER_ATTR}]`)).toHaveLength(1);
  });

  it('changes nothing when called again with the same messages', () => {
    const anchor = page();
    const first = renderBanner(document, messages, anchor);
    const observer = new MutationObserver(() => {});
    observer.observe(document.body, { childList: true, subtree: true, attributes: true, characterData: true });
    expect(renderBanner(document, [...messages, { level: 'warning', text: 'first' }], anchor)).toBe(first);
    expect(observer.takeRecords()).toEqual([]);
    observer.disconnect();
  });

  it('replaces the banner when the messages or the anchor change', () => {
    const anchor = page();
    renderBanner(document, messages, anchor);
    renderBanner(document, [{ level: 'info', text: 'third' }], anchor);
    expect([...document.querySelectorAll(`[${BANNER_ATTR}] li`)].map((li) => li.textContent)).toEqual(['third']);
    const second = document.querySelectorAll('.file')[1] as HTMLElement;
    renderBanner(document, [{ level: 'info', text: 'third' }], second);
    expect(document.querySelectorAll(`[${BANNER_ATTR}]`)).toHaveLength(1);
    expect(document.querySelector(`[${BANNER_ATTR}]`)?.nextElementSibling).toBe(second);
  });

  it('falls back to the top of main, then body', () => {
    page();
    expect(renderBanner(document, messages, null)).toBe(document.querySelector('main')?.firstElementChild);
    document.body.innerHTML = '<p>no main</p>';
    expect(renderBanner(document, messages, null)).toBe(document.body.firstElementChild);
  });

  it('falls back when GitHub replaced the anchor', () => {
    const anchor = page();
    renderBanner(document, messages, anchor);
    anchor.remove();
    const main = document.querySelector('main');
    const moved = renderBanner(document, messages, anchor);
    expect(moved?.isConnected).toBe(true);
    expect(moved).toBe(main?.firstElementChild);
    expect(document.querySelectorAll(`[${BANNER_ATTR}]`)).toHaveLength(1);
    expect(renderBanner(document, messages, document.createElement('div'))).toBe(moved);
  });

  it('removes the banner for no messages', () => {
    const anchor = page();
    const before = document.body.innerHTML;
    renderBanner(document, messages, anchor);
    expect(renderBanner(document, [], anchor)).toBeNull();
    expect(document.body.innerHTML).toBe(before);
    renderBanner(document, messages, anchor);
    removeBanner(document);
    expect(document.body.innerHTML).toBe(before);
  });
});

describe('loading indicator', () => {
  const banner = () => document.querySelector<HTMLElement>(`[${BANNER_ATTR}]`);
  const loadingText = () => banner()?.querySelector('.gotebanare-banner-loading')?.textContent ?? null;

  it('shows the indicator alone in a quiet banner', () => {
    document.body.innerHTML = '<main></main>';
    renderBanner(document, [], null, true);
    expect(loadingText()).toBe('Checking which lines to hide…');
    expect(banner()?.getAttribute('aria-busy')).toBe('true');
    expect(banner()?.hasAttribute('data-loading-only')).toBe(true);
    expect(banner()?.querySelector('ul')).toBeNull();
  });

  it('shows the indicator with the messages', () => {
    document.body.innerHTML = '<main></main>';
    renderBanner(document, [{ level: 'warning', text: 'w' }], null, true);
    expect(loadingText()).toBe('Checking which lines to hide…');
    expect(banner()?.hasAttribute('data-loading-only')).toBe(false);
    expect([...banner()!.querySelectorAll('li')].map((li) => li.textContent)).toEqual(['w']);
  });

  it('drops the indicator when loading ends', () => {
    document.body.innerHTML = '<main></main>';
    const first = renderBanner(document, [], null, true);
    expect(renderBanner(document, [], null, true)).toBe(first);
    renderBanner(document, [{ level: 'info', text: 'i' }], null, false);
    expect(loadingText()).toBeNull();
    expect(banner()?.hasAttribute('aria-busy')).toBe(false);
    renderBanner(document, [], null, false);
    expect(banner()).toBeNull();
  });
});
