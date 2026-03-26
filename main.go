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
		usage()
	}

	// Parse CLI flags
	var root string
	var testMode bool
	var filenames []string
	progArgs := []string{}
	i := 1
	for i < len(os.Args) {
		arg := os.Args[i]
		if arg == "--" {
			progArgs = os.Args[i+1:]
			break
		}
		if arg == "-root" && i+1 < len(os.Args) {
			root = os.Args[i+1]
			i += 2
			continue
		}
		if arg == "-test" {
			testMode = true
			i++
			continue
		}
		filenames = append(filenames, arg)
		i++
	}
	if len(filenames) == 0 {
		usage()
	}

	if testMode {
		runTests(root, filenames)
	} else {
		runProgram(root, filenames, progArgs)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: binate [-root dir] <file.bn> [file2.bn ...] [-- args...]\n")
	fmt.Fprintf(os.Stderr, "       binate -test [-root dir] <pkg/foo> [pkg/bar ...]\n")
	os.Exit(1)
}

// runTests runs Test* functions in the specified packages.
func runTests(root string, testPkgs []string) {
	var err error
	if root == "" {
		root, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			os.Exit(1)
		}
	}

	// Set up loader with test packages enabled
	ldr := loader.New(root)
	ldr.RegisterBuiltin("pkg/bootstrap")
	ldr.TestPackages = make(map[string]bool)
	for _, pkg := range testPkgs {
		ldr.TestPackages[pkg] = true
	}

	// Create synthetic imports to load test packages and their dependencies
	var syntheticImports []*ast.ImportSpec
	for _, pkg := range testPkgs {
		syntheticImports = append(syntheticImports, &ast.ImportSpec{
			Path: &ast.StringLit{Value: `"` + pkg + `"`},
		})
	}
	ldr.LoadImports(syntheticImports)
	if len(ldr.Errors) > 0 {
		for _, e := range ldr.Errors {
			fmt.Fprintf(os.Stderr, "%s\n", e)
		}
		os.Exit(1)
	}

	// Type check all packages in dependency order
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
	if len(c.Errors()) > 0 {
		for _, e := range c.Errors() {
			fmt.Fprintf(os.Stderr, "%s\n", e)
		}
		os.Exit(1)
	}

	// Load packages in interpreter
	interp := interpreter.New()
	for _, pkgPath := range ldr.Order {
		pkg := ldr.Packages[pkgPath]
		if pkg.Builtin || pkg.Merged == nil {
			continue
		}
		interp.LoadPackage(pkgPath, pkg.Merged, c)
	}

	// Discover and run Test* functions
	passed, failed := 0, 0
	for _, pkgPath := range testPkgs {
		pkg := ldr.Packages[pkgPath]
		if pkg == nil || pkg.Merged == nil {
			fmt.Fprintf(os.Stderr, "warning: no implementation for %s\n", pkgPath)
			continue
		}

		var testNames []string
		for _, d := range pkg.Merged.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if strings.HasPrefix(fd.Name.Name, "Test") && len(fd.Params) == 0 && fd.Body != nil {
				testNames = append(testNames, fd.Name.Name)
			}
		}

		if len(testNames) == 0 {
			fmt.Printf("?   \t%s\t[no test functions]\n", pkgPath)
			continue
		}

		pkgFailed := 0
		for _, name := range testNames {
			fmt.Printf("=== RUN   %s\n", name)
			errMsg := interp.RunTestFunc(pkgPath, name)
			if errMsg != "" {
				fmt.Printf("--- FAIL: %s\n    %s\n", name, errMsg)
				failed++
				pkgFailed++
			} else {
				fmt.Printf("--- PASS: %s\n", name)
				passed++
			}
		}

		if pkgFailed > 0 {
			fmt.Printf("FAIL\t%s\n", pkgPath)
		} else {
			fmt.Printf("ok  \t%s\t%d tests\n", pkgPath, len(testNames))
		}
	}

	// Summary
	fmt.Println()
	if failed > 0 {
		fmt.Printf("FAIL\t%d passed, %d failed\n", passed, failed)
		os.Exit(1)
	}
	fmt.Printf("ok\t%d passed\n", passed)
}

// runProgram runs a Binate program (the normal non-test mode).
func runProgram(root string, filenames []string, progArgs []string) {
	// Validate all files are in the same directory
	if len(filenames) > 1 {
		dir0, err := filepath.Abs(filepath.Dir(filenames[0]))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			os.Exit(1)
		}
		for _, f := range filenames[1:] {
			dir, err := filepath.Abs(filepath.Dir(f))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %s\n", err)
				os.Exit(1)
			}
			if dir != dir0 {
				fmt.Fprintf(os.Stderr, "error: all source files must be in the same directory\n")
				fmt.Fprintf(os.Stderr, "  %s is in %s\n", filenames[0], dir0)
				fmt.Fprintf(os.Stderr, "  %s is in %s\n", f, dir)
				os.Exit(1)
			}
		}
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

	// Project root: -root flag, or current working directory
	if root == "" {
		root, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			os.Exit(1)
		}
	}

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
