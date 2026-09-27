// Fixtures for the end-to-end tests. Each test gets a new Chromium profile
// with dist/ loaded as an unpacked extension and the github.com session
// that global-setup.ts signed in, so the tests open real pull requests as a
// signed-in reader.

import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, chromium, type BrowserContext, type Worker } from '@playwright/test';
import type { PageStatus } from '../src/shared/messages.js';
import { sessionCookies } from './github-session.js';

export { expect } from '@playwright/test';

const distDir = join(dirname(fileURLToPath(import.meta.url)), '..', 'dist');

interface ExtensionFixtures {
  /**
   * A persistent context with dist/ loaded and the session cookies of
   * global-setup.ts. The built-in page fixture opens its page here.
   */
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
      await context.addCookies(sessionCookies());
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

/**
 * tabIdOf returns the id of the tab that shows pull request pr of repo.
 * The extension has no "tabs" permission, so tab URLs are not visible; the
 * service worker asks each tab's content script for its status instead.
 * GitHub matches owner and repository names without
 * regard to case, and so does tabIdOf.
 */
export async function tabIdOf(worker: Worker, repo: string, pr: number): Promise<number> {
  const id = await worker.evaluate(
    async ([repo, pr]) => {
      for (const tab of await chrome.tabs.query({})) {
        if (tab.id === undefined) continue;
        const status = (await chrome.tabs.sendMessage(tab.id, { type: 'status' }).catch(() => null)) as PageStatus | null;
        if (status?.repo?.toLowerCase() === repo.toLowerCase() && status.pr === pr) return tab.id;
      }
      return null;
    },
    [repo, pr] as const,
  );
  if (id === null) throw new Error(`no tab shows pull request ${pr} of ${repo}`);
  return id;
}

/** pageStatus asks the content script of tabId for its status, as the popup does. */
export function pageStatus(worker: Worker, tabId: number): Promise<PageStatus> {
  return worker.evaluate((id) => chrome.tabs.sendMessage(id, { type: 'status' }) as Promise<PageStatus>, tabId);
}
