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
	var s *[]int
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
	p = p
	mp = mp
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
	var s *[]char = "hello"
	println(s)
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckCharSliceReturnString(t *testing.T) {
	src := `package "main"

func greet() *[]char {
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
	var a *[]char = "hello"
	var b *[]char = " world"
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
	name *[]char
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

func TestCheckDuplicateFunc(t *testing.T) {
	src := `package "main"

func foo() int { return 1 }
func foo() int { return 2 }
`
	c := checkFile(t, src)
	expectError(t, c, "redeclared")
}

func TestCheckDuplicateFuncNotTriggeredByBni(t *testing.T) {
	// A function declared in .bni and defined in .bn should not trigger redeclaration
	bniSrc := `package "pkg/test"
func Foo() int
`
	bnSrc := `package "pkg/test"
func Foo() int { return 42 }
`
	c := NewChecker()
	bniP := parser.NewInterface([]byte(bniSrc), "test.bni")
	bniF := bniP.ParseFile()
	if len(bniP.Errors()) > 0 {
		t.Fatalf("bni parse error: %s", bniP.Errors()[0])
	}
	c.LoadPackageInterface("pkg/test", bniF)

	bnP := parser.New([]byte(bnSrc), "test.bn")
	bnF := bnP.ParseFile()
	if len(bnP.Errors()) > 0 {
		t.Fatalf("bn parse error: %s", bnP.Errors()[0])
	}
	c.CheckPackage("pkg/test", bnF)
	expectNoErrors(t, c)
}

func TestCheckDuplicateConstInGroup(t *testing.T) {
	// Constants in a const() group should not trigger redeclaration
	src := `package "main"

const (
	A int = 1
	B int = 2
	C int = 3
)

func main() {
	var x int = A + B + C
	x = x
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckSliceNilCompareRejected(t *testing.T) {
	src := `package "main"

func main() {
	var s *[]int
	if s == nil {
	}
}
`
	c := checkFile(t, src)
	expectError(t, c, "cannot compare")
}

func TestCheckSliceNilAssignRejected(t *testing.T) {
	src := `package "main"

func main() {
	var s *[]int = nil
}
`
	c := checkFile(t, src)
	expectError(t, c, "cannot assign")
}

func TestCheckManagedSliceNilRejected(t *testing.T) {
	src := `package "main"

func main() {
	var s @[]int = nil
	if s == nil {
	}
}
`
	c := checkFile(t, src)
	expectError(t, c, "cannot")
}

func TestCheckPointerNilStillAllowed(t *testing.T) {
	src := `package "main"

func main() {
	var p *int = nil
	if p == nil {
	}
	var mp @int = nil
	if mp == nil {
	}
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestIdenticalAnonymousStructs(t *testing.T) {
	a := &StructType{Fields: []*Field{
		{Name: "X", Type: Typ_int},
		{Name: "Y", Type: Typ_int},
	}}
	b := &StructType{Fields: []*Field{
		{Name: "X", Type: Typ_int},
		{Name: "Y", Type: Typ_int},
	}}
	if !Identical(a, b) {
		t.Error("anonymous structs with same fields should be identical")
	}
	c := &StructType{Fields: []*Field{
		{Name: "A", Type: Typ_int},
		{Name: "B", Type: Typ_int},
	}}
	if Identical(a, c) {
		t.Error("different field names should not be identical")
	}
	d := &StructType{Fields: []*Field{
		{Name: "X", Type: Typ_int},
		{Name: "Y", Type: Typ_bool},
	}}
	if Identical(a, d) {
		t.Error("different field types should not be identical")
	}
}

func TestAnonymousStructParam(t *testing.T) {
	src := `package "test"
func printPoint(p struct { X int; Y int }) {
	println(p.X)
}
func main() {
	var p struct { X int; Y int }
	p.X = 1
	printPoint(p)
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestAnonymousStructReturn(t *testing.T) {
	src := `package "test"
func makePoint() struct { X int; Y int } {
	var p struct { X int; Y int }
	p.X = 5
	return p
}
func main() {
	var p struct { X int; Y int } = makePoint()
	println(p.X)
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

// ============================================================
// Method declarations (Stage 2 of plan-receivers.md)
// ============================================================

func TestCheckMethodPointerRecv(t *testing.T) {
	src := `package "main"

type Point struct {
	x int
	y int
}

func (p *Point) X() int {
	return p.x
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckMethodValueRecv(t *testing.T) {
	src := `package "main"

type Counter struct { n int }

func (c Counter) Get() int {
	return c.n
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckMethodManagedRecv(t *testing.T) {
	src := `package "main"

type Node struct { val int }

func (n @Node) Val() int {
	return n.val
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckMethodOnNamedPrimitive(t *testing.T) {
	src := `package "main"

type Celsius int

func (c Celsius) Zero() bool {
	return false
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckMethodOnAliasIsError(t *testing.T) {
	src := `package "main"

type MyInt = int

func (m MyInt) Get() int {
	return 0
}
`
	c := checkFile(t, src)
	expectError(t, c, "method receiver")
}

func TestCheckMethodOnBuiltinIsError(t *testing.T) {
	src := `package "main"

func (i int) Foo() int {
	return 0
}
`
	c := checkFile(t, src)
	expectError(t, c, "method receiver")
}

func TestCheckMethodDuplicateIsError(t *testing.T) {
	src := `package "main"

type Point struct { x int }

func (p *Point) X() int {
	return p.x
}

func (p *Point) X() int {
	return 0
}
`
	c := checkFile(t, src)
	expectError(t, c, "redeclared")
}

func TestCheckMethodAndFreeFunctionSameNameOK(t *testing.T) {
	// Methods and free functions live in different namespaces — having a
	// free function `Foo` and a method `Foo` on some type is allowed.
	src := `package "main"

type Point struct { x int }

func (p *Point) Foo() int {
	return p.x
}

func Foo() int {
	return 42
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckMethodSameNameDifferentTypesOK(t *testing.T) {
	// Two different named types can each have a method with the same name.
	src := `package "main"

type A struct { x int }
type B struct { y int }

func (a *A) Get() int {
	return a.x
}

func (b *B) Get() int {
	return b.y
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckMethodRegisteredOnNamedType(t *testing.T) {
	// Use CheckPackage so we can probe the registered package scope after
	// checking, and confirm the method landed on the NamedType (not in
	// the package symbol table).
	src := `package "main"

type Point struct {
	x int
	y int
}

func (p *Point) X() int {
	return p.x
}
`
	p := parser.New([]byte(src), "test.bn")
	f := p.ParseFile()
	c := NewChecker()
	c.CheckPackage("main", f)
	expectNoErrors(t, c)

	pkgScope := c.packages["main"]
	if pkgScope == nil {
		t.Fatalf("package scope not registered")
	}
	if pkgScope.lookupLocal("X") != nil {
		t.Errorf("method X should not be in package scope")
	}
	sym := pkgScope.lookupLocal("Point")
	if sym == nil {
		t.Fatalf("Point not found in package scope")
	}
	nt, ok := sym.Type.(*NamedType)
	if !ok {
		t.Fatalf("Point not a NamedType (got %T)", sym.Type)
	}
	m := nt.LookupMethod("X")
	if m == nil {
		t.Fatalf("method X not registered on Point")
	}
	if len(m.Func.Params) != 1 {
		t.Errorf("expected 1 param (the receiver), got %d", len(m.Func.Params))
	}
	if m.Func.Params[0].Name != "p" {
		t.Errorf("receiver name should be 'p', got %q", m.Func.Params[0].Name)
	}
}

func TestCheckMethodBodySeesReceiver(t *testing.T) {
	// The receiver name must be in scope inside the body.
	src := `package "main"

type Point struct { x int; y int }

func (p *Point) Sum() int {
	return p.x + p.y
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

// ============================================================
// Method calls (Stage 3 of plan-receivers.md)
// ============================================================

func TestCheckMethodCallPointerOnPointer(t *testing.T) {
	src := `package "main"

type Point struct { x int }

func (p *Point) X() int {
	return p.x
}

func main() {
	var pp *Point = nil
	var v int = pp.X()
	println(v)
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

func TestCheckMethodCallValueOnValue(t *testing.T) {
	src := `package "main"

type Counter struct { n int }

func (c Counter) Get() int {
	return c.n
}

func main() {
	var ctr Counter
	var v int = ctr.Get()
	println(v)
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

// @T receiver expression on @T method.
func TestCheckMethodCallManagedOnManaged(t *testing.T) {
	src := `package "main"

type Node struct { val int }

func (n @Node) Val() int {
	return n.val
}

func main() {
	var n @Node = make(Node)
	var v int = n.Val()
	println(v)
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

// @T receiver expression on *T method (managed → raw smoothing).
func TestCheckMethodCallManagedOnPointer(t *testing.T) {
	src := `package "main"

type Node struct { val int }

func (n *Node) Val() int {
	return n.val
}

func main() {
	var n @Node = make(Node)
	var v int = n.Val()
	println(v)
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

// *T receiver expression on T method (auto-deref to value).
func TestCheckMethodCallPointerOnValue(t *testing.T) {
	src := `package "main"

type Counter struct { n int }

func (c Counter) Get() int {
	return c.n
}

func main() {
	var pp *Counter = nil
	var v int = pp.Get()
	println(v)
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}

// Argument count mismatch is reported.
func TestCheckMethodCallWrongArgCount(t *testing.T) {
	src := `package "main"

type Point struct { x int }

func (p *Point) Set(v int) {
	p.x = v
}

func main() {
	var pp *Point = nil
	pp.Set()
}
`
	c := checkFile(t, src)
	expectError(t, c, "wrong number of arguments")
}

// Method call on a type with no such method is an error.
func TestCheckMethodCallUnknownMethod(t *testing.T) {
	src := `package "main"

type Point struct { x int }

func main() {
	var pp *Point = nil
	pp.MissingMethod()
}
`
	c := checkFile(t, src)
	expectError(t, c, "")
}

// *T receiver expression cannot satisfy @T method (no raw → managed).
func TestCheckMethodCallPointerOnManagedRejected(t *testing.T) {
	src := `package "main"

type Node struct { val int }

func (n @Node) Val() int {
	return n.val
}

func main() {
	var pp *Node = nil
	pp.Val()
}
`
	c := checkFile(t, src)
	expectError(t, c, "")
}

// Two methods on different types with the same name — call resolves
// based on the receiver's type.
func TestCheckMethodCallDispatchByType(t *testing.T) {
	src := `package "main"

type A struct { v int }
type B struct { v bool }

func (a *A) Get() int {
	return a.v
}

func (b *B) Get() bool {
	return b.v
}

func main() {
	var pa *A = nil
	var pb *B = nil
	var x int = pa.Get()
	var y bool = pb.Get()
	println(x)
	println(y)
}
`
	c := checkFile(t, src)
	expectNoErrors(t, c)
}
