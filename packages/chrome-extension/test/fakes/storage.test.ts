import { describe, expect, it } from 'vitest';
import { FakeEvent, FakeStorageArea, untrustedStorage, type AreaName, type ChangeListener, type StorageQuota } from './storage.js';

function area(name: AreaName, quota?: Partial<StorageQuota>) {
  const global = new FakeEvent<ChangeListener>();
  return { area: new FakeStorageArea(name, quota, global), global };
}

describe('FakeEvent', () => {
  it('adds each listener once and dispatches in order', () => {
    const e = new FakeEvent<(n: number) => number>();
    const double = (n: number) => n * 2;
    e.addListener(double);
    e.addListener(double);
    e.addListener((n) => n + 1);
    expect(e.dispatch(5)).toEqual([10, 6]);
    e.removeListener(double);
    expect(e.hasListener(double)).toBe(false);
    expect(e.hasListeners()).toBe(true);
  });
});

describe('FakeStorageArea', () => {
  it('stores copies and supports get forms', async () => {
    const { area: local } = area('local');
    const value = { list: [1, 2] };
    await local.set({ a: value, b: 2 });
    value.list.push(3);
    expect(await local.get('a')).toEqual({ a: { list: [1, 2] } });
    expect(await local.get(['a', 'missing'])).toEqual({ a: { list: [1, 2] } });
    expect(await local.get({ b: 0, c: 'default' })).toEqual({ b: 2, c: 'default' });
    expect(await local.get(null)).toEqual({ a: { list: [1, 2] }, b: 2 });
    await local.remove('a');
    expect(await local.getKeys()).toEqual(['b']);
    await local.clear();
    expect(await local.get()).toEqual({});
  });

  it('reports changes on the area and the global event', async () => {
    const { area: session, global } = area('session');
    const seen: unknown[] = [];
    global.addListener((changes, name) => seen.push([name, changes]));
    const own: unknown[] = [];
    session.onChanged.addListener((changes) => own.push(changes));
    await session.set({ k: 1 });
    await session.set({ k: 1 });
    await session.remove('k');
    expect(seen).toEqual([
      ['session', { k: { newValue: 1 } }],
      ['session', { k: { oldValue: 1 } }],
    ]);
    expect(own).toHaveLength(2);
  });

  it('rejects writes over quota and keeps the old data', async () => {
    const { area: session } = area('session', { quotaBytes: 20 });
    await session.set({ a: 'x'.repeat(10) });
    expect(await session.getBytesInUse()).toBe(1 + 12);
    await expect(session.set({ b: 'y'.repeat(10) })).rejects.toThrow(/quota/i);
    expect(await session.get(null)).toEqual({ a: 'x'.repeat(10) });
  });

  it('applies the sync per-item quota with Chrome\'s message', async () => {
    const { area: sync } = area('sync');
    await expect(sync.set({ big: 'x'.repeat(9000) })).rejects.toThrow(/^QUOTA_BYTES_PER_ITEM quota exceeded$/);
    const items = Object.fromEntries(Array.from({ length: 13 }, (_, i) => [`k${i}`, 'x'.repeat(8000)]));
    await expect(sync.set(items)).rejects.toThrow(/^QUOTA_BYTES quota exceeded$/);
  });

  it('counts UTF-8 bytes', async () => {
    const { area: sync } = area('sync');
    // 5000 two-byte characters: 5003 UTF-16 code units, 10003 UTF-8 bytes.
    await expect(sync.set({ k: '\u00e9'.repeat(5000) })).rejects.toThrow('QUOTA_BYTES_PER_ITEM');
    await sync.set({ '\u00e9': '\u00e9' });
    expect(await sync.getBytesInUse()).toBe(2 + 4);
  });

  it('fails every operation while failure is set', async () => {
    const { area: local } = area('local');
    local.failure = new Error('disk');
    await expect(local.get('a')).rejects.toThrow('disk');
    local.failure = null;
    await expect(local.get('a')).resolves.toEqual({});
  });
});

describe('untrustedStorage', () => {
  const denied = 'Access to storage is not allowed from this context.';
  const open = { accessLevel: 'TRUSTED_AND_UNTRUSTED_CONTEXTS' } as const;

  function setup() {
    const global = new FakeEvent<ChangeListener>();
    const make = (name: AreaName) => new FakeStorageArea(name, undefined, global);
    const areas = { session: make('session'), sync: make('sync'), local: make('local') };
    let active = true;
    const cs = untrustedStorage(areas, global, () => active);
    return { areas, cs, close: () => void (active = false) };
  }

  it('closes session storage until a trusted context opens it', async () => {
    const { areas, cs } = setup();
    await areas.session.set({ 'tab:1': { enabled: false } });
    await expect(cs.session.get('tab:1')).rejects.toThrow(denied);
    await expect(cs.session.set({ x: 1 })).rejects.toThrow(denied);
    await expect(cs.session.setAccessLevel(open)).rejects.toThrow('Context cannot set the storage access level');
    await areas.session.setAccessLevel(open);
    expect(await cs.session.get('tab:1')).toEqual({ 'tab:1': { enabled: false } });
    await areas.session.setAccessLevel({ accessLevel: 'TRUSTED_CONTEXTS' });
    await expect(cs.session.getKeys()).rejects.toThrow(denied);
  });

  it('opens sync and local, which do not support setAccessLevel', async () => {
    const { areas, cs } = setup();
    await cs.sync.set({ options: { debug: true } });
    expect(await areas.sync.get('options')).toEqual({ options: { debug: true } });
    await expect(areas.local.setAccessLevel({ accessLevel: 'TRUSTED_CONTEXTS' })).rejects.toThrow(/not available/);
  });

  it('reports changes only while the content script has access', async () => {
    const { areas, cs, close } = setup();
    const seen: string[] = [];
    cs.onChanged.addListener((changes, name) => void seen.push(`${name}:${Object.keys(changes).join()}`));
    cs.session.onChanged.addListener((changes) => void seen.push(`session-area:${Object.keys(changes).join()}`));
    await areas.session.set({ a: 1 });
    await areas.local.set({ b: 1 });
    await areas.session.setAccessLevel(open);
    await areas.session.set({ c: 1 });
    close();
    await areas.local.set({ d: 1 });
    await areas.session.set({ e: 1 });
    expect(seen).toEqual(['local:b', 'session-area:c', 'session:c']);
  });
});
