// Low-level access to one instance of engine.wasm. The exports and the
// memory protocol are defined by cmd/gotebanare-wasm: inputs are copied into
// buffers from alloc, every export that returns data returns a pointer to a
// JSON buffer of result_len() bytes, and the caller frees every buffer.
//
// Wasm i32 results reach JavaScript as signed numbers, so every pointer and
// length an export returns goes through `>>> 0`. Without that, an address
// past 2 GiB would be negative.

interface Exports {
  memory: WebAssembly.Memory;
  _initialize?: () => void;
  alloc(size: number): number;
  free(ptr: number): void;
  result_len(): number;
  info(): number;
  compile(ptr: number, len: number): number;
  analyze_change(
    handle: number,
    metaPtr: number,
    metaLen: number,
    oldPtr: number,
    oldLen: number,
    newPtr: number,
    newLen: number,
  ): number;
  release(handle: number): void;
  presets(): number;
  yaml_progress_ptr(): number;
}

/** Export names that engine.wasm must provide. */
export const requiredExports = [
  'memory',
  'alloc',
  'free',
  'result_len',
  'info',
  'compile',
  'analyze_change',
  'release',
  'presets',
  'yaml_progress_ptr',
] as const;

/** The yaml_progress_ptr counter after the config parsed as YAML without an error. */
export const yamlDone = 0xffffffff;

/** A pointer outside linear memory. Reading from it traps. */
const outOfBounds = 0xfffffff0;

const decoder = new TextDecoder();

/** One instantiated engine. Any exception from a method means the instance is unusable. */
export class WasmInstance {
  readonly #x: Exports;
  readonly #yamlProgressPtr: number;
  #faultNext = false;

  constructor(module: WebAssembly.Module) {
    const instance = new WebAssembly.Instance(module, {});
    const x = instance.exports as unknown as Exports;
    for (const name of requiredExports) {
      if (!(name in x)) throw new Error(`engine.wasm does not export ${name}`);
    }
    x._initialize?.();
    this.#x = x;
    this.#yamlProgressPtr = x.yaml_progress_ptr() >>> 0;
  }

  /** Current size of the linear memory in bytes. */
  get memoryBytes(): number {
    return this.#x.memory.buffer.byteLength;
  }

  /**
   * Returns how many bytes of the config the YAML parser had read during
   * the last compile call, or yamlDone when the config parsed as YAML. It
   * reads memory only, so it works after a trap.
   */
  yamlProgress(): number {
    return new DataView(this.#x.memory.buffer).getUint32(this.#yamlProgressPtr, true);
  }

  info(): unknown {
    return this.#call([], () => this.#x.info());
  }

  compile(yaml: Uint8Array): unknown {
    return this.#call([yaml], ([p]) => this.#x.compile(p!, yaml.length));
  }

  analyzeChange(handle: number, meta: Uint8Array, oldSrc: Uint8Array, newSrc: Uint8Array): unknown {
    return this.#call([meta, oldSrc, newSrc], ([m, o, n]) =>
      this.#x.analyze_change(handle, m!, meta.length, o!, oldSrc.length, n!, newSrc.length),
    );
  }

  release(handle: number): void {
    this.#x.release(handle);
  }

  presets(): unknown {
    return this.#call([], () => this.#x.presets());
  }

  /** Calls compile with a pointer outside linear memory. Tests use it to force a trap. */
  trapForTesting(): void {
    this.#x.compile(outOfBounds, 64);
  }

  /**
   * Makes the next call that passes input to an export pass a pointer
   * outside linear memory as its first input, so that the export traps.
   * For tests only.
   */
  faultNextCallForTesting(): void {
    this.#faultNext = true;
  }

  #call(inputs: Uint8Array[], fn: (ptrs: number[]) => number): unknown {
    const ptrs: number[] = [];
    try {
      for (const bytes of inputs) {
        const ptr = this.#x.alloc(bytes.length) >>> 0;
        // Read memory.buffer after alloc: alloc may grow and detach the old buffer.
        new Uint8Array(this.#x.memory.buffer, ptr, bytes.length).set(bytes);
        ptrs.push(ptr);
      }
      const args = [...ptrs];
      if (this.#faultNext && args.length > 0) {
        this.#faultNext = false;
        args[0] = outOfBounds;
      }
      const out = fn(args) >>> 0;
      const len = this.#x.result_len() >>> 0;
      ptrs.push(out);
      const text = decoder.decode(new Uint8Array(this.#x.memory.buffer, out, len));
      return JSON.parse(text);
    } finally {
      for (const ptr of ptrs) {
        try {
          this.#x.free(ptr);
        } catch {
          // The instance is already broken; the caller discards it.
        }
      }
    }
  }
}
