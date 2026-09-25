package analyzer

import (
	"fmt"
	"slices"

	"github.com/miyamo2/go-tebanare/internal/lines"
	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// FileResult is the outcome of analyzing one version of a file.
type FileResult struct {
	// Skipped says why the file was not analyzed. When it is set, the
	// other fields are empty except Diagnostics, which explains the skip.
	Skipped result.SkipReason
	// Ranges holds the lines to hide when the file is used on its own:
	// the stmt matches and the func matches whose key is not duplicated,
	// merged.
	Ranges []result.Range
	// Funcs holds the func rule matches that passed the occupancy check,
	// in source order, including the ones whose key is duplicated.
	Funcs []FuncMatch
	// Decls maps each pairing key to the number of function declarations
	// with that key. A count of 2 or more marks a duplicate.
	Decls       map[string]int
	Diagnostics []result.Diagnostic
}

// FuncMatch is a func rule match that passed the occupancy check.
type FuncMatch struct {
	// Key is the pairing key of the declaration: "Recv.Name" for a
	// method, "Name" for a function, with "#n" appended for init
	// functions and declarations named "_" (plan 4.8).
	Key    string
	RuleID string
	// Start and End are the first and last hidden line.
	Start, End int
	Hit        result.Hit
}

// AnalyzeFile analyzes one version of the file at path. A nil set has no
// rules and accepts every .go file.
func AnalyzeFile(set *rule.Set, path string, src []byte, opt Options) FileResult {
	fa := analyzeFile(set, path, src, opt)
	res := FileResult{
		Ranges:      []result.Range{},
		Funcs:       []FuncMatch{},
		Decls:       map[string]int{},
		Diagnostics: slices.Clone(fa.diags),
	}
	if res.Diagnostics == nil {
		res.Diagnostics = []result.Diagnostic{}
	}
	if fa.skip != nil {
		res.Skipped = fa.skip.reason
		return res
	}
	spans := slices.Clone(fa.nodes)
	for _, f := range fa.funcs {
		if f.shared {
			continue
		}
		res.Funcs = append(res.Funcs, FuncMatch{Key: f.key, RuleID: f.hit.RuleID, Start: f.start, End: f.end, Hit: f.hit})
		if fa.decls[f.key] < 2 {
			spans = append(spans, f.span)
		}
	}
	res.Ranges = mergeSpans(spans, fa.blank)
	res.Decls = fa.decls
	return res
}

// fileAnalysis is the internal result of analyzing one version of a file.
type fileAnalysis struct {
	// skip is set when the file was not analyzed; diags then holds one
	// skipped diagnostic.
	skip *skip
	// nodes holds the accepted stmt spans.
	nodes []span
	// funcs holds the func matches, including the ones that failed the
	// occupancy check.
	funcs []funcHit
	decls map[string]int
	diags []result.Diagnostic
	blank func(line int) bool
}

type funcHit struct {
	key string
	// shared is set when the match failed the occupancy check. Pairing
	// counts it as a match, but it is never hidden.
	shared bool
	span
}

// analyzeFile runs plan 4.7 on one version of a file.
func analyzeFile(set *rule.Set, path string, src []byte, opt Options) *fileAnalysis {
	if set == nil {
		set = &rule.Set{}
	}
	opt = opt.withDefaults()
	fa := &fileAnalysis{decls: map[string]int{}}
	var p *parsed
	var sk *skip
	if set.IsTarget(path) {
		p, sk = prepare(path, src, opt)
	} else {
		sk = notTarget(path)
	}
	if sk != nil {
		fa.skip = sk
		fa.diags = []result.Diagnostic{skipDiagnostic(sk)}
		return fa
	}
	a := &analysis{parsed: p, set: set, opt: opt, out: fa}
	a.run()
	fa.blank = lines.BlankLines(src)
	return fa
}

func skipDiagnostic(sk *skip) result.Diagnostic {
	return result.Diagnostic{
		Severity: result.SeverityInfo,
		Code:     result.CodeSkipped,
		Message:  fmt.Sprintf("skipped (%s): %s", sk.reason, sk.msg),
		Line:     sk.line,
		Column:   sk.column,
	}
}
