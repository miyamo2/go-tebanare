// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { startBackground } from '../../src/background/index.js';
import { tabKey } from '../../src/background/tabs.js';
import { startPopup, type Popup } from '../../src/popup/popup.js';
import { popupElements } from '../../src/popup/view.js';
import { headPreviewApplies, type PageStatus, type TabMessage, type TabStateMessage } from '../../src/shared/messages.js';
import { asChrome, FakeChrome, installChrome } from '../fakes/chrome.js';
import { loadStaticPage } from '../fakes/static-page.js';
import { pageStatus } from './status-fixture.js';

let restore = () => {};
afterEach(() => restore());

const PR = 'octo/repo#7';

/** A content script that answers status and applies tab-state as the controller would. */
interface FakePage {
  status: PageStatus;
  /** Answers to send before status, one per status request. */
  queue: unknown[];
  received: TabMessage[];
  onTabState?: (m: TabStateMessage) => void;
}

function setup(opts: { tab?: boolean; contentScript?: boolean; background?: boolean } = {}) {
  const hub = new FakeChrome();
  const page: FakePage = { status: pageStatus(), queue: [], received: [] };
  if (opts.tab !== false) {
    const tab = hub.addTab({ url: 'https://github.com/octo/repo/pull/7/files', active: true });
    if (opts.contentScript !== false) {
      hub.contentScript(tab.id).runtime.onMessage.addListener((msg, _sender, sendResponse) => {
        const m = msg as TabMessage;
        page.received.push(m);
        if (m.type === 'status') sendResponse(page.queue.length > 0 ? page.queue.shift() : page.status);
        else if (page.onTabState) page.onTabState(m);
        else page.status = { ...page.status, enabled: m.enabled, headPreview: headPreviewApplies(m, PR) };
      });
    }
    // A second GitHub tab in the background that the popup must not ask.
    const other = hub.addTab({ url: 'https://github.com/x/y/pull/1/files' });
    hub.contentScript(other.id).runtime.onMessage.addListener((_m, _s, sendResponse) => void sendResponse(pageStatus({ repo: 'x/y', pr: 1 })));
  }
  if (opts.background !== false) {
    startBackground(asChrome(hub.extensionContext()), async () => {
      throw new Error('the popup does not use the engine');
    });
  }
  const api = hub.extensionContext('popup.html');
  restore = installChrome(api);
  loadStaticPage('popup.html');
  const sleep = vi.fn(async (_ms: number) => {});
  const start = async (maxPolls?: number): Promise<Popup> => {
    const popup = await startPopup(document, asChrome(api), { sleep, pollInterval: 100, maxPolls });
    await popup.idle();
    return popup;
  };
  return { hub, page, sleep, start, el: popupElements(document) };
}

describe('popup', () => {
  it('shows the status of the active tab', async () => {
    const { page, start, el } = setup();
    await start();
    expect(page.received).toEqual([{ type: 'status' }]);
    expect(el.state.textContent).toBe('Active');
    expect(el.pullRequest.textContent).toBe('octo/repo #7');
    expect(el.noPage.hidden).toBe(true);
    expect(document.documentElement.lang).toBe('en');
  });

  it.each([
    ['no active tab', { tab: false }],
    ['no content script', { contentScript: false }],
  ])('asks for a pull request page when there is %s', async (_, opts) => {
    const { start, el } = setup(opts);
    await start();
    expect(el.noPage.hidden).toBe(false);
    expect(el.noPage.textContent).toBe("Open a pull request's Files changed tab.");
    expect(el.state.hidden).toBe(true);
    expect(el.toggleHiding.hidden).toBe(true);
  });

  it('treats a malformed status as no page', async () => {
    const { page, start, el } = setup();
    page.queue.push({ state: 'ready' });
    await start();
    expect(el.noPage.hidden).toBe(false);
  });

  it('asks again while the page loads', async () => {
    const { page, sleep, start, el } = setup();
    page.queue.push(pageStatus({ state: 'loading' }), pageStatus({ state: 'loading' }));
    await start();
    expect(sleep.mock.calls).toEqual([[100], [100]]);
    expect(el.state.textContent).toBe('Active');
  });

  it('stops asking after maxPolls', async () => {
    const { page, sleep, start, el } = setup();
    page.status = pageStatus({ state: 'loading' });
    await start(3);
    expect(sleep).toHaveBeenCalledTimes(3);
    expect(el.state.textContent).toBe('Loading');
  });

  it('turns hiding off and on through the background', async () => {
    const { hub, page, start, el } = setup();
    await start();
    el.toggleHiding.click();
    expect(el.toggleHiding.disabled).toBe(true);
    await vi.waitFor(() => expect(el.toggleHiding.textContent).toBe('Hiding: off'));
    expect(el.toggleHiding.disabled).toBe(false);
    expect(page.received).toContainEqual({ type: 'tab-state', enabled: false, headPreview: false });
    expect(hub.storage.session.data.get(tabKey(1))).toEqual({ enabled: false, headPreviewFor: null });

    el.toggleHiding.click();
    await vi.waitFor(() => expect(el.toggleHiding.textContent).toBe('Hiding: on'));
    expect(hub.storage.session.data.get(tabKey(1))).toEqual({ enabled: true, headPreviewFor: null });
  });

  it('turns the head config preview on for the pull request the page shows, and off', async () => {
    const { hub, page, start, el } = setup();
    const popup = await start();
    await popup.toggleHeadPreview();
    expect(hub.storage.session.data.get(tabKey(1))).toEqual({ enabled: true, headPreviewFor: PR });
    expect(page.received).toContainEqual({ type: 'tab-state', enabled: true, headPreview: true, pr: PR });
    expect(el.headPreviewBanner.hidden).toBe(false);
    expect(el.toggleHeadPreview.textContent).toBe('Stop head config preview');

    await popup.toggleHeadPreview();
    expect(hub.storage.session.data.get(tabKey(1))).toEqual({ enabled: true, headPreviewFor: null });
    expect(el.headPreviewBanner.hidden).toBe(true);
    expect(el.toggleHeadPreview.textContent).toBe('Preview with head config');
  });

  it('waits until the page has applied the change', async () => {
    const { page, sleep, start, el } = setup();
    const popup = await start();
    page.onTabState = (m) => {
      // The page still reports the old state once, then reloads with the head config.
      page.queue.push(page.status, pageStatus({ state: 'loading', headPreview: true }));
      page.status = pageStatus({ headPreview: headPreviewApplies(m, PR), configSource: 'head' });
    };
    await popup.toggleHeadPreview();
    expect(sleep).toHaveBeenCalledTimes(2);
    expect(el.state.textContent).toBe('Active');
    expect(el.headPreviewBanner.hidden).toBe(false);
  });

  it('does not offer the preview without the pull request number', async () => {
    const { page, start, hub } = setup();
    page.status = pageStatus({ pr: undefined });
    const popup = await start();
    await popup.toggleHeadPreview();
    expect(hub.storage.session.data.has(tabKey(1))).toBe(false);
  });

  it('shows the error when the background does not answer', async () => {
    const { page, start, el } = setup({ background: false });
    const popup = await start();
    await popup.toggleHiding();
    expect(el.error.hidden).toBe(false);
    expect(el.error.textContent).toBe('Could not change the setting: Could not establish connection. Receiving end does not exist.');
    expect(el.toggleHiding.textContent).toBe('Hiding: on');
    expect(el.toggleHiding.disabled).toBe(false);
    expect(page.received.filter((m) => m.type === 'tab-state')).toEqual([]);

    await popup.refresh();
    expect(el.error.hidden).toBe(false);
  });
});
