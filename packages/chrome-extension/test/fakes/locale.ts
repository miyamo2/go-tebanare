// chrome.i18n message lookup over static/_locales, used by the chrome fake.

import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

// A path string: happy-dom replaces the global URL class, which node:fs rejects.
const localesDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'static', '_locales');

export interface LocaleEntry { message: string; placeholders?: Record<string, { content: string }> }

/** formatMessage applies chrome.i18n placeholder and $1..$9 substitution rules. */
export function formatMessage(entry: LocaleEntry, subs: readonly string[]): string {
  const ph = new Map(Object.entries(entry.placeholders ?? {}).map(([k, v]) => [k.toLowerCase(), v.content]));
  const positional = (s: string) => s.replace(/\$(\d)/g, (_, d: string) => subs[Number(d) - 1] ?? '');
  return entry.message.replace(/\$\$|\$([A-Za-z0-9_@]+)\$|\$(\d)/g, (m, name?: string, digit?: string) => {
    if (m === '$$') return '$';
    if (name !== undefined) {
      const content = ph.get(name.toLowerCase());
      return content === undefined ? m : positional(content);
    }
    return subs[Number(digit) - 1] ?? '';
  });
}

const loaded = new Map<string, Record<string, LocaleEntry>>();

/**
 * localeMessages returns the messages.json entries of locale ("en" or
 * "ja") keyed by lowercase name, as chrome.i18n matches them. Each file is
 * read once.
 */
export function localeMessages(locale: string): Record<string, LocaleEntry> {
  let m = loaded.get(locale);
  if (!m) {
    const raw = JSON.parse(readFileSync(join(localesDir, locale, 'messages.json'), 'utf8')) as Record<string, LocaleEntry>;
    m = Object.fromEntries(Object.entries(raw).map(([k, v]) => [k.toLowerCase(), v]));
    loaded.set(locale, m);
  }
  return m;
}
