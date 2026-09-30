// Package config compiles a .gotebanare.yml configuration into a rule.Set.
//
// Compile walks the yaml.Node tree, so each error carries the line and
// column of the value at fault and a field path such as
// "presets[0](getter).max_depth". Decoding is strict: unknown keys and
// values of the wrong type are errors. Compile reports every error it
// finds, and a null value counts as unset.
//
// The config is one YAML document. Empty documents before or after it,
// such as the one a trailing "---" starts, are skipped. Anchors and
// aliases are allowed, but the aliases of a config may copy at most
// 64 KiB of content, and an alias inside the value it refers to is an
// error.
//
// A signature pattern error is reported at the position of the pattern
// string. Its message gives the line and column inside the pattern and
// shows the pattern with a caret under the error:
//
//	.gotebanare.yml:26:11: rules[2](generic-repo-finders).func: 1:29: expected type, found ','
//	    func (*Repository[_]) Find*(, ...) (_, error)
//	                                ^
package config
