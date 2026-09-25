// @vitest-environment happy-dom
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import {
  defaultContextProvider,
  diffUrlProvider,
  firstOf,
  hiddenInputProvider,
  type PullRequestContextProvider,
} from '../../src/content/context.js';
import type { PullPage } from '../../src/content/page.js';

// A path string: happy-dom replaces the global URL class, which node:fs rejects.
const fixtureDir = join(dirname(fileURLToPath(import.meta.url)), '..', 'fixtures');

function load(name: string): Document {
  return new DOMParser().parseFromString(readFileSync(join(fixtureDir, name), 'utf8'), 'text/html');
}

const page: PullPage = { owner: 'octo', repo: 'repo', number: 12 };
const BASE = '0123456789abcdef0123456789abcdef01234567';
const HEAD = '89abcdef0123456789abcdef0123456789abcdef';
const OTHER = 'fedcba9876543210fedcba9876543210fedcba98';
const want = { owner: 'octo', repo: 'repo', number: 12, baseSha: BASE, headSha: HEAD };

describe('hiddenInputProvider', () => {
  it('reads the commits from the hidden inputs', () => {
    expect(hiddenInputProvider.resolve(load('context-hidden-inputs.html'), page)).toEqual(want);
  });

  it('returns null when the inputs disagree', () => {
    expect(hiddenInputProvider.resolve(load('context-conflicting.html'), page)).toBeNull();
  });

  it('returns null without inputs', () => {
    expect(hiddenInputProvider.resolve(load('context-diff-urls.html'), page)).toBeNull();
    expect(hiddenInputProvider.resolve(load('context-none.html'), page)).toBeNull();
  });

  it.each([
    ['an abbreviated SHA', BASE.slice(0, 12)],
    ['an uppercase SHA', BASE.toUpperCase()],
    ['a SHA-256 object id', `${BASE}${'0'.repeat(24)}`],
    ['surrounding spaces', ` ${BASE}`],
    ['an empty value', ''],
  ])('returns null for %s', (_, value) => {
    const doc = load('context-hidden-inputs.html');
    for (const el of doc.querySelectorAll<HTMLInputElement>('input[name="comparison_start_oid"]')) el.value = value;
    expect(hiddenInputProvider.resolve(doc, page)).toBeNull();
  });

  it('returns null when only one side is present', () => {
    const doc = load('context-hidden-inputs.html');
    for (const el of doc.querySelectorAll('input[name="comparison_end_oid"]')) el.remove();
    expect(hiddenInputProvider.resolve(doc, page)).toBeNull();
  });
});

describe('diffUrlProvider', () => {
  it('reads sha1 and sha2 from URLs of the same repository', () => {
    expect(diffUrlProvider.resolve(load('context-diff-urls.html'), page)).toEqual(want);
  });

  it('takes owner and repo from the page, whatever the case in the URLs', () => {
    const upper: PullPage = { owner: 'OCTO', repo: 'Repo', number: 12 };
    expect(diffUrlProvider.resolve(load('context-diff-urls.html'), upper)).toEqual({ ...want, owner: 'OCTO', repo: 'Repo' });
  });

  it('returns null when the URLs name different pairs', () => {
    expect(diffUrlProvider.resolve(load('context-conflicting.html'), page)).toBeNull();
  });

  it('returns null without matching URLs', () => {
    expect(diffUrlProvider.resolve(load('context-none.html'), page)).toBeNull();
    const other: PullPage = { owner: 'someone', repo: 'else', number: 12 };
    expect(diffUrlProvider.resolve(load('context-diff-urls.html'), other)).toBeNull();
  });

  /** withFragment returns fixture with an include-fragment whose src is url appended to the body. */
  function withFragment(fixture: string, url: string): Document {
    const doc = load(fixture);
    const el = doc.createElement('include-fragment');
    el.setAttribute('src', url);
    doc.body.append(el);
    return doc;
  }

  it.each([
    ['a repeated parameter', `/octo/repo/diffs?sha1=${BASE}&sha1=${OTHER}&sha2=${HEAD}`],
    ['another pull request', `/octo/repo/pull/13/files?sha1=${OTHER}&sha2=${HEAD}`],
    ['the pull request list', `/octo/repo/pull/?sha1=${OTHER}&sha2=${HEAD}`],
    ['a compare view', `/octo/repo/compare?sha1=${OTHER}&sha2=${HEAD}`],
    ['a commit page', `/octo/repo/commit/${HEAD}?sha1=${OTHER}&sha2=${HEAD}`],
    ['a blob excerpt without an id', `/octo/repo/blob_excerpt/?sha1=${OTHER}&sha2=${HEAD}`],
    ['the pull_number of another pull request', `/octo/repo/diffs?pull_number=13&sha1=${OTHER}&sha2=${HEAD}`],
    ['a repeated pull_number', `/octo/repo/diffs?pull_number=12&pull_number=12&sha1=${OTHER}&sha2=${HEAD}`],
  ])('skips a URL with %s', (_, url) => {
    // A skipped URL adds no second pair, so the fixture's URLs still resolve.
    expect(diffUrlProvider.resolve(withFragment('context-diff-urls.html', url), page)).toEqual(want);
  });

  it.each([
    `/octo/repo/pull/12/files?sha1=${BASE}&sha2=${HEAD}`,
    `/octo/repo/pull/12?sha1=${BASE}&sha2=${HEAD}`,
    `/octo/repo/diffs?pull_number=12&sha1=${BASE}&sha2=${HEAD}`,
    `/octo/repo/diffs/3?sha1=${BASE}&sha2=${HEAD}`,
    `/octo/repo/blob_excerpt/${OTHER}?sha1=${BASE}&sha2=${HEAD}`,
  ])('accepts %s', (url) => {
    expect(diffUrlProvider.resolve(withFragment('context-none.html', url), page)).toEqual(want);
  });

  it('reads only include-fragment src and data-fragment-url and .js-expand data-url', () => {
    const url = `/octo/repo/diffs?sha1=${BASE}&sha2=${HEAD}`;
    const doc = load('context-none.html');
    doc.body.innerHTML = `<a href="${url}"></a><img src="${url}"><div data-url="${url}"></div><div data-fragment-url="${url}"></div>`;
    expect(diffUrlProvider.resolve(doc, page)).toBeNull();
    doc.body.innerHTML = `<a class="js-expand" href="#d" data-url="${url}"></a>`;
    expect(diffUrlProvider.resolve(doc, page)).toEqual(want);
    doc.body.innerHTML = `<include-fragment data-fragment-url="${url}"></include-fragment>`;
    expect(diffUrlProvider.resolve(doc, page)).toEqual(want);
  });

  it('returns null when one added URL names a different pair', () => {
    expect(diffUrlProvider.resolve(withFragment('context-diff-urls.html', `/octo/repo/diffs?sha1=${OTHER}&sha2=${HEAD}`), page)).toBeNull();
  });
});

describe('user content', () => {
  // The fixture's comment names OTHER as the old side in every form.
  it('never sets the context', () => {
    const doc = load('context-user-content.html');
    expect(hiddenInputProvider.resolve(doc, page)).toBeNull();
    expect(diffUrlProvider.resolve(doc, page)).toBeNull();
    expect(defaultContextProvider.resolve(doc, page)).toBeNull();
  });

  it('does not conflict with the values of the page', () => {
    const doc = load('context-user-content.html');
    const el = doc.createElement('include-fragment');
    el.setAttribute('src', `/octo/repo/diffs?sha1=${BASE}&sha2=${HEAD}`);
    doc.body.append(el);
    expect(diffUrlProvider.resolve(doc, page)).toEqual(want);
    const form = doc.createElement('form');
    form.innerHTML = `<input type="hidden" name="comparison_start_oid" value="${BASE}"><input type="hidden" name="comparison_end_oid" value="${HEAD}">`;
    doc.body.append(form);
    expect(hiddenInputProvider.resolve(doc, page)).toEqual(want);
  });
});

describe('defaultContextProvider', () => {
  it.each([
    ['context-hidden-inputs.html', want],
    ['context-diff-urls.html', want],
    ['context-conflicting.html', null],
    ['context-none.html', null],
  ])('%s', (fixture, expected) => {
    expect(defaultContextProvider.resolve(load(fixture), page)).toEqual(expected);
  });

  it('prefers the hidden inputs over the diff URLs', () => {
    const doc = load('context-hidden-inputs.html');
    const el = doc.createElement('include-fragment');
    el.setAttribute('src', `/octo/repo/diffs?sha1=${OTHER}&sha2=${OTHER}`);
    doc.body.append(el);
    expect(defaultContextProvider.resolve(doc, page)).toEqual(want);
  });
});

describe('firstOf', () => {
  it('returns the first non-null context and null when every provider fails', () => {
    const none: PullRequestContextProvider = { resolve: () => null };
    const fixed: PullRequestContextProvider = { resolve: (_, p) => ({ ...p, baseSha: OTHER, headSha: OTHER }) };
    const doc = load('context-none.html');
    expect(firstOf(none, fixed, hiddenInputProvider).resolve(doc, page)).toEqual({ ...page, baseSha: OTHER, headSha: OTHER });
    expect(firstOf(none, none).resolve(doc, page)).toBeNull();
    expect(firstOf().resolve(doc, page)).toBeNull();
  });
});
