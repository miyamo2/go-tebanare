/** User options, stored in chrome.storage.sync. */
export interface Options {
  /** Repository patterns where the extension does nothing: "owner/name", "owner/*", or "*". */
  excludedRepos: string[];
  /** Outline hidden code instead of hiding it. */
  debug: boolean;
}

export const DEFAULT_OPTIONS: { readonly excludedRepos: readonly string[]; readonly debug: boolean } = Object.freeze({
  excludedRepos: Object.freeze([]),
  debug: false,
});

/** The chrome.storage.sync key that holds Options. */
export const OPTIONS_KEY = 'options';

/** The subset of chrome.storage.StorageArea that options storage needs. */
export interface OptionsStorage {
  get(keys: string): Promise<Record<string, unknown>>;
  set(items: Record<string, unknown>): Promise<void>;
}

function syncStorage(): OptionsStorage {
  return chrome.storage.sync;
}

/** loadOptions reads Options, using the default for every missing or malformed field. */
export async function loadOptions(storage: OptionsStorage = syncStorage()): Promise<Options> {
  const items = await storage.get(OPTIONS_KEY);
  return sanitizeOptions(items[OPTIONS_KEY]);
}

/** saveOptions writes Options after dropping blank patterns. */
export async function saveOptions(opts: Options, storage: OptionsStorage = syncStorage()): Promise<void> {
  const clean: Options = { excludedRepos: cleanPatterns(opts.excludedRepos), debug: opts.debug };
  await storage.set({ [OPTIONS_KEY]: clean });
}

/** sanitizeOptions turns a stored value of unknown shape into Options. */
export function sanitizeOptions(v: unknown): Options {
  const o = typeof v === 'object' && v !== null ? (v as Record<string, unknown>) : {};
  const repos = Array.isArray(o['excludedRepos'])
    ? o['excludedRepos'].filter((p): p is string => typeof p === 'string')
    : [];
  return {
    excludedRepos: cleanPatterns(repos),
    debug: typeof o['debug'] === 'boolean' ? o['debug'] : DEFAULT_OPTIONS.debug,
  };
}

function cleanPatterns(patterns: readonly string[]): string[] {
  return patterns.map((p) => p.trim()).filter((p) => p !== '');
}

/**
 * isExcluded reports whether repo ("owner/name") matches one of the
 * patterns. A pattern is "owner/name", "owner/*", or "*"; matching ignores
 * case. Patterns of any other shape match nothing.
 */
export function isExcluded(repo: string, patterns: readonly string[]): boolean {
  const [owner, name, ...rest] = repo.toLowerCase().split('/');
  if (!owner || !name || rest.length > 0) return false;
  return patterns.some((raw) => {
    const p = raw.trim().toLowerCase();
    if (p === '*') return true;
    const [po, pn, ...prest] = p.split('/');
    if (!po || !pn || prest.length > 0 || po === '*') return false;
    return po === owner && (pn === '*' || pn === name);
  });
}
