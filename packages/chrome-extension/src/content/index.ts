// The content script (plan 6.1). GitHub moves between pages without a full
// load, so the script watches navigation and keeps one Controller running
// while the URL is a pull request's "Files changed" page (plan 6.6).

import {
  isTabMessage,
  pullRequestKey,
  send,
  type PageStatus,
  type TabStateMessage,
} from '../shared/messages.js';
import { loadOptions } from '../shared/settings.js';
import { defaultContextProvider } from './context.js';
import { Controller } from './controller.js';
import { inactiveStatus } from './controller/status.js';
import { detectVariant } from './dom/variant.js';
import { SessionFetcher } from './fetcher.js';
import { parsePullUrl, type PullPage } from './page.js';

/** The controller calls the entry point makes. Controller implements them. */
export interface PageController {
  start(): Promise<void>;
  setTabState(msg: TabStateMessage): void;
  status(): PageStatus;
  dispose(): void;
}

export type TabMessageListener = (message: unknown, sender: unknown, sendResponse: (response?: unknown) => void) => boolean;

/** The browser objects the entry point uses. */
export interface ContentEnv {
  location: { readonly href: string };
  /** Receives popstate. */
  window: EventTarget;
  /**
   * Receives turbo:before-cache, which Turbo dispatches before it keeps a
   * copy of the page for back navigation, and turbo:load, which it
   * dispatches after it renders a page.
   */
  document: EventTarget;
  /** The Navigation API object, when the browser has one. */
  navigation?: EventTarget | undefined;
  onMessage: { addListener(l: TabMessageListener): void; removeListener(l: TabMessageListener): void };
  createController(page: PullPage): PageController;
}

export interface ContentScript {
  /** The controller of the current page, or null when the page is not a pull request diff. */
  current(): PageController | null;
  /** stop removes the listeners and disposes the controller. */
  stop(): void;
}

function isTabState(msg: TabStateMessage): boolean {
  return typeof msg.enabled === 'boolean' && typeof msg.headPreview === 'boolean';
}

/**
 * startContent starts a controller when env.location is a pull request diff
 * and checks the URL again on popstate, Navigation API
 * currententrychange, and turbo:load. A move to another pull request, or
 * away from one, disposes the old controller before anything else. A
 * turbo:load always starts a new controller, because Turbo replaced the
 * page and the commits may have changed. turbo:before-cache disposes the
 * controller, so the copy Turbo keeps has no folds.
 */
export function startContent(env: ContentEnv): ContentScript {
  let current: { key: string; controller: PageController } | null = null;

  const evaluate = (force: boolean) => {
    const page = parsePullUrl(env.location.href);
    const key = page ? pullRequestKey(`${page.owner}/${page.repo}`, page.number) : null;
    if (!force && (current?.key ?? null) === key) return;
    current?.controller.dispose();
    current = null;
    if (!page || key === null) return;
    const controller = env.createController(page);
    current = { key, controller };
    void controller.start();
  };
  const onUrlChange = () => evaluate(false);
  const onTurboLoad = () => evaluate(true);
  // Turbo renders another page next, and turbo:load or a URL change starts a controller for it.
  const onBeforeCache = () => {
    current?.controller.dispose();
    current = null;
  };

  const listener: TabMessageListener = (message, _sender, sendResponse) => {
    if (!isTabMessage(message)) return false;
    if (message.type === 'status') sendResponse(current?.controller.status() ?? inactiveStatus());
    else if (isTabState(message)) current?.controller.setTabState(message);
    return false;
  };

  env.onMessage.addListener(listener);
  env.window.addEventListener('popstate', onUrlChange);
  env.navigation?.addEventListener('currententrychange', onUrlChange);
  env.document.addEventListener('turbo:load', onTurboLoad);
  env.document.addEventListener('turbo:before-cache', onBeforeCache);
  evaluate(false);

  return {
    current: () => current?.controller ?? null,
    stop() {
      env.onMessage.removeListener(listener);
      env.window.removeEventListener('popstate', onUrlChange);
      env.navigation?.removeEventListener('currententrychange', onUrlChange);
      env.document.removeEventListener('turbo:load', onTurboLoad);
      env.document.removeEventListener('turbo:before-cache', onBeforeCache);
      current?.controller.dispose();
      current = null;
    },
  };
}

// Chrome runs this module as the content script. Tests import it without chrome.
if (typeof chrome !== 'undefined' && chrome.runtime?.onMessage && typeof document !== 'undefined') {
  // One fetcher for the page, so its limit of 4 requests covers every controller.
  const fetcher = new SessionFetcher();
  startContent({
    location,
    window,
    document,
    navigation: (window as { navigation?: EventTarget }).navigation,
    onMessage: chrome.runtime.onMessage,
    createController: (page) =>
      new Controller(page, {
        doc: document,
        send,
        fetcher,
        contexts: defaultContextProvider,
        detectVariant: (doc) => detectVariant(doc),
        loadOptions: () => loadOptions(),
        frames: window,
        MutationObserver,
      }),
  });
}
