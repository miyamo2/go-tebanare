// Signs in to github.com with the account in E2E_GH_USERNAME and
// E2E_GH_PASSWORD, and keeps the session as a Playwright storage state, so
// the live tests can open pull requests as a signed-in reader.

import { createHmac } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium, type BrowserContext, type Page } from '@playwright/test';

const packageRoot = join(dirname(fileURLToPath(import.meta.url)), '..');

/**
 * authStatePath is the file that holds the session: E2E_GH_AUTH_STATE, or
 * e2e-live/.auth/github.json, which .gitignore leaves out. It holds session
 * cookies, so it must never be committed or uploaded.
 */
export function authStatePath(): string {
  return process.env['E2E_GH_AUTH_STATE'] || join(packageRoot, 'e2e-live', '.auth', 'github.json');
}

interface Credentials {
  username: string;
  password: string;
  /** The base32 secret of an authenticator app, when the account uses one for two-factor authentication. */
  totpSecret?: string;
}

function credentials(): Credentials {
  const username = process.env['E2E_GH_USERNAME'] ?? '';
  const password = process.env['E2E_GH_PASSWORD'] ?? '';
  if (username === '' || password === '') {
    throw new Error(
      'The live end-to-end tests sign in to github.com: set E2E_GH_USERNAME and E2E_GH_PASSWORD, ' +
        'for example in packages/chrome-extension/.env.e2e-live (see .env.e2e-live.example).',
    );
  }
  const totpSecret = process.env['E2E_GH_TOTP_SECRET'];
  return totpSecret ? { username, password, totpSecret } : { username, password };
}

/** signedInLogin returns the content of the page's user-login meta tag: the signed-in login, or "" when signed out. */
async function signedInLogin(page: Page): Promise<string> {
  const login = await page.locator('meta[name="user-login"]').first().getAttribute('content', { timeout: 5_000 }).catch(() => null);
  return login?.trim() ?? '';
}

/**
 * totp returns the current RFC 6238 code of a base32 secret: HMAC-SHA1,
 * 30-second steps, 6 digits, as authenticator apps compute it for GitHub.
 */
export function totp(secret: string, now = Date.now()): string {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const c of secret.toUpperCase().replace(/[\s=-]/g, '')) {
    const v = alphabet.indexOf(c);
    if (v < 0) throw new Error('E2E_GH_TOTP_SECRET is not base32');
    bits += v.toString(2).padStart(5, '0');
  }
  const key = Buffer.from((bits.match(/.{8}/g) ?? []).map((b) => parseInt(b, 2)));
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(now / 1000 / 30)));
  const mac = createHmac('sha1', key).update(counter).digest();
  const offset = (mac[mac.length - 1] ?? 0) & 0xf;
  return String((mac.readUInt32BE(offset) & 0x7fffffff) % 1_000_000).padStart(6, '0');
}

/** SIGN_IN_TIMEOUT bounds each step of the sign-in; a person verifying the device by hand gets VERIFY_BY_HAND_TIMEOUT. */
const SIGN_IN_TIMEOUT = 60_000;
const VERIFY_BY_HAND_TIMEOUT = 5 * 60_000;

/**
 * waitAfterSubmit waits until the page leaves the form just submitted:
 * GitHub either signs the reader in or asks for a second step. It returns
 * the path of the page it lands on.
 */
async function waitAfterSubmit(page: Page, from: string): Promise<string> {
  await page.waitForURL((u) => u.pathname !== from, { timeout: SIGN_IN_TIMEOUT }).catch(() => undefined);
  await page.waitForLoadState('domcontentloaded');
  return new URL(page.url()).pathname;
}

async function signIn(context: BrowserContext, creds: Credentials, headed: boolean): Promise<void> {
  const page = await context.newPage();
  await page.goto('https://github.com/login');
  await page.locator('#login_field').fill(creds.username);
  await page.locator('#password').fill(creds.password);
  await page.locator('input[type="submit"][name="commit"]').click();
  let path = await waitAfterSubmit(page, '/login');

  if (path === '/session' || path === '/login') {
    const flash = (await page.locator('.flash-error, #js-flash-container .flash').first().textContent({ timeout: 2_000 }).catch(() => null))?.trim();
    throw new Error(`GitHub rejected the sign-in of E2E_GH_USERNAME${flash ? `: ${flash}` : ''}`);
  }
  if (path.startsWith('/sessions/two-factor')) {
    if (!creds.totpSecret) {
      throw new Error('GitHub asks for a two-factor code: set E2E_GH_TOTP_SECRET to the base32 secret of the account\'s authenticator app.');
    }
    // Accounts with passkeys or security keys land on another method first.
    if (!path.startsWith('/sessions/two-factor/app')) {
      await page.goto('https://github.com/sessions/two-factor/app');
    }
    const input = page.locator('#app_totp, input[name="app_otp"]').first();
    await input.fill(totp(creds.totpSecret));
    // GitHub submits the form once the sixth digit is in.
    path = await waitAfterSubmit(page, new URL(page.url()).pathname);
  }
  if (path.startsWith('/sessions/verified-device')) {
    // Accounts without two-factor authentication get an e-mailed code on an
    // unknown device, and every CI run is one.
    if (!headed || process.env['CI']) {
      throw new Error(
        'GitHub asks to verify this device with a code it e-mailed to the account. Run the tests with --headed and enter the code ' +
          'in the browser, or enable two-factor authentication with an authenticator app on the account and set E2E_GH_TOTP_SECRET.',
      );
    }
    console.log(`GitHub e-mailed a device verification code. Enter it in the browser within ${VERIFY_BY_HAND_TIMEOUT / 60_000} minutes.`);
    await page.waitForURL((u) => !u.pathname.startsWith('/sessions/verified-device'), { timeout: VERIFY_BY_HAND_TIMEOUT });
  }

  await page.goto('https://github.com/');
  const login = await signedInLogin(page);
  if (login.toLowerCase() !== creds.username.toLowerCase()) {
    throw new Error(`The sign-in stopped at ${new URL(page.url()).pathname}: GitHub does not show E2E_GH_USERNAME as signed in`);
  }
  await page.close();
}

/**
 * ensureSession writes a signed-in storage state to authStatePath. A saved
 * state that GitHub still accepts for the same account is kept, so local
 * runs do not sign in again, and do not trigger a device check, on every run.
 */
export async function ensureSession(headed: boolean): Promise<void> {
  const creds = credentials();
  const path = authStatePath();
  const browser = await chromium.launch({ channel: 'chromium', headless: !headed });
  try {
    if (existsSync(path)) {
      const context = await browser.newContext({ storageState: path });
      const page = await context.newPage();
      await page.goto('https://github.com/');
      const login = await signedInLogin(page);
      await context.close();
      if (login.toLowerCase() === creds.username.toLowerCase()) return;
    }
    const context = await browser.newContext();
    await signIn(context, creds, headed);
    mkdirSync(dirname(path), { recursive: true });
    writeFileSync(path, JSON.stringify(await context.storageState()), { mode: 0o600 });
    await context.close();
  } finally {
    await browser.close();
  }
}

/** sessionCookies returns the cookies of the storage state that ensureSession wrote. */
export function sessionCookies(): Parameters<BrowserContext['addCookies']>[0] {
  const state = JSON.parse(readFileSync(authStatePath(), 'utf8')) as { cookies: Parameters<BrowserContext['addCookies']>[0] };
  return state.cookies;
}
