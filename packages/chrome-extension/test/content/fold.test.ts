// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ChangeResult, Hit } from '@go-tebanare/engine';
import { FOLD_ATTR, OPEN_ATTR, createFoldRow, foldSummary, describeFold, type FoldRow, type FoldView } from '../../src/content/ui/fold.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';
import { formatMessage, localeMessages, type LocaleEntry } from '../fakes/locale.js';

let restore = () => {};
afterEach(() => restore());

function useLocale(locale: string): void {
  restore = installChrome(new FakeChrome({ locale }).extensionContext());
}

const fold: FoldView = {
  key: '27:-31:37',
  lines: 24,
  colSpan: 3,
  rules: [
    { id: 'gomock', description: 'Generated mocks', labels: ['func (*MockUserRepo) Get', 'func (*MockUserRepo) Put'] },
    { id: 'stringer', labels: [] },
  ],
};

const button = (tr: HTMLElement) => tr.querySelector('button') as HTMLButtonElement;

describe('createFoldRow', () => {
  it('draws a closed fold as an unfold button and a one-line summary', () => {
    useLocale('en');
    const tr = createFoldRow(document, fold, false, () => {});
    expect(tr.getAttribute(FOLD_ATTR)).toBe('27:-31:37');
    expect(tr.hasAttribute(OPEN_ATTR)).toBe(false);
    const cells = [...tr.querySelectorAll('td')];
    expect(cells.map((c) => c.colSpan)).toEqual([2, 1]);
    expect(cells[0]?.className).toBe('gotebanare-fold-gutter');
    expect(cells[0]?.contains(button(tr))).toBe(true);
    expect(cells[1]?.className).toBe('gotebanare-fold-code');
    const summary = cells[1]?.querySelector('.gotebanare-fold-summary');
    expect(tr.textContent).toBe(summary?.textContent);
    expect(summary?.textContent).toBe(
      '24 lines hidden by gotebanare: gomock (func (*MockUserRepo) Get, func (*MockUserRepo) Put); stringer',
    );
    // Screen readers get the same text from the button.
    expect(summary?.getAttribute('aria-hidden')).toBe('true');
    expect(summary?.getAttribute('title')).toBe(button(tr).title);
    expect(button(tr).getAttribute('aria-label')).toBe('Show 24 lines hidden by gotebanare');
    expect(button(tr).getAttribute('aria-expanded')).toBe('false');
    expect(button(tr).querySelector('svg')?.getAttribute('class')).toBe('octicon octicon-unfold');
    expect(button(tr).querySelector('svg')?.getAttribute('aria-hidden')).toBe('true');
  });

  it('draws an open fold with a fold button that hides it again', () => {
    useLocale('en');
    const tr = createFoldRow(document, { ...fold, lines: 1 }, true, () => {});
    expect(tr.hasAttribute(OPEN_ATTR)).toBe(true);
    expect(button(tr).getAttribute('aria-label')).toBe('Hide 1 line again');
    expect(button(tr).getAttribute('aria-expanded')).toBe('true');
    expect(button(tr).querySelector('svg')?.getAttribute('class')).toBe('octicon octicon-fold');
    expect(tr.textContent).toBe('Showing 1 line hidden by gotebanare: gomock (func (*MockUserRepo) Get, func (*MockUserRepo) Put); stringer');
  });

  it('summarizes a fold without rules by its line count', () => {
    useLocale('en');
    expect(foldSummary({ ...fold, rules: [{ id: '', labels: [] }] }, false)).toBe('24 lines hidden by gotebanare');
    expect(foldSummary({ ...fold, rules: [] }, true)).toBe('Showing 24 lines hidden by gotebanare');
  });

  it('names the rules and the matched nodes in the tooltip', () => {
    useLocale('en');
    expect(button(createFoldRow(document, fold, false, () => {})).title).toBe(
      [
        'Show 24 lines hidden by gotebanare',
        'gomock: Generated mocks',
        'Matched: func (*MockUserRepo) Get, func (*MockUserRepo) Put',
        'stringer',
      ].join('\n'),
    );
    // Screen readers get the rules alone as the description, after the label.
    expect(button(createFoldRow(document, fold, false, () => {})).getAttribute('aria-description')).toBe(
      'gomock: Generated mocks\nMatched: func (*MockUserRepo) Get, func (*MockUserRepo) Put\nstringer',
    );
    const bare = createFoldRow(document, { ...fold, rules: [{ id: '', labels: [] }] }, false, () => {});
    expect(button(bare).title).toBe('Show 24 lines hidden by gotebanare');
    expect(button(bare).hasAttribute('aria-description')).toBe(false);
  });

  it('takes every text from the UI language', () => {
    useLocale('ja');
    const ja = localeMessages('ja');
    const lines = formatMessage(ja['countlines'] as LocaleEntry, ['24']);
    expect(button(createFoldRow(document, fold, false, () => {})).getAttribute('aria-label')).toBe(
      formatMessage(ja['foldshowtitle'] as LocaleEntry, [lines]),
    );
    expect(button(createFoldRow(document, fold, true, () => {})).getAttribute('aria-label')).toBe(
      formatMessage(ja['foldhidetitle'] as LocaleEntry, [lines]),
    );
    expect(foldSummary({ ...fold, rules: [] }, false)).toBe(formatMessage(ja['foldsummaryclosed'] as LocaleEntry, [lines]));
    expect(foldSummary({ ...fold, rules: [] }, true)).toBe(formatMessage(ja['foldsummaryopen'] as LocaleEntry, [lines]));
  });

  it('calls onToggle from its button and its summary', () => {
    const onToggle = vi.fn();
    const tr = createFoldRow(document, fold, false, onToggle);
    tr.querySelector('td')?.click();
    expect(onToggle).not.toHaveBeenCalled();
    button(tr).click();
    expect(onToggle).toHaveBeenCalledOnce();
    (tr.querySelector('.gotebanare-fold-summary') as HTMLElement).click();
    expect(onToggle).toHaveBeenCalledTimes(2);
    expect(button(tr).type).toBe('button');
  });

  it('spans the row, in one cell when it is narrower than three columns', () => {
    const spans = (colSpan: number) => [...createFoldRow(document, { ...fold, colSpan }, false, () => {}).querySelectorAll('td')].map((c) => c.colSpan);
    expect(spans(5)).toEqual([2, 3]);
    expect(spans(2)).toEqual([2]);
    expect(spans(0)).toEqual([1]);
    expect(createFoldRow(document, { ...fold, colSpan: 2 }, false, () => {}).textContent).toBe('');
  });
});

describe('describeFold', () => {
  const hit = (ruleId: string, label: string): Hit => ({ ruleId, target: 'func', node: 'FuncDecl', label });
  const result: ChangeResult = {
    old: [{ start: 1, end: 5, hits: [hit('mocks', 'func (*M) A'), hit('iferr', '')] }],
    new: [{ start: 1, end: 9, hits: [hit('mocks', 'func (*M) B'), hit('mocks', 'func (*M) A')] }],
    diagnostics: [],
    skipped: '',
  };
  const tr = document.createElement('tr');
  for (const span of [2, 1]) tr.append(Object.assign(document.createElement('td'), { colSpan: span }));
  const run: FoldRow[] = [
    { el: tr, kind: 'del', oldLine: 2 },
    { el: tr, kind: 'context', oldLine: 3, newLine: 3 },
    { el: tr, kind: 'add', newLine: 4 },
  ];

  it('counts rows and collects rules and labels in order', () => {
    expect(describeFold('k', run, [], result, [{ id: 'mocks', description: 'Mocks', target: 'func' }])).toEqual({
      key: 'k',
      lines: 3,
      colSpan: 3,
      rules: [
        { id: 'mocks', description: 'Mocks', labels: ['func (*M) A', 'func (*M) B'] },
        { id: 'iferr', labels: [] },
      ],
    });
  });

  it('falls back to the plan rules without hits', () => {
    const empty: ChangeResult = { ...result, old: [], new: [] };
    expect(describeFold('k', run, ['a', 'b'], empty, []).rules).toEqual([{ id: 'a', labels: [] }, { id: 'b', labels: [] }]);
    expect(describeFold('k', [], [], empty, []).colSpan).toBe(1);
  });
});
