// Checks and conversions used by build.mjs, kept free of I/O so that
// test/shared/build-lib.test.ts can run them.

/**
 * chromeVersion maps a package.json version to the manifest's version and
 * version_name. Chrome accepts one to four dot-separated integers from 0 to
 * 65535, without leading zeros and not all zero. A suffix such as "-rc.1"
 * moves into version_name.
 * @param {string} v
 * @returns {{ version: string; version_name?: string }}
 */
export function chromeVersion(v) {
  const r = parseVersion(v);
  if (!r) throw new Error(`package.json version ${JSON.stringify(v)} does not map to a Chrome manifest version`);
  return r;
}

/**
 * @param {string} v
 * @returns {{ version: string; version_name?: string } | null}
 */
function parseVersion(v) {
  const m = /^(\d+(?:\.\d+){0,3})([-+].*)?$/.exec(v);
  const numeric = m?.[1];
  if (numeric === undefined) return null;
  const parts = numeric.split('.');
  if (parts.some((p) => Number(p) > 65535 || (p !== '0' && p.startsWith('0')))) return null;
  if (parts.every((p) => p === '0')) return null;
  return m?.[2] === undefined ? { version: v } : { version: numeric, version_name: v };
}

/**
 * esbuildTarget returns the esbuild target for the manifest's
 * minimum_chrome_version, so bundles use only syntax that every supported
 * Chrome parses.
 * @param {{ minimum_chrome_version?: unknown }} manifest
 * @returns {string}
 */
export function esbuildTarget(manifest) {
  const v = manifest.minimum_chrome_version;
  if (typeof v !== 'string' || !/^[1-9]\d*$/.test(v)) {
    throw new Error('manifest.json needs minimum_chrome_version as a major version, for example "120"');
  }
  return `chrome${v}`;
}

/**
 * @typedef {object} BuildInput
 * @property {string} version package.json version
 * @property {readonly string[]} missingEntries entry point sources that do not exist
 * @property {boolean} wasmMissing whether ../engine/wasm/engine.wasm is missing
 * @property {boolean} zip whether --zip was given
 * @property {boolean} allowStubs whether --allow-stubs was given
 */

/**
 * buildErrors returns the reasons the build must stop before it writes
 * anything. A missing entry point stops every build unless --allow-stubs
 * is given. --zip also needs engine.wasm and a version without a suffix:
 * the Chrome Web Store needs a higher manifest version for every upload,
 * so a zip of 0.2.0-rc.1 (manifest version 0.2.0) would block the 0.2.0
 * release.
 * @param {BuildInput} input
 * @returns {string[]}
 */
export function buildErrors({ version, missingEntries, wasmMissing, zip, allowStubs }) {
  const errors = [];
  const mapped = parseVersion(version);
  if (!mapped) errors.push(`package.json version ${JSON.stringify(version)} does not map to a Chrome manifest version.`);
  if (zip) {
    for (const src of missingEntries) errors.push(`${src} does not exist. dist.zip needs every entry point.`);
    if (wasmMissing) errors.push('engine.wasm is missing. Run "make wasm" at the repository root before --zip.');
    if (mapped?.version_name !== undefined) {
      errors.push(`package.json version ${version} has a suffix. dist.zip needs a plain version such as 0.2.0.`);
    }
  } else if (!allowStubs) {
    for (const src of missingEntries) errors.push(`${src} does not exist. Pass --allow-stubs to build an empty stub for it.`);
  }
  return errors;
}
