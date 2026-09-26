import { describe, expect, it } from 'vitest';
import { isOwnerName, isRepoName, parsePullUrl } from '../../src/content/page.js';

describe('parsePullUrl', () => {
  it.each([
    'https://github.com/octo/repo/pull/12/files',
    'https://github.com/octo/repo/pull/12/files/',
    'https://github.com/octo/repo/pull/12/changes',
    'https://github.com/octo/repo/pull/12/changes/',
    'https://github.com/octo/repo/pull/12/files?diff=unified&w=1',
    'https://github.com/octo/repo/pull/12/files#diff-0123abcd',
    'https://github.com/octo/repo/pull/12/files/?w=1#r99',
    'https://github.com:443/octo/repo/pull/12/files',
  ])('accepts %s', (url) => {
    expect(parsePullUrl(url)).toEqual({ owner: 'octo', repo: 'repo', number: 12 });
  });

  it('keeps the case and punctuation of owner and repo', () => {
    expect(parsePullUrl('https://github.com/Octo-Org/my_repo.go/pull/7/files')).toEqual({
      owner: 'Octo-Org',
      repo: 'my_repo.go',
      number: 7,
    });
  });

  it.each([
    ['conversation tab', 'https://github.com/octo/repo/pull/12'],
    ['commits tab', 'https://github.com/octo/repo/pull/12/commits'],
    ['single commit view', `https://github.com/octo/repo/pull/12/commits/${'a'.repeat(40)}`],
    ['commit range view', `https://github.com/octo/repo/pull/12/files/${'a'.repeat(40)}..${'b'.repeat(40)}`],
    ['single commit files view', `https://github.com/octo/repo/pull/12/files/${'a'.repeat(40)}`],
    ['pull list', 'https://github.com/octo/repo/pulls'],
    ['compare view', 'https://github.com/octo/repo/compare/main...dev'],
    ['http', 'http://github.com/octo/repo/pull/12/files'],
    ['other host', 'https://gist.github.com/octo/repo/pull/12/files'],
    ['enterprise host', 'https://github.example.com/octo/repo/pull/12/files'],
    ['other port', 'https://github.com:8443/octo/repo/pull/12/files'],
    ['number zero', 'https://github.com/octo/repo/pull/0/files'],
    ['leading zero', 'https://github.com/octo/repo/pull/012/files'],
    ['non-numeric number', 'https://github.com/octo/repo/pull/abc/files'],
    ['huge number', 'https://github.com/octo/repo/pull/12345678901/files'],
    ['double slash', 'https://github.com/octo/repo/pull/12/files//'],
    ['bad owner', 'https://github.com/-octo/repo/pull/12/files'],
    ['encoded owner', 'https://github.com/oc%2Fto/repo/pull/12/files'],
    ['bad repo', 'https://github.com/octo/re~po/pull/12/files'],
    ['missing repo', 'https://github.com/octo/pull/12/files'],
    ['not a URL', 'github.com/octo/repo/pull/12/files'],
    ['empty', ''],
  ])('rejects the %s', (_, url) => {
    expect(parsePullUrl(url)).toBeNull();
  });
});

describe('isOwnerName and isRepoName', () => {
  it('follow the GitHub name rules', () => {
    expect(['octo', 'Octo-Org', '0x', 'a'.repeat(39)].filter(isOwnerName)).toHaveLength(4);
    expect(['', '-octo', 'octo_org', 'oc.to', 'a'.repeat(40)].filter(isOwnerName)).toEqual([]);
    expect(['repo', 'my_repo.go', '.github', '-x', 'r'.repeat(100)].filter(isRepoName)).toHaveLength(5);
    expect(['', 're~po', 're po', 'a/b', 'r'.repeat(101)].filter(isRepoName)).toEqual([]);
  });
});
