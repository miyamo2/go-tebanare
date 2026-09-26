import { describe, expect, it } from 'vitest';
import {
  checkPullRequestKey,
  DEFAULT_TAB_STATE,
  TabStates,
  tabStateMessage,
  TOGGLE_COMMAND,
  tabKey,
  type TabsApi,
} from '../../src/background/tabs.js';
import { headPreviewApplies, pullRequestKey, type TabMessage } from '../../src/shared/messages.js';
import { FakeChrome } from '../fakes/chrome.js';

const PR = 'o/r#1';

function setup() {
  const hub = new FakeChrome();
  const tab = hub.addTab({ url: 'https://github.com/o/r/pull/1/files', active: true });
  const other = hub.addTab({ url: 'https://example.com/' });
  const sw = hub.extensionContext();
  // extensionContext() always provides tabs; the fake's type marks it optional.
  const tabs = sw.tabs as TabsApi;
  const received: TabMessage[] = [];
  hub.contentScript(tab.id).runtime.onMessage.addListener((m) => void received.push(m as TabMessage));
  const states = new TabStates(sw.storage.session, tabs);
  return { hub, tab, other, area: sw.storage.session, states, received };
}

describe('TabStates', () => {
  it('defaults to hiding on and the base config', async () => {
    const { states, tab } = setup();
    expect(await states.get(tab.id)).toEqual(DEFAULT_TAB_STATE);
    expect(DEFAULT_TAB_STATE).toEqual({ enabled: true, headPreviewFor: null });
    expect(await states.visit(tab.id, PR)).toEqual({ enabled: true, headPreview: false });
  });

  it('fills missing or malformed stored fields with defaults', async () => {
    const { states, area, tab } = setup();
    await area.set({ [tabKey(tab.id)]: { enabled: 'no', headPreview: true, headPreviewFor: 5 } });
    expect(await states.get(tab.id)).toEqual(DEFAULT_TAB_STATE);
  });

  it('turns the preview on for one pull request and tells the tab which one', async () => {
    const { states, area, tab, received } = setup();
    expect(await states.update(tab.id, { headPreview: true, pr: PR })).toEqual({ enabled: true, headPreviewFor: PR });
    expect(area.data.get(`tab:${tab.id}`)).toEqual({ enabled: true, headPreviewFor: PR });
    expect(received).toEqual([{ type: 'tab-state', enabled: true, headPreview: true, pr: PR }]);
    await states.update(tab.id, { headPreview: false });
    expect(received[1]).toEqual({ type: 'tab-state', enabled: true, headPreview: false });
    expect(await states.get(tab.id)).toEqual(DEFAULT_TAB_STATE);
  });

  it('refuses to turn the preview on without a pull request', async () => {
    const { states, area, tab, received } = setup();
    await expect(states.update(tab.id, { headPreview: true })).rejects.toThrow('headPreview: true needs pr');
    expect(area.data.size).toBe(0);
    expect(received).toEqual([]);
  });

  it('answers headPreview only to the pull request it is on for', async () => {
    const { states, tab, received } = setup();
    await states.update(tab.id, { headPreview: true, pr: PR });
    expect(await states.visit(tab.id, PR)).toEqual({ enabled: true, headPreview: true });
    expect(await states.visit(tab.id, undefined)).toEqual({ enabled: true, headPreview: false });
    expect((await states.get(tab.id)).headPreviewFor).toBe(PR);
    expect(received).toHaveLength(1);
  });

  it('ends the preview when the tab shows another pull request', async () => {
    const { states, tab } = setup();
    await states.update(tab.id, { enabled: false, headPreview: true, pr: PR });
    expect(await states.visit(tab.id, 'x/y#2')).toEqual({ enabled: false, headPreview: false });
    expect(await states.get(tab.id)).toEqual({ enabled: false, headPreviewFor: null });
    expect(await states.visit(tab.id, PR)).toEqual({ enabled: false, headPreview: false });
  });

  it('answers another pull request without the preview when storage fails', async () => {
    const { states, area, tab } = setup();
    await states.update(tab.id, { headPreview: true, pr: PR });
    const failing = new TabStates({ get: (k) => area.get(k), set: () => Promise.reject(new Error('io')), remove: (k) => area.remove(k) }, {
      sendMessage: async () => undefined,
      query: async () => [],
    });
    expect(await failing.visit(tab.id, 'x/y#2')).toEqual({ enabled: true, headPreview: false });
    expect(await failing.visit(tab.id, PR)).toEqual({ enabled: true, headPreview: true });
  });

  it('toggles enabled, keeps the preview, and serializes concurrent toggles', async () => {
    const { states, tab, received } = setup();
    await states.update(tab.id, { headPreview: true, pr: PR });
    const [a, b, c] = await Promise.all([states.toggle(tab.id), states.toggle(tab.id), states.toggle(tab.id)]);
    expect([a?.enabled, b?.enabled, c?.enabled]).toEqual([false, true, false]);
    expect(await states.get(tab.id)).toEqual({ enabled: false, headPreviewFor: PR });
    expect(received.slice(1)).toEqual([false, true, false].map((enabled) => ({ type: 'tab-state', enabled, headPreview: true, pr: PR })));
  });

  it('updates a tab without a content script', async () => {
    const { states, other } = setup();
    expect(await states.toggle(other.id)).toEqual({ enabled: false, headPreviewFor: null });
  });

  it('removes the state of a closed tab', async () => {
    const { states, area, tab } = setup();
    await states.toggle(tab.id);
    await states.remove(tab.id);
    expect(area.data.has(tabKey(tab.id))).toBe(false);
    expect(await states.get(tab.id)).toEqual(DEFAULT_TAB_STATE);
  });
});

describe('pull request keys', () => {
  it('accepts what pullRequestKey builds, in any case', () => {
    expect(checkPullRequestKey(pullRequestKey('Octo-Org/My.Repo_1', 42))).toBe('octo-org/my.repo_1#42');
    expect(checkPullRequestKey('O/R#1')).toBe(PR);
  });

  it.each([5, '', 'o/r', 'o/r#0', 'o/r#1x', 'o/r/x#1', '-o/r#1', 'o/r#1 '])('rejects %j', (v) => {
    expect(() => checkPullRequestKey(v)).toThrow('pr must be "<owner>/<repo>#<number>"');
  });

  it('builds messages that apply the preview to one pull request', () => {
    const on = tabStateMessage({ enabled: true, headPreviewFor: PR });
    expect(on).toEqual({ type: 'tab-state', enabled: true, headPreview: true, pr: PR });
    expect(headPreviewApplies(on, PR)).toBe(true);
    expect(headPreviewApplies(on, 'o/r#2')).toBe(false);
    expect(headPreviewApplies(tabStateMessage(DEFAULT_TAB_STATE), PR)).toBe(false);
  });
});

describe('runCommand', () => {
  it('toggles the tab Chrome passes with the command', async () => {
    const { states, other, received } = setup();
    expect(await states.runCommand(TOGGLE_COMMAND, { id: other.id })).toEqual({ enabled: false, headPreviewFor: null });
    expect(received).toEqual([]);
  });

  it('falls back to the active tab of the current window', async () => {
    const { states, tab, received } = setup();
    expect(await states.runCommand(TOGGLE_COMMAND)).toEqual({ enabled: false, headPreviewFor: null });
    expect(received).toEqual([{ type: 'tab-state', enabled: false, headPreview: false }]);
    expect((await states.get(tab.id)).enabled).toBe(false);
  });

  it('ignores other commands and a window without an active tab', async () => {
    const { hub, states, area } = setup();
    expect(await states.runCommand('something-else')).toBeNull();
    hub.currentWindowId = 9;
    expect(await states.runCommand(TOGGLE_COMMAND)).toBeNull();
    expect(area.data.size).toBe(0);
  });
});
