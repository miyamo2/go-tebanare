// A Run is one pass of the pipeline over a pull request page for one pair
// of commits and one config source (plan 6.2 steps 3 to 6). It loads and
// compiles the config, finds the diff UI, and hands the files to a
// FileScanner. The controller replaces its run when the commits or the
// config source change, so a stopped run never touches the page again.

import type { ConfigSource, PageState, PageStatus } from '../../shared/messages.js';
import type { PullRequestContext } from '../context.js';
import type { DiffUiVariant } from '../dom/variant.js';
import type { Semaphore, SourceFetcher } from '../fetcher.js';
import { renderBanner } from '../ui/banner.js';
import type { SendFn } from './file.js';
import { FileScanner, type Compiled } from './scan.js';
import { setUpConfig } from './setup.js';
import { Report } from './status.js';
import type { Changes } from './watch.js';

/** What a run reads from the controller. */
export interface RunDeps {
  doc: Document;
  send: SendFn;
  fetcher: SourceFetcher;
  /** Limits the files analyzed at once, across the runs of one controller. */
  limit: Semaphore;
  detectVariant(doc: Document): DiffUiVariant | null;
  debug: boolean;
  enabled: () => boolean;
  /** Reports whether the loading indicator shows, given whether the run is loading (see LoadingDelay). */
  loadingShown(loading: boolean): boolean;
}

/** The fields of PageStatus that a run knows. */
export type RunStatus = Pick<PageStatus, 'state' | 'configSource' | 'configPath' | 'rules' | 'files' | 'filesWithFolds' | 'linesHidden' | 'messages'>;

export class Run {
  /** The commits of the run. Null when the page does not name them. */
  readonly ctx: PullRequestContext | null;
  readonly #deps: RunDeps;
  readonly #source: ConfigSource;
  readonly #report = new Report();
  #state: PageState = 'loading';
  #configPath: string | undefined;
  #config: Compiled | null = null;
  #scanner: FileScanner | null = null;
  #stopped = false;

  constructor(deps: RunDeps, ctx: PullRequestContext | null, source: ConfigSource) {
    this.#deps = deps;
    this.ctx = ctx;
    this.#source = source;
  }

  /** start loads the config and shows the files. Without commits it ends in the error state. */
  async start(): Promise<void> {
    if (!this.ctx) {
      this.#report.addPage({ kind: 'context-error' });
      return this.#finish('error');
    }
    this.#finish('loading');
    const { fetcher, send } = this.#deps;
    const setup = await setUpConfig(fetcher, send, this.ctx, this.#source, this.#report, () => this.#stopped);
    if (!setup || this.#stopped) return;
    this.#configPath = setup.configPath;
    if (setup.state !== 'ready') return this.#finish(setup.state);
    this.#config = setup.config;
    this.#detect();
  }

  /** update follows a batch of DOM changes. */
  update(changes: Changes): void {
    if (this.#stopped) return;
    if (!this.#scanner) return this.#detect();
    this.#scanner.scan(changes);
    this.#renderBanner();
  }

  /** render shows the banner again, for example after the loading delay passed. */
  render(): void {
    this.#renderBanner();
  }

  /** refresh shows every file again after hiding was turned on or off. */
  refresh(): void {
    this.#scanner?.refresh();
    this.#renderBanner();
  }

  /** stop drops pending work and removes what the run added to the files. The next run or the controller replaces the banner. */
  stop(): void {
    this.#stopped = true;
    this.#scanner?.stop();
    this.#scanner = null;
  }

  status(): RunStatus {
    const status: RunStatus = {
      state: this.#state,
      configSource: this.#source,
      rules: this.#config?.rules.length ?? 0,
      ...this.#report.counts(),
      messages: [...new Set(this.#report.messages().map((m) => m.text))],
    };
    if (this.#configPath !== undefined) status.configPath = this.#configPath;
    return status;
  }

  // detect looks for a known diff UI. It runs again on each DOM change while
  // none is found, because the diff can render after the content script starts.
  #detect(): void {
    const config = this.#config;
    if (!config) return;
    const { doc, send, fetcher, limit, debug, enabled } = this.#deps;
    const variant = this.#deps.detectVariant(doc);
    if (!variant) {
      this.#report.addPage({ kind: 'unsupported-ui' });
      return this.#finish('unsupported-ui');
    }
    this.#report.dropPage('unsupported-ui');
    const changed = () => this.#renderBanner();
    this.#scanner = new FileScanner({ doc, send, fetcher, limit, variant, config, source: this.#source, report: this.#report, debug, enabled, changed });
    this.#state = 'ready';
    this.#scanner.scan();
    this.#renderBanner();
  }

  #finish(state: PageState): void {
    this.#state = state;
    this.#renderBanner();
  }

  #renderBanner(): void {
    if (this.#stopped) return;
    const { doc } = this.#deps;
    const messages = this.#report.messages();
    // The indicator stays until every file shown has its result. With
    // hiding off, nothing waits for a result, and loadingShown says so.
    const loading = this.#deps.loadingShown(this.#state === 'loading' || (this.#scanner?.busy() ?? false));
    const variant = messages.length > 0 || loading ? this.#deps.detectVariant(doc) : null;
    const [anchor = null] = variant ? variant.fileContainers(doc) : [];
    renderBanner(doc, messages, anchor, loading);
  }
}
