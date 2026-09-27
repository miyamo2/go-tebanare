// Live end-to-end tests: e2e-live/*.spec.ts load dist/ into Chromium, sign
// in to github.com, and run the extension on real pull requests. Unlike
// playwright.config.ts they need the network and the E2E_GH_* account.

import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: 'e2e-live',
  globalSetup: './e2e-live/global-setup.ts',
  // One browser at a time keeps the account's traffic to github.com low.
  workers: 1,
  forbidOnly: Boolean(process.env['CI']),
  // github.com can be slow to render a pull request, so one failure on CI gets one more try.
  retries: process.env['CI'] ? 1 : 0,
  timeout: 120_000,
  expect: { timeout: 30_000 },
  reporter: 'list',
  use: { headless: true },
});
