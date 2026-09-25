// In-memory fakes of chrome.events.Event and chrome.storage.StorageArea.

type Fn = (...args: never[]) => unknown;

/** FakeEvent mirrors chrome.events.Event. dispatch() calls every listener in order. */
export class FakeEvent<F extends Fn> {
  readonly listeners: F[] = [];
  addListener(f: F): void {
    if (!this.listeners.includes(f)) this.listeners.push(f);
  }
  removeListener(f: F): void {
    const i = this.listeners.indexOf(f);
    if (i >= 0) this.listeners.splice(i, 1);
  }
  hasListener(f: F): boolean {
    return this.listeners.includes(f);
  }
  hasListeners(): boolean {
    return this.listeners.length > 0;
  }
  dispatch(...args: Parameters<F>): unknown[] {
    return [...this.listeners].map((f) => f(...args));
  }
}

export type AreaName = 'session' | 'sync' | 'local';

/** chrome.storage.AccessLevel: which contexts may use a storage area. */
export type AccessLevel = 'TRUSTED_CONTEXTS' | 'TRUSTED_AND_UNTRUSTED_CONTEXTS';

export interface StorageQuota {
  /** Total bytes: UTF-8 length of every key and of every value's JSON. */
  quotaBytes: number;
  /** Bytes of one key and its value's JSON, counted the same way. */
  quotaBytesPerItem?: number;
  /** The Error message set() rejects with when quotaBytes is exceeded. */
  errorMessage: string;
  /** The Error message set() rejects with when quotaBytesPerItem is exceeded. */
  errorMessagePerItem?: string;
}

// Chrome's limits (chrome.storage.*.QUOTA_BYTES and sync QUOTA_BYTES_PER_ITEM)
// and the messages Chrome rejects with.
const defaultQuotas: Record<AreaName, StorageQuota> = {
  session: { quotaBytes: 10485760, errorMessage: 'Session storage quota bytes exceeded. Values were not stored.' },
  local: { quotaBytes: 10485760, errorMessage: 'QUOTA_BYTES quota exceeded' },
  sync: {
    quotaBytes: 102400,
    quotaBytesPerItem: 8192,
    errorMessage: 'QUOTA_BYTES quota exceeded',
    errorMessagePerItem: 'QUOTA_BYTES_PER_ITEM quota exceeded',
  },
};

export interface StorageChange { oldValue?: unknown; newValue?: unknown }
export type ChangeListener = (changes: Record<string, StorageChange>, areaName: AreaName) => void;
export type AreaChangeListener = (changes: Record<string, StorageChange>) => void;

/** The chrome.storage.StorageArea members that the fake implements. */
export interface StorageAreaApi {
  get(keys?: string | string[] | Record<string, unknown> | null): Promise<Record<string, unknown>>;
  set(items: Record<string, unknown>): Promise<void>;
  remove(keys: string | string[]): Promise<void>;
  clear(): Promise<void>;
  getKeys(): Promise<string[]>;
  getBytesInUse(keys?: string | string[] | null): Promise<number>;
  setAccessLevel(options: { accessLevel: AccessLevel }): Promise<void>;
  readonly onChanged: FakeEvent<AreaChangeListener>;
}

/** jsonClone copies a value the way Chrome serializes messages and storage values. */
export function jsonClone<T>(v: T): T {
  return v === undefined ? v : (JSON.parse(JSON.stringify(v)) as T);
}

// Chrome measures quotas in UTF-8 bytes.
function itemBytes(key: string, value: unknown): number {
  return Buffer.byteLength(key, 'utf8') + Buffer.byteLength(JSON.stringify(value), 'utf8');
}

/**
 * FakeStorageArea is one storage area as trusted contexts (the service
 * worker and extension pages) see it. untrustedStorage() gives content
 * scripts their view of the same data.
 */
export class FakeStorageArea implements StorageAreaApi {
  readonly data = new Map<string, unknown>();
  readonly onChanged = new FakeEvent<AreaChangeListener>();
  /** When set, every operation rejects with this error. */
  failure: Error | null = null;
  quota: StorageQuota;
  /** Chrome's default: session storage is closed to content scripts, sync and local are open. */
  accessLevel: AccessLevel;

  constructor(
    readonly areaName: AreaName,
    quota: Partial<StorageQuota> | undefined,
    private readonly global: FakeEvent<ChangeListener>,
  ) {
    this.quota = { ...defaultQuotas[areaName], ...quota };
    this.accessLevel = areaName === 'session' ? 'TRUSTED_CONTEXTS' : 'TRUSTED_AND_UNTRUSTED_CONTEXTS';
  }

  async get(keys?: string | string[] | Record<string, unknown> | null): Promise<Record<string, unknown>> {
    this.check();
    const out: Record<string, unknown> = {};
    if (keys === undefined || keys === null) {
      for (const [k, v] of this.data) out[k] = jsonClone(v);
      return out;
    }
    const defaults = typeof keys === 'object' && !Array.isArray(keys) ? keys : {};
    const names = typeof keys === 'string' ? [keys] : Array.isArray(keys) ? keys : Object.keys(keys);
    for (const k of names) {
      if (this.data.has(k)) out[k] = jsonClone(this.data.get(k));
      else if (k in defaults) out[k] = jsonClone(defaults[k]);
    }
    return out;
  }

  async set(items: Record<string, unknown>): Promise<void> {
    this.check();
    const next = new Map(this.data);
    for (const [k, v] of Object.entries(items)) {
      if (v === undefined) continue;
      const perItem = this.quota.quotaBytesPerItem;
      if (perItem !== undefined && itemBytes(k, v) > perItem) {
        throw new Error(this.quota.errorMessagePerItem ?? this.quota.errorMessage);
      }
      next.set(k, jsonClone(v));
    }
    if (bytes(next) > this.quota.quotaBytes) throw new Error(this.quota.errorMessage);
    const changes: Record<string, StorageChange> = {};
    for (const [k, v] of next) {
      if (!this.data.has(k) || JSON.stringify(this.data.get(k)) !== JSON.stringify(v)) {
        changes[k] = { oldValue: jsonClone(this.data.get(k)), newValue: jsonClone(v) };
      }
    }
    this.replace(next, changes);
  }

  async remove(keys: string | string[]): Promise<void> {
    this.check();
    const next = new Map(this.data);
    const changes: Record<string, StorageChange> = {};
    for (const k of typeof keys === 'string' ? [keys] : keys) {
      if (next.delete(k)) changes[k] = { oldValue: jsonClone(this.data.get(k)) };
    }
    this.replace(next, changes);
  }

  async clear(): Promise<void> {
    await this.remove([...this.data.keys()]);
  }

  async getKeys(): Promise<string[]> {
    this.check();
    return [...this.data.keys()];
  }

  async getBytesInUse(keys?: string | string[] | null): Promise<number> {
    this.check();
    if (keys === undefined || keys === null) return bytes(this.data);
    const names = new Set(typeof keys === 'string' ? [keys] : keys);
    return bytes(new Map([...this.data].filter(([k]) => names.has(k))));
  }

  /** setAccessLevel records the level. Chrome supports it on session storage only. */
  async setAccessLevel({ accessLevel }: { accessLevel: AccessLevel }): Promise<void> {
    this.check();
    if (this.areaName !== 'session') throw new Error('This StorageArea is not available for setting access level');
    this.accessLevel = accessLevel;
  }

  private check(): void {
    if (this.failure) throw this.failure;
  }

  private replace(next: Map<string, unknown>, changes: Record<string, StorageChange>): void {
    this.data.clear();
    for (const [k, v] of next) this.data.set(k, v);
    if (Object.keys(changes).length === 0) return;
    this.onChanged.dispatch(changes);
    this.global.dispatch(changes, this.areaName);
  }
}

function bytes(m: ReadonlyMap<string, unknown>): number {
  let n = 0;
  for (const [k, v] of m) n += itemBytes(k, v);
  return n;
}

const notAllowed = 'Access to storage is not allowed from this context.';

/**
 * untrustedStorage returns chrome.storage as a content script sees it.
 * While an area's access level is TRUSTED_CONTEXTS, every call on it
 * rejects and its changes are not reported to the content script. A
 * content script cannot change the level. Events stop once active()
 * returns false (the tab was closed).
 */
export function untrustedStorage(
  areas: Readonly<Record<AreaName, FakeStorageArea>>,
  global: FakeEvent<ChangeListener>,
  active: () => boolean,
): Record<AreaName, StorageAreaApi> & { onChanged: FakeEvent<ChangeListener> } {
  const open = (a: FakeStorageArea) => active() && a.accessLevel === 'TRUSTED_AND_UNTRUSTED_CONTEXTS';
  const onChanged = new FakeEvent<ChangeListener>();
  global.addListener((changes, name) => {
    if (open(areas[name])) onChanged.dispatch(changes, name);
  });
  return { session: view(areas.session), sync: view(areas.sync), local: view(areas.local), onChanged };

  function view(area: FakeStorageArea): StorageAreaApi {
    const guard =
      <A extends unknown[], R>(f: (...args: A) => Promise<R>) =>
      async (...args: A): Promise<R> => {
        if (area.accessLevel !== 'TRUSTED_AND_UNTRUSTED_CONTEXTS') throw new Error(notAllowed);
        return f.apply(area, args);
      };
    const events = new FakeEvent<AreaChangeListener>();
    area.onChanged.addListener((changes) => {
      if (open(area)) events.dispatch(changes);
    });
    return {
      get: guard(area.get),
      set: guard(area.set),
      remove: guard(area.remove),
      clear: guard(area.clear),
      getKeys: guard(area.getKeys),
      getBytesInUse: guard(area.getBytesInUse),
      setAccessLevel: async () => {
        throw new Error('Context cannot set the storage access level');
      },
      onChanged: events,
    };
  }
}
