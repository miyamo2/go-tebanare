package analyzer

import (
	"encoding/json"
	"go/ast"
	"strings"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// userGetters matches methods of *User that take no parameters, return
// one value, and consist of one return statement: a small stand-in for
// the getter preset.
func userGetters(id string) *rule.Rule {
	return funcsWhere(id, func(fd *ast.FuncDecl) bool {
		base, pointer, ok := rule.RecvBase(fd)
		if !ok || base != "User" || !pointer || fd.Type.Params.NumFields() != 0 || fd.Type.Results.NumFields() != 1 {
			return false
		}
		if fd.Body == nil || len(fd.Body.List) != 1 {
			return false
		}
		_, ok = fd.Body.List[0].(*ast.ReturnStmt)
		return ok
	})
}

// niladic reports whether fd takes no parameters and returns nothing.
func niladic(fd *ast.FuncDecl) bool {
	return fd.Type.Params.NumFields() == 0 && fd.Type.Results.NumFields() == 0
}

const getterOld = `package p

// Name returns the name.
func (u *User) Name() string { return u.name }

func (u *User) Email() string { return u.email }

func (u *User) Age() int { return u.age }
`

func TestAnalyzeChangePairing(t *testing.T) {
	set := ruleSet(userGetters("getter"))
	tests := []struct {
		name     string
		old, new string
		wantOld  []string
		wantNew  []string
		diags    []string
	}{
		{
			// Row 1: both sides match, including a moved declaration.
			// Email is deleted (row 2).
			name:    "match/match",
			old:     getterOld,
			new:     "package p\n\nfunc (u *User) Age() int { return u.age }\n\n// Name returns the name.\nfunc (u *User) Name() string { return u.name }\n",
			wantOld: []string{"3-8 getter:func (*User) Name, getter:func (*User) Email, getter:func (*User) Age"},
			wantNew: []string{"3-6 getter:func (*User) Age, getter:func (*User) Name"},
		},
		{
			// Row 2: deleted declaration. Row 3: added declaration.
			name:    "match/none and none/match",
			old:     "package p\n\nfunc (u *User) Name() string { return u.name }\n",
			new:     "package p\n\nfunc (u *User) ID() int { return u.id }\n",
			wantOld: []string{"3-3 getter:func (*User) Name"},
			wantNew: []string{"3-3 getter:func (*User) ID"},
		},
		{
			// Row 4: the new side declares Name without matching.
			name:  "match/declared",
			old:   "package p\n\nfunc (u *User) Name() string { return u.name }\n",
			new:   "package p\n\nfunc (u *User) Name() string {\n\tlog.Print(\"name\")\n\treturn u.name\n}\n",
			diags: []string{"old match-changed getter 3"},
		},
		{
			// Row 4 reversed.
			name:  "declared/match",
			old:   "package p\n\nfunc (u *User) Name() string {\n\tlog.Print(\"name\")\n\treturn u.name\n}\n",
			new:   "package p\n\nfunc (u *User) Name() string { return u.name }\n",
			diags: []string{"new match-changed getter 3"},
		},
		{
			// *User and User share the key User.Name, so a receiver
			// change counts as a change in match state.
			name:  "receiver kind change",
			old:   "package p\n\nfunc (u *User) Name() string { return u.name }\n",
			new:   "package p\n\nfunc (u User) Name() string { return u.name }\n",
			diags: []string{"old match-changed getter 3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := AnalyzeChange(set, Side{"x.go", []byte(tt.old)}, Side{"x.go", []byte(tt.new)}, DefaultOptions())
			check(t, "old", show(res.Old), tt.wantOld)
			check(t, "new", show(res.New), tt.wantNew)
			check(t, "diagnostics", codes(res.Diagnostics), tt.diags)
			if res.Skipped != "" {
				t.Errorf("Skipped = %q", res.Skipped)
			}
		})
	}
}

func TestAnalyzeChangeAddedDeleted(t *testing.T) {
	set := ruleSet(userGetters("getter"))
	res := AnalyzeChange(set, Side{Path: "x.go"}, Side{"x.go", []byte(getterOld)}, DefaultOptions())
	check(t, "added old", show(res.Old), nil)
	check(t, "added new", show(res.New), []string{"3-8 getter:func (*User) Name, getter:func (*User) Email, getter:func (*User) Age"})
	res = AnalyzeChange(set, Side{"x.go", []byte(getterOld)}, Side{Path: "x.go"}, DefaultOptions())
	check(t, "deleted old", show(res.Old), []string{"3-8 getter:func (*User) Name, getter:func (*User) Email, getter:func (*User) Age"})
	check(t, "deleted new", show(res.New), nil)
	res = AnalyzeChange(set, Side{Path: "x.go"}, Side{Path: "x.go"}, DefaultOptions())
	if len(res.Old)+len(res.New)+len(res.Diagnostics) != 0 || res.Skipped != "" {
		t.Errorf("both sides absent: got %+v, want an empty result", res)
	}
}

func TestAnalyzeChangeOccurrenceKeys(t *testing.T) {
	set := ruleSet(funcsWhere("init", func(fd *ast.FuncDecl) bool { return fd.Name.Name == "init" && niladic(fd) }))
	old := "package p\n\nfunc init() {}\n\nvar x = 1\n\nfunc init() {}\n"
	res := AnalyzeChange(set, Side{"x.go", []byte(old)}, Side{"x.go", []byte("package p\n\nfunc init() {}\n")}, DefaultOptions())
	// init#0 is on both sides; init#1 only on the old side.
	check(t, "old", show(res.Old), []string{"3-3 init:func init", "7-7 init:func init"})
	check(t, "new", show(res.New), []string{"3-3 init:func init"})
}

func TestAnalyzeChangeDuplicateKeys(t *testing.T) {
	set := ruleSet(funcsWhere("f", func(fd *ast.FuncDecl) bool { return fd.Name.Name == "F" && niladic(fd) }))
	old := "package p\n\nfunc F() {}\n\nfunc F() {}\n"
	res := AnalyzeChange(set, Side{"x.go", []byte(old)}, Side{"x.go", []byte("package p\n\nfunc F() {}\n")}, DefaultOptions())
	check(t, "old", show(res.Old), nil)
	check(t, "new", show(res.New), nil)
	check(t, "diagnostics", codes(res.Diagnostics), []string{"old duplicate-decl  3"})
}

// TestAnalyzeChangeLineShared covers func matches that fail the occupancy
// check: they count as matches for the pairing and stay visible, and so
// does their pair.
func TestAnalyzeChangeLineShared(t *testing.T) {
	set := ruleSet(funcsWhere("f", func(fd *ast.FuncDecl) bool { return fd.Name.Name == "F" && niladic(fd) }))
	plain := "package p\n\nfunc F() {}\n"
	shared := "package p\n\nvar x = 1; func F() {}\n"
	for _, tt := range []struct {
		name     string
		old, new string
		diags    []string
	}{
		{"match/shared", plain, shared, []string{"new line-shared f 3"}},
		{"shared/match", shared, plain, []string{"old line-shared f 3"}},
		{"shared/declared", shared, "package p\n\nfunc F(x int) {}\n", []string{"old line-shared f 3", "old match-changed f 3"}},
		{"shared/absent", shared, "package p\n", []string{"old line-shared f 3"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res := AnalyzeChange(set, Side{"x.go", []byte(tt.old)}, Side{"x.go", []byte(tt.new)}, DefaultOptions())
			check(t, "old", show(res.Old), nil)
			check(t, "new", show(res.New), nil)
			check(t, "diagnostics", codes(res.Diagnostics), tt.diags)
		})
	}
}

func TestAnalyzeChangeRulePathsOneSide(t *testing.T) {
	// A rename moves the file out of the rule's paths. The new side
	// declares Name without the rule applying, so neither side hides it,
	// while the stmt rule works on each side.
	r := funcsNamed("getter", "Name")
	r.Paths = []string{"old/**"}
	src := "package p\n\nfunc (u *User) Name() string { return u.name }\n\nfunc f() {\n\tlog.Debug(\"x\")\n}\n"
	set := ruleSet(r, stmtsMatching(t, "debug", "ExprStmt", `^log\.Debug`))
	res := AnalyzeChange(set, Side{"old/x.go", []byte(src)}, Side{"new/x.go", []byte(src)}, DefaultOptions())
	check(t, "old", show(res.Old), []string{`6-6 debug:log.Debug("x")`})
	check(t, "new", show(res.New), []string{`6-6 debug:log.Debug("x")`})
	check(t, "diagnostics", codes(res.Diagnostics), []string{"old match-changed getter 3"})
}

func TestAnalyzeChangeSkips(t *testing.T) {
	set := ruleSet(userGetters("getter"))
	set.Exclude = []string{"vendor/**"}
	good := []byte(getterOld)
	for _, tt := range []struct {
		name     string
		old, new Side
		want     result.SkipReason
		diags    []string
	}{
		{"rename out of files", Side{"x.go", good}, Side{"vendor/x.go", good}, result.SkipNotTarget, []string{"new skipped  0"}},
		{"rename into files", Side{"x.txt", good}, Side{"x.go", good}, result.SkipNotTarget, []string{"old skipped  0"}},
		{"never a target", Side{"a.txt", good}, Side{"b.txt", good}, result.SkipNotTarget, nil},
		{"added non-target", Side{}, Side{"vendor/x.go", good}, result.SkipNotTarget, nil},
		{"parse error new", Side{"x.go", good}, Side{"x.go", []byte("package p\n\nfunc {\n")}, result.SkipParseError, []string{"new skipped  3"}},
		{"too large old", Side{"x.go", make([]byte, 2<<20)}, Side{"x.go", []byte("package p\n\nfunc {\n")}, result.SkipTooLarge, []string{"old skipped  0"}},
		{"too deep new", Side{"x.go", good}, Side{"x.go", []byte(nestedParens(201))}, result.SkipTooDeep, []string{"new skipped  3"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res := AnalyzeChange(set, tt.old, tt.new, DefaultOptions())
			if res.Skipped != tt.want {
				t.Errorf("Skipped = %q, want %q", res.Skipped, tt.want)
			}
			check(t, "old", show(res.Old), nil)
			check(t, "new", show(res.New), nil)
			check(t, "diagnostics", codes(res.Diagnostics), tt.diags)
			for _, d := range res.Diagnostics {
				if !strings.HasPrefix(d.Message, d.Side+" side skipped ("+string(tt.want)+")") {
					t.Errorf("message %q", d.Message)
				}
			}
		})
	}
}

func TestAnalyzeChangeIndependentNodes(t *testing.T) {
	set := ruleSet(stmtsMatching(t, "debug", "ExprStmt", `^log\.Debug\(`))
	old := "package p\n\nfunc f() {\n\tlog.Debug(\"a\")\n\tlog.Info(\"b\")\n}\n"
	new := "package p\n\nfunc f() {\n\tlog.Info(\"a\")\n\tlog.Debug(\"b\")\n}\n"
	res := AnalyzeChange(set, Side{"x.go", []byte(old)}, Side{"x.go", []byte(new)}, DefaultOptions())
	check(t, "old", show(res.Old), []string{`4-4 debug:log.Debug("a")`})
	check(t, "new", show(res.New), []string{`5-5 debug:log.Debug("b")`})
}

func TestAnalyzeChangeSeveralRules(t *testing.T) {
	// "short" stops applying on the new side while "getter" still
	// matches, so the getter match hides both sides.
	set := ruleSet(funcsNamed("getter", "Name"), funcsNamed("short", "Name"))
	old := "package p\n\nfunc (u *User) Name() string { return u.name }\n"
	res := AnalyzeChange(set, Side{"x.go", []byte(old)}, Side{"x.go", []byte(old)}, DefaultOptions())
	check(t, "same", show(res.New), []string{"3-3 getter:func (*User) Name, short:func (*User) Name"})
	set.Rules[1].Paths = []string{"old/**"}
	res = AnalyzeChange(set, Side{"old/x.go", []byte(old)}, Side{"x.go", []byte(old)}, DefaultOptions())
	check(t, "old", show(res.Old), []string{"3-3 getter:func (*User) Name"})
	check(t, "new", show(res.New), []string{"3-3 getter:func (*User) Name"})
	check(t, "diagnostics", codes(res.Diagnostics), []string{"old match-changed short 3"})
}

func TestAnalyzeChangeJSONArrays(t *testing.T) {
	set := ruleSet(userGetters("getter"))
	for _, tt := range []struct {
		name     string
		old, new Side
	}{
		{"empty", Side{}, Side{}},
		{"skipped", Side{"x.go", []byte("package")}, Side{"x.go", []byte(getterOld)}},
		{"nothing hidden", Side{"x.go", []byte("package p\n")}, Side{"x.go", []byte("package p\n")}},
		{"hidden", Side{Path: "x.go"}, Side{"x.go", []byte(getterOld)}},
	} {
		b, err := json.Marshal(AnalyzeChange(set, tt.old, tt.new, DefaultOptions()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "null") {
			t.Errorf("%s: %s", tt.name, b)
		}
		for _, key := range []string{`"old":[`, `"new":[`, `"diagnostics":[`} {
			if !strings.Contains(string(b), key) {
				t.Errorf("%s: %s has no %s", tt.name, b, key)
			}
		}
	}
}
