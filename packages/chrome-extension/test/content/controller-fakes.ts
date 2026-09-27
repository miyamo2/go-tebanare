// Fakes for the controller tests: a pull request page assembled from the
// classic fixtures, a fetcher that serves sources matching the page, a fake
// background, and animation frames that run when a test says so.

import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { ChangeResult, Diagnostic, Range } from '@go-tebanare/engine';
import { buildRecord } from '../../src/background/analysis.js';
import type { SendFn } from '../../src/content/controller/file.js';
import type { FrameApi } from '../../src/content/controller/watch.js';
import { classicVariant as classic } from '../../src/content/dom/classic.js';
import type { FetchResult, SourceFetcher } from '../../src/content/fetcher.js';
import type { PullPage } from '../../src/content/page.js';
import { fnv1a32 } from '../../src/shared/hash.js';
import type { AnalysisRecord, BgRequest, TabState } from '../../src/shared/messages.js';

export const BASE = '1405df66cbe219b0bf6355bc3d60361a8376b6b4';
export const HEAD = '1a954628a960aaef81d7b2d4521929579f3541e6';
export const PAGE: PullPage = { owner: 'octo-org', repo: 'octo-repo', number: 7 };
export const PR_KEY = 'octo-org/octo-repo#7';
export const CONFIG = 'version: 1\npresets: [getter]\n';

// A path string: happy-dom replaces the global URL class, which node:fs rejects.
const fixtures = join(dirname(fileURLToPath(import.meta.url)), '..', 'fixtures');

/** fileHtml returns the file containers of a fixture, with each [from, to] replaced in the markup. */
export function fileHtml(name: string, ...replace: [string, string][]): string {
  let html = readFileSync(join(fixtures, name), 'utf8');
  for (const [from, to] of replace) html = html.replaceAll(from, to);
  const doc = new DOMParser().parseFromString(html, 'text/html');
  return [...doc.querySelectorAll('#files .file')].map((el) => el.outerHTML).join('');
}

/** buildPage shows the files on one pull request page and returns the containers with a diff table. */
export function buildPage(...files: string[]): HTMLElement[] {
  document.body.className = '';
  document.body.innerHTML =
    `<input type="hidden" name="comparison_start_oid" value="${BASE}">` +
    `<input type="hidden" name="comparison_end_oid" value="${HEAD}">` +
    `<main><div id="files" class="diff-view"><div class="js-diff-progressive-container">${files.join('')}</div></div></main>`;
  return [...classic.fileContainers(document)];
}

/** pageSource builds the source at sha of the file the page shows at path: shown lines keep their text, others are filler. */
export function pageSource(sha: string, path: string): string | null {
  for (const c of classic.fileContainers(document)) {
    const f = classic.filePath(c);
    const status = classic.fileStatus?.(c) ?? 'modified';
    const old = sha === BASE && status !== 'added' && (f.oldPath ?? f.path) === path;
    const nu = sha === HEAD && status !== 'deleted' && f.path === path;
    if (!old && !nu) continue;
    const lines = new Map<number, string>();
    for (const r of classic.rows(c)) {
      const n = old ? r.oldLine : r.newLine;
      if (n !== undefined) lines.set(n, r.text ?? '');
    }
    const max = Math.max(0, ...lines.keys());
    return Array.from({ length: max }, (_, i) => lines.get(i + 1) ?? `// line ${i + 1}`).join('\n') + '\n';
  }
  return null;
}

const side = (sha: string) => (sha === BASE ? 'base' : sha === HEAD ? 'head' : sha);

/**
 * FakeFetcher serves files set in files, then the sources of the page, and
 * answers not-found otherwise. calls lists "base:<path>" or "head:<path>".
 */
export class FakeFetcher implements SourceFetcher {
  readonly calls: string[] = [];
  readonly files = new Map<string, FetchResult>();
  #gate: Promise<void> | null = null;
  #held: (path: string) => boolean = () => false;
  #open: () => void = () => {};

  set(sha: string, path: string, res: FetchResult | string): void {
    this.files.set(`${sha}:${path}`, typeof res === 'string' ? { ok: true, text: res } : res);
  }

  /** hold makes responses for the paths that match wait until release(). */
  hold(match: (path: string) => boolean = () => true): void {
    this.#held = match;
    this.#gate = new Promise((resolve) => (this.#open = resolve));
  }

  release(): void {
    this.#open();
    this.#gate = null;
    this.#held = () => false;
  }

  async fetchText(owner: string, repo: string, sha: string, path: string): Promise<FetchResult> {
    if (`${owner}/${repo}` !== `${PAGE.owner}/${PAGE.repo}`) throw new Error(`unexpected repository ${owner}/${repo}`);
    this.calls.push(`${side(sha)}:${path}`);
    if (this.#gate && this.#held(path)) await this.#gate;
    const set = this.files.get(`${sha}:${path}`);
    if (set) return set;
    const text = pageSource(sha, path);
    return text === null ? { ok: false, reason: 'not-found', status: 404 } : { ok: true, text };
  }
}

type Ranges = Pick<ChangeResult, 'old' | 'new'>;

/** hide returns a range with one getter hit, as the fake engine reports it. */
export const hide = (start: number, end: number): Range => ({
  start,
  end,
  hits: [{ ruleId: 'getter', target: 'func', node: 'FuncDecl', label: `func at ${start}` }],
});

/**
 * FakeBackground answers BgRequest like the service worker, with a fake
 * engine: YAML containing "invalid" fails to compile, and analyze hides the
 * ranges set for the file's new path (the old path for a deleted file).
 */
export class FakeBackground {
  readonly requests: BgRequest[] = [];
  readonly cache = new Map<string, AnalysisRecord>();
  readonly ranges = new Map<string, Ranges>();
  tab: TabState = { enabled: true, headPreview: false };
  /** When set, analyze answers with this Failure error. */
  analyzeError: string | null = null;
  readonly diagnostics: Diagnostic[] = [{ severity: 'error', code: 'config-invalid', message: 'unknown preset "gettr"', field: 'presets[0](gettr)', line: 3 }];

  readonly send: SendFn = async (req) => {
    this.requests.push(JSON.parse(JSON.stringify(req)) as BgRequest);
    // The switch answers each request type with its BgResponse.
    return this.#answer(req) as never;
  };

  /** types lists the requests by type, with the path for lookup and analyze. */
  types(): string[] {
    return this.requests.map((r) => {
      if (r.type === 'analyze') return `analyze ${r.change.newPath ?? r.change.oldPath}`;
      return r.type;
    });
  }

  #answer(req: BgRequest): unknown {
    switch (req.type) {
      case 'compile':
        if (req.yaml.includes('invalid')) return { ok: false, error: 'invalid config', diagnostics: this.diagnostics };
        return {
          ok: true,
          configKey: `key-${fnv1a32(req.yaml)}`,
          diagnostics: [],
          rules: [{ id: 'getter', description: 'Plain getters', target: 'func', preset: 'getter' }],
          engineVersion: 'v0.0.0-test',
        };
      case 'lookup':
        return { ok: true, record: this.cache.get(req.cacheKey) ?? null };
      case 'analyze': {
        if (this.analyzeError !== null) return { ok: false, error: this.analyzeError };
        const r = this.ranges.get(req.change.newPath ?? req.change.oldPath ?? '') ?? { old: [], new: [] };
        const record = buildRecord({ ...r, diagnostics: [], skipped: '' }, req.change);
        this.cache.set(req.cacheKey, record);
        return { ok: true, record };
      }
      case 'get-tab-state':
        return { ok: true, ...this.tab, headPreview: this.tab.headPreview && req.pr === PR_KEY };
      default:
        return { ok: false, error: `unexpected ${req.type}` };
    }
  }
}

/** ManualFrames queues animation frame callbacks until flush(). */
export class ManualFrames implements FrameApi {
  readonly queue = new Map<number, () => void>();
  requested = 0;

  requestAnimationFrame(cb: () => void): number {
    this.requested++;
    this.queue.set(this.requested, cb);
    return this.requested;
  }

  cancelAnimationFrame(id: number): void {
    this.queue.delete(id);
  }

  flush(): void {
    const due = [...this.queue.values()];
    this.queue.clear();
    for (const cb of due) cb();
  }
}

/** hiddenRows lists the hidden rows of container as "-<old>" for del, "+<new>" for add, and "<old>/<new>" for context rows. */
export function hiddenRows(container: HTMLElement): string[] {
  return [...classic.rows(container)]
    .filter((r) => r.el.hasAttribute('data-gotebanare-hidden'))
    .map((r) => (r.kind === 'del' ? `-${r.oldLine}` : r.kind === 'add' ? `+${r.newLine}` : `${r.oldLine}/${r.newLine}`));
}

/** bannerTexts returns the messages of the page banner. */
export function bannerTexts(): string[] {
  return [...document.querySelectorAll('[data-gotebanare-banner] li')].map((li) => li.textContent ?? '');
}

/** addedMarks counts the attributes and elements the extension added to the page. */
export function addedMarks(): number {
  return document.querySelectorAll('[data-gotebanare-hidden], [data-gotebanare-fold], [data-gotebanare-badge], [data-gotebanare-banner], [data-gotebanare-debug]').length;
}
