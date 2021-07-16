package encoder

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/goccy/go-json/internal/runtime"
)

const uintptrSize = 4 << (^uintptr(0) >> 63)

type OpFlags uint16

const (
	AnonymousHeadFlags    OpFlags = 1 << 0
	AnonymousKeyFlags     OpFlags = 1 << 1
	IndirectFlags         OpFlags = 1 << 2
	IsTaggedKeyFlags      OpFlags = 1 << 3
	NilCheckFlags         OpFlags = 1 << 4
	AddrForMarshalerFlags OpFlags = 1 << 5
	IsNextOpPtrTypeFlags  OpFlags = 1 << 6
	IsNilableTypeFlags    OpFlags = 1 << 7
	MarshalerContextFlags OpFlags = 1 << 8
)

type Opcode struct {
	Op         OpType  // operation type
	Idx        uint32  // offset to access ptr
	Next       *Opcode // next opcode
	End        *Opcode // array/slice/struct/map end
	NextField  *Opcode // next struct field
	Key        string  // struct field key
	Offset     uint32  // offset size from struct header
	PtrNum     uint8   // pointer number: e.g. double pointer is 2.
	NumBitSize uint8
	Flags      OpFlags

	Type       *runtime.Type // go type
	PrevField  *Opcode       // prev struct field
	Jmp        *CompiledCode // for recursive call
	ElemIdx    uint32        // offset to access array/slice/map elem
	Length     uint32        // offset to access slice/map length or array length
	MapIter    uint32        // offset to access map iterator
	MapPos     uint32        // offset to access position list for sorted map
	Indent     uint32        // indent number
	Size       uint32        // array/slice elem size
	DisplayIdx uint32        // opcode index
	DisplayKey string        // key text to display
}

func (c *Opcode) MaxIdx() uint32 {
	max := uint32(0)
	for _, value := range []uint32{
		c.Idx,
		c.ElemIdx,
		c.Length,
		c.MapIter,
		c.MapPos,
		c.Size,
	} {
		if max < value {
			max = value
		}
	}
	return max
}

func (c *Opcode) ToHeaderType(isString bool) OpType {
	switch c.Op {
	case OpInt:
		if isString {
			return OpStructHeadIntString
		}
		return OpStructHeadInt
	case OpIntPtr:
		if isString {
			return OpStructHeadIntPtrString
		}
		return OpStructHeadIntPtr
	case OpUint:
		if isString {
			return OpStructHeadUintString
		}
		return OpStructHeadUint
	case OpUintPtr:
		if isString {
			return OpStructHeadUintPtrString
		}
		return OpStructHeadUintPtr
	case OpFloat32:
		if isString {
			return OpStructHeadFloat32String
		}
		return OpStructHeadFloat32
	case OpFloat32Ptr:
		if isString {
			return OpStructHeadFloat32PtrString
		}
		return OpStructHeadFloat32Ptr
	case OpFloat64:
		if isString {
			return OpStructHeadFloat64String
		}
		return OpStructHeadFloat64
	case OpFloat64Ptr:
		if isString {
			return OpStructHeadFloat64PtrString
		}
		return OpStructHeadFloat64Ptr
	case OpString:
		if isString {
			return OpStructHeadStringString
		}
		return OpStructHeadString
	case OpStringPtr:
		if isString {
			return OpStructHeadStringPtrString
		}
		return OpStructHeadStringPtr
	case OpNumber:
		if isString {
			return OpStructHeadNumberString
		}
		return OpStructHeadNumber
	case OpNumberPtr:
		if isString {
			return OpStructHeadNumberPtrString
		}
		return OpStructHeadNumberPtr
	case OpBool:
		if isString {
			return OpStructHeadBoolString
		}
		return OpStructHeadBool
	case OpBoolPtr:
		if isString {
			return OpStructHeadBoolPtrString
		}
		return OpStructHeadBoolPtr
	case OpBytes:
		return OpStructHeadBytes
	case OpBytesPtr:
		return OpStructHeadBytesPtr
	case OpMap:
		return OpStructHeadMap
	case OpMapPtr:
		c.Op = OpMap
		return OpStructHeadMapPtr
	case OpArray:
		return OpStructHeadArray
	case OpArrayPtr:
		c.Op = OpArray
		return OpStructHeadArrayPtr
	case OpSlice:
		return OpStructHeadSlice
	case OpSlicePtr:
		c.Op = OpSlice
		return OpStructHeadSlicePtr
	case OpMarshalJSON:
		return OpStructHeadMarshalJSON
	case OpMarshalJSONPtr:
		return OpStructHeadMarshalJSONPtr
	case OpMarshalText:
		return OpStructHeadMarshalText
	case OpMarshalTextPtr:
		return OpStructHeadMarshalTextPtr
	}
	return OpStructHead
}

func (c *Opcode) ToFieldType(isString bool) OpType {
	switch c.Op {
	case OpInt:
		if isString {
			return OpStructFieldIntString
		}
		return OpStructFieldInt
	case OpIntPtr:
		if isString {
			return OpStructFieldIntPtrString
		}
		return OpStructFieldIntPtr
	case OpUint:
		if isString {
			return OpStructFieldUintString
		}
		return OpStructFieldUint
	case OpUintPtr:
		if isString {
			return OpStructFieldUintPtrString
		}
		return OpStructFieldUintPtr
	case OpFloat32:
		if isString {
			return OpStructFieldFloat32String
		}
		return OpStructFieldFloat32
	case OpFloat32Ptr:
		if isString {
			return OpStructFieldFloat32PtrString
		}
		return OpStructFieldFloat32Ptr
	case OpFloat64:
		if isString {
			return OpStructFieldFloat64String
		}
		return OpStructFieldFloat64
	case OpFloat64Ptr:
		if isString {
			return OpStructFieldFloat64PtrString
		}
		return OpStructFieldFloat64Ptr
	case OpString:
		if isString {
			return OpStructFieldStringString
		}
		return OpStructFieldString
	case OpStringPtr:
		if isString {
			return OpStructFieldStringPtrString
		}
		return OpStructFieldStringPtr
	case OpNumber:
		if isString {
			return OpStructFieldNumberString
		}
		return OpStructFieldNumber
	case OpNumberPtr:
		if isString {
			return OpStructFieldNumberPtrString
		}
		return OpStructFieldNumberPtr
	case OpBool:
		if isString {
			return OpStructFieldBoolString
		}
		return OpStructFieldBool
	case OpBoolPtr:
		if isString {
			return OpStructFieldBoolPtrString
		}
		return OpStructFieldBoolPtr
	case OpBytes:
		return OpStructFieldBytes
	case OpBytesPtr:
		return OpStructFieldBytesPtr
	case OpMap:
		return OpStructFieldMap
	case OpMapPtr:
		c.Op = OpMap
		return OpStructFieldMapPtr
	case OpArray:
		return OpStructFieldArray
	case OpArrayPtr:
		c.Op = OpArray
		return OpStructFieldArrayPtr
	case OpSlice:
		return OpStructFieldSlice
	case OpSlicePtr:
		c.Op = OpSlice
		return OpStructFieldSlicePtr
	case OpMarshalJSON:
		return OpStructFieldMarshalJSON
	case OpMarshalJSONPtr:
		return OpStructFieldMarshalJSONPtr
	case OpMarshalText:
		return OpStructFieldMarshalText
	case OpMarshalTextPtr:
		return OpStructFieldMarshalTextPtr
	}
	return OpStructField
}

func newOpCode(ctx *compileContext, op OpType) *Opcode {
	return newOpCodeWithNext(ctx, op, newEndOp(ctx))
}

func opcodeOffset(idx int) uint32 {
	return uint32(idx) * uintptrSize
}

func copyOpcode(code *Opcode) *Opcode {
	codeMap := map[uintptr]*Opcode{}
	return code.copy(codeMap)
}

func setTotalLengthToInterfaceOp(code *Opcode) {
	c := code
	for c.Op != OpEnd && c.Op != OpInterfaceEnd {
		if c.Op == OpInterface {
			c.Length = uint32(code.TotalLength())
		}
		switch c.Op.CodeType() {
		case CodeArrayElem, CodeSliceElem, CodeMapKey:
			c = c.End
		default:
			c = c.Next
		}
	}
}

func ToEndCode(code *Opcode) *Opcode {
	c := code
	for c.Op != OpEnd && c.Op != OpInterfaceEnd {
		switch c.Op.CodeType() {
		case CodeArrayElem, CodeSliceElem, CodeMapKey:
			c = c.End
		default:
			c = c.Next
		}
	}
	return c
}

func copyToInterfaceOpcode(code *Opcode) *Opcode {
	copied := copyOpcode(code)
	c := copied
	c = ToEndCode(c)
	c.Idx += uintptrSize
	c.ElemIdx = c.Idx + uintptrSize
	c.Length = c.Idx + 2*uintptrSize
	c.Op = OpInterfaceEnd
	return copied
}

func newOpCodeWithNext(ctx *compileContext, op OpType, next *Opcode) *Opcode {
	return &Opcode{
		Op:         op,
		Idx:        opcodeOffset(ctx.ptrIndex),
		Next:       next,
		Type:       ctx.typ,
		DisplayIdx: ctx.opcodeIndex,
		Indent:     ctx.indent,
	}
}

func newEndOp(ctx *compileContext) *Opcode {
	return newOpCodeWithNext(ctx, OpEnd, nil)
}

func (c *Opcode) copy(codeMap map[uintptr]*Opcode) *Opcode {
	if c == nil {
		return nil
	}
	addr := uintptr(unsafe.Pointer(c))
	if code, exists := codeMap[addr]; exists {
		return code
	}
	copied := &Opcode{
		Op:         c.Op,
		Key:        c.Key,
		PtrNum:     c.PtrNum,
		NumBitSize: c.NumBitSize,
		Flags:      c.Flags,
		Idx:        c.Idx,
		Offset:     c.Offset,
		Type:       c.Type,
		DisplayIdx: c.DisplayIdx,
		DisplayKey: c.DisplayKey,
		ElemIdx:    c.ElemIdx,
		Length:     c.Length,
		MapIter:    c.MapIter,
		MapPos:     c.MapPos,
		Size:       c.Size,
		Indent:     c.Indent,
	}
	codeMap[addr] = copied
	copied.End = c.End.copy(codeMap)
	copied.PrevField = c.PrevField.copy(codeMap)
	copied.NextField = c.NextField.copy(codeMap)
	copied.Next = c.Next.copy(codeMap)
	copied.Jmp = c.Jmp
	return copied
}

func (c *Opcode) BeforeLastCode() *Opcode {
	code := c
	for {
		var nextCode *Opcode
		switch code.Op.CodeType() {
		case CodeArrayElem, CodeSliceElem, CodeMapKey:
			nextCode = code.End
		default:
			nextCode = code.Next
		}
		if nextCode.Op == OpEnd {
			return code
		}
		code = nextCode
	}
}

func (c *Opcode) TotalLength() int {
	var idx int
	code := c
	for code.Op != OpEnd && code.Op != OpInterfaceEnd {
		maxIdx := int(code.MaxIdx() / uintptrSize)
		if idx < maxIdx {
			idx = maxIdx
		}
		if code.Op == OpRecursiveEnd {
			break
		}
		switch code.Op.CodeType() {
		case CodeArrayElem, CodeSliceElem, CodeMapKey:
			code = code.End
		default:
			code = code.Next
		}
	}
	maxIdx := int(code.MaxIdx() / uintptrSize)
	if idx < maxIdx {
		idx = maxIdx
	}
	return idx + 1
}

func (c *Opcode) decOpcodeIndex() {
	for code := c; code.Op != OpEnd; {
		code.DisplayIdx--
		if code.Idx > 0 {
			code.Idx -= uintptrSize
		}
		if code.ElemIdx > 0 {
			code.ElemIdx -= uintptrSize
		}
		if code.MapIter > 0 {
			code.MapIter -= uintptrSize
		}
		if code.Length > 0 && code.Op.CodeType() != CodeArrayHead && code.Op.CodeType() != CodeArrayElem {
			code.Length -= uintptrSize
		}
		switch code.Op.CodeType() {
		case CodeArrayElem, CodeSliceElem, CodeMapKey:
			code = code.End
		default:
			code = code.Next
		}
	}
}

func (c *Opcode) decIndent() {
	for code := c; code.Op != OpEnd; {
		code.Indent--
		switch code.Op.CodeType() {
		case CodeArrayElem, CodeSliceElem, CodeMapKey:
			code = code.End
		default:
			code = code.Next
		}
	}
}

func (c *Opcode) dumpHead(code *Opcode) string {
	var length uint32
	if code.Op.CodeType() == CodeArrayHead {
		length = code.Length
	} else {
		length = code.Length / uintptrSize
	}
	return fmt.Sprintf(
		`[%d]%s%s ([idx:%d][elemIdx:%d][length:%d])`,
		code.DisplayIdx,
		strings.Repeat("-", int(code.Indent)),
		code.Op,
		code.Idx/uintptrSize,
		code.ElemIdx/uintptrSize,
		length,
	)
}

func (c *Opcode) dumpMapHead(code *Opcode) string {
	return fmt.Sprintf(
		`[%d]%s%s ([idx:%d][elemIdx:%d][length:%d][mapIter:%d])`,
		code.DisplayIdx,
		strings.Repeat("-", int(code.Indent)),
		code.Op,
		code.Idx/uintptrSize,
		code.ElemIdx/uintptrSize,
		code.Length/uintptrSize,
		code.MapIter/uintptrSize,
	)
}

func (c *Opcode) dumpMapEnd(code *Opcode) string {
	return fmt.Sprintf(
		`[%d]%s%s ([idx:%d][mapPos:%d][length:%d])`,
		code.DisplayIdx,
		strings.Repeat("-", int(code.Indent)),
		code.Op,
		code.Idx/uintptrSize,
		code.MapPos/uintptrSize,
		code.Length/uintptrSize,
	)
}

func (c *Opcode) dumpElem(code *Opcode) string {
	var length uint32
	if code.Op.CodeType() == CodeArrayElem {
		length = code.Length
	} else {
		length = code.Length / uintptrSize
	}
	return fmt.Sprintf(
		`[%d]%s%s ([idx:%d][elemIdx:%d][length:%d][size:%d])`,
		code.DisplayIdx,
		strings.Repeat("-", int(code.Indent)),
		code.Op,
		code.Idx/uintptrSize,
		code.ElemIdx/uintptrSize,
		length,
		code.Size,
	)
}

func (c *Opcode) dumpField(code *Opcode) string {
	return fmt.Sprintf(
		`[%d]%s%s ([idx:%d][key:%s][offset:%d])`,
		code.DisplayIdx,
		strings.Repeat("-", int(code.Indent)),
		code.Op,
		code.Idx/uintptrSize,
		code.DisplayKey,
		code.Offset,
	)
}

func (c *Opcode) dumpKey(code *Opcode) string {
	return fmt.Sprintf(
		`[%d]%s%s ([idx:%d][elemIdx:%d][length:%d][mapIter:%d])`,
		code.DisplayIdx,
		strings.Repeat("-", int(code.Indent)),
		code.Op,
		code.Idx/uintptrSize,
		code.ElemIdx/uintptrSize,
		code.Length/uintptrSize,
		code.MapIter/uintptrSize,
	)
}

func (c *Opcode) dumpValue(code *Opcode) string {
	return fmt.Sprintf(
		`[%d]%s%s ([idx:%d][mapIter:%d])`,
		code.DisplayIdx,
		strings.Repeat("-", int(code.Indent)),
		code.Op,
		code.Idx/uintptrSize,
		code.MapIter/uintptrSize,
	)
}

func (c *Opcode) Dump() string {
	codes := []string{}
	for code := c; code.Op != OpEnd && code.Op != OpInterfaceEnd; {
		switch code.Op.CodeType() {
		case CodeSliceHead:
			codes = append(codes, c.dumpHead(code))
			code = code.Next
		case CodeMapHead:
			codes = append(codes, c.dumpMapHead(code))
			code = code.Next
		case CodeArrayElem, CodeSliceElem:
			codes = append(codes, c.dumpElem(code))
			code = code.End
		case CodeMapKey:
			codes = append(codes, c.dumpKey(code))
			code = code.End
		case CodeMapValue:
			codes = append(codes, c.dumpValue(code))
			code = code.Next
		case CodeMapEnd:
			codes = append(codes, c.dumpMapEnd(code))
			code = code.Next
		case CodeStructField:
			codes = append(codes, c.dumpField(code))
			code = code.Next
		case CodeStructEnd:
			codes = append(codes, c.dumpField(code))
			code = code.Next
		default:
			codes = append(codes, fmt.Sprintf(
				"[%d]%s%s ([idx:%d])",
				code.DisplayIdx,
				strings.Repeat("-", int(code.Indent)),
				code.Op,
				code.Idx/uintptrSize,
			))
			code = code.Next
		}
	}
	return strings.Join(codes, "\n")
}

func prevField(code *Opcode, removedFields map[*Opcode]struct{}) *Opcode {
	if _, exists := removedFields[code]; exists {
		return prevField(code.PrevField, removedFields)
	}
	return code
}

func nextField(code *Opcode, removedFields map[*Opcode]struct{}) *Opcode {
	if _, exists := removedFields[code]; exists {
		return nextField(code.NextField, removedFields)
	}
	return code
}

func linkPrevToNextField(cur *Opcode, removedFields map[*Opcode]struct{}) {
	prev := prevField(cur.PrevField, removedFields)
	prev.NextField = nextField(cur.NextField, removedFields)
	code := prev
	fcode := cur
	for {
		var nextCode *Opcode
		switch code.Op.CodeType() {
		case CodeArrayElem, CodeSliceElem, CodeMapKey:
			nextCode = code.End
		default:
			nextCode = code.Next
		}
		if nextCode == fcode {
			code.Next = fcode.Next
			break
		} else if nextCode.Op == OpEnd {
			break
		}
		code = nextCode
	}
}

func newSliceHeaderCode(ctx *compileContext) *Opcode {
	idx := opcodeOffset(ctx.ptrIndex)
	ctx.incPtrIndex()
	elemIdx := opcodeOffset(ctx.ptrIndex)
	ctx.incPtrIndex()
	length := opcodeOffset(ctx.ptrIndex)
	return &Opcode{
		Op:         OpSlice,
		Idx:        idx,
		DisplayIdx: ctx.opcodeIndex,
		ElemIdx:    elemIdx,
		Length:     length,
		Indent:     ctx.indent,
	}
}

func newSliceElemCode(ctx *compileContext, head *Opcode, size uintptr) *Opcode {
	return &Opcode{
		Op:         OpSliceElem,
		Idx:        head.Idx,
		DisplayIdx: ctx.opcodeIndex,
		ElemIdx:    head.ElemIdx,
		Length:     head.Length,
		Indent:     ctx.indent,
		Size:       uint32(size),
	}
}

func newArrayHeaderCode(ctx *compileContext, alen int) *Opcode {
	idx := opcodeOffset(ctx.ptrIndex)
	ctx.incPtrIndex()
	elemIdx := opcodeOffset(ctx.ptrIndex)
	return &Opcode{
		Op:         OpArray,
		Idx:        idx,
		DisplayIdx: ctx.opcodeIndex,
		ElemIdx:    elemIdx,
		Indent:     ctx.indent,
		Length:     uint32(alen),
	}
}

func newArrayElemCode(ctx *compileContext, head *Opcode, length int, size uintptr) *Opcode {
	return &Opcode{
		Op:         OpArrayElem,
		Idx:        head.Idx,
		DisplayIdx: ctx.opcodeIndex,
		ElemIdx:    head.ElemIdx,
		Length:     uint32(length),
		Indent:     ctx.indent,
		Size:       uint32(size),
	}
}

func newMapHeaderCode(ctx *compileContext) *Opcode {
	idx := opcodeOffset(ctx.ptrIndex)
	ctx.incPtrIndex()
	elemIdx := opcodeOffset(ctx.ptrIndex)
	ctx.incPtrIndex()
	length := opcodeOffset(ctx.ptrIndex)
	ctx.incPtrIndex()
	mapIter := opcodeOffset(ctx.ptrIndex)
	return &Opcode{
		Op:         OpMap,
		Idx:        idx,
		Type:       ctx.typ,
		DisplayIdx: ctx.opcodeIndex,
		ElemIdx:    elemIdx,
		Length:     length,
		MapIter:    mapIter,
		Indent:     ctx.indent,
	}
}

func newMapKeyCode(ctx *compileContext, head *Opcode) *Opcode {
	return &Opcode{
		Op:         OpMapKey,
		Idx:        opcodeOffset(ctx.ptrIndex),
		DisplayIdx: ctx.opcodeIndex,
		ElemIdx:    head.ElemIdx,
		Length:     head.Length,
		MapIter:    head.MapIter,
		Indent:     ctx.indent,
	}
}

func newMapValueCode(ctx *compileContext, head *Opcode) *Opcode {
	return &Opcode{
		Op:         OpMapValue,
		Idx:        opcodeOffset(ctx.ptrIndex),
		DisplayIdx: ctx.opcodeIndex,
		ElemIdx:    head.ElemIdx,
		Length:     head.Length,
		MapIter:    head.MapIter,
		Indent:     ctx.indent,
	}
}

func newMapEndCode(ctx *compileContext, head *Opcode) *Opcode {
	mapPos := opcodeOffset(ctx.ptrIndex)
	ctx.incPtrIndex()
	idx := opcodeOffset(ctx.ptrIndex)
	return &Opcode{
		Op:         OpMapEnd,
		Idx:        idx,
		Next:       newEndOp(ctx),
		DisplayIdx: ctx.opcodeIndex,
		Length:     head.Length,
		MapPos:     mapPos,
		Indent:     ctx.indent,
	}
}

func newInterfaceCode(ctx *compileContext) *Opcode {
	return &Opcode{
		Op:         OpInterface,
		Idx:        opcodeOffset(ctx.ptrIndex),
		Next:       newEndOp(ctx),
		Type:       ctx.typ,
		DisplayIdx: ctx.opcodeIndex,
		Indent:     ctx.indent,
	}
}

func newRecursiveCode(ctx *compileContext, jmp *CompiledCode) *Opcode {
	return &Opcode{
		Op:         OpRecursive,
		Idx:        opcodeOffset(ctx.ptrIndex),
		Next:       newEndOp(ctx),
		Type:       ctx.typ,
		DisplayIdx: ctx.opcodeIndex,
		Indent:     ctx.indent,
		Jmp:        jmp,
	}
}
