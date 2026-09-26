// An in-memory fake of the chrome.* APIs the extension uses. One FakeChrome
// hub holds the shared state (storage, tabs, locale); each extension page,
// the service worker, and each content script gets its own API object, so
// runtime.onMessage listeners receive only what Chrome would deliver to
// that context, and content scripts get the storage access Chrome gives
// them (none to session storage by default).

import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  FakeEvent,
  FakeStorageArea,
  jsonClone,
  untrustedStorage,
  type AreaName,
  type ChangeListener,
  type StorageAreaApi,
  type StorageQuota,
} from './storage.js';
import { formatMessage, localeMessages } from './locale.js';

export { formatMessage } from './locale.js';
export {
  FakeEvent,
  FakeStorageArea,
  type AccessLevel,
  type AreaName,
  type StorageAreaApi,
  type StorageChange,
  type StorageQuota,
} from './storage.js';

// A path string: happy-dom replaces the global URL class, which node:fs rejects.
const pkgRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..');

/** The fields of chrome.tabs.Tab that the fake tracks. */
export interface FakeTab { id: number; windowId: number; index: number; active: boolean; url?: string; title?: string }

export interface MessageSender { id: string; url?: string; tab?: FakeTab; frameId?: number }
export type MessageListener = (message: unknown, sender: MessageSender, sendResponse: (response?: unknown) => void) => unknown;

interface Context {
  kind: 'extension' | 'content';
  url: string;
  tabId?: number;
  frameId?: number;
  onMessage: FakeEvent<MessageListener>;
}

const noReceiver = 'Could not establish connection. Receiving end does not exist.';

export interface FakeChromeOptions {
  extensionId?: string;
  /** "en" or "ja": the _locales directory i18n.getMessage reads. Default "en". */
  locale?: string;
  quotas?: Partial<Record<AreaName, Partial<StorageQuota>>>;
}

/** FakeChrome is the shared browser state behind every context's API object. */
export class FakeChrome {
  readonly id: string;
  locale: string;
  readonly storageChanged = new FakeEvent<ChangeListener>();
  readonly storage: Record<AreaName, FakeStorageArea>;
  readonly tabs = new Map<number, FakeTab>();
  readonly tabRemoved = new FakeEvent<(tabId: number, info: { windowId: number; isWindowClosing: boolean }) => void>();
  readonly command = new FakeEvent<(command: string, tab?: FakeTab) => void>();
  /** Badge texts set through chrome.action, keyed by tab id (-1 for the default). */
  readonly badgeText = new Map<number, string>();
  /** Errors thrown by runtime.onMessage listeners. Chrome logs them in the receiver only. */
  readonly listenerErrors: unknown[] = [];
  currentWindowId = 1;
  private readonly contexts: Context[] = [];
  private readonly pending = new Set<Promise<unknown>>();
  private nextTabId = 1;

  constructor(opts: FakeChromeOptions = {}) {
    this.id = opts.extensionId ?? 'fakeextensionid';
    this.locale = opts.locale ?? 'en';
    const area = (n: AreaName) => new FakeStorageArea(n, opts.quotas?.[n], this.storageChanged);
    this.storage = { session: area('session'), sync: area('sync'), local: area('local') };
  }

  addTab(init: Partial<Omit<FakeTab, 'id'>> = {}): FakeTab {
    const windowId = init.windowId ?? this.currentWindowId;
    const index = [...this.tabs.values()].filter((t) => t.windowId === windowId).length;
    const tab: FakeTab = { windowId, index, active: false, ...init, id: this.nextTabId++ };
    if (tab.active) for (const t of this.tabs.values()) if (t.windowId === windowId) t.active = false;
    this.tabs.set(tab.id, tab);
    return { ...tab };
  }

  /** removeTab closes a tab: its content scripts stop and tabs.onRemoved fires. */
  removeTab(tabId: number): void {
    const tab = this.tabs.get(tabId);
    if (!tab) return;
    this.tabs.delete(tabId);
    for (let i = this.contexts.length - 1; i >= 0; i--) {
      if (this.contexts[i]?.tabId === tabId) this.contexts.splice(i, 1);
    }
    this.tabRemoved.dispatch(tabId, { windowId: tab.windowId, isWindowClosing: false });
  }

  /** runCommand fires commands.onCommand as a keyboard shortcut would, with the active tab. */
  runCommand(command: string): void {
    const tab = [...this.tabs.values()].find((t) => t.active && t.windowId === this.currentWindowId);
    this.command.dispatch(command, tab ? { ...tab } : undefined);
  }

  getMessage(key: string, subs?: string | readonly (string | number)[]): string {
    if (key === '@@extension_id') return this.id;
    if (key === '@@ui_locale') return this.locale;
    const entry = localeMessages(this.locale)[key.toLowerCase()];
    if (!entry) return '';
    const list = subs === undefined ? [] : typeof subs === 'string' ? [subs] : subs.map(String);
    return formatMessage(entry, list);
  }

  /** flush waits until every message in flight has been answered. */
  async flush(): Promise<void> {
    while (this.pending.size > 0) await Promise.allSettled([...this.pending]);
  }

  /** extensionContext returns the API object of the service worker or an extension page. */
  extensionContext(page = 'background.js'): FakeExtensionApi {
    const ctx: Context = { kind: 'extension', url: this.url(page), onMessage: new FakeEvent() };
    this.contexts.push(ctx);
    return {
      ...this.commonApi(ctx),
      storage: { ...this.storage, onChanged: this.storageChanged },
      tabs: {
        query: async (q: TabQuery) => this.query(q),
        get: async (tabId: number) => {
          const tab = this.tabs.get(tabId);
          if (!tab) throw new Error(`No tab with id: ${tabId}.`);
          return { ...tab };
        },
        sendMessage: (tabId: number, message: unknown, options?: { frameId?: number }) =>
          this.deliver(
            this.contexts.filter((c) => c.kind === 'content' && c.tabId === tabId && (options?.frameId === undefined || c.frameId === options.frameId)),
            message,
            { id: this.id, url: ctx.url },
          ),
        onRemoved: this.tabRemoved,
      },
      commands: { onCommand: this.command },
      action: {
        setBadgeText: async ({ text, tabId }: { text: string; tabId?: number }) => void this.badgeText.set(tabId ?? -1, text),
        getBadgeText: async ({ tabId }: { tabId?: number }) => this.badgeText.get(tabId ?? -1) ?? '',
        setBadgeBackgroundColor: async () => {},
        setTitle: async () => {},
      },
    };
  }

  /** contentScript returns the API object of a content script in tabId. */
  contentScript(tabId: number, frameId = 0): FakeChromeApi {
    const ctx: Context = { kind: 'content', url: this.tabs.get(tabId)?.url ?? 'https://github.com/', tabId, frameId, onMessage: new FakeEvent() };
    this.contexts.push(ctx);
    return {
      ...this.commonApi(ctx),
      storage: untrustedStorage(this.storage, this.storageChanged, () => this.contexts.includes(ctx)),
    };
  }

  private commonApi(ctx: Context): Omit<FakeChromeApi, 'storage'> {
    const sender = (): MessageSender => {
      if (ctx.kind === 'extension') return { id: this.id, url: ctx.url };
      const tab = ctx.tabId === undefined ? undefined : this.tabs.get(ctx.tabId);
      return { id: this.id, url: ctx.url, frameId: ctx.frameId, tab: tab ? { ...tab } : undefined };
    };
    return {
      runtime: {
        id: this.id,
        lastError: undefined,
        getURL: (path: string) => this.url(path),
        getManifest: () => JSON.parse(readFileSync(join(pkgRoot, 'manifest.json'), 'utf8')) as unknown,
        sendMessage: (message: unknown) =>
          this.deliver(this.contexts.filter((c) => c.kind === 'extension' && c !== ctx), message, sender()),
        onMessage: ctx.onMessage,
      },
      i18n: {
        getMessage: (key: string, subs?: string | (string | number)[]) => this.getMessage(key, subs),
        getUILanguage: () => this.locale,
      },
    };
  }

  private url(path: string): string {
    return `chrome-extension://${this.id}/${path.replace(/^\//, '')}`;
  }

  private query(q: TabQuery): FakeTab[] {
    return [...this.tabs.values()]
      .filter((t) => q.active === undefined || t.active === q.active)
      .filter((t) => q.windowId === undefined || t.windowId === q.windowId)
      .filter((t) => !q.currentWindow || t.windowId === this.currentWindowId)
      .filter((t) => q.url === undefined || t.url === q.url)
      .map((t) => ({ ...t }));
  }

  /**
   * deliver follows Chrome's one-time message rules: listeners run
   * asynchronously on JSON copies, the first sendResponse wins, a listener
   * keeps the channel open only by returning true (the fake ignores a
   * returned Promise), and the sender gets undefined when nobody answers.
   */
  private deliver(targets: Context[], message: unknown, sender: MessageSender): Promise<unknown> {
    const listeners = targets.flatMap((c) => c.onMessage.listeners);
    if (listeners.length === 0) return Promise.reject(new Error(noReceiver));
    const msg = jsonClone(message);
    const p = new Promise<unknown>((resolve) => {
      // A microtask keeps delivery asynchronous and independent of vi.useFakeTimers().
      void Promise.resolve().then(() => {
        let answered = false;
        let open = false;
        const respond = (r?: unknown) => {
          if (answered) return;
          answered = true;
          resolve(jsonClone(r));
        };
        for (const l of listeners) {
          try {
            if (l(msg, jsonClone(sender), respond) === true) open = true;
          } catch (e) {
            this.listenerErrors.push(e);
          }
        }
        if (!open) respond(undefined);
      });
    });
    this.pending.add(p);
    void p.finally(() => this.pending.delete(p));
    return p;
  }
}

export interface TabQuery { active?: boolean; currentWindow?: boolean; windowId?: number; url?: string }

/** The shape of one context's fake chrome object. */
export interface FakeChromeApi {
  runtime: {
    id: string;
    lastError: undefined;
    getURL(path: string): string;
    getManifest(): unknown;
    sendMessage(message: unknown): Promise<unknown>;
    onMessage: FakeEvent<MessageListener>;
  };
  storage: Record<AreaName, StorageAreaApi> & { onChanged: FakeEvent<ChangeListener> };
  i18n: { getMessage(key: string, subs?: string | (string | number)[]): string; getUILanguage(): string };
  tabs?: {
    query(q: TabQuery): Promise<FakeTab[]>;
    get(tabId: number): Promise<FakeTab>;
    sendMessage(tabId: number, message: unknown, options?: { frameId?: number }): Promise<unknown>;
    onRemoved: FakeChrome['tabRemoved'];
  };
  commands?: { onCommand: FakeChrome['command'] };
  action?: {
    setBadgeText(d: { text: string; tabId?: number }): Promise<void>;
    getBadgeText(d: { tabId?: number }): Promise<string>;
    setBadgeBackgroundColor(d: unknown): Promise<void>;
    setTitle(d: unknown): Promise<void>;
  };
}

/** The API object of a trusted context, whose storage areas are the hub's own. */
export interface FakeExtensionApi extends FakeChromeApi {
  storage: Record<AreaName, FakeStorageArea> & { onChanged: FakeEvent<ChangeListener> };
}

/**
 * asChrome presents a fake as the chrome namespace for code typed against
 * @types/chrome. The fake implements only the members the extension uses,
 * so the cast cannot be checked structurally.
 */
export function asChrome(api: FakeChromeApi): typeof chrome {
  return api as unknown as typeof chrome;
}

/** installChrome sets globalThis.chrome to api and returns a function that restores the previous value. */
export function installChrome(api: FakeChromeApi): () => void {
  const g = globalThis as unknown as { chrome?: unknown };
  const had = 'chrome' in g;
  const prev = g.chrome;
  g.chrome = api;
  return () => {
    if (had) g.chrome = prev;
    else delete g.chrome;
  };
}

/** installFakeChrome creates a hub and installs a service worker context on globalThis. */
export function installFakeChrome(opts?: FakeChromeOptions): { hub: FakeChrome; api: FakeExtensionApi; restore: () => void } {
  const hub = new FakeChrome(opts);
  const api = hub.extensionContext();
  return { hub, api, restore: installChrome(api) };
}
