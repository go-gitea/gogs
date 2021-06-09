package json

import (
	"reflect"
	"unsafe"

	"github.com/goccy/go-json/internal/runtime"
)

type rtype = runtime.Type

type emptyInterface struct {
	typ *rtype
	ptr unsafe.Pointer
}

func rtype_ptrTo(t *rtype) *rtype {
	return runtime.PtrTo(t)
}

func rtype2type(t *rtype) reflect.Type {
	return runtime.RType2Type(t)
}

func type2rtype(t reflect.Type) *rtype {
	return runtime.Type2RType(t)
}
