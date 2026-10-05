// Package config compiles a .gotebanare.yml configuration into a rule.Set.
//
// Compile decodes the configuration into File with a yaml.v3 decoder that
// rejects unknown keys, then checks the values. The errors of the decoder
// carry only a line, in their message's form; the checks of `version` and
// `files` give a field path such as "files.include[0]" without a
// position. The items of `presets` stay YAML nodes, so the errors in them
// carry the line and column of the value at fault and a field path such
// as "presets[0](getter).max_depth". After a decoder error Compile stops;
// otherwise it reports every error it finds. A null value counts as unset.
//
// The config is one YAML document. Empty documents before or after it,
// such as the one a trailing "---" starts, are skipped. Anchors and
// aliases are allowed, but the aliases of a config may copy at most
// 64 KiB of content, and an alias inside the value it refers to is an
// error.
package config
