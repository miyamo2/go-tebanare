// Checks on static/content.css, the stylesheet of the fold rows, badges, and banner.

import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const pkgRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const read = (name: string) => readFileSync(join(pkgRoot, name), 'utf8');

describe('content.css', () => {
  it('keeps GitHub row backgrounds in debug mode', () => {
    const css = read('static/content.css').replace(/\/\*[\s\S]*?\*\//g, '');
    const rules = [...css.matchAll(/([^{}]+)\{([^}]*)\}/g)].filter((m) => m[1]?.includes('data-gotebanare-debug'));
    expect(rules.length).toBeGreaterThan(0);
    for (const [, selector, body] of rules) expect(body, selector).not.toMatch(/(^|;)\s*background/);
  });

  it('keeps the file badge from shrinking next to a long path', () => {
    const css = read('static/content.css').replace(/\/\*[\s\S]*?\*\//g, '');
    const badge = [...css.matchAll(/([^{}]+)\{([^}]*)\}/g)].find((m) => m[1]?.trim() === '.gotebanare-badge');
    expect(badge?.[2]).toMatch(/flex-shrink:\s*0/);
  });
});
