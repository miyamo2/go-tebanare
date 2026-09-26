// The service worker. It answers BgRequest messages from the content script
// and the popup, keeps per-tab state, and handles the toggle-hiding
// command. Chrome delivers events to a restarted worker only through
// listeners added in the first turn of the event loop, so startBackground
// adds them synchronously.

import { ConfigError } from '@go-tebanare/engine';
import {
  errorText,
  isBgRequest,
  toFailure,
  type BgRequest,
  type BgRequestOf,
  type BgRequestType,
  type BgResponse,
  type ChangeInput,
  type Failure,
} from '../shared/messages.js';
import { analyze } from './analysis.js';
import { AnalysisCache, assertAnalysisKey, type SessionArea } from './cache.js';
import { EngineHost, wasmEngineFactory, type EngineFactory } from './engine-host.js';
import { checkPullRequestKey, TabStates, type TabPatch, type TabsApi } from './tabs.js';

/** The fields of runtime.MessageSender that the handlers read. */
export interface Sender {
  tab?: { id?: number };
}

export type MessageListener = (message: unknown, sender: Sender, sendResponse: (response?: unknown) => void) => boolean;

interface EventLike<F> {
  addListener(f: F): void;
}

/** The chrome APIs the service worker uses. The real chrome namespace satisfies it. */
export interface BackgroundApi {
  runtime: { getURL(path: string): string; onMessage: EventLike<MessageListener> };
  storage: { session: SessionArea };
  tabs: TabsApi & { onRemoved: EventLike<(tabId: number) => void> };
  commands: { onCommand: EventLike<(command: string, tab?: { id?: number }) => void> };
}

type Handler<K extends BgRequestType> = (req: BgRequestOf<K>, sender: Sender) => Promise<BgResponse<K>>;
type Handlers = { readonly [K in BgRequestType]: Handler<K> };
type AnyResponse = BgResponse<BgRequestType>;

export interface Background {
  host: EngineHost;
  cache: AnalysisCache;
  tabStates: TabStates;
  /** handle answers one request. It rejects on errors; the message listener turns them into a Failure. */
  handle(req: BgRequest, sender: Sender): Promise<AnyResponse>;
  listener: MessageListener;
}

/**
 * startBackground wires the handlers to api's events. factory defaults to
 * loading engine.wasm from the extension package.
 */
export function startBackground(api: BackgroundApi, factory?: EngineFactory): Background {
  const host = new EngineHost(factory ?? wasmEngineFactory((path) => api.runtime.getURL(path)));
  const cache = new AnalysisCache(api.storage.session);
  const tabStates = new TabStates(api.storage.session, api.tabs);
  const handlers = createHandlers(host, cache, tabStates);
  const handle = (req: BgRequest, sender: Sender): Promise<AnyResponse> =>
    // handlers[req.type] takes BgRequestOf<req.type>; TypeScript cannot relate the two through the union.
    (handlers[req.type] as (r: BgRequest, s: Sender) => Promise<AnyResponse>)(req, sender);
  const listener = messageListener(handle);
  api.runtime.onMessage.addListener(listener);
  api.tabs.onRemoved.addListener((tabId) => void tabStates.remove(tabId).catch(logError));
  api.commands.onCommand.addListener((command, tab) => void tabStates.runCommand(command, tab).catch(logError));
  return { host, cache, tabStates, handle, listener };
}

/**
 * messageListener answers every BgRequest asynchronously and returns true
 * to keep the channel open. It returns false for other messages. A
 * rejected handler answers with a Failure.
 */
export function messageListener(handle: (req: BgRequest, sender: Sender) => Promise<unknown>): MessageListener {
  return (message, sender, sendResponse) => {
    if (!isBgRequest(message)) return false;
    const reply = (r: unknown) => {
      try {
        sendResponse(r);
      } catch {
        // The sender went away, for example because its tab closed.
      }
    };
    // The extra then() turns a synchronous throw from handle into a rejection.
    Promise.resolve()
      .then(() => handle(message, sender))
      .then(reply, (e: unknown) => reply(failureOf(e)));
    return true;
  };
}

/** failureOf turns a thrown value into a Failure, keeping ConfigError diagnostics. */
export function failureOf(e: unknown): Failure {
  if (e instanceof ConfigError) return { ok: false, error: errorText(e), diagnostics: e.diagnostics };
  return toFailure(e);
}

function createHandlers(host: EngineHost, cache: AnalysisCache, tabStates: TabStates): Handlers {
  return {
    async compile(req) {
      const rs = await host.compile(str(req.yaml, 'yaml'));
      return { ok: true, configKey: rs.key, diagnostics: rs.diagnostics, rules: rs.rules, engineVersion: await host.engineVersion() };
    },
    async lookup(req) {
      return { ok: true, record: await cache.get(str(req.cacheKey, 'cacheKey')) };
    },
    async analyze(req) {
      const key = str(req.cacheKey, 'cacheKey');
      assertAnalysisKey(key);
      const record = await analyze(host, str(req.yaml, 'yaml'), changeInput(req.change));
      await cache.put(key, record);
      return { ok: true, record };
    },
    async 'get-tab-state'(req, sender) {
      const id = sender.tab?.id;
      if (id === undefined) throw new Error('get-tab-state needs a sender tab');
      const pr = req.pr === undefined ? undefined : checkPullRequestKey(req.pr);
      return { ok: true, ...(await tabStates.visit(id, pr)) };
    },
    async 'set-tab-state'(req, sender) {
      const id = req.tabId ?? sender.tab?.id;
      if (typeof id !== 'number' || !Number.isInteger(id)) throw new TypeError('set-tab-state needs tabId or a sender tab');
      const patch: TabPatch = {};
      if (req.enabled !== undefined) patch.enabled = bool(req.enabled, 'enabled');
      if (req.headPreview !== undefined) patch.headPreview = bool(req.headPreview, 'headPreview');
      if (req.pr !== undefined) patch.pr = checkPullRequestKey(req.pr);
      await tabStates.update(id, patch);
      return { ok: true };
    },
  };
}

// isBgRequest checks the type field only, so the handlers check the rest.

function str(v: unknown, name: string): string {
  if (typeof v !== 'string') throw new TypeError(`${name} must be a string`);
  return v;
}

function bool(v: unknown, name: string): boolean {
  if (typeof v !== 'boolean') throw new TypeError(`${name} must be a boolean`);
  return v;
}

function changeInput(v: unknown): ChangeInput {
  if (typeof v !== 'object' || v === null) throw new TypeError('change must be an object');
  const c = v as Record<string, unknown>;
  const side = (k: 'old' | 'new') => (c[k] === null || c[k] === undefined ? null : str(c[k], `change.${k}`));
  const out: ChangeInput = { old: side('old'), new: side('new') };
  if (out.old === null && out.new === null) throw new TypeError('change needs an old or a new side');
  if (c['oldPath'] !== undefined) out.oldPath = str(c['oldPath'], 'change.oldPath');
  if (c['newPath'] !== undefined) out.newPath = str(c['newPath'], 'change.newPath');
  return out;
}

function logError(e: unknown): void {
  console.warn(`go-tebanare: ${errorText(e)}`);
}

// Chrome evaluates this module as the service worker. Tests import it without chrome.
if (typeof chrome !== 'undefined' && chrome.runtime?.onMessage) startBackground(chrome);
