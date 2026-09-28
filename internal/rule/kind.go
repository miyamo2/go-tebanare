package rule

import "go/ast"

// KindOf returns the go/ast type name of n, such as "IfStmt" or "CallExpr".
// It returns "" for nil and for node types it does not know.
func KindOf(n ast.Node) string {
	switch n.(type) {
	// Statements.
	case *ast.AssignStmt:
		return "AssignStmt"
	case *ast.BadStmt:
		return "BadStmt"
	case *ast.BlockStmt:
		return "BlockStmt"
	case *ast.BranchStmt:
		return "BranchStmt"
	case *ast.CaseClause:
		return "CaseClause"
	case *ast.CommClause:
		return "CommClause"
	case *ast.DeclStmt:
		return "DeclStmt"
	case *ast.DeferStmt:
		return "DeferStmt"
	case *ast.EmptyStmt:
		return "EmptyStmt"
	case *ast.ExprStmt:
		return "ExprStmt"
	case *ast.ForStmt:
		return "ForStmt"
	case *ast.GoStmt:
		return "GoStmt"
	case *ast.IfStmt:
		return "IfStmt"
	case *ast.IncDecStmt:
		return "IncDecStmt"
	case *ast.LabeledStmt:
		return "LabeledStmt"
	case *ast.RangeStmt:
		return "RangeStmt"
	case *ast.ReturnStmt:
		return "ReturnStmt"
	case *ast.SelectStmt:
		return "SelectStmt"
	case *ast.SendStmt:
		return "SendStmt"
	case *ast.SwitchStmt:
		return "SwitchStmt"
	case *ast.TypeSwitchStmt:
		return "TypeSwitchStmt"

	// Expressions.
	case *ast.ArrayType:
		return "ArrayType"
	case *ast.BadExpr:
		return "BadExpr"
	case *ast.BasicLit:
		return "BasicLit"
	case *ast.BinaryExpr:
		return "BinaryExpr"
	case *ast.CallExpr:
		return "CallExpr"
	case *ast.ChanType:
		return "ChanType"
	case *ast.CompositeLit:
		return "CompositeLit"
	case *ast.Ellipsis:
		return "Ellipsis"
	case *ast.FuncLit:
		return "FuncLit"
	case *ast.FuncType:
		return "FuncType"
	case *ast.Ident:
		return "Ident"
	case *ast.IndexExpr:
		return "IndexExpr"
	case *ast.IndexListExpr:
		return "IndexListExpr"
	case *ast.InterfaceType:
		return "InterfaceType"
	case *ast.KeyValueExpr:
		return "KeyValueExpr"
	case *ast.MapType:
		return "MapType"
	case *ast.ParenExpr:
		return "ParenExpr"
	case *ast.SelectorExpr:
		return "SelectorExpr"
	case *ast.SliceExpr:
		return "SliceExpr"
	case *ast.StarExpr:
		return "StarExpr"
	case *ast.StructType:
		return "StructType"
	case *ast.TypeAssertExpr:
		return "TypeAssertExpr"
	case *ast.UnaryExpr:
		return "UnaryExpr"

	// Declarations and other nodes.
	case *ast.FuncDecl:
		return "FuncDecl"
	case *ast.GenDecl:
		return "GenDecl"
	case *ast.BadDecl:
		return "BadDecl"
	case *ast.ValueSpec:
		return "ValueSpec"
	case *ast.TypeSpec:
		return "TypeSpec"
	case *ast.ImportSpec:
		return "ImportSpec"
	case *ast.Field:
		return "Field"
	case *ast.FieldList:
		return "FieldList"
	case *ast.File:
		return "File"
	case *ast.Comment:
		return "Comment"
	case *ast.CommentGroup:
		return "CommentGroup"
	}
	return ""
}

// StmtKinds lists the statement kinds a stmt rule may name in `kind`.
var StmtKinds = []string{
	"AssignStmt", "BlockStmt", "BranchStmt", "CaseClause", "CommClause",
	"DeclStmt", "DeferStmt", "EmptyStmt", "ExprStmt", "ForStmt", "GoStmt",
	"IfStmt", "IncDecStmt", "LabeledStmt", "RangeStmt", "ReturnStmt",
	"SelectStmt", "SendStmt", "SwitchStmt", "TypeSwitchStmt",
}

// ExprKinds lists the expression kinds an expr rule may name in `kind`.
var ExprKinds = []string{
	"ArrayType", "BasicLit", "BinaryExpr", "CallExpr", "ChanType",
	"CompositeLit", "Ellipsis", "FuncLit", "FuncType", "Ident", "IndexExpr",
	"IndexListExpr", "InterfaceType", "KeyValueExpr", "MapType", "ParenExpr",
	"SelectorExpr", "SliceExpr", "StarExpr", "StructType", "TypeAssertExpr",
	"UnaryExpr",
}

// DefaultStmtKinds is the stmt kind filter used when `kind` is omitted:
// every statement except BlockStmt and EmptyStmt.
var DefaultStmtKinds = without(StmtKinds, "BlockStmt", "EmptyStmt")

// DefaultExprKinds is the expr kind filter used when `kind` is omitted:
// every expression except Ident and BasicLit.
var DefaultExprKinds = without(ExprKinds, "Ident", "BasicLit")

func without(list []string, drop ...string) []string {
	var out []string
outer:
	for _, s := range list {
		for _, d := range drop {
			if s == d {
				continue outer
			}
		}
		out = append(out, s)
	}
	return out
}
