import { describe, expect, it } from 'vitest';
import { parsePullUrl } from '../../src/content/page.js';
import { checkPatterns, isValidPattern } from '../../src/options/patterns.js';
import { isExcluded } from '../../src/shared/settings.js';

describe('isValidPattern', () => {
  it.each(['*', 'octo/repo', 'Octo/Repo', 'octo/*', 'octo-org/repo.go_x', 'a/b', `${'a'.repeat(39)}/${'r'.repeat(100)}`])('accepts %s', (p) => {
    expect(isValidPattern(p)).toBe(true);
  });

  it.each([
    '',
    '**',
    'octo',
    'octo/',
    '/repo',
    '*/repo',
    '*/*',
    'octo/repo/x',
    'octo/re*',
    '-octo/repo',
    'octo_org/repo',
    'octo /repo',
    'https://github.com/octo/repo',
    `${'a'.repeat(40)}/repo`,
    `octo/${'r'.repeat(101)}`,
  ])('rejects %j', (p) => {
    expect(isValidPattern(p)).toBe(false);
  });

  it('agrees with parsePullUrl on owner and repository names', () => {
    for (const repo of ['octo/repo', 'Octo-Org/my_repo.go', '-octo/repo', 'octo_org/repo', 'octo/re~po', `${'a'.repeat(40)}/r`]) {
      const onPage = parsePullUrl(`https://github.com/${repo}/pull/1/files`) !== null;
      expect(isValidPattern(repo), repo).toBe(onPage);
    }
  });

  it('accepts only patterns that isExcluded can match', () => {
    for (const [p, repo] of [
      ['*', 'x/y'],
      ['Octo/Repo', 'octo/repo'],
      ['octo/*', 'octo/any'],
    ] as const) {
      expect(isValidPattern(p)).toBe(true);
      expect(isExcluded(repo, [p])).toBe(true);
    }
  });
});

describe('checkPatterns', () => {
  it('trims lines, skips blank ones, and numbers lines as the textarea shows them', () => {
    expect(checkPatterns(' octo/repo \n\n  \nnot a pattern\r\nfoo/*\r\n*/x\n')).toEqual({
      patterns: ['octo/repo', 'foo/*'],
      invalid: [
        { line: 4, text: 'not a pattern' },
        { line: 6, text: '*/x' },
      ],
    });
  });

  it('returns nothing for an empty textarea', () => {
    expect(checkPatterns('')).toEqual({ patterns: [], invalid: [] });
  });
});
