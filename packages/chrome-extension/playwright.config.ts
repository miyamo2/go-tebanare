// End-to-end tests: e2e/*.spec.ts load dist/ into Chromium as an unpacked
// extension and run it on synthetic GitHub pages.

import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: 'e2e',
  globalSetup: './e2e/global-setup.ts',
  // Each test starts its own browser with a new profile.
  fullyParallel: true,
  forbidOnly: Boolean(process.env['CI']),
  timeout: 60_000,
  // The first analysis waits for the service worker to load engine.wasm.
  expect: { timeout: 15_000 },
  reporter: 'list',
  use: { headless: true },
});
