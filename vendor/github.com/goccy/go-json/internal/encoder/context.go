package encoder

import (
	"context"
	"sync"
	"unsafe"

	"github.com/goccy/go-json/internal/runtime"
)

type compileContext struct {
	typ                      *runtime.Type
	opcodeIndex              uint32
	ptrIndex                 int
	indent                   uint32
	escapeKey                bool
	structTypeToCompiledCode map[uintptr]*CompiledCode

	parent *compileContext
}

func (c *compileContext) context() *compileContext {
	return &compileContext{
		typ:                      c.typ,
		opcodeIndex:              c.opcodeIndex,
		ptrIndex:                 c.ptrIndex,
		indent:                   c.indent,
		escapeKey:                c.escapeKey,
		structTypeToCompiledCode: c.structTypeToCompiledCode,
		parent:                   c,
	}
}

func (c *compileContext) withType(typ *runtime.Type) *compileContext {
	ctx := c.context()
	ctx.typ = typ
	return ctx
}

func (c *compileContext) incIndent() *compileContext {
	ctx := c.context()
	ctx.indent++
	return ctx
}

func (c *compileContext) decIndent() *compileContext {
	ctx := c.context()
	ctx.indent--
	return ctx
}

func (c *compileContext) incIndex() {
	c.incOpcodeIndex()
	c.incPtrIndex()
}

func (c *compileContext) decIndex() {
	c.decOpcodeIndex()
	c.decPtrIndex()
}

func (c *compileContext) incOpcodeIndex() {
	c.opcodeIndex++
	if c.parent != nil {
		c.parent.incOpcodeIndex()
	}
}

func (c *compileContext) decOpcodeIndex() {
	c.opcodeIndex--
	if c.parent != nil {
		c.parent.decOpcodeIndex()
	}
}

func (c *compileContext) incPtrIndex() {
	c.ptrIndex++
	if c.parent != nil {
		c.parent.incPtrIndex()
	}
}

func (c *compileContext) decPtrIndex() {
	c.ptrIndex--
	if c.parent != nil {
		c.parent.decPtrIndex()
	}
}

const (
	bufSize = 1024
)

var (
	runtimeContextPool = sync.Pool{
		New: func() interface{} {
			return &RuntimeContext{
				Buf:      make([]byte, 0, bufSize),
				Ptrs:     make([]uintptr, 128),
				KeepRefs: make([]unsafe.Pointer, 0, 8),
				Option:   &Option{},
			}
		},
	}
)

type RuntimeContext struct {
	Context    context.Context
	Buf        []byte
	MarshalBuf []byte
	Ptrs       []uintptr
	KeepRefs   []unsafe.Pointer
	SeenPtr    []uintptr
	BaseIndent uint32
	Prefix     []byte
	IndentStr  []byte
	Option     *Option
}

func (c *RuntimeContext) Init(p uintptr, codelen int) {
	if len(c.Ptrs) < codelen {
		c.Ptrs = make([]uintptr, codelen)
	}
	c.Ptrs[0] = p
	c.KeepRefs = c.KeepRefs[:0]
	c.SeenPtr = c.SeenPtr[:0]
	c.BaseIndent = 0
}

func (c *RuntimeContext) Ptr() uintptr {
	header := (*runtime.SliceHeader)(unsafe.Pointer(&c.Ptrs))
	return uintptr(header.Data)
}

func TakeRuntimeContext() *RuntimeContext {
	return runtimeContextPool.Get().(*RuntimeContext)
}

func ReleaseRuntimeContext(ctx *RuntimeContext) {
	runtimeContextPool.Put(ctx)
}
