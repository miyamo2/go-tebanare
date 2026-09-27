# 0001 Run the analyzer as a TinyGo WebAssembly module

Status: Accepted

## Context

go-tebanare decides what to hide on `go/ast` nodes, so the browser extension
has to run Go's own `go/parser`, `go/ast`, and `go/printer`. WebAssembly is
the only way to run Go code in a browser. A prototype built the analyzer
three ways and analyzed 896 files (about 300,000 lines) of the Go standard
library:

| Build | Size (gzip) | Start | 896 files | Imports |
|---|---|---|---|---|
| Go, `GOOS=js`, `syscall/js` | 5.47 MB (1.47 MB) | 50 ms | 2.2 s | `wasm_exec.js` |
| TinyGo, `syscall/js`, 64 KB stack | 1.29 MB (0.45 MB) | 20 ms | 5.4 s | `wasm_exec.js` |
| TinyGo, `wasm-unknown`, `//go:wasmexport`, 8 MB stack | 0.72 MB (0.31 MB) | 6 ms | 1.2 s | none |

All three returned the same results.

## Decision

Build `cmd/gotebanare-wasm` with TinyGo 0.42 for a custom target that
inherits `wasm-unknown`, with the leaking GC, no scheduler, and an 8 MB
stack (`build/tinygo/engine.json`). Export the API with `//go:wasmexport`
and pass bytes through linear memory. The tests use the standard Go
toolchain. CI runs the same corpus through both builds and
requires identical results.

## Consequences

- The module imports nothing, so browsers and Node load it with
  `WebAssembly.instantiate(module, {})` and no `wasm_exec.js`.
- TinyGo cannot `recover` on wasm. A panic traps and kills the instance, so
  `@go-tebanare/engine` discards the instance, creates a new one (about
  4 ms), and hides nothing in that file. yaml.v3 reports YAML syntax errors
  by panicking, so a syntax error in a configuration traps too. The bridge
  counts the bytes that the YAML parser reads, and the engine reports
  `config-syntax` on the line where the parser stopped.
- Standard library code that panics and recovers internally has to stay
  off those paths. The analyzer runs `go/parser` with `AllErrors`, which
  turns off its bail-out after ten errors. The engine tests replay the fuzz
  seeds and the corpus that CI's fuzzers collect through the wasm build to
  find more such paths.
- A stack overflow corrupts memory in TinyGo, so the analyzer limits bracket
  nesting (200), `else if` chains (1,000), and syntax tree depth (1,500)
  before it walks a file.
- The build first used the precise GC. With it, compiling
  `!!!!000aaaa: 0\n--- 0` as the first call on a new instance trapped in
  yaml.v3's `panic("read handler must be set")`, although the parser sets
  that handler before it reads. The build then used the conservative GC,
  which rejected that input without a trap. With the conservative GC,
  compiling `0:\n - -\n\n - -\n` or `0: &a [00,*a]` as the first call on
  a new instance trapped instead; the native build rejects both with a
  diagnostic. Both GCs most likely freed memory that was still in use; we
  did not identify which object, and a build with an extra import or with
  debug information no longer trapped.
- The build therefore uses the leaking GC, which never frees memory and is
  the default for `wasm-unknown`. `@go-tebanare/engine` already replaces an
  instance once its memory passes `maxMemoryBytes` (256 MB by default) or
  after `maxCallsPerInstance` calls (1,000), so an instance holds at most
  that limit plus what one call allocates. Analyzing a 20,000-line file grows memory by about
  33 MB and runs faster than with the conservative GC.
- The standard Go toolchain can build the same exports for `GOOS=wasip1`
  with `-buildmode=c-shared`; CI keeps that build compiling as a fallback.
- Measured on this repository: the engine is 0.66 MB (0.29 MB gzip), a
  5,000-line file takes about 40 ms, and all 3,866 non-test files of the Go
  1.26.8 standard library give the same results as the native build.
