// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from 'vitest';
import { optionsElements, renderStatus, type OptionsElements } from '../../src/options/form.js';
import { startOptions } from '../../src/options/options.js';
import { OPTIONS_KEY, isExcluded, loadOptions } from '../../src/shared/settings.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';
import { loadStaticPage } from '../fakes/static-page.js';

let restore = () => {};
afterEach(() => restore());

function setup(locale = 'en', stored?: unknown) {
  const hub = new FakeChrome({ locale });
  restore = installChrome(hub.extensionContext('options.html'));
  if (stored !== undefined) hub.storage.sync.data.set(OPTIONS_KEY, stored);
  loadStaticPage('options.html');
  return { hub, el: optionsElements(document) };
}

function type(el: OptionsElements, repos: string, debug?: boolean): void {
  el.repos.value = repos;
  el.repos.dispatchEvent(new Event('input'));
  if (debug !== undefined) el.debug.checked = debug;
}

const invalidItems = (el: OptionsElements) => [...el.invalid.querySelectorAll('li')].map((li) => li.textContent);

describe('options page', () => {
  it('localizes the page and fills the form from chrome.storage.sync', async () => {
    const { el } = setup('en', { excludedRepos: ['octo/*', 7, 'a/b'], debug: true });
    await startOptions(document, chrome.storage.sync);
    expect(document.documentElement.lang).toBe('en');
    expect(document.querySelector('h1')?.textContent).toBe('go-tebanare options');
    expect(el.save.textContent).toBe('Save');
    expect(el.repos.value).toBe('octo/*\na/b');
    expect(el.debug.checked).toBe(true);
    expect(el.status.textContent).toBe('');
    expect(el.invalid.hidden).toBe(true);
    expect(el.save.disabled).toBe(false);
  });

  it('saves valid patterns and the debug flag', async () => {
    const { hub, el } = setup();
    const page = await startOptions(document, chrome.storage.sync);
    expect(el.repos.value).toBe('');
    type(el, ' octo/repo\n\nfoo/*\n', true);
    await page.save();
    expect(hub.storage.sync.data.get(OPTIONS_KEY)).toEqual({ excludedRepos: ['octo/repo', 'foo/*'], debug: true });
    expect(el.status.textContent).toBe('Saved.');
    expect(el.status.dataset['level']).toBe('success');
    expect(el.invalid.hidden).toBe(true);
  });

  it('saves on submit', async () => {
    const { hub, el } = setup();
    await startOptions(document, chrome.storage.sync);
    type(el, '*');
    el.form.requestSubmit();
    await expect.poll(() => hub.storage.sync.data.get(OPTIONS_KEY)).toEqual({ excludedRepos: ['*'], debug: false });
    await expect.poll(() => el.status.textContent).toBe('Saved.');
  });

  it('saves nothing while a line is not a pattern, and lists the lines to fix', async () => {
    const { hub, el } = setup('en', { excludedRepos: ['old/*'], debug: false });
    const page = await startOptions(document, chrome.storage.sync);
    const text = 'octo/repo\nhttps://github.com/a/b\n\n*/x';
    type(el, text, true);
    await page.save();
    expect(hub.storage.sync.data.get(OPTIONS_KEY)).toEqual({ excludedRepos: ['old/*'], debug: false });
    expect(el.repos.value).toBe(text);
    expect(el.status.textContent).toBe('Not saved. Fix the lines below. Each line needs owner/name, owner/*, or *.');
    expect(el.status.dataset['level']).toBe('error');
    expect(el.save.disabled).toBe(false);
    expect(el.invalid.hidden).toBe(false);
    expect(invalidItems(el)).toEqual(['Line 2: https://github.com/a/b', 'Line 4: */x']);

    type(el, 'octo/repo');
    expect(el.status.textContent).toBe('');
    expect(el.invalid.hidden).toBe(true);
    await page.save();
    expect(hub.storage.sync.data.get(OPTIONS_KEY)).toEqual({ excludedRepos: ['octo/repo'], debug: true });
  });

  it('keeps a stored exclusion when an edit makes its line invalid', async () => {
    const { el } = setup('en', { excludedRepos: ['octo/secret'], debug: false });
    const page = await startOptions(document, chrome.storage.sync);
    type(el, 'octo/secret # keep off');
    await page.save();
    const { excludedRepos } = await loadOptions(chrome.storage.sync);
    expect(isExcluded('octo/secret', excludedRepos)).toBe(true);
    expect(invalidItems(el)).toEqual(['Line 1: octo/secret # keep off']);
  });

  it('reports a failed save and keeps the stored options', async () => {
    const { hub, el } = setup('en', { excludedRepos: ['old/*'], debug: false });
    const page = await startOptions(document, chrome.storage.sync);
    type(el, Array.from({ length: 800 }, (_, i) => `owner${i}/repo${i}`).join('\n'));
    await page.save();
    expect(el.status.textContent).toBe('Could not save: QUOTA_BYTES_PER_ITEM quota exceeded');
    expect(el.status.dataset['level']).toBe('error');
    expect(el.save.disabled).toBe(false);
    expect(hub.storage.sync.data.get(OPTIONS_KEY)).toEqual({ excludedRepos: ['old/*'], debug: false });
  });

  it('turns Save off when the stored options cannot be read', async () => {
    const { hub, el } = setup();
    hub.storage.sync.failure = new Error('sync is broken');
    const page = await startOptions(document, chrome.storage.sync);
    expect(el.status.textContent).toBe('Could not load the saved options: sync is broken');
    expect(el.save.disabled).toBe(true);
    hub.storage.sync.failure = null;
    type(el, 'octo/*');
    await page.save();
    expect(el.save.disabled).toBe(true);
    expect(hub.storage.sync.data.size).toBe(0);
  });

  it('uses the Japanese messages', async () => {
    const { el } = setup('ja');
    const page = await startOptions(document, chrome.storage.sync);
    expect(document.documentElement.lang).toBe('ja');
    expect(el.save.textContent).toBe('保存');
    type(el, 'bad');
    await page.save();
    expect(el.status.textContent).toMatch(/^保存していません。/);
    expect(invalidItems(el)).toEqual(['1 行目: bad']);
  });

  it('throws when the page lacks a control', () => {
    document.body.innerHTML = '<form id="options-form"></form>';
    expect(() => optionsElements(document)).toThrow('options.html has no HTMLTextAreaElement #excluded-repos');
  });
});

describe('renderStatus', () => {
  it('turns Save off while saving', () => {
    setup();
    const el = optionsElements(document);
    renderStatus(el, { kind: 'saving' });
    expect(el.save.disabled).toBe(true);
    expect(el.status.textContent).toBe('');
    expect(el.status.dataset['level']).toBeUndefined();
    renderStatus(el, { kind: 'saved' });
    expect(el.save.disabled).toBe(false);
  });
});
