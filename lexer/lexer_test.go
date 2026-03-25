package lexer

import (
	"testing"

	"github.com/binate/bootstrap/token"
)

// tok is a shorthand for expected token pairs.
type tok struct {
	typ token.Type
	lit string
}

func TestIdentifiersAndKeywords(t *testing.T) {
	input := `foo bar break const func if return struct type var`
	expected := []tok{
		{token.IDENT, "foo"},
		{token.IDENT, "bar"},
		{token.BREAK, "break"},
		{token.CONST, "const"},
		{token.FUNC, "func"},
		{token.IF, "if"},
		{token.RETURN, "return"},
		{token.STRUCT, "struct"},
		{token.TYPE, "type"},
		{token.VAR, "var"},
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestBuiltinKeywords(t *testing.T) {
	input := `make box cast bit_cast len`
	expected := []tok{
		{token.MAKE, "make"},
		{token.BOX, "box"},
		{token.CAST, "cast"},
		{token.BIT_CAST, "bit_cast"},
		{token.LEN, "len"},
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestPredeclaredNamesAreIdentifiers(t *testing.T) {
	input := `int uint bool byte char any iota`
	expected := []tok{
		{token.IDENT, "int"},
		{token.IDENT, "uint"},
		{token.IDENT, "bool"},
		{token.IDENT, "byte"},
		{token.IDENT, "char"},
		{token.IDENT, "any"},
		{token.IDENT, "iota"},
		{token.SEMICOLON, "\n"}, // ASI at EOF: identifier triggers
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestIntegerLiterals(t *testing.T) {
	input := `0 42 100 0xFF 0Xab 0o77 0O10 0b1010 0B11`
	expected := []tok{
		{token.INT, "0"},
		{token.INT, "42"},
		{token.INT, "100"},
		{token.INT, "0xFF"},
		{token.INT, "0Xab"},
		{token.INT, "0o77"},
		{token.INT, "0O10"},
		{token.INT, "0b1010"},
		{token.INT, "0B11"},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestStringLiterals(t *testing.T) {
	input := `"hello" "world\n" "tab\there" "esc\"quote"`
	expected := []tok{
		{token.STRING, `"hello"`},
		{token.STRING, `"world\n"`},
		{token.STRING, `"tab\there"`},
		{token.STRING, `"esc\"quote"`},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestCharLiterals(t *testing.T) {
	input := `'a' '\n' '\\'`
	expected := []tok{
		{token.CHAR, `'a'`},
		{token.CHAR, `'\n'`},
		{token.CHAR, `'\\'`},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestOperators(t *testing.T) {
	input := `+ - * / % & | ^ ~ << >> == != < > <= >= && || ! = := += -= *= /= %= &= |= ^= <<= >>= ++ --`
	expected := []tok{
		{token.PLUS, "+"}, {token.MINUS, "-"}, {token.STAR, "*"},
		{token.SLASH, "/"}, {token.PERCENT, "%"}, {token.AMP, "&"},
		{token.PIPE, "|"}, {token.CARET, "^"}, {token.TILDE, "~"},
		{token.SHL, "<<"}, {token.SHR, ">>"}, {token.EQ, "=="},
		{token.NEQ, "!="}, {token.LT, "<"}, {token.GT, ">"},
		{token.LEQ, "<="}, {token.GEQ, ">="}, {token.LAND, "&&"},
		{token.LOR, "||"}, {token.NOT, "!"}, {token.ASSIGN, "="},
		{token.DEFINE, ":="}, {token.ADD_ASSIGN, "+="}, {token.SUB_ASSIGN, "-="},
		{token.MUL_ASSIGN, "*="}, {token.QUO_ASSIGN, "/="}, {token.REM_ASSIGN, "%="},
		{token.AND_ASSIGN, "&="}, {token.OR_ASSIGN, "|="}, {token.XOR_ASSIGN, "^="},
		{token.SHL_ASSIGN, "<<="}, {token.SHR_ASSIGN, ">>="}, {token.INC, "++"},
		{token.DEC, "--"},
		{token.SEMICOLON, "\n"}, // ASI at EOF: -- triggers
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestPunctuation(t *testing.T) {
	input := `. , ; : @ # ( ) [ ] { } ...`
	expected := []tok{
		{token.DOT, "."}, {token.COMMA, ","}, {token.SEMICOLON, ";"},
		{token.COLON, ":"}, {token.AT, "@"}, {token.HASH, "#"},
		{token.LPAREN, "("}, {token.RPAREN, ")"}, {token.LBRACKET, "["},
		{token.RBRACKET, "]"}, {token.LBRACE, "{"}, {token.RBRACE, "}"},
		{token.ELLIPSIS, "..."},
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestLineComment(t *testing.T) {
	input := "x // this is a comment\ny"
	expected := []tok{
		{token.IDENT, "x"},
		{token.SEMICOLON, "\n"}, // ASI after x (newline in comment)
		{token.IDENT, "y"},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestBlockComment(t *testing.T) {
	input := "x /* block comment */ y"
	expected := []tok{
		{token.IDENT, "x"},
		{token.IDENT, "y"},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestBlockCommentWithNewline(t *testing.T) {
	input := "x /* block\ncomment */ y"
	expected := []tok{
		{token.IDENT, "x"},
		{token.SEMICOLON, "\n"}, // ASI: x triggers, block comment contains newline
		{token.IDENT, "y"},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestASIBasic(t *testing.T) {
	input := "x\ny"
	expected := []tok{
		{token.IDENT, "x"},
		{token.SEMICOLON, "\n"},
		{token.IDENT, "y"},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestASIAfterLiterals(t *testing.T) {
	input := "42\n\"hello\"\n'c'\ntrue\nfalse\nnil"
	expected := []tok{
		{token.INT, "42"},
		{token.SEMICOLON, "\n"},
		{token.STRING, `"hello"`},
		{token.SEMICOLON, "\n"},
		{token.CHAR, "'c'"},
		{token.SEMICOLON, "\n"},
		{token.TRUE, "true"},
		{token.SEMICOLON, "\n"},
		{token.FALSE, "false"},
		{token.SEMICOLON, "\n"},
		{token.NIL, "nil"},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestASIAfterClosingDelimiters(t *testing.T) {
	input := ")\n]\n}"
	expected := []tok{
		{token.RPAREN, ")"},
		{token.SEMICOLON, "\n"},
		{token.RBRACKET, "]"},
		{token.SEMICOLON, "\n"},
		{token.RBRACE, "}"},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestASIAfterIncDec(t *testing.T) {
	input := "x++\ny--"
	expected := []tok{
		{token.IDENT, "x"},
		{token.INC, "++"},
		{token.SEMICOLON, "\n"},
		{token.IDENT, "y"},
		{token.DEC, "--"},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestASIAfterBreakContinueReturn(t *testing.T) {
	input := "break\ncontinue\nreturn"
	expected := []tok{
		{token.BREAK, "break"},
		{token.SEMICOLON, "\n"},
		{token.CONTINUE, "continue"},
		{token.SEMICOLON, "\n"},
		{token.RETURN, "return"},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestNoASIAfterOperators(t *testing.T) {
	input := "x +\ny"
	expected := []tok{
		{token.IDENT, "x"},
		{token.PLUS, "+"},
		// No ASI after +
		{token.IDENT, "y"},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestNoASIAfterOpeningDelimiters(t *testing.T) {
	input := "(\n[\n{"
	expected := []tok{
		{token.LPAREN, "("},
		// No ASI after (
		{token.LBRACKET, "["},
		// No ASI after [
		{token.LBRACE, "{"},
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestNoASIAfterComma(t *testing.T) {
	input := "x,\ny"
	expected := []tok{
		{token.IDENT, "x"},
		{token.COMMA, ","},
		// No ASI after ,
		{token.IDENT, "y"},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestTrailingCommaPattern(t *testing.T) {
	input := "foo(\n1,\n2,\n)"
	expected := []tok{
		{token.IDENT, "foo"},
		{token.LPAREN, "("},
		// No ASI after ( — newline follows but ( doesn't trigger ASI
		{token.INT, "1"},
		{token.COMMA, ","},
		// No ASI after , — newline follows but , doesn't trigger ASI
		{token.INT, "2"},
		{token.COMMA, ","},
		// No ASI after , — newline follows but , doesn't trigger ASI
		{token.RPAREN, ")"},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestFuncDecl(t *testing.T) {
	input := `func add(a int, b int) int {
	return a + b
}`
	expected := []tok{
		{token.FUNC, "func"},
		{token.IDENT, "add"},
		{token.LPAREN, "("},
		{token.IDENT, "a"},
		{token.IDENT, "int"},
		{token.COMMA, ","},
		{token.IDENT, "b"},
		{token.IDENT, "int"},
		{token.RPAREN, ")"},
		{token.IDENT, "int"},
		{token.LBRACE, "{"},
		// No ASI after { — newline follows but { doesn't trigger ASI
		{token.RETURN, "return"},
		{token.IDENT, "a"},
		{token.PLUS, "+"},
		{token.IDENT, "b"},
		{token.SEMICOLON, "\n"}, // ASI after b (newline before })
		{token.RBRACE, "}"},
		{token.SEMICOLON, "\n"}, // ASI at EOF after }
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestPackageAndImport(t *testing.T) {
	input := "package \"main\"\n\nimport \"pkg/foo\"\n"
	expected := []tok{
		{token.PACKAGE, "package"},
		{token.STRING, `"main"`},
		{token.SEMICOLON, "\n"}, // ASI after string literal
		{token.IMPORT, "import"},
		{token.STRING, `"pkg/foo"`},
		{token.SEMICOLON, "\n"}, // ASI after string literal
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestPositionTracking(t *testing.T) {
	input := "x\ny\nz"
	l := New([]byte(input), "test.bn")

	tok1 := l.Next()
	if tok1.Pos.Line != 1 || tok1.Pos.Column != 1 {
		t.Errorf("token 1: want 1:1, got %d:%d", tok1.Pos.Line, tok1.Pos.Column)
	}

	l.Next() // semicolon

	tok2 := l.Next()
	if tok2.Pos.Line != 2 || tok2.Pos.Column != 1 {
		t.Errorf("token 2: want 2:1, got %d:%d", tok2.Pos.Line, tok2.Pos.Column)
	}

	l.Next() // semicolon

	tok3 := l.Next()
	if tok3.Pos.Line != 3 || tok3.Pos.Column != 1 {
		t.Errorf("token 3: want 3:1, got %d:%d", tok3.Pos.Line, tok3.Pos.Column)
	}
}

func TestIllegalCharacter(t *testing.T) {
	input := "$"
	l := New([]byte(input), "")
	tok := l.Next()
	if tok.Type != token.ILLEGAL {
		t.Errorf("want ILLEGAL, got %s", tok.Type)
	}
}

func TestEmptyInput(t *testing.T) {
	input := ""
	expected := []tok{
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestManagedPointerSyntax(t *testing.T) {
	input := `@Point @[]int`
	expected := []tok{
		{token.AT, "@"},
		{token.IDENT, "Point"},
		{token.AT, "@"},
		{token.LBRACKET, "["},
		{token.RBRACKET, "]"},
		{token.IDENT, "int"},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestShortVarDecl(t *testing.T) {
	input := "x := 42"
	expected := []tok{
		{token.IDENT, "x"},
		{token.DEFINE, ":="},
		{token.INT, "42"},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

func TestForLoop(t *testing.T) {
	input := "for i := 0; i < n; i++ {\n}"
	expected := []tok{
		{token.FOR, "for"},
		{token.IDENT, "i"},
		{token.DEFINE, ":="},
		{token.INT, "0"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "i"},
		{token.LT, "<"},
		{token.IDENT, "n"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "i"},
		{token.INC, "++"},
		{token.LBRACE, "{"},
		{token.RBRACE, "}"},
		{token.SEMICOLON, "\n"}, // ASI at EOF
		{token.EOF, ""},
	}
	checkTokens(t, input, expected)
}

// checkTokens is a helper that lexes the input and checks token types and literals.
func checkTokens(t *testing.T, input string, expected []tok) {
	t.Helper()
	l := New([]byte(input), "")
	for i, exp := range expected {
		got := l.Next()
		if got.Type != exp.typ {
			t.Errorf("token %d: want type %s, got %s (lit=%q)", i, exp.typ, got.Type, got.Literal)
		}
		if got.Literal != exp.lit {
			t.Errorf("token %d: want lit %q, got %q (type=%s)", i, exp.lit, got.Literal, got.Type)
		}
	}
}
