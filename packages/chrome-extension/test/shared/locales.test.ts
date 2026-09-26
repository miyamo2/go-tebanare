import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

interface Entry { message: string; placeholders?: Record<string, { content: string }> }

const pkgRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const staticDir = join(pkgRoot, 'static');
const locales = readdirSync(join(staticDir, '_locales'));

function load(locale: string): Record<string, Entry> {
  return JSON.parse(readFileSync(join(staticDir, '_locales', locale, 'messages.json'), 'utf8')) as Record<string, Entry>;
}

/** The $name$ references in a message, lowercased, sorted, without duplicates. */
function references(message: string): string[] {
  const names = [...message.replaceAll('$$', '').matchAll(/\$([A-Za-z0-9_@]+)\$/g)].map((m) => (m[1] ?? '').toLowerCase());
  return [...new Set(names)].sort();
}

function placeholderSignature(e: Entry): string[] {
  return Object.entries(e.placeholders ?? {})
    .map(([name, p]) => `${name.toLowerCase()}=${p.content}`)
    .sort();
}

const en = load('en');

describe('locale files', () => {
  it('include en and ja', () => {
    expect(locales.sort()).toEqual(['en', 'ja']);
  });

  it.each(locales)('%s has the same keys as en', (locale) => {
    expect(Object.keys(load(locale)).sort()).toEqual(Object.keys(en).sort());
  });

  it.each(locales)('%s has the same placeholders as en', (locale) => {
    const messages = load(locale);
    for (const [key, entry] of Object.entries(en)) {
      const other = messages[key];
      if (!other) continue;
      expect(placeholderSignature(other), key).toEqual(placeholderSignature(entry));
    }
  });

  it.each(locales)('%s messages use exactly their declared placeholders', (locale) => {
    for (const [key, entry] of Object.entries(load(locale))) {
      const declared = Object.keys(entry.placeholders ?? {}).map((n) => n.toLowerCase()).sort();
      expect(references(entry.message), key).toEqual(declared);
      // Placeholder contents are $1, $2, ... in declaration order.
      const contents = Object.values(entry.placeholders ?? {}).map((p) => p.content);
      expect(contents, key).toEqual(contents.map((_, i) => `$${i + 1}`));
    }
  });

  it.each(locales)('%s has valid keys and no en or em dashes', (locale) => {
    for (const [key, entry] of Object.entries(load(locale))) {
      expect(key).toMatch(/^[A-Za-z][A-Za-z0-9_]*$/);
      expect(entry.message, key).not.toMatch(/[\u2013\u2014]/);
      expect(entry.message.trim(), key).not.toBe('');
    }
  });

  it.each(locales)('%s popupStateNoConfig names both config file names', (locale) => {
    const message = load(locale)['popupStateNoConfig']?.message;
    expect(message).toContain('.gotebanare.yml');
    expect(message).toContain('.gotebanare.yaml');
  });

  it('keep the extension name and description within Chrome Web Store limits', () => {
    for (const locale of locales) {
      const m = load(locale);
      expect(m['extName']?.message.length).toBeLessThanOrEqual(75);
      expect(m['extDescription']?.message.length).toBeLessThanOrEqual(132);
    }
  });
});

describe('manifest.json', () => {
  it('uses only __MSG_key__ names that exist', () => {
    const manifest = readFileSync(join(pkgRoot, 'manifest.json'), 'utf8');
    const keys = [...manifest.matchAll(/__MSG_(\w+)__/g)].map((m) => m[1] ?? '');
    expect(keys.length).toBeGreaterThan(0);
    for (const key of keys) expect(en, key).toHaveProperty([key]);
  });
});
