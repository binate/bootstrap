package parser

import (
	"testing"

	"github.com/binate/bootstrap/ast"
	"github.com/binate/bootstrap/token"
)

// Helper to create a parser from a string.
func parse(t *testing.T, input string) *Parser {
	t.Helper()
	return New([]byte(input), "test.bn")
}

// Helper to check for parse errors.
func noErrors(t *testing.T, p *Parser) {
	t.Helper()
	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Errorf("parse error: %s", e)
		}
	}
}

func TestParseIntLiteral(t *testing.T) {
	p := parse(t, "42")
	expr := p.ParseExpr()
	noErrors(t, p)
	lit, ok := expr.(*ast.IntLit)
	if !ok {
		t.Fatalf("expected *ast.IntLit, got %T", expr)
	}
	if lit.Value != "42" {
		t.Errorf("expected 42, got %s", lit.Value)
	}
}

func TestParseStringLiteral(t *testing.T) {
	p := parse(t, `"hello"`)
	expr := p.ParseExpr()
	noErrors(t, p)
	lit, ok := expr.(*ast.StringLit)
	if !ok {
		t.Fatalf("expected *ast.StringLit, got %T", expr)
	}
	if lit.Value != `"hello"` {
		t.Errorf("expected %q, got %q", `"hello"`, lit.Value)
	}
}

func TestParseCharLiteral(t *testing.T) {
	p := parse(t, "'a'")
	expr := p.ParseExpr()
	noErrors(t, p)
	lit, ok := expr.(*ast.CharLit)
	if !ok {
		t.Fatalf("expected *ast.CharLit, got %T", expr)
	}
	if lit.Value != "'a'" {
		t.Errorf("expected 'a', got %s", lit.Value)
	}
}

func TestParseBoolLiterals(t *testing.T) {
	p := parse(t, "true")
	expr := p.ParseExpr()
	noErrors(t, p)
	lit, ok := expr.(*ast.BoolLit)
	if !ok {
		t.Fatalf("expected *ast.BoolLit, got %T", expr)
	}
	if !lit.Value {
		t.Error("expected true")
	}

	p = parse(t, "false")
	expr = p.ParseExpr()
	noErrors(t, p)
	lit, ok = expr.(*ast.BoolLit)
	if !ok {
		t.Fatalf("expected *ast.BoolLit, got %T", expr)
	}
	if lit.Value {
		t.Error("expected false")
	}
}

func TestParseNil(t *testing.T) {
	p := parse(t, "nil")
	expr := p.ParseExpr()
	noErrors(t, p)
	_, ok := expr.(*ast.NilLit)
	if !ok {
		t.Fatalf("expected *ast.NilLit, got %T", expr)
	}
}

func TestParseIdentifier(t *testing.T) {
	p := parse(t, "foo")
	expr := p.ParseExpr()
	noErrors(t, p)
	ident, ok := expr.(*ast.Ident)
	if !ok {
		t.Fatalf("expected *ast.Ident, got %T", expr)
	}
	if ident.Name != "foo" {
		t.Errorf("expected foo, got %s", ident.Name)
	}
}

func TestParseBinaryAdd(t *testing.T) {
	p := parse(t, "1 + 2")
	expr := p.ParseExpr()
	noErrors(t, p)
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected *ast.BinaryExpr, got %T", expr)
	}
	if bin.Op != token.PLUS {
		t.Errorf("expected +, got %s", bin.Op)
	}
	assertIntLit(t, bin.X, "1")
	assertIntLit(t, bin.Y, "2")
}

func TestParsePrecedence(t *testing.T) {
	// 1 + 2 * 3 should parse as 1 + (2 * 3)
	p := parse(t, "1 + 2 * 3")
	expr := p.ParseExpr()
	noErrors(t, p)
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected *ast.BinaryExpr, got %T", expr)
	}
	if bin.Op != token.PLUS {
		t.Errorf("expected +, got %s", bin.Op)
	}
	assertIntLit(t, bin.X, "1")
	rhs, ok := bin.Y.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected *ast.BinaryExpr for RHS, got %T", bin.Y)
	}
	if rhs.Op != token.STAR {
		t.Errorf("expected *, got %s", rhs.Op)
	}
}

func TestParsePrecedenceParens(t *testing.T) {
	// (1 + 2) * 3 should parse as (1 + 2) * 3
	p := parse(t, "(1 + 2) * 3")
	expr := p.ParseExpr()
	noErrors(t, p)
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected *ast.BinaryExpr, got %T", expr)
	}
	if bin.Op != token.STAR {
		t.Errorf("expected *, got %s", bin.Op)
	}
}

func TestParseUnaryMinus(t *testing.T) {
	p := parse(t, "-x")
	expr := p.ParseExpr()
	noErrors(t, p)
	un, ok := expr.(*ast.UnaryExpr)
	if !ok {
		t.Fatalf("expected *ast.UnaryExpr, got %T", expr)
	}
	if un.Op != token.MINUS {
		t.Errorf("expected -, got %s", un.Op)
	}
	assertIdent(t, un.X, "x")
}

func TestParseUnaryDeref(t *testing.T) {
	// *p * x = (*p) * x
	p := parse(t, "*p * x")
	expr := p.ParseExpr()
	noErrors(t, p)
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected *ast.BinaryExpr, got %T", expr)
	}
	if bin.Op != token.STAR {
		t.Errorf("expected * (mul), got %s", bin.Op)
	}
	un, ok := bin.X.(*ast.UnaryExpr)
	if !ok {
		t.Fatalf("expected *ast.UnaryExpr for LHS, got %T", bin.X)
	}
	if un.Op != token.STAR {
		t.Errorf("expected * (deref), got %s", un.Op)
	}
}

func TestParseUnaryAddressOf(t *testing.T) {
	p := parse(t, "&x")
	expr := p.ParseExpr()
	noErrors(t, p)
	un, ok := expr.(*ast.UnaryExpr)
	if !ok {
		t.Fatalf("expected *ast.UnaryExpr, got %T", expr)
	}
	if un.Op != token.AMP {
		t.Errorf("expected &, got %s", un.Op)
	}
}

func TestParseUnaryNot(t *testing.T) {
	p := parse(t, "!ok")
	expr := p.ParseExpr()
	noErrors(t, p)
	un, ok := expr.(*ast.UnaryExpr)
	if !ok {
		t.Fatalf("expected *ast.UnaryExpr, got %T", expr)
	}
	if un.Op != token.NOT {
		t.Errorf("expected !, got %s", un.Op)
	}
}

func TestParseUnaryBitNot(t *testing.T) {
	p := parse(t, "~x")
	expr := p.ParseExpr()
	noErrors(t, p)
	un, ok := expr.(*ast.UnaryExpr)
	if !ok {
		t.Fatalf("expected *ast.UnaryExpr, got %T", expr)
	}
	if un.Op != token.TILDE {
		t.Errorf("expected ~, got %s", un.Op)
	}
}

func TestParseComparison(t *testing.T) {
	p := parse(t, "a == b")
	expr := p.ParseExpr()
	noErrors(t, p)
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected *ast.BinaryExpr, got %T", expr)
	}
	if bin.Op != token.EQ {
		t.Errorf("expected ==, got %s", bin.Op)
	}
}

func TestParseLogicalOps(t *testing.T) {
	// a && b || c should parse as (a && b) || c
	p := parse(t, "a && b || c")
	expr := p.ParseExpr()
	noErrors(t, p)
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected *ast.BinaryExpr, got %T", expr)
	}
	if bin.Op != token.LOR {
		t.Errorf("expected ||, got %s", bin.Op)
	}
	lhs, ok := bin.X.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected *ast.BinaryExpr for LHS, got %T", bin.X)
	}
	if lhs.Op != token.LAND {
		t.Errorf("expected &&, got %s", lhs.Op)
	}
}

func TestParseBitwiseOps(t *testing.T) {
	// a & b | c should parse as (a & b) | c
	p := parse(t, "a & b | c")
	expr := p.ParseExpr()
	noErrors(t, p)
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected *ast.BinaryExpr, got %T", expr)
	}
	if bin.Op != token.PIPE {
		t.Errorf("expected |, got %s", bin.Op)
	}
}

func TestParseShift(t *testing.T) {
	p := parse(t, "x << 2")
	expr := p.ParseExpr()
	noErrors(t, p)
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected *ast.BinaryExpr, got %T", expr)
	}
	if bin.Op != token.SHL {
		t.Errorf("expected <<, got %s", bin.Op)
	}
}

func TestParseSelectorExpr(t *testing.T) {
	p := parse(t, "x.y")
	expr := p.ParseExpr()
	noErrors(t, p)
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		t.Fatalf("expected *ast.SelectorExpr, got %T", expr)
	}
	assertIdent(t, sel.X, "x")
	if sel.Sel.Name != "y" {
		t.Errorf("expected y, got %s", sel.Sel.Name)
	}
}

func TestParseChainedSelector(t *testing.T) {
	p := parse(t, "a.b.c")
	expr := p.ParseExpr()
	noErrors(t, p)
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		t.Fatalf("expected *ast.SelectorExpr, got %T", expr)
	}
	if sel.Sel.Name != "c" {
		t.Errorf("expected c, got %s", sel.Sel.Name)
	}
	inner, ok := sel.X.(*ast.SelectorExpr)
	if !ok {
		t.Fatalf("expected *ast.SelectorExpr for inner, got %T", sel.X)
	}
	if inner.Sel.Name != "b" {
		t.Errorf("expected b, got %s", inner.Sel.Name)
	}
}

func TestParseIndexExpr(t *testing.T) {
	p := parse(t, "a[0]")
	expr := p.ParseExpr()
	noErrors(t, p)
	idx, ok := expr.(*ast.IndexExpr)
	if !ok {
		t.Fatalf("expected *ast.IndexExpr, got %T", expr)
	}
	assertIdent(t, idx.X, "a")
	assertIntLit(t, idx.Index, "0")
}

func TestParseSliceExpr(t *testing.T) {
	p := parse(t, "a[1:3]")
	expr := p.ParseExpr()
	noErrors(t, p)
	sl, ok := expr.(*ast.SliceExpr)
	if !ok {
		t.Fatalf("expected *ast.SliceExpr, got %T", expr)
	}
	assertIdent(t, sl.X, "a")
	assertIntLit(t, sl.Lo, "1")
	assertIntLit(t, sl.Hi, "3")
}

func TestParseSliceExprNoLow(t *testing.T) {
	p := parse(t, "a[:3]")
	expr := p.ParseExpr()
	noErrors(t, p)
	sl, ok := expr.(*ast.SliceExpr)
	if !ok {
		t.Fatalf("expected *ast.SliceExpr, got %T", expr)
	}
	if sl.Lo != nil {
		t.Error("expected nil Lo")
	}
	assertIntLit(t, sl.Hi, "3")
}

func TestParseSliceExprNoHigh(t *testing.T) {
	p := parse(t, "a[1:]")
	expr := p.ParseExpr()
	noErrors(t, p)
	sl, ok := expr.(*ast.SliceExpr)
	if !ok {
		t.Fatalf("expected *ast.SliceExpr, got %T", expr)
	}
	assertIntLit(t, sl.Lo, "1")
	if sl.Hi != nil {
		t.Error("expected nil Hi")
	}
}

func TestParseCallExpr(t *testing.T) {
	p := parse(t, "foo(1, 2)")
	expr := p.ParseExpr()
	noErrors(t, p)
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected *ast.CallExpr, got %T", expr)
	}
	assertIdent(t, call.Fun, "foo")
	if len(call.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(call.Args))
	}
}

func TestParseCallNoArgs(t *testing.T) {
	p := parse(t, "foo()")
	expr := p.ParseExpr()
	noErrors(t, p)
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected *ast.CallExpr, got %T", expr)
	}
	if len(call.Args) != 0 {
		t.Errorf("expected 0 args, got %d", len(call.Args))
	}
}

func TestParseCallTrailingComma(t *testing.T) {
	p := parse(t, "foo(1, 2,)")
	expr := p.ParseExpr()
	noErrors(t, p)
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected *ast.CallExpr, got %T", expr)
	}
	if len(call.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(call.Args))
	}
}

func TestParseMethodCall(t *testing.T) {
	// p.foo(x) parses as call(selector(p, foo), [x])
	p := parse(t, "p.foo(x)")
	expr := p.ParseExpr()
	noErrors(t, p)
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected *ast.CallExpr, got %T", expr)
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		t.Fatalf("expected *ast.SelectorExpr, got %T", call.Fun)
	}
	assertIdent(t, sel.X, "p")
	if sel.Sel.Name != "foo" {
		t.Errorf("expected foo, got %s", sel.Sel.Name)
	}
}

func TestParseCompositeLit(t *testing.T) {
	p := parse(t, "Point{x: 1, y: 2}")
	expr := p.ParseExpr()
	noErrors(t, p)
	cl, ok := expr.(*ast.CompositeLit)
	if !ok {
		t.Fatalf("expected *ast.CompositeLit, got %T", expr)
	}
	nt, ok := cl.Type.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected *ast.NamedType, got %T", cl.Type)
	}
	if nt.Name.Name != "Point" {
		t.Errorf("expected Point, got %s", nt.Name.Name)
	}
	if len(cl.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(cl.Elements))
	}
	// Check first element has key
	if cl.Elements[0].Key == nil {
		t.Error("expected key for first element")
	}
}

func TestParseCompositeLitNoKeys(t *testing.T) {
	p := parse(t, "Point{1, 2}")
	expr := p.ParseExpr()
	noErrors(t, p)
	cl, ok := expr.(*ast.CompositeLit)
	if !ok {
		t.Fatalf("expected *ast.CompositeLit, got %T", expr)
	}
	if len(cl.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(cl.Elements))
	}
	if cl.Elements[0].Key != nil {
		t.Error("expected no key for first element")
	}
}

func TestParseQualifiedCompositeLit(t *testing.T) {
	p := parse(t, "pkg.Point{x: 1}")
	expr := p.ParseExpr()
	noErrors(t, p)
	cl, ok := expr.(*ast.CompositeLit)
	if !ok {
		t.Fatalf("expected *ast.CompositeLit, got %T", expr)
	}
	nt, ok := cl.Type.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected *ast.NamedType, got %T", cl.Type)
	}
	if nt.Pkg == nil || nt.Pkg.Name != "pkg" {
		t.Errorf("expected pkg qualifier")
	}
	if nt.Name.Name != "Point" {
		t.Errorf("expected Point, got %s", nt.Name.Name)
	}
}

func TestParseMakePointer(t *testing.T) {
	p := parse(t, "make(Point)")
	expr := p.ParseExpr()
	noErrors(t, p)
	bc, ok := expr.(*ast.BuiltinCall)
	if !ok {
		t.Fatalf("expected *ast.BuiltinCall, got %T", expr)
	}
	if bc.Builtin != token.MAKE {
		t.Errorf("expected MAKE, got %s", bc.Builtin)
	}
}

func TestParseMakeSlice(t *testing.T) {
	p := parse(t, "make_slice(int, 10)")
	expr := p.ParseExpr()
	noErrors(t, p)
	bc, ok := expr.(*ast.BuiltinCall)
	if !ok {
		t.Fatalf("expected *ast.BuiltinCall, got %T", expr)
	}
	if bc.Builtin != token.MAKE_SLICE {
		t.Errorf("expected MAKE_SLICE, got %s", bc.Builtin)
	}
	_, ok = bc.Type.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected *ast.NamedType for element type, got %T", bc.Type)
	}
	if len(bc.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(bc.Args))
	}
}

func TestParseBox(t *testing.T) {
	p := parse(t, "box(42)")
	expr := p.ParseExpr()
	noErrors(t, p)
	bc, ok := expr.(*ast.BuiltinCall)
	if !ok {
		t.Fatalf("expected *ast.BuiltinCall, got %T", expr)
	}
	if bc.Builtin != token.BOX {
		t.Errorf("expected BOX, got %s", bc.Builtin)
	}
}

func TestParseCast(t *testing.T) {
	p := parse(t, "cast(int64, x)")
	expr := p.ParseExpr()
	noErrors(t, p)
	bc, ok := expr.(*ast.BuiltinCall)
	if !ok {
		t.Fatalf("expected *ast.BuiltinCall, got %T", expr)
	}
	if bc.Builtin != token.CAST {
		t.Errorf("expected CAST, got %s", bc.Builtin)
	}
	nt, ok := bc.Type.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected *ast.NamedType, got %T", bc.Type)
	}
	if nt.Name.Name != "int64" {
		t.Errorf("expected int64, got %s", nt.Name.Name)
	}
}

func TestParseBitCast(t *testing.T) {
	p := parse(t, "bit_cast(uint32, x)")
	expr := p.ParseExpr()
	noErrors(t, p)
	bc, ok := expr.(*ast.BuiltinCall)
	if !ok {
		t.Fatalf("expected *ast.BuiltinCall, got %T", expr)
	}
	if bc.Builtin != token.BIT_CAST {
		t.Errorf("expected BIT_CAST, got %s", bc.Builtin)
	}
}

func TestParseLen(t *testing.T) {
	p := parse(t, "len(s)")
	expr := p.ParseExpr()
	noErrors(t, p)
	bc, ok := expr.(*ast.BuiltinCall)
	if !ok {
		t.Fatalf("expected *ast.BuiltinCall, got %T", expr)
	}
	if bc.Builtin != token.LEN {
		t.Errorf("expected LEN, got %s", bc.Builtin)
	}
}

func TestParseComplexExpression(t *testing.T) {
	// a.b[0](c + d)
	p := parse(t, "a.b[0](c + d)")
	expr := p.ParseExpr()
	noErrors(t, p)
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected *ast.CallExpr, got %T", expr)
	}
	idx, ok := call.Fun.(*ast.IndexExpr)
	if !ok {
		t.Fatalf("expected *ast.IndexExpr, got %T", call.Fun)
	}
	sel, ok := idx.X.(*ast.SelectorExpr)
	if !ok {
		t.Fatalf("expected *ast.SelectorExpr, got %T", idx.X)
	}
	assertIdent(t, sel.X, "a")
}

func TestParseType_Pointer(t *testing.T) {
	p := parse(t, "*int")
	typ := p.ParseType()
	noErrors(t, p)
	pt, ok := typ.(*ast.PointerType)
	if !ok {
		t.Fatalf("expected *ast.PointerType, got %T", typ)
	}
	nt, ok := pt.Base.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected *ast.NamedType, got %T", pt.Base)
	}
	if nt.Name.Name != "int" {
		t.Errorf("expected int, got %s", nt.Name.Name)
	}
}

func TestParseType_ManagedPtr(t *testing.T) {
	p := parse(t, "@Point")
	typ := p.ParseType()
	noErrors(t, p)
	mp, ok := typ.(*ast.ManagedPtrType)
	if !ok {
		t.Fatalf("expected *ast.ManagedPtrType, got %T", typ)
	}
	nt, ok := mp.Base.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected *ast.NamedType, got %T", mp.Base)
	}
	if nt.Name.Name != "Point" {
		t.Errorf("expected Point, got %s", nt.Name.Name)
	}
}

func TestParseType_ManagedSlice(t *testing.T) {
	p := parse(t, "@[]int")
	typ := p.ParseType()
	noErrors(t, p)
	ms, ok := typ.(*ast.ManagedSliceType)
	if !ok {
		t.Fatalf("expected *ast.ManagedSliceType, got %T", typ)
	}
	nt, ok := ms.Elem.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected *ast.NamedType, got %T", ms.Elem)
	}
	if nt.Name.Name != "int" {
		t.Errorf("expected int, got %s", nt.Name.Name)
	}
}

func TestParseType_BareSliceRejected(t *testing.T) {
	// Bare "[]T" is no longer valid — raw slices must use "*[]T".
	p := parse(t, "[]byte")
	_ = p.ParseType()
	if len(p.Errors()) == 0 {
		t.Fatalf("expected parse error for bare []byte, got none")
	}
}

func TestParseType_Array(t *testing.T) {
	p := parse(t, "[10]int")
	typ := p.ParseType()
	noErrors(t, p)
	at, ok := typ.(*ast.ArrayType)
	if !ok {
		t.Fatalf("expected *ast.ArrayType, got %T", typ)
	}
	assertIntLit(t, at.Len, "10")
	nt, ok := at.Elem.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected *ast.NamedType, got %T", at.Elem)
	}
	if nt.Name.Name != "int" {
		t.Errorf("expected int, got %s", nt.Name.Name)
	}
}

func TestParseType_Struct(t *testing.T) {
	p := parse(t, "struct { x int; y int }")
	typ := p.ParseType()
	noErrors(t, p)
	st, ok := typ.(*ast.StructType)
	if !ok {
		t.Fatalf("expected *ast.StructType, got %T", typ)
	}
	if len(st.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(st.Fields))
	}
	if st.Fields[0].Name.Name != "x" {
		t.Errorf("expected x, got %s", st.Fields[0].Name.Name)
	}
	if st.Fields[1].Name.Name != "y" {
		t.Errorf("expected y, got %s", st.Fields[1].Name.Name)
	}
}

func TestParseType_QualifiedName(t *testing.T) {
	p := parse(t, "pkg.Type")
	typ := p.ParseType()
	noErrors(t, p)
	nt, ok := typ.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected *ast.NamedType, got %T", typ)
	}
	if nt.Pkg == nil || nt.Pkg.Name != "pkg" {
		t.Error("expected pkg qualifier")
	}
	if nt.Name.Name != "Type" {
		t.Errorf("expected Type, got %s", nt.Name.Name)
	}
}

func TestParseType_ManagedPtrToArray(t *testing.T) {
	// @[10]int — managed pointer to array
	p := parse(t, "@[10]int")
	typ := p.ParseType()
	noErrors(t, p)
	mp, ok := typ.(*ast.ManagedPtrType)
	if !ok {
		t.Fatalf("expected *ast.ManagedPtrType, got %T", typ)
	}
	at, ok := mp.Base.(*ast.ArrayType)
	if !ok {
		t.Fatalf("expected *ast.ArrayType, got %T", mp.Base)
	}
	assertIntLit(t, at.Len, "10")
	nt, ok := at.Elem.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected *ast.NamedType, got %T", at.Elem)
	}
	if nt.Name.Name != "int" {
		t.Errorf("expected int, got %s", nt.Name.Name)
	}
}

func TestParseType_RawSliceStarSugar(t *testing.T) {
	// *[]T — new raw-slice sugar (Stage 1 of []T → *[]T migration).
	// Same AST as []T: *ast.SliceType.
	p := parse(t, "*[]int")
	typ := p.ParseType()
	noErrors(t, p)
	sl, ok := typ.(*ast.SliceType)
	if !ok {
		t.Fatalf("expected *ast.SliceType, got %T", typ)
	}
	nt, ok := sl.Elem.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected *ast.NamedType, got %T", sl.Elem)
	}
	if nt.Name.Name != "int" {
		t.Errorf("expected int, got %s", nt.Name.Name)
	}
}

func TestParseType_RawSliceStarSugarNested(t *testing.T) {
	// *[]*[]char — raw slice of raw slice of char (nested sugar).
	p := parse(t, "*[]*[]char")
	typ := p.ParseType()
	noErrors(t, p)
	outer, ok := typ.(*ast.SliceType)
	if !ok {
		t.Fatalf("expected outer *ast.SliceType, got %T", typ)
	}
	inner, ok := outer.Elem.(*ast.SliceType)
	if !ok {
		t.Fatalf("expected inner *ast.SliceType, got %T", outer.Elem)
	}
	nt, ok := inner.Elem.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected *ast.NamedType, got %T", inner.Elem)
	}
	if nt.Name.Name != "char" {
		t.Errorf("expected char, got %s", nt.Name.Name)
	}
}

func TestParseType_BareStarBracketArrayRejected(t *testing.T) {
	// *[N]T — no longer valid; user must write *([N]T).
	p := parse(t, "*[10]int")
	_ = p.ParseType()
	if len(p.Errors()) == 0 {
		t.Fatalf("expected parse error for bare *[10]int, got none")
	}
}

func TestParseType_PointerToArrayViaParens(t *testing.T) {
	// *([N]T) — the required way to express pointer-to-array.
	p := parse(t, "*([10]int)")
	typ := p.ParseType()
	noErrors(t, p)
	pt, ok := typ.(*ast.PointerType)
	if !ok {
		t.Fatalf("expected *ast.PointerType, got %T", typ)
	}
	paren, ok := pt.Base.(*ast.ParenType)
	if !ok {
		t.Fatalf("expected *ast.ParenType, got %T", pt.Base)
	}
	at, ok := paren.Type.(*ast.ArrayType)
	if !ok {
		t.Fatalf("expected *ast.ArrayType inside parens, got %T", paren.Type)
	}
	assertIntLit(t, at.Len, "10")
}

func TestParseType_PointerToSliceViaParens(t *testing.T) {
	// *(*[]T) — the required way to express pointer-to-raw-slice.
	p := parse(t, "*(*[]int)")
	typ := p.ParseType()
	noErrors(t, p)
	pt, ok := typ.(*ast.PointerType)
	if !ok {
		t.Fatalf("expected *ast.PointerType, got %T", typ)
	}
	paren, ok := pt.Base.(*ast.ParenType)
	if !ok {
		t.Fatalf("expected *ast.ParenType, got %T", pt.Base)
	}
	if _, ok := paren.Type.(*ast.SliceType); !ok {
		t.Fatalf("expected *ast.SliceType inside parens, got %T", paren.Type)
	}
}

func TestParseType_ParenGrouping(t *testing.T) {
	// @(*[]T) — managed pointer to raw slice
	p := parse(t, "@(*[]int)")
	typ := p.ParseType()
	noErrors(t, p)
	mp, ok := typ.(*ast.ManagedPtrType)
	if !ok {
		t.Fatalf("expected *ast.ManagedPtrType, got %T", typ)
	}
	pt, ok := mp.Base.(*ast.ParenType)
	if !ok {
		t.Fatalf("expected *ast.ParenType, got %T", mp.Base)
	}
	_, ok = pt.Type.(*ast.SliceType)
	if !ok {
		t.Fatalf("expected *ast.SliceType inside parens, got %T", pt.Type)
	}
}

func TestParseArrayLiteral(t *testing.T) {
	p := parse(t, "[3]int{1, 2, 3}")
	expr := p.ParseExpr()
	noErrors(t, p)
	cl, ok := expr.(*ast.CompositeLit)
	if !ok {
		t.Fatalf("expected *ast.CompositeLit, got %T", expr)
	}
	at, ok := cl.Type.(*ast.ArrayType)
	if !ok {
		t.Fatalf("expected *ast.ArrayType, got %T", cl.Type)
	}
	assertIntLit(t, at.Len, "3")
	if len(cl.Elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(cl.Elements))
	}
}

func TestParseCompositeLitTrailingComma(t *testing.T) {
	p := parse(t, "Point{x: 1, y: 2,}")
	expr := p.ParseExpr()
	noErrors(t, p)
	cl, ok := expr.(*ast.CompositeLit)
	if !ok {
		t.Fatalf("expected *ast.CompositeLit, got %T", expr)
	}
	if len(cl.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(cl.Elements))
	}
}

// ============================================================
// Statement Tests
// ============================================================

func TestParseAssignment(t *testing.T) {
	p := parse(t, "x = 1")
	stmt := p.ParseStmt()
	noErrors(t, p)
	as, ok := stmt.(*ast.AssignStmt)
	if !ok {
		t.Fatalf("expected *ast.AssignStmt, got %T", stmt)
	}
	if as.Op != token.ASSIGN {
		t.Errorf("expected =, got %s", as.Op)
	}
	if len(as.LHS) != 1 || len(as.RHS) != 1 {
		t.Fatalf("expected 1 LHS and 1 RHS")
	}
}

func TestParseCompoundAssignment(t *testing.T) {
	p := parse(t, "x += 1")
	stmt := p.ParseStmt()
	noErrors(t, p)
	as, ok := stmt.(*ast.AssignStmt)
	if !ok {
		t.Fatalf("expected *ast.AssignStmt, got %T", stmt)
	}
	if as.Op != token.ADD_ASSIGN {
		t.Errorf("expected +=, got %s", as.Op)
	}
}

func TestParseMultiAssignment(t *testing.T) {
	p := parse(t, "x, y = 1, 2")
	stmt := p.ParseStmt()
	noErrors(t, p)
	as, ok := stmt.(*ast.AssignStmt)
	if !ok {
		t.Fatalf("expected *ast.AssignStmt, got %T", stmt)
	}
	if len(as.LHS) != 2 || len(as.RHS) != 2 {
		t.Fatalf("expected 2 LHS and 2 RHS, got %d and %d", len(as.LHS), len(as.RHS))
	}
}

func TestParseShortVarDecl(t *testing.T) {
	p := parse(t, "x := 42")
	stmt := p.ParseStmt()
	noErrors(t, p)
	svd, ok := stmt.(*ast.ShortVarDecl)
	if !ok {
		t.Fatalf("expected *ast.ShortVarDecl, got %T", stmt)
	}
	if len(svd.Names) != 1 || svd.Names[0].Name != "x" {
		t.Errorf("expected x, got %v", svd.Names)
	}
}

func TestParseIncDec(t *testing.T) {
	p := parse(t, "x++")
	stmt := p.ParseStmt()
	noErrors(t, p)
	id, ok := stmt.(*ast.IncDecStmt)
	if !ok {
		t.Fatalf("expected *ast.IncDecStmt, got %T", stmt)
	}
	if id.Op != token.INC {
		t.Errorf("expected ++, got %s", id.Op)
	}
}

func TestParseExprStmt(t *testing.T) {
	p := parse(t, "foo()")
	stmt := p.ParseStmt()
	noErrors(t, p)
	es, ok := stmt.(*ast.ExprStmt)
	if !ok {
		t.Fatalf("expected *ast.ExprStmt, got %T", stmt)
	}
	_, ok = es.X.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected *ast.CallExpr, got %T", es.X)
	}
}

func TestParseReturnNoValue(t *testing.T) {
	p := parse(t, "return")
	stmt := p.ParseStmt()
	noErrors(t, p)
	ret, ok := stmt.(*ast.ReturnStmt)
	if !ok {
		t.Fatalf("expected *ast.ReturnStmt, got %T", stmt)
	}
	if len(ret.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(ret.Results))
	}
}

func TestParseReturnValue(t *testing.T) {
	p := parse(t, "return x + 1")
	stmt := p.ParseStmt()
	noErrors(t, p)
	ret, ok := stmt.(*ast.ReturnStmt)
	if !ok {
		t.Fatalf("expected *ast.ReturnStmt, got %T", stmt)
	}
	if len(ret.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(ret.Results))
	}
}

func TestParseReturnMultiple(t *testing.T) {
	p := parse(t, "return x, y")
	stmt := p.ParseStmt()
	noErrors(t, p)
	ret, ok := stmt.(*ast.ReturnStmt)
	if !ok {
		t.Fatalf("expected *ast.ReturnStmt, got %T", stmt)
	}
	if len(ret.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(ret.Results))
	}
}

func TestParseBreak(t *testing.T) {
	p := parse(t, "break")
	stmt := p.ParseStmt()
	noErrors(t, p)
	_, ok := stmt.(*ast.BreakStmt)
	if !ok {
		t.Fatalf("expected *ast.BreakStmt, got %T", stmt)
	}
}

func TestParseContinue(t *testing.T) {
	p := parse(t, "continue")
	stmt := p.ParseStmt()
	noErrors(t, p)
	_, ok := stmt.(*ast.ContinueStmt)
	if !ok {
		t.Fatalf("expected *ast.ContinueStmt, got %T", stmt)
	}
}

func TestParseBlock(t *testing.T) {
	p := parse(t, "{ x := 1; y := 2 }")
	stmt := p.ParseStmt()
	noErrors(t, p)
	block, ok := stmt.(*ast.Block)
	if !ok {
		t.Fatalf("expected *ast.Block, got %T", stmt)
	}
	if len(block.Stmts) != 2 {
		t.Fatalf("expected 2 stmts, got %d", len(block.Stmts))
	}
}

func TestParseIfStmt(t *testing.T) {
	p := parse(t, "if x > 0 { return x }")
	stmt := p.ParseStmt()
	noErrors(t, p)
	is, ok := stmt.(*ast.IfStmt)
	if !ok {
		t.Fatalf("expected *ast.IfStmt, got %T", stmt)
	}
	if is.Else != nil {
		t.Error("expected no else")
	}
}

func TestParseIfElse(t *testing.T) {
	p := parse(t, "if x > 0 { return x } else { return 0 }")
	stmt := p.ParseStmt()
	noErrors(t, p)
	is, ok := stmt.(*ast.IfStmt)
	if !ok {
		t.Fatalf("expected *ast.IfStmt, got %T", stmt)
	}
	if is.Else == nil {
		t.Fatal("expected else")
	}
	_, ok = is.Else.(*ast.Block)
	if !ok {
		t.Fatalf("expected *ast.Block for else, got %T", is.Else)
	}
}

func TestParseIfElseIf(t *testing.T) {
	p := parse(t, "if x > 0 { return 1 } else if x < 0 { return 2 } else { return 0 }")
	stmt := p.ParseStmt()
	noErrors(t, p)
	is, ok := stmt.(*ast.IfStmt)
	if !ok {
		t.Fatalf("expected *ast.IfStmt, got %T", stmt)
	}
	elseIf, ok := is.Else.(*ast.IfStmt)
	if !ok {
		t.Fatalf("expected *ast.IfStmt for else-if, got %T", is.Else)
	}
	if elseIf.Else == nil {
		t.Fatal("expected final else")
	}
}

func TestParseForInfinite(t *testing.T) {
	p := parse(t, "for { break }")
	stmt := p.ParseStmt()
	noErrors(t, p)
	fs, ok := stmt.(*ast.ForStmt)
	if !ok {
		t.Fatalf("expected *ast.ForStmt, got %T", stmt)
	}
	if fs.Cond != nil {
		t.Error("expected nil Cond for infinite loop")
	}
	if fs.Init != nil {
		t.Error("expected nil Init for infinite loop")
	}
}

func TestParseForWhile(t *testing.T) {
	p := parse(t, "for x > 0 { x-- }")
	stmt := p.ParseStmt()
	noErrors(t, p)
	fs, ok := stmt.(*ast.ForStmt)
	if !ok {
		t.Fatalf("expected *ast.ForStmt, got %T", stmt)
	}
	if fs.Cond == nil {
		t.Fatal("expected non-nil Cond")
	}
	if fs.Init != nil {
		t.Error("expected nil Init")
	}
}

func TestParseForCStyle(t *testing.T) {
	p := parse(t, "for i := 0; i < 10; i++ { foo(i) }")
	stmt := p.ParseStmt()
	noErrors(t, p)
	fs, ok := stmt.(*ast.ForStmt)
	if !ok {
		t.Fatalf("expected *ast.ForStmt, got %T", stmt)
	}
	if fs.Init == nil {
		t.Fatal("expected non-nil Init")
	}
	if fs.Cond == nil {
		t.Fatal("expected non-nil Cond")
	}
	if fs.Post == nil {
		t.Fatal("expected non-nil Post")
	}
	_, ok = fs.Init.(*ast.ShortVarDecl)
	if !ok {
		t.Fatalf("expected *ast.ShortVarDecl for Init, got %T", fs.Init)
	}
}

func TestParseForIn(t *testing.T) {
	p := parse(t, "for x in items { foo(x) }")
	stmt := p.ParseStmt()
	noErrors(t, p)
	fs, ok := stmt.(*ast.ForStmt)
	if !ok {
		t.Fatalf("expected *ast.ForStmt, got %T", stmt)
	}
	if fs.Value == nil {
		t.Fatal("expected non-nil Value")
	}
	if fs.Value.Name != "x" {
		t.Errorf("expected x, got %s", fs.Value.Name)
	}
	if fs.Key != nil {
		t.Error("expected nil Key for single-var for-in")
	}
	if fs.Iter == nil {
		t.Fatal("expected non-nil Iter")
	}
}

func TestParseForInTwoVars(t *testing.T) {
	p := parse(t, "for i, v in items { foo(i, v) }")
	stmt := p.ParseStmt()
	noErrors(t, p)
	fs, ok := stmt.(*ast.ForStmt)
	if !ok {
		t.Fatalf("expected *ast.ForStmt, got %T", stmt)
	}
	if fs.Key == nil || fs.Key.Name != "i" {
		t.Errorf("expected Key=i, got %v", fs.Key)
	}
	if fs.Value == nil || fs.Value.Name != "v" {
		t.Errorf("expected Value=v, got %v", fs.Value)
	}
	if fs.Iter == nil {
		t.Fatal("expected non-nil Iter")
	}
}

func TestParseSwitchStmt(t *testing.T) {
	input := `switch x {
	case 1:
		foo()
	case 2, 3:
		bar()
	default:
		baz()
	}`
	p := parse(t, input)
	stmt := p.ParseStmt()
	noErrors(t, p)
	sw, ok := stmt.(*ast.SwitchStmt)
	if !ok {
		t.Fatalf("expected *ast.SwitchStmt, got %T", stmt)
	}
	if sw.Tag == nil {
		t.Fatal("expected non-nil Tag")
	}
	if len(sw.Cases) != 3 {
		t.Fatalf("expected 3 cases, got %d", len(sw.Cases))
	}
	// case 1
	if len(sw.Cases[0].Exprs) != 1 {
		t.Errorf("case 0: expected 1 expr, got %d", len(sw.Cases[0].Exprs))
	}
	// case 2, 3
	if len(sw.Cases[1].Exprs) != 2 {
		t.Errorf("case 1: expected 2 exprs, got %d", len(sw.Cases[1].Exprs))
	}
	// default
	if sw.Cases[2].Exprs != nil {
		t.Error("default case should have nil Exprs")
	}
}

func TestParseVarDecl(t *testing.T) {
	p := parse(t, "var x int")
	stmt := p.ParseStmt()
	noErrors(t, p)
	vd, ok := stmt.(*ast.VarDecl)
	if !ok {
		t.Fatalf("expected *ast.VarDecl, got %T", stmt)
	}
	if vd.Name.Name != "x" {
		t.Errorf("expected x, got %s", vd.Name.Name)
	}
	if vd.Type == nil {
		t.Fatal("expected non-nil Type")
	}
}

func TestParseVarDeclInferred(t *testing.T) {
	p := parse(t, "var x = 42")
	stmt := p.ParseStmt()
	noErrors(t, p)
	vd, ok := stmt.(*ast.VarDecl)
	if !ok {
		t.Fatalf("expected *ast.VarDecl, got %T", stmt)
	}
	if vd.Type != nil {
		t.Error("expected nil Type for inferred var")
	}
	if vd.Value == nil {
		t.Fatal("expected non-nil Value")
	}
}

func TestParseVarDeclWithInit(t *testing.T) {
	p := parse(t, "var x int = 42")
	stmt := p.ParseStmt()
	noErrors(t, p)
	vd, ok := stmt.(*ast.VarDecl)
	if !ok {
		t.Fatalf("expected *ast.VarDecl, got %T", stmt)
	}
	if vd.Type == nil {
		t.Fatal("expected non-nil Type")
	}
	if vd.Value == nil {
		t.Fatal("expected non-nil Value")
	}
}

// ============================================================
// File-Level Tests
// ============================================================

func TestParseFile(t *testing.T) {
	input := `package "main"

import "fmt"

func add(a int, b int) int {
	return a + b
}

var x int = 10

type Point struct {
	x int
	y int
}

const maxSize = 100
`
	p := parse(t, input)
	f := p.ParseFile()
	noErrors(t, p)

	if f.PkgName == nil || f.PkgName.Value != `"main"` {
		t.Errorf("expected package \"main\", got %v", f.PkgName)
	}
	if len(f.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(f.Imports))
	}
	if len(f.Decls) != 4 {
		t.Fatalf("expected 4 decls, got %d", len(f.Decls))
	}
	// func
	_, ok := f.Decls[0].(*ast.FuncDecl)
	if !ok {
		t.Errorf("expected FuncDecl, got %T", f.Decls[0])
	}
	// var
	_, ok = f.Decls[1].(*ast.VarDecl)
	if !ok {
		t.Errorf("expected VarDecl, got %T", f.Decls[1])
	}
	// type
	_, ok = f.Decls[2].(*ast.TypeDecl)
	if !ok {
		t.Errorf("expected TypeDecl, got %T", f.Decls[2])
	}
	// const
	_, ok = f.Decls[3].(*ast.ConstDecl)
	if !ok {
		t.Errorf("expected ConstDecl, got %T", f.Decls[3])
	}
}

func TestParseGroupedImports(t *testing.T) {
	input := `package "main"

import (
	"fmt"
	io "io/ioutil"
)
`
	p := parse(t, input)
	f := p.ParseFile()
	noErrors(t, p)

	if len(f.Imports) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(f.Imports))
	}
	if f.Imports[0].Alias != "" {
		t.Errorf("expected no alias for first import")
	}
	if f.Imports[1].Alias != "io" {
		t.Errorf("expected alias 'io', got %q", f.Imports[1].Alias)
	}
}

func TestParseFuncMultipleReturns(t *testing.T) {
	input := `package "main"

func divide(a int, b int) (int, bool) {
	if b == 0 {
		return 0, false
	}
	return a / b, true
}
`
	p := parse(t, input)
	f := p.ParseFile()
	noErrors(t, p)

	if len(f.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(f.Decls))
	}
	fd, ok := f.Decls[0].(*ast.FuncDecl)
	if !ok {
		t.Fatalf("expected FuncDecl, got %T", f.Decls[0])
	}
	if len(fd.Results) != 2 {
		t.Fatalf("expected 2 return types, got %d", len(fd.Results))
	}
}

func TestParseGroupedConst(t *testing.T) {
	input := `package "main"

const (
	a = 1
	b = 2
	c = 3
)
`
	p := parse(t, input)
	f := p.ParseFile()
	noErrors(t, p)

	if len(f.Decls) != 1 {
		t.Fatalf("expected 1 decl (GroupDecl), got %d", len(f.Decls))
	}
	gd, ok := f.Decls[0].(*ast.GroupDecl)
	if !ok {
		t.Fatalf("expected *ast.GroupDecl, got %T", f.Decls[0])
	}
	if len(gd.Decls) != 3 {
		t.Fatalf("expected 3 const specs, got %d", len(gd.Decls))
	}
}

func TestParseTypeAlias(t *testing.T) {
	input := `package "main"

type MyInt = int
`
	p := parse(t, input)
	f := p.ParseFile()
	noErrors(t, p)

	td, ok := f.Decls[0].(*ast.TypeDecl)
	if !ok {
		t.Fatalf("expected TypeDecl, got %T", f.Decls[0])
	}
	if !td.Assign {
		t.Error("expected Assign=true for type alias")
	}
}

func TestParseDistinctType(t *testing.T) {
	input := `package "main"

type Duration int64
`
	p := parse(t, input)
	f := p.ParseFile()
	noErrors(t, p)

	td, ok := f.Decls[0].(*ast.TypeDecl)
	if !ok {
		t.Fatalf("expected TypeDecl, got %T", f.Decls[0])
	}
	if td.Assign {
		t.Error("expected Assign=false for distinct type")
	}
}

// ============================================================
// Helpers
// ============================================================

func assertIntLit(t *testing.T, e ast.Expr, val string) {
	t.Helper()
	lit, ok := e.(*ast.IntLit)
	if !ok {
		t.Fatalf("expected *ast.IntLit, got %T", e)
	}
	if lit.Value != val {
		t.Errorf("expected %s, got %s", val, lit.Value)
	}
}

func assertIdent(t *testing.T, e ast.Expr, name string) {
	t.Helper()
	ident, ok := e.(*ast.Ident)
	if !ok {
		t.Fatalf("expected *ast.Ident, got %T", e)
	}
	if ident.Name != name {
		t.Errorf("expected %s, got %s", name, ident.Name)
	}
}
