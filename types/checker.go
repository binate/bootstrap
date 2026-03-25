package types

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/binate/bootstrap/ast"
	"github.com/binate/bootstrap/token"
)

// Checker performs type checking on an AST.
type Checker struct {
	file    *ast.File
	errors  []CheckError
	scope   *Scope
	funcRet []Type // expected return types of current function
}

// CheckError represents a type-checking error.
type CheckError struct {
	Pos token.Pos
	Msg string
}

func (e CheckError) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
}

// Symbol represents a named entity in a scope.
type Symbol struct {
	Name string
	Type Type
	Kind SymbolKind
}

// SymbolKind classifies what a symbol refers to.
type SymbolKind int

const (
	VarSym   SymbolKind = iota
	ConstSym
	TypeSym
	FuncSym
)

// Scope represents a lexical scope.
type Scope struct {
	parent  *Scope
	symbols map[string]*Symbol
}

func newScope(parent *Scope) *Scope {
	return &Scope{parent: parent, symbols: make(map[string]*Symbol)}
}

func (s *Scope) define(sym *Symbol) {
	s.symbols[sym.Name] = sym
}

func (s *Scope) lookup(name string) *Symbol {
	if sym, ok := s.symbols[name]; ok {
		return sym
	}
	if s.parent != nil {
		return s.parent.lookup(name)
	}
	return nil
}

// NewChecker creates a new type checker.
func NewChecker() *Checker {
	c := &Checker{}
	c.scope = c.universeScope()
	return c
}

// Errors returns all type-checking errors.
func (c *Checker) Errors() []CheckError {
	return c.errors
}

func (c *Checker) errorf(pos token.Pos, format string, args ...any) {
	c.errors = append(c.errors, CheckError{
		Pos: pos,
		Msg: fmt.Sprintf(format, args...),
	})
}

// universeScope creates the predeclared scope with built-in types.
func (c *Checker) universeScope() *Scope {
	s := newScope(nil)
	for name, typ := range PredeclaredTypes {
		if typ != nil {
			s.define(&Symbol{Name: name, Type: typ, Kind: TypeSym})
		}
	}
	// Predeclared constants
	s.define(&Symbol{Name: "true", Type: Typ_untypedBool, Kind: ConstSym})
	s.define(&Symbol{Name: "false", Type: Typ_untypedBool, Kind: ConstSym})
	s.define(&Symbol{Name: "nil", Type: Typ_nil, Kind: ConstSym})

	// Builtin functions — variadic ones get empty params (checked specially)
	variadicType := &FuncType{} // empty signature — checked specially
	s.define(&Symbol{Name: "print", Type: variadicType, Kind: FuncSym})
	s.define(&Symbol{Name: "println", Type: variadicType, Kind: FuncSym})
	s.define(&Symbol{Name: "exit", Type: &FuncType{
		Params: []*Param{{Name: "code", Type: Typ_int}},
	}, Kind: FuncSym})
	s.define(&Symbol{Name: "append", Type: variadicType, Kind: FuncSym})
	s.define(&Symbol{Name: "panic", Type: variadicType, Kind: FuncSym})
	s.define(&Symbol{Name: "string", Type: variadicType, Kind: FuncSym})

	// File I/O
	s.define(&Symbol{Name: "open", Type: &FuncType{
		Params:  []*Param{{Name: "path", Type: Typ_string}, {Name: "flags", Type: Typ_int}},
		Results: []Type{Typ_int},
	}, Kind: FuncSym})
	s.define(&Symbol{Name: "read", Type: &FuncType{
		Params:  []*Param{{Name: "fd", Type: Typ_int}, {Name: "buf", Type: &SliceType{Elem: Typ_uint8}}, {Name: "n", Type: Typ_int}},
		Results: []Type{Typ_int},
	}, Kind: FuncSym})
	s.define(&Symbol{Name: "write", Type: &FuncType{
		Params:  []*Param{{Name: "fd", Type: Typ_int}, {Name: "buf", Type: &SliceType{Elem: Typ_uint8}}, {Name: "n", Type: Typ_int}},
		Results: []Type{Typ_int},
	}, Kind: FuncSym})
	s.define(&Symbol{Name: "close", Type: &FuncType{
		Params:  []*Param{{Name: "fd", Type: Typ_int}},
		Results: []Type{Typ_int},
	}, Kind: FuncSym})

	// Process
	s.define(&Symbol{Name: "args", Type: &FuncType{
		Results: []Type{&SliceType{Elem: Typ_string}},
	}, Kind: FuncSym})
	return s
}

func (c *Checker) pushScope() {
	c.scope = newScope(c.scope)
}

func (c *Checker) popScope() {
	c.scope = c.scope.parent
}

// ============================================================
// File-level checking (two-pass)
// ============================================================

// Check type-checks a source file.
func (c *Checker) Check(file *ast.File) {
	c.file = file

	// Package scope
	c.pushScope()

	// Pass 1: collect all top-level declarations (types, functions, vars, consts).
	c.collectDecls(file.Decls)

	// Pass 2: check function bodies and variable initializers.
	c.checkDecls(file.Decls)

	c.popScope()
}

func (c *Checker) collectDecls(decls []ast.Decl) {
	for _, d := range decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			ft := c.resolveFuncType(d)
			c.scope.define(&Symbol{Name: d.Name.Name, Type: ft, Kind: FuncSym})
		case *ast.TypeDecl:
			c.collectTypeDecl(d)
		case *ast.VarDecl:
			// Defer full checking to pass 2, but register the name
			if d.Type != nil {
				typ := c.resolveTypeExpr(d.Type)
				c.scope.define(&Symbol{Name: d.Name.Name, Type: typ, Kind: VarSym})
			}
			// If type-inferred, we'll define it in pass 2
		case *ast.ConstDecl:
			// Defer to pass 2 for value evaluation
			if d.Type != nil && d.Value != nil {
				typ := c.resolveTypeExpr(d.Type)
				c.scope.define(&Symbol{Name: d.Name.Name, Type: typ, Kind: ConstSym})
			}
		case *ast.GroupDecl:
			c.collectDecls(d.Decls)
		}
	}
}

func (c *Checker) collectTypeDecl(d *ast.TypeDecl) {
	if d.Assign {
		// Type alias
		target := c.resolveTypeExpr(d.Type)
		alias := &AliasType{Name: d.Name.Name, Target: target}
		c.scope.define(&Symbol{Name: d.Name.Name, Type: alias, Kind: TypeSym})
	} else if st, ok := d.Type.(*ast.StructType); ok {
		// Named struct
		structType := c.resolveStructType(st)
		structType.Name = d.Name.Name
		c.scope.define(&Symbol{Name: d.Name.Name, Type: structType, Kind: TypeSym})
	} else {
		// Distinct type
		underlying := c.resolveTypeExpr(d.Type)
		named := &NamedType{Name: d.Name.Name, Underlying_: underlying}
		c.scope.define(&Symbol{Name: d.Name.Name, Type: named, Kind: TypeSym})
	}
}

func (c *Checker) checkDecls(decls []ast.Decl) {
	for _, d := range decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			c.checkFuncDecl(d)
		case *ast.VarDecl:
			c.checkVarDecl(d)
		case *ast.ConstDecl:
			c.checkConstDecl(d)
		case *ast.GroupDecl:
			c.checkDecls(d.Decls)
		case *ast.TypeDecl:
			// Already handled in pass 1
		}
	}
}

// ============================================================
// Type Resolution
// ============================================================

// resolveTypeExpr converts an AST type expression into a Type.
func (c *Checker) resolveTypeExpr(te ast.TypeExpr) Type {
	switch t := te.(type) {
	case *ast.NamedType:
		return c.resolveNamedTypeExpr(t)
	case *ast.PointerType:
		return &PointerType{Elem: c.resolveTypeExpr(t.Base)}
	case *ast.ManagedPtrType:
		return &ManagedPtrType{Elem: c.resolveTypeExpr(t.Base)}
	case *ast.ManagedSliceType:
		return &ManagedSliceType{Elem: c.resolveTypeExpr(t.Elem)}
	case *ast.SliceType:
		return &SliceType{Elem: c.resolveTypeExpr(t.Elem)}
	case *ast.ArrayType:
		length := c.evalConstInt(t.Len)
		return &ArrayType{Len: length, Elem: c.resolveTypeExpr(t.Elem)}
	case *ast.StructType:
		return c.resolveStructType(t)
	case *ast.ParenType:
		return c.resolveTypeExpr(t.Type)
	default:
		c.errorf(te.Pos(), "invalid type expression")
		return Typ_void
	}
}

func (c *Checker) resolveNamedTypeExpr(t *ast.NamedType) Type {
	if t.Pkg != nil {
		// Qualified type: pkg.Type — not fully supported in bootstrap
		c.errorf(t.Pos(), "qualified types not yet supported")
		return Typ_void
	}
	sym := c.scope.lookup(t.Name.Name)
	if sym == nil {
		c.errorf(t.Name.Pos(), "undefined type: %s", t.Name.Name)
		return Typ_void
	}
	if sym.Kind != TypeSym {
		c.errorf(t.Name.Pos(), "%s is not a type", t.Name.Name)
		return Typ_void
	}
	return sym.Type
}

func (c *Checker) resolveStructType(st *ast.StructType) *StructType {
	var fields []*Field
	for _, f := range st.Fields {
		typ := c.resolveTypeExpr(f.Type)
		name := ""
		if f.Name != nil {
			name = f.Name.Name
		}
		fields = append(fields, &Field{Name: name, Type: typ})
	}
	return &StructType{Fields: fields}
}

func (c *Checker) resolveFuncType(d *ast.FuncDecl) *FuncType {
	var params []*Param
	for _, p := range d.Params {
		typ := c.resolveTypeExpr(p.Type)
		params = append(params, &Param{Name: p.Name.Name, Type: typ})
	}
	var results []Type
	for _, r := range d.Results {
		results = append(results, c.resolveTypeExpr(r))
	}
	return &FuncType{Params: params, Results: results}
}

// ============================================================
// Declaration Checking
// ============================================================

func (c *Checker) checkFuncDecl(d *ast.FuncDecl) {
	sym := c.scope.lookup(d.Name.Name)
	if sym == nil {
		return // already errored
	}
	ft, ok := sym.Type.(*FuncType)
	if !ok {
		return
	}

	c.pushScope()

	// Define parameters
	for _, p := range ft.Params {
		c.scope.define(&Symbol{Name: p.Name, Type: p.Type, Kind: VarSym})
	}

	// Set expected return types
	oldRet := c.funcRet
	c.funcRet = ft.Results

	c.checkBlock(d.Body)

	c.funcRet = oldRet
	c.popScope()
}

func (c *Checker) checkVarDecl(d *ast.VarDecl) {
	if d.Type != nil && d.Value != nil {
		// var x T = expr
		declType := c.resolveTypeExpr(d.Type)
		valType := c.checkExpr(d.Value)
		if !AssignableTo(valType, declType) {
			c.errorf(d.Value.Pos(), "cannot assign %s to %s", valType, declType)
		}
		c.scope.define(&Symbol{Name: d.Name.Name, Type: declType, Kind: VarSym})
	} else if d.Type != nil {
		// var x T (no init)
		declType := c.resolveTypeExpr(d.Type)
		c.scope.define(&Symbol{Name: d.Name.Name, Type: declType, Kind: VarSym})
	} else if d.Value != nil {
		// var x = expr (inferred)
		valType := c.checkExpr(d.Value)
		valType = c.defaultType(valType)
		c.scope.define(&Symbol{Name: d.Name.Name, Type: valType, Kind: VarSym})
	}
}

func (c *Checker) checkConstDecl(d *ast.ConstDecl) {
	if d.Value == nil {
		// Bare name in grouped const (repeat) — not fully implemented
		return
	}
	valType := c.checkExpr(d.Value)
	if d.Type != nil {
		declType := c.resolveTypeExpr(d.Type)
		if !AssignableTo(valType, declType) {
			c.errorf(d.Value.Pos(), "cannot assign %s to %s", valType, declType)
		}
		c.scope.define(&Symbol{Name: d.Name.Name, Type: declType, Kind: ConstSym})
	} else {
		c.scope.define(&Symbol{Name: d.Name.Name, Type: valType, Kind: ConstSym})
	}
}

// ============================================================
// Statement Checking
// ============================================================

func (c *Checker) checkBlock(b *ast.Block) {
	for _, s := range b.Stmts {
		c.checkStmt(s)
	}
}

func (c *Checker) checkStmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.Block:
		c.pushScope()
		c.checkBlock(s)
		c.popScope()

	case *ast.ExprStmt:
		c.checkExpr(s.X)

	case *ast.AssignStmt:
		c.checkAssignStmt(s)

	case *ast.ShortVarDecl:
		c.checkShortVarDecl(s)

	case *ast.IncDecStmt:
		t := c.checkExpr(s.X)
		if !IsInteger(t) {
			c.errorf(s.X.Pos(), "cannot apply %s to %s", s.Op, t)
		}

	case *ast.ReturnStmt:
		c.checkReturnStmt(s)

	case *ast.IfStmt:
		cond := c.checkExpr(s.Cond)
		if !IsBool(cond) {
			c.errorf(s.Cond.Pos(), "non-bool condition: %s", cond)
		}
		c.pushScope()
		c.checkBlock(s.Body)
		c.popScope()
		if s.Else != nil {
			c.checkStmt(s.Else)
		}

	case *ast.ForStmt:
		c.checkForStmt(s)

	case *ast.SwitchStmt:
		c.checkSwitchStmt(s)

	case *ast.VarDecl:
		c.checkVarDecl(s)

	case *ast.ConstDecl:
		c.checkConstDecl(s)

	case *ast.TypeDecl:
		c.collectTypeDecl(s)

	case *ast.EmptyStmt:
		// nothing

	case *ast.BreakStmt, *ast.ContinueStmt:
		// nothing to check here

	default:
		c.errorf(s.Pos(), "unexpected statement type %T", s)
	}
}

func (c *Checker) checkAssignStmt(s *ast.AssignStmt) {
	if s.Op == token.ASSIGN {
		// Simple assignment: x, y = a, b
		if len(s.LHS) != len(s.RHS) {
			// Allow f() returning multiple values
			if len(s.RHS) == 1 {
				rhsType := c.checkExpr(s.RHS[0])
				if ft, ok := rhsType.(*FuncType); ok && len(ft.Results) == len(s.LHS) {
					// Multi-return assignment — check individual types
					for i, lhs := range s.LHS {
						lhsType := c.checkExpr(lhs)
						if !AssignableTo(ft.Results[i], lhsType) {
							c.errorf(lhs.Pos(), "cannot assign %s to %s", ft.Results[i], lhsType)
						}
					}
					return
				}
			}
			c.errorf(s.LHS[0].Pos(), "assignment count mismatch: %d = %d", len(s.LHS), len(s.RHS))
			return
		}
		for i := range s.LHS {
			lhsType := c.checkExpr(s.LHS[i])
			rhsType := c.checkExpr(s.RHS[i])
			if !AssignableTo(rhsType, lhsType) {
				c.errorf(s.RHS[i].Pos(), "cannot assign %s to %s", rhsType, lhsType)
			}
		}
	} else {
		// Compound assignment: x += 1
		lhsType := c.checkExpr(s.LHS[0])
		rhsType := c.checkExpr(s.RHS[0])
		// Check that the operation is valid
		baseOp := compoundToBase(s.Op)
		c.checkBinaryOp(s.LHS[0].Pos(), baseOp, lhsType, rhsType)
	}
}

func (c *Checker) checkShortVarDecl(s *ast.ShortVarDecl) {
	if len(s.Names) != len(s.RHS) {
		// Handle multi-return
		if len(s.RHS) == 1 {
			rhsType := c.checkExpr(s.RHS[0])
			// Check if it's a call returning multiple values
			if call, ok := s.RHS[0].(*ast.CallExpr); ok {
				fnType := c.checkExpr(call.Fun)
				if ft, ok := fnType.(*FuncType); ok && len(ft.Results) == len(s.Names) {
					for i, name := range s.Names {
						c.scope.define(&Symbol{Name: name.Name, Type: ft.Results[i], Kind: VarSym})
					}
					return
				}
			}
			_ = rhsType
		}
		c.errorf(s.Names[0].Pos(), "assignment count mismatch: %d := %d", len(s.Names), len(s.RHS))
		return
	}
	for i, name := range s.Names {
		rhsType := c.checkExpr(s.RHS[i])
		rhsType = c.defaultType(rhsType)
		c.scope.define(&Symbol{Name: name.Name, Type: rhsType, Kind: VarSym})
	}
}

func (c *Checker) checkReturnStmt(s *ast.ReturnStmt) {
	if len(c.funcRet) == 0 && len(s.Results) == 0 {
		return
	}
	if len(s.Results) != len(c.funcRet) {
		c.errorf(s.Pos(), "wrong number of return values: got %d, want %d",
			len(s.Results), len(c.funcRet))
		return
	}
	for i, r := range s.Results {
		rt := c.checkExpr(r)
		if !AssignableTo(rt, c.funcRet[i]) {
			c.errorf(r.Pos(), "cannot use %s as return type %s", rt, c.funcRet[i])
		}
	}
}

func (c *Checker) checkForStmt(s *ast.ForStmt) {
	c.pushScope()

	if s.Iter != nil {
		// For-in
		iterType := c.checkExpr(s.Iter)
		elemType := c.forInElemType(s.Iter.Pos(), iterType)
		if s.Key != nil {
			c.scope.define(&Symbol{Name: s.Key.Name, Type: Typ_int, Kind: VarSym})
		}
		if s.Value != nil {
			c.scope.define(&Symbol{Name: s.Value.Name, Type: elemType, Kind: VarSym})
		}
	} else {
		// C-style or while-style
		if s.Init != nil {
			c.checkStmt(s.Init)
		}
		if s.Cond != nil {
			cond := c.checkExpr(s.Cond)
			if !IsBool(cond) {
				c.errorf(s.Cond.Pos(), "non-bool condition: %s", cond)
			}
		}
		if s.Post != nil {
			c.checkStmt(s.Post)
		}
	}

	c.checkBlock(s.Body)
	c.popScope()
}

func (c *Checker) forInElemType(pos token.Pos, t Type) Type {
	t = ResolveAlias(t)
	switch st := t.(type) {
	case *SliceType:
		return st.Elem
	case *ManagedSliceType:
		return st.Elem
	case *ArrayType:
		return st.Elem
	}
	c.errorf(pos, "cannot range over %s", t)
	return Typ_void
}

func (c *Checker) checkSwitchStmt(s *ast.SwitchStmt) {
	var tagType Type
	if s.Tag != nil {
		tagType = c.checkExpr(s.Tag)
	}
	for _, cc := range s.Cases {
		for _, expr := range cc.Exprs {
			ct := c.checkExpr(expr)
			if tagType != nil && !AssignableTo(ct, tagType) {
				c.errorf(expr.Pos(), "cannot compare %s to %s", ct, tagType)
			}
		}
		c.pushScope()
		for _, stmt := range cc.Body {
			c.checkStmt(stmt)
		}
		c.popScope()
	}
}

// ============================================================
// Expression Checking
// ============================================================

// checkExpr type-checks an expression and returns its type.
func (c *Checker) checkExpr(e ast.Expr) Type {
	switch e := e.(type) {
	case *ast.Ident:
		return c.checkIdent(e)
	case *ast.IntLit:
		return Typ_untypedInt
	case *ast.StringLit:
		return Typ_string
	case *ast.CharLit:
		return Typ_char
	case *ast.BoolLit:
		return Typ_untypedBool
	case *ast.NilLit:
		return Typ_nil
	case *ast.BinaryExpr:
		return c.checkBinaryExpr(e)
	case *ast.UnaryExpr:
		return c.checkUnaryExpr(e)
	case *ast.CallExpr:
		return c.checkCallExpr(e)
	case *ast.IndexExpr:
		return c.checkIndexExpr(e)
	case *ast.SliceExpr:
		return c.checkSliceExpr(e)
	case *ast.SelectorExpr:
		return c.checkSelectorExpr(e)
	case *ast.CompositeLit:
		return c.checkCompositeLit(e)
	case *ast.BuiltinCall:
		return c.checkBuiltinCall(e)
	default:
		c.errorf(e.Pos(), "unexpected expression type %T", e)
		return Typ_void
	}
}

func (c *Checker) checkIdent(e *ast.Ident) Type {
	sym := c.scope.lookup(e.Name)
	if sym == nil {
		c.errorf(e.Pos(), "undefined: %s", e.Name)
		return Typ_void
	}
	return sym.Type
}

func (c *Checker) checkBinaryExpr(e *ast.BinaryExpr) Type {
	lt := c.checkExpr(e.X)
	rt := c.checkExpr(e.Y)
	return c.checkBinaryOp(e.X.Pos(), e.Op, lt, rt)
}

func (c *Checker) checkBinaryOp(pos token.Pos, op token.Type, lt, rt Type) Type {
	switch op {
	case token.PLUS, token.MINUS, token.STAR, token.SLASH, token.PERCENT:
		return c.checkArithOp(pos, op, lt, rt)
	case token.AMP, token.PIPE, token.CARET, token.SHL, token.SHR:
		return c.checkBitwiseOp(pos, op, lt, rt)
	case token.EQ, token.NEQ, token.LT, token.GT, token.LEQ, token.GEQ:
		return c.checkCompareOp(pos, op, lt, rt)
	case token.LAND, token.LOR:
		if !IsBool(lt) {
			c.errorf(pos, "operator %s requires bool operands, got %s", op, lt)
		}
		if !IsBool(rt) {
			c.errorf(pos, "operator %s requires bool operands, got %s", op, rt)
		}
		return Typ_untypedBool
	default:
		c.errorf(pos, "invalid binary operator %s", op)
		return Typ_void
	}
}

func (c *Checker) checkArithOp(pos token.Pos, op token.Type, lt, rt Type) Type {
	if !IsNumeric(lt) {
		c.errorf(pos, "operator %s requires numeric operands, got %s", op, lt)
		return Typ_void
	}
	if !IsNumeric(rt) {
		c.errorf(pos, "operator %s requires numeric operands, got %s", op, rt)
		return Typ_void
	}
	return c.commonType(pos, lt, rt)
}

func (c *Checker) checkBitwiseOp(pos token.Pos, op token.Type, lt, rt Type) Type {
	if !IsInteger(lt) {
		c.errorf(pos, "operator %s requires integer operands, got %s", op, lt)
		return Typ_void
	}
	if !IsInteger(rt) {
		c.errorf(pos, "operator %s requires integer operands, got %s", op, rt)
		return Typ_void
	}
	return c.commonType(pos, lt, rt)
}

func (c *Checker) checkCompareOp(pos token.Pos, op token.Type, lt, rt Type) Type {
	// Allow comparison of compatible types
	if !AssignableTo(lt, rt) && !AssignableTo(rt, lt) {
		c.errorf(pos, "cannot compare %s and %s", lt, rt)
	}
	return Typ_untypedBool
}

func (c *Checker) checkUnaryExpr(e *ast.UnaryExpr) Type {
	xt := c.checkExpr(e.X)
	switch e.Op {
	case token.MINUS:
		if !IsNumeric(xt) {
			c.errorf(e.OpPos, "operator - requires numeric operand, got %s", xt)
		}
		return xt
	case token.NOT:
		if !IsBool(xt) {
			c.errorf(e.OpPos, "operator ! requires bool operand, got %s", xt)
		}
		return Typ_untypedBool
	case token.TILDE:
		if !IsInteger(xt) {
			c.errorf(e.OpPos, "operator ~ requires integer operand, got %s", xt)
		}
		return xt
	case token.STAR: // dereference
		elem := PointerElem(xt)
		if elem == nil {
			c.errorf(e.OpPos, "cannot dereference non-pointer type %s", xt)
			return Typ_void
		}
		return elem
	case token.AMP: // address-of
		return &PointerType{Elem: xt}
	default:
		c.errorf(e.OpPos, "invalid unary operator %s", e.Op)
		return Typ_void
	}
}

func (c *Checker) checkCallExpr(e *ast.CallExpr) Type {
	fnType := c.checkExpr(e.Fun)
	ft, ok := fnType.(*FuncType)
	if !ok {
		c.errorf(e.Fun.Pos(), "cannot call non-function %s", fnType)
		return Typ_void
	}
	// Variadic builtins (print, println, append, etc.) have empty param lists — just check args.
	if len(ft.Params) == 0 && len(e.Args) > 0 {
		for _, arg := range e.Args {
			c.checkExpr(arg)
		}
		// Infer return type for known variadic builtins
		if ident, ok := e.Fun.(*ast.Ident); ok {
			switch ident.Name {
			case "append":
				// append returns the same type as its first argument
				if len(e.Args) > 0 {
					return c.checkExpr(e.Args[0])
				}
			case "string":
				return Typ_string
			}
		}
	} else if len(e.Args) != len(ft.Params) {
		c.errorf(e.Fun.Pos(), "wrong number of arguments: got %d, want %d",
			len(e.Args), len(ft.Params))
	} else {
		for i, arg := range e.Args {
			at := c.checkExpr(arg)
			if !AssignableTo(at, ft.Params[i].Type) {
				c.errorf(arg.Pos(), "cannot pass %s as %s", at, ft.Params[i].Type)
			}
		}
	}
	if len(ft.Results) == 0 {
		return Typ_void
	}
	if len(ft.Results) == 1 {
		return ft.Results[0]
	}
	// Multiple returns — return the FuncType itself for multi-assign detection
	return ft
}

func (c *Checker) checkIndexExpr(e *ast.IndexExpr) Type {
	xt := c.checkExpr(e.X)
	idxType := c.checkExpr(e.Index)
	if !IsInteger(idxType) {
		c.errorf(e.Index.Pos(), "index must be integer, got %s", idxType)
	}

	xt = ResolveAlias(xt)
	switch t := xt.(type) {
	case *SliceType:
		return t.Elem
	case *ManagedSliceType:
		return t.Elem
	case *ArrayType:
		return t.Elem
	case *PointerType:
		// Pointer indexing — like C: p[i]
		return t.Elem
	default:
		c.errorf(e.X.Pos(), "cannot index %s", xt)
		return Typ_void
	}
}

func (c *Checker) checkSliceExpr(e *ast.SliceExpr) Type {
	xt := c.checkExpr(e.X)
	if e.Lo != nil {
		lt := c.checkExpr(e.Lo)
		if !IsInteger(lt) {
			c.errorf(e.Lo.Pos(), "slice index must be integer, got %s", lt)
		}
	}
	if e.Hi != nil {
		ht := c.checkExpr(e.Hi)
		if !IsInteger(ht) {
			c.errorf(e.Hi.Pos(), "slice index must be integer, got %s", ht)
		}
	}

	xt = ResolveAlias(xt)
	switch t := xt.(type) {
	case *SliceType:
		return t
	case *ManagedSliceType:
		return t
	case *ArrayType:
		return &SliceType{Elem: t.Elem}
	default:
		c.errorf(e.X.Pos(), "cannot slice %s", xt)
		return Typ_void
	}
}

func (c *Checker) checkSelectorExpr(e *ast.SelectorExpr) Type {
	xt := c.checkExpr(e.X)

	// Auto-dereference pointers for field access
	if elem := PointerElem(xt); elem != nil {
		xt = elem
	}

	xt = ResolveAlias(xt)
	if st, ok := xt.(*StructType); ok {
		f := st.FieldByName(e.Sel.Name)
		if f == nil {
			c.errorf(e.Sel.Pos(), "%s has no field %s", st, e.Sel.Name)
			return Typ_void
		}
		return f.Type
	}
	if nt, ok := xt.(*NamedType); ok {
		if st, ok := nt.Underlying_.(*StructType); ok {
			f := st.FieldByName(e.Sel.Name)
			if f == nil {
				c.errorf(e.Sel.Pos(), "%s has no field %s", nt.Name, e.Sel.Name)
				return Typ_void
			}
			return f.Type
		}
	}

	c.errorf(e.Sel.Pos(), "cannot access field %s on %s", e.Sel.Name, xt)
	return Typ_void
}

func (c *Checker) checkCompositeLit(e *ast.CompositeLit) Type {
	typ := c.resolveTypeExpr(e.Type)
	xt := ResolveAlias(typ)

	switch st := xt.(type) {
	case *StructType:
		c.checkStructLit(e, st)
	case *ArrayType:
		c.checkArrayLit(e, st)
	case *NamedType:
		if inner, ok := st.Underlying_.(*StructType); ok {
			c.checkStructLit(e, inner)
		}
	}

	return typ
}

func (c *Checker) checkStructLit(e *ast.CompositeLit, st *StructType) {
	for _, elem := range e.Elements {
		if elem.Key != nil {
			// Keyed element: field name
			if ident, ok := elem.Key.(*ast.Ident); ok {
				f := st.FieldByName(ident.Name)
				if f == nil {
					c.errorf(ident.Pos(), "%s has no field %s", st, ident.Name)
					continue
				}
				vt := c.checkExpr(elem.Value)
				if !AssignableTo(vt, f.Type) {
					c.errorf(elem.Value.Pos(), "cannot assign %s to field %s of type %s",
						vt, ident.Name, f.Type)
				}
			}
		} else {
			c.checkExpr(elem.Value)
		}
	}
}

func (c *Checker) checkArrayLit(e *ast.CompositeLit, at *ArrayType) {
	for _, elem := range e.Elements {
		vt := c.checkExpr(elem.Value)
		if !AssignableTo(vt, at.Elem) {
			c.errorf(elem.Value.Pos(), "cannot use %s as %s in array literal",
				vt, at.Elem)
		}
	}
}

func (c *Checker) checkBuiltinCall(e *ast.BuiltinCall) Type {
	switch e.Builtin {
	case token.MAKE:
		typ := c.resolveTypeExpr(e.Type)
		if len(e.Args) > 0 {
			at := c.checkExpr(e.Args[0])
			if !IsInteger(at) {
				c.errorf(e.Args[0].Pos(), "make size must be integer, got %s", at)
			}
		}
		// make returns @T or @[]T depending on the type
		if _, ok := typ.(*SliceType); ok {
			return &ManagedSliceType{Elem: typ.(*SliceType).Elem}
		}
		return &ManagedPtrType{Elem: typ}

	case token.BOX:
		if len(e.Args) != 1 {
			c.errorf(e.Pos(), "box requires exactly one argument")
			return Typ_void
		}
		argType := c.checkExpr(e.Args[0])
		argType = c.defaultType(argType)
		return &ManagedPtrType{Elem: argType}

	case token.CAST:
		typ := c.resolveTypeExpr(e.Type)
		if len(e.Args) != 1 {
			c.errorf(e.Pos(), "cast requires exactly one argument")
			return typ
		}
		c.checkExpr(e.Args[0])
		// Cast validity checking deferred for simplicity
		return typ

	case token.BIT_CAST:
		typ := c.resolveTypeExpr(e.Type)
		if len(e.Args) != 1 {
			c.errorf(e.Pos(), "bit_cast requires exactly one argument")
			return typ
		}
		c.checkExpr(e.Args[0])
		return typ

	case token.LEN:
		if len(e.Args) != 1 {
			c.errorf(e.Pos(), "len requires exactly one argument")
			return Typ_int
		}
		argType := c.checkExpr(e.Args[0])
		argType = ResolveAlias(argType)
		if !IsSlice(argType) {
			if _, ok := argType.(*ArrayType); !ok {
				c.errorf(e.Args[0].Pos(), "len argument must be slice or array, got %s", argType)
			}
		}
		return Typ_int

	default:
		c.errorf(e.Pos(), "unknown builtin %s", e.Builtin)
		return Typ_void
	}
}

// ============================================================
// Helpers
// ============================================================

// commonType returns the common type of two operands, handling untyped constants.
func (c *Checker) commonType(pos token.Pos, a, b Type) Type {
	if _, ok := a.(*UntypedIntType); ok {
		return b
	}
	if _, ok := b.(*UntypedIntType); ok {
		return a
	}
	if !Identical(a, b) {
		c.errorf(pos, "mismatched types %s and %s", a, b)
	}
	return a
}

// defaultType returns the default concrete type for an untyped type.
func (c *Checker) defaultType(t Type) Type {
	switch t.(type) {
	case *UntypedIntType:
		return Typ_int
	case *UntypedBoolType:
		return Typ_bool
	}
	return t
}

// compoundToBase converts a compound assignment operator to its base operator.
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

// evalConstInt evaluates a constant integer expression (for array lengths).
func (c *Checker) evalConstInt(e ast.Expr) int64 {
	switch e := e.(type) {
	case *ast.IntLit:
		v, err := parseIntLit(e.Value)
		if err != nil {
			c.errorf(e.Pos(), "invalid integer literal: %s", e.Value)
			return 0
		}
		return v
	case *ast.Ident:
		sym := c.scope.lookup(e.Name)
		if sym != nil && sym.Kind == ConstSym {
			// TODO: evaluate const expressions properly
			return 0
		}
		c.errorf(e.Pos(), "array length must be a constant")
		return 0
	default:
		c.errorf(e.Pos(), "array length must be a constant integer expression")
		return 0
	}
}

// parseIntLit parses an integer literal string.
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
