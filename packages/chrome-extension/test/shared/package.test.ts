// Checks on package configuration that no module test covers.

import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const pkgRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const read = (name: string) => readFileSync(join(pkgRoot, name), 'utf8');

interface TsConfig { compilerOptions?: { types?: string[] }; include?: string[] }

describe('tsconfig', () => {
  it('typechecks src without Node types, and tests with them', () => {
    const src = JSON.parse(read('tsconfig.json')) as TsConfig;
    expect(src.compilerOptions?.types).toEqual(['chrome']);
    expect(src.include).toEqual(['src']);
    const test = JSON.parse(read('test/tsconfig.json')) as TsConfig;
    expect(test.compilerOptions?.types).toContain('node');
    expect(JSON.parse(read('package.json'))).toMatchObject({
      scripts: { typecheck: 'tsc --noEmit && tsc --noEmit -p test' },
    });
  });
});

describe('manifest.json', () => {
  it('requires a Chrome that supports wasm-unsafe-eval', () => {
    const manifest = JSON.parse(read('manifest.json')) as Record<string, unknown>;
    expect(JSON.stringify(manifest['content_security_policy'])).toContain("'wasm-unsafe-eval'");
    // Chrome 103 added 'wasm-unsafe-eval'.
    expect(Number(manifest['minimum_chrome_version'])).toBeGreaterThanOrEqual(103);
  });
});
