// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { BADGE_ATTR, createBadge } from '../../src/content/ui/badge.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';
import { formatMessage, localeMessages, type LocaleEntry } from '../fakes/locale.js';

let restore = () => {};
afterEach(() => restore());

function useLocale(locale: string): void {
  restore = installChrome(new FakeChrome({ locale }).extensionContext());
}

describe('createBadge', () => {
  it('counts folds and lines', () => {
    useLocale('en');
    const badge = createBadge(document, 3, 57, () => {});
    expect(badge.hasAttribute(BADGE_ATTR)).toBe(true);
    expect(badge.querySelector('.gotebanare-badge-text')?.textContent).toBe('3 folds / 57 lines hidden');
    expect(badge.querySelector('button')?.textContent).toBe('Show all');
    expect(createBadge(document, 1, 1, () => {}).textContent).toBe('1 fold / 1 line hiddenShow all');
  });

  it('takes every text from the UI language', () => {
    useLocale('ja');
    const ja = localeMessages('ja');
    const msg = (key: string, ...subs: string[]) => formatMessage(ja[key] as LocaleEntry, subs);
    const text = msg('badgetext', msg('countfolds', '3'), msg('countlines', '57'));
    expect(createBadge(document, 3, 57, () => {}).textContent).toBe(text + msg('badgeshowall'));
  });

  it('calls onShowAll and keeps the click inside the badge', () => {
    const onShowAll = vi.fn();
    const header = document.createElement('div');
    const onHeader = vi.fn();
    header.addEventListener('click', onHeader);
    header.append(createBadge(document, 2, 9, onShowAll));
    header.querySelector('button')?.click();
    expect(onShowAll).toHaveBeenCalledOnce();
    expect(onHeader).not.toHaveBeenCalled();
  });
});
