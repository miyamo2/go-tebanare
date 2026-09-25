import { WasmInstance, yamlDone } from './abi.js';
import {
  ConfigError,
  EngineError,
  type ChangeResult,
  type CompiledRuleset,
  type Diagnostic,
  type EngineInfo,
  type FileChangeInput,
  type PresetInfo,
  type RuleInfo,
} from './types.js';
import { Lru, SerialQueue, sha256Hex, toBytes } from './util.js';

/** The engine API version this package understands. */
export const supportedApiVersion = 1;

export type RecreateReason = 'crash' | 'calls' | 'memory';

export interface EngineOptions {
  /** Recreate the instance after this many calls. Default 1000. */
  maxCallsPerInstance?: number;
  /** Recreate the instance when its memory grows past this size. Default 256 MiB. */
  maxMemoryBytes?: number;
  /**
   * Called with the reason each time the engine drops its instance to start
   * a new one. dispose() does not call it. The engine ignores exceptions
   * that it throws.
   */
  onRecreate?: (reason: RecreateReason) => void;
}

export interface Engine {
  readonly engineVersion: string;
  readonly apiVersion: number;
  /**
   * The configuration file names in lookup order. An adapter uses the
   * first one that exists at the repository root.
   */
  readonly configFileNames: readonly string[];
  /**
   * Compiles a configuration. Rejects with ConfigError when it is invalid.
   * A YAML syntax error gives one config-syntax diagnostic whose line is
   * where the YAML parser stopped; the wasm build cannot report the
   * parser's message.
   */
  compile(yaml: string): Promise<CompiledRuleset>;
  /**
   * Analyzes one changed file. When the wasm instance crashes, the promise
   * resolves with a result that hides nothing, holds one engine-crashed
   * diagnostic, and has skipped set to "engine-error".
   */
  analyzeChange(rs: CompiledRuleset, change: FileChangeInput): Promise<ChangeResult>;
  presets(): Promise<PresetInfo[]>;
  /** Drops the instance. Later calls reject. */
  dispose(): void;
}

/**
 * Where createEngine gets engine.wasm. A URL or a string is fetched, such
 * as `chrome.runtime.getURL('engine.wasm')` in an extension.
 */
export type WasmSource =
  | URL
  | string
  | Response
  | ArrayBuffer
  | Uint8Array
  | WebAssembly.Module
  | PromiseLike<Response>;

/** Loads engine.wasm and returns a ready engine. */
export async function createEngine(wasm: WasmSource, options: EngineOptions = {}): Promise<Engine> {
  const engine = new EngineImpl(await toModule(wasm), options);
  await engine.init();
  return engine;
}

async function toModule(src: WasmSource): Promise<WebAssembly.Module> {
  if (src instanceof WebAssembly.Module) return src;
  const v = typeof src === 'string' || src instanceof URL ? fetch(src) : src;
  const r = await v;
  if (r instanceof ArrayBuffer || ArrayBuffer.isView(r)) {
    return WebAssembly.compile(r as Uint8Array<ArrayBuffer>);
  }
  if (typeof Response !== 'undefined' && r instanceof Response) {
    if (!r.ok) throw new Error(`loading engine.wasm failed: HTTP ${r.status}`);
    // compileStreaming requires the application/wasm content type.
    const type = r.headers.get('content-type') ?? '';
    if (typeof WebAssembly.compileStreaming === 'function' && type.split(';')[0]?.trim() === 'application/wasm') {
      return WebAssembly.compileStreaming(r);
    }
    return WebAssembly.compile(await r.arrayBuffer());
  }
  throw new TypeError('createEngine: unsupported wasm source');
}

interface CompileResponse {
  handle: number;
  diagnostics: Diagnostic[];
  rules: RuleInfo[];
  error?: string;
}

const encoder = new TextEncoder();

/** The message of the config-syntax diagnostic for a YAML syntax error. */
export const yamlSyntaxMessage =
  'the config has a YAML syntax error; the YAML parser stopped on this line';

/**
 * A trap while yaml.v3 parsed the config. yaml.v3 reports syntax errors by
 * panicking, and TinyGo wasm cannot recover.
 */
class YamlTrap extends Error {
  readonly line: number;

  constructor(line: number, cause: unknown) {
    super('YAML syntax error', { cause });
    this.line = line;
  }
}

/** Returns the 1-based line of the last of the first n bytes of src. */
function lineOfByte(src: Uint8Array, n: number): number {
  let line = 1;
  const end = Math.min(n - 1, src.length);
  for (let i = 0; i < end; i++) if (src[i] === 0x0a) line++;
  return line;
}

export class EngineImpl implements Engine {
  engineVersion = '';
  apiVersion = 0;
  configFileNames: readonly string[] = [];

  readonly #module: WebAssembly.Module;
  readonly #maxCalls: number;
  readonly #maxMemory: number;
  readonly #onRecreate: ((reason: RecreateReason) => void) | undefined;
  readonly #queue = new SerialQueue();
  readonly #rulesets: Lru<string, CompiledRuleset>;
  #instance: WasmInstance | null = null;
  #handles = new Map<string, number>();
  #calls = 0;
  #disposed = false;
  #faultNext = false;

  constructor(module: WebAssembly.Module, options: EngineOptions) {
    this.#module = module;
    this.#maxCalls = options.maxCallsPerInstance ?? 1000;
    this.#maxMemory = options.maxMemoryBytes ?? 256 * 1024 * 1024;
    this.#onRecreate = options.onRecreate;
    this.#rulesets = new Lru(16, (key) => this.#releaseLater(key));
  }

  async init(): Promise<void> {
    const info = (await this.#run((inst) => inst.info())) as EngineInfo;
    if (info.apiVersion !== supportedApiVersion) {
      throw new Error(`engine.wasm API version ${info.apiVersion}, want ${supportedApiVersion}`);
    }
    this.apiVersion = info.apiVersion;
    this.engineVersion = info.engineVersion;
    this.configFileNames = Object.freeze([...info.configFileNames]);
  }

  async compile(yaml: string): Promise<CompiledRuleset> {
    if (this.#disposed) throw new EngineError('engine disposed');
    const bytes = encoder.encode(yaml);
    const key = await sha256Hex(bytes);
    const cached = this.#rulesets.get(key);
    if (cached) return cached;
    let res: CompileResponse;
    try {
      res = await this.#run((inst) => {
        let r: CompileResponse;
        try {
          r = inst.compile(bytes) as CompileResponse;
        } catch (e) {
          const read = inst.yamlProgress();
          if (read !== yamlDone) throw new YamlTrap(lineOfByte(bytes, read), e);
          throw e;
        }
        if (!r.error) this.#handles.set(key, r.handle);
        return r;
      });
    } catch (e) {
      if (!(e instanceof YamlTrap)) throw e;
      const d: Diagnostic = { severity: 'error', code: 'config-syntax', message: yamlSyntaxMessage, line: e.line };
      throw new ConfigError(`${e.line}: ${yamlSyntaxMessage}`, [d]);
    }
    if (res.error) throw new ConfigError(res.error, res.diagnostics);
    const rs: CompiledRuleset = { key, yaml, diagnostics: res.diagnostics, rules: res.rules };
    this.#rulesets.set(key, rs);
    return rs;
  }

  async analyzeChange(rs: CompiledRuleset, change: FileChangeInput): Promise<ChangeResult> {
    const hasOld = change.old != null;
    const hasNew = change.new != null;
    const meta = encoder.encode(
      JSON.stringify({
        oldPath: change.oldPath ?? change.newPath ?? '',
        newPath: change.newPath ?? change.oldPath ?? '',
        hasOld,
        hasNew,
      }),
    );
    const oldSrc = toBytes(change.old);
    const newSrc = toBytes(change.new);
    try {
      return await this.#run((inst) => {
        const out = inst.analyzeChange(this.#handleFor(inst, rs), meta, oldSrc, newSrc);
        return checked(out) as ChangeResult;
      });
    } catch (e) {
      if (this.#disposed || e instanceof EngineError) throw e;
      return {
        old: [],
        new: [],
        diagnostics: [{ severity: 'error', code: 'engine-crashed', message: String(e) }],
        skipped: 'engine-error',
      };
    }
  }

  async presets(): Promise<PresetInfo[]> {
    const out = await this.#run((inst) => inst.presets());
    return (out as { presets: PresetInfo[] }).presets;
  }

  dispose(): void {
    this.#disposed = true;
    this.#instance = null;
    this.#handles.clear();
  }

  /** Forces a trap inside the current instance. For tests only. */
  trapForTesting(): Promise<void> {
    return this.#run((inst) => inst.trapForTesting());
  }

  /**
   * Makes the next call that passes input to the wasm module trap, as a
   * panic in the engine would. For tests only.
   */
  faultNextCallForTesting(): void {
    this.#faultNext = true;
  }

  #run<T>(op: (inst: WasmInstance) => T): Promise<T> {
    return this.#queue.run(() => {
      if (this.#disposed) throw new EngineError('engine disposed');
      if (!this.#instance) {
        this.#instance = new WasmInstance(this.#module);
        this.#handles = new Map();
        this.#calls = 0;
      }
      const inst = this.#instance;
      if (this.#faultNext) {
        this.#faultNext = false;
        inst.faultNextCallForTesting();
      }
      let result: T;
      try {
        result = op(inst);
      } catch (e) {
        if (!(e instanceof EngineError)) this.#drop('crash');
        throw e;
      }
      this.#calls++;
      if (this.#calls >= this.#maxCalls) this.#drop('calls');
      else if (inst.memoryBytes > this.#maxMemory) this.#drop('memory');
      return result;
    });
  }

  #drop(reason: RecreateReason): void {
    this.#instance = null;
    this.#handles = new Map();
    try {
      this.#onRecreate?.(reason);
    } catch {
      // A failing callback must not fail the call that triggered it.
    }
  }

  #handleFor(inst: WasmInstance, rs: CompiledRuleset): number {
    const h = this.#handles.get(rs.key);
    if (h !== undefined) return h;
    const r = inst.compile(encoder.encode(rs.yaml)) as CompileResponse;
    if (r.error) throw new EngineError(`recompiling a cached ruleset failed: ${r.error}`);
    this.#handles.set(rs.key, r.handle);
    return r.handle;
  }

  #releaseLater(key: string): void {
    void this.#queue.run(() => {
      const h = this.#handles.get(key);
      if (h === undefined || !this.#instance) return;
      this.#handles.delete(key);
      this.#instance.release(h);
    }).catch(() => undefined);
  }
}

function checked(out: unknown): unknown {
  if (out && typeof out === 'object' && 'error' in out && typeof out.error === 'string') {
    throw new EngineError(out.error);
  }
  return out;
}
