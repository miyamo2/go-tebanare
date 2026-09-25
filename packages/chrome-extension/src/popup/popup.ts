// The popup (plan 6.1, 6.5, 6.7). It asks the content script of the active
// tab for its status, shows it, and changes the tab state (hiding on or
// off, head config preview) through the background, which forwards the
// change to the tab.

import { localizePage, t } from '../shared/i18n.js';
import {
  errorText,
  pullRequestKey,
  send,
  sendToTab,
  type PageStatus,
  type RuntimeTransport,
  type SetTabStateRequest,
  type TabsTransport,
} from '../shared/messages.js';
import { parsePageStatus } from './status.js';
import { popupElements, renderPopup, type PopupModel } from './view.js';

/** The chrome APIs the popup uses. The real chrome namespace satisfies it. */
export interface PopupApi {
  tabs: TabsTransport & { query(q: { active: boolean; currentWindow: boolean }): Promise<readonly { id?: number }[]> };
  runtime: RuntimeTransport;
}

export interface PopupOptions {
  /** sleep waits between status requests. Default: setTimeout. */
  sleep?: (ms: number) => Promise<void>;
  /** The time between status requests while the page loads or applies a change. Default 250 ms. */
  pollInterval?: number;
  /** The number of status requests after the first one. Default 40, which is 10 seconds at 250 ms. */
  maxPolls?: number;
}

export interface Popup {
  /** refresh asks the tab for its status and shows it, asking again while the page loads. */
  refresh(): Promise<void>;
  /** toggleHiding turns hiding on or off for the tab. */
  toggleHiding(): Promise<void>;
  /** toggleHeadPreview turns the head config preview on or off for the pull request the tab shows. */
  toggleHeadPreview(): Promise<void>;
  /** idle resolves when every action and refresh started so far has finished. */
  idle(): Promise<void>;
}

const always = () => true;

async function activeTabId(tabs: PopupApi['tabs']): Promise<number | undefined> {
  try {
    return (await tabs.query({ active: true, currentWindow: true }))[0]?.id;
  } catch {
    return undefined;
  }
}

/** startPopup localizes doc (popup.html), shows the status of the active tab, and wires the buttons. */
export async function startPopup(doc: Document, api: PopupApi, opts: PopupOptions = {}): Promise<Popup> {
  doc.documentElement.lang = t('pageLang');
  localizePage(doc);
  const el = popupElements(doc);
  const sleep = opts.sleep ?? ((ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms)));
  const interval = opts.pollInterval ?? 250;
  const maxPolls = opts.maxPolls ?? 40;
  const model: PopupModel = { status: null, busy: false };
  const render = () => renderPopup(el, model);
  const tabId = await activeTabId(api.tabs);
  // Each refresh and action takes a new generation; an older refresh loop stops rendering.
  let generation = 0;
  let pending: Promise<unknown> = Promise.resolve();

  async function load(): Promise<PageStatus | null> {
    if (tabId === undefined) return null;
    return parsePageStatus(await sendToTab(tabId, { type: 'status' }, api.tabs));
  }

  // refresh asks again until the page has loaded and done(status) holds, or maxPolls runs out.
  async function refresh(done: (s: PageStatus) => boolean = always): Promise<void> {
    const gen = ++generation;
    for (let i = 0; ; i++) {
      const s = await load();
      if (gen !== generation) return;
      model.status = s;
      render();
      if (s === null || (s.state !== 'loading' && done(s)) || i >= maxPolls) return;
      await sleep(interval);
      if (gen !== generation) return;
    }
  }

  async function act(req: SetTabStateRequest, done: (s: PageStatus) => boolean): Promise<void> {
    generation++;
    model.busy = true;
    delete model.error;
    render();
    const res = await send(req, api.runtime);
    model.busy = false;
    if (!res.ok) model.error = res.error;
    // The background has sent the change to the tab; the page may need a moment to apply it.
    await refresh(res.ok ? done : always);
  }

  async function toggleHiding(): Promise<void> {
    const s = model.status;
    if (s === null || tabId === undefined || model.busy) return;
    const enabled = !s.enabled;
    await act({ type: 'set-tab-state', tabId, enabled }, (n) => n.enabled === enabled);
  }

  async function toggleHeadPreview(): Promise<void> {
    const s = model.status;
    if (s === null || tabId === undefined || model.busy) return;
    if (s.headPreview) {
      await act({ type: 'set-tab-state', tabId, headPreview: false }, (n) => !n.headPreview);
    } else if (s.repo !== undefined && s.pr !== undefined) {
      const pr = pullRequestKey(s.repo, s.pr);
      await act({ type: 'set-tab-state', tabId, headPreview: true, pr }, (n) => n.headPreview);
    }
  }

  function track(p: Promise<void>): Promise<void> {
    pending = Promise.allSettled([pending, p]);
    return p;
  }

  el.toggleHiding.addEventListener('click', () => void track(toggleHiding()));
  el.toggleHeadPreview.addEventListener('click', () => void track(toggleHeadPreview()));
  // popup.html starts with everything hidden; the first status decides what shows.
  void track(refresh());
  return {
    refresh: () => track(refresh()),
    toggleHiding: () => track(toggleHiding()),
    toggleHeadPreview: () => track(toggleHeadPreview()),
    async idle() {
      let seen: Promise<unknown> | undefined;
      while (seen !== pending) {
        seen = pending;
        await seen;
      }
    },
  };
}

// Chrome runs this file as the script of popup.html. Tests import it without chrome.
if (typeof chrome !== 'undefined' && chrome.tabs && chrome.runtime && typeof document !== 'undefined') {
  startPopup(document, chrome).catch((e: unknown) => console.error(`go-tebanare: ${errorText(e)}`));
}
