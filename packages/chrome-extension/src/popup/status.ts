// Reads the PageStatus a content script reports and decides what the popup
// offers for it.

import type { MessageKey } from '../shared/i18n.js';
import type { PageState, PageStatus } from '../shared/messages.js';

const stateKeys: Readonly<Record<PageState, MessageKey>> = {
  inactive: 'popupStateInactive',
  excluded: 'popupStateExcluded',
  loading: 'popupStateLoading',
  'no-config': 'popupStateNoConfig',
  'config-error': 'popupStateConfigError',
  'unsupported-ui': 'popupStateUnsupportedUi',
  ready: 'popupStateReady',
  error: 'popupStateError',
};

/** stateKey returns the message that names the state of s. */
export function stateKey(s: Pick<PageStatus, 'state' | 'configSource'>): MessageKey {
  if (s.state === 'no-config' && s.configSource === 'head') return 'popupStateNoConfigHead';
  return stateKeys[s.state];
}

const count = (v: unknown): v is number => typeof v === 'number' && Number.isSafeInteger(v) && v >= 0;
const optional = <T>(v: unknown, ok: (v: unknown) => v is T): v is T | undefined => v === undefined || ok(v);
const isString = (v: unknown): v is string => typeof v === 'string';
const isPositive = (v: unknown): v is number => count(v) && v > 0;

/**
 * parsePageStatus returns v when it has every PageStatus field with the
 * right type, and null otherwise (no content script, or one from another
 * version of the extension).
 */
export function parsePageStatus(v: unknown): PageStatus | null {
  if (typeof v !== 'object' || v === null) return null;
  const s = v as Record<string, unknown>;
  const valid =
    typeof s['state'] === 'string' &&
    Object.hasOwn(stateKeys, s['state']) &&
    (s['configSource'] === 'base' || s['configSource'] === 'head') &&
    optional(s['repo'], isString) &&
    optional(s['pr'], isPositive) &&
    optional(s['configPath'], isString) &&
    count(s['rules']) &&
    count(s['files']) &&
    count(s['filesWithFolds']) &&
    count(s['linesHidden']) &&
    Array.isArray(s['messages']) &&
    s['messages'].every(isString) &&
    typeof s['enabled'] === 'boolean' &&
    typeof s['headPreview'] === 'boolean';
  // valid checked every field PageStatus declares.
  return valid ? (s as unknown as PageStatus) : null;
}

/** The buttons the popup shows. */
export interface Controls {
  hiding: boolean;
  headPreview: boolean;
}

// The states of a pull request page where the tab state changes what the page shows.
const working: ReadonlySet<PageState> = new Set<PageState>(['loading', 'no-config', 'config-error', 'ready']);

/**
 * controlsFor returns the buttons to show for s. They show on pull request
 * pages the extension works on, and the head config preview needs the
 * repository and number to name the pull request. A button that undoes a
 * change (hiding off, preview on) shows in every state, so the reader can
 * always return to the defaults.
 */
export function controlsFor(s: PageStatus): Controls {
  const works = working.has(s.state);
  return {
    hiding: works || !s.enabled,
    headPreview: s.headPreview || (works && s.repo !== undefined && s.pr !== undefined),
  };
}
