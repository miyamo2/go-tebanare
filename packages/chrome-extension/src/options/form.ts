// DOM access for options.html, kept apart from storage so tests can
// render every status without chrome.storage.

import { t } from '../shared/i18n.js';
import type { Options } from '../shared/settings.js';
import type { InvalidLine } from './patterns.js';

export interface OptionsElements {
  form: HTMLFormElement;
  repos: HTMLTextAreaElement;
  debug: HTMLInputElement;
  save: HTMLButtonElement;
  status: HTMLElement;
  invalid: HTMLElement;
}

/** The result of the last load or save, shown under the Save button. */
export type SaveStatus =
  | { kind: 'idle' }
  | { kind: 'saving' }
  | { kind: 'saved' }
  | { kind: 'invalid'; lines: readonly InvalidLine[] }
  | { kind: 'save-failed'; error: string }
  | { kind: 'load-failed'; error: string };

function byId<T extends HTMLElement>(doc: Document, id: string, type: new () => T): T {
  const el = doc.getElementById(id);
  if (!(el instanceof type)) throw new Error(`options.html has no ${type.name} #${id}`);
  return el;
}

/** optionsElements finds the form controls of options.html and throws when one is missing. */
export function optionsElements(doc: Document): OptionsElements {
  const win = doc.defaultView;
  if (!win) throw new Error('options.html has no window');
  return {
    form: byId(doc, 'options-form', win.HTMLFormElement),
    repos: byId(doc, 'excluded-repos', win.HTMLTextAreaElement),
    debug: byId(doc, 'debug', win.HTMLInputElement),
    save: byId(doc, 'save', win.HTMLButtonElement),
    status: byId(doc, 'save-status', win.HTMLElement),
    invalid: byId(doc, 'invalid-lines', win.HTMLElement),
  };
}

/** fillForm shows opts in the form, one pattern per line. */
export function fillForm(el: OptionsElements, opts: Options): void {
  el.repos.value = opts.excludedRepos.join('\n');
  el.debug.checked = opts.debug;
}

function statusText(s: SaveStatus): { level: string; text: string } {
  switch (s.kind) {
    case 'idle':
    case 'saving':
      return { level: '', text: '' };
    case 'saved':
      return { level: 'success', text: t('optionsSaved') };
    case 'invalid':
      return { level: 'error', text: t('optionsNotSaved') };
    case 'save-failed':
      return { level: 'error', text: t('optionsSaveFailed', s.error) };
    case 'load-failed':
      return { level: 'error', text: t('optionsLoadFailed', s.error) };
  }
}

/**
 * renderStatus shows s in the status line and lists the lines that are not
 * patterns. Save stays off while saving and after a failed load, so a form
 * that never showed the stored options cannot overwrite them.
 */
export function renderStatus(el: OptionsElements, s: SaveStatus): void {
  const { level, text } = statusText(s);
  el.status.textContent = text;
  if (level === '') delete el.status.dataset['level'];
  else el.status.dataset['level'] = level;
  const doc = el.invalid.ownerDocument;
  const items = (s.kind === 'invalid' ? s.lines : []).map((l) => {
    const li = doc.createElement('li');
    li.textContent = t('optionsInvalidLine', l.line, l.text);
    return li;
  });
  el.invalid.replaceChildren(...items);
  el.invalid.hidden = items.length === 0;
  el.save.disabled = s.kind === 'saving' || s.kind === 'load-failed';
}
