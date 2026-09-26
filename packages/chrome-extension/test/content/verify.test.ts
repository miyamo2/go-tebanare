import { describe, expect, it } from 'vitest';
import { lineHash } from '../../src/shared/hash.js';
import { planVisibility } from '../../src/content/plan.js';
import { verifyHiddenRows, type VerifyRow } from '../../src/content/verify.js';

const oldSrc = ['package p', '', 'func (u *User) Name() string {', '\treturn u.name', '}'];
const newSrc = ['package p', '', '// Name returns the name.', 'func (u *User) Name() string {', '\treturn u.name', '}'];

/** hashes returns lineHash of the given 1-based lines of src, keyed by line number. */
function hashes(src: string[], lines: number[]): Record<string, string> {
  return Object.fromEntries(lines.map((n) => [String(n), lineHash(src[n - 1] ?? '')]));
}

const record = { oldLines: hashes(oldSrc, [3, 4, 5]), newLines: hashes(newSrc, [3, 4, 5, 6]) };
const result = {
  old: [{ start: 3, end: 5, hits: [] }],
  new: [{ start: 3, end: 6, hits: [] }],
};

function rows(): VerifyRow[] {
  return [
    { kind: 'hunk' },
    { kind: 'context', oldLine: 1, newLine: 1, text: 'package p' },
    { kind: 'context', oldLine: 2, newLine: 2, text: '' },
    { kind: 'add', newLine: 3, text: '// Name returns the name.' },
    { kind: 'context', oldLine: 3, newLine: 4, text: 'func (u *User) Name() string {' },
    { kind: 'del', oldLine: 4, text: '\treturn u.name' },
    { kind: 'add', newLine: 5, text: '\treturn u.name  \r' },
    { kind: 'context', oldLine: 5, newLine: 6, text: '}' },
  ];
}

describe('verifyHiddenRows', () => {
  it('accepts rows whose text matches the analyzed source', () => {
    const r = rows();
    const plan = planVisibility(r, result);
    expect(plan.hidden).toEqual([3, 4, 5, 6, 7]);
    expect(verifyHiddenRows(r, plan, record)).toBe(true);
  });

  it('ignores visible rows', () => {
    const r = rows();
    const plan = planVisibility(r, result);
    r[1] = { kind: 'context', oldLine: 1, newLine: 1, text: 'package q' };
    r[2] = { kind: 'context', oldLine: 2, newLine: 2 };
    expect(verifyHiddenRows(r, plan, record)).toBe(true);
  });

  it.each([
    ['a del row text differs', 5, { kind: 'del', oldLine: 4, text: '\treturn u.Name' }],
    ['an add row text differs', 3, { kind: 'add', newLine: 3, text: '// Name returns a name.' }],
    ['leading indentation differs', 5, { kind: 'del', oldLine: 4, text: '    return u.name' }],
    ['a row has no text', 7, { kind: 'context', oldLine: 5, newLine: 6 }],
    ['a del row has no hash', 5, { kind: 'del', oldLine: 9, text: '\treturn u.name' }],
    ['an add row has no line number', 6, { kind: 'add', text: '\treturn u.name' }],
  ] as [string, number, VerifyRow][])('rejects when %s', (_, index, row) => {
    const r = rows();
    const plan = planVisibility(r, result);
    r[index] = row;
    expect(verifyHiddenRows(r, plan, record)).toBe(false);
  });

  it('checks both sides of a context row', () => {
    const r = rows();
    const plan = planVisibility(r, result);
    expect(verifyHiddenRows(r, plan, { oldLines: record.oldLines, newLines: { ...record.newLines, '4': lineHash('x') } })).toBe(false);
    expect(verifyHiddenRows(r, plan, { oldLines: { ...record.oldLines, '3': lineHash('x') }, newLines: record.newLines })).toBe(false);
    const { '6': _, ...withoutLast } = record.newLines;
    expect(verifyHiddenRows(r, plan, { oldLines: record.oldLines, newLines: withoutLast })).toBe(false);
  });

  it('rejects hidden indices outside the rows or on rows that cannot hide', () => {
    expect(verifyHiddenRows(rows(), { hidden: [99] }, record)).toBe(false);
    expect(verifyHiddenRows(rows(), { hidden: [0] }, record)).toBe(false);
  });

  it('does not read inherited members of the hash maps', () => {
    const r: VerifyRow[] = [{ kind: 'del', oldLine: Number.NaN, text: 'x' }];
    const lines = Object.create({ NaN: lineHash('x') }) as Record<string, string>;
    expect(verifyHiddenRows(r, { hidden: [0] }, { oldLines: lines, newLines: {} })).toBe(false);
  });

  it('accepts a plan that hides nothing', () => {
    expect(verifyHiddenRows([], { hidden: [] }, { oldLines: {}, newLines: {} })).toBe(true);
  });
});
