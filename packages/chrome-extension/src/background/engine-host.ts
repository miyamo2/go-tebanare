// Hosts the wasm engine in the service worker (plan 6.1). Chrome stops an
// idle service worker and drops every module-level value, so each worker
// life creates its engine on the first request that needs one.

import {
  createEngine,
  type ChangeResult,
  type CompiledRuleset,
  type Engine,
  type FileChangeInput,
} from '@go-tebanare/engine';

/** EngineFactory loads a new engine. */
export type EngineFactory = () => Promise<Engine>;

/**
 * wasmEngineFactory loads engine.wasm from the extension package.
 * fetchFn and create exist for tests.
 */
export function wasmEngineFactory(
  getURL: (path: string) => string,
  fetchFn: (url: string) => Promise<Response> = (url) => fetch(url),
  create: typeof createEngine = createEngine,
): EngineFactory {
  return () => create(fetchFn(getURL('engine.wasm')));
}

/** EngineHost creates one engine lazily and forwards calls to it. */
export class EngineHost {
  readonly #factory: EngineFactory;
  #engine: Promise<Engine> | null = null;

  constructor(factory: EngineFactory) {
    this.#factory = factory;
  }

  /**
   * engine returns the engine, creating it on the first call. Concurrent
   * first calls share one creation. After a failed creation the next call
   * tries again.
   */
  engine(): Promise<Engine> {
    if (this.#engine) return this.#engine;
    const p = (async () => this.#factory())();
    this.#engine = p;
    p.catch(() => {
      if (this.#engine === p) this.#engine = null;
    });
    return p;
  }

  /** compile compiles yaml. The engine caches rulesets by the SHA-256 of yaml. */
  async compile(yaml: string): Promise<CompiledRuleset> {
    return (await this.engine()).compile(yaml);
  }

  async analyzeChange(rs: CompiledRuleset, change: FileChangeInput): Promise<ChangeResult> {
    return (await this.engine()).analyzeChange(rs, change);
  }

  async engineVersion(): Promise<string> {
    return (await this.engine()).engineVersion;
  }
}
