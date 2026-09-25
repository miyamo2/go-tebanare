package bridge

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/contract/*.json at the repository root")

// contractDir holds the JSON that the bridge returns for fixed inputs. The
// contract test of packages/engine runs the same inputs through engine.wasm
// and compares the results with these files.
var contractDir = filepath.Join("..", "..", "testdata", "contract")

// invalidConfig has errors in several sections.
const invalidConfig = `version: 1
files:
  exclude: ['[']
presets:
  - getter: {max_depth: 0}
  - nope
`

type compileCase struct {
	Name string          `json:"name"`
	YAML string          `json:"yaml"`
	Want json.RawMessage `json:"want"`
}

// sampleConfig reads a configuration from testdata/config/valid.
func sampleConfig(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "config", "valid", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestContractInfo(t *testing.T) {
	var info map[string]any
	if err := json.Unmarshal(New().Info(), &info); err != nil {
		t.Fatal(err)
	}
	// The engine version depends on the build.
	delete(info, "engineVersion")
	checkContract(t, "info.json", info)
}

func TestContractCompile(t *testing.T) {
	var compiles []compileCase
	for _, c := range []struct{ name, yaml string }{
		{"preset settings example", string(sampleConfig(t, "preset-settings.yml"))},
		{"errors", invalidConfig},
	} {
		compiles = append(compiles, compileCase{c.name, c.yaml, New().Compile([]byte(c.yaml))})
	}
	checkContract(t, "compile.json", map[string]any{"cases": compiles})
}

// checkContract compares the indented JSON of v with the file name in
// contractDir, or writes the file when -update is set.
func checkContract(t *testing.T, name string, v any) {
	t.Helper()
	got, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join(contractDir, name)
	if *update {
		if err := os.MkdirAll(contractDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test ./internal/bridge -run TestContract -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is out of date; run go test ./internal/bridge -run TestContract -update. Got:\n%s", path, got)
	}
}
