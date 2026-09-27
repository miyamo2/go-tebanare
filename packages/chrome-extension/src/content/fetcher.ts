// Fetches repository files through the signed-in GitHub session (plan 6.4).
// The content script runs on github.com, so a same-origin request carries
// the session cookies and reads private repositories without a token.

export type FetchFailureReason = 'not-found' | 'unauthorized' | 'sso' | 'network' | 'too-large';

export type FetchResult = { ok: true; text: string } | { ok: false; reason: FetchFailureReason; status?: number };

export interface SourceFetcher {
  fetchText(owner: string, repo: string, sha: string, path: string): Promise<FetchResult>;
}

/** The part of the fetch API that SessionFetcher calls. */
export type FetchFn = (url: string, init: RequestInit) => Promise<Response>;

/** Files larger than this many bytes are too-large. */
export const MAX_SOURCE_BYTES = 1 << 20;
/**
 * Pages larger than this many bytes are too-large. A "Files changed" page
 * embeds the whole diff, so it gets more room than a source file.
 */
export const MAX_PAGE_BYTES = 32 << 20;
/** SessionFetcher keeps at most this many requests in flight. */
export const MAX_IN_FLIGHT = 4;
/** SessionFetcher gives up on a request, body included, after this many milliseconds. */
export const REQUEST_TIMEOUT_MS = 30_000;

export interface SessionFetcherOptions {
  /** Defaults to the global fetch. */
  fetch?: FetchFn;
  /** Defaults to MAX_IN_FLIGHT; a value below 1 or not finite counts as 1. */
  maxInFlight?: number;
  /** Defaults to MAX_SOURCE_BYTES, which also replaces a negative value or NaN. */
  maxBytes?: number;
  /** Defaults to REQUEST_TIMEOUT_MS, which also replaces a value outside 1 to 2^31 - 1. */
  timeoutMs?: number;
}

/**
 * rawUrl returns https://github.com/<owner>/<repo>/raw/<sha>/<path> with
 * every segment URI-encoded, or null when a part is empty or path has an
 * empty, "." or ".." segment (git never stores such paths, and URL parsing
 * would resolve them).
 */
export function rawUrl(owner: string, repo: string, sha: string, path: string): string | null {
  const segments = [owner, repo, 'raw', sha, ...path.split('/')];
  if (segments.some((s) => s === '' || s === '.' || s === '..')) return null;
  return `https://github.com/${segments.map(encodeURIComponent).join('/')}`;
}

/**
 * Semaphore runs at most n tasks at a time and queues the rest in order. An
 * n below 1 or not finite counts as 1.
 */
export class Semaphore {
  #free: number;
  readonly #waiting: (() => void)[] = [];

  constructor(n: number) {
    this.#free = Number.isFinite(n) && n >= 1 ? Math.floor(n) : 1;
  }

  async run<T>(task: () => Promise<T>): Promise<T> {
    if (this.#free > 0) this.#free--;
    else await new Promise<void>((resolve) => this.#waiting.push(resolve));
    try {
      return await task();
    } finally {
      const next = this.#waiting.shift();
      if (next) next();
      else this.#free++;
    }
  }
}

/**
 * SessionFetcher GETs raw files (fetchText) and pages (fetchPage) from
 * github.com with the page's cookies. A request that has not finished, body included,
 * after the timeout is aborted and fails with reason network, which frees
 * its slot.
 *
 * S3: synthetic tests model the responses. Verify on real private
 * repositories that /raw/ redirects to raw.githubusercontent.com, that the
 * redirected response is readable from the content script (CORS), and how
 * sign-in and SAML SSO failures look.
 */
export class SessionFetcher implements SourceFetcher {
  readonly #fetch: FetchFn;
  readonly #limit: Semaphore;
  readonly #maxBytes: number;
  readonly #timeoutMs: number;

  constructor(opts: SessionFetcherOptions = {}) {
    // Calling fetch through globalThis keeps its receiver; a detached
    // window.fetch throws "Illegal invocation".
    this.#fetch = opts.fetch ?? ((url, init) => globalThis.fetch(url, init));
    this.#limit = new Semaphore(opts.maxInFlight ?? MAX_IN_FLIGHT);
    this.#maxBytes = inRange(opts.maxBytes, 0, Infinity) ? opts.maxBytes : MAX_SOURCE_BYTES;
    // setTimeout fires at once for delays above 2^31 - 1.
    this.#timeoutMs = inRange(opts.timeoutMs, 1, 2 ** 31 - 1) ? opts.timeoutMs : REQUEST_TIMEOUT_MS;
  }

  fetchText(owner: string, repo: string, sha: string, path: string): Promise<FetchResult> {
    const url = rawUrl(owner, repo, sha, path);
    if (url === null) return Promise.resolve({ ok: false, reason: 'not-found' });
    return this.#limit.run(() => this.#getWithTimeout(url, this.#maxBytes));
  }

  /**
   * fetchPage GETs a github.com page with the page's cookies, up to
   * MAX_PAGE_BYTES. A URL on any other origin is not-found. It drops the
   * hash, which the server never sees.
   */
  fetchPage(url: string): Promise<FetchResult> {
    let u: URL;
    try {
      u = new URL(url);
    } catch {
      return Promise.resolve({ ok: false, reason: 'not-found' });
    }
    if (u.origin !== 'https://github.com') return Promise.resolve({ ok: false, reason: 'not-found' });
    u.hash = '';
    const href = u.href;
    return this.#limit.run(() => this.#getWithTimeout(href, MAX_PAGE_BYTES));
  }

  /**
   * getWithTimeout aborts the request after the timeout. It also stops
   * waiting then, so a fetch function that ignores the signal cannot keep
   * the slot.
   */
  async #getWithTimeout(url: string, maxBytes: number): Promise<FetchResult> {
    const ctrl = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    const timeout = new Promise<FetchResult>((resolve) => {
      timer = setTimeout(() => {
        ctrl.abort();
        resolve({ ok: false, reason: 'network' });
      }, this.#timeoutMs);
    });
    try {
      return await Promise.race([this.#get(url, ctrl.signal, maxBytes), timeout]);
    } finally {
      clearTimeout(timer);
    }
  }

  /** get never rejects: every error becomes a failed result. */
  async #get(url: string, signal: AbortSignal, maxBytes: number): Promise<FetchResult> {
    try {
      const res = await this.#fetch(url, { credentials: 'same-origin', redirect: 'follow', signal });
      const { status } = res;
      const page = signInPage(res.url);
      if (status === 401 || status === 403) {
        const text = await prefix(res);
        const sso = page === 'sso' || SSO_TEXT.test(text);
        return { ok: false, reason: sso ? 'sso' : 'unauthorized', status };
      }
      if (!res.ok || page) {
        await discard(res);
        // A redirect to a sign-in page answers 200. Server errors and rate
        // limits have no reason of their own and count as network errors.
        const reason = page ?? (status === 404 ? 'not-found' : 'network');
        return { ok: false, reason, status };
      }
      const bytes = await readAtMost(res, maxBytes);
      if (bytes === null) return { ok: false, reason: 'too-large', status };
      return { ok: true, text: new TextDecoder().decode(bytes) };
    } catch {
      return { ok: false, reason: 'network' };
    }
  }
}

/** inRange reports whether value is a number from min to max. NaN is not. */
function inRange(value: number | undefined, min: number, max: number): value is number {
  return value !== undefined && value >= min && value <= max;
}

const SSO_TEXT = /\bSAML\b|\bSSO\b|single sign-on/i;

/**
 * signInPage reports whether url is a github.com sign-in page ("unauthorized")
 * or SAML SSO page ("sso"). Only github.com paths count: a repository file
 * path may contain "sso" too.
 */
function signInPage(url: string): 'unauthorized' | 'sso' | null {
  let u: URL;
  try {
    u = new URL(url);
  } catch {
    return null;
  }
  if (u.origin !== 'https://github.com') return null;
  const p = u.pathname.toLowerCase();
  if (/^\/(orgs|enterprises)\/[^/]+\/(sso|saml)(\/|$)/.test(p) || /^\/(sso|saml)(\/|$)/.test(p)) return 'sso';
  if (/^\/(login|session|sessions)(\/|$)/.test(p)) return 'unauthorized';
  return null;
}

/** discard cancels the body of a response that is not read. */
async function discard(res: Response): Promise<void> {
  try {
    await res.body?.cancel();
  } catch {
    // The body is not needed, so a failed cancel changes nothing.
  }
}

/** prefix returns the first 64 KiB of the body as text, or "" when it cannot be read. */
async function prefix(res: Response): Promise<string> {
  try {
    const bytes = await readAtMost(res, 64 * 1024, true);
    return bytes ? new TextDecoder().decode(bytes) : '';
  } catch {
    return '';
  }
}

/**
 * readAtMost reads the body, or returns null when it is longer than max
 * bytes by the Content-Length header or by the bytes received. With
 * truncate, it returns the first max bytes instead of null.
 */
async function readAtMost(res: Response, max: number, truncate = false): Promise<Uint8Array | null> {
  const declared = Number(res.headers.get('content-length') ?? '');
  if (!truncate && declared > max) {
    await discard(res);
    return null;
  }
  if (!res.body) return new Uint8Array(0);
  const reader = res.body.getReader();
  const chunks: Uint8Array[] = [];
  let size = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    if (size + value.byteLength > max) {
      // A failed cancel leaves the result unchanged.
      await reader.cancel().catch(() => undefined);
      if (!truncate) return null;
      chunks.push(value.subarray(0, max - size));
      size = max;
      break;
    }
    chunks.push(value);
    size += value.byteLength;
  }
  const out = new Uint8Array(size);
  let at = 0;
  for (const c of chunks) {
    out.set(c, at);
    at += c.byteLength;
  }
  return out;
}
