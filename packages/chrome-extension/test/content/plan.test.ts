import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import type { ChangeResult, Hit } from '@go-tebanare/engine';
import { findRange, planVisibility, rowHits, type Plan, type PlanRow, type RowKind } from '../../src/content/plan.js';

const vectorDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', '..', '..', 'testvectors', 'visibility');

interface Vector {
  name: string;
  rows: { kind: RowKind; old?: number; new?: number }[];
  result: ChangeResult;
  want: Plan;
}

function toPlanRow(r: Vector['rows'][number]): PlanRow {
  const row: PlanRow = { kind: r.kind };
  if (r.old !== undefined) row.oldLine = r.old;
  if (r.new !== undefined) row.newLine = r.new;
  return row;
}

const files = readdirSync(vectorDir).filter((f) => f.endsWith('.json')).sort();

describe('planVisibility test vectors', () => {
  it('finds the vectors', () => {
    expect(files.length).toBeGreaterThanOrEqual(9);
  });

  it.each(files)('%s', (file) => {
    const v = JSON.parse(readFileSync(join(vectorDir, file), 'utf8')) as Vector;
    expect(planVisibility(v.rows.map(toPlanRow), v.result), v.name).toEqual(v.want);
  });
});

const hit = (ruleId: string): Hit => ({ ruleId, target: 'func', node: 'FuncDecl', label: `func ${ruleId}` });

describe('planVisibility', () => {
  const result = { old: [{ start: 1, end: 5, hits: [hit('a')] }], new: [{ start: 1, end: 5, hits: [hit('b')] }] };

  it('keeps rows with malformed line numbers visible', () => {
    const rows: PlanRow[] = [
      { kind: 'del', oldLine: Number.NaN },
      { kind: 'del', oldLine: 2.5 },
      { kind: 'del', oldLine: -1 },
      { kind: 'del', oldLine: 0 },
      { kind: 'add' },
      { kind: 'context', oldLine: 2 },
    ];
    expect(planVisibility(rows, result)).toEqual({ hidden: [], folds: [] });
  });

  it('accepts rows in any line order', () => {
    const rows: PlanRow[] = [
      { kind: 'add', newLine: 4 },
      { kind: 'del', oldLine: 1 },
    ];
    expect(planVisibility(rows, result)).toEqual({
      hidden: [0, 1],
      folds: [{ first: 0, last: 1, lines: 2, deleted: 1, added: 1, rules: ['b', 'a'] }],
    });
  });
});

describe('rowHits', () => {
  it('returns the hits of each side for a context row', () => {
    const result = { old: [{ start: 3, end: 3, hits: [hit('a')] }], new: [{ start: 4, end: 4, hits: [hit('b')] }] };
    expect(rowHits({ kind: 'context', oldLine: 3, newLine: 4 }, result)).toEqual({ old: [hit('a')], new: [hit('b')] });
    expect(rowHits({ kind: 'context', oldLine: 3, newLine: 5 }, result)).toBeNull();
    expect(rowHits({ kind: 'hunk', oldLine: 3, newLine: 4 }, result)).toBeNull();
  });
});

describe('findRange', () => {
  const ranges = [
    { start: 2, end: 3, hits: [] },
    { start: 4, end: 4, hits: [] },
    { start: 10, end: 20, hits: [] },
  ];

  it.each([
    [1, undefined],
    [2, 0],
    [3, 0],
    [4, 1],
    [5, undefined],
    [10, 2],
    [15, 2],
    [20, 2],
    [21, undefined],
  ])('line %d => range %s', (line, want) => {
    expect(findRange(ranges, line)).toBe(want === undefined ? undefined : ranges[want]);
  });

  it('finds nothing in an empty list', () => {
    expect(findRange([], 1)).toBeUndefined();
  });
});
