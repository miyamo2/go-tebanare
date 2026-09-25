import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { buildErrors, chromeVersion, esbuildTarget } from '../../scripts/build-lib.mjs';

const pkgRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const readJson = (name: string) => JSON.parse(readFileSync(join(pkgRoot, name), 'utf8')) as Record<string, unknown>;

describe('chromeVersion', () => {
  it.each([
    ['0.1.0', { version: '0.1.0' }],
    ['1.2.3.4', { version: '1.2.3.4' }],
    ['0.2.0-rc.1', { version: '0.2.0', version_name: '0.2.0-rc.1' }],
    ['0.2.0+build.5', { version: '0.2.0', version_name: '0.2.0+build.5' }],
  ])('maps %s', (v, want) => {
    expect(chromeVersion(v)).toEqual(want);
  });

  it.each(['', 'v1.0.0', '1.2.3.4.5', '01.0.0', '1.70000.0', '0.0.0', '1.0.0rc1'])('rejects %j', (v) => {
    expect(() => chromeVersion(v)).toThrow('does not map to a Chrome manifest version');
  });

  it('accepts the package version', () => {
    expect(() => chromeVersion(String(readJson('package.json')['version']))).not.toThrow();
  });
});

describe('esbuildTarget', () => {
  it('follows minimum_chrome_version', () => {
    expect(esbuildTarget({ minimum_chrome_version: '120' })).toBe('chrome120');
    expect(esbuildTarget(readJson('manifest.json'))).toMatch(/^chrome\d+$/);
  });

  it.each([{}, { minimum_chrome_version: 120 }, { minimum_chrome_version: '120.0.1' }])('rejects %j', (m) => {
    expect(() => esbuildTarget(m)).toThrow('minimum_chrome_version');
  });
});

describe('buildErrors', () => {
  const ok = { version: '0.1.0', missingEntries: [], wasmMissing: false, zip: false, allowStubs: false };
  const missingEntries = ['src/content/index.ts', 'src/popup/popup.ts'];

  it('accepts a complete build with or without --zip', () => {
    expect(buildErrors(ok)).toEqual([]);
    expect(buildErrors({ ...ok, zip: true })).toEqual([]);
  });

  it('stops on a missing entry point unless --allow-stubs is given', () => {
    expect(buildErrors({ ...ok, missingEntries })).toEqual([
      'src/content/index.ts does not exist. Pass --allow-stubs to build an empty stub for it.',
      'src/popup/popup.ts does not exist. Pass --allow-stubs to build an empty stub for it.',
    ]);
    expect(buildErrors({ ...ok, missingEntries, allowStubs: true })).toEqual([]);
  });

  it('refuses stubs in dist.zip even with --allow-stubs', () => {
    expect(buildErrors({ ...ok, missingEntries, zip: true, allowStubs: true })).toEqual([
      'src/content/index.ts does not exist. dist.zip needs every entry point.',
      'src/popup/popup.ts does not exist. dist.zip needs every entry point.',
    ]);
  });

  it('refuses dist.zip without engine.wasm, and warns only without --zip', () => {
    expect(buildErrors({ ...ok, wasmMissing: true, zip: true })).toEqual([
      'engine.wasm is missing. Run "make wasm" at the repository root before --zip.',
    ]);
    expect(buildErrors({ ...ok, wasmMissing: true })).toEqual([]);
  });

  it('refuses dist.zip for a prerelease version', () => {
    expect(buildErrors({ ...ok, version: '0.2.0-rc.1', zip: true })).toEqual([
      'package.json version 0.2.0-rc.1 has a suffix. dist.zip needs a plain version such as 0.2.0.',
    ]);
    expect(buildErrors({ ...ok, version: '0.2.0-rc.1' })).toEqual([]);
  });

  it('stops on a version Chrome cannot use', () => {
    expect(buildErrors({ ...ok, version: '1.0' })).toEqual([]);
    expect(buildErrors({ ...ok, version: '0.0.0' })).toEqual([
      'package.json version "0.0.0" does not map to a Chrome manifest version.',
    ]);
  });
});
