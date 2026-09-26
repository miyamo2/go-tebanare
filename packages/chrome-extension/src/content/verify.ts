// Checks that the rows about to be hidden show the same text as the source
// the engine analyzed (plan 6.4). A mismatch means the page and the fetched
// commits differ, for example when the old side is not the merge base, and
// the caller then hides nothing in the file.

import { lineHash } from '../shared/hash.js';
import type { AnalysisRecord } from '../shared/messages.js';
import type { Plan, PlanRow } from './plan.js';

/** A displayed row with its text, as the DOM variant reads it. */
export interface VerifyRow extends PlanRow {
  text?: string;
}

/**
 * verifyHiddenRows reports whether every hidden row of plan matches record:
 * lineHash(row.text) must equal record.oldLines[oldLine] for del rows and
 * record.newLines[newLine] for add rows. A context row must match both
 * sides. A row without text, a line without a hash, a hidden index outside
 * rows, or any mismatch returns false.
 */
export function verifyHiddenRows(
  rows: readonly VerifyRow[],
  plan: Pick<Plan, 'hidden'>,
  record: Pick<AnalysisRecord, 'oldLines' | 'newLines'>,
): boolean {
  for (const i of plan.hidden) {
    const row = rows[i];
    if (!row || row.text === undefined) return false;
    const hash = lineHash(row.text);
    const matchOld = () => hashAt(record.oldLines, row.oldLine) === hash;
    const matchNew = () => hashAt(record.newLines, row.newLine) === hash;
    switch (row.kind) {
      case 'del':
        if (!matchOld()) return false;
        break;
      case 'add':
        if (!matchNew()) return false;
        break;
      case 'context':
        if (!matchOld() || !matchNew()) return false;
        break;
      default:
        return false;
    }
  }
  return true;
}

function hashAt(lines: Record<string, string>, line: number | undefined): string | undefined {
  if (line === undefined) return undefined;
  const key = String(line);
  // hasOwn keeps inherited members such as "constructor" out.
  return Object.hasOwn(lines, key) ? lines[key] : undefined;
}
