// applyFile shows a visibility plan in one file container (plan 6.6, 6.7).
// It marks GitHub's rows with data-gotebanare-* attributes and inserts its
// own fold rows and badge, so clearFile can restore the original markup.
// A fold row stays after the reader opens its fold, so a row hides the fold
// again. Parts of a fold that end up all open or all closed merge into one
// fold with one row. Applying the same input twice changes nothing the
// second time, which keeps the controller's MutationObserver quiet.
//
// S2: the React UI's table gets the same fold rows. React leaves nodes it
// did not create in place when it adds or removes its own rows, and the
// controller applies the plan again after React re-renders a file. Confirm
// both on github.com.

import type { ChangeResult, RuleInfo } from '@go-tebanare/engine';
import { t } from '../shared/i18n.js';
import type { RowRef } from './dom/variant.js';
import type { Plan } from './plan.js';
import { BADGE_ATTR, createBadge } from './ui/badge.js';
import { BANNER_ATTR } from './ui/banner.js';
import { FOLD_ATTR, createFoldRow, describeFold, hitsOf, type FoldView } from './ui/fold.js';

const HIDDEN_ATTR = 'data-gotebanare-hidden';
const DEBUG_ATTR = 'data-gotebanare-debug';
// Holds a row's own title while debug mode shows ours.
const SAVED_TITLE_ATTR = 'data-gotebanare-title';

export interface ApplyContext {
  /** The analysis behind the plan. Its hits name the rules and nodes in tooltips. */
  result: ChangeResult;
  /** Rules of the compiled config, for their descriptions. */
  rules: readonly RuleInfo[];
  /** rowKey values of rows the reader opened. The fold rows and the badge add and remove them. */
  expanded: Set<string>;
  /** Outline the rows of hidden ranges instead of hiding them. */
  debug?: boolean;
  /** The element that gets the file badge (DiffUiVariant.fileHeader). No badge when absent. */
  badgeHost?: HTMLElement | null;
  /** Called after a click on a fold row or the badge re-applied the plan. */
  onChange?: (summary: ApplySummary) => void;
}

/** Counts the closed folds and the rows they hide. */
export interface ApplySummary {
  folds: number;
  lines: number;
}

interface ShownFold {
  rows: RowRef[];
  view: FoldView;
  /** The reader opened the fold, so its rows are visible. */
  open: boolean;
  /**
   * The thread anchor right below the run, if any. A comment row ends a plan
   * fold, so the anchor is the last row of the plan fold this run came from.
   * A toggle adds or removes its key too, so when the thread goes away the
   * line takes the state of the run above it.
   */
  anchor?: RowRef;
}

interface FileState {
  rows: readonly RowRef[];
  plan: Plan;
  ctx: ApplyContext;
  folds: ShownFold[];
}

const states = new WeakMap<HTMLElement, FileState>();

/** rowKey identifies a row in its file by line numbers, so it survives re-rendering. */
export function rowKey(row: Pick<RowRef, 'oldLine' | 'newLine'>): string {
  return `${row.oldLine ?? ''}:${row.newLine ?? ''}`;
}

/** isOwnNode reports nodes the extension inserted, for filtering mutation records. */
export function isOwnNode(node: Node): boolean {
  if (node.nodeType !== 1) return false;
  const el = node as Element;
  return el.hasAttribute(FOLD_ATTR) || el.hasAttribute(BADGE_ATTR) || el.hasAttribute(BANNER_ATTR);
}

const hideable = (row: RowRef | undefined): row is RowRef =>
  row !== undefined && (row.kind === 'add' || row.kind === 'del' || row.kind === 'context');

// threadAnchors returns the indices of the rows that review threads are
// attached to: the nearest del, add, or context row above each comment row.
// Plan 6.6 never hides them, and planVisibility only keeps the comment row
// itself visible.
//
// S2: a multi-line thread also covers lines above its anchor. Keeping those
// visible needs the line range from the thread markup.
function threadAnchors(rows: readonly RowRef[]): Set<number> {
  const out = new Set<number>();
  rows.forEach((row, i) => {
    if (row.kind !== 'comment') return;
    let j = i - 1;
    while (rows[j]?.kind === 'comment') j--;
    if (hideable(rows[j])) out.add(j);
  });
  return out;
}

// shownFolds splits each plan fold at thread anchors and where rows in
// expanded (open) meet rows outside it (closed). It returns null when the
// plan does not fit rows, so the caller hides nothing.
function shownFolds(rows: readonly RowRef[], plan: Plan, expanded: ReadonlySet<string>, ctx: ApplyContext): ShownFold[] | null {
  const hidden = new Set(plan.hidden);
  for (const i of hidden) if (!hideable(rows[i])) return null;
  const anchors = threadAnchors(rows);
  const out: ShownFold[] = [];
  let end = -1;
  for (const fold of plan.folds) {
    // Folds must have integer bounds, come in order, and not overlap.
    if (!Number.isInteger(fold.first) || !Number.isInteger(fold.last) || fold.first <= end || fold.first > fold.last) return null;
    end = fold.last;
    let run: RowRef[] = [];
    let open = false;
    const flush = (anchor?: RowRef) => {
      const [first, last] = [run[0], run[run.length - 1]];
      if (first && last) {
        const key = `${rowKey(first)}-${rowKey(last)}`;
        const shown: ShownFold = { rows: run, view: describeFold(key, run, fold.rules, ctx.result, ctx.rules), open };
        if (anchor) shown.anchor = anchor;
        out.push(shown);
      }
      run = [];
    };
    for (let i = fold.first; i <= fold.last; i++) {
      const row = rows[i];
      if (!hidden.has(i) || !hideable(row)) return null;
      if (anchors.has(i)) {
        flush(row);
        continue;
      }
      const rowOpen = expanded.has(rowKey(row));
      if (rowOpen !== open) flush();
      open = rowOpen;
      run.push(row);
    }
    flush();
  }
  return out;
}

function debugTitle(row: RowRef, result: ChangeResult): string {
  const hits = hitsOf(row, result);
  const rules = [...new Set(hits.map((h) => h.ruleId).filter((id) => id !== ''))];
  const labels = [...new Set(hits.map((h) => h.label).filter((l) => l !== ''))];
  if (rules.length === 0) return t('debugRowTitleNoRules');
  if (labels.length === 0) return t('debugRowTitleNoLabels', rules.join(', '));
  return t('debugRowTitle', rules.join(', '), labels.join(', '));
}

function setFlag(el: Element, name: string, on: boolean): void {
  if (on && !el.hasAttribute(name)) el.setAttribute(name, '');
  if (!on && el.hasAttribute(name)) el.removeAttribute(name);
}

// setDebug outlines el with title, or restores it when title is undefined.
// Titles are replaced in place, so the attribute order survives a restore.
function setDebug(el: Element, title: string | undefined): void {
  const active = el.hasAttribute(DEBUG_ATTR);
  if (title === undefined) {
    if (!active) return;
    el.removeAttribute(DEBUG_ATTR);
    const saved = el.getAttribute(SAVED_TITLE_ATTR);
    if (saved === null) {
      el.removeAttribute('title');
    } else {
      el.setAttribute('title', saved);
      el.removeAttribute(SAVED_TITLE_ATTR);
    }
    return;
  }
  if (!active) {
    const own = el.getAttribute('title');
    if (own !== null) el.setAttribute(SAVED_TITLE_ATTR, own);
    el.setAttribute(DEBUG_ATTR, '');
  }
  if (el.getAttribute('title') !== title) el.setAttribute('title', title);
}

function syncRows(container: HTMLElement, rows: readonly RowRef[], hide: ReadonlySet<Element>, debug: ReadonlyMap<Element, string>): void {
  const els = new Set<Element>(rows.map((r) => r.el));
  for (const el of container.querySelectorAll(`[${HIDDEN_ATTR}], [${DEBUG_ATTR}]`)) els.add(el);
  for (const el of els) {
    setFlag(el, HIDDEN_ATTR, hide.has(el));
    setDebug(el, debug.get(el));
  }
}

// syncFolds keeps an existing fold row that is identical and in place, and
// replaces the rest.
function syncFolds(container: HTMLElement, wanted: { el: HTMLElement; before: HTMLElement }[]): void {
  const pending = [...wanted];
  for (const old of container.querySelectorAll(`[${FOLD_ATTR}]`)) {
    const i = pending.findIndex((w) => old.nextElementSibling === w.before && old.outerHTML === w.el.outerHTML);
    if (i >= 0) pending.splice(i, 1);
    else old.remove();
  }
  for (const w of pending) w.before.before(w.el);
}

function syncBadge(container: HTMLElement, host: HTMLElement | null, badge: HTMLElement | null, oldHost: HTMLElement | null): void {
  const olds = new Set<Element>(container.querySelectorAll(`[${BADGE_ATTR}]`));
  for (const h of [host, oldHost]) if (h) for (const el of h.querySelectorAll(`[${BADGE_ATTR}]`)) olds.add(el);
  let kept = false;
  for (const old of olds) {
    if (!kept && badge && old.parentElement === host && old.outerHTML === badge.outerHTML) kept = true;
    else old.remove();
  }
  if (badge && host && !kept) host.append(badge);
}

// reapply opens (open is true) or closes rows of container and applies its
// plan again.
function reapply(container: HTMLElement, rows: readonly RowRef[], open: boolean): void {
  const state = states.get(container);
  if (!state) return;
  for (const key of rows.map(rowKey)) {
    if (open) state.ctx.expanded.add(key);
    else state.ctx.expanded.delete(key);
  }
  const summary = applyFile(container, state.rows, state.plan, state.ctx);
  state.ctx.onChange?.(summary);
}

// focusedButton reports whether the button of the fold row with foldKey, or
// the badge when foldKey is null, has the focus.
function focusedButton(container: HTMLElement, foldKey: string | null): boolean {
  const active = container.ownerDocument.activeElement;
  if (!active) return false;
  if (foldKey === null) return active.hasAttribute(BADGE_ATTR);
  return active.closest(`[${FOLD_ATTR}]`)?.getAttribute(FOLD_ATTR) === foldKey;
}

// refocus moves the focus to the new button of the fold row with foldKey,
// or of the badge when foldKey is null. A click replaces the row and the
// badge, and a keyboard reader would otherwise lose their place.
function refocus(container: HTMLElement, foldKey: string | null): void {
  const state = states.get(container);
  if (foldKey === null) {
    const host = state?.ctx.badgeHost ?? container;
    host.querySelector<HTMLElement>(`[${BADGE_ATTR}]`)?.focus();
    return;
  }
  for (const row of container.querySelectorAll(`[${FOLD_ATTR}]`)) {
    if (row.getAttribute(FOLD_ATTR) === foldKey) row.querySelector<HTMLElement>('button')?.focus();
  }
}

// toggleFold opens a closed fold and closes an open one. When the fold then
// merges with a neighbor, the focus goes to the row of the merged fold.
function toggleFold(container: HTMLElement, foldKey: string): void {
  const fold = states.get(container)?.folds.find((f) => f.view.key === foldKey);
  if (!fold) return;
  const focused = focusedButton(container, foldKey);
  reapply(container, fold.anchor ? [...fold.rows, fold.anchor] : fold.rows, !fold.open);
  const first = fold.rows[0];
  const merged = states.get(container)?.folds.find((f) => first !== undefined && f.rows.includes(first));
  if (focused && merged) refocus(container, merged.view.key);
}

// toggleAll opens every fold of container while any is closed, and closes
// them all once every fold is open. It acts on every row the plan hides,
// including thread anchors that no fold holds, such as a one-line fold
// with a thread.
function toggleAll(container: HTMLElement): void {
  const state = states.get(container);
  if (!state) return;
  const focused = focusedButton(container, null);
  const hidden = state.plan.hidden.map((i) => state.rows[i]).filter((r): r is RowRef => r !== undefined);
  reapply(container, hidden, state.folds.some((f) => !f.open));
  if (focused) refocus(container, null);
}

/**
 * applyFile hides the rows of plan that are not in ctx.expanded, puts a
 * fold row before each run of hidden rows and each run the reader opened,
 * and updates the file badge. Hunk, expander, and comment rows are never
 * hidden: a plan that marks one, or that does not fit rows, clears the file
 * instead. The row a review thread is attached to stays visible and splits
 * its fold. In debug mode the rows of every fold are outlined, and nothing
 * is hidden and no fold row or badge is shown.
 */
export function applyFile(container: HTMLElement, rows: readonly RowRef[], plan: Plan, ctx: ApplyContext): ApplySummary {
  const debug = ctx.debug === true;
  const folds = shownFolds(rows, plan, debug ? new Set() : ctx.expanded, ctx);
  if (!folds) {
    clearFile(container);
    return { folds: 0, lines: 0 };
  }
  const doc = container.ownerDocument;
  const hide = new Set<Element>();
  const titles = new Map<Element, string>();
  for (const fold of folds) {
    for (const row of fold.rows) {
      if (debug) titles.set(row.el, debugTitle(row, ctx.result));
      else if (!fold.open) hide.add(row.el);
    }
  }
  syncRows(container, rows, hide, titles);

  const shown = debug ? [] : folds;
  syncFolds(
    container,
    shown.map((f) => ({
      el: createFoldRow(doc, f.view, f.open, () => toggleFold(container, f.view.key)),
      before: (f.rows[0] as RowRef).el,
    })),
  );

  const closed = shown.filter((f) => !f.open);
  const summary = { folds: closed.length, lines: hide.size };
  const host = ctx.badgeHost ?? null;
  // The badge acts on the closed folds, or on every fold once none is closed.
  const acted = closed.length > 0 ? closed : shown;
  const lines = acted.reduce((n, f) => n + f.rows.length, 0);
  const badge = shown.length > 0 ? createBadge(doc, { lines, allOpen: closed.length === 0 }, () => toggleAll(container)) : null;
  syncBadge(container, host, badge, states.get(container)?.ctx.badgeHost ?? null);

  states.set(container, { rows, plan, ctx, folds: shown });
  return summary;
}

/** clearFile removes every attribute, fold row, and badge the extension added to container. */
export function clearFile(container: HTMLElement): void {
  const oldHost = states.get(container)?.ctx.badgeHost ?? null;
  states.delete(container);
  syncRows(container, [], new Set(), new Map());
  for (const el of container.querySelectorAll(`[${FOLD_ATTR}]`)) el.remove();
  syncBadge(container, null, null, oldHost);
}
