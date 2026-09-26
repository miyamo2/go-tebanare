// A DiffUiVariant reads one flavor of GitHub's "Files changed" markup (plan
// 6.6). Everything that depends on GitHub's DOM lives behind this interface,
// so a markup change stays inside one variant module.

import type { RowKind } from '../plan.js';
import { classicVariant } from './classic.js';
import { reactVariant } from './react.js';

export type { RowKind };

/** One displayed diff row. Line numbers are 1-based and absent on the side a row lacks. */
export interface RowRef {
  el: HTMLElement;
  kind: RowKind;
  oldLine?: number;
  newLine?: number;
  /** The code on the row without the +/- marker, for the line hash check. */
  text?: string;
  split?: { left?: HTMLElement; right?: HTMLElement };
}

/** The file status as far as the page shows it. A rename with changes is "modified". */
export type FileStatus = 'added' | 'deleted' | 'modified';

export interface DiffUiVariant {
  id: string;
  detect(doc: Document): boolean;
  /** Yields the containers of files whose diff table is in the DOM. */
  fileContainers(root: ParentNode): Iterable<HTMLElement>;
  /** Returns the path on the new side and, for a rename, the old path. The path is "" when the page shows none. */
  filePath(container: HTMLElement): { path: string; oldPath?: string };
  /** Yields the rows in display order, or nothing when the markup is not understood. */
  rows(container: HTMLElement): Iterable<RowRef>;

  // Members below extend the interface of plan 6.6. They are optional so a
  // variant written against plan 6.6 alone still type-checks.

  /** Returns the element that shows the file name; the file badge goes at its end. */
  fileHeader?(container: HTMLElement): HTMLElement | null;
  /** Reports a split (side-by-side) diff, which rows() does not read yet (plan Phase 4). */
  isSplit?(container: HTMLElement): boolean;
  /** Tells whether the file is added or deleted, so the caller fetches only the sides that exist. */
  fileStatus?(container: HTMLElement): FileStatus;
}

/** The known variants, tried in order. */
export const variants: readonly DiffUiVariant[] = [classicVariant, reactVariant];

/** detectVariant returns the first variant that recognizes doc, or null. */
export function detectVariant(doc: Document, registry: readonly DiffUiVariant[] = variants): DiffUiVariant | null {
  for (const v of registry) {
    if (v.detect(doc)) return v;
  }
  return null;
}
