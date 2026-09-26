import { describe, expect, it, vi } from 'vitest';
import { AnalysisCache, clearAnalyses, isQuotaError, setEvicting } from '../../src/background/cache.js';
import { analysisKey } from '../../src/shared/cachekey.js';
import type { AnalysisRecord } from '../../src/shared/messages.js';
import { FakeChrome } from '../fakes/chrome.js';
import { range, result } from './fake-engine.js';

const key = (n: number) => analysisKey({ engineVersion: 'v1', configKey: 'c', newSha: 'a'.repeat(40), newPath: `f${n}.go` });

function record(lines = 3, skipped: AnalysisRecord['result']['skipped'] = ''): AnalysisRecord {
  const newLines: Record<string, string> = {};
  for (let i = 1; i <= lines; i++) newLines[String(i)] = 'deadbeef';
  return { result: result([], [range(1, lines)], skipped), oldLines: {}, newLines };
}

function sessionArea(quotaBytes?: number) {
  return new FakeChrome(quotaBytes === undefined ? {} : { quotas: { session: { quotaBytes } } }).storage.session;
}

describe('AnalysisCache', () => {
  it('misses, then hits after put', async () => {
    const cache = new AnalysisCache(sessionArea());
    expect(await cache.get(key(1))).toBeNull();
    expect(await cache.put(key(1), record())).toBe('stored');
    expect(await cache.get(key(1))).toEqual(record());
    expect(await cache.get(key(2))).toBeNull();
  });

  it('refuses keys outside the analysis namespace', async () => {
    const area = sessionArea();
    await area.set({ 'tab:1': { enabled: false, headPreview: false } });
    const cache = new AnalysisCache(area);
    await expect(cache.get('tab:1')).rejects.toThrow('not an analysis cache key');
    await expect(cache.put('tab:1', record())).rejects.toThrow('not an analysis cache key');
    expect(area.data.get('tab:1')).toEqual({ enabled: false, headPreview: false });
  });

  it('does not store engine-error results', async () => {
    const area = sessionArea();
    const cache = new AnalysisCache(area);
    expect(await cache.put(key(1), record(0, 'engine-error'))).toBe('skipped');
    expect(area.data.size).toBe(0);
    expect(await cache.put(key(2), record(0, 'parse-error'))).toBe('stored');
  });

  it('treats malformed values and storage failures as misses', async () => {
    const area = sessionArea();
    const cache = new AnalysisCache(area);
    await area.set({ [key(1)]: { result: { old: [] }, oldLines: {}, newLines: {} }, [key(2)]: 'x' });
    expect(await cache.get(key(1))).toBeNull();
    expect(await cache.get(key(2))).toBeNull();
    area.failure = new Error('storage down');
    expect(await cache.get(key(3))).toBeNull();
  });

  it('clears analysis keys on a quota error and retries once', async () => {
    // Room for the tab entry and four and a half records (all ASCII, so length is bytes).
    const tab = { 'tab:7': { enabled: false, headPreview: true } };
    const itemBytes = key(1).length + JSON.stringify(record(4)).length;
    const area = sessionArea(JSON.stringify(tab).length + 4.5 * itemBytes);
    const cache = new AnalysisCache(area);
    await area.set(tab);
    const set = vi.spyOn(area, 'set');
    for (let n = 1; n <= 4; n++) expect(await cache.put(key(n), record(4))).toBe('stored');
    expect(set).toHaveBeenCalledTimes(4);
    set.mockClear();
    expect(await cache.put(key(5), record(4))).toBe('stored');
    expect(set).toHaveBeenCalledTimes(2);
    expect([...area.data.keys()].sort()).toEqual([key(5), 'tab:7'].sort());
  });

  it('skips caching when the record does not fit after clearing', async () => {
    const area = sessionArea(300);
    const cache = new AnalysisCache(area);
    const set = vi.spyOn(area, 'set');
    expect(await cache.put(key(1), record(50))).toBe('failed');
    expect(set).toHaveBeenCalledTimes(2);
    expect(area.data.size).toBe(0);
  });

  it('does not clear anything for errors other than quota errors', async () => {
    const area = sessionArea();
    const cache = new AnalysisCache(area);
    await cache.put(key(1), record());
    const set = vi.spyOn(area, 'set').mockRejectedValue(new Error('IO error'));
    expect(await cache.put(key(2), record())).toBe('failed');
    expect(set).toHaveBeenCalledTimes(1);
    expect(area.data.has(key(1))).toBe(true);
  });
});

describe('helpers', () => {
  it('isQuotaError matches the messages Chrome rejects with', () => {
    expect(isQuotaError(new Error('Session storage quota bytes exceeded. Values were not stored.'))).toBe(true);
    expect(isQuotaError(new Error('QUOTA_BYTES quota exceeded'))).toBe(true);
    expect(isQuotaError(new Error('IO error'))).toBe(false);
  });

  it('clearAnalyses keeps other keys', async () => {
    const area = sessionArea();
    await area.set({ [key(1)]: 1, [key(2)]: 2, 'tab:1': 3 });
    await clearAnalyses(area);
    expect([...area.data.keys()]).toEqual(['tab:1']);
  });

  it('setEvicting rejects when the retry fails too', async () => {
    const area = sessionArea(100);
    await expect(setEvicting(area, { big: 'x'.repeat(200) })).rejects.toThrow('quota');
  });
});
