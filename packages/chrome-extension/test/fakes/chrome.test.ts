import { afterEach, describe, expect, it } from 'vitest';
import { FakeChrome, installChrome, installFakeChrome, type MessageSender } from './chrome.js';

describe('messaging', () => {
  it('delivers content script messages to extension contexts with the tab as sender', async () => {
    const hub = new FakeChrome();
    const tab = hub.addTab({ url: 'https://github.com/o/r/pull/1/files', active: true });
    const bg = hub.extensionContext();
    const cs = hub.contentScript(tab.id);
    let sender: MessageSender | undefined;
    bg.runtime.onMessage.addListener((msg, s, sendResponse) => {
      sender = s;
      sendResponse({ echo: msg });
    });
    cs.runtime.onMessage.addListener(() => {
      throw new Error('content scripts do not receive runtime.sendMessage');
    });
    expect(await cs.runtime.sendMessage({ n: 1 })).toEqual({ echo: { n: 1 } });
    expect(sender?.tab?.id).toBe(tab.id);
    expect(sender?.url).toBe(tab.url);
    expect(hub.listenerErrors).toEqual([]);
  });

  it('does not deliver a message to its sender', async () => {
    const hub = new FakeChrome();
    const bg = hub.extensionContext();
    bg.runtime.onMessage.addListener(() => true);
    await expect(bg.runtime.sendMessage({})).rejects.toThrow('Receiving end does not exist');
  });

  it('keeps the channel open only when a listener returns true', async () => {
    const hub = new FakeChrome();
    const bg = hub.extensionContext();
    bg.runtime.onMessage.addListener((msg, _s, sendResponse) => {
      if ((msg as { kind: string }).kind === 'async') {
        setTimeout(() => sendResponse('late'), 5);
        return true;
      }
      if ((msg as { kind: string }).kind === 'promise') return Promise.resolve('ignored');
      return undefined;
    });
    const popup = hub.extensionContext('popup.html');
    expect(await popup.runtime.sendMessage({ kind: 'async' })).toBe('late');
    expect(await popup.runtime.sendMessage({ kind: 'promise' })).toBeUndefined();
    expect(await popup.runtime.sendMessage({ kind: 'none' })).toBeUndefined();
  });

  it('routes tabs.sendMessage to the content scripts of one tab', async () => {
    const hub = new FakeChrome();
    const t1 = hub.addTab();
    const t2 = hub.addTab();
    const got: string[] = [];
    hub.contentScript(t1.id).runtime.onMessage.addListener((m) => void got.push(`1:${String(m)}`));
    hub.contentScript(t2.id).runtime.onMessage.addListener((m) => void got.push(`2:${String(m)}`));
    const bg = hub.extensionContext();
    await bg.tabs?.sendMessage(t2.id, 'hi');
    expect(got).toEqual(['2:hi']);
    await expect(bg.tabs?.sendMessage(99, 'hi')).rejects.toThrow('Receiving end does not exist');
  });

  it('records listener errors and flush waits for replies', async () => {
    const hub = new FakeChrome();
    const tab = hub.addTab();
    hub.contentScript(tab.id).runtime.onMessage.addListener(() => {
      throw new Error('bad listener');
    });
    void hub.extensionContext().tabs?.sendMessage(tab.id, {});
    await hub.flush();
    expect(hub.listenerErrors).toHaveLength(1);
  });
});

describe('tabs and commands', () => {
  it('query filters by active and current window', async () => {
    const hub = new FakeChrome();
    hub.addTab({ url: 'a' });
    const active = hub.addTab({ url: 'b', active: true });
    hub.addTab({ url: 'c', active: true, windowId: 2 });
    const tabs = hub.extensionContext().tabs;
    expect(await tabs?.query({ active: true, currentWindow: true })).toEqual([active]);
    expect(await tabs?.query({ windowId: 2 })).toHaveLength(1);
    await expect(tabs?.get(42)).rejects.toThrow('No tab with id: 42.');
  });

  it('removeTab fires onRemoved and drops the tab content scripts', async () => {
    const hub = new FakeChrome();
    const tab = hub.addTab();
    hub.contentScript(tab.id).runtime.onMessage.addListener(() => undefined);
    const removed: number[] = [];
    hub.extensionContext().tabs?.onRemoved.addListener((id) => void removed.push(id));
    hub.removeTab(tab.id);
    expect(removed).toEqual([tab.id]);
    await expect(hub.extensionContext().tabs?.sendMessage(tab.id, {})).rejects.toThrow();
  });

  it('runCommand passes the active tab', () => {
    const hub = new FakeChrome();
    const tab = hub.addTab({ active: true });
    const calls: unknown[] = [];
    hub.extensionContext().commands?.onCommand.addListener((c, t) => void calls.push([c, t?.id]));
    hub.runCommand('toggle-hiding');
    expect(calls).toEqual([['toggle-hiding', tab.id]]);
  });
});

describe('storage', () => {
  it('gives extension contexts the hub areas and content scripts the untrusted view', async () => {
    const hub = new FakeChrome();
    const tab = hub.addTab();
    const bg = hub.extensionContext();
    const cs = hub.contentScript(tab.id);
    expect(bg.storage.session).toBe(hub.storage.session);
    await bg.storage.session.set({ 'tab:1': { enabled: false } });
    await expect(cs.storage.session.get('tab:1')).rejects.toThrow('Access to storage is not allowed from this context.');
    await bg.storage.session.setAccessLevel({ accessLevel: 'TRUSTED_AND_UNTRUSTED_CONTEXTS' });
    expect(await cs.storage.session.get('tab:1')).toEqual({ 'tab:1': { enabled: false } });
  });

  it('stops storage events for the content scripts of a closed tab', async () => {
    const hub = new FakeChrome();
    const tab = hub.addTab();
    let events = 0;
    hub.contentScript(tab.id).storage.onChanged.addListener(() => void events++);
    await hub.storage.local.set({ a: 1 });
    hub.removeTab(tab.id);
    await hub.storage.local.set({ a: 2 });
    expect(events).toBe(1);
  });
});

describe('i18n', () => {
  it('reads the selected locale', () => {
    const hub = new FakeChrome({ locale: 'ja' });
    expect(hub.getMessage('countLines', [5])).toBe('5 行');
    expect(hub.getMessage('COUNTLINES', '5')).toBe('5 行');
    expect(hub.getMessage('@@ui_locale')).toBe('ja');
    expect(hub.getMessage('missing')).toBe('');
    hub.locale = 'en';
    expect(hub.getMessage('countLines', [5])).toBe('5 lines');
  });
});

describe('runtime and install', () => {
  let restore = () => {};
  afterEach(() => restore());

  it('installs on globalThis and restores', () => {
    const fake = installFakeChrome({ extensionId: 'abc' });
    restore = fake.restore;
    expect(chrome.runtime.getURL('/engine.wasm')).toBe('chrome-extension://abc/engine.wasm');
    expect(chrome.runtime.getManifest()).toMatchObject({ manifest_version: 3 });
    fake.restore();
    expect('chrome' in globalThis).toBe(false);
    restore = installChrome(fake.hub.contentScript(1));
    expect(chrome.i18n.getMessage('commandToggle')).toBe('Turn hiding on or off');
  });
});
