// Fold rows stand in for hidden diff rows (plan 6.7). describeFold turns a
// run of hidden rows into a FoldView, and createFoldRow draws it: a fold of
// up to THIN_MAX_LINES lines is a thin separator, a longer one is a bar
// with a summary, the rule names, and a Show button.

import type { ChangeResult, Hit, RuleInfo } from '@go-tebanare/engine';
import { countLines, t } from '../../shared/i18n.js';
import { rowHits, type PlanRow } from '../plan.js';

/** Marks every fold row; the value is the fold key. */
export const FOLD_ATTR = 'data-gotebanare-fold';
export const THIN_ATTR = 'data-gotebanare-thin';

/** Folds of at most this many lines render as a thin separator. */
export const THIN_MAX_LINES = 3;

export interface FoldRule {
  id: string;
  description?: string;
  /** Labels of the matched nodes, such as "func (*MockStore) Get". */
  labels: readonly string[];
}

export interface FoldView {
  /** Identifies the fold within its file; stored in FOLD_ATTR. */
  key: string;
  lines: number;
  deleted: number;
  added: number;
  rules: readonly FoldRule[];
  /** Number of table columns the fold row spans. */
  colSpan: number;
}

/** A hidden row and its table row element. */
export interface FoldRow extends PlanRow {
  el: Element;
}

/** hitsOf returns the hits that hide row: old side first, then new side. */
export function hitsOf(row: PlanRow, result: ChangeResult): Hit[] {
  const hits = rowHits(row, result);
  return hits ? [...hits.old, ...hits.new] : [];
}

function rowWidth(tr: Element): number {
  let width = 0;
  for (const cell of tr.children) {
    if (cell.tagName === 'TD' || cell.tagName === 'TH') width += Math.max(1, (cell as HTMLTableCellElement).colSpan);
  }
  return width;
}

/**
 * describeFold summarizes a run of hidden rows. Rules come from the hits of
 * the rows in order of first appearance, or from planRules when the rows
 * have no hits. The fold spans as many columns as its first row.
 */
export function describeFold(
  key: string,
  run: readonly FoldRow[],
  planRules: readonly string[],
  result: ChangeResult,
  rules: readonly RuleInfo[],
): FoldView {
  let deleted = 0;
  let added = 0;
  const labels = new Map<string, string[]>();
  for (const row of run) {
    if (row.kind === 'del') deleted++;
    if (row.kind === 'add') added++;
    for (const hit of hitsOf(row, result)) {
      const list = labels.get(hit.ruleId) ?? [];
      labels.set(hit.ruleId, list);
      if (hit.label && !list.includes(hit.label)) list.push(hit.label);
    }
  }
  if (labels.size === 0) for (const id of planRules) labels.set(id, []);
  const descriptions = new Map(rules.map((r) => [r.id, r.description]));
  const foldRules = [...labels].map(([id, l]): FoldRule => {
    const description = descriptions.get(id);
    return description ? { id, description, labels: l } : { id, labels: l };
  });
  const colSpan = run[0] ? rowWidth(run[0].el) : 1;
  return { key, lines: run.length, deleted, added, rules: foldRules, colSpan };
}

function ruleTitle(rule: FoldRule): string {
  const parts: string[] = [];
  if (rule.description) parts.push(rule.description);
  if (rule.labels.length > 0) parts.push(t('foldRuleMatched', rule.labels.join(', ')));
  return parts.join('\n');
}

function appendRules(doc: Document, td: HTMLElement, rules: readonly FoldRule[]): void {
  if (rules.length === 0) return;
  const list = doc.createElement('span');
  list.className = 'gotebanare-fold-rules';
  rules.forEach((rule, i) => {
    if (i > 0) list.append(', ');
    const name = doc.createElement('span');
    name.className = 'gotebanare-fold-rule';
    name.textContent = rule.id;
    const title = ruleTitle(rule);
    if (title) name.title = title;
    list.append(name);
  });
  td.append(list);
}

function thinTooltip(fold: FoldView): string {
  const lines = countLines(fold.lines);
  const ids = fold.rules.map((r) => r.id).filter((id) => id !== '');
  return ids.length > 0 ? t('thinTooltip', lines, ids.join(', ')) : t('thinTooltipNoRules', lines);
}

function fillThin(td: HTMLTableCellElement, fold: FoldView, onExpand: () => void): void {
  const tip = thinTooltip(fold);
  td.title = tip;
  td.tabIndex = 0;
  td.setAttribute('role', 'button');
  td.setAttribute('aria-label', tip);
  td.addEventListener('click', onExpand);
  td.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter' && e.key !== ' ') return;
    e.preventDefault();
    onExpand();
  });
}

function fillBar(doc: Document, td: HTMLTableCellElement, fold: FoldView, onExpand: () => void): void {
  const icon = doc.createElement('span');
  icon.className = 'gotebanare-fold-icon';
  icon.setAttribute('aria-hidden', 'true');
  const text = doc.createElement('span');
  text.className = 'gotebanare-fold-text';
  text.textContent = t('foldSummary', countLines(fold.lines), fold.deleted, fold.added);
  td.append(icon, text);
  appendRules(doc, td, fold.rules);
  const show = doc.createElement('button');
  show.type = 'button';
  show.className = 'gotebanare-fold-show';
  show.textContent = t('foldShow');
  show.addEventListener('click', onExpand);
  td.append(show);
}

/** createFoldRow builds a detached fold row that calls onExpand when the reader opens it. */
export function createFoldRow(doc: Document, fold: FoldView, onExpand: () => void): HTMLTableRowElement {
  const tr = doc.createElement('tr');
  tr.setAttribute(FOLD_ATTR, fold.key);
  const td = doc.createElement('td');
  td.colSpan = Math.max(1, fold.colSpan);
  tr.append(td);
  if (fold.lines <= THIN_MAX_LINES) {
    tr.setAttribute(THIN_ATTR, '');
    fillThin(td, fold, onExpand);
  } else {
    fillBar(doc, td, fold, onExpand);
  }
  return tr;
}
