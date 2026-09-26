// Package bridge holds the logic behind the wasm exports of
// cmd/gotebanare-wasm. It turns byte inputs into calls on the public
// tebanare API and every result into JSON, so it can be tested natively.
// No method panics on bad input: errors are reported in the JSON. The one
// exception is a YAML syntax error in the TinyGo build (see Compile).
package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"

	tebanare "github.com/miyamo2/go-tebanare"
	tbconfig "github.com/miyamo2/go-tebanare/internal/config"
)

// YAMLDone is the value of the YAMLProgress counter before the first
// Compile call and after a config that parsed as YAML without an error.
const YAMLDone = ^uint32(0)

// Bridge keeps the compiled rulesets that the JavaScript side refers to by
// handle. Handles start at 1; 0 is never valid.
type Bridge struct {
	rulesets map[uint32]*tebanare.Ruleset
	next     uint32
	yamlRead uint32
}

// New returns an empty Bridge.
func New() *Bridge {
	return &Bridge{rulesets: map[uint32]*tebanare.Ruleset{}, yamlRead: YAMLDone}
}

type infoJSON struct {
	APIVersion      int      `json:"apiVersion"`
	EngineVersion   string   `json:"engineVersion"`
	ConfigFileNames []string `json:"configFileNames"`
}

// Info returns {"apiVersion":N,"engineVersion":"...","configFileNames":[...]}.
// configFileNames is tebanare.ConfigFileNames, the lookup order of the
// configuration file.
func (b *Bridge) Info() []byte {
	return marshal(infoJSON{
		APIVersion:      tebanare.APIVersion,
		EngineVersion:   tebanare.EngineVersion,
		ConfigFileNames: tebanare.ConfigFileNames,
	})
}

type compileJSON struct {
	Handle      uint32                `json:"handle"`
	Diagnostics []tebanare.Diagnostic `json:"diagnostics"`
	Rules       []tebanare.RuleInfo   `json:"rules"`
	Error       string                `json:"error,omitempty"`
}

// Compile compiles a configuration. On success it returns the new handle,
// the warnings, and the rules. On failure the handle is 0, diagnostics hold
// the errors followed by the warnings, and error holds the first error.
//
// yaml.v3 reports YAML syntax errors by panicking, and the TinyGo build
// cannot recover, so a syntax error stops the module with a trap. Compile
// first parses src while it counts the bytes the parser reads (see
// YAMLProgress), so that the host can tell where the parser stopped.
func (b *Bridge) Compile(src []byte) []byte {
	b.scanYAML(src)
	rs, warnings, err := tebanare.Compile(src)
	out := compileJSON{Diagnostics: []tebanare.Diagnostic{}, Rules: []tebanare.RuleInfo{}}
	if err != nil {
		var ce *tebanare.ConfigError
		if errors.As(err, &ce) && len(ce.Diagnostics) > 0 {
			out.Diagnostics = append(out.Diagnostics, ce.Diagnostics...)
			out.Error = ce.Diagnostics[0].Format("")
		} else {
			out.Error = err.Error()
		}
		out.Diagnostics = append(out.Diagnostics, warnings...)
		return marshal(out)
	}
	b.next++
	b.rulesets[b.next] = rs
	out.Handle = b.next
	out.Diagnostics = append(out.Diagnostics, warnings...)
	if rules := rs.Rules(); rules != nil {
		out.Rules = rules
	}
	return marshal(out)
}

// YAMLProgress returns the counter that Compile sets while yaml.v3 parses
// the config: the number of bytes the parser has read, or YAMLDone when no
// parse is in progress and the last one found no YAML error. After a trap
// inside the compile export, the host reads the counter from linear
// memory: a value other than YAMLDone means that the parser stopped at a
// syntax error after reading that many bytes.
func (b *Bridge) YAMLProgress() *uint32 {
	return &b.yamlRead
}

// scanYAML parses the documents in src with yaml.v3, handing the parser
// one byte per read, and counts the bytes read in b.yamlRead. Like
// tebanare.Compile, it stops after the second document with content, so a
// syntax error after that document traps neither build. It sets YAMLDone
// when the documents it parsed have no YAML error and leaves the count
// when they have one.
func (b *Bridge) scanYAML(src []byte) {
	b.yamlRead = 0
	dec := yaml.NewDecoder(&byteReader{src: src, n: &b.yamlRead})
	for withContent := 0; withContent < 2; {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return
		}
		if tbconfig.HasContent(&doc) {
			withContent++
		}
	}
	b.yamlRead = YAMLDone
}

// byteReader returns src one byte per Read call. *n is the number of
// bytes read.
type byteReader struct {
	src []byte
	n   *uint32
}

func (r *byteReader) Read(p []byte) (int, error) {
	i := int(*r.n)
	if i >= len(r.src) {
		return 0, io.EOF
	}
	if len(p) == 0 {
		return 0, nil
	}
	p[0] = r.src[i]
	*r.n++
	return 1, nil
}

type changeMeta struct {
	OldPath string `json:"oldPath"`
	NewPath string `json:"newPath"`
	HasOld  bool   `json:"hasOld"`
	HasNew  bool   `json:"hasNew"`
}

// AnalyzeChange analyzes one changed file with the ruleset behind handle.
// meta is {"oldPath","newPath","hasOld","hasNew"}; a side with has*=false
// is absent (added or deleted file). It returns a ChangeResult, or
// {"error":"..."} for an unknown handle or bad meta.
func (b *Bridge) AnalyzeChange(handle uint32, meta, oldSrc, newSrc []byte) []byte {
	rs, errJSON := b.lookup(handle)
	if rs == nil {
		return errJSON
	}
	var m changeMeta
	if err := json.Unmarshal(meta, &m); err != nil {
		return errorJSON(fmt.Sprintf("bad meta: %v", err))
	}
	ch := tebanare.FileChange{OldPath: m.OldPath, NewPath: m.NewPath}
	if m.HasOld {
		ch.Old = nonNil(oldSrc)
	}
	if m.HasNew {
		ch.New = nonNil(newSrc)
	}
	return marshal(rs.AnalyzeChange(ch))
}

// Release forgets the ruleset behind handle. Unknown handles are ignored.
func (b *Bridge) Release(handle uint32) {
	delete(b.rulesets, handle)
}

type presetsJSON struct {
	Presets []tebanare.PresetInfo `json:"presets"`
}

// Presets returns {"presets":[...]}.
func (b *Bridge) Presets() []byte {
	return marshal(presetsJSON{Presets: tebanare.Presets()})
}

func (b *Bridge) lookup(handle uint32) (*tebanare.Ruleset, []byte) {
	rs := b.rulesets[handle]
	if rs == nil {
		return nil, errorJSON(fmt.Sprintf("unknown handle %d", handle))
	}
	return rs, nil
}

func nonNil(b []byte) []byte {
	if b == nil {
		return []byte{}
	}
	return b
}

type errJSON struct {
	Error string `json:"error"`
}

func errorJSON(msg string) []byte {
	return marshal(errJSON{Error: msg})
}

func marshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		b, _ = json.Marshal(errJSON{Error: "encoding the result failed: " + err.Error()})
	}
	return b
}
