// @vitest-environment happy-dom
// Added, deleted, renamed, and split files, a review thread, and a page
// without diffs.
import { describe, expect, it } from 'vitest';
import { classicVariant as classic } from '../../src/content/dom/classic.js';
import { detectVariant, type RowRef } from '../../src/content/dom/variant.js';
import { loadFixture, onlyFile } from './classic-fixture.js';

const shape = (rows: Iterable<RowRef>) => [...rows].map((r) => `${r.kind} ${r.oldLine ?? '-'} ${r.newLine ?? '-'}`);

it.each(['classic-added.html', 'classic-deleted.html', 'classic-renamed.html', 'classic-review-thread.html', 'classic-split.html'])(
  'detects %s',
  (name) => {
    loadFixture(name);
    expect(detectVariant(document)).toBe(classic);
  },
);

it('rejects a page without diff tables', () => {
  expect(loadFixture('classic-no-diff.html')).toHaveLength(0);
  expect(classic.detect(document)).toBe(false);
  expect(detectVariant(document)).toBeNull();
});

describe('renamed files', () => {
  it('finds only files that have a diff table', () => {
    const renamed = loadFixture('classic-renamed.html');
    expect(renamed.map((c) => classic.filePath(c))).toEqual([{ path: 'store/naming.go', oldPath: 'store/names.go' }]);
    expect(document.querySelectorAll('.file')).toHaveLength(2);
    expect(classic.fileStatus?.(renamed[0] as HTMLElement)).toBe('modified');
  });

  it('ignores a title that does not end with the path', () => {
    const container = onlyFile('classic-renamed.html');
    container.querySelector('.file-info a[title]')?.setAttribute('title', 'store/names.go → store/other.go');
    expect(classic.filePath(container)).toEqual({ path: 'store/naming.go' });
  });
});

describe('added and deleted files', () => {
  it('reads an added file', () => {
    const container = onlyFile('classic-added.html');
    expect(classic.fileStatus?.(container)).toBe('added');
    const added = shape(classic.rows(container));
    expect(added[0]).toBe('hunk - -');
    expect(added.slice(1)).toEqual(Array.from({ length: 15 }, (_, i) => `add - ${i + 1}`));
  });

  it('reads a deleted file', () => {
    const container = onlyFile('classic-deleted.html');
    expect(classic.fileStatus?.(container)).toBe('deleted');
    const deleted = shape(classic.rows(container));
    expect(deleted.slice(1)).toEqual(Array.from({ length: 6 }, (_, i) => `del ${i + 1} -`));
  });
});

it('reads a review thread as a comment row', () => {
  const kinds = shape(classic.rows(onlyFile('classic-review-thread.html')));
  const i = kinds.indexOf('add - 15');
  expect(kinds.slice(i, i + 3)).toEqual(['add - 15', 'comment - -', 'add - 16']);
});

it('yields nothing for a split view', () => {
  const container = onlyFile('classic-split.html');
  expect(classic.isSplit?.(container)).toBe(true);
  expect([...classic.rows(container)]).toEqual([]);
  expect(classic.isSplit?.(onlyFile('classic-modified.html'))).toBe(false);
});
