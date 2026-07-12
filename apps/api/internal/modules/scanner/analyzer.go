package scanner

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var supportedMethods = map[string]struct{}{
	"GET": {}, "POST": {}, "PUT": {}, "PATCH": {}, "DELETE": {},
}

var skippedDirectories = map[string]struct{}{
	".git": {}, "node_modules": {}, "vendor": {},
}

type SourceLocation struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

type Operation struct {
	Method  string         `json:"method"`
	Path    string         `json:"path"`
	Handler string         `json:"handler"`
	Source  SourceLocation `json:"source"`
}

// Analyze discovers direct Gin-style receiver calls such as r.GET(path, handler).
func Analyze(root string) ([]Operation, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve scan root: %w", err)
	}

	var operations []Operation
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root {
				if _, skip := skippedDirectories[entry.Name()]; skip {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		found, err := analyzeFile(root, path)
		if err != nil {
			return err
		}
		operations = append(operations, found...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", root, err)
	}

	sort.Slice(operations, func(i, j int) bool {
		if operations[i].Source.File != operations[j].Source.File {
			return operations[i].Source.File < operations[j].Source.File
		}
		return operations[i].Source.Line < operations[j].Source.Line
	})
	return operations, nil
}

func analyzeFile(root, path string) ([]Operation, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	relativePath, err := filepath.Rel(root, path)
	if err != nil {
		return nil, fmt.Errorf("make source path relative: %w", err)
	}
	relativePath = filepath.ToSlash(relativePath)

	var operations []Operation
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		method := selector.Sel.Name
		if _, supported := supportedMethods[method]; !supported {
			return true
		}
		pathLiteral, ok := call.Args[0].(*ast.BasicLit)
		if !ok || pathLiteral.Kind != token.STRING {
			return true
		}
		routePath, err := strconv.Unquote(pathLiteral.Value)
		if err != nil {
			return true
		}
		handler, err := printExpression(fset, call.Args[len(call.Args)-1])
		if err != nil {
			return true
		}
		operations = append(operations, Operation{
			Method:  method,
			Path:    routePath,
			Handler: handler,
			Source: SourceLocation{
				File: relativePath,
				Line: fset.Position(call.Pos()).Line,
			},
		})
		return true
	})
	return operations, nil
}

func printExpression(fset *token.FileSet, expression ast.Expr) (string, error) {
	var builder strings.Builder
	if err := printer.Fprint(&builder, fset, expression); err != nil {
		return "", err
	}
	return builder.String(), nil
}
