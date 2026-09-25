// Builds small wasm modules byte by byte. Tests use them for behavior that
// engine.wasm never shows on purpose: addresses past 2 GiB, or a trap at a
// chosen point. Every module has the exports that WasmInstance requires.

/** Memory page size of wasm. */
export const pageSize = 64 * 1024;

/** What one export does. */
export type Behavior =
  | { returns: number } // returns this address (or nothing for a void export)
  | { trap: true }; // executes unreachable

export interface FakeModule {
  /** Initial memory size in pages. */
  pages: number;
  /** Bytes placed in memory at start-up, keyed by address. */
  data: Map<number, Uint8Array>;
  /** The value that result_len returns. */
  resultLen: number;
  alloc: Behavior;
  info: Behavior;
  compile: Behavior;
  analyzeChange: Behavior;
  presets: Behavior;
  yamlProgressPtr: Behavior;
}

const encoder = new TextEncoder();

/** Encodes JSON padded with spaces to n bytes, so one result_len fits every result. */
export function jsonBytes(value: unknown, n: number): Uint8Array {
  const b = encoder.encode(JSON.stringify(value));
  if (b.length > n) throw new Error(`JSON of ${b.length} bytes does not fit in ${n}`);
  const out = new Uint8Array(n).fill(0x20);
  out.set(b);
  return out;
}

/** Encodes a little-endian uint32. */
export function u32Bytes(v: number): Uint8Array {
  const out = new Uint8Array(4);
  new DataView(out.buffer).setUint32(0, v, true);
  return out;
}

function uleb(n: number): number[] {
  const out: number[] = [];
  do {
    let b = n & 0x7f;
    n = Math.floor(n / 128);
    if (n !== 0) b |= 0x80;
    out.push(b);
  } while (n !== 0);
  return out;
}

function sleb32(n: number): number[] {
  let v = n | 0;
  const out: number[] = [];
  for (;;) {
    const b = v & 0x7f;
    v >>= 7;
    if ((v === 0 && (b & 0x40) === 0) || (v === -1 && (b & 0x40) !== 0)) {
      out.push(b);
      return out;
    }
    out.push(b | 0x80);
  }
}

function vec(items: number[][]): number[] {
  return [...uleb(items.length), ...items.flat()];
}

function name(s: string): number[] {
  const b = [...encoder.encode(s)];
  return [...uleb(b.length), ...b];
}

function section(id: number, content: number[]): number[] {
  return [id, ...uleb(content.length), ...content];
}

const i32 = 0x7f;

/** Builds the module described by m. */
export function buildFakeModule(m: FakeModule): Uint8Array {
  // name, parameter count, whether it returns an i32, behavior
  const funcs: [string, number, boolean, Behavior][] = [
    ['alloc', 1, true, m.alloc],
    ['free', 1, false, { returns: 0 }],
    ['result_len', 0, true, { returns: m.resultLen }],
    ['info', 0, true, m.info],
    ['compile', 2, true, m.compile],
    ['analyze_change', 7, true, m.analyzeChange],
    ['release', 1, false, { returns: 0 }],
    ['presets', 0, true, m.presets],
    ['yaml_progress_ptr', 0, true, m.yamlProgressPtr],
  ];
  const types = funcs.map(([, params, result]) => [
    0x60,
    ...vec(Array.from({ length: params }, () => [i32])),
    ...vec(result ? [[i32]] : []),
  ]);
  const bodies = funcs.map(([, , result, b]) => {
    let code: number[];
    if ('trap' in b) code = [0x00];
    else if (result) code = [0x41, ...sleb32(b.returns)];
    else code = [];
    const body = [0x00, ...code, 0x0b];
    return [...uleb(body.length), ...body];
  });
  const exports = [
    [...name('memory'), 0x02, 0x00],
    ...funcs.map(([n], i) => [...name(n), 0x00, ...uleb(i)]),
  ];
  const data = [...m.data].map(([addr, bytes]) => [0x00, 0x41, ...sleb32(addr), 0x0b, ...uleb(bytes.length), ...bytes]);
  return new Uint8Array([
    0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
    ...section(1, vec(types)),
    ...section(3, vec(funcs.map((_, i) => uleb(i)))),
    ...section(5, vec([[0x00, ...uleb(m.pages)]])),
    ...section(7, vec(exports)),
    ...section(10, vec(bodies)),
    ...section(11, vec(data)),
  ]);
}

const resultLen = 128;
/** The info JSON that fakeModule returns. */
export const fakeInfo = { apiVersion: 1, engineVersion: 'fake', configFileNames: ['.gotebanare.yml'] };
/** The compile JSON that fakeModule returns. */
export const fakeCompiled = { handle: 1, diagnostics: [], rules: [] };

/** A module whose buffers sit at base and whose exports succeed. */
export function fakeModule(base: number, pages: number, progress: number): FakeModule {
  return {
    pages,
    resultLen,
    data: new Map([
      [base, jsonBytes(fakeInfo, resultLen)],
      [base + 0x100, jsonBytes(fakeCompiled, resultLen)],
      [base + 0x200, u32Bytes(progress)],
    ]),
    alloc: { returns: base + 0x1000 },
    info: { returns: base },
    compile: { returns: base + 0x100 },
    analyzeChange: { trap: true },
    presets: { trap: true },
    yamlProgressPtr: { returns: base + 0x200 },
  };
}

export const compileFake = (m: FakeModule) => WebAssembly.compile(buildFakeModule(m) as Uint8Array<ArrayBuffer>);
