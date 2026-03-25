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
	errs []Error
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

// Errors returns the list of parse errors.
func (p *Parser) Errors() []Error {
	return p.errs
}

// next advances to the next token.
func (p *Parser) next() {
	p.prev = p.tok
	p.tok = p.lex.Next()
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
	case token.STAR: // *T
		pos := p.tok.Pos
		p.next()
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

	case token.LBRACKET: // []T or [N]T
		pos := p.tok.Pos
		p.next() // consume [
		if p.tok.Type == token.RBRACKET {
			// []T — slice type
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
		return &ast.StringLit{ValuePos: tok.Pos, Value: tok.Literal}
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
		if p.tok.Type == token.LBRACE {
			// pkg.Type{ ... } — composite literal
			return p.parseCompositeLitBody(&ast.NamedType{Pkg: ident, Name: name})
		}
		// pkg.member — this becomes a SelectorExpr in postfix handling,
		// but here at primary level it's just a qualified identifier.
		// Return as SelectorExpr.
		return &ast.SelectorExpr{X: ident, Sel: name}
	}

	// Check for composite literal: Type{ ... }
	if p.tok.Type == token.LBRACE {
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

	// MakeArg = SliceType "," Expression | Type
	typ := p.parseType()
	var args []ast.Expr
	if p.tok.Type == token.COMMA {
		p.next()
		args = append(args, p.parseExpr())
	}
	p.expect(token.RPAREN)
	return &ast.BuiltinCall{BuiltinPos: pos, Builtin: token.MAKE, Type: typ, Args: args}
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
