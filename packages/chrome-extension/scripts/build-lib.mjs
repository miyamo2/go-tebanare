// Checks and conversions used by build.mjs, release-version.mjs, and
// sync-manifest-version.mjs, kept free of I/O so that
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
 * is given. --zip also needs engine.wasm and a version that can be released
 * (see parseReleaseVersion): a plain version such as 0.2.0, or a prerelease
 * such as 0.2.0-rc.1, whose zip goes to the GitHub release only and never to
 * the Chrome Web Store.
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
    if (mapped && !parseReleaseVersion(version)) {
      errors.push(`package.json version ${version} has a suffix other than a prerelease. dist.zip needs a version such as 0.2.0 or 0.2.0-rc.1.`);
    }
  } else if (!allowStubs) {
    for (const src of missingEntries) errors.push(`${src} does not exist. Pass --allow-stubs to build an empty stub for it.`);
  }
  return errors;
}

/**
 * @typedef {object} ReleaseVersion
 * @property {string} numeric the manifest version, one to four integers
 * @property {string[] | undefined} prerelease the dot-separated identifiers
 *   after "-", or undefined for a plain version
 */

/**
 * parseReleaseVersion splits a version that can be released: a Chrome
 * manifest version, optionally followed by a semver prerelease such as
 * "-rc.1" (identifiers of [0-9A-Za-z-], numeric ones without leading zeros).
 * Build metadata ("+...") is refused. It returns null for anything else.
 * @param {string} v
 * @returns {ReleaseVersion | null}
 */
export function parseReleaseVersion(v) {
  const m = /^([^-+]+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$/.exec(v);
  const numeric = m?.[1];
  if (numeric === undefined || parseVersion(numeric) === null) return null;
  const prerelease = m?.[2]?.split('.');
  if (prerelease?.some((id) => /^0\d/.test(id) && /^\d+$/.test(id))) return null;
  return { numeric, prerelease };
}

/**
 * compareReleaseVersions compares two versions accepted by
 * parseReleaseVersion in semver order: first the numeric parts, then a
 * prerelease below the plain version, then the prerelease identifiers.
 * @param {string} a
 * @param {string} b
 * @returns {number} negative, zero, or positive as a is lower, equal, or higher
 */
export function compareReleaseVersions(a, b) {
  const pa = parseReleaseVersion(a);
  const pb = parseReleaseVersion(b);
  if (!pa || !pb) throw new Error(`cannot compare ${JSON.stringify(a)} with ${JSON.stringify(b)}`);
  const d = compareVersions(pa.numeric, pb.numeric);
  if (d !== 0) return d;
  if (pa.prerelease === undefined || pb.prerelease === undefined) {
    return (pa.prerelease === undefined ? 1 : 0) - (pb.prerelease === undefined ? 1 : 0);
  }
  for (let i = 0; i < Math.min(pa.prerelease.length, pb.prerelease.length); i++) {
    const x = pa.prerelease[i] ?? '';
    const y = pb.prerelease[i] ?? '';
    if (x === y) continue;
    const nx = /^\d+$/.test(x);
    const ny = /^\d+$/.test(y);
    if (nx && ny) return Number(x) - Number(y);
    // A numeric identifier sorts below an alphanumeric one.
    if (nx !== ny) return nx ? -1 : 1;
    return x < y ? -1 : 1;
  }
  return pa.prerelease.length - pb.prerelease.length;
}

/**
 * releaseVersionErrors returns the reasons next cannot be released after
 * previous, the highest earlier release of any kind (undefined before the
 * first). The Chrome Web Store needs a plain version, one to four integers,
 * and a higher one for every upload. A prerelease such as 0.2.0-rc.1 skips
 * the store; it must still be higher than every earlier release, so its
 * numeric part stays above the last store upload.
 * @param {string | undefined} previous
 * @param {string} next
 * @returns {string[]}
 */
export function releaseVersionErrors(previous, next) {
  if (!parseVersion(next)) return [`version ${JSON.stringify(next)} does not map to a Chrome manifest version.`];
  if (!parseReleaseVersion(next)) {
    return [`version ${next} has a suffix other than a prerelease. A release needs a version such as 0.2.0 or 0.2.0-rc.1.`];
  }
  if (previous !== undefined && compareReleaseVersions(next, previous) <= 0) {
    return [`version ${next} is not higher than the last release ${previous}.`];
  }
  return [];
}

/**
 * @typedef {object} ReleasePlan
 * @property {string[]} errors the reasons next cannot be released
 * @property {string} version next
 * @property {string} manifestVersion the version manifest.json must hold
 * @property {boolean} prerelease whether next is a prerelease, which is
 *   marked so on GitHub and not uploaded to the Chrome Web Store
 * @property {string | undefined} previous the highest earlier release
 * @property {string | undefined} notesStart the release the notes start
 *   after: the highest earlier release for a prerelease, and the highest
 *   earlier plain release for a plain version
 */

/**
 * releasePlan checks next against the versions of the earlier releases.
 * Entries of released that parseReleaseVersion refuses, or that equal next,
 * are ignored.
 * @param {string} next
 * @param {readonly string[]} released
 * @returns {ReleasePlan}
 */
export function releasePlan(next, released) {
  const earlier = released.filter((v) => v !== next && parseReleaseVersion(v) !== null);
  /** @param {readonly string[]} vs */
  const highest = (vs) => vs.reduce((/** @type {string | undefined} */ max, v) => (max === undefined || compareReleaseVersions(v, max) > 0 ? v : max), undefined);
  const previous = highest(earlier);
  const parsed = parseReleaseVersion(next);
  const prerelease = parsed?.prerelease !== undefined;
  return {
    errors: releaseVersionErrors(previous, next),
    version: next,
    manifestVersion: parsed?.numeric ?? next,
    prerelease,
    previous,
    notesStart: prerelease ? previous : highest(earlier.filter((v) => parseReleaseVersion(v)?.prerelease === undefined)),
  };
}

/**
 * compareVersions compares two dot-separated versions of integers, reading a
 * missing part as 0.
 * @param {string} a
 * @param {string} b
 * @returns {number} negative, zero, or positive as a is lower, equal, or higher
 */
export function compareVersions(a, b) {
  const pa = a.split('.').map(Number);
  const pb = b.split('.').map(Number);
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const d = (pa[i] ?? 0) - (pb[i] ?? 0);
    if (d !== 0) return d;
  }
  return 0;
}

/**
 * withVersion returns the JSON text with the first string-valued "version"
 * member in the text set to version, keeping the rest of the text as it is.
 * "manifest_version" does not match.
 * @param {string} text
 * @param {string} version
 * @returns {string}
 */
export function withVersion(text, version) {
  const re = /("version"\s*:\s*)"[^"]*"/;
  if (!re.test(text)) throw new Error('no "version" member');
  return text.replace(re, (_, key) => `${key}${JSON.stringify(version)}`);
}
