package astmatch

import (
	"go/ast"
	"go/parser"
	"strings"
	"testing"
)

// pat parses a pattern type written in Go syntax. The identifier "_"
// becomes Any, "SEQ" becomes Seq, and "TP_X" becomes TParam("X").
func pat(t *testing.T, src string) ast.Expr {
	t.Helper()
	e := typeExpr(t, src)
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			switch {
			case id.Name == "_":
				id.Name = AnyName
			case id.Name == "SEQ":
				id.Name = SeqName
			case strings.HasPrefix(id.Name, "TP_"):
				id.Name = TParamPrefix + strings.TrimPrefix(id.Name, "TP_")
			}
		}
		return true
	})
	return e
}

// typeExpr parses a source type. A leading "..." yields the *ast.Ellipsis
// of a variadic parameter.
func typeExpr(t *testing.T, src string) ast.Expr {
	t.Helper()
	if strings.HasPrefix(src, "...") {
		ft := typeExpr(t, "func("+src+")").(*ast.FuncType)
		return ft.Params.List[0].Type
	}
	e, err := parser.ParseExpr(src)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return e
}

func TestSentinels(t *testing.T) {
	if !IsAny(Any()) || IsAny(Seq()) || IsAny(ast.NewIdent("_")) {
		t.Error("IsAny")
	}
	if !IsSeq(Seq()) || IsSeq(Any()) || IsSeq(&ast.Ellipsis{}) {
		t.Error("IsSeq")
	}
	if name, ok := TParamName(TParam("T")); !ok || name != "T" {
		t.Errorf("TParamName(TParam(T)) = %q, %v", name, ok)
	}
	if _, ok := TParamName(ast.NewIdent("T")); ok {
		t.Error("TParamName(T) reported a reference")
	}
	var nilIdent *ast.Ident
	if IsAny(nilIdent) || IsSeq(nil) {
		t.Error("nil is a sentinel")
	}
}

// TestPatSentinels checks the spelling of sentinels that the tests of this
// package use in pattern types.
func TestPatSentinels(t *testing.T) {
	m := pat(t, "map[_]func(SEQ) TP_K").(*ast.MapType)
	ft := m.Value.(*ast.FuncType)
	name, ok := TParamName(ft.Results.List[0].Type)
	if !IsAny(m.Key) || !IsSeq(ft.Params.List[0].Type) || !ok || name != "K" {
		t.Errorf("pat gave key %v, params %v, result %v", m.Key, ft.Params.List[0].Type, ft.Results.List[0].Type)
	}
	if _, ok := typeExpr(t, "...int").(*ast.Ellipsis); !ok {
		t.Error(`typeExpr("...int") is not an *ast.Ellipsis`)
	}
}
