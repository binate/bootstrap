// Package interpreter implements the Binate bootstrap tree-walking interpreter.
package interpreter

import (
	"fmt"
	"strings"

	"github.com/binate/bootstrap/ast"
	"github.com/binate/bootstrap/types"
)

// Value represents a runtime value.
type Value interface {
	Type() types.Type
	String() string
}

// copyValue returns a shallow copy of struct and array values.
// Other value types (int, bool, string, slice, pointer) are left as-is.
func copyValue(v Value) Value {
	switch val := v.(type) {
	case *StructVal:
		return val.Copy()
	case *ArrayVal:
		return val.Copy()
	default:
		return v
	}
}

// IntVal represents an integer value.
type IntVal struct {
	Val int64
	Typ *types.IntType
}

func (v *IntVal) Type() types.Type { return v.Typ }
func (v *IntVal) String() string   { return fmt.Sprintf("%d", v.Val) }

// BoolVal represents a boolean value.
type BoolVal struct {
	Val bool
}

func (v *BoolVal) Type() types.Type { return types.Typ_bool }
func (v *BoolVal) String() string   { return fmt.Sprintf("%t", v.Val) }

// StringVal represents a string literal value ([]byte internally).
type StringVal struct {
	Val string // unescaped string content (without quotes)
}

func (v *StringVal) Type() types.Type { return types.Typ_string }
func (v *StringVal) String() string   { return v.Val }

// NilVal represents the nil value.
type NilVal struct{}

func (v *NilVal) Type() types.Type { return types.Typ_nil }
func (v *NilVal) String() string   { return "nil" }

// PointerVal represents a raw pointer value.
type PointerVal struct {
	Addr *HeapObject
	Typ  types.Type // the pointer type (*T)
}

func (v *PointerVal) Type() types.Type { return v.Typ }
func (v *PointerVal) String() string {
	if v.Addr == nil {
		return "nil"
	}
	return fmt.Sprintf("&%s", v.Addr.Val)
}

// ManagedPtrVal represents a managed pointer value (@T).
type ManagedPtrVal struct {
	Addr *HeapObject
	Typ  types.Type // the managed pointer type (@T)
}

func (v *ManagedPtrVal) Type() types.Type { return v.Typ }
func (v *ManagedPtrVal) String() string {
	if v.Addr == nil {
		return "nil"
	}
	return fmt.Sprintf("@&%s", v.Addr.Val)
}

// SliceVal represents a slice value (raw or managed).
type SliceVal struct {
	Elems []Value
	Typ   types.Type // the slice type
}

func (v *SliceVal) Type() types.Type { return v.Typ }
func (v *SliceVal) String() string {
	// []char ([]uint8) slices are printed as strings
	var elem types.Type
	if st, ok := v.Typ.(*types.SliceType); ok {
		elem = st.Elem
	} else if st, ok := v.Typ.(*types.ManagedSliceType); ok {
		elem = st.Elem
	}
	if elem != nil && (types.Identical(elem, types.Typ_char) || types.Identical(elem, types.Typ_uint8)) {
		buf := make([]byte, len(v.Elems))
		for i, e := range v.Elems {
			if iv, ok := e.(*IntVal); ok {
				buf[i] = byte(iv.Val)
			}
		}
		return string(buf)
	}
	parts := make([]string, len(v.Elems))
	for i, e := range v.Elems {
		parts[i] = e.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ArrayVal represents a fixed-size array value.
type ArrayVal struct {
	Elems []Value
	Typ   *types.ArrayType
}

func (v *ArrayVal) Type() types.Type { return v.Typ }

// Copy returns a deep copy of the array (recursively copies nested structs/arrays).
func (v *ArrayVal) Copy() *ArrayVal {
	elems := make([]Value, len(v.Elems))
	for i, e := range v.Elems {
		elems[i] = copyValue(e)
	}
	return &ArrayVal{Elems: elems, Typ: v.Typ}
}
func (v *ArrayVal) String() string {
	parts := make([]string, len(v.Elems))
	for i, e := range v.Elems {
		parts[i] = e.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// StructVal represents a struct value.
type StructVal struct {
	Fields []Value
	Typ    *types.StructType
}

func (v *StructVal) Type() types.Type { return v.Typ }

// Copy returns a deep copy of the struct (recursively copies nested structs/arrays).
func (v *StructVal) Copy() *StructVal {
	fields := make([]Value, len(v.Fields))
	for i, f := range v.Fields {
		fields[i] = copyValue(f)
	}
	return &StructVal{Fields: fields, Typ: v.Typ}
}
func (v *StructVal) String() string {
	parts := make([]string, len(v.Fields))
	for i, f := range v.Fields {
		name := v.Typ.Fields[i].Name
		parts[i] = fmt.Sprintf("%s: %s", name, f)
	}
	return v.Typ.String() + "{" + strings.Join(parts, ", ") + "}"
}

// FuncVal represents a function value.
type FuncVal struct {
	Name    string
	Typ     *types.FuncType
	Decl    *ast.FuncDecl         // AST declaration (for calling)
	Env     *Env                  // defining scope (for cross-package calls)
	Types   map[string]types.Type // package type map (for cross-package type resolution)
	Aliases map[string]string     // package import aliases
}

func (v *FuncVal) Type() types.Type { return v.Typ }
func (v *FuncVal) String() string   { return "<func " + v.Name + ">" }

// BuiltinFuncVal represents a built-in function (print, println, etc.).
type BuiltinFuncVal struct {
	Name string
	Fn   func(args []Value) Value
}

func (v *BuiltinFuncVal) Type() types.Type { return types.Typ_void }
func (v *BuiltinFuncVal) String() string   { return "<builtin " + v.Name + ">" }

// HeapObject represents a heap-allocated object for managed pointers.
type HeapObject struct {
	Val      Value
	Refcount int
}

// ============================================================
// Zero Values
// ============================================================

// ZeroValue returns the zero value for a given type.
func ZeroValue(t types.Type) Value {
	t = types.ResolveAlias(t)
	switch t := t.(type) {
	case *types.IntType:
		return &IntVal{Val: 0, Typ: t}
	case *types.BoolType:
		return &BoolVal{Val: false}
	case *types.StringLitType:
		return &StringVal{Val: ""}
	case *types.PointerType:
		return &PointerVal{Addr: nil, Typ: t}
	case *types.ManagedPtrType:
		return &ManagedPtrVal{Addr: nil, Typ: t}
	case *types.SliceType:
		return &SliceVal{Elems: nil, Typ: t}
	case *types.ManagedSliceType:
		return &SliceVal{Elems: nil, Typ: t}
	case *types.ArrayType:
		elems := make([]Value, t.Len)
		for i := range elems {
			elems[i] = ZeroValue(t.Elem)
		}
		return &ArrayVal{Elems: elems, Typ: t}
	case *types.StructType:
		fields := make([]Value, len(t.Fields))
		for i, f := range t.Fields {
			fields[i] = ZeroValue(f.Type)
		}
		return &StructVal{Fields: fields, Typ: t}
	case *types.NamedType:
		return ZeroValue(t.Underlying_)
	case *types.NilType:
		return &NilVal{}
	default:
		return &NilVal{}
	}
}

// MultiVal holds multiple return values from a function call.
type MultiVal struct {
	Vals []Value
}

func (v *MultiVal) Type() types.Type { return types.Typ_void }
func (v *MultiVal) String() string {
	parts := make([]string, len(v.Vals))
	for i, val := range v.Vals {
		parts[i] = val.String()
	}
	return "(" + strings.Join(parts, ", ") + ")"
}
