// Package ast defines the AST node types for the Binate bootstrap parser.
package ast

import "github.com/binate/bootstrap/token"

// Node is the interface implemented by all AST nodes.
type Node interface {
	Pos() token.Pos
	node()
}

// Expr is the interface implemented by all expression nodes.
type Expr interface {
	Node
	expr()
}

// Stmt is the interface implemented by all statement nodes.
type Stmt interface {
	Node
	stmt()
}

// Decl is the interface implemented by all declaration nodes.
type Decl interface {
	Node
	decl()
}

// TypeExpr is the interface implemented by all type expression nodes.
type TypeExpr interface {
	Node
	typeExpr()
}

// ============================================================
// Source File
// ============================================================

// File represents a complete source file.
type File struct {
	Package token.Pos // position of "package" keyword
	PkgName *StringLit
	Imports []*ImportSpec
	Decls   []Decl
}

func (f *File) Pos() token.Pos { return f.Package }
func (f *File) node()          {}

// ImportSpec represents a single import.
type ImportSpec struct {
	ImportPos token.Pos // position of "import" keyword
	Alias     string    // alias name, or "" if none
	Path      *StringLit
}

func (s *ImportSpec) Pos() token.Pos { return s.ImportPos }
func (s *ImportSpec) node()          {}

// ============================================================
// Declarations
// ============================================================

// FuncDecl represents a function declaration.
type FuncDecl struct {
	FuncPos token.Pos
	Name    *Ident
	Params  []*ParamDecl
	Results []TypeExpr // nil or empty for no return; one or more for return types
	Body    *Block
}

func (d *FuncDecl) Pos() token.Pos { return d.FuncPos }
func (d *FuncDecl) node()          {}
func (d *FuncDecl) decl()          {}

// ParamDecl represents a function parameter.
type ParamDecl struct {
	Name *Ident
	Type TypeExpr
}

func (d *ParamDecl) Pos() token.Pos { return d.Name.Pos() }
func (d *ParamDecl) node()          {}

// VarDecl represents a var declaration.
type VarDecl struct {
	VarPos token.Pos
	Name   *Ident
	Type   TypeExpr // may be nil if type is inferred
	Value  Expr     // may be nil if no initializer
}

func (d *VarDecl) Pos() token.Pos { return d.VarPos }
func (d *VarDecl) node()          {}
func (d *VarDecl) decl()          {}
func (d *VarDecl) stmt()          {}

// ConstDecl represents a const declaration.
type ConstDecl struct {
	ConstPos token.Pos
	Name     *Ident
	Type     TypeExpr // may be nil
	Value    Expr     // may be nil (repeat previous in grouped const)
}

func (d *ConstDecl) Pos() token.Pos { return d.ConstPos }
func (d *ConstDecl) node()          {}
func (d *ConstDecl) decl()          {}
func (d *ConstDecl) stmt()          {}

// TypeDecl represents a type declaration.
type TypeDecl struct {
	TypePos token.Pos
	Name    *Ident
	Assign  bool     // true for alias (type X = T)
	Type    TypeExpr // the type definition
}

func (d *TypeDecl) Pos() token.Pos { return d.TypePos }
func (d *TypeDecl) node()          {}
func (d *TypeDecl) decl()          {}
func (d *TypeDecl) stmt()          {}

// GroupDecl represents a grouped declaration: type ( ... ), var ( ... ), const ( ... ).
type GroupDecl struct {
	Decls []Decl
}

func (d *GroupDecl) Pos() token.Pos {
	if len(d.Decls) > 0 {
		return d.Decls[0].Pos()
	}
	return token.Pos{}
}
func (d *GroupDecl) node() {}
func (d *GroupDecl) decl() {}
func (d *GroupDecl) stmt() {}

// ============================================================
// Statements
// ============================================================

// Block represents a brace-delimited block of statements.
type Block struct {
	Lbrace token.Pos
	Stmts  []Stmt
	Rbrace token.Pos
}

func (s *Block) Pos() token.Pos { return s.Lbrace }
func (s *Block) node()          {}
func (s *Block) stmt()          {}

// ExprStmt wraps an expression used as a statement.
type ExprStmt struct {
	X Expr
}

func (s *ExprStmt) Pos() token.Pos { return s.X.Pos() }
func (s *ExprStmt) node()          {}
func (s *ExprStmt) stmt()          {}

// AssignStmt represents an assignment (simple or compound).
type AssignStmt struct {
	LHS []Expr
	Op  token.Type // ASSIGN, ADD_ASSIGN, SUB_ASSIGN, etc.
	RHS []Expr
}

func (s *AssignStmt) Pos() token.Pos { return s.LHS[0].Pos() }
func (s *AssignStmt) node()          {}
func (s *AssignStmt) stmt()          {}

// ShortVarDecl represents a := declaration.
type ShortVarDecl struct {
	Names []*Ident
	Op    token.Pos // position of :=
	RHS   []Expr
}

func (s *ShortVarDecl) Pos() token.Pos { return s.Names[0].Pos() }
func (s *ShortVarDecl) node()          {}
func (s *ShortVarDecl) stmt()          {}

// IncDecStmt represents x++ or x--.
type IncDecStmt struct {
	X  Expr
	Op token.Type // INC or DEC
}

func (s *IncDecStmt) Pos() token.Pos { return s.X.Pos() }
func (s *IncDecStmt) node()          {}
func (s *IncDecStmt) stmt()          {}

// ReturnStmt represents a return statement.
type ReturnStmt struct {
	ReturnPos token.Pos
	Results   []Expr // may be nil
}

func (s *ReturnStmt) Pos() token.Pos { return s.ReturnPos }
func (s *ReturnStmt) node()          {}
func (s *ReturnStmt) stmt()          {}

// BreakStmt represents a break statement.
type BreakStmt struct {
	BreakPos token.Pos
}

func (s *BreakStmt) Pos() token.Pos { return s.BreakPos }
func (s *BreakStmt) node()          {}
func (s *BreakStmt) stmt()          {}

// ContinueStmt represents a continue statement.
type ContinueStmt struct {
	ContinuePos token.Pos
}

func (s *ContinueStmt) Pos() token.Pos { return s.ContinuePos }
func (s *ContinueStmt) node()          {}
func (s *ContinueStmt) stmt()          {}

// IfStmt represents an if statement.
type IfStmt struct {
	IfPos token.Pos
	Cond  Expr
	Body  *Block
	Else  Stmt // *IfStmt or *Block, or nil
}

func (s *IfStmt) Pos() token.Pos { return s.IfPos }
func (s *IfStmt) node()          {}
func (s *IfStmt) stmt()          {}

// ForStmt represents all for-loop variants.
type ForStmt struct {
	ForPos token.Pos
	Init   Stmt   // C-style: init; may be nil
	Cond   Expr   // condition; may be nil (infinite loop)
	Post   Stmt   // C-style: post; may be nil
	Key    *Ident // for-in: loop variable (index if two vars); may be nil
	Value  *Ident // for-in: loop variable (value, or sole var); may be nil
	Iter   Expr   // for-in: iterable expression; may be nil
	Body   *Block
}

func (s *ForStmt) Pos() token.Pos { return s.ForPos }
func (s *ForStmt) node()          {}
func (s *ForStmt) stmt()          {}

// SwitchStmt represents a switch statement.
type SwitchStmt struct {
	SwitchPos token.Pos
	Tag       Expr // may be nil (tagless switch not in bootstrap, but keep for future)
	Cases     []*CaseClause
}

func (s *SwitchStmt) Pos() token.Pos { return s.SwitchPos }
func (s *SwitchStmt) node()          {}
func (s *SwitchStmt) stmt()          {}

// CaseClause represents a single case or default clause.
type CaseClause struct {
	CasePos token.Pos
	Exprs   []Expr // nil for default
	Body    []Stmt
}

func (c *CaseClause) Pos() token.Pos { return c.CasePos }
func (c *CaseClause) node()          {}

// EmptyStmt represents an empty statement.
type EmptyStmt struct {
	SemiPos token.Pos
}

func (s *EmptyStmt) Pos() token.Pos { return s.SemiPos }
func (s *EmptyStmt) node()          {}
func (s *EmptyStmt) stmt()          {}

// ============================================================
// Expressions
// ============================================================

// Ident represents an identifier.
type Ident struct {
	NamePos token.Pos
	Name    string
}

func (e *Ident) Pos() token.Pos { return e.NamePos }
func (e *Ident) node()          {}
func (e *Ident) expr()          {}

// IntLit represents an integer literal.
type IntLit struct {
	ValuePos token.Pos
	Value    string // raw literal text (e.g., "42", "0xFF")
}

func (e *IntLit) Pos() token.Pos { return e.ValuePos }
func (e *IntLit) node()          {}
func (e *IntLit) expr()          {}

// StringLit represents a string literal.
type StringLit struct {
	ValuePos token.Pos
	Value    string // raw literal text including quotes
}

func (e *StringLit) Pos() token.Pos { return e.ValuePos }
func (e *StringLit) node()          {}
func (e *StringLit) expr()          {}

// CharLit represents a character literal.
type CharLit struct {
	ValuePos token.Pos
	Value    string // raw literal text including quotes
}

func (e *CharLit) Pos() token.Pos { return e.ValuePos }
func (e *CharLit) node()          {}
func (e *CharLit) expr()          {}

// BoolLit represents true or false.
type BoolLit struct {
	ValuePos token.Pos
	Value    bool
}

func (e *BoolLit) Pos() token.Pos { return e.ValuePos }
func (e *BoolLit) node()          {}
func (e *BoolLit) expr()          {}

// NilLit represents nil.
type NilLit struct {
	NilPos token.Pos
}

func (e *NilLit) Pos() token.Pos { return e.NilPos }
func (e *NilLit) node()          {}
func (e *NilLit) expr()          {}

// BinaryExpr represents a binary operation.
type BinaryExpr struct {
	X  Expr
	Op token.Type
	Y  Expr
}

func (e *BinaryExpr) Pos() token.Pos { return e.X.Pos() }
func (e *BinaryExpr) node()          {}
func (e *BinaryExpr) expr()          {}

// UnaryExpr represents a unary operation.
type UnaryExpr struct {
	OpPos token.Pos
	Op    token.Type // NOT, TILDE, MINUS, STAR, AMP
	X     Expr
}

func (e *UnaryExpr) Pos() token.Pos { return e.OpPos }
func (e *UnaryExpr) node()          {}
func (e *UnaryExpr) expr()          {}

// CallExpr represents a function call.
type CallExpr struct {
	Fun  Expr
	Args []Expr
}

func (e *CallExpr) Pos() token.Pos { return e.Fun.Pos() }
func (e *CallExpr) node()          {}
func (e *CallExpr) expr()          {}

// IndexExpr represents an index operation: x[i].
type IndexExpr struct {
	X     Expr
	Index Expr
}

func (e *IndexExpr) Pos() token.Pos { return e.X.Pos() }
func (e *IndexExpr) node()          {}
func (e *IndexExpr) expr()          {}

// SliceExpr represents a slice operation: x[lo:hi].
type SliceExpr struct {
	X  Expr
	Lo Expr // may be nil
	Hi Expr // may be nil
}

func (e *SliceExpr) Pos() token.Pos { return e.X.Pos() }
func (e *SliceExpr) node()          {}
func (e *SliceExpr) expr()          {}

// SelectorExpr represents a field or method access: x.y.
type SelectorExpr struct {
	X   Expr
	Sel *Ident
}

func (e *SelectorExpr) Pos() token.Pos { return e.X.Pos() }
func (e *SelectorExpr) node()          {}
func (e *SelectorExpr) expr()          {}

// CompositeLit represents a composite literal: Type{elements}.
type CompositeLit struct {
	Type     TypeExpr
	Elements []*Element
	Lbrace   token.Pos
	Rbrace   token.Pos
}

func (e *CompositeLit) Pos() token.Pos { return e.Type.Pos() }
func (e *CompositeLit) node()          {}
func (e *CompositeLit) expr()          {}

// Element represents one element in a composite literal.
type Element struct {
	Key   Expr // may be nil
	Value Expr
}

func (e *Element) Pos() token.Pos {
	if e.Key != nil {
		return e.Key.Pos()
	}
	return e.Value.Pos()
}
func (e *Element) node() {}

// BuiltinCall represents a call to a builtin keyword (make, box, cast, etc.).
type BuiltinCall struct {
	BuiltinPos token.Pos
	Builtin    token.Type // MAKE, BOX, CAST, BIT_CAST, LEN
	Type       TypeExpr   // for make, cast, bit_cast
	Args       []Expr     // expression arguments
}

func (e *BuiltinCall) Pos() token.Pos { return e.BuiltinPos }
func (e *BuiltinCall) node()          {}
func (e *BuiltinCall) expr()          {}

// ============================================================
// Type Expressions
// ============================================================

// NamedType represents a simple or qualified type name: T or pkg.T.
type NamedType struct {
	Pkg  *Ident // package qualifier, or nil
	Name *Ident
}

func (t *NamedType) Pos() token.Pos {
	if t.Pkg != nil {
		return t.Pkg.Pos()
	}
	return t.Name.Pos()
}
func (t *NamedType) node()     {}
func (t *NamedType) typeExpr() {}

// PointerType represents *T.
type PointerType struct {
	Star token.Pos
	Base TypeExpr
}

func (t *PointerType) Pos() token.Pos { return t.Star }
func (t *PointerType) node()          {}
func (t *PointerType) typeExpr()      {}

// ManagedPtrType represents @T.
type ManagedPtrType struct {
	At   token.Pos
	Base TypeExpr
}

func (t *ManagedPtrType) Pos() token.Pos { return t.At }
func (t *ManagedPtrType) node()          {}
func (t *ManagedPtrType) typeExpr()      {}

// ManagedSliceType represents @[]T.
type ManagedSliceType struct {
	At   token.Pos
	Elem TypeExpr
}

func (t *ManagedSliceType) Pos() token.Pos { return t.At }
func (t *ManagedSliceType) node()          {}
func (t *ManagedSliceType) typeExpr()      {}

// SliceType represents []T.
type SliceType struct {
	Lbrack token.Pos
	Elem   TypeExpr
}

func (t *SliceType) Pos() token.Pos { return t.Lbrack }
func (t *SliceType) node()          {}
func (t *SliceType) typeExpr()      {}

// ArrayType represents [N]T.
type ArrayType struct {
	Lbrack token.Pos
	Len    Expr
	Elem   TypeExpr
}

func (t *ArrayType) Pos() token.Pos { return t.Lbrack }
func (t *ArrayType) node()          {}
func (t *ArrayType) typeExpr()      {}

// StructType represents an anonymous struct type: struct { ... }.
type StructType struct {
	StructPos token.Pos
	Fields    []*StructField
}

func (t *StructType) Pos() token.Pos { return t.StructPos }
func (t *StructType) node()          {}
func (t *StructType) typeExpr()      {}

// StructField represents a field in a struct type.
type StructField struct {
	Name *Ident // nil for anonymous/embedded fields
	Type TypeExpr
}

func (f *StructField) Pos() token.Pos {
	if f.Name != nil {
		return f.Name.Pos()
	}
	return f.Type.Pos()
}
func (f *StructField) node() {}

// ParenType represents a parenthesized type: (T).
type ParenType struct {
	Lparen token.Pos
	Type   TypeExpr
}

func (t *ParenType) Pos() token.Pos { return t.Lparen }
func (t *ParenType) node()          {}
func (t *ParenType) typeExpr()      {}
