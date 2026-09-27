// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { inactiveStatus } from '../../src/content/controller/status.js';
import { startContent, type ContentEnv, type PageController } from '../../src/content/index.js';
import type { PullPage } from '../../src/content/page.js';
import type { PageStatus, TabStateMessage } from '../../src/shared/messages.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';
import { addedMarks, buildPage, fileHtml, harness, hiddenRows, hide } from './controller-setup.js';

const PR = (n: number, tab = 'files') => `https://github.com/octo-org/octo-repo/pull/${n}/${tab}`;

let restore = () => {};
beforeEach(() => {
  restore = installChrome(new FakeChrome({ locale: 'en' }).extensionContext());
});
afterEach(() => restore());

/** FakeController writes each call to a log shared by every controller of a test. */
class FakeController implements PageController {
  constructor(
    readonly page: PullPage,
    private readonly log: string[],
  ) {}
  async start(): Promise<void> {
    this.log.push(`start ${this.page.number}`);
  }
  setTabState(msg: TabStateMessage): void {
    this.log.push(`tab-state ${this.page.number} ${msg.enabled}`);
  }
  status(): PageStatus {
    return { ...inactiveStatus(), state: 'ready', repo: `${this.page.owner}/${this.page.repo}`, pr: this.page.number };
  }
  dispose(): void {
    this.log.push(`dispose ${this.page.number}`);
  }
}

type Nav = 'popstate' | 'currententrychange' | 'turbo:load' | 'turbo:before-cache';

/** setup starts the content script at url in a fake tab and returns ways to message and navigate it. */
function setup(url: string, createController?: ContentEnv['createController']) {
  const hub = new FakeChrome({ locale: 'en' });
  const tab = hub.addTab({ url, active: true });
  const popup = hub.extensionContext('popup.html');
  const location = { href: url };
  const doc = new EventTarget();
  const targets: Record<Nav, EventTarget> = { popstate: new EventTarget(), currententrychange: new EventTarget(), 'turbo:load': doc, 'turbo:before-cache': doc };
  const log: string[] = [];
  const script = startContent({
    location,
    window: targets.popstate,
    navigation: targets.currententrychange,
    document: doc,
    onMessage: hub.contentScript(tab.id).runtime.onMessage,
    createController: createController ?? ((page) => new FakeController(page, log)),
  });
  const ask = (msg: unknown) => popup.tabs!.sendMessage(tab.id, msg);
  const go = (href: string, nav: Nav) => {
    location.href = href;
    targets[nav].dispatchEvent(new Event(nav));
  };
  return { script, log, ask, go };
}

describe('startContent', () => {
  it('starts a controller only on a pull request diff', () => {
    expect(setup(PR(7)).log).toEqual(['start 7']);
    expect(setup(PR(7, 'changes')).log).toEqual(['start 7']);
    expect(setup('https://github.com/octo-org/octo-repo/pull/7').log).toEqual([]);
  });

  it('answers status and passes tab state on', async () => {
    const { ask, log } = setup(PR(7));
    expect(await ask({ type: 'status' })).toMatchObject({ state: 'ready', repo: 'octo-org/octo-repo', pr: 7 });
    await ask({ type: 'tab-state', enabled: false, headPreview: false });
    await ask({ type: 'tab-state', enabled: 'no', headPreview: false });
    expect(await ask({ type: 'other' })).toBeUndefined();
    expect(log).toEqual(['start 7', 'tab-state 7 false']);
  });

  it('answers inactive away from a pull request diff', async () => {
    const { ask } = setup('https://github.com/octo-org/octo-repo/pulls');
    expect(await ask({ type: 'status' })).toEqual(inactiveStatus());
  });

  it('follows navigation and disposes the old controller first', async () => {
    const { script, log, go, ask } = setup(PR(7));
    go(`${PR(7)}#diff-abc`, 'popstate');
    go(PR(7, 'changes'), 'currententrychange');
    expect(log).toEqual(['start 7']);
    go(PR(8), 'currententrychange');
    expect(log).toEqual(['start 7', 'dispose 7', 'start 8']);
    // Turbo rendered a new page, so the controller starts over.
    go(PR(8), 'turbo:load');
    expect(log.slice(3)).toEqual(['dispose 8', 'start 8']);
    go('https://github.com/octo-org/octo-repo/pulls', 'popstate');
    expect(log.slice(5)).toEqual(['dispose 8']);
    expect(script.current()).toBeNull();
    expect(await ask({ type: 'status' })).toEqual(inactiveStatus());

    go(PR(9), 'popstate');
    expect(log.slice(6)).toEqual(['start 9']);
    script.stop();
    expect(log.slice(7)).toEqual(['dispose 9']);
    go(PR(10), 'turbo:load');
    expect(log).toHaveLength(8);
  });

  it('disposes the controller before Turbo keeps a copy of the page', async () => {
    const { script, log, go, ask } = setup(PR(7));
    go(PR(7), 'turbo:before-cache');
    expect(log).toEqual(['start 7', 'dispose 7']);
    expect(script.current()).toBeNull();
    expect(await ask({ type: 'status' })).toEqual(inactiveStatus());
    go(PR(7), 'turbo:load');
    expect(log.slice(2)).toEqual(['start 7']);
    go(PR(7, 'changes'), 'turbo:before-cache');
    go(PR(7, 'changes'), 'currententrychange');
    expect(log.slice(3)).toEqual(['dispose 7', 'start 7']);
    script.stop();
    go(PR(7), 'turbo:before-cache');
    expect(log).toHaveLength(6);
  });

  it('leaves no folds in the copy that Turbo keeps', async () => {
    const [store] = buildPage(fileHtml('classic-modified.html'));
    const h = harness();
    h.bg.ranges.set('store/store.go', { old: [], new: [hide(26, 29)] });
    const { script, go } = setup(PR(7), () => h.controller);
    await h.settle();
    expect(hiddenRows(store!)).toHaveLength(4);
    go(PR(7), 'turbo:before-cache');
    expect(addedMarks()).toBe(0);
    await h.settle();
    expect(addedMarks()).toBe(0);
    script.stop();
  });

  it('never applies late results of a controller left behind', async () => {
    const [store] = buildPage(fileHtml('classic-modified.html'));
    const first = harness();
    first.bg.ranges.set('store/store.go', { old: [], new: [hide(26, 29)] });
    first.fetcher.hold((p) => p.endsWith('.go'));
    // The next controller never gets past loading, so the page shows only what the first one does.
    const second = harness({ page: { owner: 'octo-org', repo: 'octo-repo', number: 8 }, deps: { loadOptions: () => new Promise(() => {}) } });
    const { script, go, ask } = setup(PR(7), (page) => (page.number === 7 ? first.controller : second.controller));
    await first.settle();
    expect(first.bg.types()).toEqual(['get-tab-state', 'compile', 'lookup']);
    expect(await ask({ type: 'status' })).toMatchObject({ state: 'ready', pr: 7, configPath: '.gotebanare.yml' });

    go(PR(8), 'popstate');
    expect(script.current()).toBe(second.controller);
    first.fetcher.release();
    await first.settle();
    expect(first.bg.types()).toEqual(['get-tab-state', 'compile', 'lookup']);
    expect(hiddenRows(store!)).toEqual([]);
    expect(addedMarks()).toBe(0);
    expect(await ask({ type: 'status' })).toMatchObject({ state: 'loading', pr: 8, files: 0 });
    script.stop();
  });
});
