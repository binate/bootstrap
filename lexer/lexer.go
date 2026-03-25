// Package lexer implements the Binate bootstrap lexer with automatic
// semicolon insertion.
package lexer

import (
	"github.com/binate/bootstrap/token"
)

// Lexer tokenizes Binate source code.
type Lexer struct {
	src  []byte
	file string

	pos     int  // current position in src
	readPos int  // next position to read
	ch      byte // current character (0 at EOF)

	line   int // 1-based line number
	col    int // 1-based column number
	lineAt int // pos at start of current line (for column calc)

	// ASI state
	lastTok token.Type     // type of last non-semicolon token emitted
	pending *internalToken // pending real token deferred by ASI semicolon
}

// New creates a new Lexer for the given source.
func New(src []byte, file string) *Lexer {
	l := &Lexer{
		src:  src,
		file: file,
		line: 1,
		col:  1,
	}
	l.advance()
	return l
}

// advance reads the next byte into l.ch.
func (l *Lexer) advance() {
	if l.readPos >= len(l.src) {
		l.ch = 0
	} else {
		l.ch = l.src[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
	l.col = l.pos - l.lineAt + 1
}

// peek returns the next byte without advancing, or 0 at EOF.
func (l *Lexer) peek() byte {
	if l.readPos >= len(l.src) {
		return 0
	}
	return l.src[l.readPos]
}

// curPos returns the current source position.
func (l *Lexer) curPos() token.Pos {
	return token.Pos{File: l.file, Line: l.line, Column: l.col}
}

// newline handles a newline character, updating line tracking.
func (l *Lexer) newline() {
	l.line++
	l.lineAt = l.readPos
}

// Next returns the next token. It handles automatic semicolon insertion.
func (l *Lexer) Next() token.Token {
	// If we have a pending real token (deferred by ASI), emit it now.
	if l.pending != nil {
		tok := *l.pending
		l.pending = nil
		l.lastTok = tok.Type
		return tok.Token
	}

	tok := l.scan()

	// ASI: if we crossed a newline boundary and the last token triggers ASI,
	// insert a semicolon before this token.
	if tok.Type != token.EOF && l.lastTok.TriggersASI() && tok.sawNewline {
		l.pending = &tok
		return token.Token{
			Type:    token.SEMICOLON,
			Literal: "\n",
			Pos:     tok.Pos,
		}
	}

	// At EOF, insert a final semicolon if the last token triggers ASI.
	if tok.Type == token.EOF && l.lastTok.TriggersASI() {
		l.lastTok = token.SEMICOLON
		return token.Token{
			Type:    token.SEMICOLON,
			Literal: "\n",
			Pos:     tok.Pos,
		}
	}

	if tok.Type != token.SEMICOLON {
		l.lastTok = tok.Type
	}
	return tok.Token
}

// internalToken is a token with extra metadata for ASI detection.
type internalToken struct {
	token.Token
	sawNewline bool
}

// scan reads the next raw token (no ASI logic).
func (l *Lexer) scan() internalToken {
	sawNewline := l.skipWhitespace()

	pos := l.curPos()
	var tok internalToken
	tok.Pos = pos
	tok.sawNewline = sawNewline

	ch := l.ch

	switch {
	case ch == 0:
		tok.Type = token.EOF
		tok.Literal = ""
		return tok

	case isLetter(ch):
		lit := l.scanIdentifier()
		tok.Type = token.Lookup(lit)
		tok.Literal = lit
		return tok

	case isDigit(ch):
		tok.Type, tok.Literal = l.scanNumber()
		return tok

	case ch == '"':
		tok.Type = token.STRING
		tok.Literal = l.scanString()
		return tok

	case ch == '\'':
		tok.Type = token.CHAR
		tok.Literal = l.scanChar()
		return tok
	}

	// Operators and punctuation
	l.advance()

	switch ch {
	case '+':
		tok = l.switch2(tok, token.PLUS, '=', token.ADD_ASSIGN)
		if tok.Type == token.PLUS && l.ch == '+' {
			l.advance()
			tok.Type = token.INC
			tok.Literal = "++"
			return tok
		}
	case '-':
		tok = l.switch2(tok, token.MINUS, '=', token.SUB_ASSIGN)
		if tok.Type == token.MINUS && l.ch == '-' {
			l.advance()
			tok.Type = token.DEC
			tok.Literal = "--"
			return tok
		}
	case '*':
		tok = l.switch2(tok, token.STAR, '=', token.MUL_ASSIGN)
	case '/':
		tok = l.switch2(tok, token.SLASH, '=', token.QUO_ASSIGN)
	case '%':
		tok = l.switch2(tok, token.PERCENT, '=', token.REM_ASSIGN)
	case '&':
		if l.ch == '&' {
			l.advance()
			tok.Type = token.LAND
			tok.Literal = "&&"
		} else if l.ch == '=' {
			l.advance()
			tok.Type = token.AND_ASSIGN
			tok.Literal = "&="
		} else {
			tok.Type = token.AMP
			tok.Literal = "&"
		}
	case '|':
		if l.ch == '|' {
			l.advance()
			tok.Type = token.LOR
			tok.Literal = "||"
		} else if l.ch == '=' {
			l.advance()
			tok.Type = token.OR_ASSIGN
			tok.Literal = "|="
		} else {
			tok.Type = token.PIPE
			tok.Literal = "|"
		}
	case '^':
		tok = l.switch2(tok, token.CARET, '=', token.XOR_ASSIGN)
	case '~':
		tok.Type = token.TILDE
		tok.Literal = "~"
	case '<':
		if l.ch == '<' {
			l.advance()
			if l.ch == '=' {
				l.advance()
				tok.Type = token.SHL_ASSIGN
				tok.Literal = "<<="
			} else {
				tok.Type = token.SHL
				tok.Literal = "<<"
			}
		} else if l.ch == '=' {
			l.advance()
			tok.Type = token.LEQ
			tok.Literal = "<="
		} else {
			tok.Type = token.LT
			tok.Literal = "<"
		}
	case '>':
		if l.ch == '>' {
			l.advance()
			if l.ch == '=' {
				l.advance()
				tok.Type = token.SHR_ASSIGN
				tok.Literal = ">>="
			} else {
				tok.Type = token.SHR
				tok.Literal = ">>"
			}
		} else if l.ch == '=' {
			l.advance()
			tok.Type = token.GEQ
			tok.Literal = ">="
		} else {
			tok.Type = token.GT
			tok.Literal = ">"
		}
	case '=':
		if l.ch == '=' {
			l.advance()
			tok.Type = token.EQ
			tok.Literal = "=="
		} else {
			tok.Type = token.ASSIGN
			tok.Literal = "="
		}
	case '!':
		if l.ch == '=' {
			l.advance()
			tok.Type = token.NEQ
			tok.Literal = "!="
		} else {
			tok.Type = token.NOT
			tok.Literal = "!"
		}
	case ':':
		if l.ch == '=' {
			l.advance()
			tok.Type = token.DEFINE
			tok.Literal = ":="
		} else {
			tok.Type = token.COLON
			tok.Literal = ":"
		}
	case '.':
		if l.ch == '.' && l.peek() == '.' {
			l.advance()
			l.advance()
			tok.Type = token.ELLIPSIS
			tok.Literal = "..."
		} else {
			tok.Type = token.DOT
			tok.Literal = "."
		}
	case ',':
		tok.Type = token.COMMA
		tok.Literal = ","
	case ';':
		tok.Type = token.SEMICOLON
		tok.Literal = ";"
	case '@':
		tok.Type = token.AT
		tok.Literal = "@"
	case '#':
		tok.Type = token.HASH
		tok.Literal = "#"
	case '(':
		tok.Type = token.LPAREN
		tok.Literal = "("
	case ')':
		tok.Type = token.RPAREN
		tok.Literal = ")"
	case '[':
		tok.Type = token.LBRACKET
		tok.Literal = "["
	case ']':
		tok.Type = token.RBRACKET
		tok.Literal = "]"
	case '{':
		tok.Type = token.LBRACE
		tok.Literal = "{"
	case '}':
		tok.Type = token.RBRACE
		tok.Literal = "}"
	default:
		tok.Type = token.ILLEGAL
		tok.Literal = string(ch)
	}

	return tok
}

// switch2 handles the pattern: if next char is `next`, use `b`; otherwise use `a`.
// Only call this for single-char lookahead cases.
func (l *Lexer) switch2(tok internalToken, a token.Type, next byte, b token.Type) internalToken {
	if l.ch == next {
		l.advance()
		tok.Type = b
		tok.Literal = typeStr(b)
	} else {
		tok.Type = a
		tok.Literal = typeStr(a)
	}
	return tok
}

func typeStr(t token.Type) string { return t.String() }

// skipWhitespace skips spaces, tabs, carriage returns, and newlines.
// Returns true if at least one newline was skipped.
func (l *Lexer) skipWhitespace() bool {
	sawNewline := false
	for {
		switch l.ch {
		case ' ', '\t', '\r':
			l.advance()
		case '\n':
			l.newline()
			l.advance()
			sawNewline = true
		case '/':
			if l.peek() == '/' {
				nl := l.skipLineComment()
				sawNewline = sawNewline || nl
			} else if l.peek() == '*' {
				nl := l.skipBlockComment()
				sawNewline = sawNewline || nl
			} else {
				return sawNewline
			}
		default:
			return sawNewline
		}
	}
}

// skipLineComment skips a // comment. Returns true if a newline follows.
func (l *Lexer) skipLineComment() bool {
	// Skip the //
	l.advance()
	l.advance()
	for l.ch != 0 && l.ch != '\n' {
		l.advance()
	}
	if l.ch == '\n' {
		l.newline()
		l.advance()
		return true
	}
	return false
}

// skipBlockComment skips a /* */ comment. Returns true if it contains newlines.
func (l *Lexer) skipBlockComment() bool {
	sawNewline := false
	// Skip the /*
	l.advance()
	l.advance()
	for l.ch != 0 {
		if l.ch == '\n' {
			l.newline()
			sawNewline = true
		}
		if l.ch == '*' && l.peek() == '/' {
			l.advance()
			l.advance()
			return sawNewline
		}
		l.advance()
	}
	// Unterminated block comment — the parser will report the error
	return sawNewline
}

// scanIdentifier reads an identifier or keyword.
func (l *Lexer) scanIdentifier() string {
	start := l.pos
	for isLetter(l.ch) || isDigit(l.ch) {
		l.advance()
	}
	return string(l.src[start:l.pos])
}

// scanNumber reads an integer literal (decimal, hex, octal, or binary).
func (l *Lexer) scanNumber() (token.Type, string) {
	start := l.pos

	if l.ch == '0' {
		l.advance()
		switch l.ch {
		case 'x', 'X':
			l.advance()
			if !isHexDigit(l.ch) {
				return token.ILLEGAL, string(l.src[start:l.pos])
			}
			for isHexDigit(l.ch) {
				l.advance()
			}
			return token.INT, string(l.src[start:l.pos])
		case 'o', 'O':
			l.advance()
			if !isOctalDigit(l.ch) {
				return token.ILLEGAL, string(l.src[start:l.pos])
			}
			for isOctalDigit(l.ch) {
				l.advance()
			}
			return token.INT, string(l.src[start:l.pos])
		case 'b', 'B':
			l.advance()
			if l.ch != '0' && l.ch != '1' {
				return token.ILLEGAL, string(l.src[start:l.pos])
			}
			for l.ch == '0' || l.ch == '1' {
				l.advance()
			}
			return token.INT, string(l.src[start:l.pos])
		default:
			// Just "0"
			return token.INT, string(l.src[start:l.pos])
		}
	}

	// Decimal
	for isDigit(l.ch) {
		l.advance()
	}
	return token.INT, string(l.src[start:l.pos])
}

// scanString reads a string literal including the quotes.
func (l *Lexer) scanString() string {
	start := l.pos
	l.advance() // skip opening "
	for l.ch != 0 && l.ch != '"' {
		if l.ch == '\n' {
			// Unterminated string
			return string(l.src[start:l.pos])
		}
		if l.ch == '\\' {
			l.advance() // skip backslash
			if l.ch == 0 || l.ch == '\n' {
				return string(l.src[start:l.pos])
			}
		}
		l.advance()
	}
	if l.ch == '"' {
		l.advance() // skip closing "
	}
	return string(l.src[start:l.pos])
}

// scanChar reads a char literal including the quotes.
func (l *Lexer) scanChar() string {
	start := l.pos
	l.advance() // skip opening '
	if l.ch == '\\' {
		l.advance() // skip backslash
		if l.ch == 0 || l.ch == '\n' {
			return string(l.src[start:l.pos])
		}
		l.advance() // skip escape char
		// Handle \xNN and \uNNNN
		// For \x, we already consumed the char after backslash.
		// The full escape validation happens in the parser/type checker.
	} else if l.ch != '\'' && l.ch != '\n' && l.ch != 0 {
		l.advance()
	}
	if l.ch == '\'' {
		l.advance() // skip closing '
	}
	return string(l.src[start:l.pos])
}

func isLetter(ch byte) bool {
	return ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '_'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F'
}

func isOctalDigit(ch byte) bool {
	return ch >= '0' && ch <= '7'
}
