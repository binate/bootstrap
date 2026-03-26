package main

import (
	"fmt"
	"os"

	"github.com/binate/bootstrap/ast"
	"github.com/binate/bootstrap/interpreter"
	"github.com/binate/bootstrap/parser"
	"github.com/binate/bootstrap/types"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: binate <file.bn> [file2.bn ...]\n")
		os.Exit(1)
	}

	filenames := os.Args[1:]

	// Parse all files
	var files []*ast.File
	hasErrors := false
	for _, filename := range filenames {
		src, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			os.Exit(1)
		}
		p := parser.New(src, filename)
		f := p.ParseFile()
		if len(p.Errors()) > 0 {
			for _, e := range p.Errors() {
				fmt.Fprintf(os.Stderr, "%s\n", e)
			}
			hasErrors = true
		}
		files = append(files, f)
	}
	if hasErrors {
		os.Exit(1)
	}

	// Merge files into a single AST
	merged, err := mergeFiles(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	// Type check
	c := types.NewChecker()
	c.Check(merged)
	if len(c.Errors()) > 0 {
		for _, e := range c.Errors() {
			fmt.Fprintf(os.Stderr, "%s\n", e)
		}
		os.Exit(1)
	}

	// Run
	interp := interpreter.New()
	runWithRecovery(interp, merged, c)
}

// mergeFiles combines multiple parsed files from the same package into one.
// All files must declare the same package name. Imports are deduplicated,
// declarations are concatenated in file order.
func mergeFiles(files []*ast.File) (*ast.File, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no files to merge")
	}
	if len(files) == 1 {
		return files[0], nil
	}

	first := files[0]
	pkgName := first.PkgName.Value

	// Verify all files declare the same package
	for _, f := range files[1:] {
		if f.PkgName.Value != pkgName {
			return nil, fmt.Errorf("%s: package %s differs from %s in %s",
				f.Pos(), f.PkgName.Value, pkgName, first.Pos())
		}
	}

	// Merge imports (deduplicate by path+alias)
	type importKey struct {
		alias string
		path  string
	}
	seen := make(map[importKey]bool)
	var imports []*ast.ImportSpec
	for _, f := range files {
		for _, imp := range f.Imports {
			key := importKey{alias: imp.Alias, path: imp.Path.Value}
			if !seen[key] {
				seen[key] = true
				imports = append(imports, imp)
			}
		}
	}

	// Concatenate declarations
	var decls []ast.Decl
	for _, f := range files {
		decls = append(decls, f.Decls...)
	}

	return &ast.File{
		Package: first.Package,
		PkgName: first.PkgName,
		Imports: imports,
		Decls:   decls,
	}, nil
}

func runWithRecovery(interp *interpreter.Interpreter, f *ast.File, c *types.Checker) {
	defer func() {
		if r := recover(); r != nil {
			if re, ok := r.(*interpreter.RuntimeError); ok {
				fmt.Fprintf(os.Stderr, "%s\n", re.Error())
			} else {
				fmt.Fprintf(os.Stderr, "runtime error: %v\n", r)
			}
			os.Exit(2)
		}
	}()
	interp.Run(f, c)
}
