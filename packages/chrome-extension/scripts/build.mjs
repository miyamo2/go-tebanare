// Builds the unpacked extension into dist/. With --zip it also writes
// dist.zip for the Chrome Web Store.
//
//   node scripts/build.mjs [--zip] [--allow-stubs]
//
// --allow-stubs writes an empty script for each missing entry point, so the
// package builds before every entry exists. --zip refuses stubs.

import { build } from 'esbuild';
import { copyFileSync, cpSync, existsSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, relative, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildErrors, chromeVersion, esbuildTarget } from './build-lib.mjs';
import { zipFiles } from './zip.mjs';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const dist = join(root, 'dist');
const wasmSource = join(root, '..', 'engine', 'wasm', 'engine.wasm');

const flags = new Set(process.argv.slice(2));
for (const f of flags) {
  if (f !== '--zip' && f !== '--allow-stubs') {
    console.error(`unknown flag ${f}; usage: node scripts/build.mjs [--zip] [--allow-stubs]`);
    process.exit(2);
  }
}
const wantZip = flags.has('--zip');

// The service worker is an ES module (manifest "type": "module"); the
// other scripts run as classic scripts.
/** @type {ReadonlyArray<{ src: string; out: string; format: import('esbuild').Format }>} */
const entries = [
  { src: 'src/background/index.ts', out: 'background.js', format: 'esm' },
  { src: 'src/content/index.ts', out: 'content.js', format: 'iife' },
  { src: 'src/options/options.ts', out: 'options.js', format: 'iife' },
  { src: 'src/popup/popup.ts', out: 'popup.js', format: 'iife' },
];

/** @param {string} message */
function loud(message) {
  const bar = '!'.repeat(72);
  console.warn(`\n${bar}\n${message}\n${bar}\n`);
}

/** @param {string} dir */
function listFiles(dir) {
  return readdirSync(dir, { withFileTypes: true, recursive: true })
    .filter((d) => d.isFile())
    .map((d) => join(d.parentPath, d.name))
    .sort();
}

const pkg = JSON.parse(readFileSync(join(root, 'package.json'), 'utf8'));
const manifest = JSON.parse(readFileSync(join(root, 'manifest.json'), 'utf8'));
const target = esbuildTarget(manifest);
const missing = new Set(entries.map((e) => e.src).filter((src) => !existsSync(join(root, src))));
const wasmMissing = !existsSync(wasmSource);

const errors = buildErrors({
  version: pkg.version,
  missingEntries: [...missing],
  wasmMissing,
  zip: wantZip,
  allowStubs: flags.has('--allow-stubs'),
});
if (errors.length > 0) {
  for (const e of errors) console.error(e);
  process.exit(1);
}

rmSync(dist, { recursive: true, force: true });
mkdirSync(dist, { recursive: true });

for (const e of entries) {
  /** @type {import('esbuild').BuildOptions} */
  const options = {
    bundle: true,
    format: e.format,
    target,
    // tsconfig.json maps @go-tebanare/engine to the engine sources.
    tsconfig: join(root, 'tsconfig.json'),
    outfile: join(dist, e.out),
    charset: 'utf8',
    legalComments: 'none',
    logLevel: 'warning',
  };
  if (missing.has(e.src)) {
    loud(`WARNING: ${e.src} does not exist; dist/${e.out} is an empty stub.`);
    await build({ ...options, stdin: { contents: 'export {};', loader: 'ts', resolveDir: root, sourcefile: e.src } });
  } else {
    await build({ ...options, entryPoints: [join(root, e.src)] });
  }
}

cpSync(join(root, 'static'), dist, { recursive: true });
writeFileSync(join(dist, 'manifest.json'), `${JSON.stringify({ ...manifest, ...chromeVersion(pkg.version) }, null, 2)}\n`);

if (wasmMissing) {
  loud(
    `WARNING: ${relative(root, wasmSource)} is missing, so dist/ has no engine.wasm.\n` +
      'The extension loads but cannot analyze anything. Run "make wasm" at the repository root.',
  );
} else {
  copyFileSync(wasmSource, join(dist, 'engine.wasm'));
}

const files = listFiles(dist);
for (const f of files) console.log(`dist/${relative(dist, f).split(sep).join('/')}`);

if (wantZip) {
  const zipPath = join(root, 'dist.zip');
  const archive = zipFiles(files.map((f) => ({ name: relative(dist, f).split(sep).join('/'), data: readFileSync(f) })));
  writeFileSync(zipPath, archive);
  console.log(`wrote ${relative(root, zipPath)} (${archive.length} bytes, ${files.length} files)`);
}
