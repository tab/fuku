// Command comments fails when a comment breaks the comment rules in CLAUDE.md
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// markers are the prefixes a comment inside a function body must carry
var markers = []string{"no-op:", "sync:", "perf:", "ponytail:"}

// directives are the comment prefixes the tools own, so the rules skip them
var directives = []string{"//nolint", "//go:", "// #nosec", "//#nosec", "// Code generated"}

func main() {
	roots := os.Args[1:]
	if len(roots) == 0 {
		roots = []string{"internal", "cmd", "e2e"}
	}

	fset := token.NewFileSet()
	failed := false

	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_mock_test.go") {
				return nil
			}

			for _, problem := range check(fset, path) {
				failed = true

				fmt.Println(problem)
			}

			return nil
		})
	}

	if failed {
		os.Exit(1)
	}
}

// check returns one line per comment in the file that breaks a rule
func check(fset *token.FileSet, path string) []string {
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", path, err)}
	}

	docs := map[*ast.CommentGroup]bool{file.Doc: true}
	trailing := map[*ast.CommentGroup]bool{}

	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			docs[x.Doc] = true
		case *ast.GenDecl:
			docs[x.Doc] = true
		case *ast.TypeSpec:
			docs[x.Doc], trailing[x.Comment] = true, true
		case *ast.ValueSpec:
			docs[x.Doc], trailing[x.Comment] = true, true
		case *ast.Field:
			docs[x.Doc], trailing[x.Comment] = true, true
		case *ast.ImportSpec:
			docs[x.Doc], trailing[x.Comment] = true, true
		}

		return true
	})

	var problems []string

	for _, group := range file.Comments {
		lines := prose(group)
		if len(lines) == 0 {
			continue
		}

		text := lines[0]

		where := fset.Position(group.Pos())
		report := func(msg string) {
			problems = append(problems, fmt.Sprintf("%s:%d: %s", where.Filename, where.Line, msg))
		}

		godoc := docs[group] || trailing[group]

		switch {
		case len(lines) > 1:
			report("a comment is one line")
		case godoc && utf8.RuneCountInString(text) > 120:
			report("a godoc line is at most 120 characters")
		case godoc:
		case inBody(file, group):
			if !hasPrefix(strings.TrimPrefix(text, "// "), markers) {
				report("a comment inside a function body starts with no-op:, sync:, perf: or ponytail:, or goes")
			}
		default:
			report("a comment between declarations belongs on a declaration or goes")
		}
	}

	return problems
}

// prose returns the comment lines that are neither directives nor empty
func prose(group *ast.CommentGroup) []string {
	var lines []string

	for _, comment := range group.List {
		if comment.Text == "//" || hasPrefix(comment.Text, directives) {
			continue
		}

		lines = append(lines, comment.Text)
	}

	return lines
}

// inBody reports whether the comment sits inside a function body
func inBody(file *ast.File, group *ast.CommentGroup) bool {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Body != nil && group.Pos() > fn.Body.Pos() && group.End() < fn.Body.End() {
			return true
		}
	}

	return false
}

// hasPrefix reports whether text starts with one of the prefixes
func hasPrefix(text string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}

	return false
}
