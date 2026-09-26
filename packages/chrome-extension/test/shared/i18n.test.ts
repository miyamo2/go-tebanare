// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from 'vitest';
import { countLines, localizePage, t, type MessageKey } from '../../src/shared/i18n.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';

let restore = () => {};
afterEach(() => restore());

function useLocale(locale: string): void {
  restore = installChrome(new FakeChrome({ locale }).extensionContext());
}

describe('t', () => {
  it('returns the key without chrome.i18n', () => {
    expect(t('foldShowTitle', 3)).toBe('foldShowTitle');
  });

  it('returns the key for a missing message', () => {
    useLocale('en');
    expect(t('noSuchMessage' as MessageKey)).toBe('noSuchMessage');
  });

  it('formats English messages', () => {
    useLocale('en');
    expect(t('bannerTitle')).toBe('gotebanare');
    expect(t('foldShowTitle', countLines(1))).toBe('Show 1 line hidden by gotebanare');
    expect(t('foldHideTitle', countLines(24))).toBe('Hide 24 lines again');
    expect(t('fileShowTitle', countLines(57))).toBe('Show the 57 lines that gotebanare hid in this file');
  });

  it('formats Japanese messages', () => {
    useLocale('ja');
    expect(t('foldShowTitle', countLines(24))).toBe('gotebanare が隠した 24 行を表示');
    expect(t('fileHideTitle', countLines(57))).toBe('このファイルの 57 行をもう一度隠す');
  });
});

describe('localizePage', () => {
  it('fills text and titles from data attributes', () => {
    useLocale('en');
    document.body.innerHTML = '<h1 data-i18n="optionsTitle"></h1><button data-i18n="optionsSave" data-i18n-title="optionsSaved"></button>';
    localizePage(document);
    expect(document.querySelector('h1')?.textContent).toBe('go-tebanare options');
    const button = document.querySelector('button');
    expect(button?.textContent).toBe('Save');
    expect(button?.title).toBe('Saved.');
  });
});
