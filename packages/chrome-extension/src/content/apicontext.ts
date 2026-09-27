// Looks up a pull request's commits through GitHub's REST API for
// signed-out visitors. GitHub renders their pages without the commits, and
// the API returns them for public repositories without credentials. GitHub
// allows 60 such requests an hour per IP address, so ApiContextSource
// caches answers and pauses for BACKOFF_MS after GitHub refuses one.

import type { PullRequestContext } from './context.js';
import type { FetchResult } from './fetcher.js';
import type { PullPage } from './page.js';

/** The API request ApiContextSource makes. SessionFetcher.fetchApi provides it. */
export type FetchApiFn = (url: string) => Promise<FetchResult>;

/** ApiContextSource reuses a pull request's answer, found or not, for this many milliseconds. */
export const PULL_TTL_MS = 5 * 60_000;
/** After GitHub refuses a request (403 or 429, the rate limit), ApiContextSource sends none for this many milliseconds. */
export const BACKOFF_MS = 15 * 60_000;
/** ApiContextSource caches at most this many pull requests, and as many merge bases, dropping the oldest first. */
export const MAX_CACHED = 64;

const SHA = /^[0-9a-f]{40}$/;

export interface ApiContextSourceOptions {
  /** Defaults to Date.now. */
  now?: () => number;
}

/**
 * ApiContextSource resolves a context from two requests:
 *
 *  1. GET /repos/<owner>/<repo>/pulls/<n> gives head.sha and base.sha.
 *     base.sha is the tip of the base branch that GitHub last recorded,
 *     which is not always the merge base the diff compares against.
 *  2. GET /repos/<owner>/<repo>/compare/<base>...<head>?per_page=1&page=2
 *     gives merge_base_commit.sha. The second page of the comparison
 *     carries no file list, so the answer stays small.
 *
 * The merge base lies on the base branch, as the page's baseOid does
 * (context.ts), so the pull request cannot change the config it reads.
 * The merge base of two commits stays the same, so a cached one stays
 * until MAX_CACHED newer ones push it out. A push moves the head, so a
 * pull request's answer expires after PULL_TTL_MS. resolve never rejects.
 * It returns null for a private repository (404), a refused or failed
 * request, and a malformed answer.
 */
export class ApiContextSource {
  readonly #fetch: FetchApiFn;
  readonly #now: () => number;
  readonly #pulls = new Map<string, { at: number; ctx: Promise<PullRequestContext | null> }>();
  readonly #mergeBases = new Map<string, Promise<string | null>>();
  #blockedUntil = 0;

  constructor(fetchApi: FetchApiFn, opts: ApiContextSourceOptions = {}) {
    this.#fetch = fetchApi;
    this.#now = opts.now ?? Date.now;
  }

  resolve(page: PullPage): Promise<PullRequestContext | null> {
    const key = `${page.owner}/${page.repo}#${page.number}`.toLowerCase();
    const cached = this.#pulls.get(key);
    if (cached && this.#now() - cached.at < PULL_TTL_MS) return cached.ctx;
    const ctx = this.#lookUp(page).catch(() => null);
    remember(this.#pulls, key, { at: this.#now(), ctx });
    return ctx;
  }

  async #lookUp(page: PullPage): Promise<PullRequestContext | null> {
    const repo = `https://api.github.com/repos/${encodeURIComponent(page.owner)}/${encodeURIComponent(page.repo)}`;
    const pull = await this.#get(`${repo}/pulls/${page.number}`);
    if (field(pull, 'number') !== page.number) return null;
    const base = field(pull, 'base', 'sha');
    const head = field(pull, 'head', 'sha');
    if (!isSha(base) || !isSha(head)) return null;
    const key = `${page.owner}/${page.repo} ${base}...${head}`.toLowerCase();
    let mergeBase = this.#mergeBases.get(key);
    if (!mergeBase) {
      mergeBase = this.#get(`${repo}/compare/${base}...${head}?per_page=1&page=2`).then((c) => {
        const sha = field(c, 'merge_base_commit', 'sha');
        return isSha(sha) ? sha : null;
      });
      remember(this.#mergeBases, key, mergeBase);
      // Drop a failed lookup so the next call asks again.
      void mergeBase.then((sha) => sha ?? this.#mergeBases.delete(key));
    }
    const baseSha = await mergeBase;
    if (baseSha === null) return null;
    return { owner: page.owner, repo: page.repo, number: page.number, baseSha, headSha: head };
  }

  /** get returns the parsed JSON of url, or undefined for a failed request, a refusal, or a body that does not parse. */
  async #get(url: string): Promise<unknown> {
    if (this.#now() < this.#blockedUntil) return undefined;
    let res: FetchResult;
    try {
      res = await this.#fetch(url);
    } catch {
      return undefined;
    }
    if (!res.ok) {
      if (res.status === 403 || res.status === 429) this.#blockedUntil = this.#now() + BACKOFF_MS;
      return undefined;
    }
    try {
      return JSON.parse(res.text);
    } catch {
      return undefined;
    }
  }
}

const isSha = (v: unknown): v is string => typeof v === 'string' && SHA.test(v);

/** remember sets key in map and drops the oldest entries past MAX_CACHED. */
function remember<V>(map: Map<string, V>, key: string, value: V): void {
  map.delete(key);
  map.set(key, value);
  for (const k of map.keys()) {
    if (map.size <= MAX_CACHED) break;
    map.delete(k);
  }
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
