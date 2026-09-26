// Decides which diff rows to hide (plan 6.6). test/content/plan.test.ts
// checks it against the vectors in testvectors/visibility.

import type { ChangeResult, Hit, Range } from '@go-tebanare/engine';

/** The kind of a displayed diff row. Only del, add, and context rows can be hidden. */
export type RowKind = 'context' | 'add' | 'del' | 'hunk' | 'expander' | 'comment';

/** One displayed row. A missing line number means the row has none on that side. */
export interface PlanRow {
  kind: RowKind;
  oldLine?: number;
  newLine?: number;
}

/** A maximal run of consecutive hidden rows. */
export interface Fold {
  /** Index of the first row, inclusive. */
  first: number;
  /** Index of the last row, inclusive. */
  last: number;
  /** Number of rows in the fold. */
  lines: number;
  /** Number of del rows in the fold. */
  deleted: number;
  /** Number of add rows in the fold. */
  added: number;
  /** Ids of the rules that hid the rows, in order of first appearance. */
  rules: string[];
}

export interface Plan {
  /** Indices of the hidden rows in ascending order. */
  hidden: number[];
  folds: Fold[];
}

/** The part of ChangeResult that planVisibility reads. */
export type PlanResult = Pick<ChangeResult, 'old' | 'new'>;

/**
 * planVisibility decides which rows to hide for result.
 *
 * A del row is hidden when its old line is in result.old, and an add row
 * when its new line is in result.new. A context row is hidden only when
 * both lines are. Rows of other kinds stay visible and end the current fold.
 * The rules of a fold come from the hits of the ranges that contain each
 * hidden row's line: the old side for del rows, the new side for add rows,
 * and the old side and then the new side for context rows.
 *
 * result.old and result.new must be sorted and free of overlaps, as the
 * engine returns them.
 */
export function planVisibility(rows: readonly PlanRow[], result: PlanResult): Plan {
  const plan: Plan = { hidden: [], folds: [] };
  let fold: Fold | undefined;
  rows.forEach((row, i) => {
    const hits = rowHits(row, result);
    if (!hits) {
      fold = undefined;
      return;
    }
    plan.hidden.push(i);
    if (!fold) {
      fold = { first: i, last: i, lines: 0, deleted: 0, added: 0, rules: [] };
      plan.folds.push(fold);
    }
    fold.last = i;
    fold.lines++;
    if (row.kind === 'del') fold.deleted++;
    if (row.kind === 'add') fold.added++;
    for (const h of [...hits.old, ...hits.new]) {
      if (!fold.rules.includes(h.ruleId)) fold.rules.push(h.ruleId);
    }
  });
  return plan;
}

/** The hits of the ranges that hide one row, per side. */
export interface RowHits {
  old: Hit[];
  new: Hit[];
}

/**
 * rowHits returns the hits of the ranges that hide row, or null when the
 * row stays visible.
 */
export function rowHits(row: PlanRow, result: PlanResult): RowHits | null {
  switch (row.kind) {
    case 'del': {
      const r = findRange(result.old, row.oldLine);
      return r ? { old: r.hits, new: [] } : null;
    }
    case 'add': {
      const r = findRange(result.new, row.newLine);
      return r ? { old: [], new: r.hits } : null;
    }
    case 'context': {
      const o = findRange(result.old, row.oldLine);
      const n = findRange(result.new, row.newLine);
      return o && n ? { old: o.hits, new: n.hits } : null;
    }
    default:
      return null;
  }
}

/**
 * findRange returns the entry of ranges that contains line, found with a
 * binary search. ranges must be sorted and free of overlaps. A missing
 * line, or one that is not a positive integer, is in no range.
 */
export function findRange(ranges: readonly Range[], line: number | undefined): Range | undefined {
  // The integer check also rejects NaN, which would compare as inside every range.
  if (line === undefined || !Number.isInteger(line) || line < 1) return undefined;
  let lo = 0;
  let hi = ranges.length;
  while (lo < hi) {
    const mid = (lo + hi) >>> 1;
    const r = ranges[mid];
    if (!r) return undefined;
    if (r.end < line) lo = mid + 1;
    else if (r.start > line) hi = mid;
    else return r;
  }
  return undefined;
}
