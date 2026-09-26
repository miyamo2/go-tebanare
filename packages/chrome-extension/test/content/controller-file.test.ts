// @vitest-environment happy-dom
import type { ChangeResult } from '@go-tebanare/engine';
import { describe, expect, it } from 'vitest';
import {
  analyzeFile,
  cacheKeyOf,
  fileKey,
  isGoFile,
  showFile,
  type DiffFile,
  type RunConfig,
  type SendFn,
} from '../../src/content/controller/file.js';
import type { FetchResult, SourceFetcher } from '../../src/content/fetcher.js';
import { lineHash } from '../../src/shared/hash.js';
import type { AnalysisRecord, BgRequest } from '../../src/shared/messages.js';
import { load } from './apply-setup.js';

const BASE = '1405df66cbe219b0bf6355bc3d60361a8376b6b4';
const HEAD = '1a954628a960aaef81d7b2d4521929579f3541e6';

const config: RunConfig = {
  ctx: { owner: 'o', repo: 'r', number: 1, baseSha: BASE, headSha: HEAD },
  yaml: 'version: 1\n',
  configKey: 'k',
  engineVersion: 'v1',
};
const file: DiffFile = { path: 'a.go', status: 'modified' };
const empty: ChangeResult = { old: [], new: [], diagnostics: [], skipped: '' };
const record = (result: ChangeResult = empty): AnalysisRecord => ({ result, oldLines: {}, newLines: {} });

/** fakeSend answers each request type with answers[type], and records the types. */
function fakeSend(answers: Partial<Record<BgRequest['type'], unknown>>) {
  const types: string[] = [];
  const send: SendFn = async (req) => {
    types.push(req.type);
    return (answers[req.type] ?? { ok: false, error: 'no answer' }) as never;
  };
  return { send, types };
}

const fetcher = (res: FetchResult | Error): SourceFetcher => ({
  fetchText: async () => {
    if (res instanceof Error) throw res;
    return res;
  },
});
const ok = fetcher({ ok: true, text: 'package a\n' });

describe('file helpers', () => {
  it.each<[DiffFile, boolean]>([
    [{ path: 'a.go', status: 'modified' }, true],
    [{ path: 'a.go', oldPath: 'a.txt', status: 'modified' }, false],
    [{ path: 'a.go', oldPath: 'b.go', status: 'modified' }, true],
    [{ path: 'README.md', status: 'added' }, false],
    [{ path: 'gone.go', status: 'deleted' }, true],
  ])('isGoFile(%j) is %s', (f, want) => {
    expect(isGoFile(f)).toBe(want);
  });

  it('keys files by status and both paths', () => {
    expect(fileKey({ path: 'a.go', status: 'added' })).not.toBe(fileKey({ path: 'a.go', status: 'modified' }));
    expect(fileKey({ path: 'b.go', oldPath: 'a.go', status: 'modified' })).not.toBe(fileKey({ path: 'b.go', status: 'modified' }));
  });

  it('leaves the absent side out of the cache key', () => {
    expect(cacheKeyOf({ path: 'a.go', status: 'added' }, config)).toBe(`analysis:v1:v1:k:::${HEAD}:a.go`);
    expect(cacheKeyOf({ path: 'a.go', status: 'deleted' }, config)).toBe(`analysis:v1:v1:k:${BASE}:a.go::`);
  });
});

describe('analyzeFile', () => {
  it('treats a failed lookup as a miss', async () => {
    const { send, types } = fakeSend({ lookup: { ok: false, error: 'x' }, analyze: { ok: true, record: record() } });
    expect(await analyzeFile(send, ok, config, file)).toEqual({ record: record() });
    expect(types).toEqual(['lookup', 'analyze']);
  });

  it('reports a fetcher that rejects as a network failure', async () => {
    const { send } = fakeSend({ lookup: { ok: true, record: null } });
    expect(await analyzeFile(send, fetcher(new Error('boom')), config, file)).toEqual({
      record: null,
      notice: { kind: 'fetch-failed', path: 'a.go', reason: 'network' },
    });
  });

  it('turns an engine crash into a notice', async () => {
    const crashed: ChangeResult = { ...empty, skipped: 'engine-error', diagnostics: [{ severity: 'error', code: 'engine-crashed', message: 'unreachable' }] };
    const { send } = fakeSend({ lookup: { ok: true, record: record(crashed) } });
    expect(await analyzeFile(send, ok, config, file)).toEqual({
      record: null,
      notice: { kind: 'analysis-failed', path: 'a.go', error: 'unreachable' },
    });
  });

  it('hides nothing without a notice for other skip reasons', async () => {
    const parse = record({ ...empty, skipped: 'parse-error' });
    const { send } = fakeSend({ lookup: { ok: true, record: parse } });
    expect(await analyzeFile(send, ok, config, file)).toEqual({ record: parse });
  });

  it('stops between requests once stale', async () => {
    const { send, types } = fakeSend({ lookup: { ok: true, record: null } });
    let stale = false;
    const slow: SourceFetcher = {
      fetchText: async () => {
        stale = true;
        return { ok: true, text: '' };
      },
    };
    expect(await analyzeFile(send, slow, config, file, () => stale)).toEqual({ record: null });
    expect(types).toEqual(['lookup']);
    expect(await analyzeFile(send, slow, config, file, () => true)).toEqual({ record: null });
    expect(types).toEqual(['lookup']);
  });
});

describe('showFile', () => {
  it('applies only a verified plan, and only when enabled', () => {
    const { container, rows } = load('classic-modified.html');
    const result: ChangeResult = { ...empty, new: [{ start: 26, end: 29, hits: [] }] };
    const ctx = { rules: [], expanded: new Set<string>() };
    const hidden = () => container.querySelectorAll('[data-gotebanare-hidden]').length;
    // Without line hashes every row to hide mismatches.
    expect(showFile(container, rows, record(result), true, ctx)).toEqual({ verified: false });
    expect(hidden()).toBe(0);

    const newLines = Object.fromEntries(
      rows.filter((r) => r.kind === 'add' && r.newLine! >= 26 && r.newLine! <= 29).map((r) => [String(r.newLine), lineHash(r.text ?? '')]),
    );
    const verified: AnalysisRecord = { result, oldLines: {}, newLines };
    expect(showFile(container, rows, verified, true, ctx)).toEqual({ verified: true, summary: { folds: 1, lines: 4 } });
    expect(hidden()).toBe(4);
    expect(showFile(container, rows, verified, false, ctx)).toEqual({ verified: true, summary: { folds: 0, lines: 0 } });
    expect(hidden()).toBe(0);
  });
});
