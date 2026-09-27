// Prints the version in package.json after checking that it can be
// released to the Chrome Web Store. The release workflows run it; see
// .github/workflows/tag-chrome-extension.yml.
//
//   node scripts/release-version.mjs [--previous <version>]
//
// --previous names the last released version, which the new one must exceed.

import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { releaseVersionErrors } from './build-lib.mjs';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');

const args = process.argv.slice(2);
/** @type {string | undefined} */
let previous;
if (args.length === 2 && args[0] === '--previous' && args[1] !== '') {
  previous = args[1];
} else if (args.length !== 0) {
  console.error('usage: node scripts/release-version.mjs [--previous <version>]');
  process.exit(2);
}

const version = String(JSON.parse(readFileSync(join(root, 'package.json'), 'utf8')).version);
const errors = releaseVersionErrors(previous, version);
if (errors.length > 0) {
  for (const e of errors) console.error(`packages/chrome-extension/package.json: ${e}`);
  process.exit(1);
}
console.log(version);
