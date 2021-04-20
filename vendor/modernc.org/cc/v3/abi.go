// Copyright 2019 The CC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cc // import "modernc.org/cc/v3"

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"runtime"
)

var (
	complexTypedefs = map[StringID]Kind{
		dict.sid("__COMPLEX_CHAR_TYPE__"):               ComplexChar,
		dict.sid("__COMPLEX_DOUBLE_TYPE__"):             ComplexDouble,
		dict.sid("__COMPLEX_FLOAT_TYPE__"):              ComplexFloat,
		dict.sid("__COMPLEX_INT_TYPE__"):                ComplexInt,
		dict.sid("__COMPLEX_LONG_TYPE__"):               ComplexLong,
		dict.sid("__COMPLEX_LONG_DOUBLE_TYPE__"):        ComplexLongDouble,
		dict.sid("__COMPLEX_LONG_LONG_TYPE__"):          ComplexLongLong,
		dict.sid("__COMPLEX_SHORT_TYPE__"):              ComplexShort,
		dict.sid("__COMPLEX_UNSIGNED_TYPE__"):           ComplexUInt,
		dict.sid("__COMPLEX_LONG_UNSIGNED_TYPE__"):      ComplexULong,
		dict.sid("__COMPLEX_LONG_LONG_UNSIGNED_TYPE__"): ComplexULongLong,
		dict.sid("__COMPLEX_SHORT_UNSIGNED_TYPE__"):     ComplexUShort,
	}
)

// NewABI creates an ABI for a given OS and architecture. The OS and architecture values are the same as used in Go.
// The ABI type map may miss advanced types like complex numbers, etc. If the os/arch pair is not recognized, a
// *ErrUnsupportedOSArch is returned.
func NewABI(os, arch string) (ABI, error) {
	order, ok := abiByteOrders[arch]
	if !ok {
		return ABI{}, fmt.Errorf("unsupported arch: %s", arch)
	}
	types, ok := abiTypes[[2]string{os, arch}]
	if !ok {
		return ABI{}, fmt.Errorf("unsupported os/arch pair: %s-%s", os, arch)
	}
	abi := ABI{
		ByteOrder: order,
		Types:     make(map[Kind]ABIType, len(types)),
		//TODO: depends on the OS?
		SignedChar: true,
	}
	// copy the map, so it can be modified by user
	for k, v := range types {
		abi.Types[k] = v
	}
	return abi, nil
}

// NewABIFromEnv uses GOOS and GOARCH values to create a corresponding ABI.
// If those environment variables are not set, an OS/arch of a Go runtime is used.
// It returns a *ErrUnsupportedOSArch if OS/arch pair is not supported.
func NewABIFromEnv() (ABI, error) {
	osv := os.Getenv("GOOS")
	if osv == "" {
		osv = runtime.GOOS
	}
	arch := os.Getenv("GOARCH")
	if arch == "" {
		arch = runtime.GOARCH
	}
	return NewABI(osv, arch)
}

// ABIType describes properties of a non-aggregate type.
type ABIType struct {
	Size       uintptr
	Align      int
	FieldAlign int
}

// ABI describes selected parts of the Application Binary Interface.
type ABI struct {
	ByteOrder binary.ByteOrder
	Types     map[Kind]ABIType
	types     map[Kind]Type

	SignedChar bool
}

func (a *ABI) sanityCheck(ctx *context, intMaxWidth int, s Scope) error {
	if intMaxWidth == 0 {
		intMaxWidth = 64
	}

	a.types = map[Kind]Type{}
	for _, k := range []Kind{
		Bool,
		Char,
		Double,
		Enum,
		Float,
		Int,
		Long,
		LongDouble,
		LongLong,
		Ptr,
		SChar,
		Short,
		UChar,
		UInt,
		ULong,
		ULongLong,
		UShort,
		Void,
	} {
		v, ok := a.Types[k]
		if !ok {
			if ctx.err(noPos, "ABI is missing %s", k) {
				return ctx.Err()
			}

			continue
		}

		if (k != Void && v.Size == 0) || v.Align == 0 || v.FieldAlign == 0 ||
			v.Align > math.MaxUint8 || v.FieldAlign > math.MaxUint8 {
			if ctx.err(noPos, "invalid ABI type %s: %+v", k, v) {
				return ctx.Err()
			}
		}

		if integerTypes[k] && v.Size > 8 {
			if ctx.err(noPos, "invalid ABI type %s size: %v, must be <= 8", k, v.Size) {
				return ctx.Err()
			}
		}
		var f flag
		if integerTypes[k] && a.isSignedInteger(k) {
			f = fSigned
		}
		t := &typeBase{
			align:      byte(a.align(k)),
			fieldAlign: byte(a.fieldAlign(k)),
			flags:      f,
			kind:       byte(k),
			size:       uintptr(a.size(k)),
		}
		a.types[k] = t
	}
	if _, ok := a.Types[Int128]; ok {
		t := &typeBase{
			align:      byte(a.align(Int128)),
			fieldAlign: byte(a.fieldAlign(Int128)),
			flags:      fSigned,
			kind:       byte(Int128),
			size:       uintptr(a.size(Int128)),
		}
		a.types[Int128] = t
	}
	if _, ok := a.Types[UInt128]; ok {
		t := &typeBase{
			align:      byte(a.align(UInt128)),
			fieldAlign: byte(a.fieldAlign(UInt128)),
			kind:       byte(UInt128),
			size:       uintptr(a.size(UInt128)),
		}
		a.types[UInt128] = t
	}
	return ctx.Err()
}

func (a *ABI) Type(k Kind) Type { return a.types[k] }

func (a *ABI) align(k Kind) int      { return a.Types[k].Align }
func (a *ABI) fieldAlign(k Kind) int { return a.Types[k].FieldAlign }
func (a *ABI) size(k Kind) int       { return int(a.Types[k].Size) }

func (a *ABI) isSignedInteger(k Kind) bool {
	if !integerTypes[k] {
		internalError()
	}

	switch k {
	case Bool, UChar, UInt, ULong, ULongLong, UShort:
		return false
	case Char:
		return a.SignedChar
	default:
		return true
	}
}

func roundup(n, to int64) int64 {
	if r := n % to; r != 0 {
		return n + to - r
	}

	return n
}

func normalizeBitFieldWidth(n byte) byte {
	switch {
	case n <= 8:
		return 8
	case n <= 16:
		return 16
	case n <= 32:
		return 32
	case n <= 64:
		return 64
	default:
		panic(todo("internal error: %v", n))
	}
}

func (a *ABI) layout(ctx *context, n Node, t *structType) *structType {
	if t == nil {
		return nil
	}

	var hasBitfields bool

	defer func() {
		if !hasBitfields {
			return
		}

		m := make(map[uintptr][]*field, len(t.fields))
		for _, f := range t.fields {
			off := f.offset
			m[off] = append(m[off], f)
		}
		for _, a := range m {
			var first *field
			var w byte
			for _, f := range a {
				if first == nil {
					first = f
				}
				if f.isBitField {
					n := f.bitFieldOffset + f.bitFieldWidth
					if n > w {
						w = n
					}
				}
			}
			w = normalizeBitFieldWidth(w)
			for _, f := range a {
				if f.isBitField {
					f.blockStart = first
					f.blockWidth = w
				}
			}
		}
		// trc("", t)
		// for _, v := range t.fields {
		// 	trc("%+v", v)
		// }
	}()

	var off int64 // bit offset
	align := 1

	switch {
	case t.Kind() == Union:
		for _, f := range t.fields {
			ft := f.Type()
			sz := ft.Size()
			if n := int64(8 * sz); n > off {
				off = n
			}
			al := ft.FieldAlign()
			if al == 0 {
				al = 1
			}
			if al > align {
				align = al
			}

			if f.isBitField {
				hasBitfields = true
				f.bitFieldMask = 1<<f.bitFieldWidth - 1
			}
			f.promote = integerPromotion(a, ft)
		}
		t.align = byte(align)
		t.fieldAlign = byte(align)
		off = roundup(off, 8*int64(align))
		t.size = uintptr(off >> 3)
		ctx.structs[StructInfo{Size: t.size, Align: t.Align()}] = struct{}{}
	default:
		var i int
		var group byte
		var f, lf *field
		for i, f = range t.fields {
			ft := f.Type()
			var sz uintptr
			switch {
			case ft.Kind() == Array && i == len(t.fields)-1:
				if ft.IsIncomplete() || ft.Len() == 0 {
					t.hasFlexibleMember = true
					f.isFlexible = true
					break
				}

				fallthrough
			default:
				sz = ft.Size()
			}

			bitSize := 8 * int(sz)
			al := ft.FieldAlign()
			if al == 0 {
				al = 1
			}
			if al > align {
				align = al
			}

			switch {
			case f.isBitField:
				hasBitfields = true
				eal := 8 * al
				if eal < bitSize {
					eal = bitSize
				}
				down := off &^ (int64(eal) - 1)
				bitoff := off - down
				downMax := off &^ (int64(bitSize) - 1)
				skip := lf != nil && lf.isBitField && lf.bitFieldWidth == 0 ||
					lf != nil && lf.bitFieldWidth == 0 && ctx.cfg.NoFieldAndBitfieldOverlap
				switch {
				case skip || int(off-downMax)+int(f.bitFieldWidth) > bitSize:
					group = 0
					off = roundup(off, 8*int64(al))
					f.offset = uintptr(off >> 3)
					f.bitFieldOffset = 0
					f.bitFieldMask = 1<<f.bitFieldWidth - 1
					off += int64(f.bitFieldWidth)
					if f.bitFieldWidth == 0 {
						lf = f
						continue
					}
				default:
					f.offset = uintptr(down >> 3)
					f.bitFieldOffset = byte(bitoff)
					f.bitFieldMask = (1<<f.bitFieldWidth - 1) << byte(bitoff)
					off += int64(f.bitFieldWidth)
				}
				group += f.bitFieldWidth
			default:
				if group != 0 {
					group %= 64
					off += int64(normalizeBitFieldWidth(group) - group)
				}
				off0 := off
				off = roundup(off, 8*int64(al))
				f.pad = byte(off-off0) >> 3
				f.offset = uintptr(off) >> 3
				off += 8 * int64(sz)
				group = 0
			}
			f.promote = integerPromotion(a, ft)
			lf = f
		}
		t.align = byte(align)
		t.fieldAlign = byte(align)
		off0 := off
		off = roundup(off, 8*int64(align))
		if f != nil && !f.IsBitField() {
			f.pad = byte(off-off0) >> 3
		}
		t.size = uintptr(off >> 3)
		ctx.structs[StructInfo{Size: t.size, Align: t.Align()}] = struct{}{}
	}
	return t
}

func (a *ABI) Ptr(n Node, t Type) Type {
	base := t.base()
	base.align = byte(a.align(Ptr))
	base.fieldAlign = byte(a.fieldAlign(Ptr))
	base.kind = byte(Ptr)
	base.size = uintptr(a.size(Ptr))
	base.flags &^= fIncomplete
	return &pointerType{
		elem:     t,
		typeBase: base,
	}
}
