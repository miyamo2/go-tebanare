// Shared setup for the applyFile tests: classic-modified.html with a result
// that hides a new field, the Owner getter, and most of Put.

import type { ChangeResult, Hit, RuleInfo } from '@go-tebanare/engine';
import type { ApplyContext } from '../../src/content/apply.js';
import { classicVariant as classic } from '../../src/content/dom/classic.js';
import type { RowRef } from '../../src/content/dom/variant.js';
import { planVisibility } from '../../src/content/plan.js';
import { onlyFile } from './classic-fixture.js';

export const hit = (ruleId: string, label: string): Hit => ({ ruleId, target: 'func', node: 'FuncDecl', label });
export const change = (old: ChangeResult['old'], nu: ChangeResult['new']): ChangeResult => ({ old, new: nu, diagnostics: [], skipped: '' });

export const result = change(
  [{ start: 27, end: 34, hits: [hit('put', 'func (*Store) Put')] }],
  [
    { start: 12, end: 12, hits: [hit('fields', 'owner string')] },
    { start: 26, end: 29, hits: [hit('getters', 'func (*Store) Owner')] },
    { start: 33, end: 40, hits: [hit('put', 'func (*Store) Put')] },
  ],
);
export const rules: RuleInfo[] = [{ id: 'getters', description: 'Plain getters', target: 'func' }];

/** load reads a one-file fixture and its rows. */
export function load(name: string): { container: HTMLElement; rows: RowRef[] } {
  const container = onlyFile(name);
  return { container, rows: [...classic.rows(container)] };
}

/** setup loads classic-modified.html and plans result; original is the markup before applyFile. */
export function setup(extra: Partial<ApplyContext> = {}) {
  const { container, rows } = load('classic-modified.html');
  const plan = planVisibility(rows, result);
  const ctx: ApplyContext = { result, rules, expanded: new Set(), badgeHost: classic.fileHeader?.(container) ?? null, ...extra };
  return { container, rows, plan, ctx, original: document.body.innerHTML };
}

export const isHidden = (el: Element | undefined) => el?.hasAttribute('data-gotebanare-hidden');
export const hiddenRows = (c: HTMLElement) => [...c.querySelectorAll('[data-gotebanare-hidden]')];
export const foldRows = (c: HTMLElement) => [...c.querySelectorAll<HTMLElement>('[data-gotebanare-fold]')];

/** mutations runs fn and returns the DOM changes it made. */
export function mutations(fn: () => void): MutationRecord[] {
  const observer = new MutationObserver(() => {});
  observer.observe(document.body, { childList: true, subtree: true, attributes: true, characterData: true });
  fn();
  const records = observer.takeRecords();
  observer.disconnect();
  return records;
}
