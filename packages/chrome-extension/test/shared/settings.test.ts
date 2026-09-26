import { describe, expect, it } from 'vitest';
import {
  DEFAULT_OPTIONS,
  OPTIONS_KEY,
  isExcluded,
  loadOptions,
  sanitizeOptions,
  saveOptions,
} from '../../src/shared/settings.js';
import { FakeChrome, installChrome } from '../fakes/chrome.js';

describe('isExcluded', () => {
  it.each([
    ['octo/repo', ['octo/repo'], true],
    ['Octo/Repo', ['octo/REPO'], true],
    ['octo/repo', ['octo/*'], true],
    ['octo/repo', ['*'], true],
    ['octo/repo', [' octo/repo '], true],
    ['octo/repo', ['octo/other', 'other/*'], false],
    ['octo/repo', ['*/repo'], false],
    ['octo/repo', ['octo'], false],
    ['octo/repo', ['octo/repo/x'], false],
    ['octo/repo', [], false],
    ['not-a-repo', ['*'], false],
  ])('%s with %j => %s', (repo, patterns, want) => {
    expect(isExcluded(repo, patterns)).toBe(want);
  });
});

describe('sanitizeOptions', () => {
  it('falls back to defaults for malformed values', () => {
    expect(sanitizeOptions(undefined)).toEqual(DEFAULT_OPTIONS);
    expect(sanitizeOptions('x')).toEqual(DEFAULT_OPTIONS);
    expect(sanitizeOptions({ excludedRepos: 'octo/*', debug: 'yes' })).toEqual(DEFAULT_OPTIONS);
    expect(sanitizeOptions({ excludedRepos: ['a/b', 3, ' '], debug: true })).toEqual({ excludedRepos: ['a/b'], debug: true });
  });
});

describe('loadOptions and saveOptions', () => {
  it('round-trip through chrome.storage.sync', async () => {
    const hub = new FakeChrome();
    const restore = installChrome(hub.extensionContext('options.html'));
    try {
      expect(await loadOptions()).toEqual(DEFAULT_OPTIONS);
      await saveOptions({ excludedRepos: ['octo/*', '  '], debug: true });
      expect(hub.storage.sync.data.get(OPTIONS_KEY)).toEqual({ excludedRepos: ['octo/*'], debug: true });
      expect(await loadOptions()).toEqual({ excludedRepos: ['octo/*'], debug: true });
    } finally {
      restore();
    }
  });

  it('reject with the per-item quota error when the options exceed 8 KiB', async () => {
    const hub = new FakeChrome();
    const excludedRepos = Array.from({ length: 800 }, (_, i) => `owner${i}/repo${i}`);
    await expect(saveOptions({ excludedRepos, debug: false }, hub.storage.sync)).rejects.toThrow(
      'QUOTA_BYTES_PER_ITEM quota exceeded',
    );
    expect(hub.storage.sync.data.size).toBe(0);
  });

  it('accept an injected storage area', async () => {
    const hub = new FakeChrome();
    await saveOptions({ excludedRepos: [], debug: true }, hub.storage.local);
    expect(await loadOptions(hub.storage.local)).toEqual({ excludedRepos: [], debug: true });
    expect(hub.storage.sync.data.size).toBe(0);
  });
});
