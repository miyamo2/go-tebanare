package analyzer

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"

	"github.com/miyamo2/go-tebanare/internal/canon"
	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// parsed is a file that passed the size, nesting, and syntax checks.
type parsed struct {
	path  string
	info  *scanInfo
	fset  *token.FileSet
	file  *ast.File
	rf    *rule.File
	cache *canon.Cache
}

// prepare runs the checks of plan 4.7 steps 1 to 4 and parses src.
func prepare(path string, src []byte, opt Options) (*parsed, *skip) {
	if sk := checkSize(src, opt); sk != nil {
		return nil, sk
	}
	info, sk := scanFile(src, opt)
	if sk != nil {
		return nil, sk
	}
	fset, file, sk := parseFile(path, src, info)
	if sk != nil {
		return nil, sk
	}
	if n := deepNode(file, opt.MaxASTDepth); n != nil {
		pos := fset.PositionFor(startPos(n), false)
		return nil, &skip{
			reason: result.SkipTooDeep, line: pos.Line, column: pos.Column,
			msg: fmt.Sprintf("the syntax tree is more than %d levels deep", opt.MaxASTDepth),
		}
	}
	cache := canon.NewCache(fset, opt.MaxNodeSize)
	rf := &rule.File{Path: path, Fset: fset, AST: file, Src: src}
	return &parsed{path: path, info: info, fset: fset, file: file, rf: rf, cache: cache}, nil
}

// parseFile parses src. It uses parser.AllErrors because without it the
// parser stops after ten errors with a panic that it recovers itself, and
// TinyGo's wasm targets cannot recover.
func parseFile(path string, src []byte, info *scanInfo) (*token.FileSet, *ast.File, *skip) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments|parser.SkipObjectResolution|parser.AllErrors)
	if err == nil {
		return fset, file, nil
	}
	sk := &skip{reason: result.SkipParseError, msg: err.Error()}
	var list scanner.ErrorList
	if errors.As(err, &list) && len(list) > 0 {
		// The list is sorted by positions that follow //line directives.
		// Report the first error in the file itself.
		first := list[0]
		for _, e := range list[1:] {
			if e.Pos.Offset < first.Pos.Offset {
				first = e
			}
		}
		sk.line, sk.column = info.lineCol(first.Pos.Offset)
		sk.msg = first.Msg
	}
	return nil, nil, sk
}
