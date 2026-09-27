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
inherits `wasm-unknown`, with the conservative GC, no scheduler, and an 8 MB
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
  that handler before it reads. The precise GC most likely freed memory
  the parser still used; we did not identify which object. Builds with the
  conservative or the leaking GC reject the same input without a trap.
- The standard Go toolchain can build the same exports for `GOOS=wasip1`
  with `-buildmode=c-shared`; CI keeps that build compiling as a fallback.
- Measured on this repository: the engine is 0.84 MB (0.36 MB gzip), a
  5,000-line file takes about 40 ms, and all 3,866 non-test files of the Go
  1.26.8 standard library give the same results as the native build.
