// Sets the version in manifest.json to the one in package.json, keeping the
// rest of the file as it is. build.mjs writes the same version into
// dist/manifest.json in any case; this keeps the source in step for a
// release. See .github/workflows/tag-chrome-extension.yml.
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
