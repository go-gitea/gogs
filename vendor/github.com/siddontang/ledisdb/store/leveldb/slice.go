// +build leveldb

package leveldb

// #cgo LDFLAGS: -lleveldb
// #include "leveldb/c.h"
import "C"

import (
	"reflect"
	"unsafe"
)

type CSlice struct {
	data unsafe.Pointer
	size int
}

func NewCSlice(p unsafe.Pointer, n int) *CSlice {
	return &CSlice{p, n}
}

func (s *CSlice) Data() []byte {
	var value []byte

	sH := (*reflect.SliceHeader)(unsafe.Pointer(&value))
	sH.Cap = int(s.size)
	sH.Len = int(s.size)
	sH.Data = uintptr(s.data)

	return value
}

func (s *CSlice) Size() int {
	return int(s.size)
}

func (s *CSlice) Free() {
	C.leveldb_free(s.data)
}
