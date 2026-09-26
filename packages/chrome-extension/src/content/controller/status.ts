// Collects what the controller reports: the banner notices and the counts
// in PageStatus (plan 6.7).

import type { PageStatus } from '../../shared/messages.js';
import type { ApplySummary } from '../apply.js';
import { noticeMessages, type BannerMessage, type Notice } from '../ui/banner.js';

/** inactiveStatus is the status of a page without a controller. */
export function inactiveStatus(): PageStatus {
  return {
    state: 'inactive',
    configSource: 'base',
    rules: 0,
    files: 0,
    filesWithFolds: 0,
    linesHidden: 0,
    messages: [],
    enabled: true,
    headPreview: false,
  };
}

interface FileTally {
  analyzed: boolean;
  summary: ApplySummary;
}

const NONE: ApplySummary = { folds: 0, lines: 0 };

/**
 * Report holds the notices of one run: page notices in the order they were
 * added (each kind once) and at most one notice per file, plus the counts
 * of each file.
 */
export class Report {
  #page: Notice[] = [];
  readonly #files = new Map<string, Notice>();
  readonly #tallies = new Map<string, FileTally>();

  /** addPage adds a page notice unless one of the same kind is there. */
  addPage(n: Notice): void {
    if (!this.#page.some((p) => p.kind === n.kind)) this.#page.push(n);
  }

  /** dropPage removes the page notice of kind. */
  dropPage(kind: Notice['kind']): void {
    this.#page = this.#page.filter((p) => p.kind !== kind);
  }

  /** setFile replaces the notice of the file key; undefined removes it. */
  setFile(key: string, n: Notice | undefined): void {
    if (n) this.#files.set(key, n);
    else this.#files.delete(key);
  }

  /** tally records whether the file key has an analysis and what is hidden in it now. */
  tally(key: string, analyzed: boolean, summary: ApplySummary = NONE): void {
    this.#tallies.set(key, { analyzed, summary });
  }

  /** hideNothing sets every file's summary to zero, for when hiding is turned off. */
  hideNothing(): void {
    for (const t of this.#tallies.values()) t.summary = NONE;
  }

  messages(): BannerMessage[] {
    return [...this.#page, ...this.#files.values()].flatMap(noticeMessages);
  }

  counts(): Pick<PageStatus, 'files' | 'filesWithFolds' | 'linesHidden'> {
    let files = 0;
    let filesWithFolds = 0;
    let linesHidden = 0;
    for (const t of this.#tallies.values()) {
      if (t.analyzed) files++;
      if (t.summary.folds > 0) filesWithFolds++;
      linesHidden += t.summary.lines;
    }
    return { files, filesWithFolds, linesHidden };
  }
}
