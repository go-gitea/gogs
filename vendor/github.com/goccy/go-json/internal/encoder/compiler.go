package encoder

import (
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/goccy/go-json/internal/errors"
	"github.com/goccy/go-json/internal/runtime"
)

var (
	marshalJSONType  = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	marshalTextType  = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	jsonNumberType   = reflect.TypeOf(json.Number(""))
	cachedOpcodeSets []*OpcodeSet
	cachedOpcodeMap  unsafe.Pointer // map[uintptr]*OpcodeSet
	typeAddr         *runtime.TypeAddr
)

func init() {
	typeAddr = runtime.AnalyzeTypeAddr()
	if typeAddr == nil {
		typeAddr = &runtime.TypeAddr{}
	}
	cachedOpcodeSets = make([]*OpcodeSet, typeAddr.AddrRange>>typeAddr.AddrShift)
}

func loadOpcodeMap() map[uintptr]*OpcodeSet {
	p := atomic.LoadPointer(&cachedOpcodeMap)
	return *(*map[uintptr]*OpcodeSet)(unsafe.Pointer(&p))
}

func storeOpcodeSet(typ uintptr, set *OpcodeSet, m map[uintptr]*OpcodeSet) {
	newOpcodeMap := make(map[uintptr]*OpcodeSet, len(m)+1)
	newOpcodeMap[typ] = set

	for k, v := range m {
		newOpcodeMap[k] = v
	}

	atomic.StorePointer(&cachedOpcodeMap, *(*unsafe.Pointer)(unsafe.Pointer(&newOpcodeMap)))
}

func compileToGetCodeSetSlowPath(typeptr uintptr) (*OpcodeSet, error) {
	opcodeMap := loadOpcodeMap()
	if codeSet, exists := opcodeMap[typeptr]; exists {
		return codeSet, nil
	}

	// noescape trick for header.typ ( reflect.*rtype )
	copiedType := *(**runtime.Type)(unsafe.Pointer(&typeptr))

	noescapeKeyCode, err := compileHead(&compileContext{
		typ:                      copiedType,
		structTypeToCompiledCode: map[uintptr]*CompiledCode{},
	})
	if err != nil {
		return nil, err
	}
	escapeKeyCode, err := compileHead(&compileContext{
		typ:                      copiedType,
		structTypeToCompiledCode: map[uintptr]*CompiledCode{},
		escapeKey:                true,
	})
	if err != nil {
		return nil, err
	}
	noescapeKeyCode = copyOpcode(noescapeKeyCode)
	escapeKeyCode = copyOpcode(escapeKeyCode)
	codeLength := noescapeKeyCode.TotalLength()
	codeSet := &OpcodeSet{
		Type:            copiedType,
		NoescapeKeyCode: noescapeKeyCode,
		EscapeKeyCode:   escapeKeyCode,
		CodeLength:      codeLength,
	}
	storeOpcodeSet(typeptr, codeSet, opcodeMap)
	return codeSet, nil
}

func compileHead(ctx *compileContext) (*Opcode, error) {
	typ := ctx.typ
	switch {
	case implementsMarshalJSON(typ):
		return compileMarshalJSON(ctx)
	case implementsMarshalText(typ):
		return compileMarshalText(ctx)
	}

	isPtr := false
	orgType := typ
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		isPtr = true
	}
	switch {
	case implementsMarshalJSON(typ):
		return compileMarshalJSON(ctx)
	case implementsMarshalText(typ):
		return compileMarshalText(ctx)
	}
	switch typ.Kind() {
	case reflect.Slice:
		ctx := ctx.withType(typ)
		elem := typ.Elem()
		if elem.Kind() == reflect.Uint8 {
			p := runtime.PtrTo(elem)
			if !p.Implements(marshalJSONType) && !p.Implements(marshalTextType) {
				if isPtr {
					return compileBytesPtr(ctx)
				}
				return compileBytes(ctx)
			}
		}
		code, err := compileSlice(ctx)
		if err != nil {
			return nil, err
		}
		optimizeStructEnd(code)
		linkRecursiveCode(code)
		return code, nil
	case reflect.Map:
		if isPtr {
			return compilePtr(ctx.withType(runtime.PtrTo(typ)))
		}
		code, err := compileMap(ctx.withType(typ))
		if err != nil {
			return nil, err
		}
		optimizeStructEnd(code)
		linkRecursiveCode(code)
		return code, nil
	case reflect.Struct:
		code, err := compileStruct(ctx.withType(typ), isPtr)
		if err != nil {
			return nil, err
		}
		optimizeStructEnd(code)
		linkRecursiveCode(code)
		return code, nil
	case reflect.Int:
		ctx := ctx.withType(typ)
		if isPtr {
			return compileIntPtr(ctx)
		}
		return compileInt(ctx)
	case reflect.Int8:
		ctx := ctx.withType(typ)
		if isPtr {
			return compileInt8Ptr(ctx)
		}
		return compileInt8(ctx)
	case reflect.Int16:
		ctx := ctx.withType(typ)
		if isPtr {
			return compileInt16Ptr(ctx)
		}
		return compileInt16(ctx)
	case reflect.Int32:
		ctx := ctx.withType(typ)
		if isPtr {
			return compileInt32Ptr(ctx)
		}
		return compileInt32(ctx)
	case reflect.Int64:
		ctx := ctx.withType(typ)
		if isPtr {
			return compileInt64Ptr(ctx)
		}
		return compileInt64(ctx)
	case reflect.Uint, reflect.Uintptr:
		ctx := ctx.withType(typ)
		if isPtr {
			return compileUintPtr(ctx)
		}
		return compileUint(ctx)
	case reflect.Uint8:
		ctx := ctx.withType(typ)
		if isPtr {
			return compileUint8Ptr(ctx)
		}
		return compileUint8(ctx)
	case reflect.Uint16:
		ctx := ctx.withType(typ)
		if isPtr {
			return compileUint16Ptr(ctx)
		}
		return compileUint16(ctx)
	case reflect.Uint32:
		ctx := ctx.withType(typ)
		if isPtr {
			return compileUint32Ptr(ctx)
		}
		return compileUint32(ctx)
	case reflect.Uint64:
		ctx := ctx.withType(typ)
		if isPtr {
			return compileUint64Ptr(ctx)
		}
		return compileUint64(ctx)
	case reflect.Float32:
		ctx := ctx.withType(typ)
		if isPtr {
			return compileFloat32Ptr(ctx)
		}
		return compileFloat32(ctx)
	case reflect.Float64:
		ctx := ctx.withType(typ)
		if isPtr {
			return compileFloat64Ptr(ctx)
		}
		return compileFloat64(ctx)
	case reflect.String:
		ctx := ctx.withType(typ)
		if isPtr {
			return compileStringPtr(ctx)
		}
		return compileString(ctx)
	case reflect.Bool:
		ctx := ctx.withType(typ)
		if isPtr {
			return compileBoolPtr(ctx)
		}
		return compileBool(ctx)
	case reflect.Interface:
		ctx := ctx.withType(typ)
		if isPtr {
			return compileInterfacePtr(ctx)
		}
		return compileInterface(ctx)
	default:
		if isPtr && typ.Implements(marshalTextType) {
			typ = orgType
		}
		code, err := compile(ctx.withType(typ), isPtr)
		if err != nil {
			return nil, err
		}
		optimizeStructEnd(code)
		linkRecursiveCode(code)
		return code, nil
	}
}

func linkRecursiveCode(c *Opcode) {
	for code := c; code.Op != OpEnd && code.Op != OpRecursiveEnd; {
		switch code.Op {
		case OpRecursive, OpRecursivePtr:
			if code.Jmp.Linked {
				code = code.Next
				continue
			}
			code.Jmp.Code = copyOpcode(code.Jmp.Code)

			c := code.Jmp.Code
			c.End.Next = newEndOp(&compileContext{})
			c.Op = c.Op.PtrHeadToHead()

			beforeLastCode := c.End
			lastCode := beforeLastCode.Next

			lastCode.Idx = beforeLastCode.Idx + uintptrSize
			lastCode.ElemIdx = lastCode.Idx + uintptrSize
			lastCode.Length = lastCode.Idx + 2*uintptrSize

			// extend length to alloc slot for elemIdx + length
			totalLength := uintptr(code.TotalLength() + 2)
			nextTotalLength := uintptr(c.TotalLength() + 2)

			c.End.Next.Op = OpRecursiveEnd

			code.Jmp.CurLen = totalLength
			code.Jmp.NextLen = nextTotalLength
			code.Jmp.Linked = true

			linkRecursiveCode(code.Jmp.Code)

			code = code.Next
			continue
		}
		switch code.Op.CodeType() {
		case CodeArrayElem, CodeSliceElem, CodeMapKey:
			code = code.End
		default:
			code = code.Next
		}
	}
}

func optimizeStructEnd(c *Opcode) {
	for code := c; code.Op != OpEnd; {
		if code.Op == OpRecursive || code.Op == OpRecursivePtr {
			// ignore if exists recursive operation
			return
		}
		switch code.Op.CodeType() {
		case CodeArrayElem, CodeSliceElem, CodeMapKey:
			code = code.End
		default:
			code = code.Next
		}
	}

	for code := c; code.Op != OpEnd; {
		switch code.Op.CodeType() {
		case CodeArrayElem, CodeSliceElem, CodeMapKey:
			code = code.End
		case CodeStructEnd:
			switch code.Op {
			case OpStructEnd:
				prev := code.PrevField
				prevOp := prev.Op.String()
				if strings.Contains(prevOp, "Head") ||
					strings.Contains(prevOp, "Slice") ||
					strings.Contains(prevOp, "Array") ||
					strings.Contains(prevOp, "Map") ||
					strings.Contains(prevOp, "MarshalJSON") ||
					strings.Contains(prevOp, "MarshalText") {
					// not exists field
					code = code.Next
					break
				}
				if prev.Op != prev.Op.FieldToEnd() {
					prev.Op = prev.Op.FieldToEnd()
					prev.Next = code.Next
				}
				code = code.Next
			default:
				code = code.Next
			}
		default:
			code = code.Next
		}
	}
}

func implementsMarshalJSON(typ *runtime.Type) bool {
	if !typ.Implements(marshalJSONType) {
		return false
	}
	if typ.Kind() != reflect.Ptr {
		return true
	}
	// type kind is reflect.Ptr
	if !typ.Elem().Implements(marshalJSONType) {
		return true
	}
	// needs to dereference
	return false
}

func implementsMarshalText(typ *runtime.Type) bool {
	if !typ.Implements(marshalTextType) {
		return false
	}
	if typ.Kind() != reflect.Ptr {
		return true
	}
	// type kind is reflect.Ptr
	if !typ.Elem().Implements(marshalTextType) {
		return true
	}
	// needs to dereference
	return false
}

func compile(ctx *compileContext, isPtr bool) (*Opcode, error) {
	typ := ctx.typ
	switch {
	case implementsMarshalJSON(typ):
		return compileMarshalJSON(ctx)
	case implementsMarshalText(typ):
		return compileMarshalText(ctx)
	}
	switch typ.Kind() {
	case reflect.Ptr:
		return compilePtr(ctx)
	case reflect.Slice:
		elem := typ.Elem()
		if elem.Kind() == reflect.Uint8 {
			p := runtime.PtrTo(elem)
			if !p.Implements(marshalJSONType) && !p.Implements(marshalTextType) {
				return compileBytes(ctx)
			}
		}
		return compileSlice(ctx)
	case reflect.Array:
		return compileArray(ctx)
	case reflect.Map:
		return compileMap(ctx)
	case reflect.Struct:
		return compileStruct(ctx, isPtr)
	case reflect.Interface:
		return compileInterface(ctx)
	case reflect.Int:
		return compileInt(ctx)
	case reflect.Int8:
		return compileInt8(ctx)
	case reflect.Int16:
		return compileInt16(ctx)
	case reflect.Int32:
		return compileInt32(ctx)
	case reflect.Int64:
		return compileInt64(ctx)
	case reflect.Uint:
		return compileUint(ctx)
	case reflect.Uint8:
		return compileUint8(ctx)
	case reflect.Uint16:
		return compileUint16(ctx)
	case reflect.Uint32:
		return compileUint32(ctx)
	case reflect.Uint64:
		return compileUint64(ctx)
	case reflect.Uintptr:
		return compileUint(ctx)
	case reflect.Float32:
		return compileFloat32(ctx)
	case reflect.Float64:
		return compileFloat64(ctx)
	case reflect.String:
		return compileString(ctx)
	case reflect.Bool:
		return compileBool(ctx)
	}
	return nil, &errors.UnsupportedTypeError{Type: runtime.RType2Type(typ)}
}

func convertPtrOp(code *Opcode) OpType {
	ptrHeadOp := code.Op.HeadToPtrHead()
	if code.Op != ptrHeadOp {
		if code.PtrNum > 0 {
			// ptr field and ptr head
			code.PtrNum--
		}
		return ptrHeadOp
	}
	switch code.Op {
	case OpInt:
		return OpIntPtr
	case OpUint:
		return OpUintPtr
	case OpFloat32:
		return OpFloat32Ptr
	case OpFloat64:
		return OpFloat64Ptr
	case OpString:
		return OpStringPtr
	case OpBool:
		return OpBoolPtr
	case OpBytes:
		return OpBytesPtr
	case OpNumber:
		return OpNumberPtr
	case OpArray:
		return OpArrayPtr
	case OpSlice:
		return OpSlicePtr
	case OpMap:
		return OpMapPtr
	case OpMarshalJSON:
		return OpMarshalJSONPtr
	case OpMarshalText:
		return OpMarshalTextPtr
	case OpInterface:
		return OpInterfacePtr
	case OpRecursive:
		return OpRecursivePtr
	}
	return code.Op
}

func compileKey(ctx *compileContext) (*Opcode, error) {
	typ := ctx.typ
	switch {
	case implementsMarshalJSON(typ):
		return compileMarshalJSON(ctx)
	case implementsMarshalText(typ):
		return compileMarshalText(ctx)
	}
	switch typ.Kind() {
	case reflect.Ptr:
		return compilePtr(ctx)
	case reflect.Interface:
		return compileInterface(ctx)
	case reflect.String:
		return compileString(ctx)
	case reflect.Int:
		return compileIntString(ctx)
	case reflect.Int8:
		return compileInt8String(ctx)
	case reflect.Int16:
		return compileInt16String(ctx)
	case reflect.Int32:
		return compileInt32String(ctx)
	case reflect.Int64:
		return compileInt64String(ctx)
	case reflect.Uint:
		return compileUintString(ctx)
	case reflect.Uint8:
		return compileUint8String(ctx)
	case reflect.Uint16:
		return compileUint16String(ctx)
	case reflect.Uint32:
		return compileUint32String(ctx)
	case reflect.Uint64:
		return compileUint64String(ctx)
	case reflect.Uintptr:
		return compileUintString(ctx)
	}
	return nil, &errors.UnsupportedTypeError{Type: runtime.RType2Type(typ)}
}

func compilePtr(ctx *compileContext) (*Opcode, error) {
	code, err := compile(ctx.withType(ctx.typ.Elem()), true)
	if err != nil {
		return nil, err
	}
	code.Op = convertPtrOp(code)
	code.PtrNum++
	return code, nil
}

func compileMarshalJSON(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpMarshalJSON)
	typ := ctx.typ
	if !typ.Implements(marshalJSONType) && runtime.PtrTo(typ).Implements(marshalJSONType) {
		code.Flags |= AddrForMarshalerFlags
	}
	if isNilableType(typ) {
		code.Flags |= IsNilableTypeFlags
	} else {
		code.Flags &= ^IsNilableTypeFlags
	}
	ctx.incIndex()
	return code, nil
}

func compileMarshalText(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpMarshalText)
	typ := ctx.typ
	if !typ.Implements(marshalTextType) && runtime.PtrTo(typ).Implements(marshalTextType) {
		code.Flags |= AddrForMarshalerFlags
	}
	if isNilableType(typ) {
		code.Flags |= IsNilableTypeFlags
	} else {
		code.Flags &= ^IsNilableTypeFlags
	}
	ctx.incIndex()
	return code, nil
}

const intSize = 32 << (^uint(0) >> 63)

func compileInt(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpInt)
	code.NumBitSize = intSize
	ctx.incIndex()
	return code, nil
}

func compileIntPtr(ctx *compileContext) (*Opcode, error) {
	code, err := compileInt(ctx)
	if err != nil {
		return nil, err
	}
	code.Op = OpIntPtr
	return code, nil
}

func compileInt8(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpInt)
	code.NumBitSize = 8
	ctx.incIndex()
	return code, nil
}

func compileInt8Ptr(ctx *compileContext) (*Opcode, error) {
	code, err := compileInt8(ctx)
	if err != nil {
		return nil, err
	}
	code.Op = OpIntPtr
	return code, nil
}

func compileInt16(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpInt)
	code.NumBitSize = 16
	ctx.incIndex()
	return code, nil
}

func compileInt16Ptr(ctx *compileContext) (*Opcode, error) {
	code, err := compileInt16(ctx)
	if err != nil {
		return nil, err
	}
	code.Op = OpIntPtr
	return code, nil
}

func compileInt32(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpInt)
	code.NumBitSize = 32
	ctx.incIndex()
	return code, nil
}

func compileInt32Ptr(ctx *compileContext) (*Opcode, error) {
	code, err := compileInt32(ctx)
	if err != nil {
		return nil, err
	}
	code.Op = OpIntPtr
	return code, nil
}

func compileInt64(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpInt)
	code.NumBitSize = 64
	ctx.incIndex()
	return code, nil
}

func compileInt64Ptr(ctx *compileContext) (*Opcode, error) {
	code, err := compileInt64(ctx)
	if err != nil {
		return nil, err
	}
	code.Op = OpIntPtr
	return code, nil
}

func compileUint(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpUint)
	code.NumBitSize = intSize
	ctx.incIndex()
	return code, nil
}

func compileUintPtr(ctx *compileContext) (*Opcode, error) {
	code, err := compileUint(ctx)
	if err != nil {
		return nil, err
	}
	code.Op = OpUintPtr
	return code, nil
}

func compileUint8(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpUint)
	code.NumBitSize = 8
	ctx.incIndex()
	return code, nil
}

func compileUint8Ptr(ctx *compileContext) (*Opcode, error) {
	code, err := compileUint8(ctx)
	if err != nil {
		return nil, err
	}
	code.Op = OpUintPtr
	return code, nil
}

func compileUint16(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpUint)
	code.NumBitSize = 16
	ctx.incIndex()
	return code, nil
}

func compileUint16Ptr(ctx *compileContext) (*Opcode, error) {
	code, err := compileUint16(ctx)
	if err != nil {
		return nil, err
	}
	code.Op = OpUintPtr
	return code, nil
}

func compileUint32(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpUint)
	code.NumBitSize = 32
	ctx.incIndex()
	return code, nil
}

func compileUint32Ptr(ctx *compileContext) (*Opcode, error) {
	code, err := compileUint32(ctx)
	if err != nil {
		return nil, err
	}
	code.Op = OpUintPtr
	return code, nil
}

func compileUint64(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpUint)
	code.NumBitSize = 64
	ctx.incIndex()
	return code, nil
}

func compileUint64Ptr(ctx *compileContext) (*Opcode, error) {
	code, err := compileUint64(ctx)
	if err != nil {
		return nil, err
	}
	code.Op = OpUintPtr
	return code, nil
}

func compileIntString(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpIntString)
	code.NumBitSize = intSize
	ctx.incIndex()
	return code, nil
}

func compileInt8String(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpIntString)
	code.NumBitSize = 8
	ctx.incIndex()
	return code, nil
}

func compileInt16String(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpIntString)
	code.NumBitSize = 16
	ctx.incIndex()
	return code, nil
}

func compileInt32String(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpIntString)
	code.NumBitSize = 32
	ctx.incIndex()
	return code, nil
}

func compileInt64String(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpIntString)
	code.NumBitSize = 64
	ctx.incIndex()
	return code, nil
}

func compileUintString(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpUintString)
	code.NumBitSize = intSize
	ctx.incIndex()
	return code, nil
}

func compileUint8String(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpUintString)
	code.NumBitSize = 8
	ctx.incIndex()
	return code, nil
}

func compileUint16String(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpUintString)
	code.NumBitSize = 16
	ctx.incIndex()
	return code, nil
}

func compileUint32String(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpUintString)
	code.NumBitSize = 32
	ctx.incIndex()
	return code, nil
}

func compileUint64String(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpUintString)
	code.NumBitSize = 64
	ctx.incIndex()
	return code, nil
}

func compileFloat32(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpFloat32)
	ctx.incIndex()
	return code, nil
}

func compileFloat32Ptr(ctx *compileContext) (*Opcode, error) {
	code, err := compileFloat32(ctx)
	if err != nil {
		return nil, err
	}
	code.Op = OpFloat32Ptr
	return code, nil
}

func compileFloat64(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpFloat64)
	ctx.incIndex()
	return code, nil
}

func compileFloat64Ptr(ctx *compileContext) (*Opcode, error) {
	code, err := compileFloat64(ctx)
	if err != nil {
		return nil, err
	}
	code.Op = OpFloat64Ptr
	return code, nil
}

func compileString(ctx *compileContext) (*Opcode, error) {
	var op OpType
	if ctx.typ == runtime.Type2RType(jsonNumberType) {
		op = OpNumber
	} else {
		op = OpString
	}
	code := newOpCode(ctx, op)
	ctx.incIndex()
	return code, nil
}

func compileStringPtr(ctx *compileContext) (*Opcode, error) {
	code, err := compileString(ctx)
	if err != nil {
		return nil, err
	}
	if code.Op == OpNumber {
		code.Op = OpNumberPtr
	} else {
		code.Op = OpStringPtr
	}
	return code, nil
}

func compileBool(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpBool)
	ctx.incIndex()
	return code, nil
}

func compileBoolPtr(ctx *compileContext) (*Opcode, error) {
	code, err := compileBool(ctx)
	if err != nil {
		return nil, err
	}
	code.Op = OpBoolPtr
	return code, nil
}

func compileBytes(ctx *compileContext) (*Opcode, error) {
	code := newOpCode(ctx, OpBytes)
	ctx.incIndex()
	return code, nil
}

func compileBytesPtr(ctx *compileContext) (*Opcode, error) {
	code, err := compileBytes(ctx)
	if err != nil {
		return nil, err
	}
	code.Op = OpBytesPtr
	return code, nil
}

func compileInterface(ctx *compileContext) (*Opcode, error) {
	code := newInterfaceCode(ctx)
	ctx.incIndex()
	return code, nil
}

func compileInterfacePtr(ctx *compileContext) (*Opcode, error) {
	code, err := compileInterface(ctx)
	if err != nil {
		return nil, err
	}
	code.Op = OpInterfacePtr
	return code, nil
}

func compileSlice(ctx *compileContext) (*Opcode, error) {
	elem := ctx.typ.Elem()
	size := elem.Size()

	header := newSliceHeaderCode(ctx)
	ctx.incIndex()

	code, err := compileListElem(ctx.withType(elem).incIndent())
	if err != nil {
		return nil, err
	}
	code.Flags |= IndirectFlags

	// header => opcode => elem => end
	//             ^        |
	//             |________|

	elemCode := newSliceElemCode(ctx, header, size)
	ctx.incIndex()

	end := newOpCode(ctx, OpSliceEnd)
	ctx.incIndex()

	header.End = end
	header.Next = code
	code.BeforeLastCode().Next = (*Opcode)(unsafe.Pointer(elemCode))
	elemCode.Next = code
	elemCode.End = end
	return (*Opcode)(unsafe.Pointer(header)), nil
}

func compileListElem(ctx *compileContext) (*Opcode, error) {
	typ := ctx.typ
	switch {
	case !typ.Implements(marshalJSONType) && runtime.PtrTo(typ).Implements(marshalJSONType):
		return compileMarshalJSON(ctx)
	case !typ.Implements(marshalTextType) && runtime.PtrTo(typ).Implements(marshalTextType):
		return compileMarshalText(ctx)
	case typ.Kind() == reflect.Map:
		return compilePtr(ctx.withType(runtime.PtrTo(typ)))
	default:
		code, err := compile(ctx, false)
		if err != nil {
			return nil, err
		}
		if code.Op == OpMapPtr {
			code.PtrNum++
		}
		return code, nil
	}
}

func compileArray(ctx *compileContext) (*Opcode, error) {
	typ := ctx.typ
	elem := typ.Elem()
	alen := typ.Len()
	size := elem.Size()

	header := newArrayHeaderCode(ctx, alen)
	ctx.incIndex()

	code, err := compileListElem(ctx.withType(elem).incIndent())
	if err != nil {
		return nil, err
	}
	code.Flags |= IndirectFlags
	// header => opcode => elem => end
	//             ^        |
	//             |________|

	elemCode := newArrayElemCode(ctx, header, alen, size)
	ctx.incIndex()

	end := newOpCode(ctx, OpArrayEnd)
	ctx.incIndex()

	header.End = end
	header.Next = code
	code.BeforeLastCode().Next = (*Opcode)(unsafe.Pointer(elemCode))
	elemCode.Next = code
	elemCode.End = end
	return (*Opcode)(unsafe.Pointer(header)), nil
}

func compileMap(ctx *compileContext) (*Opcode, error) {
	// header => code => value => code => key => code => value => code => end
	//                                     ^                       |
	//                                     |_______________________|
	ctx = ctx.incIndent()
	header := newMapHeaderCode(ctx)
	ctx.incIndex()

	typ := ctx.typ
	keyType := ctx.typ.Key()
	keyCode, err := compileKey(ctx.withType(keyType))
	if err != nil {
		return nil, err
	}

	value := newMapValueCode(ctx, header)
	ctx.incIndex()

	valueCode, err := compileMapValue(ctx.withType(typ.Elem()))
	if err != nil {
		return nil, err
	}
	valueCode.Flags |= IndirectFlags

	key := newMapKeyCode(ctx, header)
	ctx.incIndex()

	ctx = ctx.decIndent()

	end := newMapEndCode(ctx, header)
	ctx.incIndex()

	header.Next = keyCode
	keyCode.BeforeLastCode().Next = (*Opcode)(unsafe.Pointer(value))
	value.Next = valueCode
	valueCode.BeforeLastCode().Next = (*Opcode)(unsafe.Pointer(key))
	key.Next = keyCode

	header.End = end
	key.End = end
	value.End = end

	return (*Opcode)(unsafe.Pointer(header)), nil
}

func compileMapValue(ctx *compileContext) (*Opcode, error) {
	switch ctx.typ.Kind() {
	case reflect.Map:
		return compilePtr(ctx.withType(runtime.PtrTo(ctx.typ)))
	default:
		code, err := compile(ctx, false)
		if err != nil {
			return nil, err
		}
		if code.Op == OpMapPtr {
			code.PtrNum++
		}
		return code, nil
	}
}

func optimizeStructHeader(code *Opcode, tag *runtime.StructTag) OpType {
	headType := code.ToHeaderType(tag.IsString)
	if tag.IsOmitEmpty {
		headType = headType.HeadToOmitEmptyHead()
	}
	return headType
}

func optimizeStructField(code *Opcode, tag *runtime.StructTag) OpType {
	fieldType := code.ToFieldType(tag.IsString)
	if tag.IsOmitEmpty {
		fieldType = fieldType.FieldToOmitEmptyField()
	}
	return fieldType
}

func recursiveCode(ctx *compileContext, jmp *CompiledCode) *Opcode {
	code := newRecursiveCode(ctx, jmp)
	ctx.incIndex()
	return code
}

func compiledCode(ctx *compileContext) *Opcode {
	typ := ctx.typ
	typeptr := uintptr(unsafe.Pointer(typ))
	if cc, exists := ctx.structTypeToCompiledCode[typeptr]; exists {
		return recursiveCode(ctx, cc)
	}
	return nil
}

func structHeader(ctx *compileContext, fieldCode *Opcode, valueCode *Opcode, tag *runtime.StructTag) *Opcode {
	op := optimizeStructHeader(valueCode, tag)
	fieldCode.Op = op
	fieldCode.NumBitSize = valueCode.NumBitSize
	fieldCode.PtrNum = valueCode.PtrNum
	if op.IsMultipleOpHead() {
		return valueCode.BeforeLastCode()
	}
	ctx.decOpcodeIndex()
	return fieldCode
}

func structField(ctx *compileContext, fieldCode *Opcode, valueCode *Opcode, tag *runtime.StructTag) *Opcode {
	op := optimizeStructField(valueCode, tag)
	fieldCode.Op = op
	fieldCode.NumBitSize = valueCode.NumBitSize
	fieldCode.PtrNum = valueCode.PtrNum
	if op.IsMultipleOpField() {
		return valueCode.BeforeLastCode()
	}
	ctx.decIndex()
	return fieldCode
}

func isNotExistsField(head *Opcode) bool {
	if head == nil {
		return false
	}
	if head.Op != OpStructHead {
		return false
	}
	if (head.Flags & AnonymousHeadFlags) == 0 {
		return false
	}
	if head.Next == nil {
		return false
	}
	if head.NextField == nil {
		return false
	}
	if head.NextField.Op != OpStructAnonymousEnd {
		return false
	}
	if head.Next.Op == OpStructAnonymousEnd {
		return true
	}
	if head.Next.Op.CodeType() != CodeStructField {
		return false
	}
	return isNotExistsField(head.Next)
}

func optimizeAnonymousFields(head *Opcode) {
	code := head
	var prev *Opcode
	removedFields := map[*Opcode]struct{}{}
	for {
		if code.Op == OpStructEnd {
			break
		}
		if code.Op == OpStructField {
			codeType := code.Next.Op.CodeType()
			if codeType == CodeStructField {
				if isNotExistsField(code.Next) {
					code.Next = code.NextField
					diff := code.Next.DisplayIdx - code.DisplayIdx
					for i := uint32(0); i < diff; i++ {
						code.Next.decOpcodeIndex()
					}
					linkPrevToNextField(code, removedFields)
					code = prev
				}
			}
		}
		prev = code
		code = code.NextField
	}
}

type structFieldPair struct {
	prevField   *Opcode
	curField    *Opcode
	isTaggedKey bool
	linked      bool
}

func anonymousStructFieldPairMap(tags runtime.StructTags, named string, valueCode *Opcode) map[string][]structFieldPair {
	anonymousFields := map[string][]structFieldPair{}
	f := valueCode
	var prevAnonymousField *Opcode
	removedFields := map[*Opcode]struct{}{}
	for {
		existsKey := tags.ExistsKey(f.DisplayKey)
		isHeadOp := strings.Contains(f.Op.String(), "Head")
		if existsKey && f.Next != nil && strings.Contains(f.Next.Op.String(), "Recursive") {
			// through
		} else if isHeadOp && (f.Flags&AnonymousHeadFlags) == 0 {
			if existsKey {
				// TODO: need to remove this head
				f.Op = OpStructHead
				f.Flags |= AnonymousKeyFlags
				f.Flags |= AnonymousHeadFlags
			} else if named == "" {
				f.Flags |= AnonymousHeadFlags
			}
		} else if named == "" && f.Op == OpStructEnd {
			f.Op = OpStructAnonymousEnd
		} else if existsKey {
			diff := f.NextField.DisplayIdx - f.DisplayIdx
			for i := uint32(0); i < diff; i++ {
				f.NextField.decOpcodeIndex()
			}
			linkPrevToNextField(f, removedFields)
		}

		if f.DisplayKey == "" {
			if f.NextField == nil {
				break
			}
			prevAnonymousField = f
			f = f.NextField
			continue
		}

		key := fmt.Sprintf("%s.%s", named, f.DisplayKey)
		anonymousFields[key] = append(anonymousFields[key], structFieldPair{
			prevField:   prevAnonymousField,
			curField:    f,
			isTaggedKey: (f.Flags & IsTaggedKeyFlags) != 0,
		})
		if f.Next != nil && f.NextField != f.Next && f.Next.Op.CodeType() == CodeStructField {
			for k, v := range anonymousFieldPairRecursively(named, f.Next) {
				anonymousFields[k] = append(anonymousFields[k], v...)
			}
		}
		if f.NextField == nil {
			break
		}
		prevAnonymousField = f
		f = f.NextField
	}
	return anonymousFields
}

func anonymousFieldPairRecursively(named string, valueCode *Opcode) map[string][]structFieldPair {
	anonymousFields := map[string][]structFieldPair{}
	f := valueCode
	var prevAnonymousField *Opcode
	for {
		if f.DisplayKey != "" && (f.Flags&AnonymousHeadFlags) != 0 {
			key := fmt.Sprintf("%s.%s", named, f.DisplayKey)
			anonymousFields[key] = append(anonymousFields[key], structFieldPair{
				prevField:   prevAnonymousField,
				curField:    f,
				isTaggedKey: (f.Flags & IsTaggedKeyFlags) != 0,
			})
			if f.Next != nil && f.NextField != f.Next && f.Next.Op.CodeType() == CodeStructField {
				for k, v := range anonymousFieldPairRecursively(named, f.Next) {
					anonymousFields[k] = append(anonymousFields[k], v...)
				}
			}
		}
		if f.NextField == nil {
			break
		}
		prevAnonymousField = f
		f = f.NextField
	}
	return anonymousFields
}

func optimizeConflictAnonymousFields(anonymousFields map[string][]structFieldPair) {
	removedFields := map[*Opcode]struct{}{}
	for _, fieldPairs := range anonymousFields {
		if len(fieldPairs) == 1 {
			continue
		}
		// conflict anonymous fields
		taggedPairs := []structFieldPair{}
		for _, fieldPair := range fieldPairs {
			if fieldPair.isTaggedKey {
				taggedPairs = append(taggedPairs, fieldPair)
			} else {
				if !fieldPair.linked {
					if fieldPair.prevField == nil {
						// head operation
						fieldPair.curField.Op = OpStructHead
						fieldPair.curField.Flags |= AnonymousHeadFlags
						fieldPair.curField.Flags |= AnonymousKeyFlags
					} else {
						diff := fieldPair.curField.NextField.DisplayIdx - fieldPair.curField.DisplayIdx
						for i := uint32(0); i < diff; i++ {
							fieldPair.curField.NextField.decOpcodeIndex()
						}
						removedFields[fieldPair.curField] = struct{}{}
						linkPrevToNextField(fieldPair.curField, removedFields)
					}
					fieldPair.linked = true
				}
			}
		}
		if len(taggedPairs) > 1 {
			for _, fieldPair := range taggedPairs {
				if !fieldPair.linked {
					if fieldPair.prevField == nil {
						// head operation
						fieldPair.curField.Op = OpStructHead
						fieldPair.curField.Flags |= AnonymousHeadFlags
						fieldPair.curField.Flags |= AnonymousKeyFlags
					} else {
						diff := fieldPair.curField.NextField.DisplayIdx - fieldPair.curField.DisplayIdx
						removedFields[fieldPair.curField] = struct{}{}
						for i := uint32(0); i < diff; i++ {
							fieldPair.curField.NextField.decOpcodeIndex()
						}
						linkPrevToNextField(fieldPair.curField, removedFields)
					}
					fieldPair.linked = true
				}
			}
		} else {
			for _, fieldPair := range taggedPairs {
				fieldPair.curField.Flags &= ^IsTaggedKeyFlags
			}
		}
	}
}

func isNilableType(typ *runtime.Type) bool {
	switch typ.Kind() {
	case reflect.Ptr:
		return true
	case reflect.Map:
		return true
	case reflect.Func:
		return true
	default:
		return false
	}
}

func compileStruct(ctx *compileContext, isPtr bool) (*Opcode, error) {
	if code := compiledCode(ctx); code != nil {
		return code, nil
	}
	typ := ctx.typ
	typeptr := uintptr(unsafe.Pointer(typ))
	compiled := &CompiledCode{}
	ctx.structTypeToCompiledCode[typeptr] = compiled
	// header => code => structField => code => end
	//                        ^          |
	//                        |__________|
	fieldNum := typ.NumField()
	indirect := runtime.IfaceIndir(typ)
	fieldIdx := 0
	disableIndirectConversion := false
	var (
		head      *Opcode
		code      *Opcode
		prevField *Opcode
	)
	ctx = ctx.incIndent()
	tags := runtime.StructTags{}
	anonymousFields := map[string][]structFieldPair{}
	for i := 0; i < fieldNum; i++ {
		field := typ.Field(i)
		if runtime.IsIgnoredStructField(field) {
			continue
		}
		tags = append(tags, runtime.StructTagFromField(field))
	}
	for i, tag := range tags {
		field := tag.Field
		fieldType := runtime.Type2RType(field.Type)
		fieldOpcodeIndex := ctx.opcodeIndex
		fieldPtrIndex := ctx.ptrIndex
		ctx.incIndex()

		nilcheck := true
		addrForMarshaler := false
		isIndirectSpecialCase := isPtr && i == 0 && fieldNum == 1
		isNilableType := isNilableType(fieldType)

		var valueCode *Opcode
		switch {
		case isIndirectSpecialCase && !isNilableType && isPtrMarshalJSONType(fieldType):
			// *struct{ field T } => struct { field *T }
			// func (*T) MarshalJSON() ([]byte, error)
			// move pointer position from head to first field
			code, err := compileMarshalJSON(ctx.withType(fieldType))
			if err != nil {
				return nil, err
			}
			addrForMarshaler = true
			valueCode = code
			nilcheck = false
			indirect = false
			disableIndirectConversion = true
		case isIndirectSpecialCase && !isNilableType && isPtrMarshalTextType(fieldType):
			// *struct{ field T } => struct { field *T }
			// func (*T) MarshalText() ([]byte, error)
			// move pointer position from head to first field
			code, err := compileMarshalText(ctx.withType(fieldType))
			if err != nil {
				return nil, err
			}
			addrForMarshaler = true
			valueCode = code
			nilcheck = false
			indirect = false
			disableIndirectConversion = true
		case isPtr && isPtrMarshalJSONType(fieldType):
			// *struct{ field T }
			// func (*T) MarshalJSON() ([]byte, error)
			code, err := compileMarshalJSON(ctx.withType(fieldType))
			if err != nil {
				return nil, err
			}
			addrForMarshaler = true
			nilcheck = false
			valueCode = code
		case isPtr && isPtrMarshalTextType(fieldType):
			// *struct{ field T }
			// func (*T) MarshalText() ([]byte, error)
			code, err := compileMarshalText(ctx.withType(fieldType))
			if err != nil {
				return nil, err
			}
			addrForMarshaler = true
			nilcheck = false
			valueCode = code
		default:
			code, err := compile(ctx.withType(fieldType), isPtr)
			if err != nil {
				return nil, err
			}
			valueCode = code
		}

		if field.Anonymous {
			tagKey := ""
			if tag.IsTaggedKey {
				tagKey = tag.Key
			}
			for k, v := range anonymousStructFieldPairMap(tags, tagKey, valueCode) {
				anonymousFields[k] = append(anonymousFields[k], v...)
			}
			valueCode.decIndent()

			// fix issue144
			if !(isPtr && strings.Contains(valueCode.Op.String(), "Marshal")) {
				if indirect {
					valueCode.Flags |= IndirectFlags
				} else {
					valueCode.Flags &= ^IndirectFlags
				}
			}
		} else {
			if indirect {
				// if parent is indirect type, set child indirect property to true
				valueCode.Flags |= IndirectFlags
			} else {
				// if parent is not indirect type and child have only one field, set child indirect property to false
				if i == 0 && valueCode.NextField != nil && valueCode.NextField.Op == OpStructEnd {
					valueCode.Flags &= ^IndirectFlags
				}
			}
		}
		var flags OpFlags
		if indirect {
			flags |= IndirectFlags
		}
		if field.Anonymous {
			flags |= AnonymousKeyFlags
		}
		if tag.IsTaggedKey {
			flags |= IsTaggedKeyFlags
		}
		if nilcheck {
			flags |= NilCheckFlags
		}
		if addrForMarshaler {
			flags |= AddrForMarshalerFlags
		}
		if strings.Contains(valueCode.Op.String(), "Ptr") || valueCode.Op == OpInterface {
			flags |= IsNextOpPtrTypeFlags
		}
		if isNilableType {
			flags |= IsNilableTypeFlags
		}
		var key string
		if ctx.escapeKey {
			rctx := &RuntimeContext{Option: &Option{Flag: HTMLEscapeOption}}
			key = fmt.Sprintf(`%s:`, string(AppendString(rctx, []byte{}, tag.Key)))
		} else {
			key = fmt.Sprintf(`"%s":`, tag.Key)
		}
		fieldCode := &Opcode{
			Idx:        opcodeOffset(fieldPtrIndex),
			Next:       valueCode,
			Flags:      flags,
			Key:        key,
			Offset:     uint32(field.Offset),
			Type:       valueCode.Type,
			DisplayIdx: fieldOpcodeIndex,
			Indent:     ctx.indent,
			DisplayKey: tag.Key,
		}
		if fieldIdx == 0 {
			code = structHeader(ctx, fieldCode, valueCode, tag)
			head = fieldCode
			prevField = fieldCode
		} else {
			fieldCode.Idx = head.Idx
			code.Next = fieldCode
			code = structField(ctx, fieldCode, valueCode, tag)
			prevField.NextField = fieldCode
			fieldCode.PrevField = prevField
			prevField = fieldCode
		}
		fieldIdx++
	}

	structEndCode := &Opcode{
		Op:     OpStructEnd,
		Next:   newEndOp(ctx),
		Type:   nil,
		Indent: ctx.indent,
	}

	ctx = ctx.decIndent()

	// no struct field
	if head == nil {
		head = &Opcode{
			Op:         OpStructHead,
			Idx:        opcodeOffset(ctx.ptrIndex),
			NextField:  structEndCode,
			Type:       typ,
			DisplayIdx: ctx.opcodeIndex,
			Indent:     ctx.indent,
		}
		structEndCode.PrevField = head
		ctx.incIndex()
		code = head
	}

	structEndCode.DisplayIdx = ctx.opcodeIndex
	structEndCode.Idx = opcodeOffset(ctx.ptrIndex)
	ctx.incIndex()

	if prevField != nil && prevField.NextField == nil {
		prevField.NextField = structEndCode
		structEndCode.PrevField = prevField
	}

	head.End = structEndCode
	code.Next = structEndCode
	optimizeConflictAnonymousFields(anonymousFields)
	optimizeAnonymousFields(head)
	ret := (*Opcode)(unsafe.Pointer(head))
	compiled.Code = ret

	delete(ctx.structTypeToCompiledCode, typeptr)

	if !disableIndirectConversion && (head.Flags&IndirectFlags == 0) && isPtr {
		head.Flags |= IndirectFlags
	}

	return ret, nil
}

func isPtrMarshalJSONType(typ *runtime.Type) bool {
	return !typ.Implements(marshalJSONType) && runtime.PtrTo(typ).Implements(marshalJSONType)
}

func isPtrMarshalTextType(typ *runtime.Type) bool {
	return !typ.Implements(marshalTextType) && runtime.PtrTo(typ).Implements(marshalTextType)
}
