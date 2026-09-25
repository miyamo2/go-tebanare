// Renders the popup (popup.html) from the status of the active tab. Pure
// DOM code: popup.ts does the messaging.

import { t } from '../shared/i18n.js';
import type { PageStatus } from '../shared/messages.js';
import { controlsFor, stateKey } from './status.js';

export interface PopupElements {
  headPreviewBanner: HTMLElement;
  state: HTMLElement;
  pullRequest: HTMLElement;
  noPage: HTMLElement;
  details: HTMLElement;
  configRow: HTMLElement;
  config: HTMLElement;
  /** The rows of the counts, shown when the page is ready. */
  statRows: HTMLElement[];
  rules: HTMLElement;
  files: HTMLElement;
  filesWithFolds: HTMLElement;
  linesHidden: HTMLElement;
  messagesSection: HTMLElement;
  messages: HTMLElement;
  error: HTMLElement;
  toggleHiding: HTMLButtonElement;
  toggleHeadPreview: HTMLButtonElement;
}

export interface PopupModel {
  /** The status of the active tab, or null when no content script answered. */
  status: PageStatus | null;
  /** The error of the last failed button action. */
  error?: string;
  /** True while a button action runs; the buttons are off. */
  busy: boolean;
}

function byId<T extends HTMLElement>(doc: Document, id: string, type: new () => T): T {
  const el = doc.getElementById(id);
  if (!(el instanceof type)) throw new Error(`popup.html has no ${type.name} #${id}`);
  return el;
}

/** popupElements finds the parts of popup.html and throws when one is missing. */
export function popupElements(doc: Document): PopupElements {
  const win = doc.defaultView;
  if (!win) throw new Error('popup.html has no window');
  const el = (id: string) => byId(doc, id, win.HTMLElement);
  const button = (id: string) => byId(doc, id, win.HTMLButtonElement);
  return {
    headPreviewBanner: el('head-preview-banner'),
    state: el('state'),
    pullRequest: el('pull-request'),
    noPage: el('no-page'),
    details: el('details'),
    configRow: el('config-row'),
    config: el('config'),
    statRows: [...doc.querySelectorAll<HTMLElement>('#details > [data-stat]')],
    rules: el('rules'),
    files: el('files'),
    filesWithFolds: el('files-with-folds'),
    linesHidden: el('lines-hidden'),
    messagesSection: el('messages-section'),
    messages: el('messages'),
    error: el('error'),
    toggleHiding: button('toggle-hiding'),
    toggleHeadPreview: button('toggle-head-preview'),
  };
}

/** show sets the text of el, and hides el when text is null. */
function show(el: HTMLElement, text: string | null): void {
  el.textContent = text ?? '';
  el.hidden = text === null;
}

function configText(s: PageStatus): string | null {
  if (s.configPath === undefined) return null;
  return t(s.configSource === 'head' ? 'popupConfigHead' : 'popupConfigBase', s.configPath);
}

/**
 * renderPopup shows m. The details list the config and the counts; the
 * counts show only when the page is ready. The head config preview notice
 * shows whenever the preview is on for the page (plan 6.5).
 */
export function renderPopup(el: PopupElements, m: PopupModel): void {
  const s = m.status;
  el.noPage.hidden = s !== null && s.state !== 'inactive';
  show(el.state, s && t(stateKey(s)));
  show(el.pullRequest, s?.repo !== undefined && s.pr !== undefined ? t('popupPullRequest', s.repo, s.pr) : null);
  el.headPreviewBanner.hidden = s?.headPreview !== true;

  show(el.config, s && configText(s));
  el.configRow.hidden = el.config.hidden;
  const ready = s?.state === 'ready';
  for (const row of el.statRows) row.hidden = !ready;
  el.rules.textContent = String(s?.rules ?? '');
  el.files.textContent = String(s?.files ?? '');
  el.filesWithFolds.textContent = String(s?.filesWithFolds ?? '');
  el.linesHidden.textContent = String(s?.linesHidden ?? '');
  el.details.hidden = el.configRow.hidden && !ready;

  const doc = el.messages.ownerDocument;
  const items = (s?.messages ?? []).map((text) => {
    const li = doc.createElement('li');
    li.textContent = text;
    return li;
  });
  el.messages.replaceChildren(...items);
  el.messagesSection.hidden = items.length === 0;
  show(el.error, m.error === undefined ? null : t('popupActionFailed', m.error));

  const controls = s ? controlsFor(s) : { hiding: false, headPreview: false };
  el.toggleHiding.hidden = !controls.hiding;
  el.toggleHiding.textContent = t(s?.enabled === false ? 'popupHidingOff' : 'popupHidingOn');
  el.toggleHeadPreview.hidden = !controls.headPreview;
  el.toggleHeadPreview.textContent = t(s?.headPreview === true ? 'popupHeadPreviewStop' : 'popupHeadPreviewStart');
  el.toggleHiding.disabled = m.busy;
  el.toggleHeadPreview.disabled = m.busy;
}
