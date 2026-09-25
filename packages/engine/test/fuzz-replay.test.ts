// Replays fuzz inputs through engine.wasm (plan 8). TinyGo's wasm cannot
// recover from a panic, so an input that panics only there would stop the
// engine. The inputs come from testvectors/fuzz, which the Go tests keep in
// step with the fuzz seeds and testdata/fuzz corpora, and from the
// directory in GOTEBANARE_FUZZ_VECTORS when set, which
// internal/tools/fuzzcorpus fills from the corpus in GOCACHE.
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { yamlSyntaxMessage } from '../src/engine.js';
import { ConfigError, type Engine, type RecreateReason } from '../src/index.js';
import { haveWasm, loadEngine, repoRoot } from './helpers.js';

interface Input {
  text?: string;
  base64?: string;
  syntax?: boolean;
}

interface VectorFile {
  source?: string;
  inputs: Input[];
}

// Every kind of span that presets produce: func matches with and without
// doc comments, whole statements, and the explicit span of iferr with
// init: fold-body.
const analyzeConfig = `version: 1
presets:
  - getter:
      include_doc: false
  - noop:
      include_functions: true
  - iferr:
      init: fold-body
`;

// Analyzed with every fuzzed config that compiles.
const sampleSource = `package p

// Name returns the name.
func (u *User) Name() string { return u.name }

func Do(ctx context.Context) error {
	log.Debug("x")
	v, err := find(ctx)
	if err != nil {
		return err
	}
	return save(v)
}
`;

const checkedIn = join(repoRoot, 'testvectors', 'fuzz');
const extra = process.env.GOTEBANARE_FUZZ_VECTORS;

function readVectors(dir: string): [string, VectorFile][] {
  if (!existsSync(dir)) return [];
  return readdirSync(dir)
    .filter((f) => f.endsWith('.json'))
    .sort()
    .map((f) => [f.replace(/\.json$/, ''), JSON.parse(readFileSync(join(dir, f), 'utf8')) as VectorFile]);
}

const files: [string, string, VectorFile][] = [
  ...readVectors(checkedIn).map(([n, f]): [string, string, VectorFile] => [`testvectors/fuzz/${n}`, n, f]),
  ...(extra ? readVectors(extra).map(([n, f]): [string, string, VectorFile] => [`${extra}/${n}`, n, f]) : []),
];
const checkedInSources = new Map(readVectors(checkedIn).map(([n, f]) => [n, f.source]));

function bytesOf(input: Input): Uint8Array {
  if (input.text !== undefined) return new TextEncoder().encode(input.text);
  return new Uint8Array(Buffer.from(input.base64 ?? '', 'base64'));
}

function label(i: number, input: Input): string {
  const s = input.text ?? `base64:${input.base64 ?? ''}`;
  return `input ${i} ${JSON.stringify(s.length > 80 ? `${s.slice(0, 80)}...` : s)}`;
}

describe.skipIf(!haveWasm)('fuzz inputs through engine.wasm', () => {
  let engine: Engine;
  const reasons: RecreateReason[] = [];
  beforeAll(async () => {
    engine = await loadEngine({ onRecreate: (r) => reasons.push(r) });
  });
  afterAll(() => engine?.dispose());

  it('finds the checked-in vectors', () => {
    expect(files.map(([, n]) => n)).toEqual(expect.arrayContaining(['analyze', 'config']));
  });

  /** Analyzes src in every shape of change and returns the problems found. */
  async function analyzeAll(where: string, rsYaml: string, src: Uint8Array): Promise<string[]> {
    const problems: string[] = [];
    const rs = await engine.compile(rsYaml);
    const crashes = reasons.filter((r) => r === 'crash').length;
    for (const change of [
      { newPath: 'x.go', new: src },
      { oldPath: 'x.go', old: src },
      { oldPath: 'x.go', newPath: 'x.go', old: src, new: src },
    ]) {
      const res = await engine.analyzeChange(rs, change);
      if (res.skipped === 'engine-error') problems.push(`${where}: analyzeChange crashed: ${res.diagnostics[0]?.message ?? ''}`);
    }
    if (reasons.filter((r) => r === 'crash').length !== crashes) problems.push(`${where}: the instance crashed`);
    return problems;
  }

  it.each(files)('%s', async (_where, name, file) => {
    const problems: string[] = [];
    for (const [i, input] of file.inputs.entries()) {
      const where = label(i, input);
      if (name === 'analyze') {
        problems.push(...(await analyzeAll(where, analyzeConfig, bytesOf(input))));
        continue;
      }
      // A config: it compiles, or it is rejected with a ConfigError.
      const yaml = input.text ?? new TextDecoder().decode(bytesOf(input));
      const crashes = reasons.filter((r) => r === 'crash').length;
      let trapped = false;
      try {
        await engine.compile(yaml);
      } catch (e) {
        if (!(e instanceof ConfigError)) {
          problems.push(`${where}: compile crashed: ${String(e)}`);
          continue;
        }
        trapped = e.diagnostics[0]?.message === yamlSyntaxMessage;
      }
      // A trap means a YAML syntax error, and only the parser may trap.
      if (input.text !== undefined && trapped !== (input.syntax ?? false)) {
        problems.push(`${where}: YAML syntax error in wasm ${String(trapped)}, natively ${String(input.syntax ?? false)}`);
      }
      if (reasons.filter((r) => r === 'crash').length !== crashes + (trapped ? 1 : 0)) {
        problems.push(`${where}: the instance crashed outside the YAML parser`);
      }
      if (!trapped) {
        const src = file.source ?? checkedInSources.get(name) ?? sampleSource;
        const rs = await engine.compile(yaml).catch(() => null);
        if (rs) problems.push(...(await analyzeAll(where, yaml, new TextEncoder().encode(src))));
      }
    }
    expect(problems).toEqual([]);
  });
});
