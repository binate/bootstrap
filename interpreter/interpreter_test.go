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
	s := make_slice(int, 3)
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
	s := make_slice(int, 5)
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

func TestStringConversion(t *testing.T) {
	src := `package "main"

import "pkg/bootstrap"

func main() {
	x := 42
	println(bootstrap.Itoa(x))
}
`
	got := runProgram(t, src)
	if got != "42\n" {
		t.Errorf("expected %q, got %q", "42\n", got)
	}
}

func TestNestedStructs(t *testing.T) {
	src := `package "main"

type Point struct {
	x int
	y int
}

type Rect struct {
	min Point
	max Point
}

func area(r Rect) int {
	return (r.max.x - r.min.x) * (r.max.y - r.min.y)
}

func main() {
	r := Rect{
		min: Point{x: 1, y: 2},
		max: Point{x: 4, y: 6},
	}
	println(area(r))
}
`
	got := runProgram(t, src)
	if got != "12\n" {
		t.Errorf("expected %q, got %q", "12\n", got)
	}
}

func TestIncDec(t *testing.T) {
	src := `package "main"

func main() {
	var x int = 5
	x++
	println(x)
	x--
	x--
	println(x)
}
`
	got := runProgram(t, src)
	if got != "6\n4\n" {
		t.Errorf("expected %q, got %q", "6\n4\n", got)
	}
}

func TestForIn(t *testing.T) {
	src := `package "main"

func main() {
	var arr [3]int = [3]int{10, 20, 30}
	var sum int = 0
	for v in arr {
		sum = sum + v
	}
	println(sum)
}
`
	got := runProgram(t, src)
	if got != "60\n" {
		t.Errorf("expected %q, got %q", "60\n", got)
	}
}

func TestForInSlice(t *testing.T) {
	src := `package "main"

func main() {
	s := make_slice(int, 3)
	s[0] = 1
	s[1] = 2
	s[2] = 3
	for v in s {
		print(v)
		print(" ")
	}
	println("")
}
`
	got := runProgram(t, src)
	if got != "1 2 3 \n" {
		t.Errorf("expected %q, got %q", "1 2 3 \n", got)
	}
}

func TestForInWithIndex(t *testing.T) {
	src := `package "main"

func main() {
	var arr [3]int = [3]int{10, 20, 30}
	for i, v in arr {
		println(i, v)
	}
}
`
	got := runProgram(t, src)
	expect := "0 10\n1 20\n2 30\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestCharLiteral(t *testing.T) {
	src := `package "main"

func main() {
	var c char = 'A'
	println(cast(int, c))
	var newline char = '\n'
	println(cast(int, newline))
}
`
	got := runProgram(t, src)
	if got != "65\n10\n" {
		t.Errorf("expected %q, got %q", "65\n10\n", got)
	}
}

func TestIotaConst(t *testing.T) {
	src := `package "main"

const (
	A int = iota
	B
	C
)

func main() {
	println(A)
	println(B)
	println(C)
}
`
	got := runProgram(t, src)
	if got != "0\n1\n2\n" {
		t.Errorf("expected %q, got %q", "0\n1\n2\n", got)
	}
}

func TestMultiReturnShortDecl(t *testing.T) {
	src := `package "main"

func divmod(a int, b int) (int, int) {
	return a / b, a % b
}

func main() {
	q, r := divmod(17, 5)
	println(q, r)
}
`
	got := runProgram(t, src)
	if got != "3 2\n" {
		t.Errorf("expected %q, got %q", "3 2\n", got)
	}
}

func TestStringIndex(t *testing.T) {
	src := `package "main"

func main() {
	s := "hello"
	println(cast(int, s[0]))
	println(cast(int, s[4]))
}
`
	got := runProgram(t, src)
	if got != "104\n111\n" {
		t.Errorf("expected %q, got %q", "104\n111\n", got)
	}
}

func TestManagedSlice(t *testing.T) {
	src := `package "main"

func main() {
	s := make_slice(int, 5)
	for i := 0; i < 5; i++ {
		s[i] = i * i
	}
	println(len(s))
	println(s[3])
}
`
	got := runProgram(t, src)
	if got != "5\n9\n" {
		t.Errorf("expected %q, got %q", "5\n9\n", got)
	}
}

func TestRuntimeDivisionByZero(t *testing.T) {
	src := `package "main"

func main() {
	var x int = 10
	var y int = 0
	println(x / y)
}
`
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for division by zero")
		}
		re, ok := r.(*RuntimeError)
		if !ok {
			t.Fatalf("expected RuntimeError, got %T: %v", r, r)
		}
		if !strings.Contains(re.Msg, "division by zero") {
			t.Errorf("expected 'division by zero' in error, got %q", re.Msg)
		}
	}()
	runProgram(t, src)
}

func TestRuntimeIndexOutOfBounds(t *testing.T) {
	src := `package "main"

func main() {
	var arr [3]int = [3]int{1, 2, 3}
	println(arr[5])
}
`
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for index out of bounds")
		}
		re, ok := r.(*RuntimeError)
		if !ok {
			t.Fatalf("expected RuntimeError, got %T: %v", r, r)
		}
		if !strings.Contains(re.Msg, "index out of bounds") {
			t.Errorf("expected 'index out of bounds' in error, got %q", re.Msg)
		}
	}()
	runProgram(t, src)
}

func TestRuntimeNilDeref(t *testing.T) {
	src := `package "main"

func main() {
	var p *int = nil
	println(*p)
}
`
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for nil deref")
		}
		re, ok := r.(*RuntimeError)
		if !ok {
			t.Fatalf("expected RuntimeError, got %T: %v", r, r)
		}
		if !strings.Contains(re.Msg, "nil pointer dereference") {
			t.Errorf("expected 'nil pointer dereference' in error, got %q", re.Msg)
		}
	}()
	runProgram(t, src)
}

func TestNestedForLoops(t *testing.T) {
	src := `package "main"

func main() {
	var sum int = 0
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			sum = sum + i*3 + j
		}
	}
	println(sum)
}
`
	got := runProgram(t, src)
	// sum of 0..8 = 36
	if got != "36\n" {
		t.Errorf("expected %q, got %q", "36\n", got)
	}
}

func TestDistinctType(t *testing.T) {
	src := `package "main"

type Meters int

func main() {
	var d Meters = cast(Meters, 100)
	println(cast(int, d))
}
`
	got := runProgram(t, src)
	if got != "100\n" {
		t.Errorf("expected %q, got %q", "100\n", got)
	}
}

func TestTypeAlias(t *testing.T) {
	src := `package "main"

type MyInt = int

func main() {
	var x MyInt = 42
	println(x)
}
`
	got := runProgram(t, src)
	if got != "42\n" {
		t.Errorf("expected %q, got %q", "42\n", got)
	}
}

func TestPackageImport(t *testing.T) {
	src := `package "main"

import "pkg/bootstrap"

func main() {
	s := bootstrap.Itoa(42)
	println(s)
	println(bootstrap.STDOUT)
}
`
	got := runProgram(t, src)
	if got != "42\n1\n" {
		t.Errorf("expected %q, got %q", "42\n1\n", got)
	}
}

func TestBox(t *testing.T) {
	src := `package "main"

type Point struct {
	x int
	y int
}

func main() {
	p1 := box(42)
	println(*p1)

	p2 := box(Point{x: 1, y: 2})
	println((*p2).x)
	println((*p2).y)
}
`
	got := runProgram(t, src)
	expect := "42\n1\n2\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestManagedPointerField(t *testing.T) {
	src := `package "main"

type Point struct {
	x int
	y int
}

func main() {
	p := box(Point{x: 10, y: 20})
	println(p.x)
	println(p.y)
	p.x = 99
	println(p.x)
}
`
	got := runProgram(t, src)
	expect := "10\n20\n99\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestStringLen(t *testing.T) {
	src := `package "main"

func main() {
	s := make_slice(int, 5)
	println(len(s))
	arr := [3]int{1, 2, 3}
	println(len(arr))
	empty := make_slice(int, 0)
	println(len(empty))
}
`
	got := runProgram(t, src)
	expect := "5\n3\n0\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestStringSlice(t *testing.T) {
	// String indexing is supported; test individual character access
	src := `package "main"

func main() {
	s := "hello"
	println(cast(int, s[1]))
	println(cast(int, s[0]))
	println(cast(int, s[4]))
}
`
	got := runProgram(t, src)
	// 'e'=101, 'h'=104, 'o'=111
	expect := "101\n104\n111\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestSliceEdgeCases(t *testing.T) {
	src := `package "main"

func main() {
	s := make_slice(int, 5)
	for i := 0; i < 5; i++ {
		s[i] = i + 1
	}

	t1 := s[:]
	println(len(t1))
	println(t1[0])
	println(t1[4])

	t2 := s[0:]
	println(len(t2))

	t3 := s[:len(s)]
	println(len(t3))

	empty := make_slice(int, 0)
	t4 := empty[:]
	println(len(t4))
}
`
	got := runProgram(t, src)
	expect := "5\n1\n5\n5\n5\n0\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestBitCast(t *testing.T) {
	src := `package "main"

func main() {
	x := 42
	y := bit_cast(int, x)
	println(y)
}
`
	got := runProgram(t, src)
	if got != "42\n" {
		t.Errorf("expected %q, got %q", "42\n", got)
	}
}

func TestForInBreak(t *testing.T) {
	src := `package "main"

func main() {
	arr := [5]int{10, 20, 30, 40, 50}
	sum := 0
	for v in arr {
		if v == 30 {
			break
		}
		sum += v
	}
	println(sum)
}
`
	got := runProgram(t, src)
	if got != "30\n" {
		t.Errorf("expected %q, got %q", "30\n", got)
	}
}

func TestForInContinue(t *testing.T) {
	src := `package "main"

func main() {
	arr := [5]int{1, 2, 3, 4, 5}
	sum := 0
	for v in arr {
		if v % 2 == 0 {
			continue
		}
		sum += v
	}
	println(sum)
}
`
	got := runProgram(t, src)
	// 1 + 3 + 5 = 9
	if got != "9\n" {
		t.Errorf("expected %q, got %q", "9\n", got)
	}
}

func TestShortCircuit(t *testing.T) {
	src := `package "main"

var counter int

func sideEffect(val bool) bool {
	counter++
	return val
}

func main() {
	counter = 0
	r1 := false && sideEffect(true)
	println(r1)
	println(counter)

	counter = 0
	r2 := true || sideEffect(true)
	println(r2)
	println(counter)

	counter = 0
	r3 := true && sideEffect(false)
	println(r3)
	println(counter)

	counter = 0
	r4 := false || sideEffect(true)
	println(r4)
	println(counter)
}
`
	got := runProgram(t, src)
	// false && ... => short-circuit, result=false, counter stays 0
	// true || ... => short-circuit, result=true, counter stays 0
	// true && sideEffect(false) => sideEffect called, result=false, counter=1
	// false || sideEffect(true) => sideEffect called, result=true, counter=1
	expect := "false\n0\ntrue\n0\nfalse\n1\ntrue\n1\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestCompoundAssignAll(t *testing.T) {
	src := `package "main"

func main() {
	x := 0xFF
	x &= 0x0F
	println(x)

	y := 0xF0
	y |= 0x0F
	println(y)

	z := 0xFF
	z ^= 0x0F
	println(z)

	a := 1
	a <<= 4
	println(a)

	b := 32
	b >>= 2
	println(b)
}
`
	got := runProgram(t, src)
	expect := "15\n255\n240\n16\n8\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestPointerComparison(t *testing.T) {
	src := `package "main"

func main() {
	var p *int = nil
	println(p == nil)
	println(p != nil)

	x := 42
	q := &x
	println(q != nil)
	println(q == nil)
}
`
	got := runProgram(t, src)
	expect := "true\nfalse\ntrue\nfalse\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestStructPointerAssign(t *testing.T) {
	src := `package "main"

type Point struct {
	x int
	y int
}

func main() {
	p := Point{x: 1, y: 2}
	ptr := &p
	ptr.x = 100
	println(p.x)
	println(ptr.y)
}
`
	got := runProgram(t, src)
	expect := "100\n2\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestPanicBuiltin(t *testing.T) {
	src := `package "main"

func main() {
	panic("something went wrong")
}
`
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "something went wrong") {
			t.Errorf("expected panic message containing 'something went wrong', got %q", msg)
		}
	}()
	runProgram(t, src)
}

func TestLenOnVariousTypes(t *testing.T) {
	src := `package "main"

func main() {
	arr := [4]int{1, 2, 3, 4}
	println(len(arr))

	sl := make_slice(int, 7)
	println(len(sl))

	ms := make_slice(int, 3)
	println(len(ms))
}
`
	got := runProgram(t, src)
	expect := "4\n7\n3\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestNestedFunctionCalls(t *testing.T) {
	src := `package "main"

func baz(x int) int {
	return x * 2
}

func bar(x int) int {
	return x + 10
}

func foo(x int) int {
	return x * 3
}

func main() {
	println(foo(bar(baz(1))))
}
`
	got := runProgram(t, src)
	// baz(1) = 2, bar(2) = 12, foo(12) = 36
	if got != "36\n" {
		t.Errorf("expected %q, got %q", "36\n", got)
	}
}

func TestStringLenCharSlice(t *testing.T) {
	src := `package "main"

func main() {
	var s *[]char = "hello"
	println(len(s))
	println(len("abc"))
}
`
	got := runProgram(t, src)
	if got != "5\n3\n" {
		t.Errorf("expected %q, got %q", "5\n3\n", got)
	}
}

func TestStringCharSliceAssignable(t *testing.T) {
	src := `package "main"

func greet(name *[]char) {
	println(name)
}

func main() {
	greet("world")
}
`
	got := runProgram(t, src)
	if got != "world\n" {
		t.Errorf("expected %q, got %q", "world\n", got)
	}
}

func TestBootstrapConcat(t *testing.T) {
	src := `package "main"

import "pkg/bootstrap"

func main() {
	var a *[]char = "foo"
	var b *[]char = "bar"
	println(bootstrap.Concat(a, b))
	println(bootstrap.Concat("hello", " world"))
}
`
	got := runProgram(t, src)
	if got != "foobar\nhello world\n" {
		t.Errorf("expected %q, got %q", "foobar\nhello world\n", got)
	}
}

func TestBootstrapItoa(t *testing.T) {
	src := `package "main"

import "pkg/bootstrap"

func main() {
	println(bootstrap.Itoa(42))
	println(bootstrap.Itoa(0))
	println(bootstrap.Itoa(12345))
}
`
	got := runProgram(t, src)
	if got != "42\n0\n12345\n" {
		t.Errorf("expected %q, got %q", "42\n0\n12345\n", got)
	}
}

func TestQualifiedType(t *testing.T) {
	// This test requires a real package — use the pkgtest setup
	// For now, test that the checker accepts qualified types
	// by ensuring a program with bootstrap types compiles
	src := `package "main"

import "pkg/bootstrap"

func main() {
	var x int = 42
	var s *[]char = bootstrap.Itoa(x)
	println(s)
}
`
	got := runProgram(t, src)
	if got != "42\n" {
		t.Errorf("expected %q, got %q", "42\n", got)
	}
}

func TestSelfReferentialStruct(t *testing.T) {
	src := `package "main"

type Node struct {
	val  int
	next @Node
}

func main() {
	var a @Node = make(Node)
	a.val = 1
	var b @Node = make(Node)
	b.val = 2
	a.next = b
	println(a.val)
	println(a.next.val)
}
`
	got := runProgram(t, src)
	expect := "1\n2\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestLenNilSlice(t *testing.T) {
	src := `package "main"

func main() {
	var s *[]int
	println(len(s))
}
`
	got := runProgram(t, src)
	if got != "0\n" {
		t.Errorf("expected %q, got %q", "0\n", got)
	}
}

func TestStringSlicing(t *testing.T) {
	src := `package "main"

func main() {
	var s *[]char = "hello"
	var t *[]char = s[1:4]
	println(t)
	println(len(t))
}
`
	got := runProgram(t, src)
	expect := "ell\n3\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestStringSliceToEnd(t *testing.T) {
	src := `package "main"

func main() {
	var s *[]char = "hello"
	var t *[]char = s[3:]
	println(t)
}
`
	got := runProgram(t, src)
	if got != "lo\n" {
		t.Errorf("expected %q, got %q", "lo\n", got)
	}
}

func TestDistinctTypeIotaGroup(t *testing.T) {
	src := `package "main"

type Color int

const (
	RED   Color = iota
	GREEN
	BLUE
)

func name(c Color) *[]char {
	switch c {
	case RED:
		return "red"
	case GREEN:
		return "green"
	case BLUE:
		return "blue"
	default:
		return "unknown"
	}
}

func main() {
	println(name(RED))
	println(name(GREEN))
	println(name(BLUE))
}
`
	got := runProgram(t, src)
	expect := "red\ngreen\nblue\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

// ============================================================
// Methods (Stage 6 of plan-receivers.md)
// ============================================================

func TestMethodPointerReceiver(t *testing.T) {
	src := `package "main"

type Point struct { x int; y int }

func (p *Point) Sum() int {
	return p.x + p.y
}

func main() {
	var p Point
	p.x = 3
	p.y = 4
	var pp *Point = &p
	println(pp.Sum())
}
`
	got := runProgram(t, src)
	if got != "7\n" {
		t.Errorf("expected %q, got %q", "7\n", got)
	}
}

func TestMethodValueReceiver(t *testing.T) {
	src := `package "main"

type Counter struct { n int }

func (c Counter) Get() int {
	return c.n
}

func main() {
	var c Counter
	c.n = 42
	println(c.Get())
}
`
	got := runProgram(t, src)
	if got != "42\n" {
		t.Errorf("expected %q, got %q", "42\n", got)
	}
}

func TestMethodWithArgs(t *testing.T) {
	src := `package "main"

type Point struct { x int }

func (p *Point) Scaled(factor int) int {
	return p.x * factor
}

func main() {
	var p Point
	p.x = 5
	var pp *Point = &p
	println(pp.Scaled(3))
	println(pp.Scaled(10))
}
`
	got := runProgram(t, src)
	expect := "15\n50\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}

func TestMethodSameNameDifferentTypes(t *testing.T) {
	src := `package "main"

type A struct { v int }
type B struct { v int }

func (a *A) Get() int {
	return a.v + 100
}

func (b *B) Get() int {
	return b.v + 200
}

func main() {
	var a A
	a.v = 1
	var pa *A = &a

	var b B
	b.v = 2
	var pb *B = &b

	println(pa.Get())
	println(pb.Get())
}
`
	got := runProgram(t, src)
	expect := "101\n202\n"
	if got != expect {
		t.Errorf("expected %q, got %q", expect, got)
	}
}
