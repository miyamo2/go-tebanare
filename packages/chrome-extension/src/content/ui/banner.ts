// The page banner sits above the first file of the diff and lists config
// errors, fetch failures, and the other reasons the extension hides less
// than usual, plus the head config preview (plan 6.7).

import type { Diagnostic } from '@go-tebanare/engine';
import { t } from '../../shared/i18n.js';
import type { FetchFailureReason } from '../fetcher.js';

export const BANNER_ATTR = 'data-gotebanare-banner';

export type BannerLevel = 'error' | 'warning' | 'info';

export interface BannerMessage {
  level: BannerLevel;
  text: string;
}

/** Something the banner reports. The controller collects these per page. */
export type Notice =
  | { kind: 'config-error'; path: string; diagnostics: readonly Diagnostic[] }
  | { kind: 'config-changed'; path: string }
  | { kind: 'both-configs' }
  | { kind: 'head-preview'; path: string }
  | { kind: 'config-fetch-failed'; path: string; reason: FetchFailureReason }
  | { kind: 'fetch-failed'; path: string; reason: FetchFailureReason }
  | { kind: 'source-mismatch'; path: string }
  | { kind: 'analysis-failed'; path: string; error: string }
  | { kind: 'context-error' }
  | { kind: 'unsupported-ui' }
  | { kind: 'split-view' };

const reasonKeys = {
  'not-found': 'fetchNotFound',
  unauthorized: 'fetchUnauthorized',
  sso: 'fetchSso',
  network: 'fetchNetwork',
  'too-large': 'fetchTooLarge',
} as const;

/** fetchReasonText explains a fetch failure in the UI language. */
export function fetchReasonText(reason: FetchFailureReason): string {
  return t(reasonKeys[reason]);
}

/** diagnosticText formats a diagnostic as "line 3, column 25, rule getter, field presets[0](getter).max_depth: message". */
export function diagnosticText(d: Diagnostic): string {
  const where: string[] = [];
  if (d.line) where.push(t('diagnosticLine', d.line));
  if (d.column) where.push(t('diagnosticColumn', d.column));
  if (d.ruleId) where.push(t('diagnosticRule', d.ruleId));
  if (d.field) where.push(t('diagnosticField', d.field));
  return where.length > 0 ? t('diagnosticText', where.join(', '), d.message) : d.message;
}

const one = (level: BannerLevel, text: string): BannerMessage[] => [{ level, text }];

/**
 * noticeMessages returns the banner lines for n. A config error gives a
 * summary line followed by one line per diagnostic.
 */
export function noticeMessages(n: Notice): BannerMessage[] {
  switch (n.kind) {
    case 'config-error':
      return [
        { level: 'error', text: t('bannerConfigError', n.path) },
        ...n.diagnostics.map((d): BannerMessage => ({ level: d.severity, text: diagnosticText(d) })),
      ];
    case 'config-changed':
      return one('warning', t('bannerConfigChanged', n.path));
    case 'both-configs':
      return one('warning', t('bannerBothConfigs'));
    case 'head-preview':
      return one('warning', t('bannerHeadPreview', n.path));
    case 'config-fetch-failed':
      return one('error', t('bannerConfigFetchFailed', n.path, fetchReasonText(n.reason)));
    case 'fetch-failed':
      return one('warning', t('bannerFetchFailed', n.path, fetchReasonText(n.reason)));
    case 'source-mismatch':
      return one('warning', t('bannerSourceMismatch', n.path));
    case 'analysis-failed':
      return one('warning', t('bannerAnalysisFailed', n.path, n.error));
    case 'context-error':
      return one('error', t('bannerContextError'));
    case 'unsupported-ui':
      return one('info', t('bannerUnsupportedUi'));
    case 'split-view':
      return one('info', t('bannerSplitView'));
  }
}

function createBanner(doc: Document, messages: readonly BannerMessage[]): HTMLElement {
  const banner = doc.createElement('div');
  banner.className = 'gotebanare-banner';
  banner.setAttribute(BANNER_ATTR, '');
  banner.setAttribute('role', 'status');
  const title = doc.createElement('div');
  title.className = 'gotebanare-banner-title';
  title.textContent = t('bannerTitle');
  const list = doc.createElement('ul');
  list.className = 'gotebanare-banner-messages';
  for (const m of messages) {
    const li = doc.createElement('li');
    li.dataset['level'] = m.level;
    li.textContent = m.text;
    list.append(li);
  }
  banner.append(title, list);
  return banner;
}

function dedupe(messages: readonly BannerMessage[]): BannerMessage[] {
  const seen = new Set<string>();
  return messages.filter((m) => {
    const key = `${m.level}\n${m.text}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

/** removeBanner removes every banner under root. */
export function removeBanner(root: ParentNode): void {
  for (const el of root.querySelectorAll(`[${BANNER_ATTR}]`)) el.remove();
}

/**
 * renderBanner shows messages in one banner placed right before anchor (the
 * first file container), or at the top of <main> or <body> when anchor is
 * null or no longer in doc (GitHub replaced it). Empty messages remove the
 * banner. A call that changes nothing leaves the DOM untouched, so it does
 * not wake a MutationObserver.
 */
export function renderBanner(doc: Document, messages: readonly BannerMessage[], anchor: Element | null): HTMLElement | null {
  const unique = dedupe(messages);
  if (unique.length === 0) {
    removeBanner(doc);
    return null;
  }
  const banner = createBanner(doc, unique);
  const existing = [...doc.querySelectorAll<HTMLElement>(`[${BANNER_ATTR}]`)];
  const at = anchor?.parentElement && doc.contains(anchor) ? anchor : null;
  const parent = at?.parentElement ?? doc.querySelector('main') ?? doc.body;
  const placed = (el: HTMLElement) =>
    at ? el.nextElementSibling === at : el.parentElement === parent && el === parent.firstElementChild;
  const [first] = existing;
  if (existing.length === 1 && first && placed(first) && first.outerHTML === banner.outerHTML) return first;
  for (const el of existing) el.remove();
  if (at) at.before(banner);
  else parent.prepend(banner);
  return banner;
}
