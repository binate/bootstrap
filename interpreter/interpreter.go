package interpreter

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/binate/bootstrap/ast"
	"github.com/binate/bootstrap/token"
	"github.com/binate/bootstrap/types"
)

// RuntimeError represents a runtime error with source position.
type RuntimeError struct {
	Pos token.Pos
	Msg string
}

func (e *RuntimeError) Error() string {
	if e.Pos.File != "" {
		return fmt.Sprintf("%s: runtime error: %s", e.Pos, e.Msg)
	}
	return fmt.Sprintf("runtime error: %s", e.Msg)
}

func runtimePanic(pos token.Pos, format string, args ...interface{}) {
	panic(&RuntimeError{Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

// Interpreter evaluates a Binate AST.
type Interpreter struct {
	env           *Env
	funcs         map[string]*ast.FuncDecl
	types         map[string]types.Type
	stdout        *strings.Builder                 // captured output (nil = write to os.Stdout)
	files         map[int]*os.File                 // open file descriptors
	nextFD        int                              // next file descriptor to allocate
	packages      map[string]*Env                  // package path -> env
	packageTypes  map[string]map[string]types.Type // package path -> type map
	importAliases map[string]string                // local name -> package path
	progArgs      []string                         // program arguments (from -- separator)
	iota          int                              // current iota value in grouped const (-1 = not in const group)
}

// Env represents a variable environment (frame).
type Env struct {
	parent *Env
	vars   map[string]*HeapObject
}

func newEnv(parent *Env) *Env {
	return &Env{parent: parent, vars: make(map[string]*HeapObject)}
}

func (e *Env) get(name string) (Value, bool) {
	if cell, ok := e.vars[name]; ok {
		return cell.Val, true
	}
	if e.parent != nil {
		return e.parent.get(name)
	}
	return nil, false
}

func (e *Env) set(name string, val Value) bool {
	if cell, ok := e.vars[name]; ok {
		cell.Val = val
		return true
	}
	if e.parent != nil {
		return e.parent.set(name, val)
	}
	return false
}

func (e *Env) define(name string, val Value) {
	e.vars[name] = &HeapObject{Val: val}
}

// getCell returns the backing HeapObject for a variable, used by &x.
func (e *Env) getCell(name string) *HeapObject {
	if cell, ok := e.vars[name]; ok {
		return cell
	}
	if e.parent != nil {
		return e.parent.getCell(name)
	}
	return nil
}

// signalReturn is used to unwind the call stack on return.
type signalReturn struct {
	vals []Value
}

// signalBreak is used to break out of loops.
type signalBreak struct{}

// signalContinue is used to continue to the next loop iteration.
type signalContinue struct{}

// New creates a new interpreter.
func New() *Interpreter {
	interp := &Interpreter{
		funcs:         make(map[string]*ast.FuncDecl),
		types:         make(map[string]types.Type),
		files:         map[int]*os.File{0: os.Stdin, 1: os.Stdout, 2: os.Stderr},
		nextFD:        3,
		packages:      make(map[string]*Env),
		packageTypes:  make(map[string]map[string]types.Type),
		importAliases: make(map[string]string),
		iota:          -1,
	}
	interp.env = newEnv(nil)
	interp.registerBuiltins()
	interp.registerBootstrapPackage()
	return interp
}

// SetStdout captures output to a string builder instead of os.Stdout.
func (interp *Interpreter) SetStdout(buf *strings.Builder) {
	interp.stdout = buf
}

// SetArgs sets the program arguments returned by bootstrap.args().
func (interp *Interpreter) SetArgs(args []string) {
	interp.progArgs = args
}

func (interp *Interpreter) print(s string) {
	if interp.stdout != nil {
		interp.stdout.WriteString(s)
	} else {
		fmt.Fprint(os.Stdout, s)
	}
}

func (interp *Interpreter) registerBuiltins() {
	// print: prints values without newline
	interp.env.define("print", &BuiltinFuncVal{
		Name: "print",
		Fn: func(args []Value) Value {
			for i, a := range args {
				if i > 0 {
					interp.print(" ")
				}
				interp.print(a.String())
			}
			return &NilVal{}
		},
	})
	// println: prints values with newline
	interp.env.define("println", &BuiltinFuncVal{
		Name: "println",
		Fn: func(args []Value) Value {
			for i, a := range args {
				if i > 0 {
					interp.print(" ")
				}
				interp.print(a.String())
			}
			interp.print("\n")
			return &NilVal{}
		},
	})
	// append: append(slice, elems...) -> new slice
	interp.env.define("append", &BuiltinFuncVal{
		Name: "append",
		Fn: func(args []Value) Value {
			if len(args) < 2 {
				panic("append requires at least 2 arguments")
			}
			sv, ok := args[0].(*SliceVal)
			if !ok {
				panic(fmt.Sprintf("append: first argument must be a slice, got %T", args[0]))
			}
			newElems := make([]Value, len(sv.Elems), len(sv.Elems)+len(args)-1)
			copy(newElems, sv.Elems)
			for _, a := range args[1:] {
				newElems = append(newElems, a)
			}
			return &SliceVal{Elems: newElems, Typ: sv.Typ}
		},
	})
	// panic
	interp.env.define("panic", &BuiltinFuncVal{
		Name: "panic",
		Fn: func(args []Value) Value {
			msg := "panic"
			if len(args) > 0 {
				msg = args[0].String()
			}
			panic(msg)
		},
	})
}

func (interp *Interpreter) registerBootstrapPackage() {
	pkg := newEnv(nil)

	// stringToGo converts a Binate string (StringVal or []char SliceVal) to a Go string.
	stringToGo := func(v Value) string {
		if sv, ok := v.(*StringVal); ok {
			return sv.Val
		}
		if sv, ok := v.(*SliceVal); ok {
			buf := make([]byte, len(sv.Elems))
			for i, e := range sv.Elems {
				switch ev := e.(type) {
				case *IntVal:
					buf[i] = byte(ev.Val)
				case *CharVal:
					buf[i] = byte(ev.Val)
				}
			}
			return string(buf)
		}
		return v.String()
	}

	// goToCharSlice converts a Go string to a Binate []char SliceVal.
	goToCharSlice := func(s string) Value {
		return &StringVal{Val: s}
	}

	// Exit: Exit(code int)
	pkg.define("Exit", &BuiltinFuncVal{
		Name: "Exit",
		Fn: func(args []Value) Value {
			code := 0
			if len(args) > 0 {
				if iv, ok := args[0].(*IntVal); ok {
					code = int(iv.Val)
				}
			}
			os.Exit(code)
			return &NilVal{}
		},
	})
	// Itoa: Itoa(v int) []char — convert int to decimal string
	pkg.define("Itoa", &BuiltinFuncVal{
		Name: "Itoa",
		Fn: func(args []Value) Value {
			if len(args) == 0 {
				return goToCharSlice("")
			}
			if iv, ok := args[0].(*IntVal); ok {
				return goToCharSlice(strconv.FormatInt(iv.Val, 10))
			}
			return goToCharSlice(args[0].String())
		},
	})
	// Concat: Concat(a []char, b []char) []char — concatenate two strings
	pkg.define("Concat", &BuiltinFuncVal{
		Name: "Concat",
		Fn: func(args []Value) Value {
			if len(args) < 2 {
				panic("Concat requires 2 arguments")
			}
			a := stringToGo(args[0])
			b := stringToGo(args[1])
			return goToCharSlice(a + b)
		},
	})
	// Open: Open(path []char, flags int) int — returns fd
	pkg.define("Open", &BuiltinFuncVal{
		Name: "Open",
		Fn: func(args []Value) Value {
			if len(args) < 2 {
				panic("Open requires 2 arguments: path, flags")
			}
			path := stringToGo(args[0])
			flags := int(args[1].(*IntVal).Val)
			fd, err := interp.openFile(path, flags)
			if err != nil {
				return &IntVal{Val: -1, Typ: types.Typ_int}
			}
			return &IntVal{Val: int64(fd), Typ: types.Typ_int}
		},
	})
	// Read: Read(fd int, buf []uint8, n int) int — returns bytes read
	pkg.define("Read", &BuiltinFuncVal{
		Name: "Read",
		Fn: func(args []Value) Value {
			if len(args) < 3 {
				panic("Read requires 3 arguments: fd, buf, n")
			}
			fd := int(args[0].(*IntVal).Val)
			buf := args[1].(*SliceVal)
			n := int(args[2].(*IntVal).Val)
			nRead := interp.readFile(fd, buf, n)
			return &IntVal{Val: int64(nRead), Typ: types.Typ_int}
		},
	})
	// Write: Write(fd int, data []uint8, n int) int — returns bytes written
	pkg.define("Write", &BuiltinFuncVal{
		Name: "Write",
		Fn: func(args []Value) Value {
			if len(args) < 3 {
				panic("Write requires 3 arguments: fd, data, n")
			}
			fd := int(args[0].(*IntVal).Val)
			data := args[1].(*SliceVal)
			n := int(args[2].(*IntVal).Val)
			nWritten := interp.writeFile(fd, data, n)
			return &IntVal{Val: int64(nWritten), Typ: types.Typ_int}
		},
	})
	// Close: Close(fd int) int — returns 0 on success
	pkg.define("Close", &BuiltinFuncVal{
		Name: "Close",
		Fn: func(args []Value) Value {
			if len(args) < 1 {
				panic("Close requires 1 argument: fd")
			}
			fd := int(args[0].(*IntVal).Val)
			err := interp.closeFile(fd)
			if err != 0 {
				return &IntVal{Val: int64(err), Typ: types.Typ_int}
			}
			return &IntVal{Val: 0, Typ: types.Typ_int}
		},
	})
	// Args: Args() [][]char — returns program arguments (after -- separator)
	pkg.define("Args", &BuiltinFuncVal{
		Name: "Args",
		Fn: func(args []Value) Value {
			elems := make([]Value, len(interp.progArgs))
			for i, a := range interp.progArgs {
				elems[i] = goToCharSlice(a)
			}
			return &SliceVal{
				Elems: elems,
				Typ:   &types.SliceType{Elem: types.Typ_string},
			}
		},
	})

	// Constants — file open flags
	pkg.define("O_RDONLY", &IntVal{Val: 0, Typ: types.Typ_int})
	pkg.define("O_WRONLY", &IntVal{Val: 1, Typ: types.Typ_int})
	pkg.define("O_RDWR", &IntVal{Val: 2, Typ: types.Typ_int})
	pkg.define("O_CREATE", &IntVal{Val: 0x40, Typ: types.Typ_int})
	pkg.define("O_TRUNC", &IntVal{Val: 0x200, Typ: types.Typ_int})
	pkg.define("O_APPEND", &IntVal{Val: 0x400, Typ: types.Typ_int})

	// Constants — standard file descriptors
	pkg.define("STDIN", &IntVal{Val: 0, Typ: types.Typ_int})
	pkg.define("STDOUT", &IntVal{Val: 1, Typ: types.Typ_int})
	pkg.define("STDERR", &IntVal{Val: 2, Typ: types.Typ_int})

	interp.packages["pkg/bootstrap"] = pkg
}

// Run executes a parsed and type-checked file.
func (interp *Interpreter) Run(file *ast.File, checker *types.Checker) {
	// Register imports — map local names to package paths
	for _, imp := range file.Imports {
		path := imp.Path.Value
		if len(path) >= 2 {
			path = path[1 : len(path)-1]
		}
		name := imp.Alias
		if name == "" {
			parts := strings.Split(path, "/")
			name = parts[len(parts)-1]
		}
		interp.importAliases[name] = path
	}

	// Pre-register type names so self-referential structs can resolve.
	interp.preRegisterTypes(file.Decls)

	// Register types and functions, evaluate top-level vars/consts
	interp.env = newEnv(interp.env) // package scope

	for _, d := range file.Decls {
		interp.execTopLevelDecl(d)
	}

	// Call main if it exists
	if mainDecl, ok := interp.funcs["main"]; ok {
		interp.callFuncInEnv(mainDecl, nil, interp.env)
	}
}

// LoadPackage loads a non-main package: evaluates its top-level declarations
// and registers the resulting environment as a package scope.
func (interp *Interpreter) LoadPackage(path string, file *ast.File, checker *types.Checker) {
	if _, ok := interp.packages[path]; ok {
		return // already loaded
	}

	// Save interpreter state
	savedEnv := interp.env
	savedFuncs := interp.funcs
	savedTypes := interp.types
	savedAliases := interp.importAliases

	// Set up fresh state for this package
	interp.funcs = make(map[string]*ast.FuncDecl)
	interp.types = make(map[string]types.Type)
	interp.importAliases = make(map[string]string)
	interp.env = newEnv(nil)
	interp.registerBuiltins() // builtins available in all packages

	// Register this package's imports
	for _, imp := range file.Imports {
		impPath := imp.Path.Value
		if len(impPath) >= 2 {
			impPath = impPath[1 : len(impPath)-1]
		}
		name := imp.Alias
		if name == "" {
			parts := strings.Split(impPath, "/")
			name = parts[len(parts)-1]
		}
		interp.importAliases[name] = impPath
	}

	// Pre-register type names so self-referential structs can resolve.
	interp.preRegisterTypes(file.Decls)

	// Execute top-level declarations
	interp.env = newEnv(interp.env) // package scope
	for _, d := range file.Decls {
		interp.execTopLevelDecl(d)
	}

	// Save the package env and types
	interp.packages[path] = interp.env
	interp.packageTypes[path] = interp.types

	// Restore interpreter state
	interp.env = savedEnv
	interp.funcs = savedFuncs
	interp.types = savedTypes
	interp.importAliases = savedAliases
}

// RunTestFunc calls a named no-arg function in a loaded package's scope.
// Returns "" on success, or the panic/error message on failure.
func (interp *Interpreter) RunTestFunc(pkgPath string, funcName string) (errMsg string) {
	defer func() {
		if r := recover(); r != nil {
			if re, ok := r.(*RuntimeError); ok {
				errMsg = re.Error()
			} else {
				errMsg = fmt.Sprintf("%v", r)
			}
		}
	}()

	env := interp.packages[pkgPath]
	if env == nil {
		return fmt.Sprintf("package %s not loaded", pkgPath)
	}

	val, found := env.get(funcName)
	if !found {
		return fmt.Sprintf("function %s not found in %s", funcName, pkgPath)
	}

	fv, ok := val.(*FuncVal)
	if !ok {
		return fmt.Sprintf("%s.%s is not a function", pkgPath, funcName)
	}

	// Set up the package's type/alias context
	savedTypes := interp.types
	savedAliases := interp.importAliases
	if fv.Types != nil {
		interp.types = fv.Types
	}
	if fv.Aliases != nil {
		interp.importAliases = fv.Aliases
	}
	defer func() {
		interp.types = savedTypes
		interp.importAliases = savedAliases
	}()

	interp.callFuncInEnv(fv.Decl, nil, fv.Env)
	return ""
}

func (interp *Interpreter) execTopLevelDecl(d ast.Decl) {
	switch d := d.(type) {
	case *ast.FuncDecl:
		interp.funcs[d.Name.Name] = d
		ft := interp.resolveFuncType(d)
		interp.env.define(d.Name.Name, &FuncVal{
			Name:    d.Name.Name,
			Typ:     ft,
			Decl:    d,
			Env:     interp.env,
			Types:   interp.types,
			Aliases: interp.importAliases,
		})
	case *ast.VarDecl:
		interp.execVarDecl(d)
	case *ast.ConstDecl:
		interp.execConstDecl(d)
	case *ast.TypeDecl:
		interp.execTypeDecl(d)
	case *ast.GroupDecl:
		// Check if this is a const group (enable iota)
		hasConst := false
		for _, inner := range d.Decls {
			if _, ok := inner.(*ast.ConstDecl); ok {
				hasConst = true
				break
			}
		}
		if hasConst {
			savedIota := interp.iota
			interp.iota = 0
			for _, inner := range d.Decls {
				interp.execTopLevelDecl(inner)
				if _, ok := inner.(*ast.ConstDecl); ok {
					interp.iota++
				}
			}
			interp.iota = savedIota
		} else {
			for _, inner := range d.Decls {
				interp.execTopLevelDecl(inner)
			}
		}
	}
}

func (interp *Interpreter) resolveFuncType(d *ast.FuncDecl) *types.FuncType {
	var params []*types.Param
	for _, p := range d.Params {
		params = append(params, &types.Param{
			Name: p.Name.Name,
			Type: interp.resolveType(p.Type),
		})
	}
	var results []types.Type
	for _, r := range d.Results {
		results = append(results, interp.resolveType(r))
	}
	return &types.FuncType{Params: params, Results: results}
}

// ============================================================
// Type Resolution (runtime)
// ============================================================

func (interp *Interpreter) resolveType(te ast.TypeExpr) types.Type {
	switch t := te.(type) {
	case *ast.NamedType:
		if t.Pkg != nil {
			// Qualified type: pkg.Type
			pkgPath, ok := interp.importAliases[t.Pkg.Name]
			if !ok {
				panic(fmt.Sprintf("unknown package: %s", t.Pkg.Name))
			}
			if pkgTypes, ok := interp.packageTypes[pkgPath]; ok {
				if typ, ok := pkgTypes[t.Name.Name]; ok {
					return typ
				}
			}
			panic(fmt.Sprintf("undefined type: %s.%s", t.Pkg.Name, t.Name.Name))
		}
		if typ, ok := types.PredeclaredTypes[t.Name.Name]; ok {
			return typ
		}
		if typ, ok := interp.types[t.Name.Name]; ok {
			return typ
		}
		panic(fmt.Sprintf("undefined type: %s", t.Name.Name))
	case *ast.PointerType:
		return &types.PointerType{Elem: interp.resolveType(t.Base)}
	case *ast.ManagedPtrType:
		return &types.ManagedPtrType{Elem: interp.resolveType(t.Base)}
	case *ast.ManagedSliceType:
		return &types.ManagedSliceType{Elem: interp.resolveType(t.Elem)}
	case *ast.SliceType:
		return &types.SliceType{Elem: interp.resolveType(t.Elem)}
	case *ast.ArrayType:
		length := interp.evalConstInt(t.Len)
		return &types.ArrayType{Len: length, Elem: interp.resolveType(t.Elem)}
	case *ast.StructType:
		var fields []*types.Field
		for _, f := range t.Fields {
			name := ""
			if f.Name != nil {
				name = f.Name.Name
			}
			fields = append(fields, &types.Field{Name: name, Type: interp.resolveType(f.Type)})
		}
		return &types.StructType{Fields: fields}
	case *ast.ParenType:
		return interp.resolveType(t.Type)
	default:
		panic(fmt.Sprintf("unsupported type expression: %T", te))
	}
}

func (interp *Interpreter) evalConstInt(e ast.Expr) int64 {
	v := interp.evalExpr(e)
	if iv, ok := v.(*IntVal); ok {
		return iv.Val
	}
	panic("array length must be integer")
}

// ============================================================
// Declaration Execution
// ============================================================

func (interp *Interpreter) execVarDecl(d *ast.VarDecl) {
	if d.Value != nil {
		val := interp.evalExpr(d.Value)
		if d.Type != nil {
			val = interp.coerce(val, interp.resolveType(d.Type))
		}
		interp.env.define(d.Name.Name, val)
	} else if d.Type != nil {
		interp.env.define(d.Name.Name, ZeroValue(interp.resolveType(d.Type)))
	}
}

func (interp *Interpreter) execConstDecl(d *ast.ConstDecl) {
	if d.Value != nil {
		val := interp.evalExpr(d.Value)
		if d.Type != nil {
			val = interp.coerce(val, interp.resolveType(d.Type))
		}
		interp.env.define(d.Name.Name, val)
	} else if interp.iota >= 0 {
		// Bare name in grouped const — use current iota value
		val := Value(&IntVal{Val: int64(interp.iota), Typ: types.Typ_int})
		if d.Type != nil {
			val = interp.coerce(val, interp.resolveType(d.Type))
		}
		interp.env.define(d.Name.Name, val)
	}
}

// preRegisterTypes does a first pass over declarations, registering type names
// as placeholders so self-referential and forward-referenced types can resolve.
func (interp *Interpreter) preRegisterTypes(decls []ast.Decl) {
	for _, d := range decls {
		switch d := d.(type) {
		case *ast.TypeDecl:
			if _, ok := interp.types[d.Name.Name]; !ok {
				// Register placeholder — will be filled in by execTypeDecl
				interp.types[d.Name.Name] = &types.StructType{Name: d.Name.Name}
			}
		case *ast.GroupDecl:
			interp.preRegisterTypes(d.Decls)
		}
	}
}

func (interp *Interpreter) execTypeDecl(d *ast.TypeDecl) {
	if d.Assign {
		// Alias
		target := interp.resolveType(d.Type)
		interp.types[d.Name.Name] = &types.AliasType{Name: d.Name.Name, Target: target}
	} else if st, ok := d.Type.(*ast.StructType); ok {
		// Struct
		resolved := interp.resolveType(st).(*types.StructType)
		resolved.Name = d.Name.Name
		interp.types[d.Name.Name] = resolved
	} else {
		// Distinct
		underlying := interp.resolveType(d.Type)
		interp.types[d.Name.Name] = &types.NamedType{Name: d.Name.Name, Underlying_: underlying}
	}
}

// ============================================================
// Statement Execution
// ============================================================

func (interp *Interpreter) execBlock(b *ast.Block) {
	for _, s := range b.Stmts {
		interp.execStmt(s)
	}
}

func (interp *Interpreter) execStmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.ExprStmt:
		interp.evalExpr(s.X)

	case *ast.AssignStmt:
		interp.execAssign(s)

	case *ast.ShortVarDecl:
		interp.execShortVarDecl(s)

	case *ast.IncDecStmt:
		interp.execIncDec(s)

	case *ast.ReturnStmt:
		var vals []Value
		for _, r := range s.Results {
			vals = append(vals, interp.evalExpr(r))
		}
		panic(signalReturn{vals: vals})

	case *ast.BreakStmt:
		panic(signalBreak{})

	case *ast.ContinueStmt:
		panic(signalContinue{})

	case *ast.Block:
		interp.pushEnv()
		interp.execBlock(s)
		interp.popEnv()

	case *ast.IfStmt:
		interp.execIf(s)

	case *ast.ForStmt:
		interp.execFor(s)

	case *ast.SwitchStmt:
		interp.execSwitch(s)

	case *ast.VarDecl:
		interp.execVarDecl(s)

	case *ast.ConstDecl:
		interp.execConstDecl(s)

	case *ast.TypeDecl:
		interp.execTypeDecl(s)

	case *ast.EmptyStmt:
		// nothing

	default:
		panic(fmt.Sprintf("unhandled statement: %T", s))
	}
}

func (interp *Interpreter) execAssign(s *ast.AssignStmt) {
	if s.Op == token.ASSIGN {
		// Simple assignment
		if len(s.RHS) == 1 && len(s.LHS) > 1 {
			// Multi-return: x, y = f()
			rhs := interp.evalExpr(s.RHS[0])
			if mv, ok := rhs.(*MultiVal); ok {
				for i, lhs := range s.LHS {
					interp.assignTo(lhs, mv.Vals[i])
				}
				return
			}
		}
		for i, lhs := range s.LHS {
			val := interp.evalExpr(s.RHS[i])
			interp.assignTo(lhs, val)
		}
	} else {
		// Compound assignment
		lhs := interp.evalExpr(s.LHS[0])
		rhs := interp.evalExpr(s.RHS[0])
		baseOp := compoundToBase(s.Op)
		result := interp.evalBinaryOp(s.LHS[0].Pos(), baseOp, lhs, rhs)
		interp.assignTo(s.LHS[0], result)
	}
}

func (interp *Interpreter) assignTo(lhs ast.Expr, val Value) {
	switch lhs := lhs.(type) {
	case *ast.Ident:
		if !interp.env.set(lhs.Name, val) {
			interp.env.define(lhs.Name, val)
		}
	case *ast.IndexExpr:
		container := interp.evalExpr(lhs.X)
		idx := interp.evalExpr(lhs.Index)
		i := idx.(*IntVal).Val
		switch c := container.(type) {
		case *SliceVal:
			if i < 0 || int(i) >= len(c.Elems) {
				runtimePanic(lhs.Index.Pos(), "index out of bounds: %d (len %d)", i, len(c.Elems))
			}
			c.Elems[i] = val
		case *ArrayVal:
			if i < 0 || int(i) >= len(c.Elems) {
				runtimePanic(lhs.Index.Pos(), "index out of bounds: %d (len %d)", i, len(c.Elems))
			}
			c.Elems[i] = val
		default:
			panic(fmt.Sprintf("cannot index-assign to %T", container))
		}
	case *ast.SelectorExpr:
		obj := interp.evalExpr(lhs.X)
		// Auto-deref pointers
		if p, ok := obj.(*PointerVal); ok {
			obj = p.Addr.Val
		}
		if p, ok := obj.(*ManagedPtrVal); ok {
			obj = p.Addr.Val
		}
		sv, ok := obj.(*StructVal)
		if !ok {
			panic(fmt.Sprintf("cannot field-assign to %T", obj))
		}
		for i, f := range sv.Typ.Fields {
			if f.Name == lhs.Sel.Name {
				sv.Fields[i] = val
				return
			}
		}
		panic(fmt.Sprintf("no field %s", lhs.Sel.Name))
	case *ast.UnaryExpr:
		if lhs.Op == token.STAR {
			// *p = val
			ptr := interp.evalExpr(lhs.X)
			switch p := ptr.(type) {
			case *PointerVal:
				p.Addr.Val = val
			case *ManagedPtrVal:
				p.Addr.Val = val
			default:
				panic("cannot dereference-assign")
			}
			return
		}
		panic("invalid assignment target")
	default:
		panic(fmt.Sprintf("invalid assignment target: %T", lhs))
	}
}

func (interp *Interpreter) execShortVarDecl(s *ast.ShortVarDecl) {
	if len(s.RHS) == 1 && len(s.Names) > 1 {
		// Multi-return: x, y := f()
		rhs := interp.evalExpr(s.RHS[0])
		if mv, ok := rhs.(*MultiVal); ok {
			for i, name := range s.Names {
				interp.env.define(name.Name, mv.Vals[i])
			}
			return
		}
	}
	for i, name := range s.Names {
		val := interp.evalExpr(s.RHS[i])
		interp.env.define(name.Name, val)
	}
}

func (interp *Interpreter) execIncDec(s *ast.IncDecStmt) {
	val := interp.evalExpr(s.X)
	iv := val.(*IntVal)
	var newVal int64
	if s.Op == token.INC {
		newVal = iv.Val + 1
	} else {
		newVal = iv.Val - 1
	}
	interp.assignTo(s.X, &IntVal{Val: newVal, Typ: iv.Typ})
}

func (interp *Interpreter) execIf(s *ast.IfStmt) {
	cond := interp.evalExpr(s.Cond)
	if interp.isTruthy(cond) {
		interp.pushEnv()
		interp.execBlock(s.Body)
		interp.popEnv()
	} else if s.Else != nil {
		interp.execStmt(s.Else)
	}
}

func (interp *Interpreter) execFor(s *ast.ForStmt) {
	interp.pushEnv()
	defer interp.popEnv()

	if s.Iter != nil {
		// For-in
		interp.execForIn(s)
		return
	}

	// Init
	if s.Init != nil {
		interp.execStmt(s.Init)
	}

	for {
		// Condition
		if s.Cond != nil {
			cond := interp.evalExpr(s.Cond)
			if !interp.isTruthy(cond) {
				break
			}
		}

		// Body
		brk := interp.execLoopBody(s.Body)
		if brk {
			break
		}

		// Post
		if s.Post != nil {
			interp.execStmt(s.Post)
		}
	}
}

func (interp *Interpreter) execForIn(s *ast.ForStmt) {
	collection := interp.evalExpr(s.Iter)
	var elems []Value

	switch c := collection.(type) {
	case *SliceVal:
		elems = c.Elems
	case *ArrayVal:
		elems = c.Elems
	default:
		panic(fmt.Sprintf("cannot iterate over %T", collection))
	}

	for i, elem := range elems {
		if s.Key != nil {
			interp.env.define(s.Key.Name, &IntVal{Val: int64(i), Typ: types.Typ_int})
		}
		if s.Value != nil {
			interp.env.define(s.Value.Name, elem)
		}

		brk := interp.execLoopBody(s.Body)
		if brk {
			break
		}
	}
}

// execLoopBody executes a loop body, handling break/continue signals.
// Returns true if the loop should break.
func (interp *Interpreter) execLoopBody(body *ast.Block) (brk bool) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case signalBreak:
				brk = true
			case signalContinue:
				// continue — just return, loop will proceed
			default:
				panic(r) // re-panic for signalReturn or real errors
			}
		}
	}()
	interp.execBlock(body)
	return false
}

func (interp *Interpreter) execSwitch(s *ast.SwitchStmt) {
	var tag Value
	if s.Tag != nil {
		tag = interp.evalExpr(s.Tag)
	}

	for _, cc := range s.Cases {
		if cc.Exprs == nil {
			// Default — handled below
			continue
		}
		for _, expr := range cc.Exprs {
			val := interp.evalExpr(expr)
			if tag != nil && interp.valuesEqual(tag, val) {
				interp.pushEnv()
				for _, stmt := range cc.Body {
					interp.execStmt(stmt)
				}
				interp.popEnv()
				return
			}
		}
	}

	// Default case
	for _, cc := range s.Cases {
		if cc.Exprs == nil {
			interp.pushEnv()
			for _, stmt := range cc.Body {
				interp.execStmt(stmt)
			}
			interp.popEnv()
			return
		}
	}
}

// ============================================================
// Expression Evaluation
// ============================================================

func (interp *Interpreter) evalExpr(e ast.Expr) Value {
	switch e := e.(type) {
	case *ast.IntLit:
		return interp.evalIntLit(e)
	case *ast.StringLit:
		return interp.evalStringLit(e)
	case *ast.CharLit:
		return interp.evalCharLit(e)
	case *ast.BoolLit:
		return &BoolVal{Val: e.Value}
	case *ast.NilLit:
		return &NilVal{}
	case *ast.Ident:
		return interp.evalIdent(e)
	case *ast.BinaryExpr:
		return interp.evalBinary(e)
	case *ast.UnaryExpr:
		return interp.evalUnary(e)
	case *ast.CallExpr:
		return interp.evalCall(e)
	case *ast.IndexExpr:
		return interp.evalIndex(e)
	case *ast.SliceExpr:
		return interp.evalSlice(e)
	case *ast.SelectorExpr:
		return interp.evalSelector(e)
	case *ast.CompositeLit:
		return interp.evalCompositeLit(e)
	case *ast.BuiltinCall:
		return interp.evalBuiltinCall(e)
	default:
		panic(fmt.Sprintf("unhandled expression: %T", e))
	}
}

func (interp *Interpreter) evalIntLit(e *ast.IntLit) Value {
	v, err := parseIntLit(e.Value)
	if err != nil {
		panic(fmt.Sprintf("invalid integer literal: %s", e.Value))
	}
	return &IntVal{Val: v, Typ: types.Typ_int}
}

func (interp *Interpreter) evalStringLit(e *ast.StringLit) Value {
	// Strip quotes and unescape
	raw := e.Value
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		raw = raw[1 : len(raw)-1]
	}
	s := unescapeString(raw)
	return &StringVal{Val: s}
}

func (interp *Interpreter) evalCharLit(e *ast.CharLit) Value {
	raw := e.Value
	if len(raw) >= 2 && raw[0] == '\'' && raw[len(raw)-1] == '\'' {
		raw = raw[1 : len(raw)-1]
	}
	if len(raw) == 0 {
		return &CharVal{Val: 0}
	}
	if raw[0] == '\\' && len(raw) > 1 {
		ch := unescapeChar(raw)
		return &CharVal{Val: ch}
	}
	r := []rune(raw)
	return &CharVal{Val: r[0]}
}

func (interp *Interpreter) evalIdent(e *ast.Ident) Value {
	if e.Name == "iota" && interp.iota >= 0 {
		return &IntVal{Val: int64(interp.iota), Typ: types.Typ_int}
	}
	v, ok := interp.env.get(e.Name)
	if !ok {
		panic(fmt.Sprintf("undefined variable: %s", e.Name))
	}
	return v
}

func (interp *Interpreter) evalBinary(e *ast.BinaryExpr) Value {
	lhs := interp.evalExpr(e.X)
	// Short-circuit for logical operators
	if e.Op == token.LAND {
		if !interp.isTruthy(lhs) {
			return &BoolVal{Val: false}
		}
		rhs := interp.evalExpr(e.Y)
		return &BoolVal{Val: interp.isTruthy(rhs)}
	}
	if e.Op == token.LOR {
		if interp.isTruthy(lhs) {
			return &BoolVal{Val: true}
		}
		rhs := interp.evalExpr(e.Y)
		return &BoolVal{Val: interp.isTruthy(rhs)}
	}
	rhs := interp.evalExpr(e.Y)
	return interp.evalBinaryOp(e.Pos(), e.Op, lhs, rhs)
}

func (interp *Interpreter) evalBinaryOp(pos token.Pos, op token.Type, lhs, rhs Value) Value {
	// Integer arithmetic
	if lv, ok := lhs.(*IntVal); ok {
		if rv, ok := rhs.(*IntVal); ok {
			return interp.evalIntBinaryOp(pos, op, lv, rv)
		}
	}

	// Comparison with nil
	if _, ok := lhs.(*NilVal); ok {
		return interp.evalNilCompare(op, rhs)
	}
	if _, ok := rhs.(*NilVal); ok {
		return interp.evalNilCompare(op, lhs)
	}

	// Bool operations
	if lv, ok := lhs.(*BoolVal); ok {
		if rv, ok := rhs.(*BoolVal); ok {
			switch op {
			case token.EQ:
				return &BoolVal{Val: lv.Val == rv.Val}
			case token.NEQ:
				return &BoolVal{Val: lv.Val != rv.Val}
			}
		}
	}

	// String operations
	if lv, ok := lhs.(*StringVal); ok {
		if rv, ok := rhs.(*StringVal); ok {
			switch op {
			case token.PLUS:
				return &StringVal{Val: lv.Val + rv.Val}
			case token.EQ:
				return &BoolVal{Val: lv.Val == rv.Val}
			case token.NEQ:
				return &BoolVal{Val: lv.Val != rv.Val}
			case token.LT:
				return &BoolVal{Val: lv.Val < rv.Val}
			case token.GT:
				return &BoolVal{Val: lv.Val > rv.Val}
			case token.LEQ:
				return &BoolVal{Val: lv.Val <= rv.Val}
			case token.GEQ:
				return &BoolVal{Val: lv.Val >= rv.Val}
			}
		}
	}

	// Char comparison
	if lv, ok := lhs.(*CharVal); ok {
		if rv, ok := rhs.(*CharVal); ok {
			switch op {
			case token.EQ:
				return &BoolVal{Val: lv.Val == rv.Val}
			case token.NEQ:
				return &BoolVal{Val: lv.Val != rv.Val}
			}
		}
	}

	panic(fmt.Sprintf("unsupported binary operation: %s %s %s", lhs.Type(), op, rhs.Type()))
}

func (interp *Interpreter) evalIntBinaryOp(pos token.Pos, op token.Type, lv, rv *IntVal) Value {
	l, r := lv.Val, rv.Val
	switch op {
	case token.PLUS:
		return &IntVal{Val: l + r, Typ: lv.Typ}
	case token.MINUS:
		return &IntVal{Val: l - r, Typ: lv.Typ}
	case token.STAR:
		return &IntVal{Val: l * r, Typ: lv.Typ}
	case token.SLASH:
		if r == 0 {
			runtimePanic(pos, "division by zero")
		}
		return &IntVal{Val: l / r, Typ: lv.Typ}
	case token.PERCENT:
		if r == 0 {
			runtimePanic(pos, "division by zero")
		}
		return &IntVal{Val: l % r, Typ: lv.Typ}
	case token.AMP:
		return &IntVal{Val: l & r, Typ: lv.Typ}
	case token.PIPE:
		return &IntVal{Val: l | r, Typ: lv.Typ}
	case token.CARET:
		return &IntVal{Val: l ^ r, Typ: lv.Typ}
	case token.SHL:
		return &IntVal{Val: l << uint(r), Typ: lv.Typ}
	case token.SHR:
		return &IntVal{Val: l >> uint(r), Typ: lv.Typ}
	case token.EQ:
		return &BoolVal{Val: l == r}
	case token.NEQ:
		return &BoolVal{Val: l != r}
	case token.LT:
		return &BoolVal{Val: l < r}
	case token.GT:
		return &BoolVal{Val: l > r}
	case token.LEQ:
		return &BoolVal{Val: l <= r}
	case token.GEQ:
		return &BoolVal{Val: l >= r}
	default:
		panic(fmt.Sprintf("unsupported int operator: %s", op))
	}
}

func (interp *Interpreter) evalNilCompare(op token.Type, other Value) Value {
	isNil := false
	switch v := other.(type) {
	case *NilVal:
		isNil = true
	case *PointerVal:
		isNil = v.Addr == nil
	case *ManagedPtrVal:
		isNil = v.Addr == nil
	case *SliceVal:
		isNil = v.Elems == nil
	}
	switch op {
	case token.EQ:
		return &BoolVal{Val: isNil}
	case token.NEQ:
		return &BoolVal{Val: !isNil}
	default:
		panic("invalid nil comparison operator")
	}
}

func (interp *Interpreter) evalUnary(e *ast.UnaryExpr) Value {
	x := interp.evalExpr(e.X)
	switch e.Op {
	case token.MINUS:
		return &IntVal{Val: -x.(*IntVal).Val, Typ: x.(*IntVal).Typ}
	case token.NOT:
		return &BoolVal{Val: !interp.isTruthy(x)}
	case token.TILDE:
		return &IntVal{Val: ^x.(*IntVal).Val, Typ: x.(*IntVal).Typ}
	case token.STAR:
		// Dereference
		switch p := x.(type) {
		case *PointerVal:
			if p.Addr == nil {
				runtimePanic(e.Pos(), "nil pointer dereference")
			}
			return p.Addr.Val
		case *ManagedPtrVal:
			if p.Addr == nil {
				runtimePanic(e.Pos(), "nil pointer dereference")
			}
			return p.Addr.Val
		default:
			runtimePanic(e.Pos(), "cannot dereference non-pointer")
		}
		panic("unreachable")
	case token.AMP:
		// Address-of — share the variable's backing cell if operand is an ident
		if ident, ok := e.X.(*ast.Ident); ok {
			cell := interp.env.getCell(ident.Name)
			if cell != nil {
				cell.Refcount++
				return &PointerVal{Addr: cell, Typ: &types.PointerType{Elem: x.Type()}}
			}
		}
		obj := &HeapObject{Val: x, Refcount: 1}
		return &PointerVal{Addr: obj, Typ: &types.PointerType{Elem: x.Type()}}
	default:
		panic(fmt.Sprintf("unsupported unary operator: %s", e.Op))
	}
}

func (interp *Interpreter) evalCall(e *ast.CallExpr) Value {
	fn := interp.evalExpr(e.Fun)

	// Builtin function
	if bf, ok := fn.(*BuiltinFuncVal); ok {
		var args []Value
		for _, a := range e.Args {
			args = append(args, interp.evalExpr(a))
		}
		return bf.Fn(args)
	}

	// User function
	fv, ok := fn.(*FuncVal)
	if !ok {
		panic(fmt.Sprintf("cannot call %T", fn))
	}

	if fv.Decl == nil {
		panic(fmt.Sprintf("undefined function: %s", fv.Name))
	}

	var args []Value
	for _, a := range e.Args {
		args = append(args, interp.evalExpr(a))
	}

	callEnv := interp.env
	var savedTypes map[string]types.Type
	var savedAliases map[string]string
	if fv.Env != nil {
		callEnv = fv.Env
	}
	if fv.Types != nil {
		savedTypes = interp.types
		savedAliases = interp.importAliases
		interp.types = fv.Types
		interp.importAliases = fv.Aliases
	}
	result := interp.callFuncInEnv(fv.Decl, args, callEnv)
	if savedTypes != nil {
		interp.types = savedTypes
		interp.importAliases = savedAliases
	}
	return result
}

func (interp *Interpreter) callFuncInEnv(decl *ast.FuncDecl, args []Value, env *Env) Value {
	savedEnv := interp.env
	interp.env = newEnv(env)
	defer func() { interp.env = savedEnv }()

	// Bind parameters
	for i, p := range decl.Params {
		if i < len(args) {
			interp.env.define(p.Name.Name, args[i])
		}
	}

	// Execute body, catching return signal
	var retVals []Value
	func() {
		defer func() {
			if r := recover(); r != nil {
				if ret, ok := r.(signalReturn); ok {
					retVals = ret.vals
				} else {
					panic(r)
				}
			}
		}()
		interp.execBlock(decl.Body)
	}()

	if len(retVals) == 0 {
		return &NilVal{}
	}
	if len(retVals) == 1 {
		return retVals[0]
	}
	return &MultiVal{Vals: retVals}
}

func (interp *Interpreter) evalIndex(e *ast.IndexExpr) Value {
	x := interp.evalExpr(e.X)
	idx := interp.evalExpr(e.Index)
	i := idx.(*IntVal).Val

	switch c := x.(type) {
	case *SliceVal:
		if i < 0 || int(i) >= len(c.Elems) {
			runtimePanic(e.Index.Pos(), "index out of bounds: %d (len %d)", i, len(c.Elems))
		}
		return c.Elems[i]
	case *ArrayVal:
		if i < 0 || int(i) >= len(c.Elems) {
			runtimePanic(e.Index.Pos(), "index out of bounds: %d (len %d)", i, len(c.Elems))
		}
		return c.Elems[i]
	case *StringVal:
		if i < 0 || int(i) >= len(c.Val) {
			runtimePanic(e.Index.Pos(), "index out of bounds: %d (len %d)", i, len(c.Val))
		}
		return &CharVal{Val: rune(c.Val[i])}
	case *PointerVal:
		// Pointer arithmetic — not implemented in bootstrap
		panic("pointer indexing not supported")
	default:
		panic(fmt.Sprintf("cannot index %T", x))
	}
}

func (interp *Interpreter) evalSlice(e *ast.SliceExpr) Value {
	x := interp.evalExpr(e.X)
	var elems []Value

	switch c := x.(type) {
	case *SliceVal:
		elems = c.Elems
	case *ArrayVal:
		elems = c.Elems
	default:
		panic(fmt.Sprintf("cannot slice %T", x))
	}

	lo := 0
	hi := len(elems)
	if e.Lo != nil {
		lo = int(interp.evalExpr(e.Lo).(*IntVal).Val)
	}
	if e.Hi != nil {
		hi = int(interp.evalExpr(e.Hi).(*IntVal).Val)
	}

	sliced := make([]Value, hi-lo)
	copy(sliced, elems[lo:hi])

	// Determine result type
	var typ types.Type
	switch c := x.(type) {
	case *SliceVal:
		typ = c.Typ
	case *ArrayVal:
		typ = &types.SliceType{Elem: c.Typ.Elem}
	}
	return &SliceVal{Elems: sliced, Typ: typ}
}

func (interp *Interpreter) evalSelector(e *ast.SelectorExpr) Value {
	// Package-qualified access: pkg.Name
	if ident, ok := e.X.(*ast.Ident); ok {
		if pkgPath, ok := interp.importAliases[ident.Name]; ok {
			if pkg, ok := interp.packages[pkgPath]; ok {
				val, found := pkg.get(e.Sel.Name)
				if !found {
					panic(fmt.Sprintf("package %s has no member %s", ident.Name, e.Sel.Name))
				}
				return val
			}
		}
	}

	x := interp.evalExpr(e.X)

	// Auto-deref pointers
	if p, ok := x.(*PointerVal); ok {
		x = p.Addr.Val
	}
	if p, ok := x.(*ManagedPtrVal); ok {
		x = p.Addr.Val
	}

	if sv, ok := x.(*StructVal); ok {
		for i, f := range sv.Typ.Fields {
			if f.Name == e.Sel.Name {
				return sv.Fields[i]
			}
		}
		panic(fmt.Sprintf("no field %s on %s", e.Sel.Name, sv.Typ))
	}

	panic(fmt.Sprintf("cannot access field on %T", x))
}

func (interp *Interpreter) evalCompositeLit(e *ast.CompositeLit) Value {
	typ := interp.resolveType(e.Type)
	resolved := types.ResolveAlias(typ)

	switch st := resolved.(type) {
	case *types.StructType:
		return interp.evalStructLit(e, st, typ)
	case *types.ArrayType:
		return interp.evalArrayCompositeLit(e, st)
	case *types.NamedType:
		if inner, ok := st.Underlying_.(*types.StructType); ok {
			return interp.evalStructLit(e, inner, typ)
		}
	}

	panic(fmt.Sprintf("unsupported composite literal type: %T", resolved))
}

func (interp *Interpreter) evalStructLit(e *ast.CompositeLit, st *types.StructType, typ types.Type) Value {
	fields := make([]Value, len(st.Fields))
	for i, f := range st.Fields {
		fields[i] = ZeroValue(f.Type)
	}

	if len(e.Elements) > 0 && e.Elements[0].Key != nil {
		// Keyed initialization
		for _, elem := range e.Elements {
			key := elem.Key.(*ast.Ident).Name
			val := interp.evalExpr(elem.Value)
			for i, f := range st.Fields {
				if f.Name == key {
					fields[i] = val
					break
				}
			}
		}
	} else {
		// Positional initialization
		for i, elem := range e.Elements {
			if i < len(fields) {
				fields[i] = interp.evalExpr(elem.Value)
			}
		}
	}

	return &StructVal{Fields: fields, Typ: st}
}

func (interp *Interpreter) evalArrayCompositeLit(e *ast.CompositeLit, at *types.ArrayType) Value {
	elems := make([]Value, at.Len)
	for i := range elems {
		elems[i] = ZeroValue(at.Elem)
	}
	for i, elem := range e.Elements {
		if i < len(elems) {
			elems[i] = interp.evalExpr(elem.Value)
		}
	}
	return &ArrayVal{Elems: elems, Typ: at}
}

func (interp *Interpreter) evalBuiltinCall(e *ast.BuiltinCall) Value {
	switch e.Builtin {
	case token.MAKE:
		return interp.evalMake(e)
	case token.BOX:
		arg := interp.evalExpr(e.Args[0])
		obj := &HeapObject{Val: arg, Refcount: 1}
		return &ManagedPtrVal{
			Addr: obj,
			Typ:  &types.ManagedPtrType{Elem: arg.Type()},
		}
	case token.CAST:
		return interp.evalCast(e)
	case token.BIT_CAST:
		return interp.evalCast(e) // simplified — same as cast for bootstrap
	case token.LEN:
		return interp.evalLen(e)
	default:
		panic(fmt.Sprintf("unsupported builtin: %s", e.Builtin))
	}
}

func (interp *Interpreter) evalMake(e *ast.BuiltinCall) Value {
	typ := interp.resolveType(e.Type)

	if st, ok := typ.(*types.SliceType); ok {
		// make([]T, n)
		n := int64(0)
		if len(e.Args) > 0 {
			n = interp.evalExpr(e.Args[0]).(*IntVal).Val
		}
		elems := make([]Value, n)
		for i := range elems {
			elems[i] = ZeroValue(st.Elem)
		}
		return &SliceVal{
			Elems: elems,
			Typ:   &types.ManagedSliceType{Elem: st.Elem},
		}
	}

	// make(T) — allocate managed pointer
	zv := ZeroValue(typ)
	obj := &HeapObject{Val: zv, Refcount: 1}
	return &ManagedPtrVal{
		Addr: obj,
		Typ:  &types.ManagedPtrType{Elem: typ},
	}
}

func (interp *Interpreter) evalCast(e *ast.BuiltinCall) Value {
	targetType := interp.resolveType(e.Type)
	arg := interp.evalExpr(e.Args[0])

	// Integer-to-integer cast
	if it, ok := targetType.(*types.IntType); ok {
		if iv, ok := arg.(*IntVal); ok {
			return &IntVal{Val: truncateInt(iv.Val, it), Typ: it}
		}
		// Char to int
		if cv, ok := arg.(*CharVal); ok {
			return &IntVal{Val: int64(cv.Val), Typ: it}
		}
	}

	// Int to char
	if _, ok := targetType.(*types.CharType); ok {
		if iv, ok := arg.(*IntVal); ok {
			return &CharVal{Val: rune(iv.Val)}
		}
	}

	// Named type cast
	if nt, ok := targetType.(*types.NamedType); ok {
		if it, ok := nt.Underlying_.(*types.IntType); ok {
			if iv, ok := arg.(*IntVal); ok {
				return &IntVal{Val: iv.Val, Typ: it}
			}
		}
	}

	return arg // fallback — return as-is
}

func (interp *Interpreter) evalLen(e *ast.BuiltinCall) Value {
	arg := interp.evalExpr(e.Args[0])
	switch v := arg.(type) {
	case *SliceVal:
		return &IntVal{Val: int64(len(v.Elems)), Typ: types.Typ_int}
	case *ArrayVal:
		return &IntVal{Val: int64(len(v.Elems)), Typ: types.Typ_int}
	case *StringVal:
		return &IntVal{Val: int64(len(v.Val)), Typ: types.Typ_int}
	default:
		panic(fmt.Sprintf("len: unsupported type %T", arg))
	}
}

// ============================================================
// Helpers
// ============================================================

func (interp *Interpreter) pushEnv() {
	interp.env = newEnv(interp.env)
}

func (interp *Interpreter) popEnv() {
	interp.env = interp.env.parent
}

func (interp *Interpreter) isTruthy(v Value) bool {
	switch v := v.(type) {
	case *BoolVal:
		return v.Val
	case *IntVal:
		return v.Val != 0
	case *NilVal:
		return false
	case *PointerVal:
		return v.Addr != nil
	case *ManagedPtrVal:
		return v.Addr != nil
	default:
		return true
	}
}

func (interp *Interpreter) valuesEqual(a, b Value) bool {
	if ai, ok := a.(*IntVal); ok {
		if bi, ok := b.(*IntVal); ok {
			return ai.Val == bi.Val
		}
	}
	if as, ok := a.(*StringVal); ok {
		if bs, ok := b.(*StringVal); ok {
			return as.Val == bs.Val
		}
	}
	if ab, ok := a.(*BoolVal); ok {
		if bb, ok := b.(*BoolVal); ok {
			return ab.Val == bb.Val
		}
	}
	if ac, ok := a.(*CharVal); ok {
		if bc, ok := b.(*CharVal); ok {
			return ac.Val == bc.Val
		}
	}
	return false
}

// coerce converts a value to the target type.
func (interp *Interpreter) coerce(v Value, target types.Type) Value {
	target = types.ResolveAlias(target)

	// IntVal to specific int type
	if iv, ok := v.(*IntVal); ok {
		if it, ok := target.(*types.IntType); ok {
			return &IntVal{Val: iv.Val, Typ: it}
		}
		if _, ok := target.(*types.NamedType); ok {
			if it, ok := target.(*types.NamedType).Underlying_.(*types.IntType); ok {
				return &IntVal{Val: iv.Val, Typ: it}
			}
		}
	}

	// NilVal to typed nil pointer/slice
	if _, ok := v.(*NilVal); ok {
		return ZeroValue(target)
	}

	return v
}

func compoundToBase(op token.Type) token.Type {
	switch op {
	case token.ADD_ASSIGN:
		return token.PLUS
	case token.SUB_ASSIGN:
		return token.MINUS
	case token.MUL_ASSIGN:
		return token.STAR
	case token.QUO_ASSIGN:
		return token.SLASH
	case token.REM_ASSIGN:
		return token.PERCENT
	case token.AND_ASSIGN:
		return token.AMP
	case token.OR_ASSIGN:
		return token.PIPE
	case token.XOR_ASSIGN:
		return token.CARET
	case token.SHL_ASSIGN:
		return token.SHL
	case token.SHR_ASSIGN:
		return token.SHR
	}
	return op
}

// truncateInt truncates a value to the bit width of the target type.
func truncateInt(val int64, t *types.IntType) int64 {
	switch t.Width {
	case 8:
		if t.Signed {
			return int64(int8(val))
		}
		return int64(uint8(val))
	case 16:
		if t.Signed {
			return int64(int16(val))
		}
		return int64(uint16(val))
	case 32:
		if t.Signed {
			return int64(int32(val))
		}
		return int64(uint32(val))
	case 64:
		return val
	}
	return val
}

func parseIntLit(s string) (int64, error) {
	if len(s) > 2 {
		switch {
		case strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X"):
			return strconv.ParseInt(s[2:], 16, 64)
		case strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0O"):
			return strconv.ParseInt(s[2:], 8, 64)
		case strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B"):
			return strconv.ParseInt(s[2:], 2, 64)
		}
	}
	return strconv.ParseInt(s, 10, 64)
}

func unescapeString(s string) string {
	var buf strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				buf.WriteByte('\n')
			case 'r':
				buf.WriteByte('\r')
			case 't':
				buf.WriteByte('\t')
			case '\\':
				buf.WriteByte('\\')
			case '"':
				buf.WriteByte('"')
			case '\'':
				buf.WriteByte('\'')
			case '0':
				buf.WriteByte(0)
			case 'x':
				if i+2 < len(s) {
					v, err := strconv.ParseUint(s[i+1:i+3], 16, 8)
					if err == nil {
						buf.WriteByte(byte(v))
						i += 2
						continue
					}
				}
				buf.WriteByte('\\')
				buf.WriteByte(s[i])
			default:
				buf.WriteByte('\\')
				buf.WriteByte(s[i])
			}
		} else {
			buf.WriteByte(s[i])
		}
	}
	return buf.String()
}

func unescapeChar(s string) rune {
	if len(s) < 2 || s[0] != '\\' {
		return rune(s[0])
	}
	switch s[1] {
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	case '\\':
		return '\\'
	case '\'':
		return '\''
	case '0':
		return 0
	case 'x':
		if len(s) >= 4 {
			v, err := strconv.ParseUint(s[2:4], 16, 8)
			if err == nil {
				return rune(v)
			}
		}
	}
	return rune(s[1])
}

// ============================================================
// File I/O
// ============================================================

// File open flags (matching POSIX conventions).
const (
	O_RDONLY = 0
	O_WRONLY = 1
	O_RDWR   = 2
	O_CREATE = 0x40
	O_TRUNC  = 0x200
	O_APPEND = 0x400
)

func (interp *Interpreter) openFile(path string, flags int) (int, error) {
	goFlags := 0
	switch flags & 3 {
	case O_RDONLY:
		goFlags = os.O_RDONLY
	case O_WRONLY:
		goFlags = os.O_WRONLY
	case O_RDWR:
		goFlags = os.O_RDWR
	}
	if flags&O_CREATE != 0 {
		goFlags |= os.O_CREATE
	}
	if flags&O_TRUNC != 0 {
		goFlags |= os.O_TRUNC
	}
	if flags&O_APPEND != 0 {
		goFlags |= os.O_APPEND
	}

	f, err := os.OpenFile(path, goFlags, 0644)
	if err != nil {
		return -1, err
	}
	fd := interp.nextFD
	interp.nextFD++
	interp.files[fd] = f
	return fd, nil
}

func (interp *Interpreter) readFile(fd int, buf *SliceVal, n int) int {
	f, ok := interp.files[fd]
	if !ok {
		return -1
	}
	tmp := make([]byte, n)
	nRead, err := f.Read(tmp)
	if err != nil && nRead == 0 {
		return -1
	}
	// Copy into the slice value
	for i := 0; i < nRead && i < len(buf.Elems); i++ {
		buf.Elems[i] = &IntVal{Val: int64(tmp[i]), Typ: types.Typ_uint8}
	}
	return nRead
}

func (interp *Interpreter) writeFile(fd int, data *SliceVal, n int) int {
	// Special case: fd 1 (stdout) respects captured output
	if fd == 1 && interp.stdout != nil {
		for i := 0; i < n && i < len(data.Elems); i++ {
			if iv, ok := data.Elems[i].(*IntVal); ok {
				interp.stdout.WriteByte(byte(iv.Val))
			}
		}
		return n
	}
	if fd == 2 && interp.stdout != nil {
		// stderr still goes to real stderr even in test mode
		f := interp.files[fd]
		if f == nil {
			return -1
		}
		buf := make([]byte, n)
		for i := 0; i < n && i < len(data.Elems); i++ {
			if iv, ok := data.Elems[i].(*IntVal); ok {
				buf[i] = byte(iv.Val)
			}
		}
		nw, _ := f.Write(buf)
		return nw
	}

	f, ok := interp.files[fd]
	if !ok {
		return -1
	}
	buf := make([]byte, n)
	for i := 0; i < n && i < len(data.Elems); i++ {
		if iv, ok := data.Elems[i].(*IntVal); ok {
			buf[i] = byte(iv.Val)
		}
	}
	nWritten, err := f.Write(buf)
	if err != nil {
		return -1
	}
	return nWritten
}

func (interp *Interpreter) closeFile(fd int) int {
	f, ok := interp.files[fd]
	if !ok {
		return -1
	}
	// Don't close stdin/stdout/stderr
	if fd <= 2 {
		return 0
	}
	err := f.Close()
	delete(interp.files, fd)
	if err != nil {
		return -1
	}
	return 0
}
