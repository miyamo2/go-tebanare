// Runs the engine's compile against modules built by fake-wasm.ts that
// trap inside compile, before and after the YAML parse.
import { describe, expect, it } from 'vitest';
import { yamlDone } from '../src/abi.js';
import { yamlSyntaxMessage } from '../src/engine.js';
import { ConfigError, createEngine, type RecreateReason } from '../src/index.js';
import { compileFake, fakeModule } from './fake-wasm.js';

describe('compile traps', () => {
  it('reports a trap during the YAML parse as config-syntax with the line', async () => {
    const reasons: RecreateReason[] = [];
    // The parser read 13 bytes: the tab that starts line 2 and one more.
    const m = { ...fakeModule(0x1000, 1, 13), compile: { trap: true } as const };
    const engine = await createEngine(await compileFake(m), { onRecreate: (r) => reasons.push(r) });
    const err = await engine.compile('version: 1\n\tpresets: []\n').catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ConfigError);
    expect((err as ConfigError).message).toBe(`2: ${yamlSyntaxMessage}`);
    expect((err as ConfigError).diagnostics).toEqual([
      { severity: 'error', code: 'config-syntax', message: yamlSyntaxMessage, line: 2 },
    ]);
    expect(reasons).toEqual(['crash']);
  });

  it('passes on a trap after the YAML parse', async () => {
    const reasons: RecreateReason[] = [];
    const m = { ...fakeModule(0x1000, 1, yamlDone), compile: { trap: true } as const };
    const engine = await createEngine(await compileFake(m), { onRecreate: (r) => reasons.push(r) });
    const err = await engine.compile('version: 1\n').catch((e: unknown) => e);
    expect(err).toBeInstanceOf(WebAssembly.RuntimeError);
    expect(reasons).toEqual(['crash']);
  });
});
