// Compares the wasm engine with the native Go build on a corpus (plan 8).
// It runs only when GOTEBANARE_PARITY names the output of
// `go run ./internal/tools/corpusdump -config testdata/parity/config.yml -root <dir> [-pairs]`
// and GOTEBANARE_PARITY_ROOT names the same <dir>. `make parity` runs it.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import type { ChangeResult } from '../src/index.js';
import { haveWasm, loadEngine, repoRoot } from './helpers.js';

const dumpPath = process.env.GOTEBANARE_PARITY;
const corpusRoot = process.env.GOTEBANARE_PARITY_ROOT;

describe.skipIf(!haveWasm || !dumpPath || !corpusRoot)('parity with the native build', () => {
  it('returns the same result for every corpus file', async () => {
    const dump = JSON.parse(readFileSync(dumpPath!, 'utf8')) as {
      files: Record<string, ChangeResult>;
      // Set by -pairs: the file whose content is the old side of each change.
      previous?: Record<string, string>;
    };
    const config = readFileSync(join(repoRoot, 'testdata', 'parity', 'config.yml'), 'utf8');
    const engine = await loadEngine();
    try {
      const rs = await engine.compile(config);
      const paths = Object.keys(dump.files);
      expect(paths.length).toBeGreaterThan(0);
      const mismatches: string[] = [];
      for (const path of paths) {
        const prev = dump.previous?.[path];
        const got = await engine.analyzeChange(rs, {
          oldPath: prev === undefined ? undefined : path,
          newPath: path,
          old: prev === undefined ? undefined : readFileSync(join(corpusRoot!, prev)),
          new: readFileSync(join(corpusRoot!, path)),
        });
        if (JSON.stringify(got) !== JSON.stringify(dump.files[path])) mismatches.push(path);
      }
      expect(mismatches).toEqual([]);
    } finally {
      engine.dispose();
    }
  }, 30 * 60_000);
});
