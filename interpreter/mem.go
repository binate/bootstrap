package interpreter

// Raw memory access builtins for the interpreter flat memory model.
// These provide byte-level read/write on malloc'd memory blocks,
// bridging the gap between the bootstrap interpreter's Value model
// and ABI-compatible flat memory.

import (
	"encoding/binary"
	"unsafe"

	"github.com/binate/bootstrap/types"
)

// rawPtrFromValue extracts the raw memory address from a Value.
// Handles PointerVal (with RawAddr), ManagedPtrVal (with RawAddr), and nil.
func rawPtrFromValue(v Value) unsafe.Pointer {
	switch p := v.(type) {
	case *PointerVal:
		return p.RawAddr
	case *ManagedPtrVal:
		return p.RawAddr
	case *NilVal:
		return nil
	}
	return nil
}

// makeRawPtr creates a PointerVal with a raw memory address.
func makeRawPtr(addr unsafe.Pointer) *PointerVal {
	return &PointerVal{
		RawAddr: addr,
		Typ:     &types.PointerType{Elem: types.Typ_uint8},
	}
}

// registerMemBuiltins registers Malloc, Free, Memset, Memcpy, Peek*, Poke*.
func (interp *Interpreter) registerMemBuiltins(pkg *Env) {
	pkg.define("Malloc", &BuiltinFuncVal{
		Name: "Malloc",
		Fn: func(args []Value) Value {
			size := args[0].(*IntVal).Val
			if size <= 0 {
				return &PointerVal{Typ: &types.PointerType{Elem: types.Typ_uint8}}
			}
			buf := make([]byte, size)
			return makeRawPtr(unsafe.Pointer(&buf[0]))
		},
	})

	pkg.define("Free", &BuiltinFuncVal{
		Name: "Free",
		Fn: func(args []Value) Value {
			// Go GC handles deallocation; Free is a no-op.
			return nil
		},
	})

	pkg.define("Memset", &BuiltinFuncVal{
		Name: "Memset",
		Fn: func(args []Value) Value {
			ptr := rawPtrFromValue(args[0])
			val := byte(args[1].(*IntVal).Val)
			size := int(args[2].(*IntVal).Val)
			if ptr == nil || size <= 0 {
				return nil
			}
			buf := unsafe.Slice((*byte)(ptr), size)
			for i := range buf {
				buf[i] = val
			}
			return nil
		},
	})

	pkg.define("Memcpy", &BuiltinFuncVal{
		Name: "Memcpy",
		Fn: func(args []Value) Value {
			dst := rawPtrFromValue(args[0])
			src := rawPtrFromValue(args[1])
			size := int(args[2].(*IntVal).Val)
			if dst == nil || src == nil || size <= 0 {
				return nil
			}
			dstBuf := unsafe.Slice((*byte)(dst), size)
			srcBuf := unsafe.Slice((*byte)(src), size)
			copy(dstBuf, srcBuf)
			return nil
		},
	})

	// PeekI64 / PokeI64
	pkg.define("PeekI64", &BuiltinFuncVal{
		Name: "PeekI64",
		Fn: func(args []Value) Value {
			ptr := rawPtrFromValue(args[0])
			offset := int(args[1].(*IntVal).Val)
			if ptr == nil {
				return &IntVal{Val: 0, Typ: types.Typ_int}
			}
			addr := unsafe.Add(ptr, offset)
			val := *(*int64)(addr)
			return &IntVal{Val: int64(val), Typ: types.Typ_int}
		},
	})

	pkg.define("PokeI64", &BuiltinFuncVal{
		Name: "PokeI64",
		Fn: func(args []Value) Value {
			ptr := rawPtrFromValue(args[0])
			offset := int(args[1].(*IntVal).Val)
			val := args[2].(*IntVal).Val
			if ptr == nil {
				return nil
			}
			addr := unsafe.Add(ptr, offset)
			*(*int64)(addr) = int64(val)
			return nil
		},
	})

	// PeekI32 / PokeI32
	pkg.define("PeekI32", &BuiltinFuncVal{
		Name: "PeekI32",
		Fn: func(args []Value) Value {
			ptr := rawPtrFromValue(args[0])
			offset := int(args[1].(*IntVal).Val)
			if ptr == nil {
				return &IntVal{Val: 0, Typ: types.Typ_int}
			}
			addr := unsafe.Add(ptr, offset)
			buf := unsafe.Slice((*byte)(addr), 4)
			val := binary.LittleEndian.Uint32(buf)
			return &IntVal{Val: int64(val), Typ: types.Typ_int}
		},
	})

	pkg.define("PokeI32", &BuiltinFuncVal{
		Name: "PokeI32",
		Fn: func(args []Value) Value {
			ptr := rawPtrFromValue(args[0])
			offset := int(args[1].(*IntVal).Val)
			val := args[2].(*IntVal).Val
			if ptr == nil {
				return nil
			}
			addr := unsafe.Add(ptr, offset)
			buf := unsafe.Slice((*byte)(addr), 4)
			binary.LittleEndian.PutUint32(buf, uint32(val))
			return nil
		},
	})

	// PeekI16 / PokeI16
	pkg.define("PeekI16", &BuiltinFuncVal{
		Name: "PeekI16",
		Fn: func(args []Value) Value {
			ptr := rawPtrFromValue(args[0])
			offset := int(args[1].(*IntVal).Val)
			if ptr == nil {
				return &IntVal{Val: 0, Typ: types.Typ_int}
			}
			addr := unsafe.Add(ptr, offset)
			buf := unsafe.Slice((*byte)(addr), 2)
			val := binary.LittleEndian.Uint16(buf)
			return &IntVal{Val: int64(val), Typ: types.Typ_int}
		},
	})

	pkg.define("PokeI16", &BuiltinFuncVal{
		Name: "PokeI16",
		Fn: func(args []Value) Value {
			ptr := rawPtrFromValue(args[0])
			offset := int(args[1].(*IntVal).Val)
			val := args[2].(*IntVal).Val
			if ptr == nil {
				return nil
			}
			addr := unsafe.Add(ptr, offset)
			buf := unsafe.Slice((*byte)(addr), 2)
			binary.LittleEndian.PutUint16(buf, uint16(val))
			return nil
		},
	})

	// PeekI8 / PokeI8
	pkg.define("PeekI8", &BuiltinFuncVal{
		Name: "PeekI8",
		Fn: func(args []Value) Value {
			ptr := rawPtrFromValue(args[0])
			offset := int(args[1].(*IntVal).Val)
			if ptr == nil {
				return &IntVal{Val: 0, Typ: types.Typ_int}
			}
			addr := unsafe.Add(ptr, offset)
			val := *(*byte)(addr)
			return &IntVal{Val: int64(val), Typ: types.Typ_int}
		},
	})

	pkg.define("PokeI8", &BuiltinFuncVal{
		Name: "PokeI8",
		Fn: func(args []Value) Value {
			ptr := rawPtrFromValue(args[0])
			offset := int(args[1].(*IntVal).Val)
			val := args[2].(*IntVal).Val
			if ptr == nil {
				return nil
			}
			addr := unsafe.Add(ptr, offset)
			*(*byte)(addr) = byte(val)
			return nil
		},
	})

	// PeekPtr / PokePtr
	pkg.define("PeekPtr", &BuiltinFuncVal{
		Name: "PeekPtr",
		Fn: func(args []Value) Value {
			ptr := rawPtrFromValue(args[0])
			offset := int(args[1].(*IntVal).Val)
			if ptr == nil {
				return makeRawPtr(nil)
			}
			addr := unsafe.Add(ptr, offset)
			val := *(*unsafe.Pointer)(addr)
			return makeRawPtr(val)
		},
	})

	pkg.define("PokePtr", &BuiltinFuncVal{
		Name: "PokePtr",
		Fn: func(args []Value) Value {
			ptr := rawPtrFromValue(args[0])
			offset := int(args[1].(*IntVal).Val)
			valPtr := rawPtrFromValue(args[2])
			if ptr == nil {
				return nil
			}
			addr := unsafe.Add(ptr, offset)
			*(*unsafe.Pointer)(addr) = valPtr
			return nil
		},
	})
}
