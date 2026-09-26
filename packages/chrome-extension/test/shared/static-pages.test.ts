// Checks the static HTML pages against the English message catalog.

import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import enMessages from '../../static/_locales/en/messages.json';

const staticDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'static');

describe('static pages', () => {
  it('use only data-i18n keys that exist', () => {
    const pages = readdirSync(staticDir).filter((f) => f.endsWith('.html'));
    expect(pages.length).toBeGreaterThan(0);
    for (const page of pages) {
      const html = readFileSync(join(staticDir, page), 'utf8');
      for (const m of html.matchAll(/data-i18n(?:-title)?="([^"]+)"/g)) {
        expect(enMessages, `${page}: ${m[1]}`).toHaveProperty([m[1] ?? '']);
      }
    }
  });
});
