// Loads the synthetic classic diff pages in test/fixtures into happy-dom.

import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { classicVariant } from '../../src/content/dom/classic.js';

// A path string: happy-dom replaces the global URL class, which node:fs rejects.
const fixtures = join(dirname(fileURLToPath(import.meta.url)), '..', 'fixtures');

/** loadFixture puts the body of test/fixtures/<name> in the document and returns its file containers. */
export function loadFixture(name: string): HTMLElement[] {
  const html = readFileSync(join(fixtures, name), 'utf8');
  document.body.innerHTML = new DOMParser().parseFromString(html, 'text/html').body.innerHTML;
  return [...classicVariant.fileContainers(document)];
}

/** onlyFile loads a fixture that has exactly one file container and returns it. */
export function onlyFile(name: string): HTMLElement {
  const found = loadFixture(name);
  const [container] = found;
  if (!container || found.length !== 1) throw new Error(`${name}: want 1 file container, got ${found.length}`);
  return container;
}
