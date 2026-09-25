// @vitest-environment happy-dom
// applyFile in debug mode, clearFile, and isOwnNode.
import type { Hit } from '@go-tebanare/engine';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { applyFile, clearFile, isOwnNode } from '../../src/content/apply.js';
import { planVisibility } from '../../src/content/plan.js';
import { renderBanner } from '../../src/content/ui/banner.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';
import { change, foldRows, hiddenRows, hit, isHidden, load, mutations, rules, setup } from './apply-setup.js';

let restore = () => {};
beforeEach(() => {
  restore = installChrome(new FakeChrome({ locale: 'en' }).extensionContext());
});
afterEach(() => restore());

describe('debug mode', () => {
  it('outlines rows instead of hiding them', () => {
    const { container, rows, plan, ctx } = setup({ debug: true, expanded: new Set([':12']) });
    expect(applyFile(container, rows, plan, ctx)).toEqual({ folds: 0, lines: 0 });
    expect(hiddenRows(container)).toEqual([]);
    expect(container.querySelector('[data-gotebanare-fold], [data-gotebanare-badge]')).toBeNull();
    const outlined = [...container.querySelectorAll<HTMLElement>('[data-gotebanare-debug]')];
    expect(outlined).toHaveLength(14);
    expect(outlined[0]?.title).toBe('Hidden by fields: owner string');
    expect(outlined.at(-1)?.title).toBe('Hidden by put: func (*Store) Put');
  });

  it.each<[string, Hit[], string]>([
    ['no hits', [], 'Hidden'],
    ['no labels', [hit('fields', '')], 'Hidden by fields'],
    ['some labels', [hit('fields', ''), hit('fields', 'owner string')], 'Hidden by fields: owner string'],
  ])('titles a row with %s', (_, hits, title) => {
    const { container, rows } = load('classic-modified.html');
    const res = change([], [{ start: 12, end: 12, hits }]);
    applyFile(container, rows, planVisibility(rows, res), { result: res, rules, expanded: new Set(), debug: true });
    expect(container.querySelector<HTMLElement>('[data-gotebanare-debug]')?.title).toBe(title);
  });

  it('restores titles when switching modes and clearing', () => {
    const { container, rows, plan, ctx } = setup({ debug: true });
    const row = rows[plan.folds[1]?.first ?? 0]?.el as HTMLElement;
    row.title = 'own title';
    row.setAttribute('data-x', '1');
    const original = document.body.innerHTML;
    applyFile(container, rows, plan, ctx);
    expect(row.title).toBe('Hidden by getters: func (*Store) Owner');
    expect(mutations(() => applyFile(container, rows, plan, ctx))).toEqual([]);
    applyFile(container, rows, plan, { ...ctx, debug: false });
    expect(row.title).toBe('own title');
    expect(isHidden(row)).toBe(true);
    applyFile(container, rows, plan, ctx);
    clearFile(container);
    expect(document.body.innerHTML).toBe(original);
  });
});

describe('clearFile', () => {
  it.each([true, false])('restores the original markup (badge host: %s)', (withHost) => {
    const { container, rows, plan, ctx, original } = setup(withHost ? {} : { badgeHost: null });
    applyFile(container, rows, plan, ctx);
    foldRows(container)[1]?.querySelector('button')?.click();
    expect(container.querySelector('[data-gotebanare-badge]') !== null).toBe(withHost);
    clearFile(container);
    expect(document.body.innerHTML).toBe(original);
    clearFile(container);
    expect(document.body.innerHTML).toBe(original);
  });
});

describe('isOwnNode', () => {
  it('recognizes inserted elements only', () => {
    const { container, rows, plan, ctx } = setup();
    applyFile(container, rows, plan, ctx);
    const banner = renderBanner(document, [{ level: 'info', text: 'x' }], container) as HTMLElement;
    const own = [foldRows(container)[0], container.querySelector('[data-gotebanare-badge]'), banner];
    expect(own.map((n) => isOwnNode(n as Node))).toEqual([true, true, true]);
    expect(isOwnNode(rows[1]?.el as Node) || isOwnNode(document.createTextNode('x'))).toBe(false);
  });
});
