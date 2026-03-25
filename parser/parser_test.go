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
	p := parse(t, "make([]int, 10)")
	expr := p.ParseExpr()
	noErrors(t, p)
	bc, ok := expr.(*ast.BuiltinCall)
	if !ok {
		t.Fatalf("expected *ast.BuiltinCall, got %T", expr)
	}
	if bc.Builtin != token.MAKE {
		t.Errorf("expected MAKE, got %s", bc.Builtin)
	}
	_, ok = bc.Type.(*ast.SliceType)
	if !ok {
		t.Fatalf("expected *ast.SliceType, got %T", bc.Type)
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

func TestParseType_Slice(t *testing.T) {
	p := parse(t, "[]byte")
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
	if nt.Name.Name != "byte" {
		t.Errorf("expected byte, got %s", nt.Name.Name)
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

func TestParseType_ParenGrouping(t *testing.T) {
	// @([]T) — managed pointer to raw slice
	p := parse(t, "@([]int)")
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
