import { existsSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { createEngine, type Engine, type EngineOptions } from '../src/index.js';

export const repoRoot = fileURLToPath(new URL('../../../', import.meta.url));
export const wasmPath = fileURLToPath(new URL('../wasm/engine.wasm', import.meta.url));

/** True when `make wasm` has produced engine.wasm. */
export const haveWasm = existsSync(wasmPath);

export function loadEngine(options?: EngineOptions): Promise<Engine> {
  return createEngine(new Uint8Array(readFileSync(wasmPath)), options);
}
