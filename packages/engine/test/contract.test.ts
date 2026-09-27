// Runs every Go golden case in testdata/analyze through the wasm engine and
// compares the result with want.json, so the TypeScript DTOs and the wasm
// build stay in step with the native Go results. The files in
// testdata/contract hold what internal/bridge returns natively for the
// other calls (see internal/bridge/contract_test.go).
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import {
  ConfigError,
  type Diagnostic,
  type Engine,
  type PresetInfo,
  type RuleInfo,
} from '../src/index.js';
import { haveWasm, loadEngine, repoRoot } from './helpers.js';

const casesDir = join(repoRoot, 'testdata', 'analyze');
const cases = existsSync(casesDir) ? readdirSync(casesDir).sort() : [];

function readOptional(dir: string, name: string): string | null {
  const p = join(dir, name);
  return existsSync(p) ? readFileSync(p, 'utf8') : null;
}

describe.skipIf(!haveWasm)('golden cases through engine.wasm', () => {
  let engine: Engine;
  beforeAll(async () => {
    engine = await loadEngine();
  });
  afterAll(() => engine?.dispose());

  it('finds the golden cases', () => {
    expect(cases.length).toBeGreaterThan(0);
  });

  it.each(cases)('%s', async (name) => {
    const dir = join(casesDir, name);
    const meta = JSON.parse(readOptional(dir, 'meta.json') ?? '{}') as {
      oldPath?: string;
      newPath?: string;
    };
    const oldSrc = readOptional(dir, 'old.go');
    const newSrc = readOptional(dir, 'new.go');
    const rs = await engine.compile(readFileSync(join(dir, 'config.yml'), 'utf8'));
    const got = await engine.analyzeChange(rs, {
      oldPath: oldSrc === null ? undefined : (meta.oldPath ?? 'x.go'),
      newPath: newSrc === null ? undefined : (meta.newPath ?? 'x.go'),
      old: oldSrc,
      new: newSrc,
    });
    const want: unknown = JSON.parse(readFileSync(join(dir, 'want.json'), 'utf8'));
    expect(got).toEqual(want);
  });
});

function contract<T>(name: string): T {
  return JSON.parse(readFileSync(join(repoRoot, 'testdata', 'contract', name), 'utf8')) as T;
}

interface CompileWant {
  diagnostics: Diagnostic[];
  rules: RuleInfo[];
  error?: string;
}

describe.skipIf(!haveWasm)('Go contract files through engine.wasm', () => {
  let engine: Engine;
  beforeAll(async () => {
    engine = await loadEngine();
  });
  afterAll(() => engine?.dispose());

  it('info.json', () => {
    const want = contract<{ apiVersion: number; configFileNames: string[] }>('info.json');
    expect({ apiVersion: engine.apiVersion, configFileNames: engine.configFileNames }).toEqual(want);
  });

  it('presets.json', async () => {
    expect(await engine.presets()).toEqual(contract<{ presets: PresetInfo[] }>('presets.json').presets);
  });

  const compiles = contract<{ cases: { name: string; yaml: string; want: CompileWant }[] }>('compile.json').cases;
  it.each(compiles)('compile.json: $name', async ({ yaml, want }) => {
    if (want.error === undefined) {
      const rs = await engine.compile(yaml);
      expect({ diagnostics: rs.diagnostics, rules: rs.rules }).toEqual({ diagnostics: want.diagnostics, rules: want.rules });
      return;
    }
    const err = await engine.compile(yaml).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ConfigError);
    expect((err as ConfigError).message).toBe(want.error);
    expect((err as ConfigError).diagnostics).toEqual(want.diagnostics);
  });
});
