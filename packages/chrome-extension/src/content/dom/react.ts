// The React variant reads the diff markup of GitHub's React "Files
// changed" page: a div[role="region"] per file whose id is "diff-" and the
// SHA-256 of the path, and a table with the same id in data-diff-anchor
// that has one tr.diff-line-row per displayed line.
//
// The selectors come from a saved page (test/fixtures/react-modified.html),
// whose rows match the diff lines in the JSON of the same page. That page
// shows modified files in unified view with context lines, added lines,
// hunk headers, and the expander row at the end.
//
// S2: deleted lines, added, deleted, and renamed files, review threads, and
// split view were not on the saved page. The markup assumed for them is
// marked below; where it is wrong, rows returns nothing for the file.

import { FOLD_ATTR } from '../ui/fold.js';
import type { DiffUiVariant, FileStatus, RowRef } from './variant.js';

const DETECT = '[role="region"][id^="diff-"] table[data-diff-anchor^="diff-"]';

// Directional marks that GitHub puts around the path in the file header.
const MARKS = /[‎‏]/g;

// S2: assumed to match classic, where a renamed file shows the old path,
// " ", U+2192, " ", and the new path.
const RENAME_SEPARATOR = ' → ';

const HUNK_HEADER = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/;

function diffTable(container: HTMLElement): HTMLTableElement | null {
  for (const table of container.querySelectorAll<HTMLTableElement>('table[data-diff-anchor]')) {
    if (table.getAttribute('data-diff-anchor') === container.id) return table;
  }
  return null;
}

// heading returns the file name heading, which labels the region.
function heading(container: HTMLElement): HTMLElement | null {
  const id = container.getAttribute('aria-labelledby');
  if (!id) return null;
  for (const el of container.querySelectorAll<HTMLElement>('h3[id]')) {
    if (el.id === id) return el;
  }
  return null;
}

function isContainer(el: Element): el is HTMLElement {
  return el instanceof HTMLElement && el.matches('[role="region"][id^="diff-"]') && diffTable(el) !== null;
}

function fileContainers(root: ParentNode): HTMLElement[] {
  const out: HTMLElement[] = [];
  if (root instanceof HTMLElement && isContainer(root)) out.push(root);
  for (const el of root.querySelectorAll('[role="region"][id^="diff-"]')) {
    if (isContainer(el)) out.push(el);
  }
  return out;
}

// filePath reads the heading. The "Expand all lines" button in the header
// carries the path in data-file-path; when it is there it must name the
// same file, or the path is "".
function filePath(container: HTMLElement): { path: string; oldPath?: string } {
  const text = (heading(container)?.textContent ?? '').replace(MARKS, '').trim();
  const at = text.indexOf(RENAME_SEPARATOR);
  const path = at < 0 ? text : text.slice(at + RENAME_SEPARATOR.length);
  const button = container.querySelector('[data-diff-header-wrapper] [data-file-path]')?.getAttribute('data-file-path') ?? null;
  if (path === '' || (button !== null && button !== path)) return { path: '' };
  return at < 0 ? { path } : { path, oldPath: text.slice(0, at) };
}

// lineNumber returns undefined for a cell without a number and NaN for a
// malformed one.
function lineNumber(cell: Element): number | undefined {
  const raw = cell.getAttribute('data-line-number');
  if (raw === null) return undefined;
  return /^[1-9][0-9]*$/.test(raw) ? Number(raw) : NaN;
}

// S2: deleted lines are assumed to carry the class "deletion", the way
// added lines carry "addition".
function codeKind(code: Element): 'add' | 'del' | 'context' {
  if (code.classList.contains('addition')) return 'add';
  if (code.classList.contains('deletion')) return 'del';
  return 'context';
}

// readHunk reads a row with a hunk cell: a header "@@ ... @@" is a hunk,
// and the empty cell after the last hunk is an expander.
function readHunk(tr: HTMLElement, cell: Element): RowRef | null {
  if (tr.children.length !== 1) return null;
  const text = (cell.querySelector('.diff-text-inner')?.textContent ?? '').trim();
  if (text === '') return { el: tr, kind: 'expander' };
  return HUNK_HEADER.test(text) ? { el: tr, kind: 'hunk' } : null;
}

// readRow returns null for a row it does not understand. A line row has an
// old number cell, a new number cell, and the code cell. A number cell
// without a number (an added line's old side) has no data-diff-side. The
// cells may hold no text besides the numbers, the +/- marker, and the code:
// S2 does not know yet where review threads go, and a thread inside a row
// must not be hidden with it.
function readRow(tr: HTMLElement): RowRef | null {
  if (!tr.classList.contains('diff-line-row')) return null;
  const hunk = tr.querySelector(':scope > td.diff-hunk-cell');
  if (hunk) return readHunk(tr, hunk);

  const [left, right, cell, ...rest] = tr.children;
  if (!left || !right || !cell || rest.length > 0 || !cell.matches('td.diff-text-cell')) return null;
  if (left.matches('.diff-text-cell') || right.matches('.diff-text-cell')) return null;
  const leftSide = left.getAttribute('data-diff-side');
  const rightSide = right.getAttribute('data-diff-side');
  if ((leftSide !== null && leftSide !== 'left') || (rightSide !== null && rightSide !== 'right')) return null;
  const code = cell.querySelector(':scope > code.diff-text');
  if (!code) return null;
  const kind = codeKind(code);
  const oldLine = lineNumber(left);
  const newLine = lineNumber(right);
  const hasOld = oldLine !== undefined;
  const hasNew = newLine !== undefined;
  const sidesOk = kind === 'add' ? !hasOld && hasNew : kind === 'del' ? hasOld && !hasNew : hasOld && hasNew;
  if (!sidesOk || Number.isNaN(oldLine) || Number.isNaN(newLine)) return null;

  // The +/- marker is a span before .diff-text-inner, so the text is the code alone.
  const marker = code.querySelector(':scope > .diff-text-marker')?.textContent ?? '';
  const text = code.querySelector(':scope > .diff-text-inner')?.textContent;
  if (left.textContent !== (hasOld ? String(oldLine) : '') || right.textContent !== (hasNew ? String(newLine) : '')) return null;
  if (cell.textContent !== marker + (text ?? '')) return null;
  const row: RowRef = { el: tr, kind };
  if (hasOld) row.oldLine = oldLine;
  if (hasNew) row.newLine = newLine;
  if (text !== undefined) row.text = text;
  return row;
}

function tableRows(table: HTMLTableElement): HTMLElement[] {
  return [...table.querySelectorAll<HTMLElement>(':scope > tbody > tr')];
}

// S2: split view is assumed to show a code cell for each side on one row.
function isSplit(container: HTMLElement): boolean {
  const table = diffTable(container);
  return table ? tableRows(table).some((tr) => tr.querySelectorAll(':scope > td.diff-text-cell').length > 1) : false;
}

// rows returns nothing for a split table or when any row is not understood,
// so a markup change hides nothing in the file. S2: the saved page had no
// review threads, so a row that a thread adds is not understood either.
function rows(container: HTMLElement): RowRef[] {
  const table = diffTable(container);
  if (!table || isSplit(container)) return [];
  const out: RowRef[] = [];
  for (const tr of tableRows(table)) {
    // Fold rows are the extension's own (apply.ts); they are not diff rows.
    if (tr.hasAttribute(FOLD_ATTR)) continue;
    const row = readRow(tr);
    if (!row) return [];
    out.push(row);
  }
  return out;
}

// fileHeader returns the section that holds the heading and the path
// buttons, so the badge goes after them.
function fileHeader(container: HTMLElement): HTMLElement | null {
  return heading(container)?.parentElement ?? null;
}

// S2: the header shows no status that the saved page could confirm, so the
// first hunk header decides, as in classic: "@@ -0,0" means added and
// "+0,0" means deleted.
function fileStatus(container: HTMLElement): FileStatus {
  const table = diffTable(container);
  const hunk = table?.querySelector('td.diff-hunk-cell .diff-text-inner')?.textContent ?? '';
  const m = HUNK_HEADER.exec(hunk.trim());
  if (m?.[1] === '0' && m[2] === '0') return 'added';
  if (m?.[3] === '0' && m[4] === '0') return 'deleted';
  return 'modified';
}

export const reactVariant: DiffUiVariant = {
  id: 'react',
  detect: (doc) => doc.querySelector(DETECT) !== null,
  fileContainers,
  filePath,
  rows,
  fileHeader,
  isSplit,
  fileStatus,
};
