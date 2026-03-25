package interpreter

import (
	"strings"
	"testing"

	"github.com/binate/bootstrap/parser"
	"github.com/binate/bootstrap/types"
)

func runProgram(t *testing.T, src string) string {
	t.Helper()
	p := parser.New([]byte(src), "test.bn")
	f := p.ParseFile()
	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Fatalf("parse error: %s", e)
		}
	}
	c := types.NewChecker()
	c.Check(f)
	if len(c.Errors()) > 0 {
		for _, e := range c.Errors() {
			t.Fatalf("type error: %s", e)
		}
	}

	var buf strings.Builder
	interp := New()
	interp.SetStdout(&buf)
	interp.Run(f, c)
	return buf.String()
}

func TestHelloWorld(t *testing.T) {
	src := `package "main"

func main() {
	println("hello world")
}
`
	got := runProgram(t, src)
	if got != "hello world\n" {
		t.Errorf("expected %q, got %q", "hello world\n", got)
	}
}

func TestArithmetic(t *testing.T) {
	src := `package "main"

func main() {
	println(2 + 3 * 4)
	println(10 - 3)
	println(15 / 4)
	println(15 % 4)
}
`
	got := runProgram(t, src)
	expect := "14\n7\n3\n3\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestVariables(t *testing.T) {
	src := `package "main"

func main() {
	x := 10
	y := 20
	println(x + y)
	x = 42
	println(x)
}
`
	got := runProgram(t, src)
	expect := "30\n42\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestVarDecl(t *testing.T) {
	src := `package "main"

func main() {
	var x int = 5
	var y int
	println(x)
	println(y)
}
`
	got := runProgram(t, src)
	expect := "5\n0\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestFunctionCall(t *testing.T) {
	src := `package "main"

func add(a int, b int) int {
	return a + b
}

func main() {
	println(add(3, 4))
}
`
	got := runProgram(t, src)
	if got != "7\n" {
		t.Errorf("expected %q, got %q", "7\n", got)
	}
}

func TestRecursion(t *testing.T) {
	src := `package "main"

func fib(n int) int {
	if n <= 1 {
		return n
	}
	return fib(n - 1) + fib(n - 2)
}

func main() {
	println(fib(10))
}
`
	got := runProgram(t, src)
	if got != "55\n" {
		t.Errorf("expected %q, got %q", "55\n", got)
	}
}

func TestIfElse(t *testing.T) {
	src := `package "main"

func abs(x int) int {
	if x >= 0 {
		return x
	} else {
		return 0 - x
	}
}

func main() {
	println(abs(5))
	println(abs(0 - 3))
}
`
	got := runProgram(t, src)
	expect := "5\n3\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestIfElseIf(t *testing.T) {
	src := `package "main"

func classify(x int) int {
	if x > 0 {
		return 1
	} else if x < 0 {
		return 0 - 1
	} else {
		return 0
	}
}

func main() {
	println(classify(5))
	println(classify(0 - 3))
	println(classify(0))
}
`
	got := runProgram(t, src)
	expect := "1\n-1\n0\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestForCStyle(t *testing.T) {
	src := `package "main"

func main() {
	sum := 0
	for i := 1; i <= 10; i++ {
		sum += i
	}
	println(sum)
}
`
	got := runProgram(t, src)
	if got != "55\n" {
		t.Errorf("expected %q, got %q", "55\n", got)
	}
}

func TestForWhile(t *testing.T) {
	src := `package "main"

func main() {
	x := 10
	for x > 0 {
		x = x - 3
	}
	println(x)
}
`
	got := runProgram(t, src)
	if got != "-2\n" {
		t.Errorf("expected %q, got %q", "-2\n", got)
	}
}

func TestForBreak(t *testing.T) {
	src := `package "main"

func main() {
	sum := 0
	for i := 0; i < 100; i++ {
		if i >= 5 {
			break
		}
		sum += i
	}
	println(sum)
}
`
	got := runProgram(t, src)
	if got != "10\n" {
		t.Errorf("expected %q, got %q", "10\n", got)
	}
}

func TestForContinue(t *testing.T) {
	src := `package "main"

func main() {
	sum := 0
	for i := 0; i < 10; i++ {
		if i % 2 == 0 {
			continue
		}
		sum += i
	}
	println(sum)
}
`
	got := runProgram(t, src)
	if got != "25\n" {
		t.Errorf("expected %q, got %q", "25\n", got)
	}
}

func TestSwitch(t *testing.T) {
	src := `package "main"

func name(x int) {
	switch x {
	case 1:
		println("one")
	case 2:
		println("two")
	default:
		println("other")
	}
}

func main() {
	name(1)
	name(2)
	name(99)
}
`
	got := runProgram(t, src)
	expect := "one\ntwo\nother\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestStruct(t *testing.T) {
	src := `package "main"

type Point struct {
	x int
	y int
}

func main() {
	p := Point{x: 3, y: 4}
	println(p.x)
	println(p.y)
	p.x = 10
	println(p.x)
}
`
	got := runProgram(t, src)
	expect := "3\n4\n10\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestMakeAndBox(t *testing.T) {
	src := `package "main"

func main() {
	p := box(42)
	println(*p)
}
`
	got := runProgram(t, src)
	if got != "42\n" {
		t.Errorf("expected %q, got %q", "42\n", got)
	}
}

func TestMakeSlice(t *testing.T) {
	src := `package "main"

func main() {
	s := make([]int, 3)
	s[0] = 10
	s[1] = 20
	s[2] = 30
	println(s[0])
	println(s[1])
	println(s[2])
	println(len(s))
}
`
	got := runProgram(t, src)
	expect := "10\n20\n30\n3\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestCast(t *testing.T) {
	src := `package "main"

func main() {
	x := 256 + 42
	y := cast(int8, x)
	println(y)
}
`
	got := runProgram(t, src)
	if got != "42\n" {
		t.Errorf("expected %q, got %q", "42\n", got)
	}
}

func TestMultipleReturns(t *testing.T) {
	src := `package "main"

func divide(a int, b int) (int, bool) {
	if b == 0 {
		return 0, false
	}
	return a / b, true
}

func main() {
	q, ok := divide(10, 3)
	println(q)
	println(ok)
	q2, ok2 := divide(10, 0)
	println(q2)
	println(ok2)
}
`
	got := runProgram(t, src)
	expect := "3\ntrue\n0\nfalse\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestConstDecl(t *testing.T) {
	src := `package "main"

const maxSize = 100

func main() {
	println(maxSize)
}
`
	got := runProgram(t, src)
	if got != "100\n" {
		t.Errorf("expected %q, got %q", "100\n", got)
	}
}

func TestCompoundAssignment(t *testing.T) {
	src := `package "main"

func main() {
	x := 10
	x += 5
	println(x)
	x -= 3
	println(x)
	x *= 2
	println(x)
	x /= 4
	println(x)
	x %= 5
	println(x)
}
`
	got := runProgram(t, src)
	expect := "15\n12\n24\n6\n1\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestBitwiseOps(t *testing.T) {
	src := `package "main"

func main() {
	println(0xFF & 0x0F)
	println(0xF0 | 0x0F)
	println(0xFF ^ 0x0F)
	println(1 << 4)
	println(32 >> 2)
	println(~0)
}
`
	got := runProgram(t, src)
	expect := "15\n255\n240\n16\n8\n-1\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestLogicalOps(t *testing.T) {
	src := `package "main"

func main() {
	println(true && true)
	println(true && false)
	println(false || true)
	println(false || false)
	println(!true)
	println(!false)
}
`
	got := runProgram(t, src)
	expect := "true\nfalse\ntrue\nfalse\nfalse\ntrue\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestNilComparison(t *testing.T) {
	src := `package "main"

func main() {
	var p *int = nil
	println(p == nil)
	println(p != nil)
}
`
	got := runProgram(t, src)
	expect := "true\nfalse\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestForwardReference(t *testing.T) {
	src := `package "main"

func main() {
	println(helper())
}

func helper() int {
	return 42
}
`
	got := runProgram(t, src)
	if got != "42\n" {
		t.Errorf("expected %q, got %q", "42\n", got)
	}
}

func TestStringEscape(t *testing.T) {
	src := `package "main"

func main() {
	print("a\tb\nc")
}
`
	got := runProgram(t, src)
	expect := "a\tb\nc"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestArrayLiteral(t *testing.T) {
	src := `package "main"

func main() {
	a := [3]int{10, 20, 30}
	println(a[0])
	println(a[1])
	println(a[2])
	println(len(a))
}
`
	got := runProgram(t, src)
	expect := "10\n20\n30\n3\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestSliceExpr(t *testing.T) {
	src := `package "main"

func main() {
	s := make([]int, 5)
	s[0] = 1
	s[1] = 2
	s[2] = 3
	s[3] = 4
	s[4] = 5
	t := s[1:4]
	println(len(t))
	println(t[0])
	println(t[1])
	println(t[2])
}
`
	got := runProgram(t, src)
	expect := "3\n2\n3\n4\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestGroupedConst(t *testing.T) {
	src := `package "main"

const (
	a = 1
	b = 2
	c = 3
)

func main() {
	println(a)
	println(b)
	println(c)
}
`
	got := runProgram(t, src)
	expect := "1\n2\n3\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestPointerDeref(t *testing.T) {
	src := `package "main"

func main() {
	x := 42
	p := &x
	println(*p)
	*p = 100
	println(x)
}
`
	got := runProgram(t, src)
	expect := "42\n100\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestPrintMultipleArgs(t *testing.T) {
	src := `package "main"

func main() {
	println("sum:", 1 + 2)
}
`
	got := runProgram(t, src)
	if got != "sum: 3\n" {
		t.Errorf("expected %q, got %q", "sum: 3\n", got)
	}
}
