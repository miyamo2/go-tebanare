// FileScanner handles the files of one controller run once the config is
// compiled and the diff UI is known (plan 6.2 step 5, 6.6). It analyzes each
// rendered Go file once and shows the result in every container that
// displays the file, again whenever GitHub re-renders it.

import type { RuleInfo } from '@go-tebanare/engine';
import { errorText, type ConfigSource } from '../../shared/messages.js';
import { clearFile } from '../apply.js';
import { isConfigPath } from '../config.js';
import type { DiffUiVariant, RowRef } from '../dom/variant.js';
import type { Semaphore, SourceFetcher } from '../fetcher.js';
import { analyzeFile, fileKey, isGoFile, rowsKey, showFile, type DiffFile, type FileAnalysis, type RunConfig, type SendFn } from './file.js';
import type { Report } from './status.js';
import { touches, type Changes } from './watch.js';

/** A compiled config with its rules. */
export interface Compiled extends RunConfig {
  rules: readonly RuleInfo[];
}

export interface ScanContext {
  doc: Document;
  send: SendFn;
  fetcher: SourceFetcher;
  /** Limits the files analyzed at once; shared by the runs of one controller. */
  limit: Semaphore;
  variant: DiffUiVariant;
  config: Compiled;
  source: ConfigSource;
  report: Report;
  debug: boolean;
  enabled(): boolean;
  /** Called after a file changes what the banner shows. */
  changed(): void;
}

/** The last rows a container was shown with. */
interface Seen {
  key: string;
  rows: readonly RowRef[];
}

const sameElements = (a: readonly RowRef[], b: readonly RowRef[]) => a.length === b.length && a.every((r, i) => r.el === b[i]?.el);

export class FileScanner {
  readonly #c: ScanContext;
  #stopped = false;
  readonly #analyses = new Map<string, Promise<FileAnalysis>>();
  // Rows the reader opened, per file, so they stay open when GitHub re-renders the file.
  readonly #expanded = new Map<string, Set<string>>();
  #seen = new WeakMap<HTMLElement, Seen>();
  // Containers the scanner showed a file in, with the key of that file.
  readonly #touched = new Map<HTMLElement, string>();
  // Visits waiting for their file's analysis.
  readonly #pending = new Set<Seen>();

  constructor(c: ScanContext) {
    this.#c = c;
  }

  /** scan visits the file containers that changes touched, or every one without changes. */
  scan(changes?: Changes): void {
    if (this.#stopped) return;
    for (const container of this.#c.variant.fileContainers(this.#c.doc)) {
      if (!changes || touches(changes, container)) this.#visit(container);
    }
  }

  /** busy reports whether a file on the page still waits for its analysis. */
  busy(): boolean {
    return this.#pending.size > 0;
  }

  /** refresh shows every file again after enabled changed. Turning hiding off clears every file at once. */
  refresh(): void {
    if (!this.#c.enabled()) {
      for (const c of this.#touched.keys()) clearFile(c);
      this.#c.report.hideNothing();
    }
    this.#seen = new WeakMap();
    this.scan();
  }

  /** stop drops pending work and removes what the scanner added to the page. */
  stop(): void {
    this.#stopped = true;
    for (const c of this.#touched.keys()) clearFile(c);
    this.#touched.clear();
  }

  // visit skips a container whose rows are the same elements with the same
  // kinds, line numbers, and text as last time (see rowsKey). A new or
  // re-rendered container, or a row whose code changed, is shown again from
  // the file's analysis, which checks the text again. A container that can
  // no longer be read is cleared, so nothing stays hidden (fail open).
  #visit(container: HTMLElement): void {
    const { variant, report, source } = this.#c;
    const { path, oldPath } = variant.filePath(container);
    if (path === '') return this.#forget(container);
    // Plan 6.5: a config file in the diff is never analyzed, so never hidden.
    const configPath = [path, oldPath].find((p) => p !== undefined && isConfigPath(p));
    if (configPath !== undefined && source === 'base') report.addPage({ kind: 'config-changed', path: configPath });
    if (variant.isSplit?.(container)) {
      report.addPage({ kind: 'split-view' });
      return this.#forget(container);
    }
    const file: DiffFile = { path, status: variant.fileStatus?.(container) ?? 'modified' };
    if (oldPath !== undefined) file.oldPath = oldPath;
    if (!isGoFile(file)) return this.#forget(container);
    const rows = [...variant.rows(container)];
    if (rows.length === 0) return this.#forget(container);
    const seen: Seen = { key: `${fileKey(file)}\n${rowsKey(rows)}`, rows };
    const last = this.#seen.get(container);
    if (last && last.key === seen.key && sameElements(last.rows, rows)) return;
    this.#seen.set(container, seen);
    this.#pending.add(seen);
    void this.#analysis(file).then((analysis) => {
      this.#pending.delete(seen);
      if (this.#stopped) return;
      // A dropped visit can be the last one the banner waits for.
      if (this.#seen.get(container) !== seen) return this.#c.changed();
      try {
        this.#show(container, file, rows, analysis);
      } catch (e) {
        console.warn(`go-tebanare: ${path}: ${errorText(e)}`);
        clearFile(container);
        this.#c.changed();
      }
    });
  }

  // forget clears a container the scanner can no longer read: it drops a
  // pending show, removes what the scanner added, and stops counting the
  // lines it hid.
  #forget(container: HTMLElement): void {
    this.#seen.delete(container);
    const key = this.#touched.get(container);
    if (key === undefined) return;
    this.#touched.delete(container);
    clearFile(container);
    this.#c.report.tally(key, false);
    this.#c.changed();
  }

  #analysis(file: DiffFile): Promise<FileAnalysis> {
    const key = fileKey(file);
    let p = this.#analyses.get(key);
    if (!p) {
      const { send, fetcher, config, limit } = this.#c;
      p = limit
        .run(() => analyzeFile(send, fetcher, config, file, () => this.#stopped))
        .catch((e: unknown): FileAnalysis => ({ record: null, notice: { kind: 'analysis-failed', path: file.path, error: errorText(e) } }));
      this.#analyses.set(key, p);
    }
    return p;
  }

  #show(container: HTMLElement, file: DiffFile, rows: readonly RowRef[], analysis: FileAnalysis): void {
    const { report, variant } = this.#c;
    const key = fileKey(file);
    this.#touched.set(container, key);
    let notice = analysis.notice;
    if (!analysis.record) {
      clearFile(container);
      report.tally(key, false);
    } else {
      let expanded = this.#expanded.get(key);
      if (!expanded) this.#expanded.set(key, (expanded = new Set()));
      const shown = showFile(container, rows, analysis.record, this.#c.enabled(), {
        rules: this.#c.config.rules,
        expanded,
        debug: this.#c.debug,
        badgeHost: variant.fileHeader?.(container) ?? null,
        onChange: (summary) => {
          if (!this.#stopped) report.tally(key, true, summary);
        },
      });
      report.tally(key, true, shown.verified ? shown.summary : undefined);
      if (!shown.verified) notice = { kind: 'source-mismatch', path: file.path };
    }
    report.setFile(key, notice);
    this.#c.changed();
  }
}
