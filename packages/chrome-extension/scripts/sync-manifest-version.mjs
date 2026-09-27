// Sets the version in manifest.json to the one in package.json and leaves the
// rest of the file alone. tag-chrome-extension.yml runs it before tagging,
// because release-chrome-extension.yml requires manifest.json at the tag to
// hold the tag's version. build.mjs writes the package.json version into
// dist/manifest.json on every build, so no other build needs it.
//
//   node scripts/sync-manifest-version.mjs

import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromeVersion, withVersion } from './build-lib.mjs';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const manifestPath = join(root, 'manifest.json');

const { version } = chromeVersion(String(JSON.parse(readFileSync(join(root, 'package.json'), 'utf8')).version));
const text = readFileSync(manifestPath, 'utf8');
const next = withVersion(text, version);
if (next !== text) writeFileSync(manifestPath, next);
console.log(`manifest.json: version ${version}`);
