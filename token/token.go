// Package token defines the token types for the Binate bootstrap lexer.
package token

// Type represents a token type.
type Type int

const (
	// Special tokens
	ILLEGAL Type = iota
	EOF
	SEMICOLON // auto-inserted or explicit

	// Literals
	IDENT
	INT    // integer literal
	STRING // "..."
	CHAR   // '.'

	// Keywords
	keywordStart
	BREAK
	CASE
	CONST
	CONTINUE
	DEFAULT
	ELSE
	FALSE
	FOR
	FUNC
	IF
	IMPL // [DEFERRED] but reserved
	IMPORT
	IN
	INTERFACE // [DEFERRED] but reserved
	NIL
	PACKAGE
	RETURN
	STRUCT
	SWITCH
	TRUE
	TYPE
	VAR
	keywordEnd

	// Builtin keywords (special syntax — take types as arguments)
	builtinStart
	MAKE
	MAKE_RAW_DEPRECATED
	BOX
	CAST
	BIT_CAST
	LEN
	UNSAFE_INDEX // [DEFERRED] but reserved
	builtinEnd

	// Operators
	PLUS       // +
	MINUS      // -
	STAR       // *
	SLASH      // /
	PERCENT    // %
	AMP        // &
	PIPE       // |
	CARET      // ^
	TILDE      // ~
	SHL        // <<
	SHR        // >>
	EQ         // ==
	NEQ        // !=
	LT         // <
	GT         // >
	LEQ        // <=
	GEQ        // >=
	LAND       // &&
	LOR        // ||
	NOT        // !
	ASSIGN     // =
	DEFINE     // :=
	ADD_ASSIGN // +=
	SUB_ASSIGN // -=
	MUL_ASSIGN // *=
	QUO_ASSIGN // /=
	REM_ASSIGN // %=
	AND_ASSIGN // &=
	OR_ASSIGN  // |=
	XOR_ASSIGN // ^=
	SHL_ASSIGN // <<=
	SHR_ASSIGN // >>=
	INC        // ++
	DEC        // --

	// Punctuation
	DOT      // .
	COMMA    // ,
	COLON    // :
	AT       // @
	HASH     // # [DEFERRED] but lexed
	LPAREN   // (
	RPAREN   // )
	LBRACKET // [
	RBRACKET // ]
	LBRACE   // {
	RBRACE   // }
	ELLIPSIS // ...
)

var typeNames = [...]string{
	ILLEGAL:   "ILLEGAL",
	EOF:       "EOF",
	SEMICOLON: ";",

	IDENT:  "IDENT",
	INT:    "INT",
	STRING: "STRING",
	CHAR:   "CHAR",

	BREAK:     "break",
	CASE:      "case",
	CONST:     "const",
	CONTINUE:  "continue",
	DEFAULT:   "default",
	ELSE:      "else",
	FALSE:     "false",
	FOR:       "for",
	FUNC:      "func",
	IF:        "if",
	IMPL:      "impl",
	IMPORT:    "import",
	IN:        "in",
	INTERFACE: "interface",
	NIL:       "nil",
	PACKAGE:   "package",
	RETURN:    "return",
	STRUCT:    "struct",
	SWITCH:    "switch",
	TRUE:      "true",
	TYPE:      "type",
	VAR:       "var",

	MAKE:                "make",
	MAKE_RAW_DEPRECATED: "make_raw_deprecated",
	BOX:                 "box",
	CAST:         "cast",
	BIT_CAST:     "bit_cast",
	LEN:          "len",
	UNSAFE_INDEX: "unsafe_index",

	PLUS:    "+",
	MINUS:   "-",
	STAR:    "*",
	SLASH:   "/",
	PERCENT: "%",
	AMP:     "&",
	PIPE:    "|",
	CARET:   "^",
	TILDE:   "~",
	SHL:     "<<",
	SHR:     ">>",
	EQ:      "==",
	NEQ:     "!=",
	LT:      "<",
	GT:      ">",
	LEQ:     "<=",
	GEQ:     ">=",
	LAND:    "&&",
	LOR:     "||",
	NOT:     "!",
	ASSIGN:  "=",
	DEFINE:  ":=",

	ADD_ASSIGN: "+=",
	SUB_ASSIGN: "-=",
	MUL_ASSIGN: "*=",
	QUO_ASSIGN: "/=",
	REM_ASSIGN: "%=",
	AND_ASSIGN: "&=",
	OR_ASSIGN:  "|=",
	XOR_ASSIGN: "^=",
	SHL_ASSIGN: "<<=",
	SHR_ASSIGN: ">>=",

	INC: "++",
	DEC: "--",

	DOT:      ".",
	COMMA:    ",",
	COLON:    ":",
	AT:       "@",
	HASH:     "#",
	LPAREN:   "(",
	RPAREN:   ")",
	LBRACKET: "[",
	RBRACKET: "]",
	LBRACE:   "{",
	RBRACE:   "}",
	ELLIPSIS: "...",
}

func (t Type) String() string {
	if int(t) < len(typeNames) {
		if s := typeNames[t]; s != "" {
			return s
		}
	}
	return "unknown"
}

// IsKeyword reports whether t is a keyword token.
func (t Type) IsKeyword() bool {
	return t > keywordStart && t < keywordEnd
}

// IsBuiltin reports whether t is a builtin keyword token.
func (t Type) IsBuiltin() bool {
	return t > builtinStart && t < builtinEnd
}

// keywords maps keyword strings to their token types.
var keywords map[string]Type

func init() {
	keywords = make(map[string]Type)
	for i := keywordStart + 1; i < keywordEnd; i++ {
		keywords[typeNames[i]] = i
	}
	for i := builtinStart + 1; i < builtinEnd; i++ {
		keywords[typeNames[i]] = i
	}
}

// Lookup maps an identifier string to its keyword token type,
// or IDENT if it is not a keyword.
func Lookup(ident string) Type {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}

// Pos represents a source position.
type Pos struct {
	File   string
	Line   int
	Column int
}

func (p Pos) String() string {
	if p.File != "" {
		return p.File + ":" + itoa(p.Line) + ":" + itoa(p.Column)
	}
	return itoa(p.Line) + ":" + itoa(p.Column)
}

// itoa is a simple int-to-string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// Token represents a single lexical token.
type Token struct {
	Type    Type
	Literal string // the raw text of the token
	Pos     Pos
}

// TriggersASI reports whether this token type triggers automatic
// semicolon insertion when it is the last token on a line.
func (t Type) TriggersASI() bool {
	switch t {
	case IDENT, INT, STRING, CHAR,
		TRUE, FALSE, NIL,
		BREAK, CONTINUE, RETURN,
		INC, DEC,
		RPAREN, RBRACKET, RBRACE:
		return true
	}
	return false
}
