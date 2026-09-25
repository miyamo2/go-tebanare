// Message types exchanged between the content script, the popup, and the
// background service worker, plus typed senders.

import type { ChangeResult, Diagnostic, RuleInfo } from '@go-tebanare/engine';

/** One changed file as sent to the background. A null side is absent. */
export interface ChangeInput {
  oldPath?: string;
  newPath?: string;
  old: string | null;
  new: string | null;
}

/**
 * A cached analysis result. It holds no full source, only line hashes, but
 * the hit labels and some diagnostic messages in result quote up to 60
 * runes of normalized hidden code for the fold tooltip.
 */
export interface AnalysisRecord {
  result: ChangeResult;
  /** Maps each hidden old line number to the lineHash of that source line. */
  oldLines: Record<string, string>;
  /** Maps each hidden new line number to the lineHash of that source line. */
  newLines: Record<string, string>;
}

export type ConfigSource = 'base' | 'head';

export type PageState =
  | 'inactive'
  | 'excluded'
  | 'loading'
  | 'no-config'
  | 'config-error'
  | 'unsupported-ui'
  | 'ready'
  | 'error';

/** What the content script reports to the popup. */
export interface PageStatus {
  state: PageState;
  repo?: string;
  pr?: number;
  configPath?: string;
  configSource: ConfigSource;
  rules: number;
  files: number;
  filesWithFolds: number;
  linesHidden: number;
  /** Banner texts shown on the page. */
  messages: string[];
  enabled: boolean;
  headPreview: boolean;
}

// Requests to the background (from the content script or the popup).

export interface CompileRequest { type: 'compile'; yaml: string }
export interface LookupRequest { type: 'lookup'; cacheKey: string }
export interface AnalyzeRequest { type: 'analyze'; yaml: string; cacheKey: string; change: ChangeInput }
/**
 * Asks for the state of the sender's tab. pr is the pullRequestKey of the
 * pull request the content script shows; without it headPreview is false.
 */
export interface GetTabStateRequest { type: 'get-tab-state'; pr?: string }
/**
 * Changes the state of tabId, or of the sender's tab when tabId is absent.
 * headPreview: true needs pr, the pullRequestKey to preview.
 */
export interface SetTabStateRequest { type: 'set-tab-state'; tabId?: number; enabled?: boolean; headPreview?: boolean; pr?: string }

export type BgRequest =
  | CompileRequest
  | LookupRequest
  | AnalyzeRequest
  | GetTabStateRequest
  | SetTabStateRequest;

export type BgRequestType = BgRequest['type'];
export type BgRequestOf<K extends BgRequestType> = Extract<BgRequest, { type: K }>;

/**
 * Every failed request answers with Failure. A compile of an invalid
 * config also carries the diagnostics; transport failures carry none.
 */
export interface Failure { ok: false; error: string; diagnostics?: Diagnostic[] }

export interface TabState { enabled: boolean; headPreview: boolean }

export interface BgResponseMap {
  compile: { ok: true; configKey: string; diagnostics: Diagnostic[]; rules: RuleInfo[]; engineVersion: string } | Failure;
  lookup: { ok: true; record: AnalysisRecord | null } | Failure;
  analyze: { ok: true; record: AnalysisRecord } | Failure;
  'get-tab-state': ({ ok: true } & TabState) | Failure;
  'set-tab-state': { ok: true } | Failure;
}

export type BgResponse<K extends BgRequestType> = BgResponseMap[K];

// Messages to the content script of one tab (from the background or the popup).

/**
 * Sent to a tab after its state changes. When headPreview is true, pr names
 * the pull request it applies to; see headPreviewApplies.
 */
export interface TabStateMessage extends TabState { type: 'tab-state'; pr?: string }
export interface StatusMessage { type: 'status' }

export type TabMessage = TabStateMessage | StatusMessage;
export type TabMessageType = TabMessage['type'];
export type TabMessageOf<K extends TabMessageType> = Extract<TabMessage, { type: K }>;

export interface TabResponseMap {
  'tab-state': void;
  status: PageStatus;
}

/**
 * pullRequestKey names a pull request in tab state messages as
 * "<owner>/<repo>#<number>" in lower case, because GitHub matches owner and
 * repository names without regard to case. repo is "<owner>/<repo>".
 */
export function pullRequestKey(repo: string, number: number): string {
  return `${repo.toLowerCase()}#${number}`;
}

/**
 * headPreviewApplies reports whether msg turns on the head config preview
 * for the pull request pr (a pullRequestKey). A preview turned on for
 * another pull request does not apply.
 */
export function headPreviewApplies(msg: TabStateMessage, pr: string): boolean {
  return msg.headPreview && msg.pr === pr;
}

const bgRequestTypes: ReadonlySet<string> = new Set<BgRequestType>([
  'compile',
  'lookup',
  'analyze',
  'get-tab-state',
  'set-tab-state',
]);
const tabMessageTypes: ReadonlySet<string> = new Set<TabMessageType>(['tab-state', 'status']);

function typeOf(msg: unknown): unknown {
  return typeof msg === 'object' && msg !== null ? (msg as { type?: unknown }).type : undefined;
}

/** isBgRequest checks the type field only; handlers validate the rest. */
export function isBgRequest(msg: unknown): msg is BgRequest {
  const t = typeOf(msg);
  return typeof t === 'string' && bgRequestTypes.has(t);
}

/** isTabMessage checks the type field only. */
export function isTabMessage(msg: unknown): msg is TabMessage {
  const t = typeOf(msg);
  return typeof t === 'string' && tabMessageTypes.has(t);
}

/** errorText returns a one-line description of a thrown value. */
export function errorText(e: unknown): string {
  if (e instanceof Error) return e.message || e.name;
  return String(e);
}

/** toFailure wraps a thrown value in a Failure response. */
export function toFailure(e: unknown): Failure {
  return { ok: false, error: errorText(e) };
}

/** The part of chrome.runtime that send() uses. */
export interface RuntimeTransport {
  sendMessage(message: unknown): Promise<unknown>;
}

/** The part of chrome.tabs that sendToTab() uses. */
export interface TabsTransport {
  sendMessage(tabId: number, message: unknown): Promise<unknown>;
}

type Fields = Record<string, unknown>;

function isFields(v: unknown): v is Fields {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

function isAnalysisRecord(v: unknown): boolean {
  return isFields(v) && isFields(v['result']) && isFields(v['oldLines']) && isFields(v['newLines']);
}

// The fields each successful response must carry. Nested engine DTOs are
// checked only for being objects or arrays.
const successShape: Readonly<Record<BgRequestType, (r: Fields) => boolean>> = {
  compile: (r) =>
    typeof r['configKey'] === 'string' &&
    Array.isArray(r['diagnostics']) &&
    Array.isArray(r['rules']) &&
    typeof r['engineVersion'] === 'string',
  lookup: (r) => r['record'] === null || isAnalysisRecord(r['record']),
  analyze: (r) => isAnalysisRecord(r['record']),
  'get-tab-state': (r) => typeof r['enabled'] === 'boolean' && typeof r['headPreview'] === 'boolean',
  'set-tab-state': () => true,
};

function isResponse(type: BgRequestType, r: unknown): boolean {
  if (!isFields(r)) return false;
  if (r['ok'] === true) return successShape[type](r);
  return (
    r['ok'] === false &&
    typeof r['error'] === 'string' &&
    (r['diagnostics'] === undefined || Array.isArray(r['diagnostics']))
  );
}

/**
 * send delivers a request to the background and resolves with its answer.
 * It never rejects: a missing chrome.runtime, a transport error, or an
 * answer without the fields of BgResponse<type> becomes a Failure.
 * transport defaults to chrome.runtime.
 */
export async function send<R extends BgRequest>(req: R, transport?: RuntimeTransport): Promise<BgResponse<R['type']>> {
  const tr = transport ?? globalThis.chrome?.runtime;
  if (!tr) return { ok: false, error: 'chrome.runtime unavailable' };
  let res: unknown;
  try {
    res = await tr.sendMessage(req);
  } catch (e) {
    return toFailure(e);
  }
  if (!isResponse(req.type, res)) return { ok: false, error: `no valid response to ${req.type}` };
  // isResponse checked the fields that BgResponse<type> declares.
  return res as BgResponse<R['type']>;
}

/**
 * sendToTab delivers a message to the content script of tabId. It never
 * rejects, and resolves with undefined when chrome.tabs is unavailable (in
 * a content script) or no content script answered (for example on pages
 * other than github.com). It does not check the shape of the answer.
 * transport defaults to chrome.tabs.
 */
export async function sendToTab<M extends TabMessage>(
  tabId: number,
  msg: M,
  transport?: TabsTransport,
): Promise<TabResponseMap[M['type']] | undefined> {
  const tr = transport ?? globalThis.chrome?.tabs;
  if (!tr) return undefined;
  try {
    // The content script answers every message type with TabResponseMap[type].
    return (await tr.sendMessage(tabId, msg)) as TabResponseMap[M['type']] | undefined;
  } catch {
    return undefined;
  }
}
