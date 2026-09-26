// Loads the body of a static extension page into the happy-dom document,
// without its scripts, so tests can start the page code by hand.

import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

// A path string: happy-dom replaces the global URL class, which node:fs rejects.
const staticDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'static');

/** loadStaticPage replaces document.body with the body of static/<name>. */
export function loadStaticPage(name: string): void {
  const html = readFileSync(join(staticDir, name), 'utf8');
  const m = /<body([^>]*)>([\s\S]*)<\/body>/.exec(html);
  if (!m) throw new Error(`${name} has no body`);
  document.documentElement.lang = '';
  document.body.className = /class="([^"]*)"/.exec(m[1] ?? '')?.[1] ?? '';
  document.body.innerHTML = (m[2] ?? '').replace(/<script\b[^>]*>[\s\S]*?<\/script>/g, '');
}
