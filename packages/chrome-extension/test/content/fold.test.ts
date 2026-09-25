// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ChangeResult, Hit } from '@go-tebanare/engine';
import { FOLD_ATTR, THIN_ATTR, THIN_MAX_LINES, createFoldRow, describeFold, type FoldRow, type FoldView } from '../../src/content/ui/fold.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';
import { formatMessage, localeMessages, type LocaleEntry } from '../fakes/locale.js';

let restore = () => {};
afterEach(() => restore());

function useLocale(locale: string): void {
  restore = installChrome(new FakeChrome({ locale }).extensionContext());
}

const bar: FoldView = {
  key: '27:-31:37',
  lines: 24,
  deleted: 14,
  added: 10,
  colSpan: 3,
  rules: [
    { id: 'gomock', description: 'Generated mocks', labels: ['func (*MockUserRepo) Get', 'func (*MockUserRepo) Put'] },
    { id: 'stringer', labels: [] },
  ],
};

describe('createFoldRow', () => {
  it('renders a bar for a long fold', () => {
    useLocale('en');
    const tr = createFoldRow(document, bar, () => {});
    expect(tr.getAttribute(FOLD_ATTR)).toBe('27:-31:37');
    expect(tr.hasAttribute(THIN_ATTR)).toBe(false);
    const td = tr.querySelector('td');
    expect(td?.colSpan).toBe(3);
    expect(td?.textContent).toBe('gotebanare: 24 lines hidden (\u221214 / +10)gomock, stringerShow');
    const rules = [...tr.querySelectorAll<HTMLElement>('.gotebanare-fold-rule')];
    expect(rules.map((r) => r.title)).toEqual([
      'Generated mocks\nMatched: func (*MockUserRepo) Get, func (*MockUserRepo) Put',
      '',
    ]);
    expect(tr.querySelector('.gotebanare-fold-icon')?.getAttribute('aria-hidden')).toBe('true');
  });

  it('takes every text from the UI language', () => {
    useLocale('ja');
    const ja = localeMessages('ja');
    const tr = createFoldRow(document, bar, () => {});
    const lines = formatMessage(ja['countlines'] as LocaleEntry, ['24']);
    expect(tr.querySelector('.gotebanare-fold-text')?.textContent).toBe(formatMessage(ja['foldsummary'] as LocaleEntry, [lines, '14', '10']));
    expect(tr.querySelector('button')?.textContent).toBe(ja['foldshow']?.message);
  });

  it('leaves out the rule list when there are no rules', () => {
    const tr = createFoldRow(document, { ...bar, rules: [] }, () => {});
    expect(tr.querySelector('.gotebanare-fold-rules')).toBeNull();
  });

  it('calls onExpand from the Show button', () => {
    const onExpand = vi.fn();
    createFoldRow(document, bar, onExpand).querySelector('button')?.click();
    expect(onExpand).toHaveBeenCalledOnce();
  });

  it(`renders a thin separator up to ${THIN_MAX_LINES} lines`, () => {
    useLocale('en');
    const thin = createFoldRow(document, { ...bar, lines: THIN_MAX_LINES }, () => {});
    expect(thin.hasAttribute(THIN_ATTR)).toBe(true);
    const td = thin.querySelector('td');
    expect(td?.textContent).toBe('');
    expect(td?.title).toBe('3 lines hidden by gomock, stringer. Click to show.');
    expect(td?.getAttribute('aria-label')).toBe(td?.title);
    expect(td?.getAttribute('role')).toBe('button');
    expect(createFoldRow(document, { ...bar, lines: 1 }, () => {}).querySelector('td')?.title).toBe(
      '1 line hidden by gomock, stringer. Click to show.',
    );
    expect(createFoldRow(document, { ...bar, lines: THIN_MAX_LINES + 1 }, () => {}).hasAttribute(THIN_ATTR)).toBe(false);
  });

  it('leaves the rules out of a thin tooltip without rules', () => {
    useLocale('en');
    const rules = [[], [{ id: '', labels: [] }]];
    const titles = rules.map((r) => createFoldRow(document, { ...bar, lines: 2, rules: r }, () => {}).querySelector('td')?.title);
    expect(titles).toEqual(['2 lines hidden. Click to show.', '2 lines hidden. Click to show.']);
  });

  it('opens a thin separator by click, Enter, or Space', () => {
    const onExpand = vi.fn();
    const td = createFoldRow(document, { ...bar, lines: 2 }, onExpand).querySelector('td');
    td?.click();
    td?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));
    td?.dispatchEvent(new KeyboardEvent('keydown', { key: ' ' }));
    td?.dispatchEvent(new KeyboardEvent('keydown', { key: 'a' }));
    expect(onExpand).toHaveBeenCalledTimes(3);
  });

  it('spans at least one column', () => {
    expect(createFoldRow(document, { ...bar, colSpan: 0 }, () => {}).querySelector('td')?.colSpan).toBe(1);
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
      deleted: 1,
      added: 1,
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
