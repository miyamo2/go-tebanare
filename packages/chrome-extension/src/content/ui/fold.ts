// Fold rows stand in for runs of hidden diff rows (plan 6.7). describeFold
// turns a run into a FoldView, and createFoldRow draws it the way GitHub
// draws the rows that expand hidden context: a row in the hunk color whose
// line number columns hold an icon button. Like GitHub's "@@ ... @@" hunk
// header, the code column holds a one-line summary of why the run is
// hidden. The button shows the run, and the same row then hides it again.
// Its tooltip names the rules behind the fold in full.

import type { ChangeResult, Hit, RuleInfo } from '@go-tebanare/engine';
import { countLines, t } from '../../shared/i18n.js';
import { rowHits, type PlanRow } from '../plan.js';
import { createIcon } from './icons.js';

/** Marks every fold row; the value is the fold key. */
export const FOLD_ATTR = 'data-gotebanare-fold';
/** Marks the fold row of a run the reader opened. */
export const OPEN_ATTR = 'data-gotebanare-open';

// A unified diff row has two line number columns before the code.
const GUTTER_COLUMNS = 2;

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
  const labels = new Map<string, string[]>();
  for (const row of run) {
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
  return { key, lines: run.length, rules: foldRules, colSpan };
}

// ruleLines describes each rule on its own line: the id with the rule's
// description, then the nodes it matched.
function ruleLines(rules: readonly FoldRule[]): string[] {
  const lines: string[] = [];
  for (const rule of rules) {
    if (rule.id === '') continue;
    lines.push(rule.description ? `${rule.id}: ${rule.description}` : rule.id);
    if (rule.labels.length > 0) lines.push(t('foldRuleMatched', rule.labels.join(', ')));
  }
  return lines;
}

// ruleSummary describes the rules on one line: each id with the nodes it
// matched in parentheses.
function ruleSummary(rules: readonly FoldRule[]): string {
  return rules
    .filter((rule) => rule.id !== '')
    .map((rule) => (rule.labels.length > 0 ? `${rule.id} (${rule.labels.join(', ')})` : rule.id))
    .join('; ');
}

/** foldSummary is the one-line text of a fold row, such as "4 lines hidden by gotebanare: getter (func (*Store) Owner)". */
export function foldSummary(fold: FoldView, open: boolean): string {
  const lead = t(open ? 'foldSummaryOpen' : 'foldSummaryClosed', countLines(fold.lines));
  const rules = ruleSummary(fold.rules);
  return rules ? `${lead}: ${rules}` : lead;
}

/**
 * createFoldRow builds a detached fold row for fold, open or closed, whose
 * button calls onToggle. The button fills a cell over the two line number
 * columns, and a cell with the fold summary covers the rest; a row
 * narrower than three columns gets one cell and no summary. The button's
 * label says what a click does. Its tooltip adds the rules, which screen
 * readers get as its description, so they skip the summary, which repeats
 * them. A click on the summary toggles the fold too.
 */
export function createFoldRow(doc: Document, fold: FoldView, open: boolean, onToggle: () => void): HTMLTableRowElement {
  const tr = doc.createElement('tr');
  tr.setAttribute(FOLD_ATTR, fold.key);
  if (open) tr.setAttribute(OPEN_ATTR, '');
  const width = Math.max(1, fold.colSpan);
  const gutter = width > GUTTER_COLUMNS;
  const td = doc.createElement('td');
  td.className = 'gotebanare-fold-gutter';
  td.colSpan = gutter ? GUTTER_COLUMNS : width;
  const label = t(open ? 'foldHideTitle' : 'foldShowTitle', countLines(fold.lines));
  const button = doc.createElement('button');
  button.type = 'button';
  button.className = 'gotebanare-fold-toggle';
  button.setAttribute('aria-label', label);
  button.setAttribute('aria-expanded', String(open));
  const rules = ruleLines(fold.rules);
  button.title = [label, ...rules].join('\n');
  if (rules.length > 0) button.setAttribute('aria-description', rules.join('\n'));
  button.append(createIcon(doc, open ? 'fold' : 'unfold'));
  button.addEventListener('click', onToggle);
  td.append(button);
  tr.append(td);
  if (gutter) {
    const code = doc.createElement('td');
    code.className = 'gotebanare-fold-code';
    code.colSpan = width - GUTTER_COLUMNS;
    const summary = doc.createElement('span');
    summary.className = 'gotebanare-fold-summary';
    summary.setAttribute('aria-hidden', 'true');
    summary.title = button.title;
    summary.textContent = foldSummary(fold, open);
    summary.addEventListener('click', onToggle);
    code.append(summary);
    tr.append(code);
  }
  return tr;
}
