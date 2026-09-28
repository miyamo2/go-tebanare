package tsconstgen

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite packages/engine/src/generated/constants.ts")

// constantsFile is the generated TypeScript file.
var constantsFile = filepath.Join("..", "..", "packages", "engine", "src", "generated", "constants.ts")

// TestConstantsUpToDate checks that packages/engine/src/generated/constants.ts
// is the output of Generate. With -update it rewrites the file.
func TestConstantsUpToDate(t *testing.T) {
	want, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.MkdirAll(filepath.Dir(constantsFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(constantsFile, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(constantsFile)
	if err != nil {
		t.Fatalf("%v (run make constants to create it)", err)
	}
	if string(got) != want {
		t.Errorf("%s is out of date (run make constants to regenerate it)", constantsFile)
	}
}
