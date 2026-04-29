// Package loader discovers, parses, and orders Binate packages.
package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/binate/bootstrap/ast"
	"github.com/binate/bootstrap/parser"
)

// Package represents a parsed Binate package.
type Package struct {
	Path    string    // import path, e.g. "pkg/foo"
	BNI     *ast.File // parsed .bni interface file (nil if none)
	Merged  *ast.File // merged .bn implementation files (nil for builtin)
	Imports []string  // direct import paths
	Builtin bool      // true for Go-backed packages (e.g. pkg/bootstrap)
}

// Loader discovers and parses packages.
//
// Resolution uses two independent, ordered search paths:
//
//   - BniPath: directories searched for `<dir>/<path>.bni` interface
//     files.
//   - ImplPath: directories searched for `<dir>/<path>/` impl
//     directories (a directory provides an impl iff it contains at
//     least one `.bn` file). Future: also `.o`/`.a`/`.so` artifacts.
//
// First hit wins on each path; the two are searched independently, so
// interface and impl can come from different roots.
type Loader struct {
	Root         string              // primary project root directory
	BniPath      []string            // search dirs for .bni interface files
	ImplPath     []string            // search dirs for impl directories
	Packages     map[string]*Package // import path -> parsed package
	Order        []string            // topological order (dependencies first)
	Errors       []string            // accumulated errors
	TestPackages map[string]bool     // packages that should include _test.bn files
	Verbose      bool                // verbose logging to stderr
	loading      map[string]bool     // cycle detection: packages being loaded
}

// New creates a loader with the given project root, seeding both
// BniPath and ImplPath with that root.
func New(root string) *Loader {
	l := &Loader{
		Root:     root,
		Packages: make(map[string]*Package),
		loading:  make(map[string]bool),
	}
	l.AddRoot(root)
	return l
}

// AddRoot appends a directory to BOTH the interface and impl search
// paths. Sugar for AddBniPath(root) + AddImplPath(root); the common
// monorepo case where interfaces and impls share the same tree.
// Skips duplicates on each path independently.
func (l *Loader) AddRoot(root string) {
	l.AddBniPath(root)
	l.AddImplPath(root)
}

// AddBniPath appends a directory to the interface search path,
// skipping duplicates.
func (l *Loader) AddBniPath(dir string) {
	for _, d := range l.BniPath {
		if d == dir {
			return
		}
	}
	l.BniPath = append(l.BniPath, dir)
}

// AddImplPath appends a directory to the impl search path, skipping
// duplicates.
func (l *Loader) AddImplPath(dir string) {
	for _, d := range l.ImplPath {
		if d == dir {
			return
		}
	}
	l.ImplPath = append(l.ImplPath, dir)
}

// RegisterBuiltin registers a builtin package that has no .bn files on disk.
// The .bni is provided as embedded data (already parsed externally).
func (l *Loader) RegisterBuiltin(path string) {
	l.Packages[path] = &Package{
		Path:    path,
		Builtin: true,
	}
}

// LoadImports recursively loads all packages imported by the given import specs.
func (l *Loader) LoadImports(imports []*ast.ImportSpec) {
	for _, imp := range imports {
		path := unquote(imp.Path.Value)
		if _, ok := l.Packages[path]; ok {
			continue // already loaded (or builtin)
		}
		l.loadPackage(path, imp)
	}
	if len(l.Errors) > 0 {
		return
	}
	l.computeOrder()
}

func (l *Loader) loadPackage(path string, imp *ast.ImportSpec) {
	if l.loading[path] {
		l.Errors = append(l.Errors, fmt.Sprintf("import cycle: %s", path))
		return
	}
	l.loading[path] = true
	defer delete(l.loading, path)

	pkg := &Package{Path: path}

	if l.Verbose {
		fmt.Fprintf(os.Stderr, "[verbose] loading package %s\n", path)
	}

	// Two independent search loops: BniPath for the .bni interface,
	// ImplPath for the impl directory. First hit on each wins.
	var bniPath, implDir string
	for _, dir := range l.BniPath {
		candidate := filepath.Join(dir, path+".bni")
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		bniPath = candidate
		p := parser.NewInterface(data, bniPath)
		f := p.ParseFile()
		if len(p.Errors()) > 0 {
			for _, e := range p.Errors() {
				l.Errors = append(l.Errors, e.Error())
			}
			return
		}
		declPkg := unquote(f.PkgName.Value)
		if declPkg != path && declPkg != "main" {
			l.Errors = append(l.Errors, fmt.Sprintf(
				"%s: package declaration %q does not match import path %q",
				bniPath, declPkg, path))
			return
		}
		pkg.BNI = f
		pkg.Imports = append(pkg.Imports, extractImports(f)...)
		break
	}
	for _, dir := range l.ImplPath {
		candidate := filepath.Join(dir, path)
		if _, err := os.ReadDir(candidate); err != nil {
			continue
		}
		implDir = candidate
		break
	}

	if pkg.BNI == nil && implDir == "" {
		// Build error message using primary root
		primaryBni := filepath.Join(l.Root, path+".bni")
		primaryDir := filepath.Join(l.Root, path)
		l.Errors = append(l.Errors, fmt.Sprintf("package %q not found: no %s or %s/",
			path, primaryBni, primaryDir))
		return
	}

	// Parse .bn implementation files
	var bnFiles []*ast.File
	if implDir != "" {
		entries, err := os.ReadDir(implDir)
		if err == nil {
		// Sort entries for deterministic order
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".bn") {
				continue
			}
			// Exclude _test.bn files unless this is a test target package.
			if strings.HasSuffix(name, "_test.bn") && !l.TestPackages[path] {
				continue
			}
			filePath := filepath.Join(implDir, name)
			data, err := os.ReadFile(filePath)
			if err != nil {
				l.Errors = append(l.Errors, fmt.Sprintf("error reading %s: %s", filePath, err))
				return
			}
			p := parser.New(data, filePath)
			f := p.ParseFile()
			if len(p.Errors()) > 0 {
				for _, e := range p.Errors() {
					l.Errors = append(l.Errors, e.Error())
				}
				return
			}
			bnFiles = append(bnFiles, f)
			pkg.Imports = append(pkg.Imports, extractImports(f)...)
		}
		} // err == nil
	} // implDir != ""

	// Validate package names: all files must agree, and must match the import
	// path or all be "main".
	if len(bnFiles) > 0 {
		firstPkg := unquote(bnFiles[0].PkgName.Value)
		for _, f := range bnFiles[1:] {
			declPkg := unquote(f.PkgName.Value)
			if declPkg != firstPkg {
				l.Errors = append(l.Errors, fmt.Sprintf(
					"%s: package declaration %q does not match %q from other files in package",
					f.PkgName.Pos(), declPkg, firstPkg))
				return
			}
		}
		if firstPkg != path && firstPkg != "main" {
			l.Errors = append(l.Errors, fmt.Sprintf(
				"package declaration %q does not match import path %q",
				firstPkg, path))
			return
		}
	}

	if l.Verbose {
		fmt.Fprintf(os.Stderr, "[verbose]   %s: %d .bn files, bni=%v\n", path, len(bnFiles), pkg.BNI != nil)
	}

	// Merge .bn files
	if len(bnFiles) > 0 {
		merged, err := MergeFiles(bnFiles)
		if err != nil {
			l.Errors = append(l.Errors, err.Error())
			return
		}
		// Merge .bni into the implementation: prepend type/const declarations
		// and add any imports the .bni has that the .bn files don't.
		if pkg.BNI != nil {
			var bniDecls []ast.Decl
			for _, d := range pkg.BNI.Decls {
				switch d.(type) {
				case *ast.TypeDecl, *ast.ConstDecl, *ast.GroupDecl:
					bniDecls = append(bniDecls, d)
				}
			}
			if len(bniDecls) > 0 {
				merged.Decls = append(bniDecls, merged.Decls...)
			}
			// Merge .bni imports so the checker/interpreter can resolve
			// qualified types used in the prepended declarations.
			for _, bniImp := range pkg.BNI.Imports {
				found := false
				for _, existing := range merged.Imports {
					if existing.Path.Value == bniImp.Path.Value {
						found = true
						break
					}
				}
				if !found {
					merged.Imports = append(merged.Imports, bniImp)
				}
			}
		}
		pkg.Merged = merged
	}

	// Deduplicate imports
	pkg.Imports = dedup(pkg.Imports)

	l.Packages[path] = pkg

	// Recursively load dependencies
	for _, depPath := range pkg.Imports {
		if _, ok := l.Packages[depPath]; ok {
			continue
		}
		l.loadPackage(depPath, nil)
	}
}

// computeOrder performs a topological sort of loaded packages.
func (l *Loader) computeOrder() {
	visited := make(map[string]bool)
	var order []string

	var visit func(path string)
	visit = func(path string) {
		if visited[path] {
			return
		}
		visited[path] = true
		pkg := l.Packages[path]
		if pkg == nil {
			return
		}
		for _, dep := range pkg.Imports {
			visit(dep)
		}
		order = append(order, path)
	}

	for path := range l.Packages {
		visit(path)
	}
	l.Order = order
}

// MergeFiles combines multiple parsed files from the same package into one.
func MergeFiles(files []*ast.File) (*ast.File, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no files to merge")
	}
	if len(files) == 1 {
		return files[0], nil
	}

	first := files[0]
	pkgName := first.PkgName.Value

	for _, f := range files[1:] {
		if f.PkgName.Value != pkgName {
			return nil, fmt.Errorf("%s: package %s differs from %s in %s",
				f.Pos(), f.PkgName.Value, pkgName, first.Pos())
		}
	}

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

func extractImports(f *ast.File) []string {
	var paths []string
	for _, imp := range f.Imports {
		paths = append(paths, unquote(imp.Path.Value))
	}
	return paths
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func dedup(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
