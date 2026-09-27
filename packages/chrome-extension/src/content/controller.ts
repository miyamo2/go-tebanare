// Controller runs the extension on one pull request page (plan 6.2): it
// reads the options and tab state, watches the page, and keeps one Run of
// the pipeline for the commits the page names. index.ts creates one
// controller per pull request page and disposes it on navigation. A
// replaced run and a disposed controller never touch the page with late
// results.

import { errorText, headPreviewApplies, pullRequestKey, type PageState, type PageStatus, type TabStateMessage } from '../shared/messages.js';
import { isExcluded, sanitizeOptions, type Options } from '../shared/settings.js';
import { clearFile } from './apply.js';
import type { PullRequestContext, PullRequestContextProvider } from './context.js';
import type { SendFn } from './controller/file.js';
import { Run } from './controller/run.js';
import { LOADING_DELAY_MS, LoadingDelay } from './controller/loading.js';
import { inactiveStatus } from './controller/status.js';
import { watchMutations, type Changes, type FrameApi, type ObserverCtor } from './controller/watch.js';
import type { DiffUiVariant } from './dom/variant.js';
import { Semaphore, type SourceFetcher } from './fetcher.js';
import type { PullPage } from './page.js';
import { removeBanner, renderBanner } from './ui/banner.js';

/** At most this many files are analyzed at once (plan 6.4). */
export const MAX_FILES_IN_FLIGHT = 4;

/** Everything the controller reads from the browser. index.ts passes the real ones. */
export interface ControllerDeps {
  doc: Document;
  send: SendFn;
  fetcher: SourceFetcher;
  contexts: PullRequestContextProvider;
  /**
   * Resolves the context from the server's copy of the page when contexts
   * finds none in the DOM. It never rejects. Without it, such a page hides
   * nothing.
   */
  fetchContext?: (page: PullPage) => Promise<PullRequestContext | null>;
  detectVariant(doc: Document): DiffUiVariant | null;
  loadOptions(): Promise<Options>;
  frames: FrameApi;
  MutationObserver: ObserverCtor;
  /** Defaults to MAX_FILES_IN_FLIGHT. */
  maxFilesInFlight?: number;
  /** Defaults to LOADING_DELAY_MS. */
  loadingDelayMs?: number;
}

/** clearPage removes every row mark, fold row, badge, and banner of the extension from doc. */
function clearPage(doc: Document): void {
  clearFile(doc.documentElement);
  removeBanner(doc);
}

const sameCommits = (a: PullRequestContext | null, b: PullRequestContext | null) =>
  a?.baseSha === b?.baseSha && a?.headSha === b?.headSha;

export class Controller {
  readonly #deps: ControllerDeps;
  readonly #page: PullPage;
  readonly #repo: string;
  readonly #prKey: string;
  readonly #limit: Semaphore;
  readonly #loading: LoadingDelay;

  #disposed = false;
  // The state before the first run, and after the controller stops.
  #state: PageState = 'loading';
  #enabled = true;
  #headPreview = false;
  #tabStateSeen = false;
  #options: Options = sanitizeOptions(undefined);
  #run: Run | null = null;
  // The context fetchContext found. It stands in for the DOM until the DOM
  // names commits itself.
  #fetched: PullRequestContext | null = null;
  #unwatch: (() => void) | null = null;

  constructor(page: PullPage, deps: ControllerDeps) {
    this.#page = page;
    this.#deps = deps;
    this.#repo = `${page.owner}/${page.repo}`;
    this.#prKey = pullRequestKey(this.#repo, page.number);
    this.#limit = new Semaphore(deps.maxFilesInFlight ?? MAX_FILES_IN_FLIGHT);
    this.#loading = new LoadingDelay(deps.loadingDelayMs ?? LOADING_DELAY_MS, () => {
      if (this.#disposed) return;
      if (this.#run) this.#run.render();
      else if (this.#unwatch) this.#renderLoading();
    });
  }

  /** start runs the pipeline up to the first scan of the files. It never rejects. */
  start(): Promise<void> {
    // A page that Turbo or the back/forward cache restores can still carry
    // the marks of an earlier controller, and this one may hide nothing.
    clearPage(this.#deps.doc);
    return this.#begin().catch((e: unknown) => this.#fail(e));
  }

  /** setTabState applies a tab-state message: hiding on or off, and the head config preview. */
  setTabState(msg: TabStateMessage): void {
    if (this.#disposed) return;
    this.#tabStateSeen = true;
    const headPreview = headPreviewApplies(msg, this.#prKey);
    const previewChanged = headPreview !== this.#headPreview;
    const enabledChanged = msg.enabled !== this.#enabled;
    this.#enabled = msg.enabled;
    this.#headPreview = headPreview;
    // Before the first run, the pipeline has not read headPreview yet. A
    // run without commits has no config to load.
    if (previewChanged && this.#run?.ctx) void this.#startRun(this.#run.ctx);
    else if (enabledChanged && this.#run) this.#run.refresh();
    else if (enabledChanged && this.#unwatch) this.#renderLoading();
  }

  status(): PageStatus {
    return {
      ...inactiveStatus(),
      state: this.#state,
      repo: this.#repo,
      pr: this.#page.number,
      ...this.#run?.status(),
      enabled: this.#enabled,
      headPreview: this.#headPreview,
    };
  }

  /** dispose stops all work and removes every mark of the extension from the page. */
  dispose(): void {
    if (this.#disposed) return;
    this.#disposed = true;
    this.#stop();
    clearPage(this.#deps.doc);
  }

  async #begin(): Promise<void> {
    // Without the options, the repository may be excluded, so a failure to
    // read them ends in the error state (plan 0: when in doubt, hide nothing).
    const [options, tab] = await Promise.all([this.#deps.loadOptions(), this.#deps.send({ type: 'get-tab-state', pr: this.#prKey })]);
    if (this.#disposed) return;
    this.#options = options;
    // A tab-state message that arrived meanwhile is newer than this answer.
    if (!this.#tabStateSeen) {
      // Hiding may be off for this tab.
      if (!tab.ok) throw new Error(`get-tab-state: ${tab.error}`);
      this.#enabled = tab.enabled;
      this.#headPreview = tab.headPreview;
    }
    if (isExcluded(this.#repo, options.excludedRepos)) {
      this.#state = 'excluded';
      return;
    }
    const { doc, MutationObserver, frames } = this.#deps;
    this.#unwatch = watchMutations(doc.documentElement, MutationObserver, frames, (changes) => this.#onBatch(changes));
    // The run takes over the indicator once it starts.
    this.#renderLoading();
    const ctx = this.#resolve() ?? (await this.#fetchContext());
    if (this.#disposed) return;
    await this.#startRun(ctx);
  }

  // renderLoading shows the loading indicator before the first run, while
  // the commits are looked up. With hiding off, it removes the banner.
  #renderLoading(): void {
    const { doc } = this.#deps;
    const [anchor = null] = this.#deps.detectVariant(doc)?.fileContainers(doc) ?? [];
    renderBanner(doc, [], anchor, this.#loading.visible(this.#enabled));
  }

  #resolve(): PullRequestContext | null {
    return this.#deps.contexts.resolve(this.#deps.doc, this.#page) ?? this.#fetched;
  }

  // fetchContext asks the server for the commits once, at the start. DOM
  // batches that arrive meanwhile find no run and are dropped; the run
  // scans the whole page when it starts.
  async #fetchContext(): Promise<PullRequestContext | null> {
    const fetch = this.#deps.fetchContext;
    if (!fetch) return null;
    this.#fetched = await fetch(this.#page);
    return this.#resolve();
  }

  #onBatch(changes: Changes): void {
    const run = this.#run;
    if (this.#disposed || !run) return;
    try {
      // The commits decide what each file hides, so a new run starts when
      // the page names other ones. S2: GitHub may show another comparison
      // without a navigation that index.ts sees, for example in a Turbo
      // frame or stream, or when it refreshes after a push.
      const ctx = this.#resolve();
      if (!sameCommits(ctx, run.ctx)) void this.#startRun(ctx);
      else run.update(changes);
    } catch (e) {
      this.#fail(e);
    }
  }

  // startRun replaces the run with one for ctx and the current config
  // source: at the start, after the head config preview changes (plan 6.5),
  // and when the page shows other commits.
  #startRun(ctx: PullRequestContext | null): Promise<void> {
    this.#run?.stop();
    const run = new Run(
      {
        doc: this.#deps.doc,
        send: this.#deps.send,
        fetcher: this.#deps.fetcher,
        limit: this.#limit,
        detectVariant: (doc) => this.#deps.detectVariant(doc),
        debug: this.#options.debug,
        enabled: () => this.#enabled,
        loadingShown: (loading) => this.#loading.visible(loading),
      },
      ctx,
      this.#headPreview ? 'head' : 'base',
    );
    this.#run = run;
    return run.start().catch((e: unknown) => {
      if (this.#run === run) this.#fail(e);
    });
  }

  // fail ends the controller after an unexpected error: it drops all work,
  // removes what it added, and stays in the error state until disposed.
  #fail(e: unknown): void {
    if (this.#disposed) return;
    console.warn(`go-tebanare: ${errorText(e)}`);
    this.#stop();
    this.#state = 'error';
    clearPage(this.#deps.doc);
  }

  #stop(): void {
    this.#loading.stop();
    this.#unwatch?.();
    this.#unwatch = null;
    this.#run?.stop();
    this.#run = null;
  }
}
