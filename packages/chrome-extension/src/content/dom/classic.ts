// The classic variant reads GitHub's server-rendered diff markup
// (table.diff-table).
//
// Every selector and attribute here comes from synthetic fixtures
// (test/fixtures/classic-*.html) modeled on the classic markup and checked
// against a saved "Files changed" page. The fixtures themselves stay
// synthetic; they are the regression tests, not a copy of the saved page.

import { FOLD_ATTR } from '../ui/fold.js';
import type { DiffUiVariant, FileStatus, RowRef } from './variant.js';

const DETECT = '#files .file .diff-table, .js-diff-progressive-container .file .diff-table';

// The link title of a renamed file is the old path, " ", U+2192, " ", and
// the new path.
const RENAME_SEPARATOR = ' \u2192 ';

// Hunk headers look like "@@ -0,0 +1,12 @@" for an added file.
const HUNK_HEADER = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/;

function diffTable(container: HTMLElement): HTMLTableElement | null {
  return container.querySelector<HTMLTableElement>('table.diff-table');
}

function header(container: HTMLElement): HTMLElement | null {
  return container.querySelector<HTMLElement>('.file-header[data-path]');
}

function fileContainers(root: ParentNode): HTMLElement[] {
  const out: HTMLElement[] = [];
  if (root instanceof HTMLElement && root.matches('.file') && diffTable(root)) out.push(root);
  for (const el of root.querySelectorAll<HTMLElement>('.file')) {
    if (diffTable(el)) out.push(el);
  }
  return out;
}

function filePath(container: HTMLElement): { path: string; oldPath?: string } {
  const path = header(container)?.getAttribute('data-path') ?? '';
  if (path === '') return { path };
  const title = container.querySelector('.file-header .file-info a[title]')?.getAttribute('title') ?? '';
  const suffix = RENAME_SEPARATOR + path;
  if (title.length > suffix.length && title.endsWith(suffix)) {
    return { path, oldPath: title.slice(0, -suffix.length) };
  }
  return { path };
}

// lineNumber returns undefined for a cell without a number and NaN for a
// malformed one.
function lineNumber(cell: Element | undefined): number | undefined {
  const raw = cell?.getAttribute('data-line-number');
  if (raw === null || raw === undefined) return undefined;
  return /^[1-9][0-9]*$/.test(raw) ? Number(raw) : NaN;
}

function codeKind(code: Element): 'add' | 'del' | 'context' | null {
  if (code.classList.contains('blob-code-addition')) return 'add';
  if (code.classList.contains('blob-code-deletion')) return 'del';
  if (code.classList.contains('blob-code-context')) return 'context';
  return null;
}

// readRow returns null for a row it does not understand.
function readRow(tr: HTMLElement): RowRef | null {
  if (tr.classList.contains('inline-comments')) return { el: tr, kind: 'comment' };
  if (tr.querySelector(':scope > td.blob-code-hunk')) return { el: tr, kind: 'hunk' };
  if (tr.querySelector(':scope > td.blob-num-expandable')) return { el: tr, kind: 'expander' };

  const nums = tr.querySelectorAll(':scope > td.blob-num');
  const code = tr.querySelector(':scope > td.blob-code');
  if (nums.length !== 2 || !code) return null;
  const kind = codeKind(code);
  if (!kind) return null;
  const oldLine = lineNumber(nums[0]);
  const newLine = lineNumber(nums[1]);
  const hasOld = oldLine !== undefined;
  const hasNew = newLine !== undefined;
  const sidesOk = kind === 'add' ? !hasOld && hasNew : kind === 'del' ? hasOld && !hasNew : hasOld && hasNew;
  if (!sidesOk || Number.isNaN(oldLine) || Number.isNaN(newLine)) return null;

  // The +/- marker lives in data-code-marker, so the text is the code alone.
  const inner = code.classList.contains('blob-code-inner') ? code : code.querySelector('.blob-code-inner');
  const row: RowRef = { el: tr, kind };
  if (hasOld) row.oldLine = oldLine;
  if (hasNew) row.newLine = newLine;
  if (inner) row.text = inner.textContent ?? '';
  return row;
}

function isSplit(container: HTMLElement): boolean {
  return diffTable(container)?.classList.contains('file-diff-split') ?? false;
}

// rows returns nothing for a split table or when any row is not understood,
// so a markup change hides nothing in the file.
function rows(container: HTMLElement): RowRef[] {
  const table = diffTable(container);
  if (!table || table.classList.contains('file-diff-split')) return [];
  const out: RowRef[] = [];
  for (const tr of table.querySelectorAll<HTMLElement>(':scope > tbody > tr')) {
    // Fold rows are the extension's own (apply.ts); they are not diff rows.
    if (tr.hasAttribute(FOLD_ATTR)) continue;
    const row = readRow(tr);
    if (!row) return [];
    out.push(row);
  }
  return out;
}

function fileHeader(container: HTMLElement): HTMLElement | null {
  return container.querySelector<HTMLElement>('.file-header .file-info') ?? header(container);
}

// fileStatus trusts data-file-deleted on the header when it is there, so a
// file emptied in place stays "modified" and both sides are analyzed.
//
// The header carries no attribute for added files, so the first hunk header
// decides: "@@ -0,0" means added. A file that was empty before the change
// shows the same hunk and is reported as added too. The deleted guess from
// "+0,0" applies only when data-file-deleted is missing.
function fileStatus(container: HTMLElement): FileStatus {
  const deleted = header(container)?.getAttribute('data-file-deleted') ?? null;
  if (deleted === 'true') return 'deleted';
  const hunk = diffTable(container)?.querySelector('td.blob-code-hunk')?.textContent ?? '';
  const m = HUNK_HEADER.exec(hunk.trim());
  if (m?.[1] === '0' && m[2] === '0') return 'added';
  if (deleted === null && m?.[3] === '0' && m[4] === '0') return 'deleted';
  return 'modified';
}

export const classicVariant: DiffUiVariant = {
  id: 'classic',
  detect: (doc) => doc.querySelector(DETECT) !== null,
  fileContainers,
  filePath,
  rows,
  fileHeader,
  isSplit,
  fileStatus,
};
