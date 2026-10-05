# 0006 Make the JSON Schema the source of truth for the configuration

Status: Accepted

## Context

`schema/gotebanare.schema.json` gives editors completion, hover
documentation, and validation for `.gotebanare.yml`. It used to be
generated from the Go declarations: the preset settings structs carried
the types, defaults, and descriptions in struct tags, while the top-level
keys and the constraints that the struct tags could not express (such as
`minimum` and `pattern`) were written by hand in the generator. The engine
validated the configuration with its own code, so the same rules lived in
the schema generator and in the validation code, and a test compared them.

## Decision

The schema is the source of truth for the keys, types, defaults,
descriptions, and constraints of the configuration. It is edited by hand.

- The engine embeds the schema (package `schema`) and validates every
  configuration against it with
  [santhosh-tekuri/jsonschema](https://github.com/santhosh-tekuri/jsonschema)
  v6. That library builds and runs under TinyGo without imports; xeipuuv/gojsonschema
  (`net/http`, `text/template`) and google/jsonschema-go (reflection that
  fails under TinyGo, errors without instance locations) did not.
- The Go types of a configuration (`internal/configschema/types_gen.go`)
  are generated from the schema with
  [quicktype](https://github.com/glideapps/quicktype), pinned in
  `package.json` (`make generate`). Among the generators compared, it was
  the one that compiles from the schema as written (nullable types) and
  decodes the `anyOf` of a `presets` item; atombender/go-jsonschema needs
  non-nullable types and turns that `anyOf` into a string, and
  a-h/generate ignores defaults.
- What the generated types do not carry is read from the embedded schema at
  run time: the order and descriptions of the preset settings (each
  description ends with "Default: <text>."), the defaults, which are
  filled in before a value is decoded, and the allowed keys in error
  messages.
- Go code checks only what the schema cannot express: a preset listed
  twice, the syntax of globs, and the YAML itself (syntax, duplicate keys,
  merge keys, keys that are not strings, nonstandard tags, and aliases).

## Consequences

- Adding or changing a setting means editing the schema, running
  `make generate`, and using the new field in the preset's code. A preset
  in the schema without code, or code without a schema entry, fails the
  tests.
- `engine.wasm` grows by about 125 KB gzip-compressed for the validator.
- Schema errors stop the compiler, so errors that only Go checks, such as
  an invalid glob, are reported once the schema errors are fixed.
- Error messages are built from the schema error kinds and keep the
  position and field path of the value at fault. Some read more
  mechanically than before, such as a `pattern` mismatch, which names the
  regular expression.
- Values follow JSON Schema: `1.0` is an integer, so `version: 1.0` is
  valid. A `null` value counts as unset where the schema allows null,
  which every setting and section does; `version` must be an integer.
- The generator runs on bun, so the `go` CI job installs the bun
  dependencies before `make generate-check`.
