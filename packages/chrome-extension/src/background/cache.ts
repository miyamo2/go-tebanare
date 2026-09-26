// Caches AnalysisRecords in chrome.storage.session (plan 6.4). Chrome keeps
// that area in memory until the browser closes, and it outlives service
// worker restarts. Records hold ranges and line hashes, never a full
// source; hit labels and diagnostics in them can quote up to 60 runes of
// normalized hidden code (see analysis.ts).

import { ANALYSIS_KEY_PREFIX } from '../shared/cachekey.js';
import { errorText, type AnalysisRecord } from '../shared/messages.js';

/** The chrome.storage.StorageArea calls the background makes. */
export interface SessionArea {
  get(keys: string | string[] | null): Promise<Record<string, unknown>>;
  set(items: Record<string, unknown>): Promise<void>;
  remove(keys: string | string[]): Promise<void>;
}

/**
 * isQuotaError reports whether e is Chrome's rejection for a full storage
 * area. Chrome rejects with an Error whose message names the quota, such
 * as "Session storage quota bytes exceeded. Values were not stored."
 */
export function isQuotaError(e: unknown): boolean {
  return /quota/i.test(errorText(e));
}

/** clearAnalyses removes every key that starts with ANALYSIS_KEY_PREFIX. */
export async function clearAnalyses(area: SessionArea): Promise<void> {
  // get(null) lists the keys; StorageArea.getKeys() needs Chrome 130.
  const keys = Object.keys(await area.get(null)).filter((k) => k.startsWith(ANALYSIS_KEY_PREFIX));
  if (keys.length > 0) await area.remove(keys);
}

/**
 * setEvicting writes items. When the area is full it clears every
 * analysis record and tries once more. Other errors, and a second
 * failure, reject.
 */
export async function setEvicting(area: SessionArea, items: Record<string, unknown>): Promise<void> {
  try {
    await area.set(items);
    return;
  } catch (e) {
    if (!isQuotaError(e)) throw e;
  }
  await clearAnalyses(area);
  await area.set(items);
}

/** assertAnalysisKey rejects keys outside the analysis namespace, such as "tab:1". */
export function assertAnalysisKey(key: string): void {
  if (!key.startsWith(ANALYSIS_KEY_PREFIX)) throw new TypeError(`not an analysis cache key: ${key}`);
}

/** stored: written. skipped: not cacheable. failed: storage rejected it. */
export type PutOutcome = 'stored' | 'skipped' | 'failed';

export class AnalysisCache {
  constructor(private readonly area: SessionArea) {}

  /**
   * get returns the record under key, or null when there is none, when the
   * stored value is malformed, or when storage fails: a cache failure
   * leads to a fresh analysis.
   */
  async get(key: string): Promise<AnalysisRecord | null> {
    assertAnalysisKey(key);
    let v: unknown;
    try {
      v = (await this.area.get(key))[key];
    } catch {
      return null;
    }
    return isRecord(v) ? v : null;
  }

  /**
   * put stores record under key and never rejects for storage errors. A
   * result skipped with "engine-error" means the wasm instance crashed; the
   * next analysis of the same change can succeed, so put does not store it.
   */
  async put(key: string, record: AnalysisRecord): Promise<PutOutcome> {
    assertAnalysisKey(key);
    if (record.result.skipped === 'engine-error') return 'skipped';
    try {
      await setEvicting(this.area, { [key]: record });
      return 'stored';
    } catch {
      return 'failed';
    }
  }
}

function isObject(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

// Checks the fields a reader of AnalysisRecord touches first. Values come
// from this module, so a deeper check would only catch a corrupt store.
function isRecord(v: unknown): v is AnalysisRecord {
  if (!isObject(v) || !isObject(v['oldLines']) || !isObject(v['newLines'])) return false;
  const r = v['result'];
  return isObject(r) && Array.isArray(r['old']) && Array.isArray(r['new']) && typeof r['skipped'] === 'string';
}
