// Checks that the version in package.json can be released after the given
// earlier releases, and prints what the release workflows need as
// key=value lines for $GITHUB_OUTPUT. tag-chrome-extension.yml and
// release-chrome-extension.yml run it before they tag or release.
//
//   node scripts/release-version.mjs [released-version...]
//
// Each argument is the version of an earlier release (a tag without its
// prefix). Arguments that are not release versions are ignored. It prints:
//
//   version=0.3.0-rc.1        the package.json version
//   manifest-version=0.3.0    the version manifest.json must hold
//   prerelease=true           true for a version such as 0.3.0-rc.1, which
//                             skips the Chrome Web Store
//   notes-start=0.2.0         the release the notes start after, if any

import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { releasePlan } from './build-lib.mjs';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');

const released = process.argv.slice(2);
if (released.some((v) => v.startsWith('-'))) {
  console.error('usage: node scripts/release-version.mjs [released-version...]');
  process.exit(2);
}

const version = String(JSON.parse(readFileSync(join(root, 'package.json'), 'utf8')).version);
const plan = releasePlan(version, released);
if (plan.errors.length > 0) {
  for (const e of plan.errors) console.error(`packages/chrome-extension/package.json: ${e}`);
  process.exit(1);
}
console.log(`version=${plan.version}`);
console.log(`manifest-version=${plan.manifestVersion}`);
console.log(`prerelease=${plan.prerelease}`);
if (plan.notesStart !== undefined) console.log(`notes-start=${plan.notesStart}`);
