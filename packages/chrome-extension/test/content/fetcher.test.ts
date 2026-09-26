import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  MAX_IN_FLIGHT,
  MAX_SOURCE_BYTES,
  REQUEST_TIMEOUT_MS,
  Semaphore,
  SessionFetcher,
  rawUrl,
  type FetchFn,
} from '../../src/content/fetcher.js';

const SHA = '0123456789abcdef0123456789abcdef01234567';
const RAW = `https://github.com/octo/repo/raw/${SHA}/user.go`;

/** respond builds a Response whose url is the final URL after redirects. */
function respond(body: BodyInit | null, status = 200, url = RAW, headers: HeadersInit = {}): Response {
  const res = new Response(body, { status, headers });
  // Constructed responses have an empty url; fetch sets it to the final URL.
  Object.defineProperty(res, 'url', { value: url });
  return res;
}

function fetcherFor(res: Response | (() => Promise<Response>), opts: { maxBytes?: number; timeoutMs?: number } = {}) {
  const fetch = vi.fn<FetchFn>(async () => (typeof res === 'function' ? res() : res));
  return { fetch, fetcher: new SessionFetcher({ fetch, ...opts }) };
}

/** chunks streams the given byte chunks as a body. */
function chunks(...parts: Uint8Array[]): ReadableStream<Uint8Array> {
  return new ReadableStream({
    start(c) {
      for (const p of parts) c.enqueue(p);
      c.close();
    },
  });
}

describe('rawUrl', () => {
  it('encodes each path segment', () => {
    expect(rawUrl('octo', 'repo', SHA, 'cmd/a b/#1?.go')).toBe(`https://github.com/octo/repo/raw/${SHA}/cmd/a%20b/%231%3F.go`);
    expect(rawUrl('octo', 'repo', SHA, 'dir/ユーザー.go')).toBe(
      `https://github.com/octo/repo/raw/${SHA}/dir/%E3%83%A6%E3%83%BC%E3%82%B6%E3%83%BC.go`,
    );
  });

  it.each(['', '/user.go', 'dir//user.go', 'dir/', './user.go', 'dir/../user.go'])('rejects the path %j', (path) => {
    expect(rawUrl('octo', 'repo', SHA, path)).toBeNull();
  });

  it('rejects empty owner, repo, or sha', () => {
    expect(rawUrl('', 'repo', SHA, 'a.go')).toBeNull();
    expect(rawUrl('octo', '', SHA, 'a.go')).toBeNull();
    expect(rawUrl('octo', 'repo', '', 'a.go')).toBeNull();
  });
});

/** hanging returns a fetch function that never settles and the signals it received. */
function hanging() {
  const signals: AbortSignal[] = [];
  const fetch = vi.fn<FetchFn>((_, init) => {
    if (init.signal) signals.push(init.signal);
    return new Promise<Response>(() => undefined);
  });
  return { fetch, signals };
}

describe('SessionFetcher', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it('GETs the raw URL with the session cookies and returns the text', async () => {
    const { fetch, fetcher } = fetcherFor(respond('package p\n\nfunc F() {}\n'));
    expect(await fetcher.fetchText('octo', 'repo', SHA, 'user.go')).toEqual({ ok: true, text: 'package p\n\nfunc F() {}\n' });
    expect(fetch).toHaveBeenCalledWith(RAW, { credentials: 'same-origin', redirect: 'follow', signal: expect.any(AbortSignal) });
  });

  it('follows a redirect to raw.githubusercontent.com, even under a directory named sso', async () => {
    const url = `https://raw.githubusercontent.com/octo/repo/${SHA}/internal/sso/login.go?token=x`;
    const { fetcher } = fetcherFor(respond('package sso\n', 200, url));
    expect(await fetcher.fetchText('octo', 'repo', SHA, 'internal/sso/login.go')).toEqual({ ok: true, text: 'package sso\n' });
  });

  it('decodes UTF-8 split across chunks', async () => {
    const bytes = new TextEncoder().encode('// ユーザー\n');
    const { fetcher } = fetcherFor(respond(chunks(bytes.subarray(0, 4), bytes.subarray(4))));
    expect(await fetcher.fetchText('octo', 'repo', SHA, 'user.go')).toEqual({ ok: true, text: '// ユーザー\n' });
  });

  it.each([
    ['404', respond('Not Found', 404), { ok: false, reason: 'not-found', status: 404 }],
    ['401', respond('', 401), { ok: false, reason: 'unauthorized', status: 401 }],
    ['403', respond('Forbidden', 403), { ok: false, reason: 'unauthorized', status: 403 }],
    ['403 with SAML in the body', respond('Resource protected by organization SAML enforcement.', 403), { ok: false, reason: 'sso', status: 403 }],
    ['403 with SSO in the body', respond('Re-authorize with SSO to continue.', 403), { ok: false, reason: 'sso', status: 403 }],
    ['403 on an SSO page', respond('', 403, 'https://github.com/orgs/octo/sso?return_to=x'), { ok: false, reason: 'sso', status: 403 }],
    ['a redirect to the sign-in page', respond('<html>', 200, 'https://github.com/login?return_to=x'), { ok: false, reason: 'unauthorized', status: 200 }],
    ['a redirect to an SSO page', respond('<html>', 200, 'https://github.com/enterprises/acme/sso'), { ok: false, reason: 'sso', status: 200 }],
    ['500', respond('', 500), { ok: false, reason: 'network', status: 500 }],
    ['429', respond('', 429), { ok: false, reason: 'network', status: 429 }],
  ])('maps %s', async (_, res, want) => {
    const { fetcher } = fetcherFor(res);
    expect(await fetcher.fetchText('octo', 'repo', SHA, 'user.go')).toEqual(want);
  });

  it('maps a rejected fetch and a failing body to network', async () => {
    const { fetcher } = fetcherFor(() => Promise.reject(new TypeError('Failed to fetch')));
    expect(await fetcher.fetchText('octo', 'repo', SHA, 'user.go')).toEqual({ ok: false, reason: 'network' });
    const broken = new ReadableStream<Uint8Array>({
      pull(c) {
        c.error(new TypeError('network error'));
      },
    });
    const second = fetcherFor(respond(broken));
    expect(await second.fetcher.fetchText('octo', 'repo', SHA, 'user.go')).toEqual({ ok: false, reason: 'network' });
  });

  it('limits the size by Content-Length and by the bytes received', async () => {
    const declared = fetcherFor(respond('x', 200, RAW, { 'content-length': String(MAX_SOURCE_BYTES + 1) }));
    expect(await declared.fetcher.fetchText('octo', 'repo', SHA, 'a.go')).toEqual({ ok: false, reason: 'too-large', status: 200 });
    const received = fetcherFor(respond('x'.repeat(MAX_SOURCE_BYTES + 1)));
    expect(await received.fetcher.fetchText('octo', 'repo', SHA, 'a.go')).toEqual({ ok: false, reason: 'too-large', status: 200 });
    const exact = fetcherFor(respond('x'.repeat(MAX_SOURCE_BYTES)));
    expect(await exact.fetcher.fetchText('octo', 'repo', SHA, 'a.go')).toMatchObject({ ok: true });
  });

  it('counts the limit over every chunk', async () => {
    const one = new Uint8Array(3).fill(0x61);
    const over = fetcherFor(respond(chunks(one, one, one)), { maxBytes: 8 });
    expect(await over.fetcher.fetchText('octo', 'repo', SHA, 'a.go')).toEqual({ ok: false, reason: 'too-large', status: 200 });
    const fits = fetcherFor(respond(chunks(one, one)), { maxBytes: 6 });
    expect(await fits.fetcher.fetchText('octo', 'repo', SHA, 'a.go')).toEqual({ ok: true, text: 'aaaaaa' });
  });

  it('answers not-found for a path it cannot request, without fetching', async () => {
    const { fetch, fetcher } = fetcherFor(respond(''));
    expect(await fetcher.fetchText('octo', 'repo', SHA, '../x.go')).toEqual({ ok: false, reason: 'not-found' });
    expect(fetch).not.toHaveBeenCalled();
  });

  it('calls the global fetch by default', async () => {
    const fetch = vi.fn(async () => respond('package p\n'));
    vi.stubGlobal('fetch', fetch);
    expect(await new SessionFetcher().fetchText('octo', 'repo', SHA, 'user.go')).toEqual({ ok: true, text: 'package p\n' });
    expect(fetch).toHaveBeenCalledOnce();
  });

  it(`keeps at most ${MAX_IN_FLIGHT} requests in flight`, async () => {
    const pending: ((r: Response) => void)[] = [];
    const fetch = vi.fn<FetchFn>(() => new Promise<Response>((resolve) => pending.push(resolve)));
    const fetcher = new SessionFetcher({ fetch });
    const results = Array.from({ length: 6 }, (_, i) => fetcher.fetchText('octo', 'repo', SHA, `f${i}.go`));
    await vi.waitFor(() => expect(fetch).toHaveBeenCalledTimes(MAX_IN_FLIGHT));
    pending[0]?.(respond('0'));
    await vi.waitFor(() => expect(fetch).toHaveBeenCalledTimes(MAX_IN_FLIGHT + 1));
    expect(fetch.mock.calls[MAX_IN_FLIGHT]?.[0]).toBe(`https://github.com/octo/repo/raw/${SHA}/f4.go`);
    for (let i = 1; i < 6; i++) {
      await vi.waitFor(() => expect(pending.length).toBeGreaterThan(i));
      pending[i]?.(respond(String(i)));
    }
    expect((await Promise.all(results)).map((r) => (r.ok ? r.text : r.reason))).toEqual(['0', '1', '2', '3', '4', '5']);
  });

  it(`aborts a request after ${REQUEST_TIMEOUT_MS} ms and frees its slot`, async () => {
    vi.useFakeTimers();
    const { fetch, signals } = hanging();
    const fetcher = new SessionFetcher({ fetch });
    const settled: unknown[] = [];
    for (let i = 0; i <= MAX_IN_FLIGHT; i++) void fetcher.fetchText('octo', 'repo', SHA, `f${i}.go`).then((r) => settled.push(r));
    await vi.advanceTimersByTimeAsync(REQUEST_TIMEOUT_MS - 1);
    expect(fetch).toHaveBeenCalledTimes(MAX_IN_FLIGHT);
    expect(settled).toEqual([]);
    await vi.advanceTimersByTimeAsync(1);
    expect(settled).toEqual(Array.from({ length: MAX_IN_FLIGHT }, () => ({ ok: false, reason: 'network' })));
    expect(signals.slice(0, MAX_IN_FLIGHT).every((s) => s.aborted)).toBe(true);
    expect(fetch).toHaveBeenCalledTimes(MAX_IN_FLIGHT + 1);
    await vi.advanceTimersByTimeAsync(REQUEST_TIMEOUT_MS);
    expect(settled).toHaveLength(MAX_IN_FLIGHT + 1);
  });

  it('applies the timeout to a body that stops arriving', async () => {
    const stalled = new ReadableStream<Uint8Array>({
      start(c) {
        c.enqueue(new Uint8Array([0x61]));
      },
    });
    const { fetch, fetcher } = fetcherFor(respond(stalled), { timeoutMs: 5 });
    expect(await fetcher.fetchText('octo', 'repo', SHA, 'a.go')).toEqual({ ok: false, reason: 'network' });
    expect(fetch.mock.calls[0]?.[1].signal?.aborted).toBe(true);
  });

  it('clears the timer when the request finishes', async () => {
    vi.useFakeTimers();
    const { fetch, fetcher } = fetcherFor(respond('package p\n'));
    expect(await fetcher.fetchText('octo', 'repo', SHA, 'user.go')).toMatchObject({ ok: true });
    expect(vi.getTimerCount()).toBe(0);
    expect(fetch.mock.calls[0]?.[1].signal?.aborted).toBe(false);
  });

  it.each([NaN, 0, -1, 2 ** 31])('uses the default timeout for timeoutMs %s', async (timeoutMs) => {
    vi.useFakeTimers();
    const { fetch } = hanging();
    let result: unknown;
    void new SessionFetcher({ fetch, timeoutMs }).fetchText('octo', 'repo', SHA, 'a.go').then((r) => (result = r));
    await vi.advanceTimersByTimeAsync(REQUEST_TIMEOUT_MS - 1);
    expect(result).toBeUndefined();
    await vi.advanceTimersByTimeAsync(1);
    expect(result).toEqual({ ok: false, reason: 'network' });
  });

  it.each([NaN, -1])('uses the default size limit for maxBytes %s', async (maxBytes) => {
    const over = fetcherFor(respond('x'.repeat(MAX_SOURCE_BYTES + 1)), { maxBytes });
    expect(await over.fetcher.fetchText('octo', 'repo', SHA, 'a.go')).toEqual({ ok: false, reason: 'too-large', status: 200 });
    const exact = fetcherFor(respond('x'.repeat(MAX_SOURCE_BYTES)), { maxBytes });
    expect(await exact.fetcher.fetchText('octo', 'repo', SHA, 'a.go')).toMatchObject({ ok: true });
  });
});

describe('Semaphore', () => {
  it('runs queued tasks in order and frees the slot when a task rejects', async () => {
    const sem = new Semaphore(1);
    const order: string[] = [];
    const first = sem.run(async () => {
      order.push('first');
      throw new Error('boom');
    });
    const second = sem.run(async () => {
      order.push('second');
      return 2;
    });
    await expect(first).rejects.toThrow('boom');
    expect(await second).toBe(2);
    expect(order).toEqual(['first', 'second']);
  });

  it.each([0, 0.5, -1, NaN, Infinity])('treats a size of %s as one', async (n) => {
    const sem = new Semaphore(n);
    let release = () => {};
    const first = sem.run(() => new Promise<string>((resolve) => (release = () => resolve('first'))));
    let secondRan = false;
    const second = sem.run(async () => {
      secondRan = true;
      return 'second';
    });
    await Promise.resolve();
    expect(secondRan).toBe(false);
    release();
    expect([await first, await second]).toEqual(['first', 'second']);
  });
});
