package types

import (
	"strings"
	"testing"

	"github.com/binate/bootstrap/parser"
)

// checkFile is a helper that parses and type-checks a source string.
func checkFile(t *testing.T, src string) *Checker {
	t.Helper()
	p := parser.New([]byte(src), "test.bn")
	f := p.ParseFile()
	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Fatalf("parse error: %s", e)
		}
	}
	c := NewChecker()
	c.Check(f)
	return c
}

func expectNoErrors(t *testing.T, c *Checker) {
	t.Helper()
	if len(c.Errors()) > 0 {
		for _, e := range c.Errors() {
			t.Errorf("type error: %s", e)
		}
	}
}

func expectError(t *testing.T, c *Checker, substr string) {
	t.Helper()
	for _, e := range c.Errors() {
		if strings.Contains(e.Msg, substr) {
			return
		}
	}
	t.Errorf("expected error containing %q, got errors: %v", substr, c.Errors())
}

func TestCheckSimpleFunction(t *testing.T) {
	src := `package "main"

func add(a int, b int) int {
	return a + b
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckVarDecl(t *testing.T) {
	src := `package "main"

func main() {
	var x int = 42
	var y = 10
	x = y
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckShortVarDecl(t *testing.T) {
	src := `package "main"

func main() {
	x := 42
	y := x + 1
	x = y
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckTypeMismatch(t *testing.T) {
	src := `package "main"

func main() {
	var x int = true
}
`
	c := checkFile(t, src)
	expectError(t, c, "cannot assign")
}

func TestCheckUndefined(t *testing.T) {
	src := `package "main"

func main() {
	x = 42
}
`
	c := checkFile(t, src)
	expectError(t, c, "undefined")
}

func TestCheckReturnTypeMismatch(t *testing.T) {
	src := `package "main"

func foo() int {
	return true
}
`
	c := checkFile(t, src)
	expectError(t, c, "cannot use")
}

func TestCheckBoolCondition(t *testing.T) {
	src := `package "main"

func main() {
	if 42 {
	}
}
`
	c := checkFile(t, src)
	expectError(t, c, "non-bool condition")
}

func TestCheckIfElse(t *testing.T) {
	src := `package "main"

func abs(x int) int {
	if x > 0 {
		return x
	} else {
		return 0 - x
	}
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckForCStyle(t *testing.T) {
	src := `package "main"

func sum(n int) int {
	var s int = 0
	for i := 0; i < n; i++ {
		s += i
	}
	return s
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckForWhile(t *testing.T) {
	src := `package "main"

func main() {
	x := 10
	for x > 0 {
		x = x - 1
	}
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckStruct(t *testing.T) {
	src := `package "main"

type Point struct {
	x int
	y int
}

func main() {
	p := Point{x: 1, y: 2}
	var z int = p.x + p.y
	z = z
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckStructFieldAccess(t *testing.T) {
	src := `package "main"

type Point struct {
	x int
	y int
}

func main() {
	p := Point{x: 1, y: 2}
	p.z
}
`
	c := checkFile(t, src)
	expectError(t, c, "no field z")
}

func TestCheckPointers(t *testing.T) {
	src := `package "main"

func main() {
	x := 42
	var p *int = &x
	var y int = *p
	y = y
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckDerefNonPointer(t *testing.T) {
	src := `package "main"

func main() {
	x := 42
	y := *x
	y = y
}
`
	c := checkFile(t, src)
	expectError(t, c, "cannot dereference")
}

func TestCheckSlice(t *testing.T) {
	src := `package "main"

func main() {
	var s []int
	var x int = len(s)
	x = x
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckArray(t *testing.T) {
	src := `package "main"

func main() {
	var a [10]int
	var x int = a[0]
	var n int = len(a)
	x = n
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckMake(t *testing.T) {
	src := `package "main"

type Point struct {
	x int
	y int
}

func main() {
	var p @Point = make(Point)
	p = p
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckMakeSlice(t *testing.T) {
	src := `package "main"

func main() {
	var s @[]int = make_slice(int, 10)
	s = s
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckBox(t *testing.T) {
	src := `package "main"

func main() {
	var p @int = box(42)
	p = p
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckCast(t *testing.T) {
	src := `package "main"

func main() {
	x := 42
	var y int64 = cast(int64, x)
	y = y
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckLen(t *testing.T) {
	src := `package "main"

func main() {
	var n int = len(42)
}
`
	c := checkFile(t, src)
	expectError(t, c, "len argument must be slice, array, or string")
}

func TestCheckIncDecNonInteger(t *testing.T) {
	src := `package "main"

func main() {
	x := true
	x++
}
`
	c := checkFile(t, src)
	expectError(t, c, "cannot apply")
}

func TestCheckSwitch(t *testing.T) {
	src := `package "main"

func classify(x int) int {
	switch x {
	case 0:
		return 0
	case 1:
		return 1
	default:
		return 2
	}
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckForwardReference(t *testing.T) {
	src := `package "main"

func main() {
	var x int = foo()
	x = x
}

func foo() int {
	return 42
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckMultipleReturns(t *testing.T) {
	src := `package "main"

func divide(a int, b int) (int, bool) {
	if b == 0 {
		return 0, false
	}
	return a / b, true
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckTypeAlias(t *testing.T) {
	src := `package "main"

type MyInt = int

func main() {
	var x MyInt = 42
	var y int = x
	y = y
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckDistinctType(t *testing.T) {
	src := `package "main"

type Duration int

func main() {
	var d Duration = 42
	d = d
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckCompoundAssignment(t *testing.T) {
	src := `package "main"

func main() {
	x := 10
	x += 5
	x -= 3
	x *= 2
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckConst(t *testing.T) {
	src := `package "main"

const maxSize = 100

func main() {
	var x int = maxSize
	x = x
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckConstWithType(t *testing.T) {
	src := `package "main"

const maxSize int = 100

func main() {
	var x int = maxSize
	x = x
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckManagedToRawConversion(t *testing.T) {
	src := `package "main"

func takesRaw(p *int) {
}

func main() {
	var mp @int = box(42)
	takesRaw(mp)
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckNilAssignment(t *testing.T) {
	src := `package "main"

func main() {
	var p *int = nil
	var mp @int = nil
	var s []int = nil
	p = p
	mp = mp
	s = s
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckNilToNonPointer(t *testing.T) {
	src := `package "main"

func main() {
	var x int = nil
}
`
	c := checkFile(t, src)
	expectError(t, c, "cannot assign")
}

func TestCheckWrongArgCount(t *testing.T) {
	src := `package "main"

func add(a int, b int) int {
	return a + b
}

func main() {
	add(1)
}
`
	c := checkFile(t, src)
	expectError(t, c, "wrong number of arguments")
}

func TestCheckWrongReturnCount(t *testing.T) {
	src := `package "main"

func foo() (int, int) {
	return 1
}
`
	c := checkFile(t, src)
	expectError(t, c, "wrong number of return values")
}

func TestCheckStringCharSliceAssignable(t *testing.T) {
	src := `package "main"

func main() {
	var s []char = "hello"
	println(s)
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckCharSliceReturnString(t *testing.T) {
	src := `package "main"

func greet() []char {
	return "hello"
}

func main() {
	println(greet())
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckStringConcatPlus(t *testing.T) {
	src := `package "main"

func main() {
	var a []char = "hello"
	var b []char = " world"
	println(a + b)
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckStringLenAllowed(t *testing.T) {
	src := `package "main"

func main() {
	var n int = len("hello")
	println(n)
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckSelfReferentialStruct(t *testing.T) {
	src := `package "main"

type Node struct {
	val  int
	next @Node
}

func main() {
	var n @Node = make(Node)
	n.val = 42
	n.next = nil
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckForwardRefStruct(t *testing.T) {
	src := `package "main"

type Parent struct {
	child @Child
}

type Child struct {
	name []char
}

func main() {
	var c @Child = make(Child)
	c.name = "test"
	var p @Parent = make(Parent)
	p.child = c
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckDistinctTypeConstGroup(t *testing.T) {
	src := `package "main"

type Color int

const (
	RED   Color = iota
	GREEN
	BLUE
)

func paint(c Color) int {
	return cast(int, c)
}

func main() {
	paint(RED)
	paint(GREEN)
	paint(BLUE)
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}
