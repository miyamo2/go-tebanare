// Package config compiles a .gotebanare.yml configuration into a rule.Set.
//
// Compile checks the configuration in three steps and stops after a step
// that finds errors. It converts the yaml.Node tree into a JSON value,
// reporting what JSON cannot hold or the configuration does not accept
// (duplicate keys, merge keys, keys that are not strings, nonstandard
// tags). It validates the value against the embedded JSON Schema, the
// source of truth for the configuration (see package schema). Then it
// decodes the value into the types generated from the schema (see package
// configschema) and checks what the schema cannot express: globs and
// presets listed twice. Each error carries the line and column of the
// value at fault and a field path such as "presets[0](getter).max_depth".
//
// The config is one YAML document. Empty documents before or after it,
// such as the one a trailing "---" starts, are skipped. Anchors and
// aliases are allowed, but the aliases of a config may copy at most
// 64 KiB of content, and an alias inside the value it refers to is an
// error.
package config
