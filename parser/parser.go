// Package parser implements the Binate bootstrap recursive descent parser.
package parser

import (
	"fmt"

	"github.com/binate/bootstrap/ast"
	"github.com/binate/bootstrap/lexer"
	"github.com/binate/bootstrap/token"
)

// Parser parses Binate source code into an AST.
type Parser struct {
	lex  *lexer.Lexer
	tok  token.Token // current token
	prev token.Token // previous token (for error reporting)
	// peek holds a one-token lookahead. peekTok() pulls a token from
	// the lexer and stashes it here; the next next() call drains it
	// before pulling fresh. Used by parsePrimaryExpr to merge adjacent
	// string literals across an ASI-inserted semicolon without
	// committing to consume the SEMI before knowing what follows.
	peek    token.Token
	hasPeek bool
	errs    []Error

	// noCompositeLit suppresses composite literal parsing (D4).
	// Set when parsing conditions in if/for/switch where { would be
	// ambiguous with the block body.
	noCompositeLit bool

	// interfaceFile is true when parsing a .bni interface file.
	// Function declarations don't require bodies.
	interfaceFile bool
}

// Error represents a parse error with position.
type Error struct {
	Pos token.Pos
	Msg string
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
}

// New creates a new parser for the given source.
func New(src []byte, file string) *Parser {
	p := &Parser{
		lex: lexer.New(src, file),
	}
	p.next() // prime the first token
	return p
}

// NewInterface creates a new parser for a .bni interface file.
// In interface mode, function declarations don't require bodies.
func NewInterface(src []byte, file string) *Parser {
	p := &Parser{
		lex:           lexer.New(src, file),
		interfaceFile: true,
	}
	p.next()
	return p
}

// Errors returns the list of parse errors.
func (p *Parser) Errors() []Error {
	return p.errs
}

// mergeAdjacentStringLits returns the C-style concatenation of two raw
// string literals (each in `"..."` form). Drops the trailing `"` of `a`
// and the leading `"` of `b` so the result is one quoted literal with
// the contents glued together. Escape sequences inside each input pass
// through verbatim.
func mergeAdjacentStringLits(a, b string) string {
	if len(a) < 1 || len(b) < 1 {
		return a + b
	}
	return a[:len(a)-1] + b[1:]
}

// next advances to the next token. If peekTok() has stashed a
// lookahead token, drain that first.
func (p *Parser) next() {
	p.prev = p.tok
	if p.hasPeek {
		p.tok = p.peek
		p.hasPeek = false
	} else {
		p.tok = p.lex.Next()
	}
}

// peekTok returns the token after the current one without consuming
// the current. Subsequent calls return the same stashed token until
// a next() drains it.
func (p *Parser) peekTok() token.Token {
	if !p.hasPeek {
		p.peek = p.lex.Next()
		p.hasPeek = true
	}
	return p.peek
}

// expect consumes the current token if it matches typ, otherwise records an error.
func (p *Parser) expect(typ token.Type) token.Token {
	tok := p.tok
	if p.tok.Type != typ {
		p.errorf("expected %s, got %s", typ, p.tok.Type)
	} else {
		p.next()
	}
	return tok
}

// got returns true and advances if the current token matches typ.
func (p *Parser) got(typ token.Type) bool {
	if p.tok.Type == typ {
		p.next()
		return true
	}
	return false
}

func (p *Parser) errorf(format string, args ...any) {
	p.errs = append(p.errs, Error{
		Pos: p.tok.Pos,
		Msg: fmt.Sprintf(format, args...),
	})
}

// ============================================================
// Type Parsing
// ============================================================

// ParseType parses a type expression.
func (p *Parser) ParseType() ast.TypeExpr {
	return p.parseType()
}

func (p *Parser) parseType() ast.TypeExpr {
	switch p.tok.Type {
	case token.STAR: // *T or *[]T
		pos := p.tok.Pos
		p.next()
		if p.tok.Type == token.LBRACKET {
			lbrack := p.tok.Pos
			p.next() // consume [
			if p.tok.Type == token.RBRACKET {
				// *[]T — raw slice sugar (Stage 1 of raw-slice
				// syntax migration; same AST as []T).
				p.next() // consume ]
				elem := p.parseType()
				return &ast.SliceType{Lbrack: lbrack, Elem: elem}
			}
			// Bare "*[<expr>" is no longer valid — force parens for
			// pointer-to-array (*([N]T)) and pointer-to-slice (*([]T)).
			p.errorf(`bare "*[" is raw-slice sugar (*[]T); use "*([N]T)" for pointer to array or "*([]T)" for pointer to slice`)
			// Recover: parse as if it were *[N]T (pointer to array).
			length := p.parseExpr()
			p.expect(token.RBRACKET)
			elem := p.parseType()
			arr := &ast.ArrayType{Lbrack: lbrack, Len: length, Elem: elem}
			return &ast.PointerType{Star: pos, Base: arr}
		}
		base := p.parseType()
		return &ast.PointerType{Star: pos, Base: base}

	case token.AT: // @T or @[]T
		pos := p.tok.Pos
		p.next()
		// Check for @[]T (managed slice sugar): @ [ ] Type
		if p.tok.Type == token.LBRACKET {
			lbrack := p.tok.Pos
			p.next() // consume [
			if p.tok.Type == token.RBRACKET {
				// @[]T — managed slice
				p.next() // consume ]
				elem := p.parseType()
				return &ast.ManagedSliceType{At: pos, Elem: elem}
			}
			// @[N]T — managed pointer to array type
			length := p.parseExpr()
			p.expect(token.RBRACKET)
			elem := p.parseType()
			arrType := &ast.ArrayType{Lbrack: lbrack, Len: length, Elem: elem}
			return &ast.ManagedPtrType{At: pos, Base: arrType}
		}
		base := p.parseType()
		return &ast.ManagedPtrType{At: pos, Base: base}

	case token.LBRACKET: // [N]T (array); bare []T is no longer valid
		pos := p.tok.Pos
		p.next() // consume [
		if p.tok.Type == token.RBRACKET {
			// Bare []T is no longer a valid raw-slice syntax — use *[]T.
			p.errorf(`bare "[" "]" is no longer valid raw-slice syntax; use "*[]T" instead`)
			p.next() // consume ]
			elem := p.parseType()
			return &ast.SliceType{Lbrack: pos, Elem: elem}
		}
		// [N]T — array type
		length := p.parseExpr()
		p.expect(token.RBRACKET)
		elem := p.parseType()
		return &ast.ArrayType{Lbrack: pos, Len: length, Elem: elem}

	case token.LPAREN: // (T) — grouping
		pos := p.tok.Pos
		p.next()
		typ := p.parseType()
		p.expect(token.RPAREN)
		return &ast.ParenType{Lparen: pos, Type: typ}

	case token.CONST: // const T — bootstrap is permissive, strip it
		p.next()
		return p.parseType()

	case token.STRUCT: // struct { ... }
		return p.parseStructType()

	case token.FUNC:
		// func types are deferred from bootstrap, but handle for error messages
		p.errorf("function types are not supported in bootstrap")
		p.next()
		return &ast.NamedType{Name: &ast.Ident{NamePos: p.tok.Pos, Name: "<error>"}}

	case token.IDENT:
		return p.parseNamedType()

	default:
		p.errorf("expected type, got %s", p.tok.Type)
		pos := p.tok.Pos
		return &ast.NamedType{Name: &ast.Ident{NamePos: pos, Name: "<error>"}}
	}
}

func (p *Parser) parseNamedType() *ast.NamedType {
	ident := p.parseIdent()
	// Check for qualified name: pkg.Type
	if p.tok.Type == token.DOT {
		p.next()
		name := p.parseIdent()
		return &ast.NamedType{Pkg: ident, Name: name}
	}
	return &ast.NamedType{Name: ident}
}

func (p *Parser) parseStructType() *ast.StructType {
	pos := p.tok.Pos
	p.expect(token.STRUCT)
	p.expect(token.LBRACE)

	var fields []*ast.StructField
	for p.tok.Type != token.RBRACE && p.tok.Type != token.EOF {
		fields = append(fields, p.parseStructField())
		if !p.got(token.SEMICOLON) {
			break
		}
	}
	p.expect(token.RBRACE)
	return &ast.StructType{StructPos: pos, Fields: fields}
}

func (p *Parser) parseStructField() *ast.StructField {
	// D10: named field vs anonymous embed
	// If identifier is followed by another type-starting token, it's a named field.
	// If followed by ; or }, it's an anonymous embed.
	// If followed by ., it's a qualified anonymous embed.
	if p.tok.Type == token.IDENT {
		name := p.parseIdent()
		switch p.tok.Type {
		case token.SEMICOLON, token.RBRACE:
			// Anonymous embed: just a type name
			return &ast.StructField{Type: &ast.NamedType{Name: name}}
		case token.DOT:
			// Could be qualified anonymous embed: pkg.Type
			p.next()
			typeName := p.parseIdent()
			return &ast.StructField{Type: &ast.NamedType{Pkg: name, Name: typeName}}
		default:
			// Named field: name followed by type
			typ := p.parseType()
			return &ast.StructField{Name: name, Type: typ}
		}
	}
	// Non-identifier start: anonymous embed with complex type (pointer, etc.)
	typ := p.parseType()
	return &ast.StructField{Type: typ}
}

// ============================================================
// Expression Parsing
// ============================================================

// ParseExpr parses an expression.
func (p *Parser) ParseExpr() ast.Expr {
	return p.parseExpr()
}

func (p *Parser) parseExpr() ast.Expr {
	return p.parseOrExpr()
}

// parseExprNoCompositeLit parses an expression with composite literals suppressed.
// Used in if/for/switch conditions where { is ambiguous with the block body (D4).
func (p *Parser) parseExprNoCompositeLit() ast.Expr {
	old := p.noCompositeLit
	p.noCompositeLit = true
	x := p.parseExpr()
	p.noCompositeLit = old
	return x
}

func (p *Parser) parseOrExpr() ast.Expr {
	x := p.parseAndExpr()
	for p.tok.Type == token.LOR {
		p.next()
		y := p.parseAndExpr()
		x = &ast.BinaryExpr{X: x, Op: token.LOR, Y: y}
	}
	return x
}

func (p *Parser) parseAndExpr() ast.Expr {
	x := p.parseCompareExpr()
	for p.tok.Type == token.LAND {
		p.next()
		y := p.parseCompareExpr()
		x = &ast.BinaryExpr{X: x, Op: token.LAND, Y: y}
	}
	return x
}

func (p *Parser) parseCompareExpr() ast.Expr {
	x := p.parseBitOrExpr()
	// Comparison operators — no chaining
	switch p.tok.Type {
	case token.EQ, token.NEQ, token.LT, token.GT, token.LEQ, token.GEQ:
		op := p.tok.Type
		p.next()
		y := p.parseBitOrExpr()
		return &ast.BinaryExpr{X: x, Op: op, Y: y}
	}
	return x
}

func (p *Parser) parseBitOrExpr() ast.Expr {
	x := p.parseBitXorExpr()
	for p.tok.Type == token.PIPE {
		p.next()
		y := p.parseBitXorExpr()
		x = &ast.BinaryExpr{X: x, Op: token.PIPE, Y: y}
	}
	return x
}

func (p *Parser) parseBitXorExpr() ast.Expr {
	x := p.parseBitAndExpr()
	for p.tok.Type == token.CARET {
		p.next()
		y := p.parseBitAndExpr()
		x = &ast.BinaryExpr{X: x, Op: token.CARET, Y: y}
	}
	return x
}

func (p *Parser) parseBitAndExpr() ast.Expr {
	x := p.parseShiftExpr()
	for p.tok.Type == token.AMP {
		p.next()
		y := p.parseShiftExpr()
		x = &ast.BinaryExpr{X: x, Op: token.AMP, Y: y}
	}
	return x
}

func (p *Parser) parseShiftExpr() ast.Expr {
	x := p.parseAddExpr()
	for p.tok.Type == token.SHL || p.tok.Type == token.SHR {
		op := p.tok.Type
		p.next()
		y := p.parseAddExpr()
		x = &ast.BinaryExpr{X: x, Op: op, Y: y}
	}
	return x
}

func (p *Parser) parseAddExpr() ast.Expr {
	x := p.parseMulExpr()
	for p.tok.Type == token.PLUS || p.tok.Type == token.MINUS {
		op := p.tok.Type
		p.next()
		y := p.parseMulExpr()
		x = &ast.BinaryExpr{X: x, Op: op, Y: y}
	}
	return x
}

func (p *Parser) parseMulExpr() ast.Expr {
	x := p.parseUnaryExpr()
	for p.tok.Type == token.STAR || p.tok.Type == token.SLASH || p.tok.Type == token.PERCENT {
		op := p.tok.Type
		p.next()
		y := p.parseUnaryExpr()
		x = &ast.BinaryExpr{X: x, Op: op, Y: y}
	}
	return x
}

func (p *Parser) parseUnaryExpr() ast.Expr {
	switch p.tok.Type {
	case token.NOT, token.TILDE, token.MINUS, token.STAR, token.AMP:
		pos := p.tok.Pos
		op := p.tok.Type
		p.next()
		x := p.parseUnaryExpr()
		return &ast.UnaryExpr{OpPos: pos, Op: op, X: x}
	}
	return p.parsePostfixExpr()
}

func (p *Parser) parsePostfixExpr() ast.Expr {
	x := p.parsePrimaryExpr()
	for {
		switch p.tok.Type {
		case token.DOT:
			p.next()
			sel := p.parseIdent()
			x = &ast.SelectorExpr{X: x, Sel: sel}

		case token.LBRACKET:
			x = p.parseIndexOrSlice(x)

		case token.LPAREN:
			x = p.parseCallExpr(x)

		default:
			return x
		}
	}
}

func (p *Parser) parseIndexOrSlice(x ast.Expr) ast.Expr {
	p.next() // consume [

	// Check for slice with no low: [:hi]
	if p.tok.Type == token.COLON {
		p.next()
		var hi ast.Expr
		if p.tok.Type != token.RBRACKET {
			hi = p.parseExpr()
		}
		p.expect(token.RBRACKET)
		return &ast.SliceExpr{X: x, Hi: hi}
	}

	idx := p.parseExpr()

	if p.tok.Type == token.COLON {
		// Slice: x[lo:hi]
		p.next()
		var hi ast.Expr
		if p.tok.Type != token.RBRACKET {
			hi = p.parseExpr()
		}
		p.expect(token.RBRACKET)
		return &ast.SliceExpr{X: x, Lo: idx, Hi: hi}
	}

	// Index: x[i]
	p.expect(token.RBRACKET)
	return &ast.IndexExpr{X: x, Index: idx}
}

func (p *Parser) parseCallExpr(fn ast.Expr) *ast.CallExpr {
	p.next() // consume (
	var args []ast.Expr
	if p.tok.Type != token.RPAREN {
		args = p.parseExprList()
	}
	p.expect(token.RPAREN)
	return &ast.CallExpr{Fun: fn, Args: args}
}

func (p *Parser) parsePrimaryExpr() ast.Expr {
	switch p.tok.Type {
	// Builtin keywords
	case token.MAKE:
		return p.parseMakeCall()
	case token.MAKE_SLICE:
		return p.parseMakeSliceCall()
	case token.BOX:
		return p.parseBoxCall()
	case token.CAST:
		return p.parseCastCall()
	case token.BIT_CAST:
		return p.parseBitCastCall()
	case token.LEN:
		return p.parseLenCall()

	// Identifier — check for composite literal (ident { ... } or pkg.Type { ... })
	case token.IDENT:
		return p.parseIdentOrCompositeLit()

	// Literals
	case token.INT:
		tok := p.tok
		p.next()
		return &ast.IntLit{ValuePos: tok.Pos, Value: tok.Literal}
	case token.STRING:
		tok := p.tok
		p.next()
		lit := tok.Literal
		// C-style adjacent string-literal concatenation in expression
		// context. Two shapes accepted:
		//   "foo" "bar"           — same line, lexer emits STRING STRING
		//   "foo"\n"bar"          — different lines, ASI inserts a SEMI
		// For the latter, peek past the SEMI to check if a STRING
		// follows; if so, consume the SEMI (it's spurious in expression
		// context) and merge. Otherwise leave the SEMI alone — it ends
		// the enclosing statement.
		// Done here (not in the lexer) because grouped imports also
		// have STRING SEMI("\n") STRING and ASI is the path separator
		// — a lexer-level merge would silently glue separate paths.
		for {
			if p.tok.Type == token.STRING {
				lit = mergeAdjacentStringLits(lit, p.tok.Literal)
				p.next()
				continue
			}
			if p.tok.Type == token.SEMICOLON &&
				len(p.tok.Literal) > 0 && p.tok.Literal[0] == '\n' {
				pk := p.peekTok()
				if pk.Type != token.STRING {
					break
				}
				p.next() // consume the ASI SEMI
				lit = mergeAdjacentStringLits(lit, p.tok.Literal)
				p.next()
				continue
			}
			break
		}
		return &ast.StringLit{ValuePos: tok.Pos, Value: lit}
	case token.CHAR:
		tok := p.tok
		p.next()
		return &ast.CharLit{ValuePos: tok.Pos, Value: tok.Literal}
	case token.TRUE:
		tok := p.tok
		p.next()
		return &ast.BoolLit{ValuePos: tok.Pos, Value: true}
	case token.FALSE:
		tok := p.tok
		p.next()
		return &ast.BoolLit{ValuePos: tok.Pos, Value: false}
	case token.NIL:
		tok := p.tok
		p.next()
		return &ast.NilLit{NilPos: tok.Pos}

	// Array literal: [N]T{ ... }
	case token.LBRACKET:
		return p.parseArrayLit()

	// Parenthesized expression
	case token.LPAREN:
		p.next()
		x := p.parseExpr()
		p.expect(token.RPAREN)
		return x

	default:
		p.errorf("expected expression, got %s", p.tok.Type)
		pos := p.tok.Pos
		p.next()
		return &ast.Ident{NamePos: pos, Name: "<error>"}
	}
}

// parseIdentOrCompositeLit parses an identifier, possibly followed by a
// composite literal. Handles: ident, pkg.Type{...}, ident{...}.
func (p *Parser) parseIdentOrCompositeLit() ast.Expr {
	ident := p.parseIdent()

	// Check for qualified name: pkg.Type
	if p.tok.Type == token.DOT {
		p.next()
		name := p.parseIdent()
		if p.tok.Type == token.LBRACE && !p.noCompositeLit {
			// pkg.Type{ ... } — composite literal
			return p.parseCompositeLitBody(&ast.NamedType{Pkg: ident, Name: name})
		}
		// pkg.member — this becomes a SelectorExpr in postfix handling,
		// but here at primary level it's just a qualified identifier.
		// Return as SelectorExpr.
		return &ast.SelectorExpr{X: ident, Sel: name}
	}

	// Check for composite literal: Type{ ... }
	if p.tok.Type == token.LBRACE && !p.noCompositeLit {
		return p.parseCompositeLitBody(&ast.NamedType{Name: ident})
	}

	return ident
}

func (p *Parser) parseCompositeLitBody(typ ast.TypeExpr) *ast.CompositeLit {
	lbrace := p.tok.Pos
	p.expect(token.LBRACE)

	var elems []*ast.Element
	if p.tok.Type != token.RBRACE {
		elems = p.parseElementList()
	}

	rbrace := p.tok.Pos
	p.expect(token.RBRACE)
	return &ast.CompositeLit{
		Type:     typ,
		Elements: elems,
		Lbrace:   lbrace,
		Rbrace:   rbrace,
	}
}

func (p *Parser) parseElementList() []*ast.Element {
	var elems []*ast.Element
	elems = append(elems, p.parseElement())
	for p.got(token.COMMA) {
		if p.tok.Type == token.RBRACE {
			break // trailing comma
		}
		elems = append(elems, p.parseElement())
	}
	return elems
}

func (p *Parser) parseElement() *ast.Element {
	x := p.parseExpr()
	if p.tok.Type == token.COLON {
		p.next()
		val := p.parseExpr()
		return &ast.Element{Key: x, Value: val}
	}
	return &ast.Element{Value: x}
}

// parseArrayLit parses [N]T{ ... } or [...]T{ ... }.
func (p *Parser) parseArrayLit() ast.Expr {
	lbrack := p.tok.Pos
	p.expect(token.LBRACKET)

	var length ast.Expr
	if p.tok.Type == token.ELLIPSIS {
		// [...]T{ ... } — length inferred
		length = &ast.Ident{NamePos: p.tok.Pos, Name: "..."}
		p.next()
	} else {
		length = p.parseExpr()
	}

	p.expect(token.RBRACKET)
	elem := p.parseType()

	// Must be followed by {
	lbrace := p.tok.Pos
	p.expect(token.LBRACE)
	var elems []*ast.Element
	if p.tok.Type != token.RBRACE {
		elems = p.parseElementList()
	}
	rbrace := p.tok.Pos
	p.expect(token.RBRACE)

	arrType := &ast.ArrayType{Lbrack: lbrack, Len: length, Elem: elem}
	return &ast.CompositeLit{
		Type:     arrType,
		Elements: elems,
		Lbrace:   lbrace,
		Rbrace:   rbrace,
	}
}

// ============================================================
// Builtin Calls
// ============================================================

func (p *Parser) parseMakeCall() *ast.BuiltinCall {
	pos := p.tok.Pos
	p.next() // consume make
	p.expect(token.LPAREN)

	// make(T) — takes a type, no size argument
	typ := p.parseType()
	p.expect(token.RPAREN)
	return &ast.BuiltinCall{BuiltinPos: pos, Builtin: token.MAKE, Type: typ}
}

func (p *Parser) parseMakeSliceCall() *ast.BuiltinCall {
	pos := p.tok.Pos
	p.next() // consume make_slice
	p.expect(token.LPAREN)

	// make_slice(T, n) — element type + size
	typ := p.parseType()
	p.expect(token.COMMA)
	size := p.parseExpr()
	p.expect(token.RPAREN)
	return &ast.BuiltinCall{BuiltinPos: pos, Builtin: token.MAKE_SLICE, Type: typ, Args: []ast.Expr{size}}
}

func (p *Parser) parseBoxCall() *ast.BuiltinCall {
	pos := p.tok.Pos
	p.next() // consume box
	p.expect(token.LPAREN)
	arg := p.parseExpr()
	p.expect(token.RPAREN)
	return &ast.BuiltinCall{BuiltinPos: pos, Builtin: token.BOX, Args: []ast.Expr{arg}}
}

func (p *Parser) parseCastCall() *ast.BuiltinCall {
	pos := p.tok.Pos
	p.next() // consume cast
	p.expect(token.LPAREN)
	typ := p.parseType()
	p.expect(token.COMMA)
	arg := p.parseExpr()
	p.expect(token.RPAREN)
	return &ast.BuiltinCall{BuiltinPos: pos, Builtin: token.CAST, Type: typ, Args: []ast.Expr{arg}}
}

func (p *Parser) parseBitCastCall() *ast.BuiltinCall {
	pos := p.tok.Pos
	p.next() // consume bit_cast
	p.expect(token.LPAREN)
	typ := p.parseType()
	p.expect(token.COMMA)
	arg := p.parseExpr()
	p.expect(token.RPAREN)
	return &ast.BuiltinCall{BuiltinPos: pos, Builtin: token.BIT_CAST, Type: typ, Args: []ast.Expr{arg}}
}

func (p *Parser) parseLenCall() *ast.BuiltinCall {
	pos := p.tok.Pos
	p.next() // consume len
	p.expect(token.LPAREN)
	arg := p.parseExpr()
	p.expect(token.RPAREN)
	return &ast.BuiltinCall{BuiltinPos: pos, Builtin: token.LEN, Args: []ast.Expr{arg}}
}

// ============================================================
// Source File Parsing
// ============================================================

// ParseFile parses a complete source file.
func (p *Parser) ParseFile() *ast.File {
	f := &ast.File{}

	// Package clause
	f.Package = p.tok.Pos
	p.expect(token.PACKAGE)
	if p.tok.Type == token.STRING {
		f.PkgName = &ast.StringLit{ValuePos: p.tok.Pos, Value: p.tok.Literal}
		p.next()
	} else {
		p.errorf("expected package name string, got %s", p.tok.Type)
	}
	p.expect(token.SEMICOLON)

	// Import declarations
	for p.tok.Type == token.IMPORT {
		f.Imports = append(f.Imports, p.parseImportDecl()...)
		p.expect(token.SEMICOLON)
	}

	// Top-level declarations
	for p.tok.Type != token.EOF {
		d := p.parseTopLevelDecl()
		if d != nil {
			f.Decls = append(f.Decls, d)
		}
		if p.tok.Type != token.EOF {
			p.expect(token.SEMICOLON)
		}
	}

	return f
}

func (p *Parser) parseImportDecl() []*ast.ImportSpec {
	pos := p.tok.Pos
	p.expect(token.IMPORT)

	if p.tok.Type == token.LPAREN {
		// Grouped import
		p.next()
		var specs []*ast.ImportSpec
		for p.tok.Type != token.RPAREN && p.tok.Type != token.EOF {
			specs = append(specs, p.parseImportSpec(pos))
			if !p.got(token.SEMICOLON) {
				break
			}
		}
		p.expect(token.RPAREN)
		return specs
	}

	return []*ast.ImportSpec{p.parseImportSpec(pos)}
}

func (p *Parser) parseImportSpec(importPos token.Pos) *ast.ImportSpec {
	spec := &ast.ImportSpec{ImportPos: importPos}
	if p.tok.Type == token.IDENT {
		spec.Alias = p.tok.Literal
		p.next()
	}
	if p.tok.Type == token.STRING {
		spec.Path = &ast.StringLit{ValuePos: p.tok.Pos, Value: p.tok.Literal}
		p.next()
	} else {
		p.errorf("expected import path string, got %s", p.tok.Type)
	}
	return spec
}

func (p *Parser) parseTopLevelDecl() ast.Decl {
	switch p.tok.Type {
	case token.FUNC:
		return p.parseFuncDecl()
	case token.TYPE:
		return p.parseTypeDecl()
	case token.VAR:
		return p.parseVarDecl()
	case token.CONST:
		return p.parseConstDecl()
	default:
		p.errorf("expected declaration, got %s", p.tok.Type)
		p.next()
		return nil
	}
}

// ============================================================
// Declaration Parsing
// ============================================================

func (p *Parser) parseFuncDecl() *ast.FuncDecl {
	pos := p.tok.Pos
	p.expect(token.FUNC)

	// Optional receiver: `func (r RT) Name(...)`. After `func`, an
	// LPAREN means a method receiver; an IDENT means a free function.
	var recv *ast.ParamDecl
	if p.tok.Type == token.LPAREN {
		p.next()
		recv = p.parseParamDecl()
		p.expect(token.RPAREN)
	}

	name := p.parseIdent()

	// Parameters
	p.expect(token.LPAREN)
	var params []*ast.ParamDecl
	if p.tok.Type != token.RPAREN {
		params = p.parseParamList()
	}
	p.expect(token.RPAREN)

	// Result types
	var results []ast.TypeExpr
	if p.tok.Type == token.LPAREN {
		// Multiple returns: (T1, T2)
		p.next()
		results = p.parseTypeList()
		p.expect(token.RPAREN)
	} else if p.tok.Type != token.LBRACE && p.tok.Type != token.SEMICOLON && p.tok.Type != token.EOF {
		// Single return type
		results = append(results, p.parseType())
	}

	var body *ast.Block
	if p.interfaceFile {
		// In .bni files, function declarations have no body
	} else {
		body = p.parseBlock()
	}
	return &ast.FuncDecl{
		FuncPos: pos,
		Recv:    recv,
		Name:    name,
		Params:  params,
		Results: results,
		Body:    body,
	}
}

func (p *Parser) parseParamList() []*ast.ParamDecl {
	var params []*ast.ParamDecl
	params = append(params, p.parseParamDecl())
	for p.got(token.COMMA) {
		if p.tok.Type == token.RPAREN {
			break // trailing comma
		}
		params = append(params, p.parseParamDecl())
	}
	return params
}

func (p *Parser) parseParamDecl() *ast.ParamDecl {
	name := p.parseIdent()
	typ := p.parseType()
	return &ast.ParamDecl{Name: name, Type: typ}
}

func (p *Parser) parseTypeList() []ast.TypeExpr {
	var list []ast.TypeExpr
	list = append(list, p.parseType())
	for p.got(token.COMMA) {
		list = append(list, p.parseType())
	}
	return list
}

func (p *Parser) parseTypeDecl() ast.Decl {
	pos := p.tok.Pos
	p.expect(token.TYPE)

	if p.tok.Type == token.LPAREN {
		// Grouped type declarations — return only the first for now;
		// we'll wrap them in a list if needed. For simplicity, parse
		// each as a separate decl and return via a GroupDecl wrapper.
		// Actually, let's just parse the first spec for now.
		// TODO: handle grouped type declarations properly
		p.next()
		var decls []ast.Decl
		for p.tok.Type != token.RPAREN && p.tok.Type != token.EOF {
			name := p.parseIdent()
			decls = append(decls, p.parseTypeSpec(pos, name))
			if !p.got(token.SEMICOLON) {
				break
			}
		}
		p.expect(token.RPAREN)
		if len(decls) == 1 {
			return decls[0]
		}
		return &ast.GroupDecl{Decls: decls}
	}

	name := p.parseIdent()
	return p.parseTypeSpec(pos, name)
}

func (p *Parser) parseTypeSpec(typePos token.Pos, name *ast.Ident) *ast.TypeDecl {
	d := &ast.TypeDecl{TypePos: typePos, Name: name}
	if p.tok.Type == token.ASSIGN {
		// Type alias: type X = T
		p.next()
		d.Assign = true
		d.Type = p.parseType()
	} else if p.tok.Type == token.STRUCT {
		// Named struct: type X struct { ... }
		d.Type = p.parseStructType()
	} else {
		// Distinct type: type X T
		d.Type = p.parseType()
	}
	return d
}

func (p *Parser) parseVarDecl() ast.Decl {
	pos := p.tok.Pos
	p.expect(token.VAR)

	if p.tok.Type == token.LPAREN {
		// Grouped var declarations
		p.next()
		var decls []ast.Decl
		for p.tok.Type != token.RPAREN && p.tok.Type != token.EOF {
			decls = append(decls, p.parseVarSpec(pos))
			if !p.got(token.SEMICOLON) {
				break
			}
		}
		p.expect(token.RPAREN)
		if len(decls) == 1 {
			return decls[0]
		}
		return &ast.GroupDecl{Decls: decls}
	}

	return p.parseVarSpec(pos)
}

func (p *Parser) parseVarSpec(varPos token.Pos) *ast.VarDecl {
	name := p.parseIdent()
	d := &ast.VarDecl{VarPos: varPos, Name: name}

	if p.tok.Type == token.ASSIGN {
		// var x = expr (type inferred)
		p.next()
		d.Value = p.parseExpr()
	} else {
		// var x T or var x T = expr
		d.Type = p.parseType()
		if p.got(token.ASSIGN) {
			d.Value = p.parseExpr()
		}
	}
	return d
}

func (p *Parser) parseConstDecl() ast.Decl {
	pos := p.tok.Pos
	p.expect(token.CONST)

	if p.tok.Type == token.LPAREN {
		// Grouped const
		p.next()
		var decls []ast.Decl
		for p.tok.Type != token.RPAREN && p.tok.Type != token.EOF {
			decls = append(decls, p.parseConstSpec(pos))
			if !p.got(token.SEMICOLON) {
				break
			}
		}
		p.expect(token.RPAREN)
		if len(decls) == 1 {
			return decls[0]
		}
		return &ast.GroupDecl{Decls: decls}
	}

	return p.parseConstSpec(pos)
}

func (p *Parser) parseConstSpec(constPos token.Pos) *ast.ConstDecl {
	name := p.parseIdent()
	d := &ast.ConstDecl{ConstPos: constPos, Name: name}

	if p.tok.Type == token.ASSIGN {
		// const x = expr
		p.next()
		d.Value = p.parseExpr()
	} else if p.tok.Type != token.SEMICOLON && p.tok.Type != token.RPAREN {
		// const x T = expr or just const x (repeat) — check for type
		if p.tok.Type != token.ASSIGN {
			// Could be type or could be "=" — try type first
			d.Type = p.parseType()
		}
		if p.got(token.ASSIGN) {
			d.Value = p.parseExpr()
		}
		// If no type and no value, it's a bare name (repeat previous)
	}
	return d
}

// ============================================================
// Statement Parsing
// ============================================================

// ParseStmt parses a single statement.
func (p *Parser) ParseStmt() ast.Stmt {
	return p.parseStmt()
}

func (p *Parser) parseStmt() ast.Stmt {
	switch p.tok.Type {
	case token.LBRACE:
		return p.parseBlock()
	case token.IF:
		return p.parseIfStmt()
	case token.FOR:
		return p.parseForStmt()
	case token.SWITCH:
		return p.parseSwitchStmt()
	case token.RETURN:
		return p.parseReturnStmt()
	case token.BREAK:
		pos := p.tok.Pos
		p.next()
		return &ast.BreakStmt{BreakPos: pos}
	case token.CONTINUE:
		pos := p.tok.Pos
		p.next()
		return &ast.ContinueStmt{ContinuePos: pos}
	case token.VAR:
		return p.parseVarDecl().(*ast.VarDecl)
	case token.CONST:
		return p.parseConstDecl().(*ast.ConstDecl)
	case token.TYPE:
		// Function-local `type` declarations are rejected — `type`
		// is a top-level-only declaration. See claude-notes.md
		// § Scoping rules and differences-with-go.md.
		p.errorf("type declarations must be at package level, " +
			"not inside a function body")
		// Recover by parsing it anyway and dropping the result, so
		// downstream parsing isn't derailed by leftover tokens.
		return p.parseTypeDecl().(*ast.TypeDecl)
	default:
		return p.parseSimpleStmt()
	}
}

func (p *Parser) parseBlock() *ast.Block {
	lbrace := p.tok.Pos
	p.expect(token.LBRACE)
	var stmts []ast.Stmt
	for p.tok.Type != token.RBRACE && p.tok.Type != token.EOF {
		stmts = append(stmts, p.parseStmt())
		if !p.got(token.SEMICOLON) {
			if p.tok.Type != token.RBRACE && p.tok.Type != token.EOF {
				p.errorf("expected ; or }, got %s", p.tok.Type)
				p.next()
			}
		}
	}
	rbrace := p.tok.Pos
	p.expect(token.RBRACE)
	return &ast.Block{Lbrace: lbrace, Stmts: stmts, Rbrace: rbrace}
}

// parseSimpleStmt handles: expression statement, assignment, short var decl, inc/dec.
// D1: Parse LHS as expression list, then check operator to decide.
func (p *Parser) parseSimpleStmt() ast.Stmt {
	if p.tok.Type == token.SEMICOLON || p.tok.Type == token.RBRACE {
		return &ast.EmptyStmt{SemiPos: p.tok.Pos}
	}

	exprs := p.parseExprList()

	switch p.tok.Type {
	case token.DEFINE:
		// Short var decl: x, y := expr, expr
		op := p.tok.Pos
		p.next()
		rhs := p.parseExprList()
		names := make([]*ast.Ident, len(exprs))
		for i, e := range exprs {
			ident, ok := e.(*ast.Ident)
			if !ok {
				p.errorf("expected identifier on left side of :=")
				names[i] = &ast.Ident{NamePos: e.Pos(), Name: "<error>"}
			} else {
				names[i] = ident
			}
		}
		return &ast.ShortVarDecl{Names: names, Op: op, RHS: rhs}

	case token.ASSIGN:
		p.next()
		rhs := p.parseExprList()
		return &ast.AssignStmt{LHS: exprs, Op: token.ASSIGN, RHS: rhs}

	case token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN, token.QUO_ASSIGN,
		token.REM_ASSIGN, token.AND_ASSIGN, token.OR_ASSIGN, token.XOR_ASSIGN,
		token.SHL_ASSIGN, token.SHR_ASSIGN:
		op := p.tok.Type
		p.next()
		rhs := p.parseExpr()
		return &ast.AssignStmt{LHS: exprs, Op: op, RHS: []ast.Expr{rhs}}

	case token.INC:
		p.next()
		return &ast.IncDecStmt{X: exprs[0], Op: token.INC}

	case token.DEC:
		p.next()
		return &ast.IncDecStmt{X: exprs[0], Op: token.DEC}
	}

	// Expression statement
	if len(exprs) == 1 {
		return &ast.ExprStmt{X: exprs[0]}
	}
	// Multiple expressions without an operator — error
	p.errorf("expected assignment or declaration")
	return &ast.ExprStmt{X: exprs[0]}
}

func (p *Parser) parseReturnStmt() *ast.ReturnStmt {
	pos := p.tok.Pos
	p.next() // consume return

	var results []ast.Expr
	// Return may have no values. If the next token is on the same
	// logical line (i.e., not a semicolon), parse expression list.
	if p.tok.Type != token.SEMICOLON && p.tok.Type != token.RBRACE && p.tok.Type != token.EOF {
		results = p.parseExprList()
	}
	return &ast.ReturnStmt{ReturnPos: pos, Results: results}
}

func (p *Parser) parseIfStmt() *ast.IfStmt {
	pos := p.tok.Pos
	p.expect(token.IF)
	cond := p.parseExprNoCompositeLit()
	body := p.parseBlock()

	var elseStmt ast.Stmt
	if p.got(token.ELSE) {
		if p.tok.Type == token.IF {
			elseStmt = p.parseIfStmt()
		} else {
			elseStmt = p.parseBlock()
		}
	}
	return &ast.IfStmt{IfPos: pos, Cond: cond, Body: body, Else: elseStmt}
}

// parseForStmt handles all for-loop variants via D2 disambiguation.
func (p *Parser) parseForStmt() *ast.ForStmt {
	pos := p.tok.Pos
	p.expect(token.FOR)

	// Suppress composite literals in for header (D4 — { is ambiguous with block)
	oldNCL := p.noCompositeLit
	p.noCompositeLit = true
	defer func() { p.noCompositeLit = oldNCL }()

	// Infinite loop: for { }
	if p.tok.Type == token.LBRACE {
		body := p.parseBlock()
		return &ast.ForStmt{ForPos: pos, Body: body}
	}

	// Check for for-in: for ident in expr { }  or  for ident, ident in expr { }
	// "in" is a keyword so it can't appear as a normal expression.
	if p.tok.Type == token.IDENT {
		first := p.parseIdent()
		if p.tok.Type == token.IN {
			// Single variable for-in: for x in collection { }
			p.next() // consume "in"
			iter := p.parseExprNoCompositeLit()
			body := p.parseBlock()
			return &ast.ForStmt{ForPos: pos, Value: first, Iter: iter, Body: body}
		}
		if p.tok.Type == token.COMMA {
			p.next() // consume ","
			if p.tok.Type == token.IDENT {
				second := p.parseIdent()
				if p.tok.Type == token.IN {
					// Two variable for-in: for i, v in collection { }
					p.next() // consume "in"
					iter := p.parseExprNoCompositeLit()
					body := p.parseBlock()
					return &ast.ForStmt{ForPos: pos, Key: first, Value: second, Iter: iter, Body: body}
				}
				// Not for-in. We consumed "ident, ident" — reconstruct as expression.
				// This is: first , second <op> ...
				// It must be an expression list for assignment or short var decl.
				return p.finishForCStyle(pos, first, second)
			}
		}
		// Not for-in. We have a single identifier. Continue parsing as expression.
		return p.finishForAfterIdent(pos, first)
	}

	// Not starting with identifier — parse as simple statement.
	first := p.parseSimpleStmt()
	return p.finishForWithStmt(pos, first)
}

// finishForAfterIdent continues parsing a for statement after consuming one identifier.
// The identifier was not followed by "in" or ",".
func (p *Parser) finishForAfterIdent(forPos token.Pos, ident *ast.Ident) *ast.ForStmt {
	// Continue parsing the rest of the expression using ident as the start.
	// The identifier is a primary expression; apply postfix and binary operators.
	x := p.continuePostfix(ident)
	x = p.continueBinaryExpr(x, 1)

	// Now we have the first expression. Check what follows.
	first := p.finishSimpleStmt([]ast.Expr{x})
	return p.finishForWithStmt(forPos, first)
}

// finishForCStyle handles the case where we consumed "ident, ident" in a for header
// but it wasn't followed by "in". So it's a multi-assignment or short var decl
// as the init of a C-style for loop.
func (p *Parser) finishForCStyle(forPos token.Pos, first, second *ast.Ident) *ast.ForStmt {
	// We have two identifiers with a comma. This should be the start
	// of a short var decl or assignment: x, y := ... or x, y = ...
	exprs := []ast.Expr{first, second}
	for p.got(token.COMMA) {
		exprs = append(exprs, p.parseExpr())
	}
	init := p.finishSimpleStmt(exprs)
	return p.finishForWithStmt(forPos, init)
}

// finishForWithStmt continues parsing a for statement given the first statement.
func (p *Parser) finishForWithStmt(forPos token.Pos, first ast.Stmt) *ast.ForStmt {
	if p.tok.Type == token.LBRACE {
		// While-style: for cond { }
		cond := p.stmtToExpr(first)
		body := p.parseBlock()
		return &ast.ForStmt{ForPos: forPos, Cond: cond, Body: body}
	}

	// C-style: for init; cond; post { }
	p.expect(token.SEMICOLON)
	var cond ast.Expr
	if p.tok.Type != token.SEMICOLON {
		cond = p.parseExpr()
	}
	p.expect(token.SEMICOLON)
	var post ast.Stmt
	if p.tok.Type != token.LBRACE {
		post = p.parseSimpleStmt()
	}
	body := p.parseBlock()
	return &ast.ForStmt{ForPos: forPos, Init: first, Cond: cond, Post: post, Body: body}
}

// continuePostfix applies postfix operations to an already-parsed expression.
func (p *Parser) continuePostfix(x ast.Expr) ast.Expr {
	for {
		switch p.tok.Type {
		case token.DOT:
			p.next()
			sel := p.parseIdent()
			x = &ast.SelectorExpr{X: x, Sel: sel}
		case token.LBRACKET:
			x = p.parseIndexOrSlice(x)
		case token.LPAREN:
			x = p.parseCallExpr(x)
		default:
			return x
		}
	}
}

// continueBinaryExpr continues parsing binary expression after a primary/postfix expr.
// minPrec is the minimum precedence level (1 = lowest = ||).
func (p *Parser) continueBinaryExpr(x ast.Expr, minPrec int) ast.Expr {
	for {
		prec := p.binaryPrec(p.tok.Type)
		if prec < minPrec {
			return x
		}
		op := p.tok.Type
		p.next()
		y := p.parseUnaryExpr()
		// Check for higher-precedence operators on the right
		for {
			nextPrec := p.binaryPrec(p.tok.Type)
			if nextPrec <= prec {
				break
			}
			y = p.continueBinaryExpr(y, nextPrec)
		}
		x = &ast.BinaryExpr{X: x, Op: op, Y: y}
	}
}

// binaryPrec returns the precedence of a binary operator (0 if not a binary op).
func (p *Parser) binaryPrec(op token.Type) int {
	switch op {
	case token.LOR:
		return 1
	case token.LAND:
		return 2
	case token.EQ, token.NEQ, token.LT, token.GT, token.LEQ, token.GEQ:
		return 3
	case token.PIPE:
		return 4
	case token.CARET:
		return 5
	case token.AMP:
		return 6
	case token.SHL, token.SHR:
		return 7
	case token.PLUS, token.MINUS:
		return 8
	case token.STAR, token.SLASH, token.PERCENT:
		return 9
	}
	return 0
}

// finishSimpleStmt completes a simple statement given an already-parsed expression list.
func (p *Parser) finishSimpleStmt(exprs []ast.Expr) ast.Stmt {
	switch p.tok.Type {
	case token.DEFINE:
		op := p.tok.Pos
		p.next()
		rhs := p.parseExprList()
		names := make([]*ast.Ident, len(exprs))
		for i, e := range exprs {
			ident, ok := e.(*ast.Ident)
			if !ok {
				p.errorf("expected identifier on left side of :=")
				names[i] = &ast.Ident{NamePos: e.Pos(), Name: "<error>"}
			} else {
				names[i] = ident
			}
		}
		return &ast.ShortVarDecl{Names: names, Op: op, RHS: rhs}

	case token.ASSIGN:
		p.next()
		rhs := p.parseExprList()
		return &ast.AssignStmt{LHS: exprs, Op: token.ASSIGN, RHS: rhs}

	case token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN, token.QUO_ASSIGN,
		token.REM_ASSIGN, token.AND_ASSIGN, token.OR_ASSIGN, token.XOR_ASSIGN,
		token.SHL_ASSIGN, token.SHR_ASSIGN:
		op := p.tok.Type
		p.next()
		rhs := p.parseExpr()
		return &ast.AssignStmt{LHS: exprs, Op: op, RHS: []ast.Expr{rhs}}

	case token.INC:
		p.next()
		return &ast.IncDecStmt{X: exprs[0], Op: token.INC}

	case token.DEC:
		p.next()
		return &ast.IncDecStmt{X: exprs[0], Op: token.DEC}
	}

	if len(exprs) == 1 {
		return &ast.ExprStmt{X: exprs[0]}
	}
	p.errorf("expected assignment or declaration")
	return &ast.ExprStmt{X: exprs[0]}
}

// stmtToExpr extracts an expression from an ExprStmt.
func (p *Parser) stmtToExpr(s ast.Stmt) ast.Expr {
	if es, ok := s.(*ast.ExprStmt); ok {
		return es.X
	}
	p.errorf("expected expression")
	return &ast.Ident{NamePos: s.Pos(), Name: "<error>"}
}

func (p *Parser) parseSwitchStmt() *ast.SwitchStmt {
	pos := p.tok.Pos
	p.expect(token.SWITCH)

	var tag ast.Expr
	if p.tok.Type != token.LBRACE {
		tag = p.parseExprNoCompositeLit()
	}

	p.expect(token.LBRACE)
	var cases []*ast.CaseClause
	for p.tok.Type != token.RBRACE && p.tok.Type != token.EOF {
		cases = append(cases, p.parseCaseClause())
	}
	p.expect(token.RBRACE)

	return &ast.SwitchStmt{SwitchPos: pos, Tag: tag, Cases: cases}
}

func (p *Parser) parseCaseClause() *ast.CaseClause {
	cc := &ast.CaseClause{CasePos: p.tok.Pos}
	if p.tok.Type == token.CASE {
		p.next()
		cc.Exprs = p.parseExprList()
	} else {
		p.expect(token.DEFAULT)
	}
	p.expect(token.COLON)

	for p.tok.Type != token.CASE && p.tok.Type != token.DEFAULT &&
		p.tok.Type != token.RBRACE && p.tok.Type != token.EOF {
		cc.Body = append(cc.Body, p.parseStmt())
		if !p.got(token.SEMICOLON) {
			break
		}
	}
	return cc
}

// ============================================================
// Helpers
// ============================================================

func (p *Parser) parseIdent() *ast.Ident {
	tok := p.tok
	if p.tok.Type != token.IDENT {
		p.errorf("expected identifier, got %s", p.tok.Type)
		return &ast.Ident{NamePos: tok.Pos, Name: "<error>"}
	}
	p.next()
	return &ast.Ident{NamePos: tok.Pos, Name: tok.Literal}
}

func (p *Parser) parseExprList() []ast.Expr {
	var list []ast.Expr
	list = append(list, p.parseExpr())
	for p.got(token.COMMA) {
		if p.tok.Type == token.RPAREN {
			break // trailing comma
		}
		list = append(list, p.parseExpr())
	}
	return list
}
