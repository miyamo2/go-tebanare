// Per-tab state (plan 6.7): hiding on or off, and the head config preview
// (plan 6.5). The state lives in chrome.storage.session under "tab:<id>",
// so it outlives service worker restarts, and goes away with the tab.
//
// The preview is a temporary switch for one pull request. The tab stores
// the pull request it was turned on for, answers headPreview: true only to
// a content script showing that pull request, and ends the preview when a
// content script in the tab reports another one. A PR author's head config
// therefore never decides what is hidden in a pull request the reviewer
// did not choose to preview.

import { sendToTab, type TabState, type TabStateMessage, type TabsTransport } from '../shared/messages.js';
import { setEvicting, type SessionArea } from './cache.js';

/** The keyboard command that flips hiding for the active tab (manifest "commands"). */
export const TOGGLE_COMMAND = 'toggle-hiding';

/** What TabStates stores for one tab. */
export interface StoredTabState {
  enabled: boolean;
  /** The pullRequestKey the head config preview is on for, or null. */
  headPreviewFor: string | null;
}

/** A change from set-tab-state. headPreview: true needs pr. */
export interface TabPatch {
  enabled?: boolean;
  headPreview?: boolean;
  pr?: string;
}

export const DEFAULT_TAB_STATE: Readonly<StoredTabState> = Object.freeze({ enabled: true, headPreviewFor: null });

export function tabKey(tabId: number): string {
  return `tab:${tabId}`;
}

// The shape pullRequestKey produces from a URL that parsePullUrl accepts.
const PULL_REQUEST_KEY = /^[a-z0-9][a-z0-9-]{0,38}\/[a-z0-9._-]{1,100}#[1-9][0-9]{0,9}$/;

/** checkPullRequestKey returns v in lower case, or throws when it is not a pullRequestKey. */
export function checkPullRequestKey(v: unknown): string {
  const key = typeof v === 'string' ? v.toLowerCase() : '';
  if (!PULL_REQUEST_KEY.test(key)) throw new TypeError('pr must be "<owner>/<repo>#<number>"');
  return key;
}

/** tabStateMessage is the tab-state message that tells a content script about s. */
export function tabStateMessage(s: StoredTabState): TabStateMessage {
  if (s.headPreviewFor === null) return { type: 'tab-state', enabled: s.enabled, headPreview: false };
  return { type: 'tab-state', enabled: s.enabled, headPreview: true, pr: s.headPreviewFor };
}

/** The chrome.tabs calls TabStates makes. */
export interface TabsApi extends TabsTransport {
  query(q: { active: boolean; currentWindow: boolean }): Promise<readonly { id?: number }[]>;
}

export class TabStates {
  // Updates run one at a time, so two quick toggles read each other's writes.
  #queue: Promise<unknown> = Promise.resolve();

  constructor(
    private readonly area: SessionArea,
    private readonly tabs: TabsApi,
  ) {}

  /** get returns the stored state of tabId, with defaults for missing or malformed fields. */
  async get(tabId: number): Promise<StoredTabState> {
    const key = tabKey(tabId);
    const v: unknown = (await this.area.get(key))[key];
    const s = typeof v === 'object' && v !== null ? (v as Partial<Record<keyof StoredTabState, unknown>>) : {};
    return {
      enabled: typeof s.enabled === 'boolean' ? s.enabled : DEFAULT_TAB_STATE.enabled,
      headPreviewFor: typeof s.headPreviewFor === 'string' ? s.headPreviewFor : DEFAULT_TAB_STATE.headPreviewFor,
    };
  }

  /**
   * visit answers get-tab-state from the content script of tabId, which
   * shows the pull request pr (a checked pullRequestKey, or undefined).
   * headPreview is true only when the preview is on for pr. A preview on
   * for another pull request ends. visit sends no tab-state message.
   */
  async visit(tabId: number, pr: string | undefined): Promise<TabState> {
    return this.#serial(async () => {
      const s = await this.get(tabId);
      const headPreview = pr !== undefined && s.headPreviewFor === pr;
      if (pr !== undefined && s.headPreviewFor !== null && !headPreview) {
        try {
          await setEvicting(this.area, { [tabKey(tabId)]: { ...s, headPreviewFor: null } });
        } catch {
          // The answer is right without the write. A kept headPreviewFor
          // still applies only to its own pull request.
        }
      }
      return { enabled: s.enabled, headPreview };
    });
  }

  /**
   * update applies patch to the state of tabId, stores it, and sends it to
   * the tab as a tab-state message. headPreview: true turns the preview on
   * for patch.pr and rejects without it; headPreview: false turns it off.
   */
  update(tabId: number, patch: TabPatch): Promise<StoredTabState> {
    if (patch.headPreview === true && patch.pr === undefined) {
      return Promise.reject(new TypeError('headPreview: true needs pr'));
    }
    return this.#change(tabId, (s) => ({
      enabled: patch.enabled ?? s.enabled,
      headPreviewFor: patch.headPreview === undefined ? s.headPreviewFor : patch.headPreview ? (patch.pr ?? null) : null,
    }));
  }

  /** toggle flips enabled for tabId. */
  toggle(tabId: number): Promise<StoredTabState> {
    return this.#change(tabId, (s) => ({ ...s, enabled: !s.enabled }));
  }

  /** remove drops the state of a closed tab. */
  remove(tabId: number): Promise<void> {
    return this.#serial(() => this.area.remove(tabKey(tabId)));
  }

  /**
   * runCommand handles commands.onCommand. For TOGGLE_COMMAND it toggles
   * tab, or the active tab of the current window when Chrome passes none.
   * It returns the new state, or null when nothing changed.
   */
  async runCommand(command: string, tab?: { id?: number }): Promise<StoredTabState | null> {
    if (command !== TOGGLE_COMMAND) return null;
    const id = tab?.id ?? (await this.tabs.query({ active: true, currentWindow: true }))[0]?.id;
    return id === undefined ? null : this.toggle(id);
  }

  async #change(tabId: number, f: (s: StoredTabState) => StoredTabState): Promise<StoredTabState> {
    const next = await this.#serial(async () => {
      const state = f(await this.get(tabId));
      await setEvicting(this.area, { [tabKey(tabId)]: state });
      return state;
    });
    // A tab without a content script (any page other than github.com) ignores it.
    await sendToTab(tabId, tabStateMessage(next), this.tabs);
    return next;
  }

  #serial<T>(f: () => Promise<T>): Promise<T> {
    const run = this.#queue.then(f, f);
    this.#queue = run.catch(() => undefined);
    return run;
  }
}
