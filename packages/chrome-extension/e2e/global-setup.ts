// Builds dist/ before the end-to-end tests, so they load the current
// sources, then signs in to github.com once, so every test starts with the
// same session.

import { execFileSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { FullConfig } from '@playwright/test';
import { ensureSession } from './github-session.js';

const packageRoot = join(dirname(fileURLToPath(import.meta.url)), '..');

export default async function globalSetup(config: FullConfig): Promise<void> {
  // stderr keeps the build's warnings; stdout only lists the files of dist/.
  execFileSync(process.execPath, [join(packageRoot, 'scripts', 'build.mjs')], { cwd: packageRoot, stdio: ['ignore', 'ignore', 'inherit'] });
  if (!existsSync(join(packageRoot, 'dist', 'engine.wasm'))) {
    throw new Error('dist/engine.wasm is missing. Run "make wasm" at the repository root first.');
  }
  await ensureSession(config.projects[0]?.use.headless === false);
}
