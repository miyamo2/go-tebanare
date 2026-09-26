package analyzer

import (
	"bytes"
	"fmt"
	"go/ast"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// genSource returns a file of at least n lines that mixes getters, if err
// blocks, and log calls. Blocks with an index divisible by every change a
// little, so two sources with different every values differ in places.
func genSource(n, every int) []byte {
	var b bytes.Buffer
	b.WriteString("package p\n\nimport \"context\"\n\n")
	for i := 0; bytes.Count(b.Bytes(), []byte{'\n'}) < n; i++ {
		fmt.Fprintf(&b, "// Name%d returns the name.\nfunc (u *User) Name%d() string { return u.name%d }\n\n", i, i, i)
		fmt.Fprintf(&b, "func (s *Service) Do%d(ctx context.Context, id string) error {\n", i)
		b.WriteString("\tlog.Debug(\"do\", \"id\", id)\n")
		b.WriteString("\tv, err := s.repo.Find(ctx, id)\n\tif err != nil {\n\t\treturn err\n\t}\n")
		if i%every == 0 {
			b.WriteString("\tlog.Printf(\"changed\")\n")
		}
		fmt.Fprintf(&b, "\tif v.n > %d {\n\t\tlog.Printf(\"big %%d\", v.n)\n\t}\n\treturn s.save(ctx, v)\n}\n\n", i)
	}
	return b.Bytes()
}

// benchSet has a getter-like func rule and iferr-like and debug-log stmt
// rules.
func benchSet(tb testing.TB) *rule.Set {
	getter := funcsWhere("getter", func(fd *ast.FuncDecl) bool {
		if _, _, ok := rule.RecvBase(fd); !ok || fd.Body == nil || len(fd.Body.List) != 1 {
			return false
		}
		_, ok := fd.Body.List[0].(*ast.ReturnStmt)
		return ok && fd.Type.Params.NumFields() == 0 && fd.Type.Results.NumFields() == 1
	})
	return ruleSet(
		getter,
		stmtsMatching(tb, "iferr", "IfStmt", `^if err != nil \{\s*return (\w+, )*err\s*\}$`),
		stmtsMatching(tb, "debug", "ExprStmt", `^log\.Debug\w*\(`),
	)
}

func benchmarkAnalyze(b *testing.B, lines int) {
	set := benchSet(b)
	oldSrc, newSrc := genSource(lines, 7), genSource(lines, 5)
	b.Run("file", func(b *testing.B) {
		b.SetBytes(int64(len(newSrc)))
		for b.Loop() {
			if res := AnalyzeFile(set, "x.go", newSrc, DefaultOptions()); res.Skipped != "" || len(res.Ranges) == 0 {
				b.Fatalf("unexpected result: %q, %d ranges", res.Skipped, len(res.Ranges))
			}
		}
	})
	b.Run("change", func(b *testing.B) {
		b.SetBytes(int64(len(oldSrc) + len(newSrc)))
		for b.Loop() {
			if res := AnalyzeChange(set, Side{"x.go", oldSrc}, Side{"x.go", newSrc}, DefaultOptions()); res.Skipped != "" {
				b.Fatalf("skipped: %q", res.Skipped)
			}
		}
	})
}

func BenchmarkAnalyze5k(b *testing.B)  { benchmarkAnalyze(b, 5000) }
func BenchmarkAnalyze20k(b *testing.B) { benchmarkAnalyze(b, 20000) }

func TestGenSource(t *testing.T) {
	src := genSource(200, 5)
	res := AnalyzeFile(benchSet(t), "x.go", src, DefaultOptions())
	if res.Skipped != "" {
		t.Fatal(res.Diagnostics)
	}
	count := map[string]int{}
	for _, r := range res.Ranges {
		for _, h := range r.Hits {
			count[h.RuleID]++
		}
	}
	// Each block has one getter, one iferr, and one debug call. Equal
	// hits within a range are listed once, so count distinct labels.
	if count["getter"] == 0 || count["iferr"] == 0 || count["debug"] == 0 {
		t.Errorf("hits per rule = %v, want getter, iferr, and debug hits", count)
	}
	for _, d := range res.Diagnostics {
		if d.Severity != result.SeverityInfo {
			t.Errorf("diagnostic %v", d)
		}
	}
}
