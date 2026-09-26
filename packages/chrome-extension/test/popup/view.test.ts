// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { popupElements, renderPopup, type PopupElements, type PopupModel } from '../../src/popup/view.js';
import { localizePage } from '../../src/shared/i18n.js';
import type { PageState } from '../../src/shared/messages.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';
import { loadStaticPage } from '../fakes/static-page.js';
import { pageStatus, STATES } from './status-fixture.js';

let restore = () => {};
afterEach(() => restore());

function setup(locale: string): PopupElements {
  restore();
  restore = installChrome(new FakeChrome({ locale }).extensionContext('popup.html'));
  loadStaticPage('popup.html');
  localizePage(document);
  return popupElements(document);
}

let el: PopupElements;
beforeEach(() => {
  el = setup('en');
});

function render(model: Partial<PopupModel>): void {
  renderPopup(el, { status: pageStatus(), busy: false, ...model });
}

/** visible returns the texts of the shown parts, in page order. */
function visible(): string[] {
  const parts = [el.headPreviewBanner, el.state, el.pullRequest, el.noPage, el.error, el.toggleHiding, el.toggleHeadPreview];
  return parts.filter((p) => !p.hidden).map((p) => p.textContent ?? '');
}

/** details returns the shown rows of the details list as "label: value". */
function details(): string[] {
  if (el.details.hidden) return [];
  return [el.configRow, ...el.statRows].filter((r) => !r.hidden).map((r) => `${r.querySelector('dt')?.textContent}: ${r.querySelector('dd')?.textContent}`);
}

describe('renderPopup', () => {
  it('asks for a pull request page when no content script answered', () => {
    render({ status: null });
    expect(visible()).toEqual(["Open a pull request's Files changed tab."]);
    expect(details()).toEqual([]);
    expect(el.messagesSection.hidden).toBe(true);
  });

  it('shows a ready page with its counts and buttons', () => {
    render({});
    expect(visible()).toEqual(['Active', 'octo/repo #7', 'Hiding: on', 'Preview with head config']);
    expect(details()).toEqual([
      'Config: .gotebanare.yml (base branch)',
      'Rules: 3',
      'Files analyzed: 5',
      'Files with folds: 2',
      'Lines hidden: 57',
    ]);
  });

  it.each<[PageState, string[]]>([
    ['inactive', ['Not a pull request diff', "Open a pull request's Files changed tab."]],
    ['excluded', ['Off for this repository (see options)', 'octo/repo #7']],
    ['loading', ['Loading', 'octo/repo #7', 'Hiding: on', 'Preview with head config']],
    ['no-config', ['No .gotebanare.yml or .gotebanare.yaml in this repository', 'octo/repo #7', 'Hiding: on', 'Preview with head config']],
    ['config-error', ['The config has errors', 'octo/repo #7', 'Hiding: on', 'Preview with head config']],
    ['unsupported-ui', ['Unsupported GitHub UI', 'octo/repo #7']],
    ['error', ['Error', 'octo/repo #7']],
  ])('shows the %s state', (state, want) => {
    const pr = state === 'inactive' ? { repo: undefined, pr: undefined } : {};
    render({ status: pageStatus({ state, configPath: undefined, ...pr }) });
    expect(visible()).toEqual(want);
    expect(details()).toEqual([]);
  });

  it('shows the config of a page that has one but is not ready', () => {
    render({ status: pageStatus({ state: 'config-error', configPath: '.gotebanare.yaml' }) });
    expect(details()).toEqual(['Config: .gotebanare.yaml (base branch)']);
  });

  it('keeps the head config preview in view while it is on', () => {
    render({ status: pageStatus({ headPreview: true, configSource: 'head' }) });
    expect(visible()).toEqual([
      'Previewing with the head branch config. The pull request author controls this config.',
      'Active',
      'octo/repo #7',
      'Hiding: on',
      'Stop head config preview',
    ]);
    expect(details()[0]).toBe('Config: .gotebanare.yml (head branch preview)');

    render({ status: pageStatus({ state: 'no-config', headPreview: true, configSource: 'head', configPath: undefined }) });
    expect(visible()[1]).toBe('No .gotebanare.yml or .gotebanare.yaml on the head branch');
  });

  it('labels hiding that is off', () => {
    render({ status: pageStatus({ enabled: false }) });
    expect(el.toggleHiding.textContent).toBe('Hiding: off');
    expect(el.toggleHiding.hidden).toBe(false);
  });

  it('lists the page messages and replaces them on the next render', () => {
    render({ status: pageStatus({ messages: ['one', '<b>two</b>'] }) });
    expect(el.messagesSection.hidden).toBe(false);
    expect([...el.messages.children].map((li) => li.textContent)).toEqual(['one', '<b>two</b>']);
    expect(el.messages.querySelector('b')).toBeNull();
    render({ status: pageStatus({ messages: ['three'] }) });
    expect(el.messages.children.length).toBe(1);
    render({});
    expect(el.messagesSection.hidden).toBe(true);
  });

  it('shows an action error and turns the buttons off while busy', () => {
    render({ error: 'no tab', busy: true });
    expect(el.error.textContent).toBe('Could not change the setting: no tab');
    expect(el.error.hidden).toBe(false);
    expect(el.toggleHiding.disabled).toBe(true);
    expect(el.toggleHeadPreview.disabled).toBe(true);
    render({});
    expect(el.error.hidden).toBe(true);
    expect(el.toggleHiding.disabled).toBe(false);
  });

  it('uses the Japanese messages', () => {
    el = setup('ja');
    expect(document.querySelector('h1')?.textContent).toBe('go-tebanare');
    render({ status: pageStatus({ headPreview: true, configSource: 'head' }) });
    expect(visible()).toEqual([
      'head 側の設定でプレビュー中です。この設定は PR の作成者が変更できます。',
      '有効',
      'octo/repo #7',
      '非表示: ON',
      'head 側の設定でのプレビューを終了',
    ]);
    expect(details()[0]).toBe('設定: .gotebanare.yml（head 側でプレビュー中）');
  });

  it('names a state for every PageState', () => {
    for (const state of STATES) {
      render({ status: pageStatus({ state }) });
      expect(el.state.hidden, state).toBe(false);
      expect(el.state.textContent, state).not.toMatch(/^popupState/);
    }
  });
});

describe('popupElements', () => {
  it('throws when popup.html lacks a part', () => {
    document.getElementById('toggle-hiding')?.remove();
    expect(() => popupElements(document)).toThrow('popup.html has no HTMLButtonElement #toggle-hiding');
  });
});
