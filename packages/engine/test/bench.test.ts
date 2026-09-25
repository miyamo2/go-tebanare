// Measures the plan's performance budget (plan 8 and 10): analyzing a
// 5,000-line file in the wasm engine takes less than 200 ms. It also times
// a 20,000-line file, with a loose bound, so that slowdowns on large files
// show up. The sources have the same shape as internal/analyzer's
// BenchmarkAnalyze5k and BenchmarkAnalyze20k.
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import type { CompiledRuleset, Engine } from '../src/index.js';
import { haveWasm, loadEngine } from './helpers.js';

const config = 'version: 1\npresets: [getter, iferr]\n';

function generate(lines: number, every: number): string {
  let s = 'package p\n\nimport "context"\n\n';
  for (let i = 0; s.split('\n').length - 1 < lines; i++) {
    s += `// Name${i} returns the name.\nfunc (u *User) Name${i}() string { return u.name${i} }\n\n`;
    s += `func (s *Service) Do${i}(ctx context.Context, id string) error {\n`;
    s += '\tlog.Debug("do", "id", id)\n';
    s += '\tv, err := s.repo.Find(ctx, id)\n\tif err != nil {\n\t\treturn err\n\t}\n';
    if (i % every === 0) s += '\tlog.Printf("changed")\n';
    s += `\tif v.n > ${i} {\n\t\tlog.Printf("big %d", v.n)\n\t}\n\treturn s.save(ctx, v)\n}\n\n`;
  }
  return s;
}

describe.skipIf(!haveWasm)('performance budget', () => {
  let engine: Engine;
  let rs: CompiledRuleset;
  beforeAll(async () => {
    engine = await loadEngine();
    rs = await engine.compile(config);
  });
  afterAll(() => engine?.dispose());

  /** Returns the median time of five analyses of src, after one to warm up. */
  async function median(src: string): Promise<number> {
    // The warm-up run keeps the first instantiation out of the timing.
    await engine.analyzeChange(rs, { newPath: 'x.go', new: src });
    const runs: number[] = [];
    for (let i = 0; i < 5; i++) {
      const start = performance.now();
      const res = await engine.analyzeChange(rs, { newPath: 'x.go', new: src });
      runs.push(performance.now() - start);
      expect(res.skipped).toBe('');
      expect(res.new.length).toBeGreaterThan(0);
    }
    runs.sort((a, b) => a - b);
    return runs[2]!;
  }

  it('analyzes a 5,000-line file in under 200 ms', async () => {
    const ms = await median(generate(5000, 5));
    console.log(`5,000 lines: median ${ms.toFixed(1)} ms over 5 runs`);
    expect(ms).toBeLessThan(200);
  });

  it('analyzes a 20,000-line file in under 1 s', async () => {
    const ms = await median(generate(20000, 5));
    console.log(`20,000 lines: median ${ms.toFixed(1)} ms over 5 runs`);
    expect(ms).toBeLessThan(1000);
  });
});
