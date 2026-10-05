// Package tebanare computes which lines of a changed Go file can be hidden
// from a pull request diff because the team agreed they need no
// line-by-line review: getters, empty methods, plain
// `if err != nil { return err }` blocks, and any code that the rules of a
// .gotebanare.yml file match.
//
// Compile turns the YAML configuration into a Ruleset.
// Ruleset.AnalyzeChange takes the old and new content of one file and
// returns the line ranges to hide on each side, the rule matches behind
// every range, and diagnostics about matches it left visible.
// Ruleset.Explain lists the syntax nodes on one line with their normalized
// text and matching rules, to help write rules. Presets describes the
// built-in presets.
//
// The package reads no files. Each adapter, such as the browser extension
// through the WebAssembly engine, fetches the configuration and the sources
// itself, and SelectConfigFile applies the lookup order in ConfigFileNames
// that all adapters share.
package tebanare
