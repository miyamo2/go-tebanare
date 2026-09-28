import { describe, expect, it } from 'vitest';
import { isConfigPath, loadConfig } from '../../src/content/config.js';
import type { PullRequestContext } from '../../src/content/context.js';
import type { FetchResult, SourceFetcher } from '../../src/content/fetcher.js';

const BASE = '0123456789abcdef0123456789abcdef01234567';
const HEAD = '89abcdef0123456789abcdef0123456789abcdef';
const ctx: PullRequestContext = { owner: 'octo', repo: 'repo', number: 12, baseSha: BASE, headSha: HEAD };

/** FakeFetcher answers from a map of "<sha>:<path>" to results; other files are not-found. */
class FakeFetcher implements SourceFetcher {
  readonly calls: string[] = [];
  constructor(private readonly files: Record<string, FetchResult>) {}

  async fetchText(owner: string, repo: string, sha: string, path: string): Promise<FetchResult> {
    this.calls.push(`${owner}/${repo}@${sha}:${path}`);
    return this.files[`${sha}:${path}`] ?? { ok: false, reason: 'not-found', status: 404 };
  }
}

const text = (s: string): FetchResult => ({ ok: true, text: s });

describe('loadConfig', () => {
  it('reads .gotebanare.yml from the base commit', async () => {
    const fetcher = new FakeFetcher({ [`${BASE}:.gotebanare.yml`]: text('version: 1\n') });
    expect(await loadConfig(fetcher, ctx, 'base')).toEqual({ ok: true, path: '.gotebanare.yml', yaml: 'version: 1\n', warnings: [] });
    expect(fetcher.calls).toEqual([`octo/repo@${BASE}:.gotebanare.yml`, `octo/repo@${BASE}:.gotebanare.yaml`]);
  });

  it('falls back to .gotebanare.yaml', async () => {
    const fetcher = new FakeFetcher({ [`${BASE}:.gotebanare.yaml`]: text('version: 1\n') });
    expect(await loadConfig(fetcher, ctx, 'base')).toMatchObject({ ok: true, path: '.gotebanare.yaml', warnings: [] });
  });

  it('reads the head commit for a head preview', async () => {
    const fetcher = new FakeFetcher({ [`${BASE}:.gotebanare.yml`]: text('base'), [`${HEAD}:.gotebanare.yml`]: text('head') });
    expect(await loadConfig(fetcher, ctx, 'head')).toMatchObject({ ok: true, yaml: 'head' });
  });

  it('returns null when neither file exists', async () => {
    expect(await loadConfig(new FakeFetcher({}), ctx, 'base')).toBeNull();
  });

  it('prefers .yml and warns when both exist', async () => {
    const fetcher = new FakeFetcher({
      [`${BASE}:.gotebanare.yml`]: text('yml'),
      [`${BASE}:.gotebanare.yaml`]: text('yaml'),
    });
    expect(await loadConfig(fetcher, ctx, 'base')).toEqual({
      ok: true,
      path: '.gotebanare.yml',
      yaml: 'yml',
      warnings: [
        { severity: 'warning', code: 'config-ignored', message: '.gotebanare.yaml is ignored because .gotebanare.yml exists' },
      ],
    });
  });

  it('returns the error when .yml cannot be read, since it would win', async () => {
    const fetcher = new FakeFetcher({
      [`${BASE}:.gotebanare.yml`]: { ok: false, reason: 'sso', status: 403 },
      [`${BASE}:.gotebanare.yaml`]: text('yaml'),
    });
    expect(await loadConfig(fetcher, ctx, 'base')).toEqual({ ok: false, path: '.gotebanare.yml', reason: 'sso', status: 403 });
    const network = new FakeFetcher({ [`${BASE}:.gotebanare.yml`]: { ok: false, reason: 'network' } });
    expect(await loadConfig(network, ctx, 'base')).toEqual({ ok: false, path: '.gotebanare.yml', reason: 'network' });
  });

  it('returns the error for .yaml when .yml does not exist', async () => {
    const fetcher = new FakeFetcher({ [`${BASE}:.gotebanare.yaml`]: { ok: false, reason: 'too-large', status: 200 } });
    expect(await loadConfig(fetcher, ctx, 'base')).toEqual({ ok: false, path: '.gotebanare.yaml', reason: 'too-large', status: 200 });
  });

  it('uses .yml without a warning when .yaml cannot be read', async () => {
    const fetcher = new FakeFetcher({
      [`${BASE}:.gotebanare.yml`]: text('yml'),
      [`${BASE}:.gotebanare.yaml`]: { ok: false, reason: 'unauthorized', status: 401 },
    });
    expect(await loadConfig(fetcher, ctx, 'base')).toEqual({ ok: true, path: '.gotebanare.yml', yaml: 'yml', warnings: [] });
  });
});

describe('isConfigPath', () => {
  it.each([
    ['.gotebanare.yml', true],
    ['.gotebanare.yaml', true],
    ['sub/.gotebanare.yml', false],
    ['.gotebanare.json', false],
    ['.GOTEBANARE.YML', false],
  ])('%s => %s', (path, want) => {
    expect(isConfigPath(path)).toBe(want);
  });
});
