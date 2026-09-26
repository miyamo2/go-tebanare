// UI strings come from static/_locales/<locale>/messages.json through
// chrome.i18n. The English file defines the set of valid keys.

import type enMessages from '../../static/_locales/en/messages.json';

export type MessageKey = keyof typeof enMessages;

export type Substitution = string | number;

/**
 * t returns the localized message for key with $1..$9 replaced by subs.
 * It returns the key itself when the message is missing or chrome.i18n is
 * unavailable (unit tests without the chrome fake).
 */
export function t(key: MessageKey, ...subs: Substitution[]): string {
  const i18n = globalThis.chrome?.i18n;
  if (!i18n) return key;
  const msg = i18n.getMessage(key, subs.map(String));
  return msg === '' ? key : msg;
}

/** countLines returns "1 line" or "N lines" in the UI language. */
export function countLines(n: number): string {
  return n === 1 ? t('countLinesOne') : t('countLines', n);
}

/**
 * localizePage fills static extension pages: an element with
 * data-i18n="key" gets the message as its text, and one with
 * data-i18n-title="key" gets it as its title.
 */
export function localizePage(root: ParentNode): void {
  for (const el of root.querySelectorAll<HTMLElement>('[data-i18n]')) {
    // The attribute comes from our own HTML; t() falls back to the key when it is wrong.
    el.textContent = t(el.dataset['i18n'] as MessageKey);
  }
  for (const el of root.querySelectorAll<HTMLElement>('[data-i18n-title]')) {
    el.title = t(el.dataset['i18nTitle'] as MessageKey);
  }
}
