// Finds the two commits that a pull request diff compares (plan 6.3).
// GitHub does not document where its pages keep them, so each lookup is a
// separate strategy, and every strategy returns null for a missing,
// malformed, or ambiguous value. A null context means the page hides nothing.

import type { PullPage } from './page.js';

export interface PullRequestContext {
  owner: string;
  repo: string;
  number: number;
  /**
   * The old side of the diff. Config loading reads the base config at this
   * commit (plan 6.5): it is the merge base, which lies on the base branch,
   * so the pull request cannot change it.
   */
  baseSha: string;
  /** The new side of the diff. */
  headSha: string;
}

export interface PullRequestContextProvider {
  resolve(doc: Document, page: PullPage): PullRequestContext | null;
}

const SHA = /^[0-9a-f]{40}$/;

/**
 * Containers of text that users write: comments, review comments, and the
 * pull request description. GitHub renders them from Markdown, and a link or
 * image there can carry any commit pair, so the providers skip every element
 * inside them.
 */
const USER_CONTENT = '.comment-body, .markdown-body, .js-comment-body, .review-comment';

/** The JSON that GitHub's React pages embed for the app in a react-app element. */
const EMBEDDED_DATA = 'react-app > script[type="application/json"][data-target="react-app.embeddedData"]';

/**
 * embeddedDataProvider reads the JSON that the React "Files changed" page
 * embeds for its app outside user content. In
 * payload.pullRequestsChangesRoute, comparison.fullDiff holds baseOid (old
 * side) and headOid (new side), and pullRequest.comparison repeats them;
 * when both are present they must agree. The route must name page's
 * repository and pull request number, because the app keeps the JSON of
 * the page it loaded with while it moves to other pages. Several react-app
 * elements must all name the same pair.
 *
 * S3: the fields come from one saved page, trimmed into
 * context-react.html. There baseOid equals the oldCommitOid of every file
 * diff in the JSON, so it is the old side of the diff on display.
 */
export const embeddedDataProvider: PullRequestContextProvider = {
  resolve(doc, page) {
    const pairs = new Set<string>();
    for (const el of doc.querySelectorAll(EMBEDDED_DATA)) {
      if (el.closest(USER_CONTENT)) continue;
      const pair = routePair(embeddedData(el), page);
      if (pair) pairs.add(pair);
    }
    const [base = null, head = null] = single(pairs)?.split(' ') ?? [];
    return context(page, base, head);
  },
};

/**
 * hiddenInputProvider reads input[name="comparison_start_oid"] (old side)
 * and input[name="comparison_end_oid"] (new side) outside user content. The
 * page may repeat the inputs, for example once per review form; every copy
 * must agree.
 *
 * S3: synthetic fixtures model these inputs on GitHub's classic pull
 * request markup. Confirm the names, and that the start oid is the merge
 * base, on real pages.
 */
export const hiddenInputProvider: PullRequestContextProvider = {
  resolve(doc, page) {
    const values = (name: string) =>
      Array.from(doc.querySelectorAll<HTMLInputElement>(`input[name="${name}"]`))
        .filter((el) => !el.closest(USER_CONTENT))
        .map((el) => el.value);
    return context(page, single(values('comparison_start_oid')), single(values('comparison_end_oid')));
  },
};

/**
 * The elements and attributes that carry diff and expander URLs in the
 * classic markup: deferred diffs (include-fragment) and context expanders
 * (.js-expand). Links are never read, because users write their own.
 */
const URL_SOURCES: readonly (readonly [selector: string, attr: string])[] = [
  ['include-fragment', 'src'],
  ['include-fragment', 'data-fragment-url'],
  ['.js-expand', 'data-url'],
];

/**
 * diffUrlProvider reads the sha1 (old side) and sha2 (new side) query
 * parameters of the URLs in URL_SOURCES outside user content. It skips URLs
 * that point anywhere but this pull request's diff endpoints (see diffPath)
 * and URLs without exactly one valid value of each parameter; the remaining
 * URLs must all name the same pair.
 *
 * S3: synthetic fixtures model these URLs on the deferred diff and context
 * expander URLs of GitHub's classic markup. Comments and descriptions can
 * carry the same parameters in links and images, which is why the provider
 * reads only the elements above. Confirm the elements, attributes, and
 * paths on real pages.
 */
export const diffUrlProvider: PullRequestContextProvider = {
  resolve(doc, page) {
    const pairs = new Set<string>();
    for (const [selector, attr] of URL_SOURCES) {
      for (const el of doc.querySelectorAll(`${selector}[${attr}]`)) {
        if (el.closest(USER_CONTENT)) continue;
        const pair = shaPair(el.getAttribute(attr), page);
        if (pair) pairs.add(pair);
      }
    }
    const [base = null, head = null] = single(pairs)?.split(' ') ?? [];
    return context(page, base, head);
  },
};

/** firstOf returns a provider that tries each provider in order and returns the first context found. */
export function firstOf(...providers: PullRequestContextProvider[]): PullRequestContextProvider {
  return {
    resolve(doc, page) {
      for (const p of providers) {
        const ctx = p.resolve(doc, page);
        if (ctx) return ctx;
      }
      return null;
    },
  };
}

/** The strategies in the order the content script tries them. The embedded data comes first because it was checked on a real page. */
export const defaultContextProvider: PullRequestContextProvider = firstOf(embeddedDataProvider, hiddenInputProvider, diffUrlProvider);

function context(page: PullPage, base: string | null, head: string | null): PullRequestContext | null {
  if (base === null || head === null || !SHA.test(base) || !SHA.test(head)) return null;
  return { owner: page.owner, repo: page.repo, number: page.number, baseSha: base, headSha: head };
}

/** single returns the value that every entry of values holds, or null when values is empty or the entries differ. */
function single(values: Iterable<string>): string | null {
  let found: string | null = null;
  for (const v of values) {
    if (found !== null && v !== found) return null;
    found = v;
  }
  return found;
}

// GitHub treats owner and repository names without regard to case.
const sameName = (a: unknown, b: string) => typeof a === 'string' && a.toLowerCase() === b.toLowerCase();

// The parsed JSON of each embedded data element with the text it came
// from. The controller resolves the context again after every batch of DOM
// changes, and the JSON holds the whole diff, so each text is parsed once.
const parsedData = new WeakMap<Element, { text: string; data: unknown }>();

/** embeddedData returns the parsed JSON of el, or undefined when it does not parse. */
function embeddedData(el: Element): unknown {
  const text = el.textContent ?? '';
  const cached = parsedData.get(el);
  if (cached?.text === text) return cached.data;
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch {
    data = undefined;
  }
  parsedData.set(el, { text, data });
  return data;
}

/** field returns value.k1.k2..., or undefined when a step is not a plain object. */
function field(value: unknown, ...keys: string[]): unknown {
  let v = value;
  for (const k of keys) {
    if (typeof v !== 'object' || v === null || Array.isArray(v)) return undefined;
    v = (v as Record<string, unknown>)[k];
  }
  return v;
}

/**
 * routePair returns "<baseOid> <headOid>" from the "Files changed" route of
 * data when the route names page's pull request and its copies of the pair
 * agree, or null.
 */
function routePair(data: unknown, page: PullPage): string | null {
  const route = field(data, 'payload', 'pullRequestsChangesRoute');
  if (!sameName(field(route, 'repository', 'ownerLogin'), page.owner)) return null;
  if (!sameName(field(route, 'repository', 'name'), page.repo)) return null;
  if (field(route, 'pullRequest', 'number') !== page.number) return null;
  const base = field(route, 'comparison', 'fullDiff', 'baseOid');
  const head = field(route, 'comparison', 'fullDiff', 'headOid');
  if (typeof base !== 'string' || typeof head !== 'string' || !SHA.test(base) || !SHA.test(head)) return null;
  const copy = field(route, 'pullRequest', 'comparison');
  if (copy !== undefined && copy !== null && (field(copy, 'baseOid') !== base || field(copy, 'headOid') !== head)) return null;
  return `${base} ${head}`;
}

/**
 * shaPair returns "<sha1> <sha2>" for a URL of page's pull request that has
 * one valid value of each, or null. A pull_number parameter, when present,
 * must equal the page's number.
 */
function shaPair(value: string | null, page: PullPage): string | null {
  if (value === null) return null;
  let u: URL;
  try {
    u = new URL(value, 'https://github.com/');
  } catch {
    return null;
  }
  if (u.origin !== 'https://github.com' || !diffPath(u.pathname, page)) return null;
  const pull = u.searchParams.getAll('pull_number');
  if (pull.length > 1 || (pull.length === 1 && pull[0] !== String(page.number))) return null;
  const sha1 = u.searchParams.getAll('sha1');
  const sha2 = u.searchParams.getAll('sha2');
  if (sha1.length !== 1 || sha2.length !== 1) return null;
  const [a = '', b = ''] = [sha1[0], sha2[0]];
  return SHA.test(a) && SHA.test(b) ? `${a} ${b}` : null;
}

/**
 * diffPath reports whether pathname is /<owner>/<repo>/diffs...,
 * /<owner>/<repo>/blob_excerpt/<id>..., or /<owner>/<repo>/pull/<number>...
 * for page's repository and pull request number.
 */
function diffPath(pathname: string, page: PullPage): boolean {
  const [root, owner = '', repo = '', kind, next = ''] = pathname.split('/');
  if (root !== '' || !sameName(owner, page.owner) || !sameName(repo, page.repo)) return false;
  switch (kind) {
    case 'diffs':
      return true;
    case 'blob_excerpt':
      return next !== '';
    case 'pull':
      return next === String(page.number);
    default:
      return false;
  }
}
