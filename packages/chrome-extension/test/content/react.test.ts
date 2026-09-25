// @vitest-environment happy-dom
// The React variant on a saved "Files changed" page (react-modified.html).
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { classicVariant as classic } from '../../src/content/dom/classic.js';
import { reactVariant as react } from '../../src/content/dom/react.js';
import { detectVariant, type RowRef } from '../../src/content/dom/variant.js';
import { loadFixture as loadClassic } from './classic-fixture.js';

// A path string: happy-dom replaces the global URL class, which node:fs rejects.
const fixtures = join(dirname(fileURLToPath(import.meta.url)), '..', 'fixtures');

const MAIN = 'cmd/server/main.go';
const TASK = 'domain/task.go';

/** load puts the body of react-modified.html in the document and returns its file containers. */
function load(): HTMLElement[] {
  const html = readFileSync(join(fixtures, 'react-modified.html'), 'utf8');
  document.body.innerHTML = new DOMParser().parseFromString(html, 'text/html').body.innerHTML;
  return [...react.fileContainers(document)];
}

/** file loads the fixture and returns the container of path. */
function file(path: string): HTMLElement {
  const container = load().find((c) => react.filePath(c).path === path);
  if (!container) throw new Error(`no container for ${path}`);
  return container;
}

/** addRow returns the row of the line that main.go adds. */
function addRow(container: HTMLElement): HTMLElement {
  const code = container.querySelector('code.diff-text.addition');
  const tr = code?.closest('tr');
  if (!tr) throw new Error('no add row');
  return tr;
}

const shape = (rows: Iterable<RowRef>) => [...rows].map((r) => `${r.kind} ${r.oldLine ?? '-'} ${r.newLine ?? '-'}`);

describe('detect', () => {
  it('recognizes the React page and no other', () => {
    load();
    expect(react.detect(document)).toBe(true);
    expect(classic.detect(document)).toBe(false);
    expect(detectVariant(document)).toBe(react);
    loadClassic('classic-modified.html');
    expect(react.detect(document)).toBe(false);
    expect(detectVariant(document)).toBe(classic);
  });
});

describe('file containers', () => {
  it('finds the regions whose table has their id', () => {
    const [main, task] = load() as [HTMLElement, HTMLElement];
    expect([main, task].map((c) => react.filePath(c))).toEqual([{ path: MAIN }, { path: TASK }]);
    expect([...react.fileContainers(main)]).toEqual([main]);
    main.querySelector('table')?.setAttribute('data-diff-anchor', 'diff-other');
    expect([...react.fileContainers(document)]).toEqual([task]);
  });

  it('reads the path without the directional marks around it', () => {
    const container = file(MAIN);
    expect(container.querySelector('h3')?.textContent).toBe(`‎${MAIN}‎`);
    expect(react.filePath(container)).toEqual({ path: MAIN });
  });

  it('returns an empty path without the heading or when the header button names another file', () => {
    let container = file(MAIN);
    container.querySelector('[data-file-path]')?.setAttribute('data-file-path', 'cmd/server/other.go');
    expect(react.filePath(container)).toEqual({ path: '' });
    container = file(MAIN);
    container.querySelector('h3')?.removeAttribute('id');
    expect(react.filePath(container)).toEqual({ path: '' });
  });

  it('reads a heading with an arrow as a rename', () => {
    const container = file(MAIN);
    (container.querySelector('h3 code') as HTMLElement).textContent = `‎cmd/old/main.go → ${MAIN}‎`;
    expect(react.filePath(container)).toEqual({ path: MAIN, oldPath: 'cmd/old/main.go' });
  });

  it('puts the badge host around the heading and the path buttons', () => {
    const host = react.fileHeader?.(file(MAIN));
    expect(host?.querySelector(':scope > h3')).not.toBeNull();
    expect(host?.querySelector(':scope > [data-file-path]')).not.toBeNull();
  });

  it.each([
    ['@@ -31,6 +31,7 @@ func main() {', 'modified'],
    ['@@ -0,0 +1,3 @@', 'added'],
    ['@@ -1,3 +0,0 @@', 'deleted'],
  ])('reads a first hunk of %s as %s', (hunk, want) => {
    const container = file(MAIN);
    (container.querySelector('td.diff-hunk-cell .diff-text-inner') as HTMLElement).textContent = hunk;
    expect(react.fileStatus?.(container)).toBe(want);
  });
});

describe('rows', () => {
  it('reads a modified file', () => {
    const rows = [...react.rows(file(MAIN))];
    expect(shape(rows)).toEqual([
      'hunk - -', 'context 31 31', 'context 32 32', 'context 33 33', 'add - 34',
      'context 34 35', 'context 35 36', 'context 36 37', 'expander - -',
    ]);
    expect(rows.every((r) => r.el.tagName === 'TR')).toBe(true);
  });

  it('reads every hunk of a file', () => {
    const kinds = shape(react.rows(file(TASK)));
    expect(kinds).toHaveLength(39);
    expect(kinds.filter((k) => k === 'hunk - -')).toHaveLength(2);
    const second = kinds.lastIndexOf('hunk - -');
    expect(kinds.slice(second - 1, second + 2)).toEqual(['context 23 33', 'hunk - -', 'context 63 73']);
    expect(kinds.at(-1)).toBe('expander - -');
  });

  it('takes the text from the code without the marker', () => {
    const rows = [...react.rows(file(MAIN))];
    expect(rows[4]?.text).toBe('\tmux.HandleFunc("DELETE /tasks/{id}", taskHandler.Delete)');
    expect(rows[3]?.text).toBe('\tmux.HandleFunc("POST /tasks/{id}/complete", taskHandler.Complete)');
    expect(rows[5]?.text).toBe('');
    expect(rows[0]?.text).toBeUndefined();
  });

  it('reads a deleted line', () => {
    const container = file(MAIN);
    const tr = addRow(container);
    const [left, right] = [...tr.children] as [HTMLElement, HTMLElement];
    left.setAttribute('data-diff-side', 'left');
    left.setAttribute('data-line-number', '34');
    left.textContent = '34';
    right.removeAttribute('data-diff-side');
    right.removeAttribute('data-line-number');
    right.textContent = '';
    const code = tr.querySelector('code') as HTMLElement;
    code.classList.replace('addition', 'deletion');
    (code.querySelector('.diff-text-marker') as HTMLElement).textContent = '-';
    const row = [...react.rows(container)][4];
    expect(row && shape([row])).toEqual(['del 34 -']);
    expect(row?.text).toBe('\tmux.HandleFunc("DELETE /tasks/{id}", taskHandler.Delete)');
  });

  it.each([
    ['a missing line number', (tr: Element) => tr.children[1]?.removeAttribute('data-line-number')],
    ['a malformed line number', (tr: Element) => tr.children[1]?.setAttribute('data-line-number', '34a')],
    ['a line number on the empty side', (tr: Element) => tr.children[0]?.setAttribute('data-line-number', '33')],
    ['a side that does not match its cell', (tr: Element) => tr.children[1]?.setAttribute('data-diff-side', 'left')],
    ['a deletion class on an added line', (tr: Element) => tr.querySelector('code')?.classList.replace('addition', 'deletion')],
    ['a missing code cell', (tr: Element) => tr.querySelector('td.diff-text-cell')?.remove()],
    ['more text in the code cell', (tr: Element) => tr.querySelector('td.diff-text-cell')?.append('LGTM')],
    ['more text in a number cell', (tr: Element) => tr.children[0]?.append('Comment')],
    ['a hunk cell that is not a header', (tr: Element) => tr.closest('tbody')?.querySelector('.diff-hunk-cell .diff-text-inner')?.replaceChildren('Load diff')],
    ['a row of another class', (tr: Element) => tr.classList.remove('diff-line-row')],
  ])('yields nothing for a file with %s', (_, spoil) => {
    const container = file(MAIN);
    spoil(addRow(container));
    expect([...react.rows(container)]).toEqual([]);
  });

  it('skips fold rows the extension inserted', () => {
    const container = file(MAIN);
    const before = shape(react.rows(container));
    const fold = document.createElement('tr');
    fold.setAttribute('data-gotebanare-fold', 'x');
    addRow(container).before(fold);
    expect(shape(react.rows(container))).toEqual(before);
  });

  it('reads a row with two code cells as split view and yields nothing', () => {
    const container = file(MAIN);
    expect(react.isSplit?.(container)).toBe(false);
    const tr = addRow(container);
    tr.append((tr.querySelector('td.diff-text-cell') as HTMLElement).cloneNode(true));
    expect(react.isSplit?.(container)).toBe(true);
    expect([...react.rows(container)]).toEqual([]);
  });
});
