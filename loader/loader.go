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

// Loader discovers and parses packages from a project root.
type Loader struct {
	Root     string              // project root directory
	Packages map[string]*Package // import path -> parsed package
	Order    []string            // topological order (dependencies first)
	Errors   []string            // accumulated errors
	loading  map[string]bool     // cycle detection: packages being loaded
}

// New creates a loader with the given project root.
func New(root string) *Loader {
	return &Loader{
		Root:     root,
		Packages: make(map[string]*Package),
		loading:  make(map[string]bool),
	}
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

	// Discover files
	bniPath := filepath.Join(l.Root, path+".bni")
	implDir := filepath.Join(l.Root, path)

	// Parse .bni if it exists
	if data, err := os.ReadFile(bniPath); err == nil {
		p := parser.NewInterface(data, bniPath)
		f := p.ParseFile()
		if len(p.Errors()) > 0 {
			for _, e := range p.Errors() {
				l.Errors = append(l.Errors, e.Error())
			}
			return
		}
		// Validate package name matches path
		declPkg := unquote(f.PkgName.Value)
		if declPkg != path {
			l.Errors = append(l.Errors, fmt.Sprintf(
				"%s: package declaration %q does not match import path %q",
				bniPath, declPkg, path))
			return
		}
		pkg.BNI = f
		pkg.Imports = append(pkg.Imports, extractImports(f)...)
	}

	// Parse .bn implementation files
	entries, err := os.ReadDir(implDir)
	if err != nil && pkg.BNI == nil {
		l.Errors = append(l.Errors, fmt.Sprintf("package %q not found: no %s or %s/",
			path, bniPath, implDir))
		return
	}

	var bnFiles []*ast.File
	if err == nil {
		// Sort entries for deterministic order
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".bn") {
				continue
			}
			filePath := filepath.Join(implDir, entry.Name())
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
			// Validate package name
			declPkg := unquote(f.PkgName.Value)
			if declPkg != path {
				l.Errors = append(l.Errors, fmt.Sprintf(
					"%s: package declaration %q does not match import path %q",
					filePath, declPkg, path))
				return
			}
			bnFiles = append(bnFiles, f)
			pkg.Imports = append(pkg.Imports, extractImports(f)...)
		}
	}

	// Merge .bn files
	if len(bnFiles) > 0 {
		merged, err := MergeFiles(bnFiles)
		if err != nil {
			l.Errors = append(l.Errors, err.Error())
			return
		}
		// Prepend type and const declarations from .bni into the merged .bn,
		// so implementation files can reference types/consts declared in .bni.
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
