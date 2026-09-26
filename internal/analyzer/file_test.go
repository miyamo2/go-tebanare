package analyzer

import (
	"testing"
)

// occupancyExample is the code block of the line occupancy check in
// plan 4.7.
const occupancyExample = `package p

func f() {
	f := func(x int) int { return x + 1 }
	x := compute(log.Debug("..."))
	log.Debug("...")
	tests := []tc{
		{name: "legacy", in: 1, want: 2},
	}
	_, _, _ = f, x, tests
}
`

func TestAnalyzeFileOccupancy(t *testing.T) {
	set := ruleSet(
		stmtsMatching(t, "ret", "ReturnStmt", `^return x \+ 1$`),
		stmtsMatching(t, "debug", "ExprStmt", `^log\.Debug\(`),
	)
	res := AnalyzeFile(set, "x.go", []byte(occupancyExample), DefaultOptions())
	check(t, "ranges", show(res.Ranges), []string{`6-6 debug:log.Debug("...")`})
	check(t, "diagnostics", codes(res.Diagnostics), []string{"line-shared ret 4"})
}
