// Builds dist/ as the offline tests do, then signs in to github.com once,
// so every live test starts with the same session.

import type { FullConfig } from '@playwright/test';
import buildExtension from '../e2e/global-setup.js';
import { ensureSession } from './github-session.js';

export default async function globalSetup(config: FullConfig): Promise<void> {
  buildExtension();
  await ensureSession(config.projects[0]?.use.headless === false);
}
