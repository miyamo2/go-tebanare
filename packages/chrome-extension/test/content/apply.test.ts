// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { applyFile, rowKey, type ApplyContext } from '../../src/content/apply.js';
import { classicVariant as classic } from '../../src/content/dom/classic.js';
import type { RowRef } from '../../src/content/dom/variant.js';
import { planVisibility } from '../../src/content/plan.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';
import { change, foldRows, hiddenRows, hit, isHidden, load, mutations, result, rules, setup } from './apply-setup.js';

let restore = () => {};
beforeEach(() => {
  restore = installChrome(new FakeChrome({ locale: 'en' }).extensionContext());
});
afterEach(() => restore());

const badge = (c: HTMLElement) => c.querySelector<HTMLButtonElement>('.file-info > [data-gotebanare-badge]');
const toggle = (fold: HTMLElement | undefined) => fold?.querySelector<HTMLButtonElement>('button.gotebanare-fold-toggle');
const labels = (c: HTMLElement) => foldRows(c).map((f) => toggle(f)?.getAttribute('aria-label'));
const keys = (c: HTMLElement) => foldRows(c).map((f) => `${f.getAttribute('data-gotebanare-fold')}${f.hasAttribute('data-gotebanare-open') ? ' open' : ''}`);

describe('applyFile', () => {
  it('hides planned rows behind fold rows', () => {
    const { container, rows, plan, ctx } = setup();
    const ends = plan.folds.map((f) => `${rowKey(rows[f.first] ?? {})} ${rowKey(rows[f.last] ?? {})}`);
    expect(ends).toEqual([':12 :12', ':26 :29', '27: 31:37', '33:39 34:40']);
    expect(applyFile(container, rows, plan, ctx)).toEqual({ folds: 4, lines: 14 });
    expect(hiddenRows(container)).toEqual(plan.hidden.map((i) => rows[i]?.el));
    const folds = foldRows(container);
    expect(folds.map((f) => f.nextElementSibling)).toEqual(plan.folds.map((f) => rows[f.first]?.el));
    expect(folds.map((f) => [...f.querySelectorAll('td')].map((td) => td.colSpan))).toEqual([[2, 1], [2, 1], [2, 1], [2, 1]]);
    expect(folds.map((f) => f.textContent)).toEqual(['', '', '', '']);
    expect(labels(container)).toEqual([
      'Show 1 line hidden by gotebanare',
      'Show 4 lines hidden by gotebanare',
      'Show 7 lines hidden by gotebanare',
      'Show 2 lines hidden by gotebanare',
    ]);
    expect(toggle(folds[1])?.title).toBe('Show 4 lines hidden by gotebanare\ngetters: Plain getters\nMatched: func (*Store) Owner');
    expect(badge(container)?.getAttribute('aria-label')).toBe('Show the 14 lines that gotebanare hid in this file');
  });

  it('changes nothing when applied again', () => {
    const { container, rows, plan, ctx } = setup();
    applyFile(container, rows, plan, ctx);
    const once = document.body.innerHTML;
    expect(mutations(() => applyFile(container, rows, plan, ctx))).toEqual([]);
    expect(mutations(() => applyFile(container, [...classic.rows(container)], planVisibility(rows, result), ctx))).toEqual([]);
    expect(document.body.innerHTML).toBe(once);
    // Open and closed folds together, and then every fold open.
    toggle(foldRows(container)[1])?.click();
    expect(mutations(() => applyFile(container, rows, plan, ctx))).toEqual([]);
    badge(container)?.click();
    expect(mutations(() => applyFile(container, rows, plan, ctx))).toEqual([]);
  });

  it('opens folds and keeps them open after GitHub re-renders the rows', () => {
    const onChange = vi.fn();
    const { container, rows, plan, ctx } = setup({ onChange });
    const tbody = container.querySelector('tbody') as HTMLElement;
    const pristine = tbody.innerHTML;
    applyFile(container, rows, plan, ctx);
    toggle(foldRows(container)[2])?.click();
    expect(onChange).toHaveBeenLastCalledWith({ folds: 3, lines: 7 });
    expect([...ctx.expanded]).toEqual(['27:', '28:', ':33', ':34', '29:35', '30:36', '31:37']);
    toggle(foldRows(container)[0])?.click();
    expect(onChange).toHaveBeenLastCalledWith({ folds: 2, lines: 6 });
    expect(keys(container)).toEqual([':12-:12 open', ':26-:29', '27:-31:37 open', '33:39-34:40']);
    expect(labels(container)[2]).toBe('Hide 7 lines again');
    expect(badge(container)?.getAttribute('aria-label')).toBe('Show the 6 lines that gotebanare hid in this file');

    tbody.innerHTML = pristine;
    const fresh = [...classic.rows(container)];
    expect(applyFile(container, fresh, planVisibility(fresh, result), ctx)).toEqual({ folds: 2, lines: 6 });
    expect(fresh.filter((r) => isHidden(r.el)).map(rowKey)).toEqual([':26', ':27', ':28', ':29', '33:39', '34:40']);
    expect(keys(container)).toEqual([':12-:12 open', ':26-:29', '27:-31:37 open', '33:39-34:40']);
  });

  it('hides an open fold again from its row', () => {
    const onChange = vi.fn();
    const { container, rows, plan, ctx } = setup({ onChange });
    applyFile(container, rows, plan, ctx);
    const closed = document.body.innerHTML;
    toggle(foldRows(container)[2])?.click();
    toggle(foldRows(container)[2])?.click();
    expect(onChange).toHaveBeenLastCalledWith({ folds: 4, lines: 14 });
    expect(ctx.expanded.size).toBe(0);
    expect(document.body.innerHTML).toBe(closed);
  });

  it('opens every fold from the badge and hides them all again', () => {
    const onChange = vi.fn();
    const { container, rows, plan, ctx } = setup({ onChange });
    applyFile(container, rows, plan, ctx);
    const closed = document.body.innerHTML;
    toggle(foldRows(container)[2])?.click();
    expect(badge(container)?.getAttribute('aria-label')).toBe('Show the 7 lines that gotebanare hid in this file');

    badge(container)?.click();
    expect(onChange).toHaveBeenLastCalledWith({ folds: 0, lines: 0 });
    expect(ctx.expanded.size).toBe(14);
    expect(hiddenRows(container)).toEqual([]);
    expect(keys(container)).toEqual([':12-:12 open', ':26-:29 open', '27:-31:37 open', '33:39-34:40 open']);
    expect(badge(container)?.getAttribute('aria-label')).toBe('Hide the 14 lines in this file again');
    expect(badge(container)?.querySelector('svg')?.getAttribute('class')).toBe('octicon octicon-eye-closed');

    badge(container)?.click();
    expect(onChange).toHaveBeenLastCalledWith({ folds: 4, lines: 14 });
    expect(ctx.expanded.size).toBe(0);
    expect(document.body.innerHTML).toBe(closed);
  });

  it('keeps the focus on the button that the reader used', () => {
    const { container, rows, plan, ctx } = setup();
    applyFile(container, rows, plan, ctx);
    toggle(foldRows(container)[1])?.focus();
    toggle(foldRows(container)[1])?.click();
    expect(document.activeElement).toBe(toggle(foldRows(container)[1]));
    expect(document.activeElement?.getAttribute('aria-label')).toBe('Hide 4 lines again');
    badge(container)?.focus();
    badge(container)?.click();
    expect(document.activeElement).toBe(badge(container));
    expect(document.activeElement?.getAttribute('aria-label')).toBe('Hide the 14 lines in this file again');
  });

  it('splits a fold where open rows meet closed ones', () => {
    const { container, rows, plan, ctx } = setup({ expanded: new Set(['30:36']) });
    expect(applyFile(container, rows, plan, ctx)).toEqual({ folds: 5, lines: 13 });
    expect(keys(container)).toEqual([':12-:12', ':26-:29', '27:-29:35', '30:36-30:36 open', '31:37-31:37', '33:39-34:40']);
  });

  it('keeps the focus on the merged fold when a toggle joins two parts', () => {
    const { container, rows, plan, ctx } = setup({ expanded: new Set(['30:36']) });
    applyFile(container, rows, plan, ctx);
    toggle(foldRows(container)[2])?.focus();
    toggle(foldRows(container)[2])?.click();
    expect(keys(container)).toEqual([':12-:12', ':26-:29', '27:-30:36 open', '31:37-31:37', '33:39-34:40']);
    expect(document.activeElement).toBe(toggle(foldRows(container)[2]));
    toggle(foldRows(container)[3])?.focus();
    toggle(foldRows(container)[3])?.click();
    expect(keys(container)).toEqual([':12-:12', ':26-:29', '27:-31:37 open', '33:39-34:40']);
    expect(document.activeElement).toBe(toggle(foldRows(container)[2]));
  });
});

describe('fail safe', () => {
  it.each([
    ['a hunk row', (rows: RowRef[]) => rows.findIndex((r) => r.kind === 'hunk')],
    ['an expander row', (rows: RowRef[]) => rows.findIndex((r) => r.kind === 'expander')],
    ['an index past the rows', (rows: RowRef[]) => rows.length],
  ])('hides nothing when the plan marks %s', (_, pick) => {
    const { container, rows, plan, ctx, original } = setup();
    applyFile(container, rows, plan, ctx);
    const bad = { hidden: [...plan.hidden, pick(rows)], folds: plan.folds };
    expect(applyFile(container, rows, bad, ctx)).toEqual({ folds: 0, lines: 0 });
    expect(document.body.innerHTML).toBe(original);
  });

  it('hides nothing when a fold does not match the hidden rows', () => {
    const { container, rows, plan, ctx, original } = setup();
    const f = plan.folds[1];
    if (!f) throw new Error('no fold');
    for (const folds of [[{ ...f, last: f.last + 1 }], [{ ...f, first: f.last, last: f.first }], [{ ...f, first: 0.5 }], [f, f]]) {
      expect(applyFile(container, rows, { hidden: plan.hidden, folds }, ctx)).toEqual({ folds: 0, lines: 0 });
      expect(document.body.innerHTML).toBe(original);
    }
  });
});

describe('review threads', () => {
  const getters = change([], [{ start: 9, end: 17, hits: [hit('getters', 'func (*User) Name')] }]);
  const shown = (rows: RowRef[]) => rows.filter((r) => !isHidden(r.el)).map((r) => `${r.kind} ${rowKey(r)}`);

  it('keeps a thread and the line it is on visible', () => {
    const { container, rows } = load('classic-review-thread.html');
    const ctx: ApplyContext = { result: getters, rules, expanded: new Set() };
    const plan = planVisibility(rows, getters);
    expect(applyFile(container, rows, plan, ctx)).toEqual({ folds: 2, lines: 8 });
    expect(shown(rows)).toEqual(['hunk :', 'context 5:5', 'context 6:6', 'context 7:7', 'add :8', 'add :15', 'comment :']);
    expect(foldRows(container).map((f) => f.getAttribute('data-gotebanare-fold'))).toEqual([':9-:14', ':16-:17']);

    const [a, b] = plan.folds;
    if (!a || !b) throw new Error('want two folds');
    const comment = rows.findIndex((r) => r.kind === 'comment');
    const across = { hidden: [...plan.hidden, comment].sort((x, y) => x - y), folds: [{ ...a, last: b.last }] };
    expect(applyFile(container, rows, across, ctx)).toEqual({ folds: 0, lines: 0 });
    expect(hiddenRows(container)).toEqual([]);
  });

  interface ThreadCase {
    /** The line the thread comes under. */
    line: string;
    /** The rows the reader opened before the plan was applied. */
    expanded?: string[];
    /** What the reader does before the thread comes. */
    before?: (container: HTMLElement) => void;
    /** What the reader does while the thread is there. */
    act: (container: HTMLElement) => void;
  }

  // A thread comes under a line, the reader acts, and the thread goes away.
  function threadComesAndGoes({ line, expanded = [], before = () => {}, act }: ThreadCase) {
    const { container, rows, plan, ctx } = setup({ expanded: new Set(expanded) });
    const tbody = container.querySelector('tbody') as HTMLElement;
    const pristine = tbody.innerHTML;
    applyFile(container, rows, plan, ctx);
    before(container);
    const el = rows.find((r) => rowKey(r) === line)?.el;
    el?.after(Object.assign(document.createElement('tr'), { className: 'inline-comments', innerHTML: '<td colspan="3">ok</td>' }));
    const withThread = [...classic.rows(container)];
    applyFile(container, withThread, planVisibility(withThread, result), ctx);
    act(container);
    tbody.innerHTML = pristine;
    const fresh = [...classic.rows(container)];
    applyFile(container, fresh, planVisibility(fresh, result), ctx);
    return { container, ctx };
  }
  const allClosed = [':12-:12', ':26-:29', '27:-31:37', '33:39-34:40'];
  const allOpen = allClosed.map((k) => `${k} open`);
  const twice = (c: HTMLElement) => {
    badge(c)?.click();
    badge(c)?.click();
  };

  it.each<[string, ThreadCase]>([
    ['inside an open fold', { line: ':34', before: (c) => toggle(foldRows(c)[2])?.click(), act: twice }],
    ['on the first line of an open fold', { line: '27:', before: (c) => toggle(foldRows(c)[2])?.click(), act: twice }],
    ['on a one-line fold, after Show all,', { line: ':12', before: (c) => badge(c)?.click(), act: (c) => badge(c)?.click() }],
  ])('hides the line again after Hide all when a thread %s goes away', (_, threadCase) => {
    const { container, ctx } = threadComesAndGoes(threadCase);
    expect(ctx.expanded.size).toBe(0);
    expect(keys(container)).toEqual(allClosed);
  });

  it.each<[string, ThreadCase]>([
    ['inside a fold', { line: ':34', act: (c) => badge(c)?.click() }],
    ['on a one-line fold', { line: ':12', act: (c) => badge(c)?.click() }],
  ])('shows the line after Show all when a thread %s goes away', (_, threadCase) => {
    const { container } = threadComesAndGoes(threadCase);
    expect(keys(container)).toEqual(allOpen);
    expect(badge(container)?.getAttribute('aria-label')).toBe('Hide the 14 lines in this file again');
  });

  it('leaves the line of a thread alone when a run further up is toggled', () => {
    // 27:-31:37 has three runs: open, closed, and open with the thread under its last line.
    const { container } = threadComesAndGoes({
      line: '31:37',
      expanded: ['27:', '28:', '30:36', '31:37'],
      act: (c) => toggle(foldRows(c).find((f) => f.getAttribute('data-gotebanare-fold') === '27:-28:'))?.click(),
    });
    expect(keys(container)).toEqual([':12-:12', ':26-:29', '27:-29:35', '30:36-31:37 open', '33:39-34:40']);
  });

  const foldAt = (c: HTMLElement, key: string) => foldRows(c).find((f) => f.getAttribute('data-gotebanare-fold') === key);

  it.each<[string, ThreadCase, string[]]>([
    [
      'closes',
      { line: ':34', before: (c) => toggle(foldRows(c)[2])?.click(), act: (c) => toggle(foldAt(c, '27:-:33'))?.click() },
      [':12-:12', ':26-:29', '27:-:34', '29:35-31:37 open', '33:39-34:40'],
    ],
    ['opens', { line: ':34', act: (c) => toggle(foldAt(c, '27:-:33'))?.click() }, [':12-:12', ':26-:29', '27:-:34 open', '29:35-31:37', '33:39-34:40']],
  ])('the fold row right above a thread %s its line too', (_, threadCase, want) => {
    expect(keys(threadComesAndGoes(threadCase).container)).toEqual(want);
  });

  it('shows a hidden line when threads appear under it', () => {
    const { container, rows, plan, ctx } = setup();
    applyFile(container, rows, plan, ctx);
    const line = rows.find((r) => rowKey(r) === ':27')?.el;
    const thread = () => Object.assign(document.createElement('tr'), { className: 'inline-comments', innerHTML: '<td colspan="3">ok</td>' });
    line?.after(thread(), thread());
    const fresh = [...classic.rows(container)];
    expect(applyFile(container, fresh, planVisibility(fresh, result), ctx)).toEqual({ folds: 5, lines: 13 });
    expect(isHidden(line)).toBe(false);
    expect(foldRows(container).map((f) => f.getAttribute('data-gotebanare-fold'))).toContain(':28-:29');
  });
});
