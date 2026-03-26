# Binate Bootstrap Interpreter

A tree-walking interpreter for the Binate programming language, written in Go. This is the bootstrap toolchain — its purpose is to run the self-hosted Binate compiler/interpreter, which will then replace it.

## Usage

```
binate [-root dir] <file.bn> [file2.bn ...] [-- args...]
```

Run from your project root (or specify it with `-root`):

```sh
# Single file
go run main.go hello.bn

# Multiple files in the same package
go run main.go math.bn main.bn

# With program arguments
go run main.go cat.bn -- /etc/hosts

# With explicit project root
go run main.go -root myproject myproject/main.bn
```

## Project Structure

```
bootstrap/
  main.go              CLI entry point
  ast/                 AST node types
  token/               Token types and positions
  lexer/               Lexer with automatic semicolon insertion
  parser/              Recursive descent parser
  types/               Type system and type checker
    bootstrap.bni      Embedded interface for pkg/bootstrap
  interpreter/         Tree-walking interpreter
    interpreter.go     Expression/statement evaluation
    value.go           Runtime value types
  loader/              Package discovery and dependency resolution
  testdata/            Test programs
    hello.bn           Hello world
    fib.bn             Fibonacci sequence
    cat.bn             File concatenation (uses pkg/bootstrap)
    wc.bn              Word count (uses pkg/bootstrap)
    multi_*.bn         Multi-file package example
    pkgtest/           Multi-package project example
```

## Language Subset

The bootstrap interpreter supports a subset of Binate sufficient for self-hosting. What's included:

### Types
- `int`, `uint`, `int8`..`int64`, `uint8`..`uint64` (integer types)
- `bool`, `byte` (= uint8), `char` (= uint8)
- `string` (string literal type)
- `*T` (raw pointers), `@T` (managed pointers)
- `[]T` (slices), `[N]T` (arrays)
- Named structs, distinct types, type aliases

### Declarations
```
var x int = 5
x := 5                      // short declaration
const PI int = 3
const ( A int = iota; B; C ) // grouped const with iota

type Point struct { x int; y int }
type Meters int              // distinct type
type MyInt = int             // alias

func add(a int, b int) int { return a + b }
func divmod(a int, b int) (int, int) { return a / b, a % b }
```

### Control Flow
```
if x > 0 { ... } else if x < 0 { ... } else { ... }
for i := 0; i < n; i++ { ... }    // C-style
for n > 0 { ... }                   // while-style
for { ... }                          // infinite
for v in collection { ... }         // iteration
for i, v in collection { ... }      // with index
switch x { case 1: ... default: ... }
break, continue, return
```

### Expressions
- Arithmetic: `+`, `-`, `*`, `/`, `%`
- Bitwise: `&`, `|`, `^`, `~`, `<<`, `>>`
- Comparison: `==`, `!=`, `<`, `>`, `<=`, `>=`
- Logical: `&&`, `||`, `!`
- Assignment: `=`, `+=`, `-=`, `*=`, `/=`, `%=`, `&=`, `|=`, `^=`, `<<=`, `>>=`
- Increment/decrement: `x++`, `x--`
- Pointers: `&x`, `*p`, auto-deref with `.`
- Indexing: `a[i]`, `s[lo:hi]`
- Builtins: `make(T)`, `box(expr)`, `cast(T, v)`, `bit_cast(T, v)`, `len(x)`

### Builtins
- `print(args...)`, `println(args...)` — output to stdout
- `append(slice, elems...)` — append to slice
- `panic(msg)` — runtime error

### Packages

Binate uses a filesystem-based package system:

```
myproject/
  main.bn                    package "main"
  pkg/
    math.bni                 interface file (declarations only)
    math/
      math.bn                implementation
```

Import and use:
```
import "pkg/math"

func main() {
    println(math.abs(-42))
}
```

The `.bni` interface file declares the package's public API (function signatures, constants, types) without bodies. The `.bn` files under the package directory provide implementations. Package declarations must match their filesystem path.

### pkg/bootstrap

The `pkg/bootstrap` package provides OS-level primitives backed by Go in the bootstrap interpreter:

| Function | Signature | Description |
|----------|-----------|-------------|
| `open`   | `(path string, flags int) int` | Open a file, returns fd (-1 on error) |
| `read`   | `(fd int, buf []uint8, n int) int` | Read up to n bytes into buf |
| `write`  | `(fd int, buf []uint8, n int) int` | Write n bytes from buf |
| `close`  | `(fd int) int` | Close a file descriptor |
| `exit`   | `(code int)` | Exit the process |
| `args`   | `() []string` | Program arguments (after `--`) |
| `string` | `(v ...) string` | Convert a value to string |

Constants: `O_RDONLY`, `O_WRONLY`, `O_RDWR`, `O_CREATE`, `O_TRUNC`, `O_APPEND`, `STDIN`, `STDOUT`, `STDERR`.

## Deferred from Bootstrap

These Binate features are not in the bootstrap interpreter:
- Generics (type parameters, constraints, instantiation)
- Interfaces, `impl`, method receivers
- Annotations (`#[...]`)
- Variadic functions (`...T`)
- Closures / function literals
- Float types (`float32`, `float64`)
- `unsafe_index`
- `const` in types (const pointers/slices)
- Function types as values

## Runtime Errors

The interpreter reports errors with source positions:

```
test.bn:5:15: runtime error: division by zero
test.bn:8:12: runtime error: index out of bounds: 5 (len 3)
test.bn:3:10: runtime error: nil pointer dereference
```

## Development

```sh
# Run tests
go test ./...

# Run a specific test package
go test ./interpreter/ -v

# Format code
gofmt -w .

# Lint
go vet ./...
```
