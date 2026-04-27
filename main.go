package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
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
	var addRoots []string
	var testMode bool
	var verbose bool
	var cpuProfile string
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
		if arg == "-add-root" && i+1 < len(os.Args) {
			addRoots = append(addRoots, os.Args[i+1])
			i += 2
			continue
		}
		if arg == "-test" {
			testMode = true
			i++
			continue
		}
		if arg == "-v" || arg == "-verbose" {
			verbose = true
			i++
			continue
		}
		if arg == "-cpuprofile" && i+1 < len(os.Args) {
			cpuProfile = os.Args[i+1]
			i += 2
			continue
		}
		filenames = append(filenames, arg)
		i++
	}
	// Expand directory arguments for non-test mode.
	// Test mode handles directories separately via runDirTests.
	if !testMode {
		filenames = expandDirArgs(filenames)
	}

	if len(filenames) == 0 {
		usage()
	}

	// CPU profiling
	if cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating profile: %s\n", err)
			os.Exit(1)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "error starting profile: %s\n", err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	if testMode {
		// Check if any argument is a directory (main package test mode)
		var dirArgs []string
		var pkgArgs []string
		for _, arg := range filenames {
			info, err := os.Stat(arg)
			if err == nil && info.IsDir() {
				dirArgs = append(dirArgs, arg)
			} else {
				pkgArgs = append(pkgArgs, arg)
			}
		}
		if len(dirArgs) > 0 && len(pkgArgs) > 0 {
			fmt.Fprintf(os.Stderr, "error: cannot mix directory and package path arguments in -test mode\n")
			os.Exit(1)
		}
		if len(dirArgs) > 0 {
			for _, dir := range dirArgs {
				runDirTests(root, addRoots, dir, verbose)
			}
		} else {
			// Validate that test arguments are package paths, not file paths
			for _, arg := range pkgArgs {
				if strings.HasSuffix(arg, ".bn") || strings.HasSuffix(arg, ".bni") {
					fmt.Fprintf(os.Stderr, "error: -test takes package paths, not files: %s\n", arg)
					fmt.Fprintf(os.Stderr, "  use: binate -test [-root dir] pkg/foo\n")
					os.Exit(1)
				}
				if filepath.IsAbs(arg) {
					fmt.Fprintf(os.Stderr, "error: -test takes package paths (e.g. pkg/foo), not absolute paths: %s\n", arg)
					os.Exit(1)
				}
			}
			runTests(root, addRoots, pkgArgs, verbose)
		}
	} else {
		runProgram(root, addRoots, filenames, progArgs, verbose)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: binate [-v] [-root dir] <file.bn|dir> [file2.bn ...] [-- args...]\n")
	fmt.Fprintf(os.Stderr, "       binate [-v] -test [-root dir] <pkg/foo> [pkg/bar ...]\n")
	os.Exit(1)
}

// expandDirArgs expands any directory arguments into the .bn files they contain,
// excluding _test.bn files. Non-directory arguments are passed through unchanged.
func expandDirArgs(args []string) []string {
	var result []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil || !info.IsDir() {
			result = append(result, arg)
			continue
		}
		entries, err := os.ReadDir(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading directory %s: %s\n", arg, err)
			os.Exit(1)
		}
		var found int
		for _, e := range entries {
			name := e.Name()
			if strings.HasSuffix(name, ".bn") && !strings.HasSuffix(name, "_test.bn") {
				result = append(result, filepath.Join(arg, name))
				found++
			}
		}
		if found == 0 {
			fmt.Fprintf(os.Stderr, "error: directory %s contains no .bn files\n", arg)
			os.Exit(1)
		}
	}
	return result
}

// runDirTests runs Test* functions in a main package directory.
// It loads all .bn files (including _test.bn) from the directory.
func runDirTests(root string, addRoots []string, dir string, verbose bool) {
	// Collect all .bn files in the directory (including _test.bn)
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading directory %s: %s\n", dir, err)
		os.Exit(1)
	}
	var filenames []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".bn") {
			filenames = append(filenames, filepath.Join(dir, name))
		}
	}
	if len(filenames) == 0 {
		fmt.Fprintf(os.Stderr, "error: directory %s contains no .bn files\n", dir)
		os.Exit(1)
	}

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

	// Merge
	merged, err := loader.MergeFiles(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	// Project root
	if root == "" {
		root, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			os.Exit(1)
		}
	}

	// Load imports
	ldr := loader.New(root)
	for _, ar := range addRoots {
		ldr.AddRoot(ar)
	}
	ldr.Verbose = verbose
	ldr.RegisterBuiltin("pkg/bootstrap")
	ldr.LoadImports(merged.Imports)
	if len(ldr.Errors) > 0 {
		for _, e := range ldr.Errors {
			fmt.Fprintf(os.Stderr, "%s\n", e)
		}
		os.Exit(1)
	}

	// Type check imported packages
	c := types.NewChecker()
	c.Verbose = verbose
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
	// Type check the main package
	c.CheckPackage("main", merged)
	if len(c.Errors()) > 0 {
		for _, e := range c.Errors() {
			fmt.Fprintf(os.Stderr, "%s\n", e)
		}
		os.Exit(1)
	}

	// Load packages in interpreter
	interp := interpreter.New()
	interp.Verbose = verbose
	for _, pkgPath := range ldr.Order {
		pkg := ldr.Packages[pkgPath]
		if pkg.Builtin || pkg.Merged == nil {
			continue
		}
		interp.LoadPackage(pkgPath, pkg.Merged, c)
	}
	// Load the main package
	interp.LoadPackage("main", merged, c)

	// Discover and run Test* functions
	var testNames []string
	for _, d := range merged.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if !strings.HasPrefix(fd.Name.Name, "Test") || fd.Body == nil {
			continue
		}
		if fd.Recv != nil {
			continue // methods can't be top-level test functions
		}
		if len(fd.Params) == 0 && isTestResultReturn(fd) {
			testNames = append(testNames, fd.Name.Name)
		} else {
			fmt.Fprintf(os.Stderr, "warning: %s has Test prefix but wrong signature (want TestXxx() testing.TestResult)\n", fd.Name.Name)
		}
	}

	if len(testNames) == 0 {
		fmt.Printf("?   \t%s\t[no test functions]\n", dir)
		return
	}

	passed, failed := 0, 0
	for _, name := range testNames {
		fmt.Printf("=== RUN   %s\n", name)
		errMsg := interp.RunTestFunc("main", name)
		if errMsg != "" {
			fmt.Printf("--- FAIL: %s\n    %s\n", name, errMsg)
			failed++
		} else {
			fmt.Printf("--- PASS: %s\n", name)
			passed++
		}
	}

	fmt.Println()
	if failed > 0 {
		fmt.Printf("FAIL\t%d passed, %d failed\n", passed, failed)
		os.Exit(1)
	}
	fmt.Printf("ok\t%d passed\n", passed)
}

// runTests runs Test* functions in the specified packages.
func runTests(root string, addRoots []string, testPkgs []string, verbose bool) {
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
	for _, ar := range addRoots {
		ldr.AddRoot(ar)
	}
	ldr.Verbose = verbose
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

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] loaded %d packages in order: %v\n", len(ldr.Order), ldr.Order)
	}

	// Type check all packages in dependency order
	c := types.NewChecker()
	c.Verbose = verbose
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
	interp.Verbose = verbose
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
			if !strings.HasPrefix(fd.Name.Name, "Test") || fd.Body == nil {
				continue
			}
			if len(fd.Params) == 0 && isTestResultReturn(fd) {
				testNames = append(testNames, fd.Name.Name)
			} else {
				fmt.Fprintf(os.Stderr, "warning: %s has Test prefix but wrong signature (want TestXxx() testing.TestResult)\n", fd.Name.Name)
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

// isTestResultReturn checks whether a function has a single return type
// that is testing.TestResult (or equivalently @[]char).
func isTestResultReturn(fd *ast.FuncDecl) bool {
	if len(fd.Results) != 1 {
		return false
	}
	r := fd.Results[0]
	// Accept testing.TestResult (qualified named type)
	if nt, ok := r.(*ast.NamedType); ok {
		if nt.Pkg != nil && nt.Pkg.Name == "testing" && nt.Name.Name == "TestResult" {
			return true
		}
	}
	// Accept @[]char directly
	if ms, ok := r.(*ast.ManagedSliceType); ok {
		if nt, ok := ms.Elem.(*ast.NamedType); ok {
			if nt.Pkg == nil && nt.Name.Name == "char" {
				return true
			}
		}
	}
	// Accept []char directly (legacy)
	if st, ok := r.(*ast.SliceType); ok {
		if nt, ok := st.Elem.(*ast.NamedType); ok {
			if nt.Pkg == nil && nt.Name.Name == "char" {
				return true
			}
		}
	}
	return false
}

// runProgram runs a Binate program (the normal non-test mode).
func runProgram(root string, addRoots []string, filenames []string, progArgs []string, verbose bool) {
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
	for _, ar := range addRoots {
		ldr.AddRoot(ar)
	}
	ldr.Verbose = verbose
	ldr.RegisterBuiltin("pkg/bootstrap")
	ldr.LoadImports(merged.Imports)
	if len(ldr.Errors) > 0 {
		for _, e := range ldr.Errors {
			fmt.Fprintf(os.Stderr, "%s\n", e)
		}
		os.Exit(1)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] loaded %d packages in order: %v\n", len(ldr.Order), ldr.Order)
	}

	// Type check: packages in dependency order, then main
	c := types.NewChecker()
	c.Verbose = verbose
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

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] type checking passed\n")
	}

	// Run: load packages in dependency order, then run main
	interp := interpreter.New()
	interp.Verbose = verbose
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
