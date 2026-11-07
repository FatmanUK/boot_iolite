// Command lint enforces the RULES.md code limits: 70 columns with
// 8-space tabs (lines holding string literals are exempt) and
// function bodies of at most 40 lines.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"strings"
)

const maxCols = 70
const maxFuncLines = 40
const tabWidth = 8

func main() {
	files, err := goFiles(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	bad := 0
	for _, f := range files {
		bad += lintFile(f)
	}
	if bad > 0 {
		fmt.Fprintf(os.Stderr, "lint: %d problem(s)\n", bad)
		os.Exit(1)
	}
}

// goFiles expands package patterns with go list.
func goFiles(patterns []string) ([]string, error) {
	tmpl := "{{$d := .Dir}}{{range .GoFiles}}{{$d}}/{{.}}\n{{end}}" +
		"{{range .TestGoFiles}}{{$d}}/{{.}}\n{{end}}"
	args := append([]string{"list", "-f", tmpl}, patterns...)
	out, err := exec.Command("go", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %w", err)
	}
	var files []string
	for _, l := range strings.Split(string(out), "\n") {
		if l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

func lintFile(path string) int {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	exempt := stringLines(fset, f)
	return checkWidth(path, src, exempt) + checkFuncs(fset, f)
}

// stringLines returns the line numbers spanned by string literals.
func stringLines(fset *token.FileSet, f *ast.File) map[int]bool {
	m := map[int]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		from := fset.Position(lit.Pos()).Line
		to := fset.Position(lit.End()).Line
		for i := from; i <= to; i++ {
			m[i] = true
		}
		return true
	})
	return m
}

func checkWidth(path string, src []byte, exempt map[int]bool) int {
	bad := 0
	for i, line := range strings.Split(string(src), "\n") {
		n := i + 1
		w := width(line)
		if exempt[n] || w <= maxCols {
			continue
		}
		fmt.Printf("%s:%d: line is %d columns\n", path, n, w)
		bad++
	}
	return bad
}

func width(line string) int {
	w := 0
	for _, r := range line {
		if r == '\t' {
			w += tabWidth - w%tabWidth
		} else {
			w++
		}
	}
	return w
}

func checkFuncs(fset *token.FileSet, f *ast.File) int {
	bad := 0
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		from := fset.Position(fn.Body.Lbrace).Line
		to := fset.Position(fn.Body.Rbrace).Line
		lines := to - from - 1
		if lines <= maxFuncLines {
			continue
		}
		p := fset.Position(fn.Pos())
		fmt.Printf("%s:%d: func %s body is %d lines\n",
			p.Filename, p.Line, fn.Name.Name, lines)
		bad++
	}
	return bad
}
