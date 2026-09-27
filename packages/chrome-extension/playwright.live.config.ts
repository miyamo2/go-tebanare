// Live end-to-end tests: e2e-live/*.spec.ts load dist/ into Chromium, sign
// in to github.com, and run the extension on the sample pull request
// miyamo2/go-tebanare-sample#1. Unlike
// playwright.config.ts they need the network and the E2E_GH_* account.

import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: 'e2e-live',
  globalSetup: './e2e-live/global-setup.ts',
  // A single worker keeps the account's traffic to github.com low.
  workers: 1,
  forbidOnly: Boolean(process.env['CI']),
  // On CI, Playwright retries a failed test once, because github.com can be slow to render a pull request.
  retries: process.env['CI'] ? 1 : 0,
  timeout: 120_000,
  expect: { timeout: 30_000 },
  reporter: 'list',
  use: { headless: true },
});
