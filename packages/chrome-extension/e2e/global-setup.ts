// Builds dist/ before the end-to-end tests, so they load the current sources.

import { execFileSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const packageRoot = join(dirname(fileURLToPath(import.meta.url)), '..');

export default function globalSetup(): void {
  // stderr keeps the build's warnings; stdout only lists the files of dist/.
  execFileSync(process.execPath, [join(packageRoot, 'scripts', 'build.mjs')], { cwd: packageRoot, stdio: ['ignore', 'ignore', 'inherit'] });
  if (!existsSync(join(packageRoot, 'dist', 'engine.wasm'))) {
    throw new Error('dist/engine.wasm is missing. Run "make wasm" at the repository root first.');
  }
}
