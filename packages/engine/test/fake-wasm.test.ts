// Runs WasmInstance against modules built by fake-wasm.ts, which place
// buffers where a real engine.wasm cannot be made to put them.
import { describe, expect, it } from 'vitest';
import { WasmInstance } from '../src/abi.js';
import { compileFake, fakeCompiled, fakeInfo, fakeModule, pageSize } from './fake-wasm.js';

describe('WasmInstance', () => {
  it('reads and writes buffers at addresses past 2 GiB', async () => {
    const base = 0x80000000;
    const inst = new WasmInstance(await compileFake(fakeModule(base, base / pageSize + 1, 7)));
    expect(inst.memoryBytes).toBeGreaterThan(2 ** 31);
    expect(inst.info()).toEqual(fakeInfo);
    expect(inst.compile(new TextEncoder().encode('version: 1\n'))).toEqual(fakeCompiled);
    expect(inst.yamlProgress()).toBe(7);
  });
});
