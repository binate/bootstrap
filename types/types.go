// Package types defines the type representations and type checker
// for the Binate bootstrap interpreter.
package types

import "fmt"

// Type is the interface implemented by all Binate types.
type Type interface {
	String() string
	// Underlying returns the underlying type, stripping named/alias wrappers.
	Underlying() Type
	typeNode()
}

// ============================================================
// Primitive Types
// ============================================================

// IntType represents an integer type.
type IntType struct {
	Width  int  // bit width: 8, 16, 32, 64
	Signed bool // true for int, int8, etc.; false for uint, uint8, etc.
	Name   string
}

func (t *IntType) String() string   { return t.Name }
func (t *IntType) Underlying() Type { return t }
func (t *IntType) typeNode()        {}

// BoolType represents the bool type.
type BoolType struct{}

func (t *BoolType) String() string   { return "bool" }
func (t *BoolType) Underlying() Type { return t }
func (t *BoolType) typeNode()        {}

// CharType represents the char type (32-bit Unicode code point).
type CharType struct{}

func (t *CharType) String() string   { return "char" }
func (t *CharType) Underlying() Type { return t }
func (t *CharType) typeNode()        {}

// StringLitType is the type of string literals (internally []byte).
type StringLitType struct{}

func (t *StringLitType) String() string   { return "string" }
func (t *StringLitType) Underlying() Type { return t }
func (t *StringLitType) typeNode()        {}

// VoidType represents no type (used for functions with no return).
type VoidType struct{}

func (t *VoidType) String() string   { return "void" }
func (t *VoidType) Underlying() Type { return t }
func (t *VoidType) typeNode()        {}

// NilType is the type of the nil literal.
type NilType struct{}

func (t *NilType) String() string   { return "nil" }
func (t *NilType) Underlying() Type { return t }
func (t *NilType) typeNode()        {}

// UntypedIntType is the type of untyped integer constants.
type UntypedIntType struct{}

func (t *UntypedIntType) String() string   { return "untyped int" }
func (t *UntypedIntType) Underlying() Type { return t }
func (t *UntypedIntType) typeNode()        {}

// UntypedBoolType is the type of untyped bool constants.
type UntypedBoolType struct{}

func (t *UntypedBoolType) String() string   { return "untyped bool" }
func (t *UntypedBoolType) Underlying() Type { return t }
func (t *UntypedBoolType) typeNode()        {}

// ============================================================
// Composite Types
// ============================================================

// PointerType represents a raw pointer *T.
type PointerType struct {
	Elem Type
}

func (t *PointerType) String() string   { return "*" + t.Elem.String() }
func (t *PointerType) Underlying() Type { return t }
func (t *PointerType) typeNode()        {}

// ManagedPtrType represents a managed pointer @T.
type ManagedPtrType struct {
	Elem Type
}

func (t *ManagedPtrType) String() string   { return "@" + t.Elem.String() }
func (t *ManagedPtrType) Underlying() Type { return t }
func (t *ManagedPtrType) typeNode()        {}

// SliceType represents a raw slice []T.
type SliceType struct {
	Elem Type
}

func (t *SliceType) String() string   { return "[]" + t.Elem.String() }
func (t *SliceType) Underlying() Type { return t }
func (t *SliceType) typeNode()        {}

// ManagedSliceType represents a managed slice @[]T.
type ManagedSliceType struct {
	Elem Type
}

func (t *ManagedSliceType) String() string   { return "@[]" + t.Elem.String() }
func (t *ManagedSliceType) Underlying() Type { return t }
func (t *ManagedSliceType) typeNode()        {}

// ArrayType represents a fixed-size array [N]T.
type ArrayType struct {
	Len  int64
	Elem Type
}

func (t *ArrayType) String() string   { return fmt.Sprintf("[%d]%s", t.Len, t.Elem) }
func (t *ArrayType) Underlying() Type { return t }
func (t *ArrayType) typeNode()        {}

// StructType represents a struct type.
type StructType struct {
	Name   string // empty for anonymous structs
	Fields []*Field
}

type Field struct {
	Name string
	Type Type
}

func (t *StructType) String() string {
	if t.Name != "" {
		return t.Name
	}
	return "struct{...}"
}
func (t *StructType) Underlying() Type { return t }
func (t *StructType) typeNode()        {}

// FieldByName returns the field with the given name, or nil.
func (t *StructType) FieldByName(name string) *Field {
	for _, f := range t.Fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// FuncType represents a function signature.
type FuncType struct {
	Params  []*Param
	Results []Type
}

type Param struct {
	Name string
	Type Type
}

func (t *FuncType) String() string {
	s := "func("
	for i, p := range t.Params {
		if i > 0 {
			s += ", "
		}
		s += p.Type.String()
	}
	s += ")"
	if len(t.Results) == 1 {
		s += " " + t.Results[0].String()
	} else if len(t.Results) > 1 {
		s += " ("
		for i, r := range t.Results {
			if i > 0 {
				s += ", "
			}
			s += r.String()
		}
		s += ")"
	}
	return s
}
func (t *FuncType) Underlying() Type { return t }
func (t *FuncType) typeNode()        {}

// ============================================================
// Named and Alias Types
// ============================================================

// NamedType represents a distinct named type (type Duration int64).
type NamedType struct {
	Name        string
	Underlying_ Type // the underlying type
}

func (t *NamedType) String() string   { return t.Name }
func (t *NamedType) Underlying() Type { return t.Underlying_ }
func (t *NamedType) typeNode()        {}

// AliasType represents a type alias (type MyInt = int).
type AliasType struct {
	Name   string
	Target Type
}

func (t *AliasType) String() string   { return t.Name }
func (t *AliasType) Underlying() Type { return t.Target.Underlying() }
func (t *AliasType) typeNode()        {}

// ============================================================
// Predeclared Types (singletons)
// ============================================================

var (
	Typ_bool = &BoolType{}
	Typ_char = &CharType{}

	Typ_int   = &IntType{Width: 64, Signed: true, Name: "int"}
	Typ_int8  = &IntType{Width: 8, Signed: true, Name: "int8"}
	Typ_int16 = &IntType{Width: 16, Signed: true, Name: "int16"}
	Typ_int32 = &IntType{Width: 32, Signed: true, Name: "int32"}
	Typ_int64 = &IntType{Width: 64, Signed: true, Name: "int64"}

	Typ_uint   = &IntType{Width: 64, Signed: false, Name: "uint"}
	Typ_uint8  = &IntType{Width: 8, Signed: false, Name: "uint8"}
	Typ_uint16 = &IntType{Width: 16, Signed: false, Name: "uint16"}
	Typ_uint32 = &IntType{Width: 32, Signed: false, Name: "uint32"}
	Typ_uint64 = &IntType{Width: 64, Signed: false, Name: "uint64"}

	Typ_byte = Typ_uint8 // byte is an alias for uint8

	Typ_void        = &VoidType{}
	Typ_nil         = &NilType{}
	Typ_untypedInt  = &UntypedIntType{}
	Typ_untypedBool = &UntypedBoolType{}
	Typ_string      = &StringLitType{}
)

// PredeclaredTypes maps predeclared type names to their types.
var PredeclaredTypes = map[string]Type{
	"bool":   Typ_bool,
	"char":   Typ_char,
	"int":    Typ_int,
	"int8":   Typ_int8,
	"int16":  Typ_int16,
	"int32":  Typ_int32,
	"int64":  Typ_int64,
	"uint":   Typ_uint,
	"uint8":  Typ_uint8,
	"uint16": Typ_uint16,
	"uint32": Typ_uint32,
	"uint64": Typ_uint64,
	"byte":   Typ_byte,
	"any":    nil, // placeholder — not a real type in bootstrap
}

// ============================================================
// Type Utilities
// ============================================================

// Identical reports whether two types are identical.
func Identical(a, b Type) bool {
	if a == b {
		return true
	}
	// Resolve aliases
	a = ResolveAlias(a)
	b = ResolveAlias(b)
	if a == b {
		return true
	}

	switch at := a.(type) {
	case *PointerType:
		if bt, ok := b.(*PointerType); ok {
			return Identical(at.Elem, bt.Elem)
		}
	case *ManagedPtrType:
		if bt, ok := b.(*ManagedPtrType); ok {
			return Identical(at.Elem, bt.Elem)
		}
	case *SliceType:
		if bt, ok := b.(*SliceType); ok {
			return Identical(at.Elem, bt.Elem)
		}
	case *ManagedSliceType:
		if bt, ok := b.(*ManagedSliceType); ok {
			return Identical(at.Elem, bt.Elem)
		}
	case *ArrayType:
		if bt, ok := b.(*ArrayType); ok {
			return at.Len == bt.Len && Identical(at.Elem, bt.Elem)
		}
	case *FuncType:
		if bt, ok := b.(*FuncType); ok {
			if len(at.Params) != len(bt.Params) || len(at.Results) != len(bt.Results) {
				return false
			}
			for i := range at.Params {
				if !Identical(at.Params[i].Type, bt.Params[i].Type) {
					return false
				}
			}
			for i := range at.Results {
				if !Identical(at.Results[i], bt.Results[i]) {
					return false
				}
			}
			return true
		}
	}
	return false
}

// ResolveAlias strips alias wrappers.
func ResolveAlias(t Type) Type {
	for {
		if a, ok := t.(*AliasType); ok {
			t = a.Target
		} else {
			return t
		}
	}
}

// IsInteger reports whether t is an integer type.
func IsInteger(t Type) bool {
	t = ResolveAlias(t)
	switch t.(type) {
	case *IntType, *UntypedIntType:
		return true
	}
	if nt, ok := t.(*NamedType); ok {
		_, ok := nt.Underlying_.(*IntType)
		return ok
	}
	return false
}

// IsNumeric reports whether t is a numeric type (integer in bootstrap).
func IsNumeric(t Type) bool {
	return IsInteger(t)
}

// IsBool reports whether t is a boolean type.
func IsBool(t Type) bool {
	t = ResolveAlias(t)
	switch t.(type) {
	case *BoolType, *UntypedBoolType:
		return true
	}
	return false
}

// IsPointer reports whether t is a pointer type (raw or managed).
func IsPointer(t Type) bool {
	t = ResolveAlias(t)
	switch t.(type) {
	case *PointerType, *ManagedPtrType:
		return true
	}
	return false
}

// IsSlice reports whether t is a slice type (raw or managed).
func IsSlice(t Type) bool {
	t = ResolveAlias(t)
	switch t.(type) {
	case *SliceType, *ManagedSliceType:
		return true
	}
	return false
}

// IsNillable reports whether t can be assigned nil.
func IsNillable(t Type) bool {
	return IsPointer(t) || IsSlice(t)
}

// AssignableTo reports whether a value of type src can be assigned to a
// variable of type dst.
func AssignableTo(src, dst Type) bool {
	if Identical(src, dst) {
		return true
	}

	// Untyped constants are assignable to their concrete counterparts.
	if _, ok := src.(*UntypedIntType); ok {
		return IsInteger(dst)
	}
	if _, ok := src.(*UntypedBoolType); ok {
		return IsBool(dst)
	}

	// nil is assignable to nillable types.
	if _, ok := src.(*NilType); ok {
		return IsNillable(dst)
	}

	// @T is implicitly convertible to *T (managed → raw pointer).
	if mp, ok := src.(*ManagedPtrType); ok {
		if rp, ok := dst.(*PointerType); ok {
			return Identical(mp.Elem, rp.Elem)
		}
	}

	// @[]T is implicitly convertible to []T (managed → raw slice).
	if ms, ok := src.(*ManagedSliceType); ok {
		if rs, ok := dst.(*SliceType); ok {
			return Identical(ms.Elem, rs.Elem)
		}
	}

	return false
}

// SliceElem returns the element type of a slice or managed slice, or nil.
func SliceElem(t Type) Type {
	t = ResolveAlias(t)
	switch st := t.(type) {
	case *SliceType:
		return st.Elem
	case *ManagedSliceType:
		return st.Elem
	}
	return nil
}

// PointerElem returns the element type of a pointer (raw or managed), or nil.
func PointerElem(t Type) Type {
	t = ResolveAlias(t)
	switch pt := t.(type) {
	case *PointerType:
		return pt.Elem
	case *ManagedPtrType:
		return pt.Elem
	}
	return nil
}
