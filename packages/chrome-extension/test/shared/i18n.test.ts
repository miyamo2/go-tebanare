// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from 'vitest';
import { countFolds, countLines, localizePage, t, type MessageKey } from '../../src/shared/i18n.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';

let restore = () => {};
afterEach(() => restore());

function useLocale(locale: string): void {
  restore = installChrome(new FakeChrome({ locale }).extensionContext());
}

describe('t', () => {
  it('returns the key without chrome.i18n', () => {
    expect(t('foldShow')).toBe('foldShow');
  });

  it('returns the key for a missing message', () => {
    useLocale('en');
    expect(t('noSuchMessage' as MessageKey)).toBe('noSuchMessage');
  });

  it('formats English messages', () => {
    useLocale('en');
    expect(t('foldShow')).toBe('Show');
    expect(t('foldSummary', countLines(24), 14, 10)).toBe('gotebanare: 24 lines hidden (\u221214 / +10)');
    expect(t('badgeText', countFolds(1), countLines(1))).toBe('1 fold / 1 line hidden');
    expect(t('badgeText', countFolds(3), countLines(57))).toBe('3 folds / 57 lines hidden');
  });

  it('formats Japanese messages', () => {
    useLocale('ja');
    expect(t('foldSummary', countLines(24), 14, 10)).toBe('gotebanare: 24 行を非表示（\u221214 / +10）');
    expect(t('badgeText', countFolds(3), countLines(57))).toBe('3 箇所 / 57 行を非表示');
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
