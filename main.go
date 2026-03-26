package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/binate/bootstrap/ast"
	"github.com/binate/bootstrap/interpreter"
	"github.com/binate/bootstrap/loader"
	"github.com/binate/bootstrap/parser"
	"github.com/binate/bootstrap/types"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: binate <file.bn> [file2.bn ...] [-- args...]\n")
		os.Exit(1)
	}

	// Split args at "--": files before, program args after
	var filenames []string
	progArgs := []string{}
	seenSep := false
	for _, arg := range os.Args[1:] {
		if arg == "--" {
			seenSep = true
			continue
		}
		if seenSep {
			progArgs = append(progArgs, arg)
		} else {
			filenames = append(filenames, arg)
		}
	}
	if len(filenames) == 0 {
		fmt.Fprintf(os.Stderr, "usage: binate <file.bn> [file2.bn ...] [-- args...]\n")
		os.Exit(1)
	}

	// Parse all main package files
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

	// Merge main package files
	merged, err := loader.MergeFiles(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	// Determine project root from first file's path and package declaration
	root := inferRoot(filenames[0], merged.PkgName.Value)

	// Load all imported packages
	ldr := loader.New(root)
	ldr.RegisterBuiltin("pkg/bootstrap")
	ldr.LoadImports(merged.Imports)
	if len(ldr.Errors) > 0 {
		for _, e := range ldr.Errors {
			fmt.Fprintf(os.Stderr, "%s\n", e)
		}
		os.Exit(1)
	}

	// Type check: packages in dependency order, then main
	c := types.NewChecker()
	for _, pkgPath := range ldr.Order {
		pkg := ldr.Packages[pkgPath]
		if pkg.Builtin {
			continue
		}
		if pkg.BNI != nil {
			c.LoadPackageInterface(pkgPath, pkg.BNI)
		}
		if pkg.Merged != nil {
			c.CheckPackage(pkgPath, pkg.Merged)
		}
	}
	c.Check(merged)
	if len(c.Errors()) > 0 {
		for _, e := range c.Errors() {
			fmt.Fprintf(os.Stderr, "%s\n", e)
		}
		os.Exit(1)
	}

	// Run: load packages in dependency order, then run main
	interp := interpreter.New()
	interp.SetArgs(progArgs)
	for _, pkgPath := range ldr.Order {
		pkg := ldr.Packages[pkgPath]
		if pkg.Builtin || pkg.Merged == nil {
			continue
		}
		interp.LoadPackage(pkgPath, pkg.Merged, c)
	}
	runWithRecovery(interp, merged, c)
}

// inferRoot determines the project root from a source file path and its
// package declaration. For example, if the file is at /project/cmd/app/main.bn
// and declares package "cmd/app", root is /project. For package "main",
// root is the directory containing the file.
func inferRoot(filename string, pkgName string) string {
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return filepath.Dir(filename)
	}
	dir := filepath.Dir(absPath)

	// Strip quotes from package name
	pkg := pkgName
	if len(pkg) >= 2 && pkg[0] == '"' && pkg[len(pkg)-1] == '"' {
		pkg = pkg[1 : len(pkg)-1]
	}

	// For "main" package, root is the directory of the file
	if pkg == "main" {
		return dir
	}

	// For other packages, walk up from dir by the number of path components
	parts := strings.Split(pkg, "/")
	root := dir
	for range parts {
		root = filepath.Dir(root)
	}
	return root
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
