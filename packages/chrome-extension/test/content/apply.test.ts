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

const badgeText = (c: HTMLElement) => c.querySelector('.gotebanare-badge-text')?.textContent;

describe('applyFile', () => {
  it('hides planned rows behind fold rows', () => {
    const { container, rows, plan, ctx } = setup();
    const ends = plan.folds.map((f) => `${rowKey(rows[f.first] ?? {})} ${rowKey(rows[f.last] ?? {})}`);
    expect(ends).toEqual([':12 :12', ':26 :29', '27: 31:37', '33:39 34:40']);
    expect(applyFile(container, rows, plan, ctx)).toEqual({ folds: 4, lines: 14 });
    expect(hiddenRows(container)).toEqual(plan.hidden.map((i) => rows[i]?.el));
    const folds = foldRows(container);
    expect(folds.map((f) => f.nextElementSibling)).toEqual(plan.folds.map((f) => rows[f.first]?.el));
    expect(folds.map((f) => f.hasAttribute('data-gotebanare-thin'))).toEqual([true, false, false, true]);
    expect(folds.map((f) => f.querySelector('td')?.colSpan)).toEqual([3, 3, 3, 3]);
    expect(folds[2]?.querySelector('.gotebanare-fold-text')?.textContent).toBe('gotebanare: 7 lines hidden (−2 / +2)');
    expect(folds[1]?.querySelector('.gotebanare-fold-rule')?.getAttribute('title')).toBe('Plain getters\nMatched: func (*Store) Owner');
    expect(container.querySelector('.file-info > [data-gotebanare-badge]')?.textContent).toBe('4 folds / 14 lines hiddenShow all');
  });

  it('changes nothing when applied again', () => {
    const { container, rows, plan, ctx } = setup();
    applyFile(container, rows, plan, ctx);
    const once = document.body.innerHTML;
    expect(mutations(() => applyFile(container, rows, plan, ctx))).toEqual([]);
    expect(mutations(() => applyFile(container, [...classic.rows(container)], planVisibility(rows, result), ctx))).toEqual([]);
    expect(document.body.innerHTML).toBe(once);
  });

  it('opens folds and keeps them open after GitHub re-renders the rows', () => {
    const onChange = vi.fn();
    const { container, rows, plan, ctx } = setup({ onChange });
    const tbody = container.querySelector('tbody') as HTMLElement;
    const pristine = tbody.innerHTML;
    applyFile(container, rows, plan, ctx);
    foldRows(container)[2]?.querySelector('button')?.click();
    expect(onChange).toHaveBeenLastCalledWith({ folds: 3, lines: 7 });
    expect([...ctx.expanded]).toEqual(['27:', '28:', ':33', ':34', '29:35', '30:36', '31:37']);
    foldRows(container)[0]?.querySelector('td')?.click();
    expect(onChange).toHaveBeenLastCalledWith({ folds: 2, lines: 6 });
    expect(badgeText(container)).toBe('2 folds / 6 lines hidden');

    tbody.innerHTML = pristine;
    const fresh = [...classic.rows(container)];
    expect(applyFile(container, fresh, planVisibility(fresh, result), ctx)).toEqual({ folds: 2, lines: 6 });
    expect(fresh.filter((r) => isHidden(r.el)).map(rowKey)).toEqual([':26', ':27', ':28', ':29', '33:39', '34:40']);
  });

  it('opens everything from the badge and then removes it', () => {
    const onChange = vi.fn();
    const { container, rows, plan, ctx, original } = setup({ onChange });
    applyFile(container, rows, plan, ctx);
    container.querySelector<HTMLElement>('.gotebanare-badge-show-all')?.click();
    expect(onChange).toHaveBeenCalledWith({ folds: 0, lines: 0 });
    expect(ctx.expanded.size).toBe(14);
    expect(document.body.innerHTML).toBe(original);
  });

  it('splits a fold around expanded rows', () => {
    const { container, rows, plan, ctx } = setup({ expanded: new Set(['30:36']) });
    expect(applyFile(container, rows, plan, ctx)).toEqual({ folds: 5, lines: 13 });
    const texts = foldRows(container).map((f) => f.querySelector('.gotebanare-fold-text')?.textContent ?? 'thin');
    expect(texts).toEqual(['thin', 'gotebanare: 4 lines hidden (−0 / +4)', 'gotebanare: 5 lines hidden (−2 / +2)', 'thin', 'thin']);
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
