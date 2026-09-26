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
  it('shows the hidden lines with an eye button', () => {
    useLocale('en');
    const badge = createBadge(document, { lines: 57, allOpen: false }, () => {});
    expect(badge.hasAttribute(BADGE_ATTR)).toBe(true);
    expect(badge.tagName).toBe('BUTTON');
    expect(badge.textContent).toBe('');
    expect(badge.getAttribute('aria-label')).toBe('Show the 57 lines that gotebanare hid in this file');
    expect(badge.title).toBe(badge.getAttribute('aria-label'));
    expect(badge.querySelector('svg')?.getAttribute('class')).toBe('octicon octicon-eye');
  });

  it('hides them again with an eye-closed button once every fold is open', () => {
    useLocale('en');
    const badge = createBadge(document, { lines: 1, allOpen: true }, () => {});
    expect(badge.getAttribute('aria-label')).toBe('Hide the 1 line in this file again');
    expect(badge.querySelector('svg')?.getAttribute('class')).toBe('octicon octicon-eye-closed');
  });

  it('takes every text from the UI language', () => {
    useLocale('ja');
    const ja = localeMessages('ja');
    const msg = (key: string, ...subs: string[]) => formatMessage(ja[key] as LocaleEntry, subs);
    const lines = msg('countlines', '57');
    expect(createBadge(document, { lines: 57, allOpen: false }, () => {}).getAttribute('aria-label')).toBe(msg('fileshowtitle', lines));
    expect(createBadge(document, { lines: 57, allOpen: true }, () => {}).getAttribute('aria-label')).toBe(msg('filehidetitle', lines));
  });

  it('calls onToggle and keeps the click inside the badge', () => {
    const onToggle = vi.fn();
    const header = document.createElement('div');
    const onHeader = vi.fn();
    header.addEventListener('click', onHeader);
    header.append(createBadge(document, { lines: 9, allOpen: false }, onToggle));
    header.querySelector('button')?.click();
    expect(onToggle).toHaveBeenCalledOnce();
    expect(onHeader).not.toHaveBeenCalled();
  });
});
