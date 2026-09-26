// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest';
import { classicVariant as classic } from '../../src/content/dom/classic.js';
import { detectVariant, type DiffUiVariant, type RowRef } from '../../src/content/dom/variant.js';
import { loadFixture, onlyFile } from './classic-fixture.js';

const shape = (rows: Iterable<RowRef>) => [...rows].map((r) => `${r.kind} ${r.oldLine ?? '-'} ${r.newLine ?? '-'}`);
const rowAt = (rows: RowRef[], oldLine?: number, newLine?: number) =>
  rows.find((r) => r.oldLine === oldLine && r.newLine === newLine && r.kind !== 'comment');

describe('detect', () => {
  it('recognizes a diff page', () => {
    loadFixture('classic-modified.html');
    expect(classic.detect(document)).toBe(true);
    expect(detectVariant(document)).toBe(classic);
  });

  it('tries the registry in order', () => {
    loadFixture('classic-modified.html');
    const never: DiffUiVariant = { ...classic, id: 'never', detect: () => false };
    const always: DiffUiVariant = { ...classic, id: 'always', detect: () => true };
    expect(detectVariant(document, [never, always, classic])?.id).toBe('always');
    expect(detectVariant(document, [never])).toBeNull();
  });
});

describe('file containers', () => {
  it('accepts a file container as the root', () => {
    const container = onlyFile('classic-modified.html');
    expect([...classic.fileContainers(container)]).toEqual([container]);
  });

  it('reads the path, or an empty path when the header has none', () => {
    const container = onlyFile('classic-modified.html');
    expect(classic.filePath(container)).toEqual({ path: 'store/store.go' });
    container.querySelector('.file-header')?.removeAttribute('data-path');
    expect(classic.filePath(container)).toEqual({ path: '' });
  });

  it('finds the badge host', () => {
    expect(classic.fileHeader?.(onlyFile('classic-modified.html'))?.classList.contains('file-info')).toBe(true);
  });

  it.each([
    ['false', '@@ -9,6 +9,7 @@', 'modified'],
    ['false', '@@ -1,3 +0,0 @@', 'modified'],
    ['false', '@@ -0,0 +1,3 @@', 'added'],
    ['true', '@@ -9,6 +9,7 @@', 'deleted'],
    [null, '@@ -1,3 +0,0 @@', 'deleted'],
    [null, '@@ -9,6 +9,7 @@', 'modified'],
  ])('reads data-file-deleted=%s with hunk %s as %s', (deleted, hunk, want) => {
    const container = onlyFile('classic-modified.html');
    const header = container.querySelector('.file-header') as HTMLElement;
    if (deleted === null) header.removeAttribute('data-file-deleted');
    else header.setAttribute('data-file-deleted', deleted);
    (container.querySelector('td.blob-code-hunk') as HTMLElement).textContent = hunk;
    expect(classic.fileStatus?.(container)).toBe(want);
  });
});

describe('rows', () => {
  it('reads a modified file', () => {
    const rows = [...classic.rows(onlyFile('classic-modified.html'))];
    const kinds = shape(rows);
    expect(kinds).toHaveLength(41);
    expect(kinds.slice(0, 9)).toEqual([
      'hunk - -', 'context 9 9', 'context 10 10', 'context 11 11', 'add - 12',
      'context 12 13', 'context 13 14', 'context 14 15', 'hunk - -',
    ]);
    expect(kinds.slice(17, 25)).toEqual([
      'context 25 31', 'context 26 32', 'del 27 -', 'del 28 -', 'add - 33', 'add - 34', 'context 29 35', 'context 30 36',
    ]);
    expect(kinds.at(-1)).toBe('expander - -');
    expect(kinds.filter((k) => k.startsWith('hunk'))).toHaveLength(3);
    expect(rows.every((r) => r.el.tagName === 'TR')).toBe(true);
  });

  it('takes the text from the code without the marker or the comment button', () => {
    const rows = [...classic.rows(onlyFile('classic-modified.html'))];
    expect(rowAt(rows, undefined, 12)?.text).toBe('\towner string');
    expect(rowAt(rows, 28, undefined)?.text).toBe('\t\treturn errors.New("empty key")');
    expect(rowAt(rows, 24, 25)?.text).toBe('');
    expect(rows[0]?.text).toBeUndefined();
  });

  it.each([
    ['a missing line number', (tr: Element) => tr.querySelector('[data-line-number]')?.removeAttribute('data-line-number')],
    ['a malformed line number', (tr: Element) => tr.querySelector('[data-line-number]')?.setAttribute('data-line-number', '12a')],
    ['an extra line number', (tr: Element) => tr.querySelector('.empty-cell')?.setAttribute('data-line-number', '3')],
    ['an unknown code class', (tr: Element) => tr.querySelector('.blob-code')?.classList.remove('blob-code-addition')],
    ['a missing code cell', (tr: Element) => tr.querySelector('.blob-code')?.remove()],
  ])('yields nothing for a row with %s', (_, spoil) => {
    const container = onlyFile('classic-modified.html');
    const add = [...container.querySelectorAll('tbody > tr')].find((tr) => tr.querySelector('.blob-code-addition'));
    if (!add) throw new Error('no add row');
    spoil(add);
    expect([...classic.rows(container)]).toEqual([]);
  });

  it('skips fold rows the extension inserted', () => {
    const container = onlyFile('classic-modified.html');
    const before = shape(classic.rows(container));
    const fold = document.createElement('tr');
    fold.setAttribute('data-gotebanare-fold', 'x');
    container.querySelector('tbody > tr:nth-child(3)')?.before(fold);
    expect(shape(classic.rows(container))).toEqual(before);
  });

  it('leaves the text undefined without a code span', () => {
    const container = onlyFile('classic-modified.html');
    container.querySelector('.blob-code-addition .blob-code-inner')?.remove();
    expect(rowAt([...classic.rows(container)], undefined, 12)?.text).toBeUndefined();
  });
});
