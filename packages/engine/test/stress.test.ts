// Checks that inputs at the nesting limits of plan 4.7 run in the TinyGo
// build without exhausting its stack, and that one step past a limit is
// skipped as too-deep. The limits are 200 levels of brackets, 1,000 else-if
// branches, and a syntax tree 1,500 levels deep.
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import type { CompiledRuleset, Engine } from '../src/index.js';
import { haveWasm, loadEngine } from './helpers.js';

const config = 'version: 1\npresets: [iferr]\n';

function brackets(depth: number): string {
  return `package p\n\nvar x = ${'('.repeat(depth)}1${')'.repeat(depth)}\n`;
}

function elseIfChain(n: number): string {
  let body = 'if x == 0 {\n}';
  for (let i = 1; i <= n; i++) body += ` else if x == ${i} {\n}`;
  return `package p\n\nfunc f(x int) {\n${body}\n}\n`;
}

function plusChain(terms: number): string {
  return `package p\n\nvar x = ${Array.from({ length: terms }, () => 'a').join(' + ')}\n`;
}

// Shapes that nest deeply without brackets or else-if chains. Each one
// passes the bracket and else-if limits and stops at the syntax tree limit.
const deepShapes: [string, string][] = [
  ['unary operators', `package p\nvar x = ${'-'.repeat(60000)}1\n`],
  ['pointer types', `package p\nvar x ${'*'.repeat(60000)}int\n`],
  ['slice types', `package p\nvar x ${'[]'.repeat(30000)}int\n`],
  ['receive operators', `package p\nvar x = ${'<-'.repeat(30000)}c\n`],
  ['channel types', `package p\nvar x ${'chan '.repeat(20000)}int\n`],
  ['selectors', `package p\nvar x = a${'.b'.repeat(30000)}\n`],
  ['calls', `package p\nvar x = f${'()'.repeat(30000)}\n`],
  ['index expressions', `package p\nvar x = f${'[0]'.repeat(30000)}\n`],
  ['func types', `package p\nvar x ${'func() '.repeat(20000)}\n`],
  ['map types', `package p\nvar x ${'map[int]'.repeat(20000)}int\n`],
  ['labels', `package p\nfunc f() {\n${Array.from({ length: 20000 }, (_, i) => `L${i}:`).join('\n')}\n}\n`],
];

describe.skipIf(!haveWasm)('nesting limits in wasm', () => {
  let engine: Engine;
  let rs: CompiledRuleset;
  beforeAll(async () => {
    engine = await loadEngine();
    rs = await engine.compile(config);
  });
  afterAll(() => engine?.dispose());

  const analyze = (src: string) => engine.analyzeChange(rs, { newPath: 'p.go', new: src });

  it.each([
    ['brackets at 200', brackets(200), ''],
    ['brackets at 201', brackets(201), 'too-deep'],
    ['else-if chain of 1000', elseIfChain(1000), ''],
    ['else-if chain of 1001', elseIfChain(1001), 'too-deep'],
    ['+ chain of 1400 terms', plusChain(1400), ''],
    ['+ chain of 1600 terms', plusChain(1600), 'too-deep'],
    ['brackets at 5000', brackets(5000), 'too-deep'],
    ['else-if chain of 5000', elseIfChain(5000), 'too-deep'],
    ['+ chain of 20000 terms', plusChain(20000), 'too-deep'],
    ...deepShapes.map(([name, src]): [string, string, string] => [name, src, 'too-deep']),
  ])('%s', async (_name, src, want) => {
    const res = await analyze(src);
    expect(res.skipped).toBe(want);
  });
});

describe.skipIf(!haveWasm)('syntax tree depth limit in wasm', () => {
  let engine: Engine;
  let rs: CompiledRuleset;
  beforeAll(async () => {
    engine = await loadEngine();
    rs = await engine.compile('version: 1\npresets: [getter, iferr]\n');
  });
  afterAll(() => engine?.dispose());

  // The file, the declaration, and the value spec take three levels, the
  // 1,496 BinaryExpr nodes of 1,497 terms the next ones, and the last
  // identifier level 1,500.
  it.each([
    [1497, ''],
    [1498, 'too-deep'],
  ])('+ chain of %i terms', async (terms, want) => {
    const res = await engine.analyzeChange(rs, { newPath: 'p.go', new: plusChain(terms) });
    expect(res.skipped).toBe(want);
    if (want !== '') {
      expect(res.diagnostics[0]?.message).toContain('more than 1500 levels deep');
    }
  });
});
