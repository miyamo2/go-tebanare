// Fixtures for the end-to-end tests. Each test gets a new Chromium profile
// with dist/ loaded as an unpacked extension, and a fake github.com:
// context.route answers every https://github.com request from e2e/fixtures,
// so the tests never reach the network.

import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, chromium, type BrowserContext, type Locator, type Page, type Worker } from '@playwright/test';
import type { BgRequest, PageStatus } from '../src/shared/messages.js';

export { expect } from '@playwright/test';

const packageRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const distDir = join(packageRoot, 'dist');
const fixtureDir = join(packageRoot, 'e2e', 'fixtures');

// The commits that the hidden inputs of fixtures/pull-7-files.html name.
export const BASE_SHA = '6f1e0c3a9b7d4e2f8a1c5b3d7e9f0a2b4c6d8e1f';
export const HEAD_SHA = 'c4a7e2d9f1b3a5c7e9d0b2f4a6c8e1d3f5a7b9c2';
export const REPO = 'acme/widgets';
export const PULL_NUMBER = 7;
export const PULL_URL = `https://github.com/${REPO}/pull/${PULL_NUMBER}/files`;

export function readFixture(path: string): string {
  return readFileSync(join(fixtureDir, path), 'utf8');
}

/** The files of each commit by path. A path a commit lacks answers 404. */
export type Commits = Record<string, Record<string, string>>;

/**
 * fixtureCommits returns the files under fixtures/testdata: the config and
 * store/store.go at BASE_SHA, and store/store.go at HEAD_SHA. Neither
 * commit has .gotebanare.yaml.
 */
export function fixtureCommits(): Commits {
  return {
    [BASE_SHA]: {
      '.gotebanare.yml': readFixture('testdata/base/.gotebanare.yml'),
      'store/store.go': readFixture('testdata/base/store/store.go'),
    },
    [HEAD_SHA]: { 'store/store.go': readFixture('testdata/head/store/store.go') },
  };
}

/**
 * serveGitHub answers the https://github.com requests of context: PULL_URL
 * gets pageHtml (fixtures/pull-7-files.html by default),
 * /acme/widgets/raw/<sha>/<path> gets the file from commits, and every
 * other request gets 404. It returns the list of requested paths, which
 * grows as requests arrive.
 */
export async function serveGitHub(context: BrowserContext, commits: Commits, pageHtml = readFixture('pull-7-files.html')): Promise<string[]> {
  const requested: string[] = [];
  const pagePath = new URL(PULL_URL).pathname;
  const rawPrefix = `/${REPO}/raw/`;
  await context.route('https://github.com/**', async (route) => {
    const { pathname } = new URL(route.request().url());
    requested.push(pathname);
    if (pathname === pagePath) {
      return route.fulfill({ contentType: 'text/html; charset=utf-8', body: pageHtml });
    }
    if (pathname.startsWith(rawPrefix)) {
      const [sha = '', ...segments] = pathname.slice(rawPrefix.length).split('/');
      const text = commits[sha]?.[segments.map(decodeURIComponent).join('/')];
      if (text !== undefined) return route.fulfill({ contentType: 'text/plain; charset=utf-8', body: text });
    }
    return route.fulfill({ status: 404, contentType: 'text/plain; charset=utf-8', body: 'Not Found' });
  });
  return requested;
}

interface ExtensionFixtures {
  /** A persistent context with dist/ loaded. The built-in page fixture opens its page here. */
  context: BrowserContext;
  /** The extension's service worker (background.js). */
  serviceWorker: Worker;
}

interface Launched {
  context: BrowserContext;
  worker: Worker;
  profile: string;
}

/** LAUNCH_ATTEMPTS bounds the browser starts of one test. */
const LAUNCH_ATTEMPTS = 3;

/**
 * launch starts Chromium with dist/ loaded and returns once Playwright
 * reports the extension's service worker. Chromium loads unpacked
 * extensions only into a persistent context, and the "chromium" channel
 * runs the new headless mode, which supports them.
 *
 * On a busy machine Chromium can start the service worker before
 * Playwright attaches to new targets, and Playwright then never reports
 * it. launch starts the browser again with a new profile when no worker
 * shows up within 10 s.
 *
 * Chromium picks the UI locale, and so the extension's strings, from the
 * environment. LANGUAGE=en pins it to English, so the tests pass on
 * machines with another locale as well.
 */
async function launch(headless: boolean): Promise<Launched> {
  for (let attempt = 1; ; attempt++) {
    const profile = mkdtempSync(join(tmpdir(), 'gotebanare-e2e-'));
    const context = await chromium.launchPersistentContext(profile, {
      channel: 'chromium',
      headless,
      env: { ...process.env, LANGUAGE: 'en' },
      args: [`--disable-extensions-except=${distDir}`, `--load-extension=${distDir}`],
    });
    const worker =
      context.serviceWorkers()[0] ?? (await context.waitForEvent('serviceworker', { timeout: 10_000 }).catch(() => undefined));
    if (worker) return { context, worker, profile };
    await context.close();
    rmSync(profile, { recursive: true, force: true });
    if (attempt === LAUNCH_ATTEMPTS) throw new Error(`no extension service worker in ${LAUNCH_ATTEMPTS} browser starts`);
  }
}

const workers = new WeakMap<BrowserContext, Worker>();

export const test = base.extend<ExtensionFixtures>({
  context: async ({ headless }, use) => {
    const { context, worker, profile } = await launch(headless);
    workers.set(context, worker);
    try {
      await use(context);
    } finally {
      await context.close();
      rmSync(profile, { recursive: true, force: true });
    }
  },
  serviceWorker: async ({ context }, use) => {
    const worker = workers.get(context);
    if (!worker) throw new Error('the context fixture did not record a service worker');
    await use(worker);
  },
});

/** newRows returns the diff rows with the given new-side line numbers. */
export function newRows(page: Page, lines: readonly number[]): Locator[] {
  return lines.map((n) => page.locator(`table.diff-table tr:has(td.blob-num:nth-child(2)[data-line-number="${n}"])`));
}

/**
 * tabIdOf returns the id of the tab that shows PULL_URL. The extension has
 * no "tabs" permission, so tab URLs are not visible; the service worker
 * asks each tab's content script for its status instead.
 */
export async function tabIdOf(worker: Worker): Promise<number> {
  const id = await worker.evaluate(
    async ([repo, pr]) => {
      for (const tab of await chrome.tabs.query({})) {
        if (tab.id === undefined) continue;
        const status = (await chrome.tabs.sendMessage(tab.id, { type: 'status' }).catch(() => null)) as PageStatus | null;
        if (status?.repo === repo && status.pr === pr) return tab.id;
      }
      return null;
    },
    [REPO, PULL_NUMBER] as const,
  );
  if (id === null) throw new Error(`no tab shows ${PULL_URL}`);
  return id;
}

/** pageStatus asks the content script of tabId for its status, as the popup does. */
export function pageStatus(worker: Worker, tabId: number): Promise<PageStatus> {
  return worker.evaluate((id) => chrome.tabs.sendMessage(id, { type: 'status' }) as Promise<PageStatus>, tabId);
}

/**
 * sendFromPopup opens popup.html in a new tab, sends req to the service
 * worker from there with chrome.runtime.sendMessage, as the popup does,
 * and closes the tab. It returns the answer.
 */
export async function sendFromPopup(context: BrowserContext, worker: Worker, req: BgRequest): Promise<unknown> {
  const popup = await context.newPage();
  try {
    await popup.goto(new URL('popup.html', worker.url()).href);
    return await popup.evaluate((r) => chrome.runtime.sendMessage(r) as Promise<unknown>, req);
  } finally {
    await popup.close();
  }
}

/**
 * runCommand fires chrome.commands.onCommand in the service worker for
 * tabId, the event Chrome fires when the reader presses the command's
 * shortcut. Chromium's extension bindings give every event a dispatch
 * method, which the typed chrome API leaves out.
 */
export async function runCommand(worker: Worker, command: string, tabId: number): Promise<void> {
  await worker.evaluate(
    ([name, id]) => {
      const event = chrome.commands.onCommand as unknown as { dispatch(command: string, tab: { id: number }): void };
      event.dispatch(name, { id });
    },
    [command, tabId] as const,
  );
}
