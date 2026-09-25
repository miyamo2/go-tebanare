# Design

This page describes how go-tebanare is built: the layers, the boundary between Go and JavaScript, and what happens when something fails. The [architecture decision records](adr/README.md) explain why. [configuration.md](configuration.md) specifies what gets hidden.

## Layers

Each layer uses only the layers below it.

| Layer | Location | Role |
|---|---|---|
| Browser extension | `packages/chrome-extension` | GitHub adapter: reads the diff from the page, fetches the configuration and the sources, and folds lines. |
| Engine | `packages/engine` (`@go-tebanare/engine`) | Loads `engine.wasm` and gives a typed API. Runs in browsers, service workers, and Node, and knows nothing about GitHub. |
| wasm bridge | `cmd/gotebanare-wasm`, `internal/bridge` | Exports the facade to JavaScript. Built with TinyGo ([ADR 0001](adr/0001-tinygo-wasm-engine.md)). |
| Facade | package `tebanare` at the repository root | The public Go API: `Compile`, `Ruleset.AnalyzeChange`, `Presets`, `ConfigFileNames`. |
| Core | `internal/...` | Configuration, presets, normalized text, analysis, and line ranges. No I/O. |

The core owns every decision about what to hide. Adapters own I/O: the extension fetches files with the browser's GitHub session. The lookup order of the configuration file is a core constant, `tebanare.ConfigFileNames`, which the engine publishes as `engine.configFileNames`.

## Engine boundary

`engine.wasm` imports nothing. Its exports take and return `uint32` values:

- The host copies each input into a buffer from `alloc`, calls an export, reads `result_len()` bytes of JSON at the address the export returns, and frees every buffer with `free`.
- `compile` returns a handle to a compiled ruleset. `analyze_change` takes the handle, a JSON meta object, and the source bytes of both sides. `info` returns the API version, the engine version, and the configuration file names.
- Errors come back as JSON (`{"error": "..."}` or diagnostics). Only a trap escapes this rule.

Wasm returns `i32` values, which JavaScript reads as signed numbers, so the engine converts every returned address with `>>> 0`. Addresses past 2 GiB stay valid.

`@go-tebanare/engine` compiles the module once and keeps one instance at a time:

- Calls run one at a time through a queue.
- Compiled rulesets are cached by the SHA-256 of their YAML (16 entries). A `CompiledRuleset` outlives the instance: after a new instance starts, the engine compiles the YAML again on first use.
- The engine drops the instance after a trap, after 1,000 calls, or when its memory grows past 256 MiB, and starts a new one on the next call. Wasm memory never shrinks, so the last two keep a long session from holding on to memory.

## Failures

TinyGo cannot `recover` on wasm, so a panic traps and leaves the instance unusable. The engine turns traps into results that hide nothing:

| Call | After a trap |
|---|---|
| `analyzeChange` | Resolves with no ranges, an `engine-crashed` diagnostic, and `skipped: "engine-error"`. |
| `compile`, while yaml.v3 parses the YAML | Rejects with a `ConfigError` holding one `config-syntax` diagnostic on the line where the parser stopped. yaml.v3 reports syntax errors by panicking, and the bridge counts the bytes the parser reads so the engine can find that line. |
| `compile`, later, and `presets` | Rejects with the trap's error. |

Code must not rely on `recover` on the paths the engine runs. Standard library packages that panic and recover internally are called in modes that do not panic: `go/parser` runs with `AllErrors`. Deep nesting would overflow the 8 MB TinyGo stack, so the analyzer checks bracket depth, `else if` chains, and syntax tree depth before it recurses ([safety behavior](configuration.md#safety-behavior)).

## TypeScript workspace

The TypeScript packages live under `packages/` in one bun workspace: the root `package.json` lists `packages/*`, pins bun in `packageManager`, and `bun.lock` holds the resolved versions. A package depends on another with `workspace:*`, as the extension does on `@go-tebanare/engine`. A VS Code extension would join as `packages/vscode-extension` and load the same `engine.wasm` through `@go-tebanare/engine`.

Run the scripts from the repository root:

1. `bun install`
2. `bun run typecheck`, `bun run lint`, and `bun run test` run the script of every package.
3. `bun run --filter <package> <script>` runs one package's script.

The tests run on vitest and the end-to-end tests on Playwright, both under Node.

## Tests across the boundary

| Test | What it compares |
|---|---|
| `testdata/analyze/*` | `AnalyzeChange` results. Go checks them natively, and `packages/engine/test/contract.test.ts` runs the same cases through `engine.wasm`. |
| `testdata/contract/*.json` | The JSON of `info`, `presets`, and `compile`, written by `internal/bridge` natively and compared with the engine's results. |
| `make parity` | Every non-test file of the Go standard library, analyzed natively and in wasm. `PARITY_FLAGS=-pairs` analyzes each file as a change from the file before it, so declarations pair across the two sides. |
| `testvectors/fuzz/*.json` | The seeds and `testdata/fuzz` corpora of `FuzzAnalyze` and `FuzzCompile`, replayed through `engine.wasm`. In CI the replay also runs the corpus that the fuzzers collected. A trap fails the test unless it is a YAML syntax error that the native build reports too. |
| `packages/engine/test/stress.test.ts` | Inputs exactly at each nesting limit and one step past it. |
| `packages/engine/test/bench.test.ts` | Analysis time in wasm: under 200 ms for 5,000 lines, under 1 s for 20,000 lines. |
