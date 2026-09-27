import { describe, expect, it, vi } from 'vitest';
import { ApiContextSource, BACKOFF_MS, MAX_CACHED, PULL_TTL_MS, type FetchApiFn } from '../../src/content/apicontext.js';
import type { FetchResult } from '../../src/content/fetcher.js';
import type { PullPage } from '../../src/content/page.js';

const BASE = 'b'.repeat(40);
const HEAD = 'c'.repeat(40);
const MERGE = 'a'.repeat(40);
const page: PullPage = { owner: 'octo', repo: 'repo', number: 12 };
const PULL_URL = 'https://api.github.com/repos/octo/repo/pulls/12';
const COMPARE_URL = `https://api.github.com/repos/octo/repo/compare/${BASE}...${HEAD}?per_page=1&page=2`;

const json = (v: unknown): FetchResult => ({ ok: true, text: JSON.stringify(v) });
const pull = (over: Record<string, unknown> = {}) => json({ number: 12, base: { sha: BASE }, head: { sha: HEAD }, ...over });
const compare = (sha: unknown = MERGE) => json({ merge_base_commit: { sha } });

/** api answers pull and compare requests from the given results. */
function api(answers: { pull?: FetchResult; compare?: FetchResult } = {}) {
  let t = 0;
  const fetch = vi.fn<FetchApiFn>(async (url) => {
    if (url === PULL_URL) return answers.pull ?? pull();
    if (url === COMPARE_URL) return answers.compare ?? compare();
    return { ok: false, reason: 'not-found', status: 404 };
  });
  const source = new ApiContextSource(fetch, { now: () => t });
  return { fetch, source, advance: (ms: number) => (t += ms) };
}

describe('ApiContextSource', () => {
  it('takes the head from the pull request and the merge base from the comparison', async () => {
    const { fetch, source } = api();
    expect(await source.resolve(page)).toEqual({ ...page, baseSha: MERGE, headSha: HEAD });
    expect(fetch.mock.calls.map(([url]) => url)).toEqual([PULL_URL, COMPARE_URL]);
  });

  it('returns null for a private or missing repository', async () => {
    const { source } = api({ pull: { ok: false, reason: 'not-found', status: 404 } });
    expect(await source.resolve(page)).toBeNull();
  });

  it.each([
    ['another number', pull({ number: 13 })],
    ['a short base', pull({ base: { sha: 'abc' } })],
    ['no head', pull({ head: null })],
    ['a body that does not parse', { ok: true, text: '{' } as FetchResult],
  ])('returns null for a pull request with %s', async (_, answer) => {
    const { source } = api({ pull: answer });
    expect(await source.resolve(page)).toBeNull();
  });

  it('returns null without a valid merge base, and asks for it again next time', async () => {
    const { fetch, source, advance } = api({ compare: compare('nope') });
    expect(await source.resolve(page)).toBeNull();
    advance(PULL_TTL_MS);
    expect(await source.resolve(page)).toBeNull();
    expect(fetch.mock.calls.filter(([url]) => url === COMPARE_URL)).toHaveLength(2);
  });

  it('reuses the answer until PULL_TTL_MS passes, and the merge base after that', async () => {
    const { fetch, source, advance } = api();
    await source.resolve(page);
    advance(PULL_TTL_MS - 1);
    expect(await source.resolve({ ...page, owner: 'Octo' })).toEqual({ ...page, baseSha: MERGE, headSha: HEAD });
    expect(fetch).toHaveBeenCalledTimes(2);
    advance(1);
    await source.resolve(page);
    expect(fetch.mock.calls.map(([url]) => url)).toEqual([PULL_URL, COMPARE_URL, PULL_URL]);
  });

  it('shares one lookup between concurrent calls', async () => {
    const { fetch, source } = api();
    await Promise.all([source.resolve(page), source.resolve(page)]);
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it.each([403, 429])('stops asking for BACKOFF_MS after a %i', async (status) => {
    const { fetch, source, advance } = api({ pull: { ok: false, reason: 'unauthorized', status } });
    expect(await source.resolve(page)).toBeNull();
    advance(PULL_TTL_MS);
    expect(await source.resolve(page)).toBeNull();
    expect(fetch).toHaveBeenCalledTimes(1);
    advance(BACKOFF_MS);
    await source.resolve(page);
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it('never rejects', async () => {
    const source = new ApiContextSource(() => Promise.reject(new Error('boom')));
    expect(await source.resolve(page)).toBeNull();
  });

  it('keeps at most MAX_CACHED pull requests', async () => {
    const { fetch, source } = api();
    for (let n = 1; n <= MAX_CACHED + 1; n++) await source.resolve({ ...page, number: n });
    const calls = fetch.mock.calls.length;
    await source.resolve({ ...page, number: MAX_CACHED + 1 });
    expect(fetch).toHaveBeenCalledTimes(calls);
    await source.resolve({ ...page, number: 1 });
    expect(fetch).toHaveBeenCalledTimes(calls + 1);
  });
});
